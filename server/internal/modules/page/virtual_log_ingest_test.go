package page

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// These are consumer DTOs, not persisted marks or a producer fixture.
type virtualLogReadsFixture struct {
	t        *testing.T
	ctx      context.Context
	notebook uuid.UUID
	limit    int
	rows     []domain.Changeset
	facts    map[uuid.UUID]domain.ChangesetFacts
	changes  map[uuid.UUID][]domain.ChangeRow
	sources  map[uuid.UUID][]*uuid.UUID
	targets  []postgresadapter.LinkPath
	names    map[uuid.UUID]string
	pathIDs  map[uuid.UUID]bool
	calls    []string
	fail     string
	err      error
}

func (f *virtualLogReadsFixture) call(ctx context.Context, name string) error {
	f.t.Helper()
	if ctx != f.ctx {
		f.t.Fatalf("%s replaced the caller's snapshot context", name)
	}
	f.calls = append(f.calls, name)
	if f.fail == name {
		return f.err
	}
	return nil
}

func (f *virtualLogReadsFixture) changesetIDs(ids []uuid.UUID) {
	f.t.Helper()
	if len(f.rows) != 2 {
		f.t.Fatal("Recent read another batch after empty history")
	}
	want := []uuid.UUID{f.rows[0].ID, f.rows[1].ID}
	if !reflect.DeepEqual(ids, want) {
		f.t.Fatalf("changeset batch = %v, want %v", ids, want)
	}
}

func (f *virtualLogReadsFixture) ListChangesets(ctx context.Context, notebook uuid.UUID, filter domain.LogFilter, below domain.ChangesetCursor, limit int) ([]domain.Changeset, error) {
	if notebook != f.notebook || !reflect.DeepEqual(filter, domain.LogFilter{}) || below != domain.ChangesetFrom() || limit != f.limit {
		f.t.Fatal("Recent changed the notebook, ordinary-log filter, first cursor, or limit")
	}
	return f.rows, f.call(ctx, "list")
}

func (f *virtualLogReadsFixture) ChangesetFacts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.ChangesetFacts, error) {
	f.changesetIDs(ids)
	return f.facts, f.call(ctx, "facts")
}

func (f *virtualLogReadsFixture) FirstChanges(ctx context.Context, ids []uuid.UUID, n int) (map[uuid.UUID][]domain.ChangeRow, error) {
	f.changesetIDs(ids)
	if n != 3 {
		f.t.Fatalf("change preview = %d, want 3", n)
	}
	return f.changes, f.call(ctx, "changes")
}

func (f *virtualLogReadsFixture) FirstIngestSources(ctx context.Context, ids []uuid.UUID, n int) (map[uuid.UUID][]*uuid.UUID, error) {
	f.changesetIDs(ids)
	if n != 5 {
		f.t.Fatalf("source preview = %d, want 5", n)
	}
	return f.sources, f.call(ctx, "sources")
}

func (f *virtualLogReadsFixture) DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if !reflect.DeepEqual(ids, []uuid.UUID{f.rows[0].By}) {
		f.t.Fatalf("actor batch was not deduplicated: %v", ids)
	}
	return f.names, f.call(ctx, "names")
}

func (f *virtualLogReadsFixture) LinkTargetsByIDs(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) ([]postgresadapter.LinkPath, error) {
	if notebook != f.notebook || len(ids) != len(f.pathIDs) {
		f.t.Fatalf("path batch scope/size: %s %v", notebook, ids)
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if !f.pathIDs[id] || seen[id] {
			f.t.Fatalf("path batch contains an unrelated or duplicate ID: %v", ids)
		}
		seen[id] = true
	}
	return f.targets, f.call(ctx, "paths")
}

// Only the old-behavior baseline uses PathsOf. It returns hand-written live
// paths; the proposed Recent must use the combined ID/kind batch above.
func (f *virtualLogReadsFixture) PathsOf(ctx context.Context, _ uuid.UUID, _ []uuid.UUID) (map[uuid.UUID]string, error) {
	return map[uuid.UUID]string{{15: 90}: "raw/Z.md", {15: 12}: "Page.md"}, f.call(ctx, "paths")
}

