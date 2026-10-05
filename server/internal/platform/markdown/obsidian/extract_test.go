package obsidian

import (
	"testing"
)

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
