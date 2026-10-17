package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// Entry is which entry a tool is listed in.
type Entry uint8

const (
	NotebookEntry Entry = 1 << iota // /mcp?notebook=<id>
	AccountEntry                    // /mcp
	BothEntries   = NotebookEntry | AccountEntry
)

// Annotations are a tool's hints (M9 design 5): openWorldHint is always
// false, destructiveHint only for a write.
type Annotations struct{ ReadOnly, Destructive, Idempotent bool }

// Spec describes a tool to its clients.
type Spec struct {
	Name, Title, Description string
	Annotations              Annotations
	Entry                    Entry
	// Notebook marks a tool that acts on a notebook: in the account entry
	// it takes a required "notebook", the notebook's id, which the
	// notebook entry takes from its address.
	Notebook bool
}

// Tool is a tool of the registry, as a module's adapter/mcp builds it.
type Tool struct {
	spec   Spec
	in     reflect.Type
	handle func(ctx context.Context, c Call, in any) (Result, error)
}

// Spec is t's description: the composition root's wiring test reads it
// (M9/P1 design 3.5).
func (t Tool) Spec() Spec { return t.spec }

// NewTool is the tool s, whose arguments decode into In, a struct: its
// fields without omitempty are required, a pointer field is optional and
// takes null as absent, and a string field is the one way to pass an id.
func NewTool[In any](s Spec, handle func(ctx context.Context, c Call, in In) (Result, error)) Tool {
	t := Tool{spec: s, in: reflect.TypeFor[In]()}
	if handle != nil { // New refuses a tool without one
		t.handle = func(ctx context.Context, c Call, in any) (Result, error) { return handle(ctx, c, *in.(*In)) }
	}
	return t
}

// Call is what a handler is told of its call.
type Call struct {
	Entry Entry
	// Notebook is the notebook entry's, gated, or the account entry's
	// argument: the zero id when it is no UUID, which the use cases
	// answer notebook.not_found without a read.
	Notebook        uuid.UUID
	Client          string // cleaned; UnknownClient when none is known
	UserID, TokenID uuid.UUID
}

// UnknownClient names a client that told no name (M9 design 4.5).
const UnknownClient = "unknown"

// Result is a tool's answer: Structured for a structured tool, whose
// content is its JSON text; Blocks for a text tool (read), which has no
// structuredContent. Exactly one is set.
type Result struct {
	Structured any
	Blocks     []Block
}

// Block is one content block: an image with Image, a resource link with
// Link, a text block otherwise, its Text possibly empty (read's lines of
// none, M9/P1 design 3.11.1). Text beside an image or a link, or both of
// them, is no block.
type Block struct {
	Text  string
	Image *Image
	Link  *Link
}

// Image is an image block.
type Image struct {
	Data []byte // the file's bytes; the SDK writes them in base64
	MIME string
}

// Link is a resource link block: where to fetch a file, and what it is.
type Link struct {
	URI, Name, MIME string
	Size            int64
}

// toolName is the names a tool may have: the SDK only logs a name it
// would refuse, and a client may refuse the tool.
var toolName = regexp.MustCompile(`^[a-z_]{1,32}$`)

// addTools registers tools in the servers of their entries, each through
// the SDK's low-level AddTool, which checks no argument, writes no
// structuredContent and wants no output schema: the handler mcpserver
// wraps around each does that.
func (s *Server) addTools(tools []Tool) {
	named := map[Entry]map[string]bool{NotebookEntry: {}, AccountEntry: {}}
	for _, t := range tools {
		spec := t.spec
		switch {
		case !toolName.MatchString(spec.Name):
			panic(fmt.Sprintf("mcpserver: tool %q: a name is 1 to 32 of a-z and _", spec.Name))
		case spec.Entry == 0 || spec.Entry&^BothEntries != 0:
			panic(fmt.Sprintf("mcpserver: tool %s is listed in no entry", spec.Name))
		case spec.Notebook && spec.Entry&AccountEntry == 0:
			panic(fmt.Sprintf("mcpserver: tool %s takes a notebook, but only the account entry gives it one", spec.Name))
		case t.handle == nil:
			panic(fmt.Sprintf("mcpserver: tool %s has no handler", spec.Name))
		}
		for _, e := range []Entry{NotebookEntry, AccountEntry} {
			if spec.Entry&e == 0 {
				continue
			}
			if named[e][spec.Name] {
				panic(fmt.Sprintf("mcpserver: tool %s is registered twice in the %s entry", spec.Name, e))
			}
			named[e][spec.Name] = true
			notebook := spec.Notebook && e == AccountEntry
			schema, table := inputOf(spec.Name, t.in, notebook)
			s.serverOf(e).AddTool(&mcp.Tool{
				Name: spec.Name, Title: spec.Title, Description: spec.Description, InputSchema: schema,
				Annotations: annotationsOf(spec.Annotations),
			}, s.handlerOf(t, e, notebook, table))
		}
	}
}

