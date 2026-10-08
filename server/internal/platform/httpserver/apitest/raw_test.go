package apitest

import (
	"errors"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

// rawContract has a raw download, whose 200 names its file, and a raw
// upload that answers JSON.
const rawContract = `
openapi: 3.1.0
info: {title: raw, version: v0}
x-problem-codes: [bad_request]
paths:
  /api/v0/things/{thing_id}/content:
    parameters:
      - {name: thing_id, in: path, required: true, schema: {type: string}}
    get:
      operationId: getThingContent
      security: []
      x-raw: true
      x-problem-codes: [not_found]
      responses:
        '200':
          description: The bytes.
          headers:
            Content-Disposition: {required: true, schema: {type: string}}
          content:
            '*/*':
              schema: {type: string, format: binary}
        '206':
          description: A range of the bytes.
          content:
            '*/*':
              schema: {type: string, format: binary}
        '304': {description: Not modified.}
        default: {$ref: '#/components/responses/Problem'}
    put:
      operationId: putThingContent
      security: [{bearer: []}]
      x-raw: true
      x-problem-codes: []
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              additionalProperties: false
              properties:
                file: {type: string, format: binary}
      responses:
        '201':
          description: Stored.
          content:
            application/json:
              schema:
                type: object
                additionalProperties: false
                required: [id]
                properties:
                  id: {type: string}
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

// A raw operation's answer through */* is checked by its status,
// Content-Type and headers, whatever its bytes; its JSON answers, the
// problem among them, are checked as any other.
func TestCheckResponseOfARawOperation(t *testing.T) {
	c := contractFrom(t, rawContract)
	png := "\x89PNG\r\n\x1a\n\x00\x00"
	disposition := http.Header{"Content-Disposition": {`inline; filename="a.png"`}}
	tests := []struct {
		name, method      string
		status            int
		header            http.Header
		contentType, body string
		valid             bool
	}{
		{"image", "GET", 200, disposition, "image/png", png, true},
		{"JSON as a file", "GET", 200, disposition, "application/json", `[1,`, true},
		{"HTML as a file", "GET", 200, disposition, "text/html; charset=utf-8", "<p>", true},
		{"range", "GET", 206, nil, "application/pdf", "%PDF-", true},
		{"not modified", "GET", 304, nil, "", "", true},
		{"without its header", "GET", 200, nil, "image/png", png, false},
		{"without a Content-Type", "GET", 200, disposition, "", png, false},
		{"undocumented status", "GET", 202, nil, "image/png", png, false},
		{"its problem", "GET", 404, nil, "application/problem+json", `{"code":"not_found"}`, true},
		{"its problem without a Content-Type", "GET", 404, nil, "", `{"code":"not_found"}`, false},
		{"an undeclared code", "GET", 404, nil, "application/problem+json", `{"code":"page.locked"}`, false},
		{"its JSON answer", "PUT", 201, nil, "application/json", `{"id":"x"}`, true},
		{"a JSON answer off its schema", "PUT", 201, nil, "application/json", `{"id":"x","extra":1}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v0/things/a/content", nil)
			header := http.Header{}
			maps.Copy(header, tt.header)
			if tt.contentType != "" {
				header.Set("Content-Type", tt.contentType)
			}
			if err := c.validateResponse(req, tt.status, header, []byte(tt.body)); (err == nil) != tt.valid {
				t.Errorf("validateResponse() = %v, want valid = %v", err, tt.valid)
			}
		})
	}
}

// generationConfig is the part of a module's oapi-codegen configuration
// that leaves operations out of the generated code.
type generationConfig struct {
	OutputOptions struct {
		ExcludeOperationIDs []string `yaml:"exclude-operation-ids"`
	} `yaml:"output-options"`
}

// generationViolation compares the operations a module's configuration
// leaves out of the generated code, excluded, with doc's raw operations:
// a raw operation that is generated would register a second route, or
// none that reads its bytes; an operation left out that is not raw would
// be served by no handler. It is "" when they are the same.
func generationViolation(doc *openapi3.T, config []byte) string {
	var cfg generationConfig
	if err := yaml.Unmarshal(config, &cfg); err != nil {
		return err.Error()
	}
	var rawIDs []string
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if raw(op) {
				rawIDs = append(rawIDs, op.OperationID)
			}
		}
	}
	slices.Sort(rawIDs)
	excluded := slices.Sorted(slices.Values(cfg.OutputOptions.ExcludeOperationIDs))
	if !slices.Equal(excluded, rawIDs) {
		return "output-options.exclude-operation-ids lists " + strings.Join(quoted(excluded), ", ") +
			"; want the raw operations, " + strings.Join(quoted(rawIDs), ", ")
	}
	return ""
}

func quoted(ids []string) []string {
	out := []string{"[]"}
	if len(ids) > 0 {
		out = nil
	}
	for _, id := range ids {
		out = append(out, `"`+id+`"`)
	}
	return out
}

// Each module's generated code leaves out exactly its raw operations
// (M7/P2 design 3.2): their handlers are the module's own. A module
// without a configuration, events, generates nothing.
func TestRawOperationsAreNotGenerated(t *testing.T) {
	names, err := moduleNames()
	if err != nil || len(names) == 0 {
		t.Fatalf("module files = %q, %v; want at least one", names, err)
	}
	checked := 0
	for _, name := range names {
		path := filepath.Join(apiDir(), "..", "server", "internal", "modules", name, "adapter", "http", "gen", "oapi-codegen.yaml")
		config, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		doc, err := loadModule(name)
		if err != nil {
			t.Fatal(err)
		}
		if v := generationViolation(doc, config); v != "" {
			t.Errorf("%s: %s", path, v)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no module has an oapi-codegen configuration")
	}
}

func TestGenerationViolation(t *testing.T) {
	doc := parse(t, rawContract)
	tests := []struct {
		name, config, want string
	}{
		{"the raw operations left out", "output-options:\n  exclude-operation-ids: [putThingContent, getThingContent]\n", ""},
		{"one generated", "output-options:\n  exclude-operation-ids: [getThingContent]\n",
			`output-options.exclude-operation-ids lists "getThingContent"; want the raw operations, "getThingContent", "putThingContent"`},
		{"none left out", "output-options:\n  name-normalizer: ToCamelCaseWithInitialisms\n",
			`output-options.exclude-operation-ids lists []; want the raw operations, "getThingContent", "putThingContent"`},
		{"one that is not raw left out", "output-options:\n  exclude-operation-ids: [getThingContent, listThings, putThingContent]\n",
			`output-options.exclude-operation-ids lists "getThingContent", "listThings", "putThingContent"; want the raw operations, "getThingContent", "putThingContent"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generationViolation(doc, []byte(tt.config)); got != tt.want {
				t.Errorf("generationViolation() = %q, want %q", got, tt.want)
			}
		})
	}
	notRaw := parse(t, strings.ReplaceAll(rawContract, "      x-raw: true\n", ""))
	if got := generationViolation(notRaw, []byte("package: gen\n")); got != "" {
		t.Errorf("generationViolation() of a module without raw operations = %q, want none", got)
	}
}
