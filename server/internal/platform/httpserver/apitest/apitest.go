// Package apitest checks HTTP responses against Nerve Wiki's OpenAPI
// contract, api/dist/openapi.yaml, with kin-openapi. Architecture rule 8 lets
// only tests import this package, which keeps it out of the nervewiki binary.
// That kin-openapi stays out of the binary by any other route too (generated
// code with an embedded spec, a validator middleware) is guarded separately,
// by archtest's check of the binary's transitive dependencies.
package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// Contract is the loaded and validated OpenAPI document.
type Contract struct {
	doc    *openapi3.T
	router routers.Router
}

// Load reads api/dist/openapi.yaml and fails t unless it is a valid OpenAPI
// document.
func Load(t testing.TB) *Contract {
	t.Helper()
	c, err := load(contractPath())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// CheckResponse fails t unless res, the response to req, is documented by
// req's operation: the status, the Content-Type, the body schema and, for a
// problem, a code the operation may answer (x-problem-codes, v0.1 design
// 6.1). The code is recorded for Main. res.Body stays readable for the
// caller. The 200 of a long-lived operation (x-long-lived) never ends: its
// status and Content-Type alone are checked, and its body is not read. A
// raw operation's (x-raw) bytes, documented as */* with a binary string,
// are taken as they come; an answer without a Content-Type fails wherever
// its response documents a body, though kin-openapi reads it as */*.
func (c *Contract) CheckResponse(t testing.TB, req *http.Request, res *http.Response) {
	t.Helper()
	if route, _, err := c.findRoute(req); err == nil && longLived(route.Operation) && res.StatusCode == http.StatusOK {
		if err := streamHead(route.Operation, res.Header); err != nil {
			t.Errorf("%s %s answered %d: %v", req.Method, req.URL.Path, res.StatusCode, err)
		}
		return
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	res.Body = io.NopCloser(bytes.NewReader(body))
	if err := c.validateResponse(req, res.StatusCode, res.Header, body); err != nil {
		t.Errorf("%s %s answered %d %s: %v", req.Method, req.URL.Path, res.StatusCode, body, err)
	}
}

// CheckRequest fails t unless req, a request a test is about to send, is
// documented: path, method, parameters and body. Security is not checked:
// tests send tokens the contract cannot judge. req.Body stays readable. A
// test that sends a request breaking the contract on purpose skips it.
func (c *Contract) CheckRequest(t testing.TB, req *http.Request) {
	t.Helper()
	if err := c.validateRequest(req); err != nil {
		t.Errorf("%s %s does not follow the contract: %v", req.Method, req.URL.Path, err)
	}
}

// CheckSchema fails t unless body is a JSON value valid against the schema
// components.schemas[name], e.g. "Problem".
func (c *Contract) CheckSchema(t testing.TB, name string, body []byte) {
	t.Helper()
	if err := c.validateSchema(name, body); err != nil {
		t.Errorf("%s: %v", body, err)
	}
}

// Enum returns the string enum of property in components.schemas[name], e.g.
// Enum(t, "FieldError", "code") for the field codes, and fails t unless the
// property has one.
func (c *Contract) Enum(t testing.TB, name, property string) []string {
	t.Helper()
	values, err := c.enum(name, property)
	if err != nil {
		t.Fatal(err)
	}
	return values
}

// apiDir locates the repository's api/ from this file, five directories below
// the repository root (tests run without -trimpath).
func apiDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "api")
}

// contractPath is api/dist/openapi.yaml, the bundled contract.
func contractPath() string {
	return filepath.Join(apiDir(), "dist", "openapi.yaml")
}

// newLoader returns the loader for contract documents. IncludeOrigin records
// the keywords each schema spells out: the authoring-rules test needs them to
// see `const: null` and `nullable: false`, which the decoded fields cannot
// tell from an absent keyword.
func newLoader() *openapi3.Loader {
	loader := openapi3.NewLoader()
	loader.IncludeOrigin = true
	return loader
}

