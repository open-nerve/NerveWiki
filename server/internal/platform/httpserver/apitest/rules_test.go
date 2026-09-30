package apitest

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// platformCodes are the codes without a module prefix: those httpserver
// writes itself and those of internal/shared's constructors.
func platformCodes() []string {
	return []string{
		"bad_request", "unauthorized", "forbidden", "not_found", "payload_too_large", "validation_failed",
		"rate_limited", "internal_error", "not_ready", "server_busy",
	}
}

// moduleNames lists the modules that have an api/modules/<m>.yaml.
func moduleNames() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(apiDir(), "modules", "*.yaml"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range files {
		names = append(names, strings.TrimSuffix(filepath.Base(f), ".yaml"))
	}
	return names, nil
}

// TestContractFollowsAuthoringRules holds api/dist/openapi.yaml to the
// authoring rules of M0/P4 design 3.2. Generated Go code and the TS client
// rely on them, but neither the bundler nor doc.Validate enforces them.
func TestContractFollowsAuthoringRules(t *testing.T) {
	c := Load(t)
	if c.doc.Paths.Len() == 0 {
		t.Fatal("the contract has no paths")
	}
	modules, err := moduleNames()
	if err != nil || len(modules) == 0 {
		t.Fatalf("module files = %q, %v; want at least one", modules, err)
	}
	for _, v := range authoringViolations(c.doc, modules) {
		t.Error(v)
	}
}

// pathOwners maps every path of api/modules/*.yaml to its module.
func pathOwners(t *testing.T) map[string]string {
	t.Helper()
	names, err := moduleNames()
	if err != nil || len(names) == 0 {
		t.Fatalf("module files = %q, %v; want at least one", names, err)
	}
	owners := map[string]string{}
	for _, name := range names {
		doc, err := loadModule(name)
		if err != nil {
			t.Fatal(err)
		}
		for path := range doc.Paths.Map() {
			owners[path] = name
		}
	}
	return owners
}

// Go serves every path of api/modules/*.yaml, but the TS client and the
// contract tests know only the paths that the root api/openapi.yaml lists.
func TestRootListsEveryModulePath(t *testing.T) {
	c := Load(t)
	owners := pathOwners(t)
	for _, path := range slices.Sorted(maps.Keys(owners)) {
		if c.doc.Paths.Value(path) == nil {
			t.Errorf("api/modules/%s.yaml declares %s, which api/dist/openapi.yaml lacks: list it in api/openapi.yaml and run make gen",
				owners[path], path)
		}
	}
}

