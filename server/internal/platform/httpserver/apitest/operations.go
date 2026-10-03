package apitest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Operation is one operation of the contract, as the whole-program tests of
// bootstrap see it (M0/P4 design 3.6, M1/P1 design 3.9).
type Operation struct {
	ID     string   // operationId
	Tags   []string // the module the operation is of (the rules hold each to one tag)
	Method string   // upper case
	Path   string
	Public bool // security: [], needs no token
	// LongLived holds its response open (x-long-lived), such as the event
	// stream (M5 design 4.10): a test reads its head, not to the end.
	LongLived bool
	// ProblemHeaders are the headers its default response, the problem,
	// declares, sorted.
	ProblemHeaders []string
	params         openapi3.Parameters
	body           *openapi3.Schema
}

// Pattern is the route pattern the generated code registers, e.g.
// "POST /api/v0/auth/register".
func (o Operation) Pattern() string { return o.Method + " " + o.Path }

// HasJSONBody reports whether the operation takes an application/json body.
func (o Operation) HasJSONBody() bool { return o.body != nil }

// Target is the request target of an example call: the path with each path
// parameter, and each required query parameter, set to a valid value.
// Parameters bind before the middlewares, so a wrong one would answer 400
// before anything else runs.
func (o Operation) Target() string {
	return o.target("", "")
}

// target is Target with parameter name set to value, when name is not "".
func (o Operation) target(name, value string) string {
	path, query := o.Path, url.Values{}
	for _, ref := range o.params {
		p := ref.Value
		v, set := fmt.Sprint(validValue(p.Schema.Value)), p.Required
		if p.Name == name {
			v, set = value, true
		}
		switch {
		case p.In == openapi3.ParameterInPath:
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(v))
		case p.In == openapi3.ParameterInQuery && set:
			query.Set(p.Name, v)
		}
	}
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}

// ParamCase is a request target with one parameter that cannot bind, and
// the parameter the 400 must name.
type ParamCase struct {
	Name   string
	Target string
	Field  string
}

// ParamCases derives the cases of the parameter binding whole-program test
// (M1/P1 design 3.9): for each path or query parameter whose Go type rejects
// some strings, a number, a boolean, or a string whose format is generated
// as a Go type, the example target with that parameter wrong. Other strings,
// enums too, bind whatever they are.
func (o Operation) ParamCases() []ParamCase {
	var cases []ParamCase
	for _, ref := range o.params {
		p := ref.Value
		if p.In != openapi3.ParameterInPath && p.In != openapi3.ParameterInQuery {
			continue
		}
		var wrong string
		switch s := p.Schema.Value; {
		case s.Type.Includes("integer"), s.Type.Includes("number"):
			wrong = "not-a-number"
		case s.Type.Includes("boolean"):
			wrong = "not-a-boolean"
		case checkedFormat(s):
			wrong = "not-a-" + s.Format
		default:
			continue
		}
		cases = append(cases, ParamCase{Name: "wrong " + p.Name, Target: o.target(p.Name, wrong), Field: p.Name})
	}
	return cases
}

// TextCase is a request target with one free-text parameter set to what a
// database text cannot hold.
type TextCase struct {
	Name   string
	Target string
}

// TextCases derives the cases of the free-text whole-program test (M2/P1
// review): for each path or query parameter that binds any string, one
// without a checked format (enums bind any string too), the example target
// with it set to a NUL, then to a byte that is not UTF-8. Such a parameter
// reaches the operation as it came.
func (o Operation) TextCases() []TextCase {
	var cases []TextCase
	for _, ref := range o.params {
		p := ref.Value
		s := p.Schema.Value
		if (p.In != openapi3.ParameterInPath && p.In != openapi3.ParameterInQuery) || !s.Type.Includes("string") || checkedFormat(s) {
			continue
		}
		cases = append(cases,
			TextCase{Name: "NUL in " + p.Name, Target: o.target(p.Name, "\x00")},
			TextCase{Name: "not UTF-8 in " + p.Name, Target: o.target(p.Name, "\xff")})
	}
	return cases
}

// ExampleBody is a body the operation's structure accepts, or nil when it
// takes none.
func (o Operation) ExampleBody() []byte {
	if o.body == nil {
		return nil
	}
	out, _ := json.Marshal(validValue(o.body))
	return out
}

