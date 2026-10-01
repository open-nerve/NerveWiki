package apitest

import (
	"io"
	"iter"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const instanceJSON = `{"product":"Nerve Wiki","version":"0.1.0-dev","commit":"unknown","api_version":"v0","signup_enabled":false,"workspace_creation_enabled":true}`

// Component names become Go and TypeScript type names. Redocly's bundler
// renames a clash between module files to "Name-2" and only warns, so a
// clash has to fail here instead. Security schemes are the exception: they
// name no type, and every module will declare the same bearer.
func TestComponentNamesAreTypeNames(t *testing.T) {
	c := Load(t)
	valid := regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	names := componentNames(c.doc)
	if len(names) == 0 {
		t.Fatal("the contract has no components")
	}
	for _, name := range names {
		kind, base, _ := strings.Cut(name, "/")
		if kind == "securitySchemes" {
			continue
		}
		if !valid.MatchString(base) {
			t.Errorf("component %s is not a PascalCase type name; two module files may define it differently", name)
		}
	}
}

// componentNames lists every component of doc as "<kind>/<name>", sorted.
func componentNames(doc *openapi3.T) []string {
	c := doc.Components
	var names []string
	add := func(kind string, keys iter.Seq[string]) {
		for name := range keys {
			names = append(names, kind+"/"+name)
		}
	}
	add("schemas", maps.Keys(c.Schemas))
	add("responses", maps.Keys(c.Responses))
	add("parameters", maps.Keys(c.Parameters))
	add("requestBodies", maps.Keys(c.RequestBodies))
	add("headers", maps.Keys(c.Headers))
	add("securitySchemes", maps.Keys(c.SecuritySchemes))
	add("examples", maps.Keys(c.Examples))
	add("links", maps.Keys(c.Links))
	add("callbacks", maps.Keys(c.Callbacks))
	slices.Sort(names)
	return names
}

// parse loads an in-memory OpenAPI document the way Load reads the contract.
func parse(t *testing.T, yaml string) *openapi3.T {
	t.Helper()
	doc, err := newLoader().LoadFromData([]byte(yaml))
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}
	return doc
}

func TestComponentNamesCoverEveryKind(t *testing.T) {
	doc := parse(t, `
openapi: 3.1.0
info: {title: every component kind, version: v0}
paths: {}
components:
  schemas: {S: {type: string}}
  responses: {R: {description: r}}
  parameters: {P: {name: p, in: query, schema: {type: string}}}
  requestBodies: {B: {content: {application/json: {schema: {type: string}}}}}
  headers: {H: {schema: {type: string}}}
  securitySchemes: {Sec: {type: http, scheme: bearer}}
  examples: {E: {value: 1}}
  links: {L: {operationId: getThing}}
  callbacks: {C: {'{$request.body#/url}': {post: {responses: {'200': {description: ok}}}}}}
`)
	want := []string{
		"callbacks/C", "examples/E", "headers/H", "links/L", "parameters/P",
		"requestBodies/B", "responses/R", "schemas/S", "securitySchemes/Sec",
	}
	if got := componentNames(doc); !slices.Equal(got, want) {
		t.Errorf("componentNames() = %q, want %q", got, want)
	}
}