// annotationsOf is a's hints as the SDK writes them.
func annotationsOf(a Annotations) *mcp.ToolAnnotations {
	hints := &mcp.ToolAnnotations{ReadOnlyHint: a.ReadOnly, IdempotentHint: a.Idempotent, OpenWorldHint: new(false)}
	if a.Destructive {
		hints.DestructiveHint = new(true)
	}
	return hints
}

// handlerOf wraps t for entry e: its arguments checked against the
// table of its schema, the account entry's notebook taken out of them
// when notebook says it is there, the rest decoded into a new In; the
// handler called; its result or error answered, within the answer's
// bound; and one log line. It never answers a result and an error
// together: the SDK's jsonrpc2 would log the result, content and all.
func (s *Server) handlerOf(t Tool, e Entry, notebook bool, table *bodyshape.Table) mcp.ToolHandler {
	name := t.spec.Name
	return func(ctx context.Context, req *mcp.CallToolRequest) (res *mcp.CallToolResult, err error) {
		start := time.Now()
		r := requestOf(ctx)
		call := Call{Entry: e, Notebook: r.notebook, Client: s.clientOf(req), UserID: r.cred.UserID, TokenID: r.cred.TokenID}
		outcome := outcomeOK
		defer func() { s.logCall(ctx, name, call, outcome, time.Since(start)) }()
		defer s.recoverTool(ctx, name, &res, &err, &outcome)
		args := []byte(req.Params.Arguments)
		if trimmed := bytes.TrimSpace(args); len(trimmed) == 0 || string(trimmed) == "null" {
			args = []byte("{}")
		}
		var shape *bodyshape.Error
		switch cerr := table.Check(name, args); {
		case errors.As(cerr, &shape):
			res, outcome = s.argumentsError(ctx, name, shape)
			return res, nil
		case cerr != nil:
			res, outcome = s.toolError(ctx, name, fmt.Errorf("mcpserver: tool %s: checking its arguments: %w", name, cerr))
			return res, nil
		}
		if notebook {
			id, rest, nerr := takeNotebook(args)
			if nerr != nil {
				res, outcome = s.toolError(ctx, name, fmt.Errorf("mcpserver: tool %s: %w", name, nerr))
				return res, nil
			}
			call.Notebook, args = id, rest
		}
		in := reflect.New(t.in)
		dec := json.NewDecoder(bytes.NewReader(args))
		dec.DisallowUnknownFields()
		if err := dec.Decode(in.Interface()); err != nil {
			// The check passed: the schema and the type disagree.
			res, outcome = s.toolError(ctx, name, fmt.Errorf("mcpserver: tool %s: decoding its checked arguments: %w", name, err))
			return res, nil
		}
		result, herr := t.handle(ctx, call, in.Interface())
		if herr != nil {
			res, outcome = s.toolError(ctx, name, herr)
			return res, nil
		}
		res, outcome = s.answer(ctx, name, result)
		return res, nil
	}
}

