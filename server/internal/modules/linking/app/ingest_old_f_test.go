package app

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"uuid"
)

type privateOldFFetch func(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]BodyReference, error)

func (f privateOldFFetch) OldBodyReferences(ctx context.Context, nb uuid.UUID, ids []uuid.UUID, after *EdgePos, take int) ([]BodyReference, error) {
	return f(ctx, nb, ids, after, take)
}

func privateOldFMany(n int) []BodyReference {
	out := make([]BodyReference, n)
	for i := range out {
		source, target := uuid.UUID{}, uuid.UUID{15: 11}
		if i >= 10000 {
			source = uuid.UUID{15: 2}
			target = uuid.UUID{15: 12}
		}
		out[i] = BodyReference{EdgePos: EdgePos{SourceID: source, Start: int32(2 * (i % 10000))}, ResolvedID: target, ResolvedAsset: i%2 == 0}
	}
	return out
}

func TestIngestOldBodyBatchesOwnCursorAndPrefixes(t *testing.T) {
	book := uuid.UUID{15: 50}
	ids := []uuid.UUID{{}, {15: 2}}
	originalIDs := slices.Clone(ids)
	raw := privateOldFMany(16385)
	want := slices.Clone(raw)
	calls, offset := 0, 0
	var held []*EdgePos
	var snapshots []EdgePos
	fetch := func(ctx context.Context, nb uuid.UUID, gotIDs []uuid.UUID, after *EdgePos, take int) ([]BodyReference, error) {
		calls++
		if ctx.Err() != nil || nb != book || !slices.Equal(gotIDs, originalIDs) || take != 16384 {
			t.Fatal("private query relation/context/raw limit")
		}
		if offset == 0 {
			if after != nil {
				t.Fatal("first private cursor is not nil")
			}
		} else {
			if after == nil || *after != want[offset-1].EdgePos {
				t.Fatalf("owned query cursor=%+v offset%d", after, offset)
			}
			held = append(held, after)
			snapshots = append(snapshots, *after)
		}
		n := min(16384, len(raw)-offset)
		out := raw[offset : offset+n]
		offset += n
		return out, nil
	}
	stream := ingestOldBodyBatches(book, ids, fetch)
	ids[0] = uuid.UUID{15: 99} // The factory owns the complete emitter relation.
	position := 0
	for prefix := range 34 {
		got, err := stream.Next(t.Context())
		n := min(512, len(want)-position)
		if err != nil || len(got) != n || cap(got) != n || !slices.Equal(got, want[position:position+n]) {
			t.Fatalf("prefix%d=%+v err%v want%d at%d", prefix, got, err, n, position)
		}
		if prefix < 32 && calls != 1 || prefix == 32 && calls != 2 || prefix == 33 && calls != 3 {
			t.Fatalf("prefetch calls=%d at prefix%d", calls, prefix)
		}
		position += n
		if n > 0 {
			// Returned prefixes cannot append into future buffered rows, and a
			// returned position cannot mutate the iterator's saved cursor.
			grown := append(got, BodyReference{EdgePos: EdgePos{Start: -9}})
			grown[len(grown)-1].Start = -10
			got[n-1].SourceID = uuid.UUID{15: 250}
			got[n-1].Start = -7
		}
	}
	if position != 16385 || calls != 3 || len(held) != 2 {
		t.Fatal("incomplete raw stream/empty EOF")
	}
	for i, p := range held {
		if *p != snapshots[i] {
			t.Fatal("query cursor aliases mutable iterator state")
		}
	}
	got, err := stream.Next(t.Context())
	if err != nil || got != nil || calls != 3 {
		t.Fatal("terminal stream reissued SQL")
	}
}

