package domain_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// A wikilink to a page is written as its name when no other page has its
// title key, as its path from the root otherwise, and with ".md" after
// that when only then is it read as the page (M6/P4 design 3.1).
func TestALinktextIsTheNameThePathOrThePathWithMd(t *testing.T) {
	tests := []struct {
		name  string
		pages []string
		page  string
		want  string
	}{
		{"a name of its own", []string{"a", "a/x"}, "a/x", "x"},
		{"a name in two folders", []string{"a", "a/x", "b", "b/X"}, "a/x", "a/x"},
		{"a name at the root and in a folder", []string{"x", "a", "a/x"}, "x", "x"},
		{"x.md beside x", []string{"x", "x.md"}, "x.md", "x.md.md"},
		{"x.md in a folder beside x", []string{"a", "a/x", "a/x.md"}, "a/x.md", "a/x.md.md"},
		{"x beside x.md", []string{"x", "x.md"}, "x", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := pagesOf(t, tt.pages, make([][]string, len(tt.pages)))
			got, ok := domain.Linktext(p.byID[p.ids[tt.page]], nil, domain.Tree{Named: p.named})
			if got != tt.want || !ok {
				t.Errorf("got %q, %t; want %q", got, ok, tt.want)
			}
		})
	}
}

// In random trees, many-levelled, with titles alike but for case, with
// ".md" and with aliases, every page's linktext, written from the root as
// the link targets are, leads to it alone from every page (M6/P5 design 6).
func TestALinktextLeadsToItsPageFromEveryPage(t *testing.T) {
	for seed := range uint64(2000) {
		r := rand.New(rand.NewPCG(seed, 6))
		var paths []string
		for len(paths) < 3+r.IntN(12) {
			parent := ""
			if len(paths) > 0 && r.IntN(3) > 0 {
				parent = paths[r.IntN(len(paths))] + "/"
			}
			path := parent + randomTitle(r)
			if !slices.ContainsFunc(paths, func(p string) bool { return siblings(p, path) }) {
				paths = append(paths, path)
			}
		}
		aliases := make([][]string, len(paths))
		for i := range paths {
			if r.IntN(4) == 0 {
				aliases[i] = []string{randomTitle(r)}
			}
		}
		p := pagesOf(t, paths, aliases)
		for _, path := range paths {
			n := p.byID[p.ids[path]]
			text, ok := domain.Linktext(n, nil, domain.Tree{Named: p.named})
			if !ok {
				t.Fatalf("seed %d: no linktext of %q in %q", seed, path, paths)
			}
			for _, from := range paths {
				if got := p.resolve(domain.Link{Kind: "wikilink", Target: text}, from); got.ID != n.ID || got.Ambiguous {
					t.Fatalf("seed %d: %q, the linktext of %q, leads from %q to %+v in %q", seed, text, path, from, got, paths)
				}
			}
		}
	}
}
