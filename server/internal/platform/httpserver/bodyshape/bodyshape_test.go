package bodyshape

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"uuid"
)

const pattern = "POST /api/v0/things"

// things is what bodyshapegen writes for a schema with every supported
// construct: required and optional properties, a nullable object and
// nullable scalars, nesting, an array of objects, an open map, formats.
func things() *Table {
	return &Table{
		Nodes: []Node{
			/* 0 */ {Types: Object, Extra: Closed, Items: Open, Required: []string{"name", "nested"},
				Props: map[string]int{"count": 2, "id": 4, "labels": 10, "maybe": 5, "name": 1, "nested": 6, "note": 11, "tags": 8, "when": 3}},
			/* 1 */ {Types: String, Extra: Open, Items: Open},
			/* 2 */ {Types: Integer, Extra: Open, Items: Open},
			/* 3 */ {Types: String, Format: FormatTime, Extra: Open, Items: Open},
			/* 4 */ {Types: String | Null, Format: FormatUUID, Extra: Open, Items: Open},
			/* 5 */ {Types: Object | Null, Extra: Closed, Items: Open, Props: map[string]int{"a": 1}, Required: []string{"a"}},
			/* 6 */ {Types: Object, Extra: Closed, Items: Open, Props: map[string]int{"a": 1, "b": 7}, Required: []string{"a"}},
			/* 7 */ {Types: Integer | Null, Extra: Open, Items: Open},
			/* 8 */ {Types: Array, Extra: Open, Items: 9},
			/* 9 */ {Types: Object, Extra: Closed, Items: Open, Props: map[string]int{"name": 1}, Required: []string{"name"}},
			/* 10 */ {Types: Object, Extra: 1, Items: Open, Props: map[string]int{}},
			/* 11 */ {Types: Number, Extra: Open, Items: Open},
		},
		Roots: map[string]int{pattern: 0},
	}
}

// fields writes problems as path and code: %v would write each one's
// Error(), which leaves the path out.
func fields(errs []FieldError) string {
	s := make([]string, len(errs))
	for i, f := range errs {
		s[i] = fmt.Sprintf("%q %s", f.Field, f.Code)
	}
	return "[" + strings.Join(s, ", ") + "]"
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []FieldError
	}{
		{"valid", `{"name":"a","count":3,"when":"2026-09-25T10:00:00Z","id":"0199a2b4-7c3e-7d2a-9f10-2b3c4d5e6f70",` +
			`"maybe":{"a":"x"},"nested":{"a":"y","b":2},"tags":[{"name":"t"}],"labels":{"k":"v"},"note":1.5}`, nil},
		{"only the required properties", `{"name":"a","nested":{"a":"y"}}`, nil},
		{"unknown top-level property", `{"name":"a","nested":{"a":"y"},"extra":1}`,
			[]FieldError{{"extra", "not_allowed"}}},
		{"unknown nested property", `{"name":"a","nested":{"a":"y","x":true}}`,
			[]FieldError{{"nested.x", "not_allowed"}}},
		{"null for an optional non-nullable property", `{"name":"a","nested":{"a":"y"},"count":null}`,
			[]FieldError{{"count", "invalid_format"}}},
		{"null for a required non-nullable property", `{"name":null,"nested":{"a":"y"}}`,
			[]FieldError{{"name", "invalid_format"}}},
		{"null where the contract allows it", `{"name":"a","nested":{"a":"y","b":null},"maybe":null,"id":null}`, nil},
		{"missing required properties", `{"nested":{}}`,
			[]FieldError{{"name", "required"}, {"nested.a", "required"}}},
		{"wrong types", `{"name":5,"nested":[],"count":"3"}`,
			[]FieldError{{"count", "invalid_format"}, {"name", "invalid_format"}, {"nested", "invalid_format"}}},
		{"a decimal is not an integer", `{"name":"a","nested":{"a":"y"},"count":1.5}`,
			[]FieldError{{"count", "invalid_format"}}},
		{"an exponent is not an integer", `{"name":"a","nested":{"a":"y"},"count":1e2}`,
			[]FieldError{{"count", "invalid_format"}}},
		{"an integer is a number", `{"name":"a","nested":{"a":"y"},"note":7}`, nil},
		{"an integer no int64 holds", `{"name":"a","nested":{"a":"y"},"count":9223372036854775808}`,
			[]FieldError{{"count", "invalid_format"}}},
		{"a number no float64 holds", `{"name":"a","nested":{"a":"y"},"note":1e400}`,
			[]FieldError{{"note", "invalid_format"}}},
		{"array items", `{"name":"a","nested":{"a":"y"},"tags":[{"name":"t"},{},{"name":"u","x":1}]}`,
			[]FieldError{{"tags[1].name", "required"}, {"tags[2].x", "not_allowed"}}},
		{"open map values", `{"name":"a","nested":{"a":"y"},"labels":{"k":5,"j":"ok"}}`,
			[]FieldError{{"labels.k", "invalid_format"}}},
		{"wrong date-time", `{"name":"a","nested":{"a":"y"},"when":"2026-09-25 10:00:00Z"}`,
			[]FieldError{{"when", "invalid_format"}}},
		{"date-time with an escaped Z", `{"name":"a","nested":{"a":"y"},"when":"2026-09-25T10:00:00\u005a"}`, nil},
		{"wrong uuid", `{"name":"a","nested":{"a":"y"},"id":"xyz"}`,
			[]FieldError{{"id", "invalid_format"}}},
		{"the body is not an object", `["name"]`, []FieldError{{"", "invalid_format"}}},
		{"a space before the body", " " + `{"name":"a","nested":{"a":"y"}}`, nil},
		{"a newline before the body", "\n" + `{"name":"a","nested":{"a":"y"}}`, nil},
		{"a tab before the body", "\t" + `{"name":"a","nested":{"a":"y"}}`, nil},
		{"CRLF before the body", "\r\n" + `{"name":"a","nested":{"a":"y"}}`, nil},
		{"whitespace after the body", `{"name":"a","nested":{"a":"y"}}` + " \t\r\n", nil},
		{"every problem at once, sorted by path", `{"when":"yesterday","zzz":1,"nested":{"a":"y","b":"2"}}`,
			[]FieldError{{"name", "required"}, {"nested.b", "invalid_format"}, {"when", "invalid_format"}, {"zzz", "not_allowed"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := things().Check(pattern, []byte(tt.body))

			var shape *Error
			if err != nil && !errors.As(err, &shape) {
				t.Fatalf("Check(%s) = %v, want nil or a *Error", tt.body, err)
			}
			var got []FieldError
			if shape != nil {
				got = shape.Fields
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Check(%s) = %s, want %s", tt.body, fields(got), fields(tt.want))
			}
		})
	}
}

