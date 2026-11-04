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

func TestIngestAssetCacheExactKeys(t *testing.T) {
	a, b, c := uuid.UUID{15: 11}, uuid.UUID{15: 12}, uuid.UUID{15: 13}
	var cache ingestAssetCache
	calls := 0
	for i, tc := range []struct {
		raw, want []uuid.UUID
		calls     int
	}{
		{nil, nil, 1}, {nil, nil, 1},
		{[]uuid.UUID{}, []uuid.UUID{}, 2}, {[]uuid.UUID{}, []uuid.UUID{}, 2},
		{[]uuid.UUID{b, a, a}, []uuid.UUID{a, b}, 3},
		{[]uuid.UUID{b, a, a}, []uuid.UUID{a, b}, 3},
		{[]uuid.UUID{a, b, a}, []uuid.UUID{a, b}, 4}, // Same set, different original order.
		{[]uuid.UUID{a, b, a}, []uuid.UUID{a, b}, 4},
		{[]uuid.UUID{a, b, c}, []uuid.UUID{a, b, c}, 5}, // Same prefix, distinct tail.
		{[]uuid.UUID{a, b}, []uuid.UUID{a, b}, 6},
		{[]uuid.UUID{{}, a}, []uuid.UUID{{}, a}, 7},
		{[]uuid.UUID{{}, a}, []uuid.UUID{{}, a}, 7},
	} {
		raw := slices.Clone(tc.raw)
		got, err := cache.qualify(raw, func(ids []uuid.UUID) ([]uuid.UUID, error) {
			calls++
			if !reflect.DeepEqual(ids, tc.raw) {
				t.Fatal("qualification did not receive the complete original input")
			}
			if len(ids) > 0 {
				ids[0] = uuid.UUID{15: 250}
			} // Real qualification may reorder/filter in place.
			return slices.Clone(tc.want), nil
		})
		if err != nil || calls != tc.calls || !reflect.DeepEqual(got, tc.want) || len(got) != cap(got) {
			t.Fatalf("CACHE_EXACT: case%d calls%d want%d result%v want%v err%v", i, calls, tc.calls, got, tc.want, err)
		}
		if len(got) > 0 {
			got[0] = uuid.UUID{15: 251}
		} // A callback cannot rewrite the qualified snapshot.
	}
}

func TestIngestAssetCacheOwnsSnapshotsAndCurrentBuffer(t *testing.T) {
	a, b := uuid.UUID{15: 11}, uuid.UUID{15: 12}
	var cache ingestAssetCache
	calls := 0
	qualify := func(ids []uuid.UUID) ([]uuid.UUID, error) {
		calls++
		ids[0], ids[1] = a, b
		return ids, nil
	}
	first := []uuid.UUID{b, a}
	got, err := cache.qualify(first, qualify)
	if err != nil || !slices.Equal(got, []uuid.UUID{a, b}) {
		t.Fatal("first literal qualification")
	}
	first[0] = uuid.UUID{15: 240}
	for range 2 {
		current := []uuid.UUID{b, a}
		got, err = cache.qualify(current, qualify)
		if err != nil || calls != 1 || !slices.Equal(got, []uuid.UUID{a, b}) || &got[0] != &current[0] {
			t.Fatal("CACHE_OWNER: hit borrowed a snapshot/previous output or repeated qualification")
		}
		got[0] = uuid.UUID{15: 241}
	}
	var disabled *ingestAssetCache
	for range 2 {
		if _, err := disabled.qualify([]uuid.UUID{b, a}, qualify); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Fatal("PUBLIC_NO_CACHE: nil cache reused qualification")
	}
}

