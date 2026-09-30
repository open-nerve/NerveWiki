// Package bodyshape checks the structure of JSON request bodies at the API
// boundary, before the generated strict handler decodes them: first that the
// body can be read one way only (no member name twice in an object, no
// string that is not valid Unicode), then JSON types, undeclared properties,
// null where the contract does not allow it, missing required properties,
// and the string formats that the generated code decodes into Go types. The
// problems of a kind are collected in one pass, each once, at most
// maxProblems of them, at paths of at most maxPath bytes. Values (lengths,
// enums, ranges, e-mail syntax) are the domain's.
//
// The tables are generated per module from the API description by
// server/tools/bodyshapegen, next to the module's server.gen.go. The package
// uses only the standard library.
package bodyshape

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"
	"uuid"
)

// Type is a set of JSON types.
type Type uint8

// The JSON types. Integer is a number literal that an int64 can hold; it also
// counts as a Number, a literal that a float64 can hold. These are the Go
// types bodyshapegen lets the generated code decode numbers into, so the
// check covers their whole range.
const (
	Null Type = 1 << iota
	Boolean
	Integer
	Number
	String
	Array
	Object
)

// Any accepts every JSON type: a schema without `type`.
const Any Type = 0

// Format is a string format that the generated code decodes into a Go type,
// named after that type. A wrong value cannot be held by the generated type,
// so it is checked before decoding.
type Format uint8

// The formats with a checker.
const (
	FormatNone Format = iota
	FormatTime        // time.Time (date-time)
	FormatUUID        // the standard library's uuid.UUID (uuid)
)

// Node.Extra and Node.Items hold a node index or one of these.
const (
	// Closed rejects undeclared properties: additionalProperties: false.
	Closed = -1
	// Open does not check them, or the items of an array without an item
	// schema: additionalProperties: true, or no schema.
	Open = -2
)

// Node is one schema of a request body. Child schemas are indexes into
// Table.Nodes, so recursive schemas are fine.
type Node struct {
	Types    Type
	Format   Format
	Props    map[string]int // object: declared property → node
	Required []string       // object: required properties
	Extra    int            // object: node of undeclared properties, Closed or Open
	Items    int            // array: node of the items, or Open
}

// Table holds the request-body schemas of one module.
type Table struct {
	Nodes []Node
	Roots map[string]int // route pattern, e.g. "POST /api/v0/auth/register" → body node
}

// Field codes, the subset of the closed set of api/common.yaml FieldError
// that the structure check produces.
const (
	codeRequired      = "required"
	codeInvalidFormat = "invalid_format"
	codeNotAllowed    = "not_allowed"
	codeDuplicate     = "duplicate"
)

// FieldError is one structural problem. Field is the JSON path, e.g.
// tags[1].name; the empty path is the body itself. It satisfies the field
// interface of httpserver.ProblemError by structure.
type FieldError struct {
	Field string
	Code  string
}

func (f FieldError) Error() string {
	switch f.Code {
	case codeRequired:
		return "is required"
	case codeNotAllowed:
		return "is not a property of this request"
	case codeDuplicate:
		return "appears more than once in its object"
	default:
		return "has the wrong type or format"
	}
}

// ProblemField returns the JSON path.
func (f FieldError) ProblemField() string { return f.Field }

// ProblemCode returns the field code.
func (f FieldError) ProblemCode() string { return f.Code }

// Error lists every structural problem of one request body. It satisfies
// httpserver.ProblemError by structure: 400 bad_request with the fields.
type Error struct {
	Fields []FieldError
}

func (e *Error) Error() string { return "The request body does not match the API description." }

// ProblemStatus is 400: the request breaks the contract's structure.
func (e *Error) ProblemStatus() int { return 400 }

// ProblemCode is the platform's bad_request.
func (e *Error) ProblemCode() string { return "bad_request" }

// ProblemFields returns the field errors.
func (e *Error) ProblemFields() []error {
	errs := make([]error, len(e.Fields))
	for i, f := range e.Fields {
		errs[i] = f
	}
	return errs
}