// A path longer than maxPath bytes is cut short: its first bytes, cut at a
// rune boundary, then an ellipsis, maxPath bytes in all at most. The cut is
// maxPath minus the ellipsis's 3 bytes, 253; the names of é (2 bytes) and 中
// (3 bytes) put it inside a rune.
func TestCheckCutsALongPathShort(t *testing.T) {
	x := func(n int) string { return strings.Repeat("x", n) }
	const valid = `"name":"a","nested":{"a":"y"`
	tests := []struct {
		name string
		body string
		want string
	}{
		{"a path of maxPath bytes is whole", `{` + valid + `},"` + x(256) + `":1}`, x(256)},
		{"a path of one byte more is cut short", `{` + valid + `},"` + x(257) + `":1}`, x(253) + "…"},
		{"a nested path of maxPath bytes is whole", `{` + valid + `,"` + x(249) + `":1}}`, "nested." + x(249)},
		{"a nested path is cut short as a whole", `{` + valid + `,"` + x(250) + `":1}}`, "nested." + x(246) + "…"},
		{"the cut falls on the second byte of é", `{` + valid + `},"` + strings.Repeat("é", 200) + `":1}`,
			strings.Repeat("é", 126) + "…"},
		{"the cut falls on the second byte of 中", `{` + valid + `},"` + strings.Repeat("中", 100) + `":1}`,
			strings.Repeat("中", 84) + "…"},
		{"the cut falls on the third byte of 中", `{` + valid + `},"xx` + strings.Repeat("中", 100) + `":1}`,
			"xx" + strings.Repeat("中", 83) + "…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var shape *Error
			if err := things().Check(pattern, []byte(tt.body)); !errors.As(err, &shape) {
				t.Fatalf("Check = %v, want a *Error", err)
			}
			if want := []FieldError{{tt.want, "not_allowed"}}; !slices.Equal(shape.Fields, want) {
				t.Errorf("Check = %s, want %s", fields(shape.Fields), fields(want))
			}
			if got := shape.Fields[0].Field; len(got) > maxPath || !utf8.ValidString(got) {
				t.Errorf("path of %d bytes, valid UTF-8 %v; want at most %d, valid", len(got), utf8.ValidString(got), maxPath)
			}
		})
	}
}

// Check owns what a body without one JSON value means, so it answers every
// body: nothing to check when the body is empty or only JSON whitespace (the
// generated decoder answers it), ErrNotJSON when it is not JSON.
func TestCheckOfABodyThatIsNotOneJSONValue(t *testing.T) {
	tests := []struct {
		name, body string
		want       error
	}{
		{"empty", "", nil},
		{"only JSON whitespace", " \t\r\n", nil},
		{"cut short", `{"name":`, ErrNotJSON},
		{"trailing data", `{"name":"a","nested":{"a":"y"}} trailing`, ErrNotJSON},
		// A form feed is not JSON whitespace (RFC 8259 §2), so neither body
		// is empty or an object with space before it.
		{"a form feed", "\f", ErrNotJSON},
		{"a form feed before the body", "\f" + `{"name":"a","nested":{"a":"y"}}`, ErrNotJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := things().Check(pattern, []byte(tt.body)); !errors.Is(err, tt.want) {
				t.Errorf("Check(%q) = %v, want %v", tt.body, err, tt.want)
			}
		})
	}
}

