package postgresadapter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type childrenRead struct {
	SQL  string
	Args []any
	Mode pgx.QueryExecMode
	PID  uint32
	Rows int64
	Err  error
}

type childrenTrace struct{ reads []*childrenRead }
type childrenTraceKey struct{}

func (tr *childrenTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if !strings.HasPrefix(d.SQL, "-- name: ChildrenOfAll :many\n") {
		return ctx
	}
	r := &childrenRead{SQL: d.SQL, PID: conn.PgConn().PID()}
	for _, arg := range d.Args {
		if mode, ok := arg.(pgx.QueryExecMode); ok {
			r.Mode = mode
		} else {
			r.Args = append(r.Args, arg)
		}
	}
	tr.reads = append(tr.reads, r)
	return context.WithValue(ctx, childrenTraceKey{}, r)
}

func (*childrenTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	if r, ok := ctx.Value(childrenTraceKey{}).(*childrenRead); ok {
		r.Rows, r.Err = d.CommandTag.RowsAffected(), d.Err
	}
}

// This catches the former ANY-array plan visiting unrelated parent ranges.
// Actual transaction counters, not query text or a time budget, prove the
// work of each level. This fixture has fresh insert-only metadata; the HTTP
// move cost test separately exercises source-write and reset history.
func TestSubtreeChildrenKeepTheirParentRanges(t *testing.T) {
	f := newFixture(t)
	f.exec(t, "ALTER TABLE nodes SET (autovacuum_enabled=false)")
	root := f.page(t, f.eng, nil, "Read", 0)
	other := f.page(t, f.eng, nil, "Unrelated", 1)
	f.sourceChildren(t, f.eng, other.ID, 100000)
	f.sourceChildren(t, f.eng, root.ID, 9999)
	for _, statistics := range []string{"no_statistics", "analyzed"} {
		t.Run(statistics, func(t *testing.T) {
			if statistics == "analyzed" {
				f.exec(t, "ANALYZE nodes")
			}
			stats := f.count(t, "SELECT count(*) FROM pg_stats WHERE schemaname=current_schema() AND tablename='nodes'")
			if (statistics == "no_statistics" && stats != 0) || (statistics == "analyzed" && stats == 0) {
				t.Fatalf("%s has %d node column statistics", statistics, stats)
			}
			for _, mode := range []string{"force_generic_plan", "force_custom_plan"} {
				t.Run(mode, func(t *testing.T) {
					tr := &childrenTrace{}
					cfg := f.pool.Config()
					cfg.MaxConns, cfg.ConnConfig.Tracer = 1, tr
					cfg.ConnConfig.RuntimeParams["plan_cache_mode"] = mode
					pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
					if err != nil {
						t.Fatal(err)
					}
					defer pool.Close()
					// First exercise one parent, so a bad per-parent plan fails on
					// observed work before attempting 9,999 repeated bad scans.
					p := gen.ChildrenOfAllParams{NotebookID: f.eng, Parents: []uuid.UUID{root.ID}}
					first := childrenRange(t, pool, p, mode, 9999)
					p.Parents = make([]uuid.UUID, len(first))
					for i, row := range first {
						p.Parents[i] = row.ID
						if row.Name != fmt.Sprintf("Child %d", 9999-i) || row.SortOrder != float64(i) {
							t.Fatalf("child %d is %q at %v, want reverse insertion order", i, row.Name, row.SortOrder)
						}
					}
					childrenRange(t, pool, p, mode, 0)
					tr.reads = nil
					sub, err := postgresadapter.New(pool).Subtree(t.Context(), f.eng, root.ID)
					if err != nil || len(sub) != 10000 || sub.Height() != 2 || sub[0].Node != root {
						t.Fatalf("actual Store subtree: %d nodes, height%d, %v; want10000/2", len(sub), sub.Height(), err)
					}
					if len(tr.reads) != 2 {
						t.Fatalf("actual Store child reads=%d want2", len(tr.reads))
					}
					for i, read := range tr.reads {
						if read.Mode != pgx.QueryExecModeDescribeExec || read.Err != nil || read.Rows != []int64{9999, 0}[i] {
							t.Fatalf("Store layer%d mode=%s rows=%d err=%v", i, read.Mode, read.Rows, read.Err)
						}
						parents := []uuid.UUID{root.ID}
						if i == 1 {
							parents = p.Parents
							for j, row := range first {
								if sub[j+1].Node.ID != row.ID || sub[j+1].Level != 2 {
									t.Fatal("Store lost a child or changed its level/order")
								}
							}
						}
						want := statementOf(t, func(q *gen.Queries) error {
							_, e := q.ChildrenOfAll(t.Context(), gen.ChildrenOfAllParams{NotebookID: f.eng, Parents: parents})
							return e
						})
						if read.SQL != want.sql || !reflect.DeepEqual(read.Args, want.args) {
							t.Fatal("Store did not send the generated statement with the actual layer's parents")
						}
					}
				})
			}
		})
	}
}

