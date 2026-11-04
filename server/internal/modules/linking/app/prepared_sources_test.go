package app

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// The existing private qualification fixture supplies real old head/metadata
// obligations; only the new port is recorded here. PostgreSQL tests cover its
// native completeness and Close. This test checks the new app phase order.
func TestPreparedSourcesCaptureFallbackBeforePrepared(t *testing.T) {
	a, deleted, b, asset := uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 3}, uuid.UUID{15: 11}
	f := &privateOldFBefore{t: t, notebook: uuid.UUID{15: 50}, nodes: map[uuid.UUID]IngestNodeBefore{}, head: map[uuid.UUID]IngestIndexedHead{}}
	for _, id := range []uuid.UUID{a, deleted, b} {
		privateOldFSource(f, id)
	}
	f.nodes[asset] = IngestNodeBefore{ID: asset, Exists: true, Kind: "asset", InSource: true}
	var events []string
	f.fetch = func(_ context.Context, _ uuid.UUID, ids []uuid.UUID, after *EdgePos, _ int) ([]BodyReference, error) {
		if !slices.Equal(ids, []uuid.UUID{deleted}) || after != nil {
			t.Fatal("fallback source relation changed")
		}
		events = append(events, "fallback-eof")
		return nil, nil
	}
	place := &domain.Place{Name: "source"}
	changes := []domain.Change{{NodeID: b, Before: place, After: place, Revision: 8},
		{NodeID: deleted, Before: place}, {NodeID: a, Before: place, After: place, Revision: 8}}
	factory := &preparedAppFactory{events: &events, assets: []uuid.UUID{asset}}
	u := privateOldFUnit(f, []uuid.UUID{a, deleted, b})
	u.event.Changes = changes
	u.prepared = newSourcePreparationSet(u.event, factory)
	if len(events) != 0 {
		t.Fatal("creating unit preparation performed reads")
	}
	if err := captureForIngest(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	want := []string{"new:1", "new:3", "fallback-eof", "read:1", "read:3"}
	if !slices.Equal(events, want) || f.calls != 1 || len(u.prepared.plans) != 2 {
		t.Fatalf("capture events=%v, want%v", events, want)
	}
	if len(u.bounds.retained) != 3 || u.bounds.retained[0].source != deleted || len(u.bounds.retained[0].assets) != 0 ||
		u.bounds.retained[1].source != a || !slices.Equal(u.bounds.retained[1].assets, []uuid.UUID{asset}) ||
		u.bounds.retained[2].source != b || !slices.Equal(u.bounds.retained[2].assets, []uuid.UUID{asset}) {
		t.Fatalf("complete fallback/prepared F=%+v", u.bounds.retained)
	}
	// A stale Before head cannot reach NewSource, regardless of storage facts.
	f.head[a] = IngestIndexedHead{ID: a, Present: true, PageRevision: 7, IndexedRevision: 6, Extractor: 2}
	events = nil
	u = privateOldFUnit(f, []uuid.UUID{a})
	u.event.Changes = changes[2:]
	u.prepared = newSourcePreparationSet(u.event, factory)
	if err := captureForIngest(t.Context(), u); err != nil || len(events) != 0 || len(u.prepared.plans) != 0 {
		t.Fatalf("stale head was prepared: events%v err%v", events, err)
	}
}

type preparedAppFactory struct {
	events *[]string
	assets []uuid.UUID
}

func (f *preparedAppFactory) NewSource(_ context.Context, _ *SourcePreparationUnit, p Page, _ domain.Facts) PreparedSource {
	*f.events = append(*f.events, "new:"+string(rune('0'+p.ID[15])))
	return &preparedAppPlan{factory: f, source: p.ID}
}
func (f *preparedAppFactory) LinksExcept(context.Context, uuid.UUID, domain.Reach, []uuid.UUID) ([]Link, error) {
	return nil, errors.New("unexpected residual read in capture test")
}

type preparedAppPlan struct {
	factory *preparedAppFactory
	source  uuid.UUID
}

func (p *preparedAppPlan) ReadBefore(context.Context, *SourcePreparationUnit) ([]uuid.UUID, error) {
	*p.factory.events = append(*p.factory.events, "read:"+string(rune('0'+p.source[15])))
	return slices.Clone(p.factory.assets), nil
}
func (p *preparedAppPlan) Apply(context.Context, *SourcePreparationUnit) (Dropped, *SourceLinks, error) {
	return Dropped{}, nil, errors.New("unexpected Apply in capture test")
}

func TestPreparedSourcesDoNotDeduplicateChanges(t *testing.T) {
	id := uuid.UUID{15: 1}
	place := &domain.Place{Name: "x"}
	write := domain.Change{NodeID: id, Before: place, After: place, Revision: 2}
	for _, changes := range [][]domain.Change{
		{write, write}, {write, {NodeID: id, Before: place}}, {{NodeID: id, After: place, Revision: 1}},
	} {
		var events []string
		p := newSourcePreparationSet(PagesChanged{Changes: changes}, &preparedAppFactory{events: &events})
		if p.plan(t.Context(), id) || len(events) != 0 {
			t.Fatal("duplicate/deleted/new source entered preparation")
		}
	}
}

func TestPreparedSourcesMergeCompleteEmptyAndGlobalResidual(t *testing.T) {
	p1, empty, p3, p4 := uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 3}, uuid.UUID{15: 4}
	supplied := []*SourceLinks{
		{SourceID: p1, Links: []Link{{SourceID: p1, Start: 0, Target: "first"}, {SourceID: p1, Start: 99, Target: "tail"}}},
		{SourceID: empty, Links: []Link{}},
		{SourceID: p4, Links: []Link{{SourceID: p4, Start: 7, Target: "written"}}},
	}
	residual := []Link{{SourceID: p3, Start: 3, Target: "global"}}
	want := []Link{{SourceID: p1, Start: 0, Target: "first"}, {SourceID: p1, Start: 99, Target: "tail"},
		{SourceID: p3, Start: 3, Target: "global"}, {SourceID: p4, Start: 7, Target: "written"}}
	got, err := mergeSourceLinks(supplied, residual)
	if err != nil || !reflect.DeepEqual(got, want) || len(got) != cap(got) {
		t.Fatalf("merge=%+v err%v", got, err)
	}
	got, err = mergeSourceLinks([]*SourceLinks{{SourceID: empty}}, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatal("complete empty lost non-nil result")
	}
	for _, repeated := range []uuid.UUID{p1, empty} {
		if got, err := mergeSourceLinks(supplied, []Link{{SourceID: repeated, Start: 77}}); err == nil || got != nil {
			t.Fatal("global OR reintroduced supplied source")
		}
	}
	if got, err := mergeSourceLinks([]*SourceLinks{{SourceID: p1, Links: []Link{{SourceID: p1}, {SourceID: p1}}}}, nil); err == nil || got != nil {
		t.Fatal("duplicate range was silently removed")
	}
}
