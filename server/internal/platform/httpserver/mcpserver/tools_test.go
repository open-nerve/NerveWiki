package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// echoIn is echo's arguments: one optional string.
type echoIn struct {
	S *string `json:"s,omitempty" jsonschema:"A string the answer repeats."`
}

// echoTool answers what its call was told, in both entries: the entry,
// the notebook, the client and the credential, and its string.
func echoTool() Tool {
	return NewTool(Spec{Name: "echo", Title: "Echo", Description: "Answers what its call was told.",
		Annotations: Annotations{ReadOnly: true, Idempotent: true}, Entry: BothEntries, Notebook: true},
		func(_ context.Context, c Call, in echoIn) (Result, error) {
			out := map[string]any{"entry": c.Entry.String(), "notebook": c.Notebook.String(), "client": c.Client,
				"user": c.UserID.String(), "token": c.TokenID.String()}
			if in.S != nil {
				out["s"] = *in.S
			}
			return Result{Structured: out}, nil
		})
}

// testTools are the tools every unit test's server has.
func testTools() []Tool { return []Tool{echoTool()} }

// toolOf is a tool named name of both entries, its arguments none, that
// answers what handle does.
func toolOf(name string, handle func(ctx context.Context, c Call) (Result, error)) Tool {
	return NewTool(Spec{Name: name, Title: name, Description: "A test's tool.", Entry: BothEntries},
		func(ctx context.Context, c Call, _ struct{}) (Result, error) { return handle(ctx, c) })
}

// listIn and readIn are shaped like S2's list and read.
type listIn struct {
	Pattern *string `json:"pattern,omitempty" jsonschema:"A glob."`
	Limit   *int    `json:"limit,omitempty"`
	Cursor  *string `json:"cursor,omitempty"`
}

type readIn struct {
	Path   string `json:"path" jsonschema:"The path."`
	Offset *int   `json:"offset,omitempty"`
	Column *int   `json:"column,omitempty"`
	Limit  *int   `json:"limit,omitempty"`
}

// schemaJSON is inputOf's schema of in, as tools/list writes it, decoded.
func schemaJSON(t *testing.T, in reflect.Type, notebook bool) map[string]any {
	t.Helper()
	schema, _ := inputOf("t", in, notebook)
	b, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// decodeJSON decodes s.
func decodeJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("%s: %v", s, err)
	}
	return v
}

