package markdown

import (
	"strings"
	"testing"
)

func TestTheFrontmatterIsWhereRuleOneSaysItIs(t *testing.T) {
	tests := []struct {
		name, src string
		ok        bool
		yaml      string // between the delimiters
		end       int    // where the parse's text starts
	}{
		{"LF", "---\na: 1\n---\nbody", true, "a: 1\n", 13},
		{"CRLF", "---\r\na: 1\r\n---\r\nbody", true, "a: 1\r\n", 16},
		{"a byte order mark first", "\xef\xbb\xbf---\na: 1\n---\nbody", true, "a: 1\n", 16},
		{"empty", "---\n---\nbody", true, "", 8},
		{"the closing line last, without a newline", "---\na: 1\n---", true, "a: 1\n", 12},
		{"the closing line with \\r last", "---\na: 1\n---\r", true, "a: 1\n", 13},
		{"no closing line", "---\na: 1\nbody", false, "", 0},
		{"a blank line first", "\n---\na: 1\n---\n", false, "", 0},
		{"a space after the opening ---", "--- \na: 1\n---\n", false, "", 0},
		{"a space after the closing ---", "---\na: 1\n--- \nb\n---\nbody", true, "a: 1\n--- \nb\n", 20},
		{"only the opening line", "---\n", false, "", 0},
		{"no newline after the opening ---", "---", false, "", 0},
		{"four dashes", "----\na\n---\n", false, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ok := frontmatterSpan([]byte(tt.src))
			if ok != tt.ok {
				t.Fatalf("found %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if got := tt.src[s.from:s.to]; got != tt.yaml || s.end != tt.end {
				t.Errorf("YAML %q ending at %d, want %q ending at %d", got, s.end, tt.yaml, tt.end)
			}
		})
	}
}

// The parse's text is the content with its byte order mark and frontmatter
// made spaces but for the line breaks: every offset stays.
func TestBlankKeepsEveryOffset(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{"a frontmatter", "---\r\na: 1\n---\nbody", "   \r\n    \n   \nbody"},
		{"a byte order mark and a frontmatter", "\xef\xbb\xbf---\na\n---\n# t", "      \n \n   \n# t"},
		{"a byte order mark alone", "\xef\xbb\xbf# t", "   # t"},
		{"neither", "# t\n---\n", "# t\n---\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, end := frontmatterOf([]byte(tt.src))
			if got := string(blank([]byte(tt.src), end)); got != tt.want {
				t.Errorf("blank = %q, want %q", got, tt.want)
			}
		})
	}
	src := []byte("---\na: 1\n---\nbody")
	_, end := frontmatterOf(src)
	blank(src, end)
	if !strings.HasPrefix(string(src), "---") {
		t.Errorf("blank changed the content: %q", src)
	}
}
