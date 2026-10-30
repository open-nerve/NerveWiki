package postgresadapter_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func TestLintCrossNotebook(t *testing.T) {
	t.Run("heads-keep-literal-v1-and-current-v2", func(t *testing.T) {
		f := newFixture(t)
		zero := uuid.UUID{}
		current := uuid.NewV7()
		f.replace(t, app.Page{ID: zero, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
		f.replace(t, app.Page{ID: current, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: false})
		f.replace(t, app.Page{ID: uuid.NewV7(), NotebookID: f.ops, Revision: 1}, domain.Facts{FrontmatterValid: true})
		if _, err := f.pool.Exec(t.Context(), `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, zero); err != nil {
			t.Fatal(err)
		}
		got, err := postgresadapter.NewLintReads(f.pool, 5*1024*1024).Heads(t.Context(), f.eng)
		want := []app.IndexedHead{{ID: zero, Extractor: 1, FrontmatterValid: true}, {ID: current, Extractor: 2, FrontmatterValid: false}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("stored heads = %+v, %v; want %+v", got, err, want)
		}
	})
}

// lintExec is fixture SQL, never the production read path.
func lintExec(t *testing.T, f fixture, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func TestLintEdgeBatches(t *testing.T) {
	t.Run("paired-positions", func(t *testing.T) {
		f := newFixture(t)
		r := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
		a, b, c := uuid.UUID{}, uuid.NewV7(), uuid.NewV7()
		for _, p := range []app.Page{{ID: a, NotebookID: f.eng, Revision: 1}, {ID: b, NotebookID: f.eng, Revision: 1}, {ID: c, NotebookID: f.ops, Revision: 1}} {
			f.replace(t, p, domain.Facts{FrontmatterValid: true, Links: []domain.Link{{Start: 0, End: 1, Kind: "wikilink", Target: "zero"}, {Start: 10, End: 11, Kind: "link", Target: "ten"}}, Properties: []domain.Property{{Key: "sources", Value: []byte(`"zero"`)}, {Key: "SOURCES", Value: []byte(`"one"`)}}})
		}
		pairs := []app.EdgePos{{SourceID: a, Start: 10}, {SourceID: b, Start: 0}, {SourceID: c, Start: 0}}
		edges, err := r.Edges(t.Context(), f.eng, pairs)
		if err != nil || len(edges) != 2 || edges[0].SourceID != a || edges[0].Start != 10 || edges[0].Target != "ten" || edges[1].SourceID != b || edges[1].Start != 0 || edges[1].Target != "zero" {
			t.Fatalf("paired exact edges: %+v, %v", edges, err)
		}
		slots, err := r.SourceSlotEdges(t.Context(), f.eng, pairs)
		if err != nil || len(slots) != 2 || slots[0].Target != "ten" || slots[1].Target != "zero" || slots[0].PropertyKey != nil {
			t.Fatalf("paired slots preserve SQL NULL: %+v, %v", slots, err)
		}
		values, err := r.SourceProperties(t.Context(), f.eng, []app.SourcePropertyPos{{PageID: a, Position: 1}, {PageID: b, Position: 0}, {PageID: c, Position: 0}})
		if err != nil || len(values) != 2 || values[0].Key != "SOURCES" || string(values[0].JSON) != `"one"` || string(values[1].JSON) != `"zero"` {
			t.Fatalf("paired exact properties: %+v, %v", values, err)
		}
		for _, bad := range [][]app.EdgePos{{pairs[0], pairs[0]}, {pairs[1], pairs[0]}, {{SourceID: a, Start: -1}}, make([]app.EdgePos, 513)} {
			if _, err := r.Edges(t.Context(), f.eng, bad); err == nil {
				t.Fatalf("accepted invalid positions: %d", len(bad))
			}
		}
	})
	t.Run("first-after-zero-uuid-and-current", func(t *testing.T) {
		f := newFixture(t)
		r := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
		zero, current, old, excluded, foreign := uuid.UUID{}, uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		for _, p := range []app.Page{{ID: zero, NotebookID: f.eng, Revision: 1}, {ID: current, NotebookID: f.eng, Revision: 1}, {ID: old, NotebookID: f.eng, Revision: 1}, {ID: excluded, NotebookID: f.eng, Revision: 1}, {ID: foreign, NotebookID: f.ops, Revision: 1}} {
			f.replace(t, p, originFacts(t, "[[x]] [[y]]"))
		}
		lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, old)
		first, err := r.WantedEdgeHeads(t.Context(), f.eng, []uuid.UUID{excluded}, nil, 1)
		if err != nil || len(first) != 1 || first[0].SourceID != zero || first[0].Start != 2 {
			t.Fatalf("first: %+v, %v", first, err)
		}
		after, err := r.WantedEdgeHeads(t.Context(), f.eng, []uuid.UUID{excluded}, &first[0].EdgePos, 512)
		want := []app.EdgeHead{{EdgePos: app.EdgePos{SourceID: zero, Start: 8}, TargetBytes: 1}, {EdgePos: app.EdgePos{SourceID: current, Start: 2}, TargetBytes: 1}, {EdgePos: app.EdgePos{SourceID: current, Start: 8}, TargetBytes: 1}}
		if err != nil || !reflect.DeepEqual(after, want) {
			t.Fatalf("after zero position: %+v, %v; want %+v", after, err, want)
		}
		for _, take := range []int{0, 513} {
			if _, err := r.WantedEdgeHeads(t.Context(), f.eng, nil, nil, take); err == nil {
				t.Fatalf("take %d accepted", take)
			}
		}
		// The target key is projected independently from an SQL NULL alt key.
		edges, err := r.Edges(t.Context(), f.eng, []app.EdgePos{first[0].EdgePos})
		if err != nil || len(edges) != 1 || edges[0].TargetKey == nil || *edges[0].TargetKey != "x" || edges[0].AltKey != nil || edges[0].ResolvedID != nil {
			t.Fatalf("nullable wanted projection: %+v, %v", edges, err)
		}
	})
	t.Run("ambiguous-current-target-and-asset", func(t *testing.T) {
		f := newFixture(t)
		r := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
		p, target, stale, asset := uuid.UUID{}, uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, originFacts(t, "[[a]] [[b]] [[c]]"))
		for _, id := range []uuid.UUID{target, stale} {
			f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
		}
		lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, stale)
		f.resolve(t, p, domain.Resolution{ID: target, Ambiguous: true}, 2)
		f.resolve(t, p, domain.Resolution{ID: stale, Ambiguous: true}, 8)
		f.resolve(t, p, domain.Resolution{ID: asset, Asset: true, Ambiguous: true}, 14)
		first, err := r.AmbiguousEdgeHeads(t.Context(), f.eng, nil, nil, 1)
		if err != nil || len(first) != 1 || first[0].Start != 2 {
			t.Fatalf("ambiguous first: %+v, %v", first, err)
		}
		after, err := r.AmbiguousEdgeHeads(t.Context(), f.eng, nil, &first[0].EdgePos, 512)
		if err != nil || len(after) != 1 || after[0].Start != 14 {
			t.Fatalf("ambiguous after skips stale page, retains asset: %+v, %v", after, err)
		}
		empty, err := r.AmbiguousEdgeHeads(t.Context(), f.eng, []uuid.UUID{p}, nil, 512)
		if err != nil || len(empty) != 0 {
			t.Fatalf("excluded ambiguous: %+v, %v", empty, err)
		}
	})
	t.Run("body-origins-and-literal-v1", func(t *testing.T) {
		f := newFixture(t)
		r := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
		current, old, foreign, target := uuid.UUID{}, uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		content := "---\n\"\": '[[empty]]'\nnamed: '[[named]]'\n---\n[[body]] [link](body) ![[embed]] ![image](body)"
		for _, p := range []app.Page{{ID: current, NotebookID: f.eng, Revision: 1}, {ID: old, NotebookID: f.eng, Revision: 1}, {ID: foreign, NotebookID: f.ops, Revision: 1}} {
			facts := originFacts(t, content)
			f.replace(t, p, facts)
			for _, link := range facts.Links {
				f.resolve(t, p.ID, domain.Resolution{ID: target, Asset: link.Kind == "image" || link.Kind == "embed"}, link.Start)
			}
		}
		lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, old)
		lintExec(t, f, `UPDATE page_links SET property_key=NULL WHERE source_id=$1 AND property_key=''`, old)
		ids := []uuid.UUID{current, old, foreign}
		first, err := r.BodyReferences(t.Context(), f.eng, ids, nil, 1)
		start := int32(strings.Index(content, "body"))
		if err != nil || len(first) != 1 || first[0].SourceID != current || first[0].Start != start {
			t.Fatalf("body first: %+v, %v; start %d", first, err, start)
		}
		rest, err := r.BodyReferences(t.Context(), f.eng, ids, &first[0].EdgePos, 512)
		if err != nil || len(rest) != 3 {
			t.Fatalf("body after: %+v, %v", rest, err)
		}
		for _, v := range rest {
			if v.SourceID != current || v.ResolvedID != target {
				t.Fatalf("foreign or stale body: %+v", v)
			}
		}
		exact, err := r.SourceSlotEdges(t.Context(), f.eng, []app.EdgePos{{SourceID: current, Start: 11}, {SourceID: current, Start: start}})
		if err != nil || len(exact) != 2 || exact[0].PropertyKey == nil || *exact[0].PropertyKey != "" || exact[1].PropertyKey != nil {
			t.Fatalf("NULL versus empty: %+v, %v", exact, err)
		}
	})
	t.Run("max-materialized-bytes-and-progress", func(t *testing.T) {
		f := newFixture(t)
		p := uuid.UUID{}
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
		// Controlled persisted sizes exercise batching; legal parser-growth
		// evidence is separate in TestLintJSONToast, not claimed by this seed.
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,ambiguous)
   SELECT $1,k,k+1,$2,'wikilink',repeat('x',CASE WHEN k=17 THEN 9*1024*1024 ELSE 1024*1024 END),'x',false FROM generate_series(0,19) k`, p, f.eng)
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		reads := app.NewLintReads(postgresadapter.NewLintReads(pool, 5*1024*1024), 5*1024*1024)
		batches := reads.Wanted(f.eng, nil)
		var sizes []int
		var starts []int32
		for {
			rows, err := batches.Next(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 {
				break
			}
			sizes = append(sizes, len(rows))
			var size int64
			for _, row := range rows {
				size += int64(len(row.Target))
				starts = append(starts, row.Start)
			}
			if size > 8*1024*1024 && len(rows) != 1 {
				t.Fatalf("materialized %d bytes in %d rows", size, len(rows))
			}
		}
		if !reflect.DeepEqual(sizes, []int{8, 8, 1, 1, 2}) || len(starts) != 20 {
			t.Fatalf("prefix/progress batches %v, starts %v", sizes, starts)
		}
		for i, start := range starts {
			if start != int32(i) {
				t.Fatalf("position %d = %d", i, start)
			}
		}
		names := tr.names()
		want := []string{"LintWantedEdgeHeadsFirst", "LintEdgesByPositions", "LintEdgesByPositions", "LintEdgesByPositions", "LintEdgesByPositions", "LintEdgesByPositions", "LintWantedEdgeHeadsAfter"}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("unconsumed heads were refetched: %v; want %v", names, want)
		}
	})
}

type lintTrace struct {
	mu    sync.Mutex
	calls []string
	modes []pgx.QueryExecMode
}

func (t *lintTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "-- name: Lint") {
		t.mu.Lock()
		t.calls = append(t.calls, strings.Fields(data.SQL)[2])
		if len(data.Args) > 0 {
			if mode, ok := data.Args[0].(pgx.QueryExecMode); ok {
				t.modes = append(t.modes, mode)
			}
		}
		t.mu.Unlock()
	}
	return ctx
}
func (*lintTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (t *lintTrace) names() []string                                               { t.mu.Lock(); defer t.mu.Unlock(); return slices.Clone(t.calls) }
func lintPool(t *testing.T, f fixture, tracer pgx.QueryTracer, connections int32) *pgxpool.Pool {
	t.Helper()
	cfg := f.pool.Config()
	cfg.MaxConns = connections
	cfg.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestLintEdgeBatchesCleanup(t *testing.T) {
	t.Run("cancel-and-query-error", func(t *testing.T) {
		for _, failure := range []string{"query-error", "scan-error", "cancel"} {
			t.Run(failure, func(t *testing.T) {
				f := newFixture(t)
				p := uuid.UUID{}
				f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
				lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,ambiguous) SELECT $1,k,k+1,$2,'wikilink','x','x',false FROM generate_series(0,512) k`, p, f.eng)
				tr := &lintTrace{}
				pool := lintPool(t, f, tr, 1)
				reads := app.NewLintReads(postgresadapter.NewLintReads(pool, 5*1024*1024), 5*1024*1024)
				stream := reads.Wanted(f.eng, nil)
				first, err := stream.Next(t.Context())
				if err != nil || len(first) != 512 {
					t.Fatalf("first actual batch: %d, %v", len(first), err)
				}
				switch failure {
				case "query-error":
					lintExec(t, f, `ALTER TABLE page_links RENAME COLUMN target TO hidden_target`)
					_, err = stream.Next(t.Context())
					lintExec(t, f, `ALTER TABLE page_links RENAME COLUMN hidden_target TO target`)
				case "scan-error":
					lintExec(t, f, `ALTER TABLE page_links ALTER COLUMN kind DROP NOT NULL`)
					lintExec(t, f, `UPDATE page_links SET kind=NULL WHERE source_id=$1 AND range_start=512`, p)
					_, err = stream.Next(t.Context())
					lintExec(t, f, `UPDATE page_links SET kind='wikilink' WHERE source_id=$1`, p)
				case "cancel":
					tx, e := f.pool.Begin(t.Context())
					if e != nil {
						t.Fatal(e)
					}
					defer func() { _ = tx.Rollback(context.Background()) }()
					if _, e := tx.Exec(t.Context(), `LOCK TABLE page_links IN ACCESS EXCLUSIVE MODE`); e != nil {
						t.Fatal(e)
					}
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					result := make(chan error, 1)
					go func() { _, e := stream.Next(ctx); result <- e }()
					pgtest.WaitForTableLockWaits(t, f.pool, "page_links", 1, time.Second)
					cancel()
					err = <-result
					if e := tx.Rollback(t.Context()); e != nil {
						t.Fatal(e)
					}
				}
				if err == nil {
					t.Fatal("the second real batch did not fail")
				}
				if failure == "scan-error" && !strings.Contains(err.Error(), "cannot scan NULL") {
					t.Fatalf("not a generated Rows.Scan failure: %v", err)
				}
				// pgxpool destroys cancelled connections asynchronously. A new
				// real request on its only slot is the cleanup barrier.
				recoveryCtx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				recovery, err := reads.Wanted(f.eng, nil).Next(recoveryCtx)
				if err != nil || len(recovery) != 512 {
					t.Fatalf("subsequent request: %d, %v", len(recovery), err)
				}
				if pool.Stat().AcquiredConns() != 0 {
					t.Fatalf("failed query retained small pool: acquired=%d", pool.Stat().AcquiredConns())
				}
			})
		}
	})
	t.Run("caller-transaction", func(t *testing.T) {
		f := newFixture(t)
		r := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
		p := uuid.NewV7()
		rollback := errors.New("fixture rollback")
		err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
			if err := f.s.AddPage(ctx, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, originFacts(t, "[[x]]")); err != nil {
				return err
			}
			rows, err := r.WantedEdgeHeads(ctx, f.eng, nil, nil, 512)
			if err != nil || len(rows) != 1 || rows[0].SourceID != p {
				t.Fatalf("read lost caller transaction: %+v, %v", rows, err)
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatal(err)
		}
		rows, err := r.Heads(t.Context(), f.eng)
		if err != nil || len(rows) != 0 {
			t.Fatalf("transaction leaked: %+v, %v", rows, err)
		}
	})
}

func TestLintEdgeBatchesRows(t *testing.T) {
	t.Run("row-bound-and-next-source", func(t *testing.T) {
		f := newFixture(t)
		first, second := uuid.UUID{}, lintID(1)
		for _, id := range []uuid.UUID{first, second} {
			f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
		}
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,ambiguous) SELECT $1,k,k+1,$2,'wikilink','x','x',false FROM generate_series(0,512) k`, first, f.eng)
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,ambiguous) VALUES($1,0,1,$2,'wikilink','next','next',false)`, second, f.eng)
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		r := app.NewLintReads(postgresadapter.NewLintReads(pool, 5*1024*1024), 5*1024*1024)
		stream := r.Wanted(f.eng, nil)
		one, err := stream.Next(t.Context())
		if err != nil || len(one) != 512 {
			t.Fatalf("first row-bound batch: %d %v", len(one), err)
		}
		two, err := stream.Next(t.Context())
		if err != nil || len(two) != 2 || two[0].SourceID != first || two[0].Start != 512 || two[1].SourceID != second || two[1].Start != 0 {
			t.Fatalf("adjacent source continuation: %+v %v", two, err)
		}
		end, err := stream.Next(t.Context())
		if err != nil || len(end) != 0 {
			t.Fatalf("row end: %d %v", len(end), err)
		}
		want := []string{"LintWantedEdgeHeadsFirst", "LintEdgesByPositions", "LintWantedEdgeHeadsAfter", "LintEdgesByPositions", "LintWantedEdgeHeadsAfter"}
		if got := tr.names(); !reflect.DeepEqual(got, want) {
			t.Fatalf("row-bound production sequence: %v", got)
		}
	})
}
