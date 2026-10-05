package obsidian_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// A tag's name is of tag runes, and not all ASCII digits (rule 9), as the
// body's tags are found.
func TestIsTagIsRuleNine(t *testing.T) {
	for name, want := range map[string]bool{
		"todo": true, "a/b-c_d": true, "1a": true, "½": true, "١٢": true, "中文": true, "é": true,
		"": false, "123": false, "a.b": false, "a b": false, "a,b": false, "#a": false, "a😀": false, "\xff": false,
	} {
		if got := obsidian.IsTag(name); got != want {
			t.Errorf("IsTag(%q) = %v, want %v", name, got, want)
		}
	}
}