// Operations lists every operation of the contract, sorted by pattern.
func (c *Contract) Operations() []Operation {
	var ops []Operation
	for path, item := range c.doc.Paths.Map() {
		for method, op := range item.Operations() {
			o := Operation{ID: op.OperationID, Tags: op.Tags, Method: strings.ToUpper(method), Path: path, Public: !needsToken(op),
				LongLived: longLived(op), params: slices.Concat(item.Parameters, op.Parameters)}
			if rb := op.RequestBody; rb != nil && rb.Value != nil {
				if media := rb.Value.Content.Get("application/json"); media != nil && media.Schema != nil {
					o.body = media.Schema.Value
				}
			}
			if problem := op.Responses.Default(); problem != nil && problem.Value != nil {
				o.ProblemHeaders = slices.Sorted(maps.Keys(problem.Value.Headers))
			}
			ops = append(ops, o)
		}
	}
	slices.SortFunc(ops, func(a, b Operation) int { return strings.Compare(a.Pattern(), b.Pattern()) })
	return ops
}

// FieldProblem is one entry of a problem's errors: field and code.
type FieldProblem struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// BodyCase is a request body that breaks the operation's structure in one
// way, and what it must get: 400 bad_request with exactly Fields, in order;
// or, when Accepted, anything but 400.
type BodyCase struct {
	Name     string
	Body     []byte
	Fields   []FieldProblem
	Accepted bool
}

// unknownField is the property no schema declares.
const unknownField = "nwiki_undeclared"

// rawMark stands for a value that json.Marshal cannot write, until BodyCases
// puts the JSON text in its place.
const rawMark = "nwiki_raw_value"

// BodyCases derives the cases of the body whole-program test (M1/P1 design
// 3.9) from the operation's body schema: a valid body, broken one way at a time.
// Cases whose kind of field the schema lacks are left out.
//
//  1. an undeclared top-level property;
//  2. an undeclared property in a nested object;
//  3. null for an optional property that is not nullable;
//  4. each required property missing;
//  5. null for a nullable property: not 400;
//  6. each format property with a wrong string;
//  7. the first property twice: duplicate (a body read two ways);
//  8. the first property of the nested object twice, in it;
//  9. bytes that are not UTF-8 in the first string property: invalid_format;
//  10. every kind of 1–4 and 6 that the schema has, each on a property of its
//     own, together: every problem in one answer. Left out when the schema
//     has only the first kind. 7–9 are not in it: a body read two ways gets
//     only those answers, before its structure is checked.
func (o Operation) BodyCases() []BodyCase {
	s := o.body
	valid := validValue(s).(map[string]any)
	with := func(change func(body map[string]any)) []byte {
		body := maps.Clone(valid)
		change(body)
		out, _ := json.Marshal(body)
		return out
	}
	names := slices.Sorted(maps.Keys(s.Properties))
	var nested string
	var optional, required, formatted []string
	for _, name := range names {
		p := s.Properties[name].Value
		if nested == "" && isObject(p) {
			nested = name
		}
		if !isNullable(p) && !slices.Contains(s.Required, name) {
			optional = append(optional, name)
		}
		if checkedFormat(p) {
			formatted = append(formatted, name)
		}
	}
	required = slices.Sorted(slices.Values(s.Required))
	undeclaredIn := func(name string) map[string]any {
		v := validValue(s.Properties[name].Value).(map[string]any)
		v[unknownField] = 1
		return v
	}
	wrong := func(name string) string { return "not-a-" + s.Properties[name].Value.Format }
	// raw is the valid body with name's value written as text, which json.Marshal would not write: a
	// second member of the same name, bytes that are not UTF-8. The mark must be the body's only one, or
	// the text would land elsewhere, or nowhere, and the case would test something else.
	raw := func(name, text string) []byte {
		body := maps.Clone(valid)
		body[name] = rawMark
		out, _ := json.Marshal(body)
		mark := []byte(`"` + rawMark + `"`)
		if n := bytes.Count(out, mark); n != 1 {
			panic(fmt.Sprintf("apitest: the body %s has the raw mark %d times, want once", out, n))
		}
		return bytes.Replace(out, mark, []byte(text), 1)
	}
	// twice is the text of name's value v, then of a second member name: v.
	twice := func(name string, v any) string {
		value, _ := json.Marshal(v)
		return string(value) + `,"` + name + `":` + string(value)
	}
	// valueOf is name's value in values, else a valid one for its schema.
	valueOf := func(schema *openapi3.Schema, name string, values map[string]any) any {
		if v, ok := values[name]; ok {
			return v
		}
		return validValue(schema.Properties[name].Value)
	}

	cases := []BodyCase{{Name: "undeclared property", Body: with(func(b map[string]any) { b[unknownField] = 1 }),
		Fields: []FieldProblem{{unknownField, "not_allowed"}}}}
	if nested != "" {
		cases = append(cases, BodyCase{Name: "undeclared property in " + nested, Body: with(func(b map[string]any) { b[nested] = undeclaredIn(nested) }),
			Fields: []FieldProblem{{nested + "." + unknownField, "not_allowed"}}})
	}
	for _, name := range names {
		switch {
		case isNullable(s.Properties[name].Value):
			cases = append(cases, BodyCase{Name: "null for nullable " + name, Body: with(func(b map[string]any) { b[name] = nil }), Accepted: true})
		case slices.Contains(optional, name):
			cases = append(cases, BodyCase{Name: "null for optional " + name, Body: with(func(b map[string]any) { b[name] = nil }),
				Fields: []FieldProblem{{name, "invalid_format"}}})
		}
	}
	for _, name := range required {
		cases = append(cases, BodyCase{Name: "missing " + name, Body: with(func(b map[string]any) { delete(b, name) }),
			Fields: []FieldProblem{{name, "required"}}})
	}
	for _, name := range formatted {
		cases = append(cases, BodyCase{Name: "wrong " + s.Properties[name].Value.Format + " in " + name,
			Body: with(func(b map[string]any) { b[name] = wrong(name) }), Fields: []FieldProblem{{name, "invalid_format"}}})
	}
	if len(names) > 0 {
		name := names[0]
		cases = append(cases, BodyCase{Name: name + " twice", Body: raw(name, twice(name, valueOf(s, name, valid))),
			Fields: []FieldProblem{{name, "duplicate"}}})
	}
	if nested != "" {
		n := s.Properties[nested].Value
		if inner := slices.Sorted(maps.Keys(n.Properties)); len(inner) > 0 {
			text := `{"` + inner[0] + `":` + twice(inner[0], valueOf(n, inner[0], nil)) + `}`
			cases = append(cases, BodyCase{Name: nested + "." + inner[0] + " twice", Body: raw(nested, text),
				Fields: []FieldProblem{{nested + "." + inner[0], "duplicate"}}})
		}
	}
	if i := slices.IndexFunc(names, func(name string) bool { return s.Properties[name].Value.Type.Includes("string") }); i >= 0 {
		cases = append(cases, BodyCase{Name: "not UTF-8 in " + names[i], Body: raw(names[i], "\"\xff\""),
			Fields: []FieldProblem{{names[i], "invalid_format"}}})
	}

	// Case 10: each kind takes the first property no earlier kind took.
	body := maps.Clone(valid)
	body[unknownField] = 1
	all := []FieldProblem{{unknownField, "not_allowed"}}
	used := map[string]bool{}
	first := func(names []string) string {
		for _, name := range names {
			if !used[name] {
				used[name] = true
				return name
			}
		}
		return ""
	}
	if name := first([]string{nested}); name != "" {
		body[name] = undeclaredIn(name)
		all = append(all, FieldProblem{name + "." + unknownField, "not_allowed"})
	}
	if name := first(optional); name != "" {
		body[name] = nil
		all = append(all, FieldProblem{name, "invalid_format"})
	}
	if name := first(required); name != "" {
		delete(body, name)
		all = append(all, FieldProblem{name, "required"})
	}
	if name := first(formatted); name != "" {
		body[name] = wrong(name)
		all = append(all, FieldProblem{name, "invalid_format"})
	}
	if len(all) >= 2 {
		slices.SortFunc(all, func(a, b FieldProblem) int { return strings.Compare(a.Field, b.Field) })
		out, _ := json.Marshal(body)
		cases = append(cases, BodyCase{Name: "every problem at once", Body: out, Fields: all})
	}
	return cases
}

