package app

import (
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Each raw target first reaches a shallow source. Freezing that first reach
// loses the deep source, even when the raw forms share a key or an alternate.
// A missing first source must not suppress the later valid source's reach.
func TestResolverReachKeepsSharedRawAcrossMissingAndDeeperSources(t *testing.T) {
	a := []domain.Step{{ID: resolverCacheID(10), Key: "a", Name: "A"}}
	b := []domain.Step{{ID: resolverCacheID(11), Key: "b", Name: "B"}, {ID: resolverCacheID(12), Key: "mid", Name: "Mid"}, {ID: resolverCacheID(13), Key: "inner", Name: "Inner"}}
	w := &resolverCacheWorld{
		candidates: []domain.Node{resolverCacheNode(50, a, "x"), resolverCacheNode(51, b, "x")},
		nodes:      []domain.Node{resolverCacheNode(0, a, "zero"), resolverCacheNode(1, b, "one")},
	}
	links := []Link{
		{SourceID: resolverCacheID(9), Target: "./x"},
		{SourceID: resolverCacheID(0), Target: "./x"},
		{SourceID: resolverCacheID(0), Target: "./X"},
		{SourceID: resolverCacheID(0), Target: "./X.MD"},
		{SourceID: resolverCacheID(0), Target: "../inner/x"},
		{SourceID: resolverCacheID(1), Target: "./x"},
		{SourceID: resolverCacheID(1), Target: "./X"},
		{SourceID: resolverCacheID(1), Target: "./X.MD"},
		{SourceID: resolverCacheID(1), Target: "../inner/x"},
		{SourceID: resolverCacheID(0), Target: "./x"},
		{SourceID: resolverCacheID(1), Target: "./X.MD"},
	}
	for i := range links {
		links[i].Start = i
		links[i].Resolution = domain.Resolution{ID: resolverCacheID(99), Asset: true, Ambiguous: true}
	}
	before := slices.Clone(links)
	got, missing, err := resolutions(t.Context(), w, w, resolverCacheID(100), links, nil, nil)
	want := []domain.Resolution{{}, {ID: resolverCacheID(50)}, {ID: resolverCacheID(50)},
		{ID: resolverCacheID(50)}, {}, {ID: resolverCacheID(51)}, {ID: resolverCacheID(51)},
		{ID: resolverCacheID(51)}, {ID: resolverCacheID(51)}, {ID: resolverCacheID(50)}, {ID: resolverCacheID(51)}}
	if err != nil || len(got) != len(want) || !slices.Equal(missing, []uuid.UUID{resolverCacheID(9)}) {
		t.Fatalf("REACH_SHARED_RAW: len=%d missing=%v err=%v", len(got), missing, err)
	}
	for i, r := range want {
		if got[i] != r {
			t.Fatalf("REACH_SHARED_RAW: row%d=%+v want%+v", i, got[i], r)
		}
	}
	if !slices.Equal(w.ids, []uuid.UUID{resolverCacheID(0), resolverCacheID(1), resolverCacheID(9)}) ||
		!slices.Equal(w.keys, []string{"x", "x.md"}) || !slices.Equal(w.calls, []string{"keys", "aliases", "paths"}) {
		t.Fatalf("REACH_SHARED_INPUTS: ids=%v keys=%v calls=%v", w.ids, w.keys, w.calls)
	}
	if !slices.Equal(links, before) {
		t.Fatal("REACH_SHARED_OWNERSHIP: original link resolutions changed")
	}
}