func TestIngestOldBodyBatchesPreservePrefixErrors(t *testing.T) {
	book := uuid.UUID{15: 50}
	ids := []uuid.UUID{{}}
	for _, mode := range []string{"cancel_buffered", "negative_later_prefix", "negative_nonasset_prefix", "duplicate_later_prefix", "raw_overflow", "native_error"} {
		t.Run(mode, func(t *testing.T) {
			raw := privateOldFMany(1025)
			calls := 0
			native := errors.New("adapter native failure")
			if mode == "negative_later_prefix" {
				raw[700].Start = -1
			}
			if mode == "negative_nonasset_prefix" {
				raw[701].Start = -1
			}
			if mode == "duplicate_later_prefix" {
				raw[700].EdgePos = raw[699].EdgePos
			}
			if mode == "raw_overflow" {
				raw = privateOldFMany(16385)
			}
			stream := ingestOldBodyBatches(book, ids, func(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]BodyReference, error) {
				calls++
				if mode == "native_error" {
					return nil, native
				}
				return raw, nil
			})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			got, err := stream.Next(ctx)
			if mode == "raw_overflow" || mode == "native_error" {
				if err == nil || got != nil {
					t.Fatal("invalid native batch accepted")
				}
				//nolint:errorlint // The stream must return the exact native error object.
				if mode == "native_error" && err != native {
					t.Fatal("adapter error identity changed")
				}
			} else {
				if err != nil || len(got) != 512 {
					t.Fatalf("later buffered error overtook first prefix: %v", err)
				}
				if mode == "cancel_buffered" {
					cancel()
				}
				got, err = stream.Next(ctx)
				if got != nil || err == nil || mode == "cancel_buffered" && !errors.Is(err, context.Canceled) {
					t.Fatal("buffered prefix did not preserve cancellation/row validation")
				}
			}
			first := err
			got, err = stream.Next(t.Context())
			//nolint:errorlint // Later Next calls must preserve the exact terminal error object.
			if got != nil || err != first || calls != 1 {
				t.Fatal("terminal prefix error retried/changed")
			}
		})
	}
}

func TestIngestPrivateBodyAssetsKeepSourceOrderAndOwnership(t *testing.T) {
	book := uuid.UUID{15: 50}
	ids := []uuid.UUID{{}, {15: 1}, {15: 2}}
	raw := privateOldFMany(16385)
	offset, calls := 0, 0
	var emitted []uuid.UUID
	var retained [][]uuid.UUID
	var callsAtEmit []int
	fetch := privateOldFFetch(func(_ context.Context, nb uuid.UUID, gotIDs []uuid.UUID, after *EdgePos, take int) ([]BodyReference, error) {
		calls++
		if nb != book || !slices.Equal(gotIDs, ids) || take != 16384 || offset > 0 && (after == nil || *after != raw[offset-1].EdgePos) {
			t.Fatal("one complete source relation/cursor lost")
		}
		n := min(take, len(raw)-offset)
		batch := raw[offset : offset+n]
		offset += n
		return batch, nil
	})
	err := referenceIngestBodyAssets(t.Context(), book, ids, fetch, func(id uuid.UUID, assets []uuid.UUID) error {
		emitted = append(emitted, id)
		retained = append(retained, assets)
		callsAtEmit = append(callsAtEmit, calls)
		return nil
	})
	if err != nil || !slices.Equal(emitted, ids) || !slices.Equal(callsAtEmit, []int{1, 1, 3}) || calls != 3 || offset != 16385 {
		t.Fatalf("source/EOF sequence %v/%v calls%d offset%d %v", emitted, callsAtEmit, calls, offset, err)
	}
	for source, n := range []int{5000, 0, 3193} {
		if len(retained[source]) != n || retained[source] == nil {
			t.Fatalf("complete asset-flag stream for source%d: %d", source, len(retained[source]))
		}
		want := uuid.UUID{15: 11}
		if source == 2 {
			want = uuid.UUID{15: 12}
		}
		for _, id := range retained[source] {
			if id != want {
				t.Fatal("source result aliased a later source or retained a nonasset row")
			}
		}
	}
	for _, outside := range []struct {
		name string
		ids  []uuid.UUID
		raw  []BodyReference
	}{
		{"before", []uuid.UUID{{15: 1}}, raw[:1]},
		{"after", []uuid.UUID{{}}, raw[10000:10001]},
	} {
		t.Run(outside.name, func(t *testing.T) {
			reads := 0
			f := privateOldFFetch(func(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]BodyReference, error) {
				reads++
				if reads == 1 {
					return outside.raw, nil
				}
				return nil, nil
			})
			if err := referenceIngestBodyAssets(t.Context(), book, outside.ids, f, func(uuid.UUID, []uuid.UUID) error { return nil }); err == nil {
				t.Fatal("body row outside requested source relation accepted")
			}
		})
	}
}

type privateOldFBefore struct {
	LintReadStore
	t                    *testing.T
	notebook             uuid.UUID
	nodes                map[uuid.UUID]IngestNodeBefore
	head                 map[uuid.UUID]IngestIndexedHead
	fetch                privateOldFFetch
	metadataID           uuid.UUID
	metadataErr, headErr error
	loaded               [][]uuid.UUID
	headRequests         [][]IngestRevision
	calls                int
}