func childrenRange(t *testing.T, pool *pgxpool.Pool, p gen.ChildrenOfAllParams, mode string, want int) []gen.ChildrenOfAllRow {
	t.Helper()
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	settings := map[string]string{}
	for _, key := range []string{"plan_cache_mode", "enable_seqscan", "enable_indexscan", "enable_bitmapscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin", "enable_sort", "track_counts"} {
		var value string
		if err := tx.QueryRow(ctx, "SELECT current_setting($1)", key).Scan(&value); err != nil {
			t.Fatal(err)
		}
		settings[key] = value
		expected := "on"
		if key == "plan_cache_mode" {
			expected = mode
		}
		if value != expected {
			t.Fatalf("actual backend %s=%s want%s", key, value, expected)
		}
	}
	s := statementOf(t, func(q *gen.Queries) error { _, e := q.ChildrenOfAll(ctx, p); return e })
	before := sourceWork(t, tx)
	rows, err := gen.New(postgres.Planned(tx)).ChildrenOfAll(ctx, p)
	if err != nil || len(rows) != want {
		t.Fatalf("ChildrenOfAll returned%d, %v; want%d", len(rows), err, want)
	}
	work := sourceWorkDelta(before, sourceWork(t, tx))
	const name = "ordinary_children_range"
	if _, err = tx.Exec(ctx, "PREPARE "+name+" AS "+s.sql); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = tx.Exec(context.Background(), "DEALLOCATE "+name) }()
	before = sourceWork(t, tx)
	var raw json.RawMessage
	if err = tx.QueryRow(ctx, "EXPLAIN(ANALYZE,BUFFERS,VERBOSE,FORMAT JSON) "+sourceProbeExecute(t, name, s.args)).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	explainWork := sourceWorkDelta(before, sourceWork(t, tx))
	var generic, custom int64
	if err = tx.QueryRow(ctx, "SELECT generic_plans,custom_plans FROM pg_prepared_statements WHERE name=$1", name).Scan(&generic, &custom); err != nil {
		t.Fatal(err)
	}
	evidence := struct {
		Mode, SQL         string
		Parents, Rows     int
		Settings          map[string]string
		Generic, Custom   int64
		Work, ExplainWork map[string]sourceRelationWork
		Plan              json.RawMessage
	}{mode, s.sql, len(p.Parents), len(rows), settings, generic, custom, work, explainWork, raw}
	sourceEvidence(t, "ordinary-children", evidence)
	if (mode == "force_generic_plan" && (generic != 1 || custom != 0)) || (mode == "force_custom_plan" && (custom != 1 || generic != 0)) {
		t.Fatalf("prepared counters generic%d custom%d in%s", generic, custom, mode)
	}
	// Fresh insert-only metadata has no old versions or deleted children.
	// Every index entry and fetched heap tuple must belong to the result.
	for label, w := range map[string]map[string]sourceRelationWork{"direct": work, "explain": explainWork} {
		var entries, fetched int64
		for _, r := range w {
			entries += r.Returned
			fetched += r.Fetched
		}
		if entries > int64(want) || fetched > int64(want) {
			t.Fatalf("%s ChildrenOfAll crossed parent ranges: entries=%d heap=%d want at most%d", label, entries, fetched, want)
		}
		if entries < int64(want) {
			t.Fatalf("%s ChildrenOfAll work counter missed visible rows: entries=%d want at least%d", label, entries, want)
		}
	}
	return rows
}

func TestSubtreeOrdersChildrenAcrossParents(t *testing.T) {
	f := newFixture(t)
	r := f.page(t, f.eng, nil, "Root", 0)
	a, b := f.page(t, f.eng, &r.ID, "A", 0), f.page(t, f.eng, &r.ID, "B", 1)
	high := f.page(t, f.eng, &a.ID, "High", 3)
	low := f.asset(t, f.eng, &b.ID, "low.bin", 0)
	middle := f.page(t, f.eng, &b.ID, "Middle", 2)
	x, y := f.page(t, f.eng, &a.ID, "TieA", 1), f.page(t, f.eng, &b.ID, "TieB", 1)
	if bytes.Compare(x.ID[:], y.ID[:]) > 0 {
		x, y = y, x
	}
	deleted := f.page(t, f.eng, &a.ID, "Deleted", -1)
	f.exec(t, "UPDATE nodes SET deleted_at=$2 WHERE id=$1", deleted.ID, now())
	foreign := f.page(t, f.ops, nil, "Foreign", 0)
	f.page(t, f.ops, &foreign.ID, "Foreign child", 0)
	want := domain.Subtree{{Node: r, Level: 1}, {Node: a, Level: 2}, {Node: b, Level: 2}}
	for _, node := range []domain.Node{low, x, y, middle, high} {
		want = append(want, domain.SubtreeNode{Node: node, Level: 3})
	}
	got, err := f.s.Subtree(t.Context(), f.eng, r.ID)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("global level order/fields = %+v, %v; want %+v", got, err, want)
	}
	for _, tc := range []struct {
		name    string
		parents []uuid.UUID
		want    []uuid.UUID
	}{
		{"duplicate and foreign", []uuid.UUID{a.ID, foreign.ID, b.ID, a.ID}, []uuid.UUID{low.ID, x.ID, y.ID, middle.ID, high.ID}},
		{"empty", []uuid.UUID{}, nil},
		{"null", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := gen.New(postgres.Planned(f.pool)).ChildrenOfAll(t.Context(), gen.ChildrenOfAllParams{NotebookID: f.eng, Parents: tc.parents})
			var ids []uuid.UUID
			for _, row := range rows {
				ids = append(ids, row.ID)
			}
			if err != nil || !slices.Equal(ids, tc.want) {
				t.Fatalf("children ids=%v, %v; want%v", ids, err, tc.want)
			}
		})
	}
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(t.Context(), "UPDATE nodes SET deleted_at=$2 WHERE id=$1", a.ID, now()); err != nil {
		t.Fatal(err)
	}
	rows, err := gen.New(postgres.Planned(tx)).ChildrenOfAll(t.Context(), gen.ChildrenOfAllParams{NotebookID: f.eng, Parents: []uuid.UUID{a.ID}})
	if err != nil || len(rows) != 2 {
		t.Fatalf("deleted parent's live children=%d, %v; want2", len(rows), err)
	}
}