// The schemas of S2's two tools, field by field, in each entry: required
// and optional, an optional scalar's null kept, the account entry's
// notebook first among the properties and the required, none in the
// notebook entry.
func TestTheInputSchemasNormalize(t *testing.T) {
	const listProps = `"pattern":{"type":["null","string"],"description":"A glob."},"limit":{"type":["null","integer"]},"cursor":{"type":["null","string"]}`
	const readProps = `"path":{"type":"string","description":"The path."},"offset":{"type":["null","integer"]},` +
		`"column":{"type":["null","integer"]},"limit":{"type":["null","integer"]}`
	const notebook = `"notebook":{"type":"string","description":"The notebook's id, from notebooks."}`
	tests := []struct {
		name     string
		in       reflect.Type
		notebook bool
		want     string
	}{
		{"list, the notebook entry", reflect.TypeFor[listIn](), false,
			`{"type":"object","properties":{` + listProps + `},"additionalProperties":false}`},
		{"list, the account entry", reflect.TypeFor[listIn](), true,
			`{"type":"object","properties":{` + notebook + `,` + listProps + `},"required":["notebook"],"additionalProperties":false}`},
		{"read, the notebook entry", reflect.TypeFor[readIn](), false,
			`{"type":"object","properties":{` + readProps + `},"required":["path"],"additionalProperties":false}`},
		{"read, the account entry", reflect.TypeFor[readIn](), true,
			`{"type":"object","properties":{` + notebook + `,` + readProps + `},"required":["notebook","path"],"additionalProperties":false}`},
		{"no arguments", reflect.TypeFor[struct{}](), false, `{"type":"object","additionalProperties":false}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := schemaJSON(t, tt.in, tt.notebook); !reflect.DeepEqual(got, decodeJSON(t, tt.want)) {
				t.Errorf("schema = %v, want %s", got, tt.want)
			}
			schema, _ := inputOf("t", tt.in, tt.notebook)
			if first := slices.Index(schema.PropertyOrder, "notebook"); tt.notebook && first != 0 {
				t.Errorf("properties in the order %q, want notebook first", schema.PropertyOrder)
			}
		})
	}
}

// opIn is an item of an array, as P3's batch has one.
type opIn struct {
	Path string  `json:"path"`
	Text *string `json:"text,omitempty"`
}

// nestedIn has what the list and read do not: a required array of
// objects, a required pointer, an optional array.
type nestedIn struct {
	Ops  []opIn    `json:"ops"`
	Must *int      `json:"must"`
	Tags *[]string `json:"tags,omitempty"`
}

// A required field, an array and an array's items take no null; an
// optional scalar keeps it, down an array's objects.
func TestTheInputSchemasTakeNullOnlyForAnOptionalScalar(t *testing.T) {
	want := `{"type":"object","properties":{` +
		`"ops":{"type":"array","items":{"type":"object","properties":{"path":{"type":"string"},"text":{"type":["null","string"]}},` +
		`"required":["path"],"additionalProperties":false}},` +
		`"must":{"type":"integer"},"tags":{"type":"array","items":{"type":"string"}}},` +
		`"required":["ops","must"],"additionalProperties":false}`
	if got := schemaJSON(t, reflect.TypeFor[nestedIn](), false); !reflect.DeepEqual(got, decodeJSON(t, want)) {
		t.Errorf("schema = %v, want %s", got, want)
	}
}

// The table holds the schema's structure: each property's types,
// required, closed objects, an array's items.
func TestTheStructureTableIsTheSchemas(t *testing.T) {
	_, table := inputOf("nested", reflect.TypeFor[nestedIn](), true)
	tests := []struct {
		args string
		want []string // field: code
	}{
		{`{"notebook":"x","ops":[],"must":1}`, nil},
		{`{"notebook":"x","ops":[{"path":"a","text":null}],"must":1,"tags":["a"]}`, nil},
		{`{"ops":[],"must":1}`, []string{"notebook: required"}},
		{`{"notebook":"x","ops":[{"path":"a","extra":1}],"must":null,"tags":null}`,
			[]string{"must: invalid_format", "ops[0].extra: not_allowed", "tags: invalid_format"}},
		{`{"notebook":1,"ops":[{}],"must":"1","Must":2}`,
			[]string{"Must: not_allowed", "must: invalid_format", "notebook: invalid_format", "ops[0].path: required"}},
		{`[]`, []string{": invalid_format"}},
	}
	for _, tt := range tests {
		var got []string
		if err := table.Check("nested", []byte(tt.args)); err != nil {
			for _, f := range err.(interface{ ProblemFields() []error }).ProblemFields() {
				pf := f.(interface {
					ProblemField() string
					ProblemCode() string
				})
				got = append(got, pf.ProblemField()+": "+pf.ProblemCode())
			}
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("Check(%s) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

// exotic are input types inputOf refuses, each for its reason.
type (
	withID       struct{ ID uuid.UUID }
	withTime     struct{ At time.Time }
	withMap      struct{ M map[string]string }
	withAny      struct{ V any }
	withArray    struct{ A [3]int }
	withUint     struct{ N uint }
	withBytes    struct{ B []byte }
	withOptional struct {
		S string `json:"s,omitempty"`
	}
	withNotebook struct {
		Notebook string `json:"notebook"`
	}
	withDeepTime struct{ Ops []struct{ At *time.Time } }
)

// A type the schema cannot hold panics, from the Go type when its schema
// would look like a string's or a slice's.
func TestTheInputSchemasRefuseWhatTheyCannotHold(t *testing.T) {
	tests := []struct {
		name string
		in   reflect.Type
		want string
	}{
		{"no struct", reflect.TypeFor[string](), "is no struct"},
		{"a uuid", reflect.TypeFor[withID](), "JSON form of its own"},
		{"a time", reflect.TypeFor[withTime](), "JSON form of its own"},
		{"a time deep down", reflect.TypeFor[withDeepTime](), "JSON form of its own"},
		{"a map", reflect.TypeFor[withMap](), "is a map"},
		{"an interface", reflect.TypeFor[withAny](), "is a interface"},
		{"an array", reflect.TypeFor[withArray](), "is a array"},
		{"a bounded integer", reflect.TypeFor[withUint](), `the keyword "minimum"`},
		{"bytes", reflect.TypeFor[withBytes](), `the keyword "m`},
		{"an optional field that is no pointer", reflect.TypeFor[withOptional](), "optional but no pointer"},
		{"the entry's notebook", reflect.TypeFor[withNotebook](), `has a "notebook"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if v := recover(); v == nil || !strings.Contains(v.(string), tt.want) {
					t.Errorf("inputOf(%v) panicked with %v, want %q", tt.in, v, tt.want)
				}
			}()
			inputOf("t", tt.in, false)
		})
	}
}

// A keyword the table has no place for is refused: a value's checks are
// the use case's 422, after the structure's 400.
func TestTheTableRefusesAKeywordOfAValue(t *testing.T) {
	for _, keyword := range []string{"enum", "minimum", "pattern", "maxLength"} {
		schema := map[string]any{"type": "object", "properties": map[string]any{"p": map[string]any{"type": "string", keyword: 1}}}
		err := addNode(&bodyshape.Table{}, schema, "arguments")
		if err == nil || !strings.Contains(err.Error(), `arguments.p has the keyword "`+keyword+`"`) {
			t.Errorf("a schema with %s: %v, want it refused at arguments.p", keyword, err)
		}
	}
}

// Both entries list their own tools, by name; each says tools without
// listChanged, and no logging or prompts; their lists are private and
// never cached.
func TestBothEntriesListTheirTools(t *testing.T) {
	none := func(context.Context, Call, struct{}) (Result, error) {
		return Result{Structured: map[string]any{}}, nil
	}
	nbOnly := NewTool(Spec{Name: "nb_only", Entry: NotebookEntry}, none)
	acctOnly := NewTool(Spec{Name: "acct_only", Entry: AccountEntry}, none)
	s := serveMCP(t, setup{options: func(o *Options) { o.Tools = append(o.Tools, nbOnly, acctOnly) }})
	for _, tt := range []struct {
		path string
		want []string
	}{
		{Pattern + "?notebook=" + visible.String(), []string{"echo", "nb_only"}},
		{Pattern, []string{"acct_only", "echo"}},
	} {
		cs := s.connect(t, tt.path, nil)
		caps, _ := json.Marshal(cs.InitializeResult().Capabilities)
		if !jsonEqual(decodeJSON(t, string(caps)), map[string]any{"tools": map[string]any{}}) {
			t.Errorf("%s: capabilities %s, want tools without listChanged, no logging, no prompts", tt.path, caps)
		}
		list := result(t, s.post(t, context.Background(), tt.path, patToken, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, v2025()))
		var names []string
		for _, tool := range list["tools"].([]any) {
			names = append(names, tool.(map[string]any)["name"].(string))
		}
		if !slices.Equal(names, tt.want) || list["cacheScope"] != "private" || list["ttlMs"] != float64(0) {
			t.Errorf("%s: tools/list = %v, cacheScope %v, ttlMs %v; want %q, private, 0", tt.path, names, list["cacheScope"], list["ttlMs"], tt.want)
		}
	}
}

// A tool of the account entry alone that acts on a notebook, as S3's
// open_notebook, registers, its notebook required.
func TestAnAccountNotebookToolRegisters(t *testing.T) {
	open := NewTool(Spec{Name: "open_notebook", Entry: AccountEntry, Notebook: true},
		func(_ context.Context, c Call, _ struct{}) (Result, error) {
			return Result{Structured: map[string]any{"notebook": c.Notebook.String()}}, nil
		})
	s := serveMCP(t, setup{options: func(o *Options) { o.Tools = []Tool{open} }})
	cs := s.connect(t, Pattern, nil)
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	schema, _ := json.Marshal(list.Tools[0].InputSchema)
	if len(list.Tools) != 1 || !strings.Contains(string(schema), `"required":["notebook"]`) {
		t.Errorf("the account entry's tools = %v %s, want open_notebook, its notebook required", list.Tools, schema)
	}
	got := result(t, s.call(t, Pattern, "open_notebook", map[string]any{"notebook": visible.String()}))
	if sc := got["structuredContent"].(map[string]any); sc["notebook"] != visible.String() {
		t.Errorf("open_notebook = %v, want its notebook", got)
	}
	nb := s.connect(t, Pattern+"?notebook="+visible.String(), nil)
	if list, err := nb.ListTools(context.Background(), nil); err != nil || len(list.Tools) != 0 {
		t.Errorf("the notebook entry's tools = %v, %v; want none", list, err)
	}
}

// New refuses each fault of its wiring: a tool's name, its entry, a
// Notebook tool the account entry has not, a name twice in an entry, a
// prompt twice or without its handler.
func TestNewRefusesItsWiringFaults(t *testing.T) {
	tool := func(name string, e Entry, notebook bool) Tool {
		return NewTool(Spec{Name: name, Entry: e, Notebook: notebook},
			func(context.Context, Call, struct{}) (Result, error) { return Result{}, nil })
	}
	get := func(context.Context, Call, map[string]string) (string, []string, error) { return "", nil, nil }
	tests := []struct {
		name    string
		tools   []Tool
		prompts []Prompt
		want    string
	}{
		{"a capital", []Tool{tool("Echo", BothEntries, false)}, nil, "a name is 1 to 32"},
		{"no name", []Tool{tool("", BothEntries, false)}, nil, "a name is 1 to 32"},
		{"a long name", []Tool{tool(strings.Repeat("a", 33), BothEntries, false)}, nil, "a name is 1 to 32"},
		{"a dash", []Tool{tool("a-b", BothEntries, false)}, nil, "a name is 1 to 32"},
		{"no entry", []Tool{tool("a", 0, false)}, nil, "listed in no entry"},
		{"an entry that is none", []Tool{tool("a", 4, false)}, nil, "listed in no entry"},
		{"a notebook of the notebook entry alone", []Tool{tool("a", NotebookEntry, true)}, nil, "only the account entry"},
		{"twice in an entry", []Tool{tool("a", NotebookEntry, false), tool("a", BothEntries, false)}, nil, "twice in the notebook entry"},
		{"a prompt twice", nil, []Prompt{{Name: "p", Get: get}, {Name: "p", Get: get}}, "prompt p is registered twice"},
		{"a prompt without its handler", nil, []Prompt{{Name: "p"}}, "prompt p has no handler"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if v := recover(); v == nil || !strings.Contains(v.(string), tt.want) {
					t.Errorf("New() panicked with %v, want %q", v, tt.want)
				}
			}()
			New(Options{Tools: tt.tools, Prompts: tt.prompts, Credentials: credentialsOf, Notebooks: &fakeNotebooks{},
				RequestTimeout: time.Second, MaxBodyBytes: 1, Logger: discard()})
		})
	}
	// Each entry's own name: one of each entry is no conflict.
	New(Options{Tools: []Tool{tool("a", NotebookEntry, false), tool("a", AccountEntry, false)}, Credentials: credentialsOf,
		Notebooks: &fakeNotebooks{}, RequestTimeout: time.Second, MaxBodyBytes: 1, Logger: discard()})
}

// annotations are a tool's hints on the wire: openWorldHint false always,
// destructiveHint only for a write.
func TestTheAnnotationsAreTheHints(t *testing.T) {
	for _, tt := range []struct {
		a    Annotations
		want string
	}{
		{Annotations{ReadOnly: true, Idempotent: true}, `{"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`},
		{Annotations{Destructive: true}, `{"destructiveHint":true,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`},
	} {
		if got, _ := json.Marshal(annotationsOf(tt.a)); string(got) != tt.want {
			t.Errorf("annotations of %+v = %s, want %s", tt.a, got, tt.want)
		}
	}
}
