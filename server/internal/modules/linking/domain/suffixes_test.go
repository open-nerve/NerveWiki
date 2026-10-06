package domain_test

import (
	"math/rand/v2"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// The pages of a read, read once by the ends of their paths (Suffixes),
// resolve a target as the pages of its last keys read one by one do (M6
// closeout FA-I1): over random trees, a target to one of their pages or to
// none, written from a random page in each way a page is, or a name alone,
// resolves to the same page, as ambiguously, as none.
func TestSuffixesResolveAsTheCandidatesOfTheLastKeys(t *testing.T) {
	var found, ambiguous, relative, aliased int
	for seed := range uint64(4000) {
		r := rand.New(rand.NewPCG(seed, 11))
		c := randomCase(r)
		p := pagesOf(t, c.pages, c.aliases)
		all := make([]domain.Node, len(c.pages))
		for i, path := range c.pages {
			all[i] = p.byID[p.ids[path]]
		}
		suffixes := domain.NewSuffixes(all)
		for range 20 {
			from := c.pages[r.IntN(len(c.pages))]
			written := randomTarget(r, c.pages, from)
			if r.IntN(3) == 0 {
				// A name alone: a page's, an alias, or none's.
				written = randomTitle(r)
			}
			target, ok := domain.ParseTarget(written)
			if !ok {
				continue
			}
			want := p.resolve(domain.Link{Target: written}, from)
			if got := suffixes.Resolve(target, p.paths[from], p.aliased); got != want {
				t.Fatalf("seed %d: %q from %q resolves to %+v among the suffixes of %q, to %+v among its candidates",
					seed, written, from, got, c.pages, want)
			}
			if want.ID != (uuid.UUID{}) {
				found++
				switch {
				case want.Ambiguous:
					ambiguous++
				case target.Relative:
					relative++
				case !slices.ContainsFunc(p.named[target.LastKeys()[0]], func(n domain.Node) bool { return n.ID == want.ID }) &&
					(len(target.LastKeys()) < 2 || !slices.ContainsFunc(p.named[target.LastKeys()[1]], func(n domain.Node) bool { return n.ID == want.ID })):
					aliased++
				}
			}
		}
	}
	if found < 10_000 || ambiguous < 20 || relative < 1_000 || aliased < 100 {
		t.Errorf("the random targets found %d pages, %d ambiguously, %d relative, %d by alias: too few to tell", found, ambiguous, relative, aliased)
	}
}
