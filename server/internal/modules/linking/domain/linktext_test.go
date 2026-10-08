package domain_test

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// A wikilink to a page is written as its name when no other page has its
// title key, as its path from the root otherwise, and with ".md" after
// that when only then is it read as the page (M6/P4 design 3.1): a page
// whose title an attachment has (M7/P3 design 4.4). An attachment's is its
// name when no other attachment has its title key, a page's no matter; an
// attachment without an extension, which nothing leads to, has none.
func TestALinktextIsTheNameThePathOrThePathWithMd(t *testing.T) {
	tests := []struct {
		name   string
		pages  []string
		page   string
		want   string
		assets []string
	}{
		{"a name of its own", []string{"a", "a/x"}, "a/x", "x", nil},
		{"a name in two folders", []string{"a", "a/x", "b", "b/X"}, "a/x", "a/x", nil},
		{"a name at the root and in a folder", []string{"x", "a", "a/x"}, "x", "x", nil},
		{"x.md beside x", []string{"x", "x.md"}, "x.md", "x.md.md", nil},
		{"x.md in a folder beside x", []string{"a", "a/x", "a/x.md"}, "a/x.md", "a/x.md.md", nil},
		{"x beside x.md", []string{"x", "x.md"}, "x", "x", nil},
		{"a page shadowed by an attachment", []string{"a", "b", "b/x.png"}, "b/x.png", "b/x.png.md", []string{"a/x.png"}},
		{"a page at the root shadowed", []string{"a", "x.png"}, "x.png", "x.png.md", []string{"a/x.png"}},
		{"an attachment of its own name", []string{"a", "b", "b/x.png"}, "a/x.png", "x.png", []string{"a/x.png"}},
		{"an attachment of a name in two folders", []string{"a", "b"}, "a/x.png", "a/x.png", []string{"a/x.png", "b/X.PNG"}},
		{"an attachment without an extension", []string{"a"}, "a/x", "", []string{"a/x"}},
		// Written with ".md", read as a page's: x.png.md, no page x.png being
		// there, is the page titled so, which the attachment x.png is not.
		{"x.png.md beside an attachment x.png", []string{"a", "x.png.md"}, "x.png.md", "x.png.md", []string{"a/x.png"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := slices.Concat(tt.pages, tt.assets)
			asset := make([]bool, len(nodes))
			for i := range tt.assets {
				asset[len(tt.pages)+i] = true
			}
			p := pagesOf(t, nodes, make([][]string, len(nodes)), asset...)
			got, ok := domain.Linktext(p.byID[p.ids[tt.page]], nil, domain.Tree{Named: p.named})
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("got %q, %t; want %q", got, ok, tt.want)
			}
		})
	}
}

// Linktexts are each node's linktext, an attachment without an extension's
// its path from the root, which nothing leads to.
func TestLinktextsAreEachNodesFromTheRoot(t *testing.T) {
	p := pagesOf(t, []string{"a", "b", "b/x.png", "a/x.png", "a/data"}, make([][]string, 5), false, false, false, true, true)
	nodes := []domain.Node{p.byID[p.ids["b/x.png"]], p.byID[p.ids["a/x.png"]], p.byID[p.ids["a/data"]], p.byID[p.ids["a"]]}
	if got, want := domain.Linktexts(nodes), []string{"b/x.png.md", "x.png", "a/data", "a"}; !slices.Equal(got, want) {
		t.Errorf("Linktexts = %q, want %q", got, want)
	}
}

// In random trees, many-levelled, with titles alike but for case, with
// ".md" and with aliases, and attachments, some of the pages' titles, every
// page's and every attachment's with an extension linktext, written from the
// root as the link targets are, leads to it alone from every page (M6/P5
// design 6; M7/P3 design 4.5).
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
		pages := len(paths)
		for range r.IntN(5) {
			parent := ""
			if r.IntN(3) > 0 {
				parent = paths[r.IntN(pages)] + "/"
			}
			path := parent + randomAssetName(r)
			if !slices.ContainsFunc(paths, func(p string) bool { return siblings(p, path) }) {
				paths = append(paths, path)
			}
		}
		aliases := make([][]string, len(paths))
		asset := make([]bool, len(paths))
		for i := range paths {
			if i >= pages {
				asset[i] = true
			} else if r.IntN(4) == 0 {
				aliases[i] = []string{randomTitle(r)}
			}
		}
		p := pagesOf(t, paths, aliases, asset...)
		for i, path := range paths {
			n := p.byID[p.ids[path]]
			text, ok := domain.Linktext(n, nil, domain.Tree{Named: p.named})
			if !ok {
				if asset[i] && !strings.Contains(lastOf(path), ".") {
					continue
				}
				t.Fatalf("seed %d: no linktext of %q in %q", seed, path, paths)
			}
			for _, from := range paths[:pages] {
				if got := p.resolve(domain.Link{Kind: "wikilink", Target: text}, from); got.ID != n.ID || got.Ambiguous {
					t.Fatalf("seed %d: %q, the linktext of %q, leads from %q to %+v in %q", seed, text, path, from, got, paths)
				}
			}
		}
	}
}
