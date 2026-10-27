package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const sourceParentIndex = "nodes_parent_id_name_key_key"

// These are backend-local transaction counters, including indexes. Unlike
// cumulative pg_stat_user_indexes they need no flush or statistics snapshot
// reset. Returned counts entries emitted by the index AM, not averaged plan
// rows multiplied by loops; blocks include cache hits and are not disk IO.
type sourceRelationWork struct {
	Scans, Returned, Fetched, Blocks, Hits int64
}

func sourceWork(t *testing.T, tx pgx.Tx) map[string]sourceRelationWork {
	t.Helper()
	rows, err := tx.Query(context.Background(), `SELECT c.relname,
		pg_stat_get_xact_numscans(c.oid), pg_stat_get_xact_tuples_returned(c.oid),
		pg_stat_get_xact_tuples_fetched(c.oid), pg_stat_get_xact_blocks_fetched(c.oid),
		pg_stat_get_xact_blocks_hit(c.oid)
		FROM pg_class c WHERE c.oid = 'nodes'::regclass
		OR c.oid IN (SELECT indexrelid FROM pg_index WHERE indrelid = 'nodes'::regclass)
		ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := make(map[string]sourceRelationWork)
	for rows.Next() {
		var name string
		var w sourceRelationWork
		if err := rows.Scan(&name, &w.Scans, &w.Returned, &w.Fetched, &w.Blocks, &w.Hits); err != nil {
			t.Fatal(err)
		}
		out[name] = w
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func sourceWorkDelta(before, after map[string]sourceRelationWork) map[string]sourceRelationWork {
	out := make(map[string]sourceRelationWork, len(after))
	for name, a := range after {
		b := before[name]
		out[name] = sourceRelationWork{a.Scans - b.Scans, a.Returned - b.Returned,
			a.Fetched - b.Fetched, a.Blocks - b.Blocks, a.Hits - b.Hits}
	}
	return out
}

type sourceExecutedPlan struct {
	NodeType          string               `json:"Node Type"`
	RelationName      string               `json:"Relation Name"`
	IndexName         string               `json:"Index Name"`
	IndexCond         string               `json:"Index Cond"`
	ActualRows        float64              `json:"Actual Rows"`
	ActualLoops       float64              `json:"Actual Loops"`
	RowsRemovedFilter float64              `json:"Rows Removed by Filter"`
	SharedHitBlocks   int64                `json:"Shared Hit Blocks"`
	SharedReadBlocks  int64                `json:"Shared Read Blocks"`
	IndexSearches     int64                `json:"Index Searches"`
	Plans             []sourceExecutedPlan `json:"Plans"`
}

type sourceRangeEvidence struct {
	Case, Mode       string
	SQL, ExecutedSQL string
	Arguments        []any
	Parents, Take    int
	IDs              []uuid.UUID
	Settings         map[string]string
	GenericPlans     int64
	CustomPlans      int64
	ExecutionWork    map[string]sourceRelationWork
	ExplainWork      map[string]sourceRelationWork
	Plan             json.RawMessage
}

// The complete raw document remains in test output when no evidence folder
// is requested. The dedicated large run uses the same test and assertions:
// NERVEWIKI_SOURCE_RANGE_ROWS=1000000 selects only its serial million case.
func sourceEvidence(t *testing.T, kind string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("NERVEWIKI_SOURCE_RANGE_EVIDENCE"); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		file, err := os.CreateTemp(dir, kind+"-*.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(b); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		path, err := filepath.Abs(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s evidence: %s", kind, path)
		return
	}
	t.Logf("%s evidence: %s", kind, b)
}

// EXECUTE is a utility command, so its arguments are SQL literals rather
// than protocol parameters. Only internal typed UUIDs, UUID arrays and the
// bounded integer are accepted here; the prepared statement itself is the
// exact SQL captured from the generated production method.
func sourceExecute(t *testing.T, name string, args []any) string {
	t.Helper()
	literals := make([]string, len(args))
	for i, arg := range args {
		switch value := arg.(type) {
		case uuid.UUID:
			literals[i] = "'" + value.String() + "'::uuid"
		case []uuid.UUID:
			ids := make([]string, len(value))
			for j, id := range value {
				ids[j] = "'" + id.String() + "'::uuid"
			}
			literals[i] = "ARRAY[" + strings.Join(ids, ",") + "]::uuid[]"
		case int32:
			literals[i] = fmt.Sprintf("%d::integer", value)
		default:
			t.Fatalf("unexpected SourceChildrenWithin argument %T", arg)
		}
	}
	return "EXECUTE " + pgx.Identifier{name}.Sanitize() + " (" + strings.Join(literals, ",") + ")"
}

func sourceResultIDs(t *testing.T, tx pgx.Tx, sql string, args []any) []uuid.UUID {
	t.Helper()
	rows, err := postgres.Planned(tx).Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (f fixture) sourceRange(t *testing.T, label string, p gen.SourceChildrenWithinParams, generic bool, rows, scans int) sourceRangeEvidence {
	t.Helper()
	ctx := context.Background()
	s := statementOf(t, func(q *gen.Queries) error { _, err := q.SourceChildrenWithin(ctx, p); return err })
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	e := sourceRangeEvidence{Case: label, Mode: "custom", SQL: s.sql, ExecutedSQL: s.sql,
		Arguments: s.args, Parents: len(p.Parents), Take: int(p.MaxRows), Settings: make(map[string]string)}
	args := s.args
	const prepared = "source_children_within_evidence"
	if generic {
		e.Mode = "generic"
		if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode = force_generic_plan"); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, "PREPARE "+prepared+" AS "+s.sql); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = tx.Exec(ctx, "DEALLOCATE "+prepared) }()
		e.ExecutedSQL, args = sourceExecute(t, prepared, args), nil
	} else if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode = force_custom_plan"); err != nil {
		t.Fatal(err)
	}
	for _, setting := range []string{"plan_cache_mode", "enable_seqscan", "enable_bitmapscan", "enable_indexscan",
		"enable_indexonlyscan", "enable_sort", "enable_incremental_sort", "enable_nestloop", "enable_material", "track_counts"} {
		var value string
		if err := tx.QueryRow(ctx, "SELECT current_setting($1)", setting).Scan(&value); err != nil {
			t.Fatal(err)
		}
		e.Settings[setting] = value
		if setting != "plan_cache_mode" && value != "on" {
			t.Fatalf("range evidence requires default enabled planner/statistics setting %s, got %s", setting, value)
		}
	}
	before := sourceWork(t, tx)
	e.IDs = sourceResultIDs(t, tx, e.ExecutedSQL, args)
	e.ExecutionWork = sourceWorkDelta(before, sourceWork(t, tx))
	before = sourceWork(t, tx)
	if err := postgres.Planned(tx).QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, VERBOSE, FORMAT JSON) "+e.ExecutedSQL, args...).Scan(&e.Plan); err != nil {
		t.Fatal(err)
	}
	e.ExplainWork = sourceWorkDelta(before, sourceWork(t, tx))
	if generic {
		if err := tx.QueryRow(ctx, "SELECT generic_plans, custom_plans FROM pg_prepared_statements WHERE name = $1", prepared).
			Scan(&e.GenericPlans, &e.CustomPlans); err != nil {
			t.Fatal(err)
		}
		if e.GenericPlans < 2 || e.CustomPlans != 0 {
			t.Errorf("actual prepared executions used generic/custom plans %d/%d, want >=2/0", e.GenericPlans, e.CustomPlans)
		}
	}
	sourceEvidence(t, "source-range", e)
	if len(e.IDs) != rows {
		t.Errorf("%s %s returned %d rows, want %d", label, e.Mode, len(e.IDs), rows)
	}
	for _, work := range []map[string]sourceRelationWork{e.ExecutionWork, e.ExplainWork} {
		if w, ok := work[sourceParentIndex]; !ok || w.Returned != int64(rows) || w.Scans != int64(scans) {
			t.Errorf("%s %s index work = %+v, want exactly %d emitted entries in %d parent probes", label, e.Mode, w, rows, scans)
		}
		if w := work["nodes"]; w.Scans != 0 || w.Returned != 0 {
			t.Errorf("%s %s read the heap sequentially: %+v", label, e.Mode, w)
		}
		for name, w := range work {
			if name != "nodes" && name != sourceParentIndex && w.Returned != 0 {
				t.Errorf("%s %s read unrelated index %s: %+v", label, e.Mode, name, w)
			}
		}
	}
	var plans []struct{ Plan sourceExecutedPlan }
	if err := json.Unmarshal(e.Plan, &plans); err != nil || len(plans) != 1 {
		t.Fatalf("decode actual plan: %v, documents=%d", err, len(plans))
	}
	if root := plans[0].Plan; root.NodeType != "Limit" || root.ActualRows != float64(rows) || root.ActualLoops != 1 {
		t.Errorf("%s %s outer actual plan = %+v; want one Limit returning %d", label, e.Mode, root, rows)
	}
	var index bool
	var inspect func(sourceExecutedPlan)
	inspect = func(n sourceExecutedPlan) {
		switch n.NodeType {
		case "Sort", "Incremental Sort", "Seq Scan", "Bitmap Heap Scan", "Bitmap Index Scan", "Materialize":
			t.Errorf("%s %s has unbounded child work at %s", label, e.Mode, n.NodeType)
		}
		if n.RelationName == "nodes" && strings.Contains(n.NodeType, "Index") {
			index = true
			if n.IndexName != sourceParentIndex || !strings.Contains(n.IndexCond, "parent_id") ||
				!strings.Contains(n.IndexCond, "p.id") || strings.Contains(n.IndexCond, "::text") {
				t.Errorf("%s %s lacks the UUID parent range: %+v", label, e.Mode, n)
			}
			if n.RowsRemovedFilter != 0 {
				t.Errorf("%s %s filtered rows outside its verified parents: %+v", label, e.Mode, n)
			}
		}
		for _, child := range n.Plans {
			inspect(child)
		}
	}
	inspect(plans[0].Plan)
	if !index {
		t.Errorf("%s %s has no actual parent index scan", label, e.Mode)
	}
	return e
}

type sourceRangeResource struct {
	Case, Stage, WAL                                 string
	Heap, Indexes, Total, ParentIndex, WALSinceStart int64
}

func (f fixture) sourceResources(t *testing.T, label, stage, startWAL string) string {
	t.Helper()
	r := sourceRangeResource{Case: label, Stage: stage}
	if err := f.pool.QueryRow(context.Background(), `SELECT pg_relation_size('nodes'), pg_indexes_size('nodes'),
		pg_total_relation_size('nodes'), pg_relation_size('nodes_parent_id_name_key_key'),
		pg_current_wal_insert_lsn()::text`).Scan(&r.Heap, &r.Indexes, &r.Total, &r.ParentIndex, &r.WAL); err != nil {
		t.Fatal(err)
	}
	if startWAL != "" {
		if err := f.pool.QueryRow(context.Background(), "SELECT pg_wal_lsn_diff($1::pg_lsn, $2::pg_lsn)::bigint", r.WAL, startWAL).
			Scan(&r.WALSinceStart); err != nil {
			t.Fatal(err)
		}
	}
	sourceEvidence(t, "source-resource", r)
	return r.WAL
}

func (f fixture) sourceStatistics(t *testing.T, analyzed bool) {
	t.Helper()
	if analyzed {
		f.exec(t, "ANALYZE nodes")
	}
	count := f.count(t, "SELECT count(*) FROM pg_stats WHERE tablename = 'nodes'")
	if (count > 0) != analyzed {
		t.Fatalf("nodes statistics columns = %d, analyzed=%t", count, analyzed)
	}
}

// A has more foreign nodes than the fixed target. B adds another wide
// foreign layer, then deletes it to return logically to A. Its dead tuples
// and enlarged B-tree stay: record buffers, do not demand identical blocks.
// Each statistics mode owns a disposable DB, and each width runs serially.
func TestSourceSubtreeReadsWithinIndexRanges(t *testing.T) {
	widths := []int{10_000, 100_000}
	if selected := os.Getenv("NERVEWIKI_SOURCE_RANGE_ROWS"); selected != "" {
		if selected != "1000000" {
			t.Fatalf("NERVEWIKI_SOURCE_RANGE_ROWS=%q; only the explicit serial 1000000 run is supported", selected)
		}
		widths = []int{1_000_000}
	}
	for _, width := range widths {
		for _, analyzed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d_children/analyzed_%t", width, analyzed), func(t *testing.T) {
				f := newFixture(t)
				f.exec(t, "ALTER TABLE nodes SET (autovacuum_enabled = false)")
				label := t.Name()
				wal := f.sourceResources(t, label, "empty", "")
				root := f.page(t, f.eng, nil, "Target", 0)
				foreign := f.page(t, f.ops, nil, "Other", 0)
				f.sourceChildren(t, f.eng, root.ID, width)
				f.sourceChildren(t, f.ops, foreign.ID, width+1000)
				f.sourceResources(t, label, "A-loaded", wal)
				baseline := make(map[string][]uuid.UUID)
				for _, stage := range []string{"A-before", "B", "A-after"} {
					switch stage {
					case "B":
						f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
							created_by_id, updated_by_id, created_at, updated_at)
							SELECT uuidv7(), $1, $2, 'page', 'Growth ' || i, 'growth ' || lpad(i::text, 8, '0'), i,
							$3, $3, $5, $5 FROM generate_series(1, $4::integer) i`, f.ops, foreign.ID, f.alice, width, now())
					case "A-after":
						f.exec(t, "DELETE FROM nodes WHERE notebook_id = $1 AND parent_id = $2 AND name_key LIKE 'growth %'", f.ops, foreign.ID)
					}
					f.sourceStatistics(t, analyzed)
					f.sourceResources(t, label, stage, wal)
					s, trace, _ := f.tracedSourceSubtree(t)
					got, complete, err := s.SubtreeWithin(context.Background(), f.eng, root.ID, domain.MaxUnitNodes)
					if err != nil || complete || got != nil || len(trace.reads) != 1 || trace.reads[0].Rows != domain.MaxUnitNodes {
						t.Fatalf("%s wide reader = %d nodes, complete=%t, %v, probes=%+v", stage, len(got), complete, err, trace.reads)
					}
					sourceEvidence(t, "source-reader", struct {
						Case  string
						Reads []*sourceSubtreeRead
					}{label + "/" + stage, trace.reads})
					for _, take := range []int32{domain.MaxUnitNodes, 2} {
						for _, generic := range []bool{false, true} {
							p := gen.SourceChildrenWithinParams{NotebookID: f.eng, Parents: []uuid.UUID{root.ID}, MaxRows: take}
							e := f.sourceRange(t, label+"/"+stage, p, generic, int(take), 1)
							key := fmt.Sprintf("%s/%d", e.Mode, take)
							if stage == "A-before" {
								baseline[key] = e.IDs
							} else if !reflect.DeepEqual(e.IDs, baseline[key]) {
								t.Errorf("%s %s changed target results with only foreign notebook changes", stage, key)
							}
						}
					}
				}
			})
		}
	}
}

