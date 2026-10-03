package apitest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// thingsContract has a public operation with a body and one that needs a
// token, each with its own problem codes.
const thingsContract = `
openapi: 3.1.0
info: {title: things, version: v0}
x-problem-codes: [bad_request, internal_error]
paths:
  /api/v0/things:
    post:
      operationId: createThing
      security: []
      x-problem-codes: [things.taken]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              additionalProperties: false
              required: [name]
              properties:
                name: {type: string}
      responses:
        '204': {description: created}
        default: {$ref: '#/components/responses/Problem'}
    get:
      operationId: listThings
      security: [{bearer: []}]
      x-problem-codes: []
      responses:
        '204': {description: none}
        default: {$ref: '#/components/responses/Problem'}
components:
  securitySchemes:
    bearer: {type: http, scheme: bearer}
  responses:
    Problem:
      description: Error.
      content:
        application/problem+json:
          schema:
            type: object
            required: [code]
            properties:
              code: {type: string}
`

func contractFrom(t *testing.T, doc string) *Contract {
	t.Helper()
	path := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestValidateResponseChecksTheProblemCode(t *testing.T) {
	c := contractFrom(t, thingsContract)
	tests := []struct {
		name, method, code string
		valid              bool
	}{
		{"the operation's own code", "POST", "things.taken", true},
		{"a top-level code", "POST", "internal_error", true},
		{"an undeclared code", "POST", "not_found", false},
		{"unauthorized from a public operation", "POST", "unauthorized", false},
		{"unauthorized from an operation that needs a token", "GET", "unauthorized", true},
		{"another operation's code", "GET", "things.taken", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v0/things", nil)
			header := http.Header{"Content-Type": {"application/problem+json"}}
			err := c.validateResponse(req, 409, header, []byte(`{"code":"`+tt.code+`"}`))
			if (err == nil) != tt.valid {
				t.Errorf("validateResponse() = %v, want valid = %v", err, tt.valid)
			}
		})
	}
}

// forget drops what r recorded for operationID: a test that asserts what the
// recorder holds starts from nothing, however often -count runs it.
func (r *recorder) forget(operationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.codes, operationID)
}

// The operation and its code are this test's own: no other test answers
// them, so what the recorder holds for them is what CheckResponse put there.
func TestCheckResponseRecordsTheAnsweredCode(t *testing.T) {
	doc := strings.NewReplacer("operationId: createThing", "operationId: recordThing", "[things.taken]", "[things.recorded]").Replace(thingsContract)
	c := contractFrom(t, doc)
	answered.forget("recordThing")
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/problem+json")
	rec.WriteHeader(http.StatusConflict)
	_, _ = rec.WriteString(`{"code":"things.recorded"}`)

	c.CheckResponse(t, httptest.NewRequest(http.MethodPost, "/api/v0/things", nil), rec.Result())

	if got := answered.snapshot()["recordThing"]; !maps.Equal(got, map[string]bool{"things.recorded": true}) {
		t.Errorf("recordThing answered %v, want only things.recorded", got)
	}
}

func TestUnansweredListsTheCodesNoTestAnswered(t *testing.T) {
	doc := parse(t, strings.Replace(thingsContract, "x-problem-codes: []", "x-problem-codes: [things.gone, not_found]", 1))
	got := unanswered(doc, map[string]map[string]bool{"listThings": {"not_found": true}, "other": {"things.gone": true}})

	want := []string{"createThing: things.taken", "listThings: things.gone"}
	if !slices.Equal(got, want) {
		t.Errorf("unanswered() = %q, want %q", got, want)
	}
}

func TestValidateRequest(t *testing.T) {
	c := contractFrom(t, thingsContract)
	tests := []struct {
		name, method, path, body string
		valid                    bool
	}{
		{"documented body", "POST", "/api/v0/things", `{"name":"a"}`, true},
		{"missing required field", "POST", "/api/v0/things", `{}`, false},
		{"undocumented field", "POST", "/api/v0/things", `{"name":"a","x":1}`, false},
		{"security is not checked", "GET", "/api/v0/things", "", true},
		{"undocumented path", "GET", "/api/v0/nope", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			err := c.validateRequest(req)
			if (err == nil) != tt.valid {
				t.Errorf("validateRequest() = %v, want valid = %v", err, tt.valid)
			}
			if body, _ := io.ReadAll(req.Body); string(body) != tt.body {
				t.Errorf("body after validation = %q, want %q", body, tt.body)
			}
		})
	}
}