func TestIngestAssetCacheBoundAndQualificationFailure(t *testing.T) {
	a, b := uuid.UUID{15: 11}, uuid.UUID{15: 12}
	var cache ingestAssetCache
	calls := 0
	identity := func(ids []uuid.UUID) ([]uuid.UUID, error) { calls++; return ids, nil }
	small := make([]uuid.UUID, 10000)
	for i := range small {
		small[i] = a
	}
	large := append(slices.Clone(small), b)
	for i, tc := range []struct {
		raw   []uuid.UUID
		calls int
	}{
		{small, 1}, {small, 1}, {large, 2}, {small, 3},
	} {
		got, err := cache.qualify(slices.Clone(tc.raw), identity)
		if err != nil || calls != tc.calls || !slices.Equal(got, tc.raw) {
			t.Fatalf("CACHE_BOUND: case%d truncated/retained the over-bound input: calls%d len%d err%v", i, calls, len(got), err)
		}
		if i != 2 && (len(cache.raw) != 10000 || cap(cache.raw) != 10000 || len(cache.qualified) != 10000 || cap(cache.qualified) != 10000) {
			t.Fatal("CACHE_BOUND: snapshots exceed the exact 10000-ID limit")
		}
		if i == 2 && (cache.ready || cache.raw != nil || cache.qualified != nil) {
			t.Fatal("CACHE_BOUND: long input retained cache snapshots")
		}
	}
	cause := errors.New("first metadata/head qualification failed")
	for range 2 {
		got, err := cache.qualify([]uuid.UUID{b}, func([]uuid.UUID) ([]uuid.UUID, error) { calls++; return nil, cause })
		//nolint:errorlint // Qualification must return the exact callback error object.
		if err != cause || got != nil || cache.ready || cache.raw != nil || cache.qualified != nil {
			t.Fatal("CACHE_ERROR: failed qualification cached/changed its cause")
		}
	}
	if calls != 5 {
		t.Fatal("failed qualification was reused")
	}
	if _, err := cache.qualify(slices.Clone(small), identity); err != nil || calls != 6 {
		t.Fatal("stale cache survived failure")
	}
}

// Only the port result is simulated here. Existing native prepared/River tests
// retain actual Scan/Err/EOF/Close faults; this test verifies their app ordering.
type assetCachePlans struct {
	preparedAppFactory
	fail  uuid.UUID
	cause error
}

func (f *assetCachePlans) NewSource(ctx context.Context, unit *SourcePreparationUnit, p Page, facts domain.Facts) PreparedSource {
	return assetCachePlan{PreparedSource: f.preparedAppFactory.NewSource(ctx, unit, p, facts), factory: f, source: p.ID}
}

type assetCachePlan struct {
	PreparedSource
	factory *assetCachePlans
	source  uuid.UUID
}

func (p assetCachePlan) ReadBefore(ctx context.Context, unit *SourcePreparationUnit) ([]uuid.UUID, error) {
	assets, err := p.PreparedSource.ReadBefore(ctx, unit)
	if p.source == p.factory.fail {
		return nil, p.factory.cause
	}
	return assets, err
}

type assetCachePublicBodies struct {
	*privateOldFBefore
	sources   []uuid.UUID
	rows      []BodyReference
	bodyCalls int
}

func (p *assetCachePublicBodies) BodyReferences(_ context.Context, nb uuid.UUID, ids []uuid.UUID, after *EdgePos, take int) ([]BodyReference, error) {
	p.bodyCalls++
	if nb != p.notebook || take != 512 || !slices.Equal(ids, p.sources) {
		p.t.Fatal("PUBLIC_512: public body contract changed")
	}
	if p.bodyCalls == 1 && after == nil {
		return p.rows, nil
	}
	if p.bodyCalls != 2 || after == nil || *after != p.rows[len(p.rows)-1].EdgePos {
		p.t.Fatal("PUBLIC_EOF: public cursor/empty read changed")
	}
	return nil, nil
}