func TestCheckOfARouteWithoutABody(t *testing.T) {
	if got := things().Check("GET /api/v0/things", []byte(`{"x":1}`)); got != nil {
		t.Errorf("Check() = %v, want nil for a route without a root", got)
	}
}

// The checker must accept exactly what the generated decoder accepts: it
// decodes the field's own bytes into the same Go type.
func TestFormatsMatchTheDecoder(t *testing.T) {
	type decoded struct {
		When time.Time `json:"when"`
		ID   uuid.UUID `json:"id"`
	}
	tests := []struct {
		field  string
		format Format
		raw    string
	}{
		{"when", FormatTime, `"2026-09-25T10:00:00Z"`},
		{"when", FormatTime, `"2026-09-25T10:00:00+08:00"`},
		{"when", FormatTime, `"2026-09-25 10:00:00Z"`},
		{"when", FormatTime, `"2026-09-25"`},
		{"when", FormatTime, `"2026-09-25T10:00:00\u005a"`},
		{"when", FormatTime, `"not a time"`},
		{"id", FormatUUID, `"0199a2b4-7c3e-7d2a-9f10-2b3c4d5e6f70"`},
		{"id", FormatUUID, `"0199A2B4-7C3E-7D2A-9F10-2B3C4D5E6F70"`},
		{"id", FormatUUID, `"0199a2b47c3e7d2a9f102b3c4d5e6f70"`},
		{"id", FormatUUID, `"{0199a2b4-7c3e-7d2a-9f10-2b3c4d5e6f70}"`},
		{"id", FormatUUID, `"urn:uuid:0199a2b4-7c3e-7d2a-9f10-2b3c4d5e6f70"`},
		{"id", FormatUUID, `"\u0030199a2b4-7c3e-7d2a-9f10-2b3c4d5e6f70"`},
		{"id", FormatUUID, `"xyz"`},
	}
	accepted := 0
	for _, tt := range tests {
		got := checkFormat(tt.format, []byte(tt.raw))
		want := json.Unmarshal([]byte(`{"`+tt.field+`":`+tt.raw+`}`), new(decoded))
		if (got == nil) != (want == nil) {
			t.Errorf("%s %s: checker %v, decoder %v", tt.field, tt.raw, got, want)
		}
		if got == nil {
			accepted++
		}
	}
	// Both outcomes occur, so the comparison is not trivially one-sided.
	if accepted == 0 || accepted == len(tests) {
		t.Errorf("%d of %d values accepted, want some of each", accepted, len(tests))
	}
}

// A number passes the check exactly when the generated decoder can decode it
// into the Go types bodyshapegen allows: int64 (and int) for Integer, float64
// for Number.
func TestNumbersMatchTheDecoder(t *testing.T) {
	for _, raw := range []string{
		"0", "-0", "7", "9223372036854775807", "-9223372036854775808", "9223372036854775808", "-9223372036854775809",
		"1.5", "1e2", "1E-2", "-0.0", "1.7976931348623157e308", "1.8e308", "1e400", "-1e400", "4.9e-324", "1e-400",
	} {
		kind := kindOf([]byte(raw))
		intErr := json.Unmarshal([]byte(raw), new(int64))
		floatErr := json.Unmarshal([]byte(raw), new(float64))
		if (kind&Integer != 0) != (intErr == nil) {
			t.Errorf("%s: Integer %v, int64 decoder %v", raw, kind&Integer != 0, intErr)
		}
		if (kind&Number != 0) != (floatErr == nil) {
			t.Errorf("%s: Number %v, float64 decoder %v", raw, kind&Number != 0, floatErr)
		}
	}
}

func TestErrorIsAProblem(t *testing.T) {
	err := &Error{Fields: []FieldError{{"name", "required"}, {"extra", "not_allowed"}}}

	if err.ProblemStatus() != 400 || err.ProblemCode() != "bad_request" {
		t.Errorf("problem = %d %s, want 400 bad_request", err.ProblemStatus(), err.ProblemCode())
	}
	fields := err.ProblemFields()
	if len(fields) != 2 {
		t.Fatalf("ProblemFields() = %v, want 2", fields)
	}
	f, ok := fields[0].(interface {
		ProblemField() string
		ProblemCode() string
	})
	if !ok || f.ProblemField() != "name" || f.ProblemCode() != "required" || fields[0].Error() != "is required" {
		t.Errorf("fields[0] = %v, want name required with a message", fields[0])
	}
}