// Successful max-size trees still perform thousands of empty parent range
// probes. These are real work, reported separately from the returned rows.
// Remaining-two uses the same long verified parent list but must stop after
// its first nonempty range; the second parent's children are not visited.
func TestSourceSubtreeMeasuresLeafAndRemainingRanges(t *testing.T) {
	for _, remaining := range []bool{false, true} {
		t.Run(fmt.Sprintf("remaining_two_%t", remaining), func(t *testing.T) {
			f := newFixture(t)
			f.exec(t, "ALTER TABLE nodes SET (autovacuum_enabled = false)")
			root := f.page(t, f.eng, nil, "Root", 0)
			parents := 9999
			if remaining {
				parents = 9998
			}
			f.sourceChildren(t, f.eng, root.ID, parents)
			if remaining {
				var first uuid.UUID
				if err := f.pool.QueryRow(context.Background(), "SELECT id FROM nodes WHERE parent_id = $1 ORDER BY name_key LIMIT 1", root.ID).Scan(&first); err != nil {
					t.Fatal(err)
				}
				f.sourceChildren(t, f.eng, first, 10_000)
			}
			for _, analyzed := range []bool{false, true} {
				f.sourceStatistics(t, analyzed)
				s, trace, _ := f.tracedSourceSubtree(t)
				got, complete, err := s.SubtreeWithin(context.Background(), f.eng, root.ID, domain.MaxUnitNodes)
				if err != nil || complete == remaining || len(trace.reads) != 2 {
					t.Fatalf("leaf/remaining read = %d nodes, complete=%t, %v, queries=%d", len(got), complete, err, len(trace.reads))
				}
				var ids []uuid.UUID
				for _, arg := range trace.reads[1].Args {
					if value, ok := arg.([]uuid.UUID); ok {
						ids = value
					}
				}
				if len(ids) != parents {
					t.Fatalf("actual last query used %d parents, want %d", len(ids), parents)
				}
				p := gen.SourceChildrenWithinParams{NotebookID: f.eng, Parents: ids, MaxRows: 1}
				rows, scans := 0, parents
				if remaining {
					p.MaxRows, rows, scans = 2, 2, 1
				}
				for _, generic := range []bool{false, true} {
					f.sourceRange(t, fmt.Sprintf("%s/analyzed_%t", t.Name(), analyzed), p, generic, rows, scans)
				}
			}
		})
	}
}