// Tags group the operations by module in the bundled contract and its
// documentation, so every operation of api/modules/<m>.yaml carries exactly
// the tag <m> (M0/P4 design 3.2).
func TestEveryOperationIsTaggedWithItsModule(t *testing.T) {
	names, err := moduleNames()
	if err != nil || len(names) == 0 {
		t.Fatalf("module files = %q, %v; want at least one", names, err)
	}
	for _, name := range names {
		doc, err := loadModule(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range slices.Sorted(maps.Keys(doc.Paths.Map())) {
			ops := doc.Paths.Value(path).Operations()
			for _, method := range slices.Sorted(maps.Keys(ops)) {
				if tags := ops[method].Tags; !slices.Equal(tags, []string{name}) {
					t.Errorf("%s %s in api/modules/%s.yaml: tags = %q, want [%q]", method, path, name, tags, name)
				}
			}
		}
	}
}

// authoringViolations reports where doc breaks the authoring rules of M0/P4
// design 3.2; modules are Nerve Wiki's modules, those with a file in
// api/modules/. It takes the document so that each check is proven on
// hand-built bad documents (rules_cases_test.go) as well as applied to the
// real contract.
func authoringViolations(doc *openapi3.T, modules []string) []string {
	r := &ruleCheck{
		doc: doc, modules: modules,
		lowerCamel: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`),
		// A platform code, or a module's code prefixed with a module, e.g.
		// page.locked.
		codePattern: regexp.MustCompile(`^([a-z]+\.)?[a-z_]+$`),
	}
	r.topCodes()
	for _, path := range slices.Sorted(maps.Keys(doc.Paths.Map())) {
		r.path(path, doc.Paths.Value(path))
	}
	if doc.Components != nil {
		r.components(doc.Components)
	}
	return r.found
}

// ruleCheck walks one document and collects its violations: path checks the
// /api/v0/ prefix, operation the operationId, tags, default problem,
// security and problem codes, and schema the keywords that oapi-codegen
// mistranslates and, through closedObject, additionalProperties.
type ruleCheck struct {
	doc         *openapi3.T
	modules     []string
	lowerCamel  *regexp.Regexp
	codePattern *regexp.Regexp
	found       []string
}

func (r *ruleCheck) report(where, format string, args ...any) {
	r.found = append(r.found, where+": "+fmt.Sprintf(format, args...))
}

func (r *ruleCheck) path(path string, item *openapi3.PathItem) {
	if !strings.HasPrefix(path, "/api/v0/") {
		r.report(path, "does not start with /api/v0/")
	}
	for _, p := range item.Parameters {
		r.parameter(path+" parameters/"+p.Value.Name, p)
	}
	ops := item.Operations()
	for _, method := range slices.Sorted(maps.Keys(ops)) {
		r.operation(method+" "+path, ops[method])
	}
}

// topCodes checks the codes every operation can answer: only platform
// codes, listed once at the top level.
func (r *ruleCheck) topCodes() {
	codes, present, err := problemCodes(r.doc.Extensions)
	switch {
	case err != nil:
		r.report("top level", "%v", err)
	case !present:
		r.report("top level", "has no %s", problemCodesKey)
	}
	for _, code := range codes {
		if !slices.Contains(platformCodes(), code) {
			r.report("top level", "%s holds %q, which is not a platform code", problemCodesKey, code)
		}
	}
}

// security: every operation declares its own security, [] for a public one
// (M0/P4 design 3.2): the bundler drops a module file's top-level security,
// and doc.Validate accepts a reference to a scheme the bundle lacks.
func (r *ruleCheck) security(where string, op *openapi3.Operation) {
	if op.Security == nil {
		r.report(where, "declares no security; write [{bearer: []}], or [] for a public operation")
		return
	}
	for _, req := range *op.Security {
		for _, name := range slices.Sorted(maps.Keys(req)) {
			if r.doc.Components == nil || r.doc.Components.SecuritySchemes[name] == nil {
				r.report(where, "security scheme %q is not declared in components.securitySchemes", name)
			}
		}
	}
}

// problemCodes: every operation lists the codes it can answer beyond the
// top-level ones (v0.1 design 6.1). A module's code is prefixed with the
// module that refuses, which need not be the file that declares the path: a
// page operation may answer space.not_found. A code without prefix is a
// platform code.
func (r *ruleCheck) problemCodes(where string, op *openapi3.Operation) {
	codes, present, err := problemCodes(op.Extensions)
	switch {
	case err != nil:
		r.report(where, "%v", err)
	case !present:
		r.report(where, "has no %s; write [] when it answers only the top-level codes", problemCodesKey)
	}
	for _, code := range codes {
		prefix, _, prefixed := strings.Cut(code, ".")
		switch {
		case !r.codePattern.MatchString(code):
			r.report(where, "problem code %q is not spelled [module.]lower_snake", code)
		case prefixed && !slices.Contains(r.modules, prefix):
			r.report(where, "problem code %q is prefixed with %q, which is not a module: want one of %q", code, prefix, r.modules)
		case !prefixed && !slices.Contains(platformCodes(), code):
			r.report(where, "problem code %q has no module prefix and is not a platform code", code)
		}
	}
}

func (r *ruleCheck) operation(where string, op *openapi3.Operation) {
	r.security(where, op)
	r.problemCodes(where, op)
	if !r.lowerCamel.MatchString(op.OperationID) {
		r.report(where, "operationId %q is not lower camelCase", op.OperationID)
	}
	if len(op.Tags) == 0 {
		r.report(where, "has no tag")
	}
	for _, tag := range op.Tags {
		if r.doc.Tags.Get(tag) == nil {
			r.report(where, "tag %q is not declared in the top-level tags", tag)
		}
	}
	if !r.defaultIsProblem(op) {
		r.report(where, "has no default response whose application/problem+json schema is the Problem component")
	}
	for _, p := range op.Parameters {
		r.parameter(where+" parameters/"+p.Value.Name, p)
	}
	if body := op.RequestBody; body != nil && body.Ref == "" {
		r.content(where+" requestBody", body.Value.Content)
	}
	responses := op.Responses.Map()
	for _, status := range slices.Sorted(maps.Keys(responses)) {
		r.response(where+" responses/"+status, responses[status])
	}
}

// defaultIsProblem compares by schema identity: the bundler turns every
// reference into a local $ref, and the loader resolves each one to the
// component's own *Schema.
func (r *ruleCheck) defaultIsProblem(op *openapi3.Operation) bool {
	res := op.Responses.Default()
	if r.doc.Components == nil || res == nil || res.Value == nil {
		return false
	}
	problem := r.doc.Components.Schemas["Problem"]
	media := res.Value.Content["application/problem+json"]
	return problem != nil && media != nil && media.Schema != nil && media.Schema.Value == problem.Value
}

func (r *ruleCheck) components(c *openapi3.Components) {
	for _, name := range slices.Sorted(maps.Keys(c.Schemas)) {
		r.schema("components/schemas/"+name, c.Schemas[name])
	}
	for _, name := range slices.Sorted(maps.Keys(c.Parameters)) {
		r.parameter("components/parameters/"+name, c.Parameters[name])
	}
	for _, name := range slices.Sorted(maps.Keys(c.Headers)) {
		if h := c.Headers[name]; h.Ref == "" {
			r.schema("components/headers/"+name, h.Value.Schema)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(c.RequestBodies)) {
		if body := c.RequestBodies[name]; body.Ref == "" {
			r.content("components/requestBodies/"+name, body.Value.Content)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(c.Responses)) {
		r.response("components/responses/"+name, c.Responses[name])
	}
}

// closedObject lets contract tests catch undocumented fields: every object
// schema, a component or an inline one, must set additionalProperties: false.
// The rule targets response objects and is applied to request schemas as
// well, where it keeps the decoder's case-insensitive match of field names
// out of reach: bodyshape rejects a property the schema does not declare.
// Exempt are
// composition wrappers (allOf, oneOf or anyOf without properties of their
// own): additionalProperties does not see the subschemas' properties, so
// closing a wrapper would reject every instance.
func (r *ruleCheck) closedObject(where string, ref *openapi3.SchemaRef) {
	s := ref.Value
	if ref.Ref != "" || !s.Type.Includes("object") {
		return
	}
	if len(s.Properties) == 0 && len(s.AllOf)+len(s.OneOf)+len(s.AnyOf) > 0 {
		return
	}
	if closed := s.AdditionalProperties.Has; closed == nil || *closed {
		r.report(where, "object schema does not set additionalProperties: false")
	}
}

func (r *ruleCheck) parameter(where string, p *openapi3.ParameterRef) {
	if p.Ref != "" {
		return
	}
	r.schema(where, p.Value.Schema)
	r.content(where, p.Value.Content)
}

func (r *ruleCheck) response(where string, res *openapi3.ResponseRef) {
	if res.Ref != "" {
		return
	}
	for _, name := range slices.Sorted(maps.Keys(res.Value.Headers)) {
		if h := res.Value.Headers[name]; h.Ref == "" {
			r.schema(where+" headers/"+name, h.Value.Schema)
		}
	}
	r.content(where, res.Value.Content)
}

func (r *ruleCheck) content(where string, content openapi3.Content) {
	for _, media := range slices.Sorted(maps.Keys(content)) {
		r.schema(where+" "+media, content[media].Schema)
	}
}

// schema checks that an object is closed and the keywords that oapi-codegen
// mistranslates, in the schema and, recursively, its inline subschemas; a
// $ref is checked where its target is defined. kin-openapi decodes
// `nullable: false` and `const: null` to zero values that look like an
// absent keyword, so the keys the loader recorded per schema (Origin) are
// checked as well as the decoded fields. Walking the
// raw YAML instead would need a direct YAML dependency and would have to
// tell a schema's keywords from, say, a property named const.
func (r *ruleCheck) schema(where string, ref *openapi3.SchemaRef) {
	if ref == nil || ref.Ref != "" || ref.Value == nil {
		return
	}
	r.closedObject(where, ref)
	s := ref.Value
	if s.Nullable || spells(s, "nullable") {
		r.report(where, "uses nullable, the OpenAPI 3.0 keyword; write type: [T, 'null']")
	}
	if s.Const != nil || spells(s, "const") {
		r.report(where, "uses const, which oapi-codegen turns into interface{}; write a single-value enum")
	}
	if slices.Contains(s.Enum, nil) {
		r.report(where, `has null in enum, which adds a "<nil>" Go constant; write oneOf: [{$ref: …}, {type: 'null'}]`)
	}
	for _, name := range slices.Sorted(maps.Keys(s.Properties)) {
		r.schema(where+"/properties/"+name, s.Properties[name])
	}
	r.schema(where+"/items", s.Items)
	r.schema(where+"/additionalProperties", s.AdditionalProperties.Schema)
	r.schema(where+"/not", s.Not)
	for _, list := range []struct {
		keyword string
		refs    openapi3.SchemaRefs
	}{{"allOf", s.AllOf}, {"oneOf", s.OneOf}, {"anyOf", s.AnyOf}} {
		for i, sub := range list.refs {
			r.schema(fmt.Sprintf("%s/%s/%d", where, list.keyword, i), sub)
		}
	}
}

// spells reports whether the source of s wrote keyword, whatever its value.
func spells(s *openapi3.Schema, keyword string) bool {
	if s.Origin == nil {
		return false
	}
	_, ok := s.Origin.Fields.Lookup(keyword)
	return ok
}