func virtualLogFixture(t *testing.T) (*virtualLogReadsFixture, []VirtualLogChangeset) {
	t.Helper()
	book, cs, ordinary, actor := uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 3}, uuid.UUID{15: 4}
	second, fourth, fifth, page, gone := uuid.UUID{15: 90}, uuid.UUID{15: 10}, uuid.UUID{15: 50}, uuid.UUID{15: 12}, uuid.UUID{15: 13}
	at := time.Date(2026, time.November, 3, 1, 2, 3, 0, time.UTC)
	message := "kept message"
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "same snapshot")
	f := &virtualLogReadsFixture{
		t: t, ctx: ctx, notebook: book, limit: 1000,
		rows: []domain.Changeset{
			{ID: cs, NotebookID: book, Kind: domain.ChangesetEdit, Client: "web", By: actor, CreatedAt: at, Message: &message, IngestCount: 25},
			{ID: ordinary, NotebookID: book, Kind: domain.ChangesetImport, Client: "web", By: actor, CreatedAt: at.Add(-time.Hour)},
		},
		facts: map[uuid.UUID]domain.ChangesetFacts{cs: {Nodes: 7, Contents: 8}, ordinary: {Nodes: 1, Contents: 1}},
		changes: map[uuid.UUID][]domain.ChangeRow{
			cs: {
				{NodeID: second, NodeKind: domain.KindPage, Name: "Z", Alive: true},
				{NodeID: gone, NodeKind: domain.KindPage, Name: "Gone"},
				{NodeID: page, NodeKind: domain.KindPage, Name: "Page", Alive: true},
			},
			ordinary: {{NodeID: page, NodeKind: domain.KindPage, Name: "Page", Alive: true}},
		},
		sources: map[uuid.UUID][]*uuid.UUID{cs: {nil, &second, nil, &fourth, &fifth}, ordinary: {}},
		// Result order deliberately differs from seq, UUID, and path order.
		targets: []postgresadapter.LinkPath{
			{ID: fifth, Steps: []postgresadapter.LinkStep{{Name: "raw"}, {Name: "M"}}},
			{ID: page, Steps: []postgresadapter.LinkStep{{Name: "Page"}}},
			{ID: fourth, Asset: true, Steps: []postgresadapter.LinkStep{{Name: "raw"}, {Name: "A.png"}}},
			{ID: second, Steps: []postgresadapter.LinkStep{{Name: "raw"}, {Name: "Z"}}},
		},
		names: map[uuid.UUID]string{actor: "Reader"}, pathIDs: map[uuid.UUID]bool{second: true, fourth: true, fifth: true, page: true},
	}
	want := []VirtualLogChangeset{
		{ID: cs, Kind: "edit", Client: "web", ByName: "Reader", CreatedAt: at, Message: &message,
			NodeCount: 7, ContentCount: 8, IngestCount: 25,
			Changes: []VirtualLogChange{
				{NodeID: second, Name: "Z", Path: new("raw/Z.md"), Kind: "content"},
				{NodeID: gone, Name: "Gone", Kind: "content"},
				{NodeID: page, Name: "Page", Path: new("Page.md"), Kind: "content"},
			},
			IngestSources: []VirtualLogSource{{}, {NodeID: &second, Path: new("raw/Z.md")}, {},
				{NodeID: &fourth, Path: new("raw/A.png"), Asset: true}, {NodeID: &fifth, Path: new("raw/M.md")}}},
		{ID: ordinary, Kind: "import", Client: "web", ByName: "Reader", CreatedAt: at.Add(-time.Hour), NodeCount: 1, ContentCount: 1,
			Changes: []VirtualLogChange{{NodeID: page, Name: "Page", Path: new("Page.md"), Kind: "content"}}, IngestSources: []VirtualLogSource{}},
	}
	return f, want
}

func TestVirtualLogRecent(t *testing.T) {
	all := []string{"list", "facts", "changes", "sources", "names", "paths"}
	for _, missing := range []bool{false, true} {
		name := "ingest_prefix"
		if missing {
			name = "missing_path_keeps_slot"
		}
		t.Run(name, func(t *testing.T) {
			f, want := virtualLogFixture(t)
			if missing {
				f.targets = f.targets[1:] // Fifth ID was returned by preview but has no current path.
				want[0].IngestSources[4] = VirtualLogSource{}
			}
			got, err := (virtualLogReads{store: f, names: f}).Recent(f.ctx, f.notebook, f.limit)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("complete Recent DTO = %#v, %v; want %#v", got, err, want)
			}
			if !reflect.DeepEqual(f.calls, all) {
				t.Fatalf("read calls = %v, want one batch each %v", f.calls, all)
			}
		})
	}
	t.Run("errors", func(t *testing.T) {
		for i, stage := range all {
			t.Run(stage, func(t *testing.T) {
				f, _ := virtualLogFixture(t)
				f.fail, f.err = stage, errors.New("read failed")
				got, err := (virtualLogReads{store: f, names: f}).Recent(f.ctx, f.notebook, f.limit)
				if got != nil || err != f.err || !reflect.DeepEqual(f.calls, all[:i+1]) { //nolint:errorlint // Preserve the exact read error object through Recent.
					t.Fatalf("error identity/short circuit: rows=%v err=%v calls=%v", got, err, f.calls)
				}
			})
		}
	})
	t.Run("source_cancel", func(t *testing.T) {
		f, _ := virtualLogFixture(t)
		f.fail, f.err = "sources", context.Canceled
		got, err := (virtualLogReads{store: f, names: f}).Recent(f.ctx, f.notebook, f.limit)
		if got != nil || !errors.Is(err, context.Canceled) || !reflect.DeepEqual(f.calls, all[:4]) {
			t.Fatalf("cancel identity/short circuit: rows=%v err=%v calls=%v", got, err, f.calls)
		}
	})
	t.Run("empty", func(t *testing.T) {
		f, _ := virtualLogFixture(t)
		f.rows = nil
		got, err := (virtualLogReads{store: f, names: f}).Recent(f.ctx, f.notebook, f.limit)
		if got != nil || err != nil || !reflect.DeepEqual(f.calls, []string{"list"}) {
			t.Fatalf("empty history did extra reads: %v %v %v", got, err, f.calls)
		}
	})
	t.Run("limits", func(t *testing.T) {
		for _, limit := range []int{0, 1001} {
			f, _ := virtualLogFixture(t)
			got, err := (virtualLogReads{store: f, names: f}).Recent(f.ctx, f.notebook, limit)
			if got != nil || err == nil || len(f.calls) != 0 {
				t.Fatalf("invalid limit %d reached storage: %v %v %v", limit, got, err, f.calls)
			}
		}
	})
}
