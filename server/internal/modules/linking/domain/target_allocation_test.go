//go:build !race

package domain_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// The keys own only the storage they return. Already-normalized ASCII needs
// one segment slice; a link's separate last-key result is needed only for
// multiple segments or the two .md forms. No shared result cache is allowed.
func TestTargetKeysUseOnlyTheirOwnedSlices(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		target      domain.Target
		keys        []string
		linkAllocs  float64
	}{
		{"plain", "note", domain.Target{Keys: []string{"note"}, Name: "note"}, []string{"note"}, 1},
		{"segments", "a/b/note", domain.Target{Keys: []string{"a", "b", "note"}, Name: "note"}, []string{"note"}, 2},
		{"relative", "./note", domain.Target{Relative: true, Keys: []string{"note"}, Name: "note"}, []string{"note"}, 1},
		{"up", "../note", domain.Target{Relative: true, Up: 1, Keys: []string{"note"}, Name: "note"}, []string{"note"}, 1},
		{"rooted", "/note", domain.Target{Rooted: true, Keys: []string{"note"}, Name: "note"}, []string{"note"}, 1},
		{"extension", "note.md", domain.Target{Keys: []string{"note"}, AltLast: "note.md", Name: "note"}, []string{"note", "note.md"}, 2},
		{"dot_md", ".md", domain.Target{Keys: []string{".md"}, Name: ".md"}, []string{".md"}, 1},
		{"repeated_md", "note.md.md", domain.Target{Keys: []string{"note.md"}, AltLast: "note.md.md", Name: "note.md"}, []string{"note.md", "note.md.md"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got domain.Target
			var ok bool
			targetAllocs := testing.AllocsPerRun(100, func() { got, ok = domain.ParseTarget(tc.input) })
			if !ok || !reflect.DeepEqual(got, tc.target) {
				t.Fatalf("target = %#v, %v; want %#v, true", got, ok, tc.target)
			}
			if targetAllocs > 1 {
				t.Errorf("ParseTarget allocated %.0f slices, want at most one owned segment slice", targetAllocs)
			}
			var keys []string
			link := domain.Link{Target: tc.input}
			linkAllocs := testing.AllocsPerRun(100, func() { keys = link.Keys() })
			if !reflect.DeepEqual(keys, tc.keys) {
				t.Fatalf("link keys = %#v, want %#v", keys, tc.keys)
			}
			if linkAllocs > tc.linkAllocs {
				t.Errorf("Link.Keys allocated %.0f objects, want at most %.0f owned slices", linkAllocs, tc.linkAllocs)
			}
		})
	}
}

func TestTargetKeyResultsRemainIndependent(t *testing.T) {
	for _, input := range []string{"note", "a/b/note", "note.md"} {
		first, ok := domain.ParseTarget(input)
		if !ok {
			t.Fatal("literal target rejected")
		}
		second, ok := domain.ParseTarget(input)
		if !ok {
			t.Fatal("second literal target rejected")
		}
		first.Keys[len(first.Keys)-1] = "changed"
		if second.Keys[len(second.Keys)-1] != "note" || first.Name != "note" {
			t.Fatalf("target result aliased another result or Name for %q", input)
		}
		publicKeys := second.LastKeys()
		publicKeys[0] = "changed publicly"
		if second.Keys[len(second.Keys)-1] != "note" {
			t.Fatalf("public LastKeys aliases its Target for %q", input)
		}
		link := domain.Link{Target: input}
		one, two := link.Keys(), link.Keys()
		one[0] = "changed link"
		if two[0] != "note" || link.Target != input {
			t.Fatalf("link result aliases another result or input for %q", input)
		}
	}
	unicode, ok := domain.ParseTarget("A/Straße.MD")
	want := domain.Target{Keys: []string{"a", "strasse"}, AltLast: "strasse.md", Name: "Straße"}
	if !ok || !reflect.DeepEqual(unicode, want) {
		t.Fatalf("original spelling and full folding = %#v, %v; want %#v", unicode, ok, want)
	}
}

func TestLinkKeyFilteringKeepsNilAndOwnedEmptyResults(t *testing.T) {
	for _, input := range []string{"", "a//note", "a/../note", "/../note"} {
		if got := (domain.Link{Target: input}).Keys(); got != nil {
			t.Errorf("invalid %q keys = %#v, want nil", input, got)
		}
	}
	tooLong := strings.Repeat("x", domain.MaxKey+1)
	if got := (domain.Link{Target: tooLong}).Keys(); got == nil || len(got) != 0 {
		t.Errorf("valid oversized key = %#v, want nonnil empty", got)
	}
	for _, tc := range []struct {
		stem  int
		forms int
	}{{1021, 2}, {1022, 1}} {
		stem := strings.Repeat("x", tc.stem)
		got := (domain.Link{Target: stem + ".md"}).Keys()
		if len(got) != tc.forms || got[0] != stem || tc.forms == 2 && got[1] != stem+".md" {
			t.Errorf("%d-byte stem .md key lengths/forms differ: count=%d want%d", tc.stem, len(got), tc.forms)
		}
	}
}
