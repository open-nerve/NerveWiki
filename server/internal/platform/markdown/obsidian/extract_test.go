package obsidian

import (
	"testing"
)

// decodeURI is JavaScript's (rule 8).
func TestDecodeURIIsJavaScripts(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"a.md", "a.md"},
		{"c%20d.md", "c d.md"},
		{"%E4%B8%AD", "中"},
		{"%e4%b8%ad%2F", "中%2F"},
		{"a%2Fb%3F%23%25", "a%2Fb%3F%23%"},
		{"%41%3b", "A%3b"},
		{"%F0%9F%98%80", "😀"},
		{"%", "%"},
		{"%4", "%4"},
		{"%ZZ x", "%ZZ x"},
		{"%E4%B8 x", "%E4%B8 x"},               // a character cut short
		{"%C0%80", "%C0%80"},                   // an overlong form
		{"%ED%A0%80", "%ED%A0%80"},             // a surrogate
		{"%80", "%80"},                         // a continuation byte alone
		{"ok %20 then %FF", "ok %20 then %FF"}, // one bad escape keeps all
		{"%ZZ%20", "%ZZ%20"},
	} {
		if got := decodeURI(tt.in); got != tt.want {
			t.Errorf("decodeURI(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// A Markdown link's destination is a link of the notebook's unless it has
// a scheme, starts with "//", or is an anchor alone (rule 8). Its range is
// its target's, as written.
func TestAMarkdownLinksDestination(t *testing.T) {
	for _, tt := range []struct {
		written        string
		ok             bool
		target, anchor string
		size           int // how many bytes the target is written in
	}{
		{"a.md", true, "a.md", "", 4},
		{"sub/b%20c.md#h%20i", true, "sub/b c.md", "h i", 12},
		{"/abs.md", true, "/abs.md", "", 7},
		{"../up", true, "../up", "", 5},
		{"1x:y", true, "1x:y", "", 4},
		{"https://x.com/a.md", false, "", "", 0},
		{"mailto:a@b.c", false, "", "", 0},
		{"x+y.z-w:rest", false, "", "", 0},
		{"//host/a.md", false, "", "", 0},
		{"#h", false, "", "", 0},
	} {
		l, ok := markdownLink(KindLink, tt.written, 10)
		if ok != tt.ok || ok && (l.Target != tt.target || l.Anchor != tt.anchor || l.Range.Start != 10 || l.Range.Stop != 10+tt.size) {
			t.Errorf("markdownLink(%q) = %+v, %v", tt.written, l, ok)
		}
	}
}