// takeNotebook takes the account entry's notebook out of args, which the
// check found to hold it as a string: its id as REST parses one, the zero
// id for no UUID, and args without it.
func takeNotebook(args []byte) (uuid.UUID, []byte, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(args, &members); err != nil {
		return uuid.UUID{}, nil, fmt.Errorf("reading its checked arguments: %w", err)
	}
	var value string
	if err := json.Unmarshal(members[notebookParameter], &value); err != nil {
		return uuid.UUID{}, nil, fmt.Errorf("reading its checked notebook: %w", err)
	}
	delete(members, notebookParameter)
	rest, err := json.Marshal(members)
	if err != nil {
		return uuid.UUID{}, nil, fmt.Errorf("writing its arguments: %w", err)
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, rest, nil
	}
	return id, rest, nil
}

// answer is result as the SDK sends it, within MaxAnswerBytes: a
// structured result encoded once, its text block and structuredContent
// the same bytes; a text result's blocks. Neither or both set, a
// structured result that is no JSON object (MCP's structuredContent is
// one), a block of two kinds, and an answer over the bound are the tool's
// fault: internal_error.
func (s *Server) answer(ctx context.Context, name string, result Result) (*mcp.CallToolResult, string) {
	var res *mcp.CallToolResult
	switch {
	case result.Structured == nil && result.Blocks == nil:
		return s.toolError(ctx, name, fmt.Errorf("mcpserver: tool %s answered no result", name))
	case result.Structured != nil && result.Blocks != nil:
		return s.toolError(ctx, name, fmt.Errorf("mcpserver: tool %s answered both a structured result and blocks", name))
	case result.Structured != nil:
		text, err := encode(result.Structured)
		if err != nil {
			return s.toolError(ctx, name, fmt.Errorf("mcpserver: tool %s: encoding its result: %w", name, err))
		}
		if !bytes.HasPrefix(text, []byte("{")) {
			return s.toolError(ctx, name, fmt.Errorf("mcpserver: tool %s answered a structured result that is no JSON object", name))
		}
		res = &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(text)}}, StructuredContent: text}
	default:
		content, err := contentOf(result.Blocks)
		if err != nil {
			return s.toolError(ctx, name, fmt.Errorf("mcpserver: tool %s: %w", name, err))
		}
		res = &mcp.CallToolResult{Content: content}
	}
	if n := answerBytes(res); n > MaxAnswerBytes {
		return s.overBound(ctx, name, n)
	}
	return res, outcomeOK
}

// contentOf is blocks as the SDK's content.
func contentOf(blocks []Block) ([]mcp.Content, error) {
	content := make([]mcp.Content, 0, len(blocks))
	for i, b := range blocks {
		kinds := 0
		for _, set := range []bool{b.Image != nil, b.Link != nil} {
			if set {
				kinds++
			}
		}
		switch {
		case kinds > 1 || kinds == 1 && b.Text != "":
			return nil, fmt.Errorf("block %d is of two kinds, not one", i)
		case b.Image != nil:
			content = append(content, &mcp.ImageContent{Data: b.Image.Data, MIMEType: b.Image.MIME})
		case b.Link != nil:
			content = append(content, &mcp.ResourceLink{URI: b.Link.URI, Name: b.Link.Name, MIMEType: b.Link.MIME, Size: new(b.Link.Size)})
		default:
			content = append(content, &mcp.TextContent{Text: b.Text})
		}
	}
	return content, nil
}

// encode is v as mcpserver writes a structured result: without the HTML
// escapes, so that the model reads <, > and & as they are, and without
// the encoder's newline.
func encode(v any) (json.RawMessage, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// clientOf is the name of the client req tells, cleaned (M9 design 4.5);
// UnknownClient without one, or without the names (S3).
func (s *Server) clientOf(req mcp.Request) string {
	if s.names == nil {
		return UnknownClient
	}
	var info *mcp.Implementation
	switch r := req.(type) {
	case *mcp.InitializeRequest:
		if r.Params != nil {
			info = r.Params.ClientInfo
		}
	case interface{ ClientInfo() *mcp.Implementation }:
		info = r.ClientInfo()
	}
	if info == nil {
		return UnknownClient
	}
	if name := s.names.Clean(info.Name); name != "" {
		return name
	}
	return UnknownClient
}