func (f *privateOldFBefore) NodesBefore(_ context.Context, ids []uuid.UUID) ([]IngestNodeBefore, error) {
	f.loaded = append(f.loaded, slices.Clone(ids))
	if len(ids) > 512 {
		f.t.Fatal("metadata batch widened")
	}
	out := make([]IngestNodeBefore, len(ids))
	for i, id := range ids {
		if f.metadataErr != nil && id == f.metadataID {
			return nil, f.metadataErr
		}
		var ok bool
		out[i], ok = f.nodes[id]
		if !ok {
			f.t.Fatalf("unexpected metadata ID %s", id)
		}
	}
	return out, nil
}
func (f *privateOldFBefore) IndexedHeads(_ context.Context, nb uuid.UUID, ids []IngestRevision) ([]IngestIndexedHead, error) {
	f.headRequests = append(f.headRequests, slices.Clone(ids))
	if nb != f.notebook || len(ids) > 512 {
		f.t.Fatal("head relation/bound changed")
	}
	out := make([]IngestIndexedHead, len(ids))
	for i, id := range ids {
		if f.headErr != nil && id.ID == (uuid.UUID{15: 13}) {
			return nil, f.headErr
		}
		var ok bool
		out[i], ok = f.head[id.ID]
		if !ok || out[i].PageRevision != id.Revision {
			f.t.Fatalf("unexpected target head request %+v", id)
		}
	}
	return out, nil
}
func (f *privateOldFBefore) BodyReferences(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]BodyReference, error) {
	return nil, errors.New("public body reader reached by private capture")
}
func (f *privateOldFBefore) OldBodyReferences(ctx context.Context, nb uuid.UUID, ids []uuid.UUID, after *EdgePos, take int) ([]BodyReference, error) {
	f.calls++
	if take != 16384 {
		f.t.Fatal("private capture did not request the fixed raw bound")
	}
	return f.fetch(ctx, nb, ids, after, take)
}
func privateOldFUnit(f *privateOldFBefore, sources []uuid.UUID) *ingestUnit {
	return &ingestUnit{deps: &Ingest{Heads: f, Reads: NewLintReads(f, 5<<20), OldBodies: f}, event: PagesChanged{NotebookID: f.notebook, BeforeIngest: f}, writtenOrDeleted: sources,
		bounds: ingestBounds{knownDirect: map[uuid.UUID]bool{}, possibleDirect: map[uuid.UUID]bool{}, knownFollowed: map[uuid.UUID]bool{}, possibleFollowed: map[uuid.UUID]bool{}}}
}
func privateOldFSource(f *privateOldFBefore, id uuid.UUID) {
	f.nodes[id] = IngestNodeBefore{ID: id, Exists: true, Kind: "page", InSource: true, PageBytes: 200000, PageBytesKnown: true, Revision: 7, RevisionKnown: true}
	f.head[id] = IngestIndexedHead{ID: id, Present: true, PageRevision: 7, IndexedRevision: 7, Extractor: 2, FrontmatterValid: true}
}

