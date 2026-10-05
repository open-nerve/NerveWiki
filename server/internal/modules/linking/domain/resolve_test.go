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
	tree := treeOf(t, []string{"Note", "A", "A/Note", "A/B", "A/B/src", "X", "X/Al", "Y", "Y/src", "Z", "Z/Al", "Q", "Q/Name"},
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

// A page in the source folder's subtree wins over one with a shorter path
// elsewhere, and among those in it, the one with the shorter path wins,
// with no tie; from the root, every page is in it.
func TestResolvePrefersTheSourceFoldersSubtree(t *testing.T) {
	tree := treeOf(t, []string{"P", "P/src", "P/src/dup", "P/Q", "P/Q/Rr", "P/Q/Rr/dup", "P/Q/Rr/src", "S", "S/dup"}, nil)
	for from, want := range map[string]string{
		"P/src":      "P/src/dup",
		"P/Q":        "P/src/dup",
		"P/Q/Rr/src": "P/Q/Rr/dup",
		"S":          "S/dup",
	} {
		target, _ := domain.ParseTarget("dup")
		if got := tree.resolve(target, from); got != (domain.Resolution{ID: tree.ids[want]}) {
			t.Errorf("dup from %s: %s, want %s", from, tree.name(got), want)
		}
	}
}

// The shortest path is Obsidian's: the fewest characters as JavaScript
// counts them, in UTF-16 code units (é one, 😀 two), whatever the levels;
// the least id breaks a tie, and tells it.
func TestTheShortestPathIsCountedInUTF16(t *testing.T) {
	tests := []struct {
		pages     []string
		want      string
		ambiguous bool
	}{
		{[]string{"Longfoldername", "Longfoldername/dup", "a", "a/b", "a/b/dup"}, "a/b/dup", false},
		{[]string{"abc", "abc/dup", "éé", "éé/dup"}, "éé/dup", false},
		{[]string{"ab", "ab/dup", "😀", "😀/dup"}, "ab/dup", true},
	}
	for _, tt := range tests {
		tree := treeOf(t, append(tt.pages, "src"), nil)
		target, _ := domain.ParseTarget("dup")
		if got, want := tree.resolve(target, "src"), (domain.Resolution{ID: tree.ids[tt.want], Ambiguous: tt.ambiguous}); got != want {
			t.Errorf("dup among %v: %s, want %s", tt.pages, tree.name(got), tree.name(want))
		}
	}
}

// A target written with ".md", in any case, is read in one form, as
// Obsidian reads it: the page without it when the notebook has a page of
// that name anywhere, in every step; the page with it otherwise. Aliases,
// which Obsidian does not resolve, are tried without it, then with it.
func TestATargetWithMdIsReadInOneForm(t *testing.T) {
	tests := []struct {
		pages   []string
		aliases map[string][]string
		target  string
		want    string
	}{
		{[]string{"A", "A/x", "x.md"}, nil, "x.md", "A/x"},
		{[]string{"A", "A/x", "x.md"}, nil, "/x.md", ""},
		{[]string{"x", "B", "B/x.md"}, nil, "B/x.md", ""},
		{[]string{"B", "B/x.md"}, nil, "B/x.md", "B/x.md"},
		{[]string{"A", "A/x"}, nil, "x.MD", "A/x"},
		{[]string{"A", "A/x.MD"}, nil, "x.md", "A/x.MD"},
		{[]string{"P", "P/Q", "P/Q/X", "Y"}, map[string][]string{"P/Q/X": {"Al"}, "Y": {"Al.md"}}, "Al.md", "P/Q/X"},
		{[]string{"Y"}, map[string][]string{"Y": {"Al.md"}}, "Al.md", "Y"},
	}
	for _, tt := range tests {
		tree := treeOf(t, append(tt.pages, "src"), tt.aliases)
		target, ok := domain.ParseTarget(tt.target)
		if !ok {
			t.Fatalf("ParseTarget(%q) refused", tt.target)
		}
		if got := tree.resolve(target, "src"); got != (domain.Resolution{ID: tree.ids[tt.want]}) {
			t.Errorf("%s among %v: %s, want %s", tt.target, tt.pages, tree.name(got), tt.want)
		}
	}
}
