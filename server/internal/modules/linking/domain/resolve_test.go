package domain_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// What the fixture set does not write: a relative target goes up no
// further than the root, and finds nothing past its own path; one from the
// root does not fall back on a suffix; aliases count for a name alone, after
// every page's name, preferred and tied as suffixes are; nothing found is
// nothing.
func TestResolveStopsAtEachStepsEdge(t *testing.T) {
	tree := treeOf([]string{"Note", "A", "A/Note", "A/B", "A/B/src", "X", "X/Al", "Y", "Y/src", "Z", "Z/Al", "Q", "Q/Name"},
		map[string][]string{"X": {"Shared", "Name"}, "Z": {"Shared"}, "Y/src": {"Own"}})
	tests := []struct {
		target, from string
		want         string
		ambiguous    bool
	}{
		{"../../../Note", "A/B/src", "Note", false},
		{"./Note", "A/B/src", "", false},
		{"/B/src", "Note", "", false},
		{"/A/Note.md", "Note", "A/Note", false},
		{"Shared", "Note", "X", true},
		{"Shared", "Y/src", "X", true},
		{"Shared", "A/B/src", "X", true},
		{"Own", "Y/src", "Y/src", false},
		{"A/Own", "Note", "", false},
		{"Name", "Note", "Q/Name", false},
		{"Al", "Note", "X/Al", true},
		{"Missing", "A/B/src", "", false},
	}
	for _, tt := range tests {
		target, ok := domain.ParseTarget(tt.target)
		if !ok {
			t.Fatalf("ParseTarget(%q) refused", tt.target)
		}
		want := domain.Resolution{ID: tree.ids[tt.want], Ambiguous: tt.ambiguous}
		if got := tree.resolve(target, tt.from); got != want {
			t.Errorf("%s from %s: %s, want %s", tt.target, tt.from, tree.name(got), tree.name(want))
		}
	}
}

// A page in the source folder's subtree wins over one with fewer levels
// elsewhere, and among those in it, the one with fewer levels wins, with no
// tie; from the root, every page is in it.
func TestResolvePrefersTheSourceFoldersSubtree(t *testing.T) {
	tree := treeOf([]string{"P", "P/src", "P/src/dup", "P/Q", "P/Q/R", "P/Q/R/dup", "P/Q/R/src", "S", "S/dup"}, nil)
	for from, want := range map[string]string{
		"P/src":     "P/src/dup",
		"P/Q":       "P/src/dup",
		"P/Q/R/src": "P/Q/R/dup",
		"S":         "S/dup",
	} {
		target, _ := domain.ParseTarget("dup")
		if got := tree.resolve(target, from); got != (domain.Resolution{ID: tree.ids[want]}) {
			t.Errorf("dup from %s: %s, want %s", from, tree.name(got), want)
		}
	}
}
