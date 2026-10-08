package domain_test

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// The nodes of a read, read once by the ends of their paths (Suffixes),
// resolve a target as the nodes of its last keys read one by one do (M6
// closeout FA-I1): over random trees of pages and attachments, a target to
// one of their nodes or to none, written from a random page in each way a
// node is, or a name alone, resolves to the same node, as ambiguously, as
// none.
func TestSuffixesResolveAsTheCandidatesOfTheLastKeys(t *testing.T) {
	var found, ambiguous, relative, aliased, assets int
	for seed := range uint64(4000) {
		r := rand.New(rand.NewPCG(seed, 11))
		c := randomCase(r)
		p := pagesOf(t, c.nodes(), c.aliases, c.kinds()...)
		all := make([]domain.Node, len(c.nodes()))
		for i, path := range c.nodes() {
			all[i] = p.byID[p.ids[path]]
		}
		for range 20 {
			from := c.pages[r.IntN(len(c.pages))]
			written := randomTarget(r, c.nodes(), from)
			if r.IntN(3) == 0 {
				// A name alone: a page's, an alias, or none's.
				written = randomTitle(r)
			}
			target, ok := domain.ParseTarget(written)
			if !ok {
				continue
			}
			want := p.resolve(domain.Link{Target: written}, from)
			// As deep as the target reaches, no deeper, the pages of its keys alone.
			reach := map[string]int{}
			for _, key := range target.LastKeys() {
				reach[key] = target.Reach(p.paths[from])
			}
			suffixes := domain.NewSuffixes(all, reach)
			if got := suffixes.Resolve(target, p.paths[from], p.aliased); got != want {
				t.Fatalf("seed %d: %q from %q resolves to %+v among the suffixes of %q, to %+v among its candidates",
					seed, written, from, got, c.pages, want)
			}
			if want.ID != (uuid.UUID{}) {
				found++
				if want.Asset {
					assets++
				}
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
	if found < 10_000 || ambiguous < 20 || relative < 1_000 || aliased < 100 || assets < 1_000 {
		t.Errorf("the random targets found %d nodes, %d ambiguously, %d relative, %d by alias, %d attachments: too few to tell",
			found, ambiguous, relative, aliased, assets)
	}
}

// Suffixes read pages only as deep as the targets of their title keys reach:
// names alone among 10,000 pages of distinct titles 64 steps deep read each
// page's last step, not its 64; one target as deep as the paths reads its
// key's pages so deep, not all (M6 closeout FA2-M1: 100,000 such pages
// took 2.2 GB a read; FA3-N2).
func TestSuffixesReadThePathsAsDeepAsTheTargetsReach(t *testing.T) {
	folders := make([]domain.Step, 63)
	for i := range folders {
		folders[i] = domain.Step{ID: uuid.NewV7(), Key: fmt.Sprintf("f%d", i), Name: fmt.Sprintf("F%d", i)}
	}
	pages := make([]domain.Node, 10_000)
	for i := range pages {
		id := uuid.NewV7()
		key := fmt.Sprintf("t%d", i)
		pages[i] = domain.Node{ID: id, Path: append(slices.Clip(folders), domain.Step{ID: id, Key: key, Name: key})}
	}
	target, _ := domain.ParseTarget("t9999")
	reach := make(map[string]int, len(pages))
	for i := range pages {
		reach[fmt.Sprintf("t%d", i)] = 1
	}
	reach["t0"] = 64
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	suffixes := domain.NewSuffixes(pages, reach)
	runtime.ReadMemStats(&after)
	if got := suffixes.Resolve(target, nil, nil); got.ID != pages[9_999].ID {
		t.Fatalf("t9999 resolves to %+v, want %s", got, pages[9_999].ID)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 32<<20 {
		t.Errorf("the suffixes of 10,000 pages read for names alone took %d MiB", allocated>>20)
	}
}
