package app

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"uuid"
)

// This small capture test protects the qualification boundary, not database
// locking. Bootstrap B18 and the real public-head-change tests cover routing.
func TestIngestPrivateSourceQualification(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		read, uncertain, possible, unknown bool
	}{
		{"current", true, false, false, false},
		{"bytes_unknown", true, true, true, false},
		{"revision_unknown", false, true, false, true},
		{"revision_mismatch", false, true, false, true},
		{"literal_v1", false, true, false, true},
		{"missing_head", false, true, false, true},
		{"known_empty", false, false, false, false},
		{"source_root", false, false, false, false},
		{"absent", false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, a, nb := uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 3}
			f := &privateBeforeFixture{t: t, notebook: nb, source: s, asset: a,
				node: IngestNodeBefore{ID: s, Exists: true, Kind: "page", InSource: true, PageBytes: 17, PageBytesKnown: true, Revision: 7, RevisionKnown: true},
				head: IngestIndexedHead{ID: s, Present: true, PageRevision: 7, IndexedRevision: 7, Extractor: 2, FrontmatterValid: true}}
			switch tc.name {
			case "bytes_unknown":
				f.node.PageBytesKnown = false
			case "revision_unknown":
				f.node.RevisionKnown = false
			case "revision_mismatch":
				f.head.IndexedRevision = 6
			case "literal_v1":
				f.head.Extractor = 1
			case "missing_head":
				f.head.Present = false
				f.head.IndexedRevision = 0
				f.head.Extractor = 0
			case "known_empty":
				f.node.PageBytes = 0
			case "source_root":
				f.node.IsSourceRoot = true
			case "absent":
				f.node = IngestNodeBefore{ID: s}
			}
			u := &ingestUnit{deps: &Ingest{Heads: f, Reads: NewLintReads(f, 5<<20), OldBodies: f},
				event: PagesChanged{NotebookID: nb, BeforeIngest: f}, writtenOrDeleted: []uuid.UUID{s},
				bounds: ingestBounds{possibleDirect: map[uuid.UUID]bool{}, possibleFollowed: map[uuid.UUID]bool{}}}
			if err := captureForIngest(t.Context(), u); err != nil {
				t.Fatal(err)
			}
			calls := 0
			var retained []oldFollowed
			if tc.read {
				calls = 2
				retained = []oldFollowed{{source: s, assets: []uuid.UUID{a}, possible: tc.possible}}
			}
			var unknown []uuid.UUID
			if tc.unknown {
				unknown = []uuid.UUID{s}
			}
			if f.calls != calls || u.bounds.uncertain != tc.uncertain || !reflect.DeepEqual(u.bounds.retained, retained) || !slices.Equal(u.bounds.unknownFParents, unknown) {
				t.Fatalf("private source qualification: calls=%d want%d bounds=%+v retained=%+v unknown=%v", f.calls, calls, u.bounds, retained, unknown)
			}
			if len(u.bounds.possibleDirect)+len(u.bounds.possibleFollowed) != 0 || u.bounds.allDirect || u.bounds.allInheritedAssets {
				t.Fatalf("source-only old F changed independent wiki D or inherited global bound: %+v", u.bounds)
			}
		})
	}
}

type privateBeforeFixture struct {
	LintReadStore
	t                       *testing.T
	notebook, source, asset uuid.UUID
	node                    IngestNodeBefore
	head                    IngestIndexedHead
	calls                   int
}

func (f *privateBeforeFixture) NodesBefore(_ context.Context, ids []uuid.UUID) ([]IngestNodeBefore, error) {
	out := make([]IngestNodeBefore, len(ids))
	for i, id := range ids {
		switch id {
		case f.source:
			out[i] = f.node
		case f.asset:
			out[i] = IngestNodeBefore{ID: id, Exists: true, Kind: "asset", InSource: true}
		default:
			f.t.Fatalf("unexpected old metadata ID %s", id)
		}
	}
	return out, nil
}

func (f *privateBeforeFixture) IndexedHeads(_ context.Context, nb uuid.UUID, ids []IngestRevision) ([]IngestIndexedHead, error) {
	if nb != f.notebook || len(ids) > 1 {
		f.t.Fatal("unexpected private qualification heads")
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if ids[0] != (IngestRevision{ID: f.source, Revision: 7}) {
		f.t.Fatalf("before revision identity: %+v", ids)
	}
	return []IngestIndexedHead{f.head}, nil
}

func (f *privateBeforeFixture) BodyReferences(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]BodyReference, error) {
	f.t.Fatal("private source capture used public Body reader")
	return nil, nil
}

func (f *privateBeforeFixture) OldBodyReferences(ctx context.Context, nb uuid.UUID, ids []uuid.UUID, after *EdgePos, take int) ([]BodyReference, error) {
	f.calls++
	if ctx.Err() != nil || nb != f.notebook || !slices.Equal(ids, []uuid.UUID{f.source}) || take != 16384 {
		f.t.Fatal("private source relation/cancellation/batch bound")
	}
	if f.calls == 1 && after == nil {
		return []BodyReference{{EdgePos: EdgePos{SourceID: f.source, Start: 11}, ResolvedID: f.asset, ResolvedAsset: true}}, nil
	}
	if f.calls != 2 || after == nil || *after != (EdgePos{SourceID: f.source, Start: 11}) {
		f.t.Fatalf("private source complete EOF cursor: calls%d after%+v", f.calls, after)
	}
	return nil, nil
}
