package markdown

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// asJSON is a frontmatter's properties as the fixtures write them.
func asJSON(t *testing.T, props []Property) string {
	t.Helper()
	b, err := json.Marshal(jsonOf(props))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// jsonOf turns properties into maps and slices json.Marshal writes.
func jsonOf(v any) any {
	switch v := v.(type) {
	case []Property:
		m := map[string]any{}
		for _, p := range v {
			m[p.Key] = jsonOf(p.Value)
		}
		return m
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = jsonOf(e)
		}
		return out
	}
	return v
}

func TestScalarsFollowTheCoreSchema(t *testing.T) {
	tests := []struct {
		yaml string
		want any
	}{
		{"~", nil}, {"null", nil}, {"Null", nil}, {"NULL", nil}, {"", nil}, {"nul", "nul"},
		{"true", true}, {"True", true}, {"TRUE", true}, {"false", false}, {"FALSE", false},
		{"yes", "yes"}, {"no", "no"}, {"on", "on"}, {"tRUE", "tRUE"},
		{"010", int64(10)}, {"+12", int64(12)}, {"-0", int64(0)}, {"0o17", int64(15)}, {"0x1F", int64(31)},
		{"0x", "0x"}, {"-0x1", "-0x1"}, {"0o8", "0o8"}, {"0b1", "0b1"},
		{"9223372036854775807", int64(9223372036854775807)}, {"9223372036854775808", float64(9223372036854775808)},
		{"0xFFFFFFFFFFFFFFFFF", float64(0xFFFFFFFFFFFFFFFFF)},
		{"1.0", 1.0}, {"1e3", 1000.0}, {".5", 0.5}, {"-1.", -1.0}, {"1e400", "1e400"}, {"1_000", "1_000"},
		{".inf", ".inf"}, {"-.Inf", "-.Inf"}, {"+.INF", "+.INF"}, {".NaN", ".NaN"}, {".nan", ".nan"},
		{"2024-01-01", "2024-01-01"}, {"2024-01-01T10:00:00Z", "2024-01-01T10:00:00Z"},
		{`"010"`, "010"}, {"'true'", "true"}, {"|\n  010\n", "010\n"}, {">\n  a\n  b\n", "a b\n"},
		{"!!str 10", "10"}, {"!!int 10", int64(10)}, {`!!int "10"`, int64(10)}, {"!!float .inf", ".inf"}, {"!!bool true", true},
		{"!!null ~", nil},
	}
	for _, tt := range tests {
		t.Run(tt.yaml, func(t *testing.T) {
			props, ok := properties([]byte("k: " + tt.yaml))
			if !ok || len(props) != 1 {
				t.Fatalf("properties = %v, %v", props, ok)
			}
			if got := props[0].Value; fmt.Sprintf("%T %v", got, got) != fmt.Sprintf("%T %v", tt.want, tt.want) {
				t.Errorf("k: %s = %T %v, want %T %v", tt.yaml, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestAFrontmatterIsAMappingOfScalarKeys(t *testing.T) {
	tests := []struct {
		name, yaml string
		valid      bool
		want       string // the properties as JSON, when valid
	}{
		{"empty", "", true, "{}"},
		{"only comments", "# a\n# b\n", true, "{}"},
		{"an empty mapping", "{}", true, "{}"},
		{"keys in order", "b: 1\na: 2\n", true, `{"a":2,"b":1}`},
		{"a key resolved", "010: a\ntrue: b\n~: c\n1.5: d\n", true, `{"1.5":"d","10":"a","null":"c","true":"b"}`},
		{"a merge key is a key", "<<: {a: 1}\n", true, `{"\u003c\u003c":{"a":1}}`},
		{"nested", "a:\n  b: [1, {c: d}]\n", true, `{"a":{"b":[1,{"c":"d"}]}}`},
		{"aliases expanded", "a: &x [1, 2]\nb: *x\n", true, `{"a":[1,2],"b":[1,2]}`},
		{"an alias as a key", "a: &k x\n*k : y\n", true, `{"a":"x","x":"y"}`},
		{"aliases in aliases", "a: &a [1]\nb: &b [*a, *a]\nc: [*b]\n", true, `{"a":[1],"b":[[1],[1]],"c":[[[1],[1]]]}`},
		{"a sequence", "- a\n- b\n", false, ""},
		{"a scalar", "hello", false, ""},
		{"null", "~", false, ""},
		{"a duplicate key", "a: 1\na: 2\n", false, ""},
		{"a sequence as a key", "? [a]\n: b\n", false, ""},
		{"a custom tag", "a: !x 1\n", false, ""},
		{"a tag that does not fit", "a: !!int x\n", false, ""},
		{"a mapping tagged as a sequence", "a: !!seq {b: 1}\n", false, ""},
		{"a syntax error", "a: [unclosed\n", false, ""},
		{"two documents", "a: 1\n...\n---\nb: 2\n", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props, ok := properties([]byte(tt.yaml))
			if ok != tt.valid {
				t.Fatalf("valid = %v, want %v (%v)", ok, tt.valid, props)
			}
			if ok {
				if got := asJSON(t, props); got != tt.want {
					t.Errorf("properties = %s, want %s", got, tt.want)
				}
			}
		})
	}
}

// Aliases expand to at most maxYAMLNodes values nested at most maxYAMLDepth
// deep; past either the frontmatter is not valid, and an alias bomb costs
// what its limit allows.
func TestAliasesExpandWithinTheLimits(t *testing.T) {
	seq := func(n int) string { return "[" + strings.TrimSuffix(strings.Repeat("x, ", n), ", ") + "]" }
	tests := []struct {
		name, yaml string
		valid      bool
	}{
		// The mapping, its two keys, and the sequence with its n items twice:
		// 5 + 2n values.
		{"at the node limit", "a: &s " + seq((maxYAMLNodes-1)/2-2) + "\nb: *s\n", true},
		{"past the node limit", "a: &s " + seq((maxYAMLNodes-1)/2) + "\nb: *s\n", false},
		{"at the depth limit", "a: " + strings.Repeat("[", maxYAMLDepth) + strings.Repeat("]", maxYAMLDepth), true},
		{"past the depth limit", "a: " + strings.Repeat("[", maxYAMLDepth+1) + strings.Repeat("]", maxYAMLDepth+1), false},
		{"an alias bomb", "a: &a [x,x,x,x,x,x,x,x,x,x]\nb: &b [*a,*a,*a,*a,*a,*a,*a,*a,*a,*a]\n" +
			"c: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b,*b]\nd: &d [*c,*c,*c,*c,*c,*c,*c,*c,*c,*c]\n" +
			"e: &e [*d,*d,*d,*d,*d,*d,*d,*d,*d,*d]\nf: &f [*e,*e,*e,*e,*e,*e,*e,*e,*e,*e]\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := properties([]byte(tt.yaml)); ok != tt.valid {
				t.Errorf("valid = %v, want %v", ok, tt.valid)
			}
		})
	}
}
