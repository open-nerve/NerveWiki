package app

import (
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// The old implementation must pass too. In one real resolution call, a source
// returns after a different folder, and relative x needs a deeper suffix than
// the first source needed. Results do not come from a second resolver run.
func TestResolverSourceBoundariesKeepUnorderedReturns(t *testing.T) {
	a := []domain.Step{{ID: resolverCacheID(10), Key: "a", Name: "A"}}
	b := []domain.Step{{ID: resolverCacheID(11), Key: "b", Name: "B"}, {ID: resolverCacheID(12), Key: "mid", Name: "Mid"}, {ID: resolverCacheID(13), Key: "inner", Name: "Inner"}}
	w := &resolverCacheWorld{
		candidates: []domain.Node{resolverCacheNode(50, a, "x"), resolverCacheNode(51, b, "x")},
		nodes:      []domain.Node{resolverCacheNode(0, a, "zero"), resolverCacheNode(1, b, "one"), resolverCacheNode(2, b, "two")},
	}
	links := []Link{
		{SourceID: resolverCacheID(0), Target: "./x"},
		{SourceID: resolverCacheID(0), Target: "./X"},
		{SourceID: resolverCacheID(1), Target: "./x"},
		{SourceID: resolverCacheID(1), Target: "./X"},
		{SourceID: resolverCacheID(2), Target: "./x"},
		{SourceID: resolverCacheID(0), Target: "./x"},
		{SourceID: resolverCacheID(2), Target: "./x"},
	}
	for i := range links {
		links[i].Start = i
		links[i].Resolution = domain.Resolution{ID: resolverCacheID(99), Asset: true, Ambiguous: true}
	}
	before := slices.Clone(links)
	got, missing, err := resolutions(t.Context(), w, w, resolverCacheID(100), links, nil, nil)
	want := []domain.Resolution{{ID: resolverCacheID(50)}, {ID: resolverCacheID(50)},
		{ID: resolverCacheID(51)}, {ID: resolverCacheID(51)}, {ID: resolverCacheID(51)},
		{ID: resolverCacheID(50)}, {ID: resolverCacheID(51)}}
	if err != nil || len(missing) != 0 || len(got) != len(want) {
		t.Fatalf("SOURCE_BOUNDARY_UNORDERED: len=%d missing=%v err=%v", len(got), missing, err)
	}
	for i, r := range want {
		if got[i] != r {
			t.Fatalf("SOURCE_BOUNDARY_UNORDERED: row%d=%+v want%+v", i, got[i], r)
		}
	}
	if !slices.Equal(w.ids, []uuid.UUID{resolverCacheID(0), resolverCacheID(1), resolverCacheID(2)}) ||
		!slices.Equal(w.keys, []string{"x"}) || !slices.Equal(w.calls, []string{"keys", "aliases", "paths"}) {
		t.Fatalf("SOURCE_BOUNDARY_INPUTS: ids=%v keys=%v calls=%v", w.ids, w.keys, w.calls)
	}
	if !slices.Equal(links, before) {
		t.Fatal("SOURCE_BOUNDARY_OWNERSHIP: original link resolutions changed")
	}
}
