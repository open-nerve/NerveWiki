package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// MaxPromptArgumentBytes bounds each argument a prompt declares (M10
// design 4.15): 60 KiB of question fit in a request of MaxOtherBytes, and
// an answer that repeats it would pass its bound.
const MaxPromptArgumentBytes = 4096

// methodGetPrompt is the method of a prompt's get.
const methodGetPrompt = "prompts/get"

// codeValidationFailed is shared.CodeValidationFailed, the code of the
// arguments a prompt refuses: mcpserver does not import shared.
const codeValidationFailed = "validation_failed"

// Prompt is a prompt of the registry, listed in Entry. Its arguments are
// strings, as MCP's PromptArgument are: no JSON schema.
type Prompt struct {
	Name, Title, Description string
	Entry                    Entry
	Arguments                []PromptArgument
	Get                      func(ctx context.Context, c Call, args map[string]string) (description string, messages []string, err error)
}

// PromptArgument is one named string a prompt takes.
type PromptArgument struct {
	Name, Description string
	Required          bool
}

// addPrompts registers prompts in their entries: a prompt of no entry, one
// named twice, a name that is none and one without its handler are wiring
// faults.
func (s *Server) addPrompts(prompts []Prompt) {
	named := map[string]bool{}
	for _, p := range prompts {
		switch {
		case !toolName.MatchString(p.Name):
			panic(fmt.Sprintf("mcpserver: prompt %q: a name is 1 to 32 of a-z and _", p.Name))
		case p.Entry == 0 || p.Entry&^BothEntries != 0:
			panic(fmt.Sprintf("mcpserver: prompt %s is listed in no entry", p.Name))
		case named[p.Name]:
			panic(fmt.Sprintf("mcpserver: prompt %s is registered twice", p.Name))
		case p.Get == nil:
			panic(fmt.Sprintf("mcpserver: prompt %s has no handler", p.Name))
		}
		named[p.Name] = true
		args := make([]*mcp.PromptArgument, len(p.Arguments))
		for i, a := range p.Arguments {
			args[i] = &mcp.PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required}
		}
		for _, e := range []Entry{NotebookEntry, AccountEntry} {
			if p.Entry&e != 0 {
				s.serverOf(e).AddPrompt(&mcp.Prompt{Name: p.Name, Title: p.Title, Description: p.Description, Arguments: args}, s.promptHandler(p, e))
			}
		}
	}
}

// promptHandler is p's prompts/get in entry e, in its order (M10 design
// 4.15): the declared arguments, before the handler; the handler, whose
// error is classified as a tool's is; the answer's bound. A get that
// reaches it has its one log line, a panic's too, which outer recovers
// after it: a name the SDK does not know reaches no handler and has none
// (M10 decision 162).
func (s *Server) promptHandler(p Prompt, e Entry) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		r := requestOf(ctx)
		call := Call{Entry: e, Notebook: r.notebook, Client: clientOf(ctx), UserID: r.cred.UserID, TokenID: r.cred.TokenID}
		start := time.Now()
		outcome := httpserver.CodeInternal
		defer func() { s.logPrompt(ctx, p.Name, call, outcome, time.Since(start)) }()
		if err := promptArgumentsError(p, req.Params.Arguments); err != nil {
			outcome = codeValidationFailed
			return nil, err
		}
		description, messages, err := p.Get(ctx, call, req.Params.Arguments)
		if err != nil {
			outcome, err = s.promptError(ctx, err)
			return nil, err
		}
		n := TextBytes(description)
		res := &mcp.GetPromptResult{Description: description, Messages: make([]*mcp.PromptMessage, len(messages))}
		for i, m := range messages {
			n += TextBytes(m)
			res.Messages[i] = &mcp.PromptMessage{Role: "user", Content: &mcp.TextContent{Text: m}}
		}
		if n > MaxAnswerBytes {
			s.logger.LogAttrs(ctx, slog.LevelError, "MCP answer over its bound",
				slog.String("prompt", p.Name), slog.Int("bytes", n), slog.String("request_id", httpserver.RequestID(ctx)))
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: httpserver.CodeInternal + ": " + internalFailed + "; request id " + httpserver.RequestID(ctx)}
		}
		outcome = outcomeOK
		return res, nil
	}
}

// promptArgumentsError refuses args in the order p declares its
// arguments: a required one left out, one over MaxPromptArgumentBytes.
// Its words name the argument, never its value. An argument given empty
// is given: a tool that follows judges it.
func promptArgumentsError(p Prompt, args map[string]string) error {
	for _, a := range p.Arguments {
		v, ok := args[a.Name]
		switch {
		case a.Required && !ok:
			return invalidPrompt(fmt.Sprintf("prompt %s: missing the argument %q", p.Name, a.Name))
		case len(v) > MaxPromptArgumentBytes:
			return invalidPrompt(fmt.Sprintf("prompt %s: the argument %q is over %d bytes", p.Name, a.Name, MaxPromptArgumentBytes))
		}
	}
	return nil
}

// invalidPrompt is the -32602 of arguments a prompt refuses.
func invalidPrompt(words string) error {
	return codedError(jsonrpc.CodeInvalidParams, codeValidationFailed, words)
}

// promptError is the JSON-RPC error of err, a prompt's handler's, and the
// outcome its log line says: a problem in its own words, -32602 for a 4xx
// and -32603 for a 5xx, its code in data, as a client that reads no text
// needs it; a fault as methodError answers any method's.
func (s *Server) promptError(ctx context.Context, err error) (string, error) {
	a := httpserver.ProblemOf(ctx, err)
	if a.Fault != httpserver.FaultNone {
		return httpserver.CodeInternal, s.methodError(ctx, methodGetPrompt, err)
	}
	code := int64(jsonrpc.CodeInvalidParams)
	if a.Problem.Status >= http.StatusInternalServerError {
		code = jsonrpc.CodeInternalError
	}
	return a.Problem.Code, codedError(code, a.Problem.Code, a.Problem.Detail)
}

// codedError is the JSON-RPC error code of words, which it starts with
// problem, as a tool's error text does, and data that says problem.
func codedError(code int64, problem, words string) error {
	// A member of strings alone always encodes.
	data, _ := json.Marshal(map[string]string{"code": problem})
	return &jsonrpc.Error{Code: code, Message: problem + ": " + words, Data: data}
}