func load(path string) (*Contract, error) {
	loader := newLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	// Route by path and method only: with servers, the router would also
	// match each request's scheme and host, which no test host satisfies.
	doc.Servers = nil
	for _, item := range doc.Paths.Map() {
		item.Servers = nil
		for _, op := range item.Operations() {
			op.Servers = nil
		}
	}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("route %s: %w", path, err)
	}
	return &Contract{doc: doc, router: router}, nil
}

// findRoute is the operation req is for, and the parameters of its path:
// found by the path as sent, escaped, so that a parameter's escaped '/'
// (%2F, a nested tag's) stays in its segment, as the server's router keeps
// it; each parameter unescaped.
func (c *Contract) findRoute(req *http.Request) (*routers.Route, map[string]string, error) {
	if req.URL.RawPath == "" {
		return c.router.FindRoute(req)
	}
	escaped := req.Clone(req.Context())
	escaped.URL.Path, escaped.URL.RawPath = req.URL.EscapedPath(), ""
	route, params, err := c.router.FindRoute(escaped)
	if err != nil {
		return nil, nil, err
	}
	for name, value := range params {
		if params[name], err = url.PathUnescape(value); err != nil {
			return nil, nil, err
		}
	}
	return route, params, nil
}

func (c *Contract) validateResponse(req *http.Request, status int, header http.Header, body []byte) error {
	route, pathParams, err := c.findRoute(req)
	if err != nil {
		return fmt.Errorf("no documented operation: %w", err)
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: pathParams, Route: route},
		Status:                 status,
		Header:                 header,
		Options:                &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	if header.Get("Content-Type") == "" && documentsBody(route.Operation, status) {
		return errors.New("no Content-Type, where the response documents a body")
	}
	in.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		return err
	}
	return c.checkProblemCode(route.Operation, header, body)
}

func (c *Contract) validateRequest(req *http.Request) error {
	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(req.Body); err != nil {
			return fmt.Errorf("read request body: %w", err)
		}
	}
	defer func() { req.Body = io.NopCloser(bytes.NewReader(body)) }()
	req.Body = io.NopCloser(bytes.NewReader(body))
	route, pathParams, err := c.findRoute(req)
	if err != nil {
		return fmt.Errorf("no documented operation: %w", err)
	}
	return openapi3filter.ValidateRequest(context.Background(), &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc, MultiError: true},
	})
}

func (c *Contract) validateSchema(name string, body []byte) error {
	schema, ok := c.doc.Components.Schemas[name]
	if !ok {
		return fmt.Errorf("no schema %q in the contract", name)
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return fmt.Errorf("not JSON: %w", err)
	}
	opts := []openapi3.SchemaValidationOption{openapi3.MultiErrors()}
	if c.doc.IsOpenAPI31OrLater() {
		opts = append(opts, openapi3.EnableJSONSchema2020())
	}
	return schema.Value.VisitJSON(value, opts...)
}

func (c *Contract) enum(name, property string) ([]string, error) {
	schema, ok := c.doc.Components.Schemas[name]
	if !ok {
		return nil, fmt.Errorf("no schema %q in the contract", name)
	}
	prop, ok := schema.Value.Properties[property]
	if !ok || len(prop.Value.Enum) == 0 {
		return nil, fmt.Errorf("%s.%s has no enum", name, property)
	}
	values := make([]string, len(prop.Value.Enum))
	for i, v := range prop.Value.Enum {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s.%s: enum value %v is not a string", name, property, v)
		}
		values[i] = s
	}
	return values, nil
}

// documentsBody reports whether op's response for status, or its default,
// documents a body.
func documentsBody(op *openapi3.Operation, status int) bool {
	res := op.Responses.Status(status)
	if res == nil {
		res = op.Responses.Default()
	}
	return res != nil && res.Value != nil && len(res.Value.Content) > 0
}

// streamHead checks the head of a long-lived operation's 200: a
// Content-Type the operation documents for it.
func streamHead(op *openapi3.Operation, header http.Header) error {
	ok := op.Responses.Status(http.StatusOK)
	if ok == nil || ok.Value == nil {
		return errors.New("no documented 200")
	}
	media, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil || ok.Value.Content.Get(media) == nil {
		return fmt.Errorf("Content-Type %q is not the 200's", header.Get("Content-Type"))
	}
	return nil
}
