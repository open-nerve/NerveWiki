package bodyshape

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// The bound on a body's JSON values (M9/P2 S1 review A2-I1).

// valuesOf is how many JSON values body holds, counted on its decoding:
// itself, and in an object or an array each value in it, down to the
// scalars. It is the tests' own count, apart from the scan's.
func valuesOf(t *testing.T, body string) int {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	var count func(v any) int
	count = func(v any) int {
		n := 1
		switch v := v.(type) {
		case map[string]any:
			for _, x := range v {
				n += count(x)
			}
		case []any:
			for _, x := range v {
				n += count(x)
			}
		}
		return n
	}
	return count(v)
}

// valuesShapes are the shapes a client makes many values of in few bytes,
// each a body of n values: an object of many members, an array of empty
// arrays, an array of numbers.
func valuesShapes() []struct {
	name string
	body func(n int) string
} {
	return []struct {
		name string
		body func(n int) string
	}{
		{"many members", func(n int) string {
			members := make([]string, n-1)
			for i := range members {
				members[i] = fmt.Sprintf(`"m%d":1`, i)
			}
			return "{" + strings.Join(members, ",") + "}"
		}},
		{"nested empty arrays", func(n int) string { return "[" + strings.TrimSuffix(strings.Repeat("[],", n-1), ",") + "]" }},
		{"a long flat array", func(n int) string { return "[" + strings.TrimSuffix(strings.Repeat("1,", n-1), ",") + "]" }},
	}
}

// openTable takes any JSON body, at most most values of it.
func openTable(most int) *Table {
	return &Table{Nodes: []Node{{Types: Any, Extra: Open, Items: Open}}, Roots: map[string]int{pattern: 0}, MaxValues: most}
}

// A body of MaxValues values is read on, the REST API's bound and another
// alike; one value more is *TooManyValues in each shape, before its other
// problems are looked for; with no bound, the same body is read on.
func TestABodyPastMaxValuesIsTooManyValues(t *testing.T) {
	for _, most := range []int{MaxRESTValues, 2500} {
		for _, shape := range valuesShapes() {
			at, past := shape.body(most), shape.body(most+1)
			if n := valuesOf(t, at); n != most {
				t.Fatalf("%s: the body at the bound has %d values, want %d", shape.name, n, most)
			}
			if err := openTable(most).Check(pattern, []byte(at)); err != nil {
				t.Errorf("%s, %d values, the bound: Check = %v, want nil", shape.name, most, err)
			}
			var over *TooManyValues
			if err := openTable(most).Check(pattern, []byte(past)); !errors.As(err, &over) || over.Most != most {
				t.Errorf("%s, %d values: Check = %v, want *TooManyValues of %d", shape.name, most+1, err, most)
			}
			if err := openTable(0).Check(pattern, []byte(past)); err != nil {
				t.Errorf("%s, %d values, no bound: Check = %v, want nil", shape.name, most+1, err)
			}
		}
	}
	// The values are counted wherever they are; a body read two ways or
	// broken past the bound is too many values all the same.
	closed := things()
	closed.MaxValues = 10
	for _, body := range []string{
		`{"name":"a","nested":{"a":"y"},"tags":[{"name":"t"},{"name":"u"}],"labels":{"x":"1"}}`,
		`{"name":"a","name":"b","nested":{"a":"y"},"extra":[1,2,3,4,5,6,7,8]}`,
		`{"undeclared":{"deep":[[[[[[[[1]]]]]]]]}}`,
	} {
		var over *TooManyValues
		if err := closed.Check(pattern, []byte(body)); !errors.As(err, &over) || over.Most != 10 {
			t.Errorf("Check(%s) = %v, want *TooManyValues of 10", body, err)
		}
	}
	if err := closed.Check(pattern, []byte(`{"name":"a","nested":{"a":"y"},"tags":[{"name":"t"}]}`)); err != nil {
		t.Errorf("a body of 7 values = %v, want nil", err)
	}
}

// The refusal is bodyshape's 400 bad_request, its one field the body
// itself, too_long, its message naming the bound.
func TestTooManyValuesIsTheBodysTooLong(t *testing.T) {
	tests := []struct {
		most int
		want string
	}{
		{MaxRESTValues, "has more than 1,000 JSON values: each object, array, string, number, true, false and null is one"},
		{10_000, "has more than 10,000 JSON values: each object, array, string, number, true, false and null is one"},
		{999, "has more than 999 JSON values: each object, array, string, number, true, false and null is one"},
		{1_234_567, "has more than 1,234,567 JSON values: each object, array, string, number, true, false and null is one"},
	}
	for _, tt := range tests {
		e := &TooManyValues{Most: tt.most}
		fields := e.ProblemFields()
		if e.ProblemStatus() != 400 || e.ProblemCode() != "bad_request" || e.Error() != (&Error{}).Error() || len(fields) != 1 {
			t.Fatalf("TooManyValues{%d} = %d %s %q %v, want *Error's 400 bad_request with one field", tt.most,
				e.ProblemStatus(), e.ProblemCode(), e.Error(), fields)
		}
		f, ok := fields[0].(interface {
			ProblemField() string
			ProblemCode() string
		})
		if !ok || f.ProblemField() != "" || f.ProblemCode() != "too_long" || fields[0].Error() != tt.want {
			t.Errorf("TooManyValues{%d}'s field = %#v %q, want the body's too_long: %q", tt.most, fields[0], fields[0].Error(), tt.want)
		}
	}
}