func TestModuleOf(t *testing.T) {
	tests := []struct {
		file, want string
		ok         bool
	}{
		{"/src/server/internal/modules/instance/adapter/http/main_test.go", "instance", true},
		{"/src/server/internal/modules/page/adapter/http/handler_test.go", "page", true},
		{"/src/server/internal/modules/page/app/main_test.go", "page", false},
		{"/src/server/internal/modules/page/adapter/http/gen/x_test.go", "page", false},
		{"/src/server/internal/bootstrap/main_test.go", "", false},
	}
	for _, tt := range tests {
		if got, ok := moduleOf(tt.file); ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("moduleOf(%s) = %q, %v; want %q, %v", tt.file, got, ok, tt.want, tt.ok)
		}
	}
}

// Main is what checks, after a module's HTTP adapter tests, that every code
// the module declares was answered: a module whose adapter tests do not run
// it would drop that half of the check without failing.
func TestEveryModuleRunsMain(t *testing.T) {
	names, err := moduleNames()
	if err != nil || len(names) == 0 {
		t.Fatalf("module files = %q, %v; want at least one", names, err)
	}
	for _, name := range names {
		dir := filepath.Join(apiDir(), "..", "server", "internal", "modules", name, "adapter", "http")
		if !runsMain(t, dir) {
			t.Errorf("api/modules/%s.yaml: no test file in %s has func TestMain(m *testing.M) { apitest.Main(m) }", name, dir)
		}
	}
}

// runsMain reports whether a test file in dir declares a TestMain that calls
// apitest.Main.
func runsMain(t *testing.T, dir string) bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "TestMain" || fn.Body == nil {
				continue
			}
			found := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Main" {
						if id, ok := sel.X.(*ast.Ident); ok && id.Name == "apitest" {
							found = true
						}
					}
				}
				return !found
			})
			if found {
				return true
			}
		}
	}
	return false
}

// streamsContract has a long-lived operation.
const streamsContract = `
openapi: 3.1.0
info: {title: streams, version: v0}
x-problem-codes: [internal_error]
paths:
  /api/v0/events:
    get:
      operationId: streamEvents
      security: [{bearer: []}]
      x-long-lived: true
      x-problem-codes: [not_ready]
      responses:
        '200':
          description: the stream
          content:
            text/event-stream:
              schema: {type: string}
        default: {$ref: '#/components/responses/Problem'}
components:
  securitySchemes:
    bearer: {type: http, scheme: bearer}
  responses:
    Problem:
      description: Error.
      content:
        application/problem+json:
          schema:
            type: object
            required: [code]
            properties:
              code: {type: string}
`

// readTrap fails the test when a response body is read.
type readTrap struct{ t *testing.T }

func (r readTrap) Read([]byte) (int, error) {
	r.t.Error("the long-lived response's body was read")
	return 0, io.EOF
}

func (readTrap) Close() error { return nil }

// A long-lived operation is marked so; the head of its 200 is checked
// without reading the body, which never ends: a Content-Type it documents
// passes, another does not. Its problem is checked as any other.
func TestCheckResponseReadsTheHeadOfAStream(t *testing.T) {
	c := contractFrom(t, streamsContract)
	if ops := c.Operations(); len(ops) != 1 || !ops[0].LongLived {
		t.Errorf("Operations() = %+v, want the stream, long-lived", ops)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v0/events", nil)
	res := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream; charset=utf-8"}},
		Body: readTrap{t}}
	c.CheckResponse(t, req, res)

	route, _, err := c.router.FindRoute(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := streamHead(route.Operation, http.Header{"Content-Type": {"application/json"}}); err == nil {
		t.Error("streamHead() of application/json = nil, want an error")
	}
	if err := c.validateResponse(req, http.StatusServiceUnavailable, http.Header{"Content-Type": {"application/problem+json"}},
		[]byte(`{"code":"not_ready"}`)); err != nil {
		t.Errorf("validateResponse() of its problem = %v", err)
	}
}