func TestValidateResponse(t *testing.T) {
	c := Load(t)
	tests := []struct {
		name, method, path string
		status             int
		contentType, body  string
		valid              bool
	}{
		{"documented 200", "GET", "/api/v0/instance", 200, "application/json", instanceJSON, true},
		{"problem as default", "GET", "/api/v0/instance", 500, "application/problem+json", `{"status":500,"code":"internal_error","title":"Internal Server Error"}`, true},
		{"missing field", "GET", "/api/v0/instance", 200, "application/json", `{"product":"Nerve Wiki","version":"0.1.0-dev","api_version":"v0"}`, false},
		{"value outside the enum", "GET", "/api/v0/instance", 200, "application/json", strings.Replace(instanceJSON, `"v0"`, `"v1"`, 1), false},
		{"undocumented field", "GET", "/api/v0/instance", 200, "application/json", strings.Replace(instanceJSON, `}`, `,"extra":1}`, 1), false},
		{"undocumented content type", "GET", "/api/v0/instance", 200, "text/plain", "Nerve Wiki", false},
		{"problem without code", "GET", "/api/v0/instance", 500, "application/problem+json", `{"status":500,"title":"Internal Server Error"}`, false},
		{"undocumented path", "GET", "/api/v0/nope", 404, "application/problem+json", `{"status":404,"code":"not_found","title":"Not Found"}`, false},
		{"undocumented method", "POST", "/api/v0/instance", 404, "application/problem+json", `{"status":404,"code":"not_found","title":"Not Found"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			header := http.Header{"Content-Type": {tt.contentType}}
			err := c.validateResponse(req, tt.status, header, []byte(tt.body))
			if (err == nil) != tt.valid {
				t.Errorf("validateResponse() = %v, want valid = %v", err, tt.valid)
			}
		})
	}
}

// instanceResponse is a valid answer to GET /api/v0/instance.
func instanceResponse() *http.Response {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.WriteHeader(http.StatusOK)
	_, _ = rec.WriteString(instanceJSON)
	return rec.Result()
}

func TestCheckResponseLeavesTheBodyReadable(t *testing.T) {
	c := Load(t)
	res := instanceResponse()

	c.CheckResponse(t, httptest.NewRequest(http.MethodGet, "/api/v0/instance", nil), res)

	body, err := io.ReadAll(res.Body)
	if err != nil || string(body) != instanceJSON {
		t.Errorf("body after CheckResponse = %q, %v; want %s", body, err, instanceJSON)
	}
}

// With servers in the contract, the router would also match each request's
// scheme and host against them, which no test host does.
func TestLoadIgnoresServers(t *testing.T) {
	dist, err := os.ReadFile(contractPath())
	if err != nil {
		t.Fatal(err)
	}
	text := string(dist)
	for _, edit := range []struct{ before, insert string }{
		{"paths:\n", "servers:\n  - url: https://wiki.example.org\n"},
		{"    get:\n", "    servers:\n      - url: https://path.example.org\n"},
		{"      responses:\n", "      servers:\n        - url: https://operation.example.org\n"},
	} {
		if !strings.Contains(text, edit.before) {
			t.Fatalf("api/dist/openapi.yaml has no %q to insert servers before", edit.before)
		}
		text = strings.Replace(text, edit.before, edit.insert+edit.before, 1)
	}
	path := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := load(path)
	if err != nil {
		t.Fatal(err)
	}

	c.CheckResponse(t, httptest.NewRequest(http.MethodGet, "/api/v0/instance", nil), instanceResponse())
}

func TestEnum(t *testing.T) {
	c := Load(t)
	if codes, err := c.enum("FieldError", "code"); err != nil || !slices.Contains(codes, "required") {
		t.Errorf("enum(FieldError, code) = %q, %v; want the field codes", codes, err)
	}
	for _, tt := range []struct{ schema, property string }{
		{"Nope", "code"},
		{"FieldError", "nope"},
		{"FieldError", "field"}, // a property without an enum
	} {
		if values, err := c.enum(tt.schema, tt.property); err == nil {
			t.Errorf("enum(%s, %s) = %q, want an error", tt.schema, tt.property, values)
		}
	}
}

func TestValidateSchema(t *testing.T) {
	c := Load(t)
	tests := []struct {
		name, schema, body string
		valid              bool
	}{
		{"problem", "Problem", `{"status":404,"code":"not_found","title":"Not Found","detail":"no API endpoint for GET /api/v0/nope"}`, true},
		{"problem with field errors", "Problem", `{"status":422,"code":"validation_failed","title":"Unprocessable Entity","errors":[{"field":"name","code":"required","message":"is required"}]}`, true},
		{"field error without code", "Problem", `{"status":422,"code":"validation_failed","title":"Unprocessable Entity","errors":[{"field":"name","message":"is required"}]}`, false},
		{"field error with an unknown code", "Problem", `{"status":422,"code":"validation_failed","title":"Unprocessable Entity","errors":[{"field":"name","code":"blank","message":"is required"}]}`, false},
		{"problem without code", "Problem", `{"status":404,"title":"Not Found"}`, false},
		{"problem with an unknown member", "Problem", `{"status":404,"code":"not_found","title":"Not Found","instance":"/x"}`, false},
		{"unknown schema", "Nope", `{}`, false},
		{"not JSON", "Problem", `not json`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.validateSchema(tt.schema, []byte(tt.body))
			if (err == nil) != tt.valid {
				t.Errorf("validateSchema() = %v, want valid = %v", err, tt.valid)
			}
		})
	}
}