func TestIngestAssetCacheCaptureSemantics(t *testing.T) {
	for _, mode := range []string{"healthy", "metadata", "head", "second_native", "callback", "public"} {
		t.Run(mode, func(t *testing.T) {
			sources := []uuid.UUID{{15: 1}, {15: 2}}
			a, b, page, absent, outside, root := uuid.UUID{15: 11}, uuid.UUID{15: 12}, uuid.UUID{15: 13}, uuid.UUID{15: 14}, uuid.UUID{15: 15}, uuid.UUID{15: 16}
			f := &privateOldFBefore{t: t, notebook: uuid.UUID{15: 50}, nodes: map[uuid.UUID]IngestNodeBefore{}, head: map[uuid.UUID]IngestIndexedHead{}}
			for _, source := range sources {
				privateOldFSource(f, source)
			}
			n := f.nodes[sources[1]]
			n.PageBytesKnown = false
			f.nodes[sources[1]] = n
			for _, id := range []uuid.UUID{{}, a, b, outside, root} {
				f.nodes[id] = IngestNodeBefore{ID: id, Exists: true, Kind: "asset", InSource: true}
			}
			n = f.nodes[outside]
			n.InSource = false
			f.nodes[outside] = n
			n = f.nodes[root]
			n.IsSourceRoot = true
			f.nodes[root] = n
			f.nodes[absent] = IngestNodeBefore{ID: absent}
			f.nodes[page] = IngestNodeBefore{ID: page, Exists: true, Kind: "page", InSource: true, PageBytesKnown: true, Revision: 3, RevisionKnown: true}
			f.head[page] = IngestIndexedHead{ID: page, Present: true, PageRevision: 3, IndexedRevision: 3, Extractor: 2}
			cause := errors.New(mode + " designated original cause")
			if mode == "metadata" {
				f.metadataID, f.metadataErr = a, cause
			}
			if mode == "head" {
				f.headErr = cause
			}
			assets := []uuid.UUID{b, page, a, {}, outside, root, absent, a}
			var events []string
			plans := &assetCachePlans{preparedAppFactory: preparedAppFactory{events: &events, assets: assets}}
			if mode == "second_native" {
				plans.fail, plans.cause = sources[1], cause
			}
			f.fetch = func(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]BodyReference, error) {
				t.Fatal("unexpected private fallback")
				return nil, nil
			}
			u := privateOldFUnit(f, sources)
			place := &domain.Place{Name: "source"}
			for _, source := range sources {
				u.event.Changes = append(u.event.Changes, domain.Change{NodeID: source, Before: place, After: place, Revision: 8})
			}
			prepared := newSourcePreparationSet(u.event, plans)
			var got []OldSourceFacts
			followed := func(value OldSourceFacts) error {
				copyOf := value
				copyOf.AssetIDs, copyOf.Unknown.PossibleTargets = slices.Clone(value.AssetIDs), slices.Clone(value.Unknown.PossibleTargets)
				got = append(got, copyOf)
				if len(value.AssetIDs) != cap(value.AssetIDs) {
					t.Fatal("F result retains excess input capacity")
				}
				if len(value.AssetIDs) > 0 {
					value.AssetIDs[0] = uuid.UUID{15: 250}
				}
				if mode == "callback" {
					return cause
				}
				return nil
			}
			q := QualifiedBefore{Heads: f, Reads: u.deps.Reads}
			var public *assetCachePublicBodies
			var err error
			if mode == "public" {
				public = &assetCachePublicBodies{privateOldFBefore: f, sources: sources}
				for _, source := range sources {
					for i, id := range assets {
						public.rows = append(public.rows, BodyReference{EdgePos: EdgePos{SourceID: source, Start: int32(i)}, ResolvedID: id, ResolvedAsset: true})
					}
				}
				q.Reads = NewLintReads(public, 5<<20)
				err = q.Capture(t.Context(), f.notebook, f, sources, func(OldQualifiedFacts) error { return nil }, followed)
			} else {
				err = q.capture(t.Context(), f.notebook, f, sources, func(OldQualifiedFacts) error { return nil }, followed, f, prepared)
			}
			want := []OldSourceFacts{{PageID: sources[0], State: OldFactsKnown, AssetIDs: []uuid.UUID{{}, a, b}},
				{PageID: sources[1], State: OldFactsUnknown, AssetIDs: []uuid.UUID{{}, a, b}, Unknown: OldUncertainty{Extent: OldUnknownPossibleTargets, PossibleTargets: []uuid.UUID{{}, a, b}}}}
			wantEvents := []string{"new:1", "new:2", "read:1", "read:2"}
			if mode == "metadata" || mode == "head" {
				want = nil
				wantEvents = wantEvents[:3]
			}
			if mode == "second_native" || mode == "callback" {
				want = want[:1]
			}
			if mode == "callback" {
				wantEvents = wantEvents[:3]
			}
			wantErr := cause
			if mode == "healthy" || mode == "public" {
				wantErr = nil
			}
			if mode == "public" {
				wantEvents = nil
			}
			//nolint:errorlint // Capture must return the exact metadata, native, or callback error object.
			if err != wantErr || !reflect.DeepEqual(got, want) || !slices.Equal(events, wantEvents) || f.calls != 0 || len(f.loaded) != 2 || !slices.Equal(f.loaded[1], []uuid.UUID{{}, a, b, page, absent, outside, root}) {
				t.Fatalf("CAPTURE_CACHE: mode%s err%v want%v events%v F%+v loaded%v", mode, err, wantErr, events, got, f.loaded)
			}
			if mode != "metadata" && (len(f.headRequests) != 2 || !reflect.DeepEqual(f.headRequests[1], []IngestRevision{{ID: page, Revision: 3}})) {
				t.Fatal("asset-flagged page head obligation was skipped")
			}
			if public != nil && public.bodyCalls != 2 {
				t.Fatal("public full stream/EOF changed")
			}
		})
	}
}