func TestIngestPrivateCaptureLoadsFlaggedPageBeforeFiltering(t *testing.T) {
	for _, mode := range []string{"known", "size_unknown", "metadata_error", "page_head_error"} {
		t.Run(mode, func(t *testing.T) {
			source, a, b, page, absent, outside, root, nonasset := uuid.UUID{15: 1}, uuid.UUID{15: 11}, uuid.UUID{15: 12}, uuid.UUID{15: 13}, uuid.UUID{15: 14}, uuid.UUID{15: 15}, uuid.UUID{15: 16}, uuid.UUID{15: 17}
			f := &privateOldFBefore{t: t, notebook: uuid.UUID{15: 50}, nodes: map[uuid.UUID]IngestNodeBefore{}, head: map[uuid.UUID]IngestIndexedHead{}}
			privateOldFSource(f, source)
			for _, id := range []uuid.UUID{{}, a, b, outside, root} {
				f.nodes[id] = IngestNodeBefore{ID: id, Exists: true, Kind: "asset", InSource: true}
			}
			n := f.nodes[outside]
			n.InSource = false
			f.nodes[outside] = n
			n = f.nodes[root]
			n.IsSourceRoot = true
			f.nodes[root] = n
			f.nodes[absent] = IngestNodeBefore{ID: absent}
			f.nodes[page] = IngestNodeBefore{ID: page, Exists: true, Kind: "page", InSource: true, PageBytesKnown: true, Revision: 3, RevisionKnown: true}
			f.head[page] = IngestIndexedHead{ID: page, Present: true, PageRevision: 3, IndexedRevision: 3, Extractor: 2}
			if mode == "size_unknown" {
				n := f.nodes[source]
				n.PageBytesKnown = false
				f.nodes[source] = n
			}
			var wantErr error
			if mode == "metadata_error" {
				wantErr = errors.New("target metadata failed")
				f.metadataID = a
				f.metadataErr = wantErr
			}
			if mode == "page_head_error" {
				wantErr = errors.New("flagged page head failed")
				f.headErr = wantErr
			}
			targets := []uuid.UUID{b, page, a, {}, outside, root, absent, a, nonasset}
			rows := make([]BodyReference, len(targets))
			for i, id := range targets {
				rows[i] = BodyReference{EdgePos: EdgePos{SourceID: source, Start: int32(i)}, ResolvedID: id, ResolvedAsset: i != 8}
			}
			f.fetch = func(_ context.Context, nb uuid.UUID, ids []uuid.UUID, after *EdgePos, _ int) ([]BodyReference, error) {
				if nb != f.notebook || !slices.Equal(ids, []uuid.UUID{source}) {
					t.Fatal("private capture source relation")
				}
				if f.calls == 1 && after == nil {
					return rows, nil
				}
				if f.calls != 2 || after == nil || *after != rows[len(rows)-1].EdgePos {
					t.Fatal("private full EOF cursor")
				}
				return nil, nil
			}
			u := privateOldFUnit(f, []uuid.UUID{source})
			err := captureForIngest(t.Context(), u)
			//nolint:errorlint // Capture must return the exact metadata or head error object.
			if err != wantErr || f.calls != 2 || len(f.loaded) != 2 || !slices.Equal(f.loaded[1], []uuid.UUID{{}, a, b, page, absent, outside, root}) {
				t.Fatalf("full target load before filter: calls%d loaded%v err%v want%v", f.calls, f.loaded, err, wantErr)
			}
			if mode != "metadata_error" && (len(f.headRequests) != 2 || !reflect.DeepEqual(f.headRequests[1], []IngestRevision{{ID: page, Revision: 3}})) {
				t.Fatalf("asset-flagged real page head was skipped: %v", f.headRequests)
			}
			if wantErr != nil {
				if len(u.bounds.retained) != 0 {
					t.Fatal("failed metadata/head emitted F")
				}
				return
			}
			want := []oldFollowed{{source: source, assets: []uuid.UUID{{}, a, b}, possible: mode == "size_unknown"}}
			if !reflect.DeepEqual(u.bounds.retained, want) || cap(u.bounds.retained[0].assets) != 3 || u.bounds.uncertain != (mode == "size_unknown") {
				t.Fatalf("compact literal F=%+v uncertain%t", u.bounds.retained, u.bounds.uncertain)
			}
		})
	}
}

func TestIngestPrivateCapturePrefetchErrorOrder(t *testing.T) {
	for _, mode := range []string{"current_raw_native", "next_raw_native", "later_logical_prefix"} {
		t.Run(mode, func(t *testing.T) {
			sources := []uuid.UUID{{15: 1}, {15: 2}, {15: 3}}
			asset := uuid.UUID{15: 11}
			metadataErr, nativeErr := errors.New("earlier source metadata"), errors.New("adapter returned future native Scan error")
			f := &privateOldFBefore{t: t, notebook: uuid.UUID{15: 50}, nodes: map[uuid.UUID]IngestNodeBefore{}, head: map[uuid.UUID]IngestIndexedHead{}, metadataID: asset, metadataErr: metadataErr}
			for _, id := range sources {
				privateOldFSource(f, id)
			}
			rows := make([]BodyReference, 16385)
			rows[0] = BodyReference{EdgePos: EdgePos{SourceID: sources[0]}, ResolvedID: asset, ResolvedAsset: true}
			for i := 1; i < len(rows); i++ {
				source, start := sources[1], i-1
				if i > 8000 {
					source, start = sources[2], i-8001
				}
				rows[i] = BodyReference{EdgePos: EdgePos{SourceID: source, Start: int32(start)}, ResolvedID: uuid.UUID{15: 17}}
			}
			if mode == "later_logical_prefix" {
				rows[700].Start = -1
			}
			f.fetch = func(_ context.Context, _ uuid.UUID, ids []uuid.UUID, _ *EdgePos, take int) ([]BodyReference, error) {
				if !slices.Equal(ids, sources) || take != 16384 {
					t.Fatal("private prefetch shape")
				}
				if mode == "current_raw_native" || f.calls > 1 {
					return nil, nativeErr
				}
				return rows[:16384], nil
			}
			u := privateOldFUnit(f, sources)
			err := captureForIngest(t.Context(), u)
			want, loads := metadataErr, 2
			if mode == "current_raw_native" {
				want, loads = nativeErr, 1
			}
			//nolint:errorlint // Error priority must preserve the exact selected native or metadata error object.
			if err != want || f.calls != 1 || len(f.loaded) != loads || len(u.bounds.retained) != 0 {
				t.Fatalf("%s priority err%v want%v native calls%d loads%d", mode, err, want, f.calls, len(f.loaded))
			}
		})
	}
}

var _ IngestOldBodyReads = privateOldFFetch(nil)