// jsonSpace is JSON's insignificant whitespace (RFC 8259 §2): space, tab, CR
// and LF. Other spaces, such as a form feed or U+00A0, are not JSON: the
// decoder rejects a body that starts with one, and so does this package.
const jsonSpace = " \t\r\n"

// ErrNotJSON is Check's answer for a body that is not one valid JSON document.
var ErrNotJSON = errors.New("the request body is not valid JSON")

// Check checks body, a whole request body, against the root of pattern. It
// returns nil when there is nothing to check: a pattern without a root has no
// JSON body, and an empty body, or one of only JSON whitespace, is left to the
// generated decoder. A body that is not one valid JSON document is ErrNotJSON.
// One that can be read two ways (ambiguity.go) is an *Error with its
// ambiguities: its structure is checked once it has one reading. One that
// breaks the structure is an *Error with its problems.
//
// Either lists each problem once, at a path of at most maxPath bytes, and at
// most maxProblems of them: the first ones that a reading in a fixed order
// meets. The scan reads the body in document order; the walk reads an
// object's members by name, depth first, then its missing required members.
// The problems are then sorted by path. So the same body always gets the same
// problems, but not necessarily those with the smallest paths: 16 undeclared
// members can crowd out a missing email.
func (t *Table) Check(pattern string, body []byte) error {
	root, ok := t.Roots[pattern]
	// walk takes the exact bytes of one value; the whitespace around the
	// document's root value is not part of it.
	value := bytes.Trim(body, jsonSpace)
	switch {
	case !ok, len(value) == 0:
		return nil
	case !json.Valid(value):
		return ErrNotJSON
	}
	var p problems
	ambiguities(value, &p)
	if len(p.errs) == 0 {
		t.walk(root, value, &p)
	}
	if len(p.errs) == 0 {
		return nil
	}
	slices.SortFunc(p.errs, func(a, b FieldError) int {
		return cmp.Or(cmp.Compare(a.Field, b.Field), cmp.Compare(a.Code, b.Code))
	})
	return &Error{Fields: p.errs}
}

// maxProblems bounds the problems of one body. A body can have a problem
// every few bytes: listing them all could cost the square of the body.
const maxProblems = 16

// maxPath bounds a reported path, in bytes. A name is the client's and can
// be as long as the body, and every problem under it repeats it: a path
// longer than this is cut short and ends with an ellipsis, so the paths of
// one answer are at most maxProblems × maxPath bytes, whatever the body. The
// contract's paths are tens of bytes.
const maxPath = 256

// ellipsis ends a path that was cut short.
const ellipsis = "…"

// problems collects the problems of one body while it is read. It keeps the
// path of the value being read as text, cut after maxPath+1 bytes, and where
// each segment began. Writing a path for every value would cost the square of
// the depth, and the whole path for every problem the problems times the
// name. This way a report copies at most maxPath bytes and compares them with
// the paths already reported, at most maxProblems of them, however long or
// deep the path, so a body can repeat one problem as often as it likes.
type problems struct {
	path  []byte // names joined by dots, indexes in brackets, e.g. tags[1].name
	marks []int  // len(path) before each segment, for leave
	shown []byte // at's buffer for a path cut short
	errs  []FieldError
}

// enter adds a member's name to the path.
func (p *problems) enter(name string) {
	p.marks = append(p.marks, len(p.path))
	if len(p.path) > 0 {
		p.path = append(p.path, '.')
	}
	p.path = append(p.path, name[:min(len(name), maxPath+1)]...) // no more than clip keeps
	p.clip()
}

// enterItem adds an item's index to the path.
func (p *problems) enterItem(i int) {
	p.marks = append(p.marks, len(p.path))
	p.path = append(strconv.AppendInt(append(p.path, '['), int64(i), 10), ']')
	p.clip()
}

// leave takes the last segment off the path.
func (p *problems) leave() {
	p.path = p.path[:p.marks[len(p.marks)-1]]
	p.marks = p.marks[:len(p.marks)-1]
}

// clip keeps the first maxPath+1 bytes of the path: one more than a reported
// path has, which tells at that the path is longer.
func (p *problems) clip() { p.path = p.path[:min(len(p.path), maxPath+1)] }