// validValue is a value that s's structure accepts: every required
// property, the first enum value, a valid string for each checked format.
func validValue(s *openapi3.Schema) any {
	if len(s.AnyOf) > 0 {
		for _, alt := range s.AnyOf {
			if !alt.Value.Type.Is("null") {
				return validValue(alt.Value)
			}
		}
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	switch {
	case isObject(s):
		obj := map[string]any{}
		for _, name := range s.Required {
			obj[name] = validValue(s.Properties[name].Value)
		}
		return obj
	case s.Type.Includes("array"):
		return []any{}
	case s.Type.Includes("integer"), s.Type.Includes("number"):
		return 1
	case s.Type.Includes("boolean"):
		return false
	}
	switch s.Format {
	case "date-time":
		return "2026-01-01T00:00:00Z"
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "email":
		return "someone@example.com"
	}
	return "x"
}

func isObject(s *openapi3.Schema) bool {
	return s.Type.Includes("object") && len(s.AnyOf) == 0
}

func isNullable(s *openapi3.Schema) bool {
	if s.Type.IncludesNull() {
		return true
	}
	for _, alt := range s.AnyOf {
		if alt.Value.Type.Is("null") {
			return true
		}
	}
	return false
}

// checkedFormat reports a string format that bodyshape checks: those the
// module template generates as Go types (M0/P4 design 3.3).
func checkedFormat(s *openapi3.Schema) bool {
	return s.Type.Includes("string") && (s.Format == "date-time" || s.Format == "uuid")
}
