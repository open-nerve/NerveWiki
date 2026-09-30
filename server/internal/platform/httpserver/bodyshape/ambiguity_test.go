package bodyshape

import (
	"errors"
	"slices"
	"testing"
	"unicode/utf8"
)

// A body that can be read two ways is refused before its structure is
// checked: a member name twice in one object, at any depth, compared as
// decoded; a string, name or value, that is not valid Unicode. The JSON
// escapes are written \\u in these Go strings, so the body holds the escape
// itself.
func TestCheckReadsABodyOneWayOnly(t *testing.T) {
	const valid = `"name":"a","nested":{"a":"y"}`
	replacement := string(utf8.RuneError)
	tests := []struct {
		name string
		body string
		want []FieldError
	}{
		{"a top-level name twice", `{` + valid + `,"name":"b"}`, []FieldError{{"name", "duplicate"}}},
		// Read as a map, the second nested wins and the first one's x goes unseen; decoded into a struct,
		// the two merge and x is set. Either way the body is refused, whatever the objects hold.
		{"an object twice, the first with what the contract refuses", `{"name":"a","nested":{"a":"y","x":1},"nested":{"a":"y"}}`,
			[]FieldError{{"nested", "duplicate"}}},
		{"a nested name twice", `{"name":"a","nested":{"a":"y","a":"z"}}`, []FieldError{{"nested.a", "duplicate"}}},
		{"a name three times, said once", `{` + valid + `,"name":"b","name":"c"}`, []FieldError{{"name", "duplicate"}}},
		{"an escaped name is the name it decodes to", "{" + valid + ",\"n\\u0061me\":\"b\"}", []FieldError{{"name", "duplicate"}}},
		{"two escapes of one name", "{\"\\u006eame\":\"a\",\"n\\u0061me\":\"b\",\"nested\":{\"a\":\"y\"}}",
			[]FieldError{{"name", "duplicate"}}},
		{"a name twice in an item of an array", `{` + valid + `,"tags":[{"name":"t"},{"name":"t","name":"u"}]}`,
			[]FieldError{{"tags[1].name", "duplicate"}}},
		{"a key twice in an open map", `{` + valid + `,"labels":{"k":"v","k":"w"}}`, []FieldError{{"labels.k", "duplicate"}}},
		{"a name twice where the structure check never looks", `{` + valid + `,"extra":{"q":1,"q":2}}`,
			[]FieldError{{"extra.q", "duplicate"}}},
		{"names that differ in case are two names: the undeclared one is refused", `{` + valid + `,"Name":"b"}`,
			[]FieldError{{"Name", "not_allowed"}}},
		{"the same name in two objects is no duplicate", `{"name":"a","nested":{"a":"y"},"maybe":{"a":"z"}}`, nil},
		{"bytes that are not UTF-8 in a value", "{\"name\":\"A\xffB\",\"nested\":{\"a\":\"y\"}}",
			[]FieldError{{"name", "invalid_format"}}},
		{"bytes that are not UTF-8 in a nested value", "{\"name\":\"a\",\"nested\":{\"a\":\"\xc3\"}}",
			[]FieldError{{"nested.a", "invalid_format"}}},
		{"bytes that are not UTF-8 in an item", "{" + valid + ",\"tags\":[{\"name\":\"\xed\xa0\x80\"}]}",
			[]FieldError{{"tags[0].name", "invalid_format"}}},
		{"bytes that are not UTF-8 in a name", "{" + valid + ",\"n\xffx\":1}", []FieldError{{"n" + replacement + "x", "invalid_format"}}},
		{"a lone high surrogate", "{\"name\":\"A\\ud800B\",\"nested\":{\"a\":\"y\"}}", []FieldError{{"name", "invalid_format"}}},
		{"a high surrogate before an escape that is no surrogate", "{\"name\":\"\\ud800\\u0041\",\"nested\":{\"a\":\"y\"}}",
			[]FieldError{{"name", "invalid_format"}}},
		{"a high surrogate at the end", "{\"name\":\"A\\udbff\",\"nested\":{\"a\":\"y\"}}", []FieldError{{"name", "invalid_format"}}},
		{"a lone low surrogate", "{\"name\":\"\\udc00\",\"nested\":{\"a\":\"y\"}}", []FieldError{{"name", "invalid_format"}}},
		{"two high surrogates", "{\"name\":\"\\ud83d\\ud83d\\ude00\",\"nested\":{\"a\":\"y\"}}", []FieldError{{"name", "invalid_format"}}},
		{"a surrogate pair", "{\"name\":\"\\ud83d\\ude00\",\"nested\":{\"a\":\"y\"}}", nil},
		{"an escaped backslash before u", "{\"name\":\"\\\\ud800\",\"nested\":{\"a\":\"y\"}}", nil},
		{"U+FFFD itself", "{\"name\":\"A" + replacement + "B\\ufffd\",\"nested\":{\"a\":\"y\"}}", nil},
		// The structure of a body with two readings would be checked on one of them: only its ambiguities come back.
		{"every ambiguity, and nothing of the structure", "{\"nested\":{\"a\":\"\xff\"},\"nested\":{},\"zzz\":1}",
			[]FieldError{{"nested", "duplicate"}, {"nested.a", "invalid_format"}}},
		{"a string body that is not UTF-8", "\"\xff\"", []FieldError{{"", "invalid_format"}}},
		// A problem is reported once, however often the body repeats it.
		{"a name twice, its value not UTF-8 each time", "{\"a\":\"\xff\",\"a\":\"\xff\"}",
			[]FieldError{{"a", "duplicate"}, {"a", "invalid_format"}}},
		{"a name that is not UTF-8, twice", "{\"\xff\":1,\"\xff\":2}", []FieldError{{replacement, "invalid_format"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := things().Check(pattern, []byte(tt.body))

			var shape *Error
			if err != nil && !errors.As(err, &shape) {
				t.Fatalf("Check(%q) = %v, want nil or a *Error", tt.body, err)
			}
			var got []FieldError
			if shape != nil {
				got = shape.Fields
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Check(%q) = %s, want %s", tt.body, fields(got), fields(tt.want))
			}
		})
	}
}

func TestADuplicateSaysWhy(t *testing.T) {
	if got := (FieldError{"name", "duplicate"}).Error(); got != "appears more than once in its object" {
		t.Errorf("Error() = %q", got)
	}
}
