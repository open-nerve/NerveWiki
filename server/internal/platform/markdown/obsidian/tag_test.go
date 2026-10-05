package obsidian_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// The tag pane counts a name without one '/' it ends with, unless it is
// then empty, all ASCII digits, or has white space as JavaScript takes it
// (U+FEFF, not U+0085), a General or Supplemental Punctuation, or an ASCII
// punctuation but '-', '_' and '/'.
func TestCountedTagIsTheTagPanes(t *testing.T) {
	counted := map[string]string{
		"todo": "todo", "a/b-c_d": "a/b-c_d", "1a": "1a", "½": "½", "١٢": "١٢", "中文": "中文", "é": "é",
		"a/": "a", "a//": "a/", "a😀": "a😀", "a→b": "a→b", "a€": "a€", "a。b": "a。b", "a\u0085": "a\u0085",
		"a\u180eb": "a\u180eb", "a\x00b": "a\x00b",
	}
	for name, want := range counted {
		if got, ok := obsidian.CountedTag(name); !ok || got != want {
			t.Errorf("CountedTag(%q) = %q, %v; want %q", name, got, ok, want)
		}
	}
	for _, name := range []string{
		"", "/", "123", "123/", "a.b", "a b", "a\tb", "a\u00a0b", "a\u3000b", "a\ufeff", "a,b", "#a", "a'b", "a`b", "a\\b",
		"a\u2014b", "a\u2026", "a\u2e3ab", "a[b]", "a{b}", "a~b",
	} {
		if got, ok := obsidian.CountedTag(name); ok {
			t.Errorf("CountedTag(%q) = %q, true; want none", name, got)
		}
	}
}
