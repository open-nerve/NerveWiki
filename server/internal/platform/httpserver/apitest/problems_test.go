package apitest

import (
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

// The operation and its code are this test's own: no other test answers
// them, so what the recorder holds for them is what CheckResponse put there.
func TestCheckResponseRecordsTheAnsweredCode(t *testing.T) {
	doc := strings.NewReplacer("operationId: createThing", "operationId: recordThing", "[things.taken]", "[things.recorded]").Replace(thingsContract)
	c := contractFrom(t, doc)
	if got := answered.snapshot()["recordThing"]; got != nil {
		t.Fatalf("recordThing has answers %v before CheckResponse", got)
	}
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