// full reports whether maxProblems have been reported: the rest is not read.
func (p *problems) full() bool { return len(p.errs) >= maxProblems }

// report adds a problem at the current path, unless p is full or has it
// already. A problem is reported once: a name repeated in its object can
// break the same rule each time, and two long paths can be cut short to the
// same text, which a client could not tell apart anyway. A repeat does not
// count against maxProblems.
func (p *problems) report(code string) {
	if p.full() {
		return
	}
	path := p.at()
	for _, f := range p.errs {
		if f.Field == string(path) && f.Code == code {
			return
		}
	}
	p.errs = append(p.errs, FieldError{string(path), code})
}

// at returns the current path as it is reported; the body itself is the
// empty path. A path longer than maxPath is cut short: its first bytes, cut
// at a rune boundary, then an ellipsis, maxPath bytes in all at most. The
// path's names are decoded, so it is UTF-8 up to the cut. The result is valid
// until the path changes.
func (p *problems) at() []byte {
	if len(p.path) <= maxPath {
		return p.path
	}
	cut := maxPath - len(ellipsis)
	for !utf8.RuneStart(p.path[cut]) {
		cut--
	}
	p.shown = append(append(p.shown[:0], p.path[:cut]...), ellipsis...)
	return p.shown
}

// walk checks raw, the exact bytes of one JSON value, against node i. It
// goes only where the schema goes: declared properties, the values of an
// open map, the items of an array with an item schema. It reads the members
// of an object in name order, so a body with more than maxProblems problems
// gets the same ones every time.
func (t *Table) walk(i int, raw []byte, p *problems) {
	if p.full() {
		return
	}
	n := t.Nodes[i]
	kind := kindOf(raw)
	if n.Types != Any && kind&n.Types == 0 {
		p.report(codeInvalidFormat)
		return
	}
	switch kind {
	case String:
		if n.Format != FormatNone && checkFormat(n.Format, raw) != nil {
			p.report(codeInvalidFormat)
		}
	case Object:
		var props map[string]json.RawMessage
		_ = json.Unmarshal(raw, &props) // the whole body was valid JSON
		for _, name := range slices.Sorted(maps.Keys(props)) {
			p.enter(name)
			child, declared := n.Props[name]
			switch {
			case declared:
				t.walk(child, props[name], p)
			case n.Extra == Closed:
				p.report(codeNotAllowed)
			case n.Extra >= 0:
				t.walk(n.Extra, props[name], p)
			}
			p.leave()
		}
		for _, name := range n.Required {
			if _, ok := props[name]; !ok {
				p.enter(name)
				p.report(codeRequired)
				p.leave()
			}
		}
	case Array:
		if n.Items < 0 {
			return
		}
		var items []json.RawMessage
		_ = json.Unmarshal(raw, &items)
		for j, item := range items {
			p.enterItem(j)
			t.walk(n.Items, item, p)
			p.leave()
		}
	}
}

// kindOf returns the JSON type of a valid JSON value. A number literal that
// not even a float64 holds, such as 1e400, has no type: the generated decoder
// could not decode it into any field bodyshapegen allows.
func kindOf(raw []byte) Type {
	switch raw[0] {
	case 'n':
		return Null
	case 't', 'f':
		return Boolean
	case '"':
		return String
	case '[':
		return Array
	case '{':
		return Object
	}
	if _, err := strconv.ParseInt(string(raw), 10, 64); err == nil {
		return Integer | Number
	}
	if _, err := strconv.ParseFloat(string(raw), 64); err == nil {
		return Number
	}
	return 0
}

// checkFormat decodes the value's own bytes into the format's Go type with
// encoding/json: exactly what the generated decoder does with this field, so
// the two accept the same values by construction.
func checkFormat(f Format, raw []byte) error {
	switch f {
	case FormatTime:
		return json.Unmarshal(raw, new(time.Time))
	case FormatUUID:
		return json.Unmarshal(raw, new(uuid.UUID))
	}
	return fmt.Errorf("bodyshape: unknown format %d", f)
}
