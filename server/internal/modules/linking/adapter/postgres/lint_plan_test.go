//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const lintRangeIndex = "page_links_notebook_id_source_id_range_start_idx"

// lintWork observes actual tables and indexes on this backend, including
// cache hits. It is not a count of returned rows or a wall-time proxy.
type lintWork struct{ Scans, Returned, Fetched, Blocks, Hits int64 }

func lintRelationWork(t *testing.T, tx pgx.Tx) map[string]lintWork {
	t.Helper()
	rows, err := tx.Query(t.Context(), `WITH tables AS (
 SELECT oid FROM pg_class WHERE relname IN ('indexed_pages','page_links','page_properties')
 UNION SELECT reltoastrelid FROM pg_class WHERE relname IN ('page_links','page_properties') AND reltoastrelid<>0), relations AS (
 SELECT oid FROM tables UNION SELECT indexrelid FROM pg_index WHERE indrelid IN (SELECT oid FROM tables))
 SELECT c.relname,pg_stat_get_xact_numscans(c.oid),pg_stat_get_xact_tuples_returned(c.oid),pg_stat_get_xact_tuples_fetched(c.oid),pg_stat_get_xact_blocks_fetched(c.oid),pg_stat_get_xact_blocks_hit(c.oid) FROM relations r JOIN pg_class c ON c.oid=r.oid ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]lintWork{}
	for rows.Next() {
		var n string
		var w lintWork
		if err := rows.Scan(&n, &w.Scans, &w.Returned, &w.Fetched, &w.Blocks, &w.Hits); err != nil {
			t.Fatal(err)
		}
		out[n] = w
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
func lintWorkDelta(a, b map[string]lintWork) map[string]lintWork {
	out := map[string]lintWork{}
	for n, v := range b {
		old := a[n]
		out[n] = lintWork{v.Scans - old.Scans, v.Returned - old.Returned, v.Fetched - old.Fetched, v.Blocks - old.Blocks, v.Hits - old.Hits}
	}
	return out
}
func lintExecute(t *testing.T, name string, args []any) string {
	t.Helper()
	literals := make([]string, len(args))
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	for i, arg := range args {
		switch v := arg.(type) {
		case uuid.UUID:
			literals[i] = quote(v.String()) + "::uuid"
		case string:
			literals[i] = quote(v) + "::text"
		case int32:
			literals[i] = fmt.Sprintf("%d::integer", v)
		case []uuid.UUID:
			a := make([]string, len(v))
			for j, id := range v {
				a[j] = quote(id.String())
			}
			literals[i] = "ARRAY[" + strings.Join(a, ",") + "]::uuid[]"
		case []string:
			a := make([]string, len(v))
			for j, s := range v {
				a[j] = quote(s)
			}
			literals[i] = "ARRAY[" + strings.Join(a, ",") + "]::text[]"
		case []bool:
			a := make([]string, len(v))
			for j, b := range v {
				a[j] = fmt.Sprint(b)
			}
			literals[i] = "ARRAY[" + strings.Join(a, ",") + "]::boolean[]"
		case []int32:
			a := make([]string, len(v))
			for j, n := range v {
				a[j] = fmt.Sprint(n)
			}
			literals[i] = "ARRAY[" + strings.Join(a, ",") + "]::integer[]"
		default:
			t.Fatalf("unsupported prepared argument %T", arg)
		}
	}
	return "EXECUTE " + pgx.Identifier{name}.Sanitize() + " (" + strings.Join(literals, ",") + ")"
}

type lintPlanEvidence struct {
	Statistics                 map[string]int
	IndexLevels                map[string]int64
	ParameterBytes             []int
	ParameterEncodingAlloc     uint64
	ExecutionGoAlloc           uint64
	ExecutionWallNS            int64
	ExecuteSQLBytes            int
	Case, Mode, SQL            string
	Parameters                 []string
	GenericPlans, CustomPlans  int64
	Rows                       int
	Settings                   map[string]string
	ExecutionWork, ExplainWork map[string]lintWork
	PlanningWork               map[string]lintWork
	Plan                       json.RawMessage
}

func lintEvidence(t *testing.T, name string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s evidence: %s", name, raw)
}
func lintExecuted(t *testing.T, f fixture, s statement, generic bool) (planNode, lintPlanEvidence) {
	t.Helper()
	ctx := t.Context()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	evidence := lintPlanEvidence{Case: t.Name(), SQL: s.sql, Settings: map[string]string{}}
	evidence.Statistics = lintAssertStatistics(t, f, strings.Contains(t.Name(), "/analyzed"))
	evidence.IndexLevels = lintIndexLevels(t, tx)
	// Diagnose the exact server OIDs and this connection's real pgx encoding.
	// This extra encoding is separate from measured execution allocations.
	described, err := tx.Conn().PgConn().Prepare(ctx, "", s.sql, nil)
	if err != nil {
		t.Fatal(err)
	}
	var encodingBefore, encodingAfter runtime.MemStats
	runtime.ReadMemStats(&encodingBefore)
	for i, arg := range s.args {
		encoded, err := tx.Conn().TypeMap().Encode(described.ParamOIDs[i], pgtype.BinaryFormatCode, arg, nil)
		if err != nil {
			t.Fatal(err)
		}
		evidence.ParameterBytes = append(evidence.ParameterBytes, len(encoded))
	}
	runtime.ReadMemStats(&encodingAfter)
	evidence.ParameterEncodingAlloc = encodingAfter.TotalAlloc - encodingBefore.TotalAlloc
	for _, name := range []string{"track_counts", "enable_seqscan", "enable_indexscan", "enable_bitmapscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin"} {
		var v string
		if err := tx.QueryRow(ctx, "SHOW "+name).Scan(&v); err != nil {
			t.Fatal(err)
		}
		evidence.Settings[name] = v
		if v != "on" {
			t.Fatalf("production planner switch %s=%s", name, v)
		}
	}
	mode := "force_custom_plan"
	if generic {
		mode = "force_generic_plan"
	}
	evidence.Mode = mode
	if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode="+mode); err != nil {
		t.Fatal(err)
	}
	for _, arg := range s.args {
		v := reflect.ValueOf(arg)
		if v.Kind() == reflect.Slice {
			evidence.Parameters = append(evidence.Parameters, fmt.Sprintf("%T length=%d", arg, v.Len()))
		} else {
			evidence.Parameters = append(evidence.Parameters, fmt.Sprintf("%T %v", arg, arg))
		}
	}
	sql, args := s.sql, s.args
	if generic {
		if _, err := tx.Conn().Prepare(ctx, "lint_actual", s.sql); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Conn().Deallocate(context.Background(), "lint_actual") }()
		sql, args = lintExecute(t, "lint_actual", s.args), nil
	}
	// A non-executing plan records relation accesses used to estimate bounds.
	// Keep these separate: Planned's real call below still includes its own
	// planning work, and only its actual results count as execution evidence.
	planningBefore := lintRelationWork(t, tx)
	var planningOnly string
	if err := postgres.Planned(tx).QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+sql, args...).Scan(&planningOnly); err != nil {
		t.Fatal(err)
	}
	evidence.PlanningWork = lintWorkDelta(planningBefore, lintRelationWork(t, tx))
	before := lintRelationWork(t, tx)
	evidence.ExecuteSQLBytes = len(sql)
	var executionBefore, executionAfter runtime.MemStats
	runtime.ReadMemStats(&executionBefore)
	executedAt := time.Now()
	rows, err := postgres.Planned(tx).Query(ctx, sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		if _, err := rows.Values(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		evidence.Rows++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	evidence.ExecutionWallNS = time.Since(executedAt).Nanoseconds()
	runtime.ReadMemStats(&executionAfter)
	evidence.ExecutionGoAlloc = executionAfter.TotalAlloc - executionBefore.TotalAlloc
	after := lintRelationWork(t, tx)
	evidence.ExecutionWork = lintWorkDelta(before, after)
	if generic {
		if err := tx.QueryRow(ctx, `SELECT generic_plans,custom_plans FROM pg_prepared_statements WHERE name='lint_actual'`).Scan(&evidence.GenericPlans, &evidence.CustomPlans); err != nil {
			t.Fatal(err)
		}
		if evidence.GenericPlans < 1 || evidence.CustomPlans != 0 {
			t.Fatalf("not an actually executed generic plan: %+v", evidence)
		}
	}
	var raw string
	if err := postgres.Planned(tx).QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	evidence.ExplainWork = lintWorkDelta(after, lintRelationWork(t, tx))
	evidence.Plan = json.RawMessage(raw)
	var plan []struct {
		Plan planNode `json:"Plan"`
	}
	if err := json.Unmarshal([]byte(raw), &plan); err != nil || len(plan) != 1 {
		t.Fatalf("plan: %s, %v", raw, err)
	}
	lintEvidence(t, "executed-plan", evidence)
	return plan[0].Plan, evidence
}
func lintReaderID(n uint64) uuid.UUID { id := lintID(n); id[0] = 0x10; return id }
func lintPlanFixture(t *testing.T, hidden string, hiddenPages int) fixture {
	return lintPlanFixtureKind(t, hidden, hiddenPages, "wanted")
}

// lintRangeIndexes inspects the real migration's indexes. The optional flag=0
// control removes only the admitted range index; all other definitions stay put.
func lintRangeIndexes(t *testing.T, f fixture) map[string]string {
	t.Helper()
	rows, err := f.pool.Query(t.Context(), `SELECT c.relname,pg_get_indexdef(c.oid),i.indisvalid AND i.indisready
 FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE i.indrelid='page_links'::regclass ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, definition string
		var ready bool
		if err := rows.Scan(&name, &definition, &ready); err != nil {
			t.Fatal(err)
		}
		if !ready {
			t.Fatalf("migration index is not ready/valid: %s", name)
		}
		out[name] = definition
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func lintPlanFixtureKind(t *testing.T, hidden string, hiddenPages int, kind string) fixture {
	t.Helper()
	f := newFixture(t)
	indexes := lintRangeIndexes(t, f)
	definition := indexes[lintRangeIndex]
	if !strings.HasPrefix(definition, "CREATE INDEX "+lintRangeIndex+" ON ") || !strings.HasSuffix(definition, " USING btree (notebook_id, source_id, range_start)") {
		t.Fatalf("production migration 52 did not create the complete range index: %q", definition)
	}
	if os.Getenv("NERVEWIKI_LINT_CANDIDATE_INDEX") == "0" {
		lintExec(t, f, "DROP INDEX "+lintRangeIndex)
		delete(indexes, lintRangeIndex)
		if remaining := lintRangeIndexes(t, f); !reflect.DeepEqual(remaining, indexes) {
			t.Fatalf("old-index control changed unrelated definitions: %+v, want %+v", remaining, indexes)
		}
	}
	lintEvidence(t, "migration-range-index", map[string]any{"definition": definition, "old_index_control": os.Getenv("NERVEWIKI_LINT_CANDIDATE_INDEX") == "0"})
	lintExec(t, f, "CREATE EXTENSION IF NOT EXISTS pageinspect")
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		lintExec(t, f, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ('10000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,true FROM generate_series(1,2000) k`, f.eng)
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ($2||'-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,true FROM generate_series(1,$3::integer) k`, f.ops, hidden, hiddenPages)
	lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,target_key,ambiguous,resolved_id)
 SELECT node_id,k,k+1,notebook_id,'wikilink',CASE WHEN k=1 THEN 'sources.0' ELSE NULL END,'x','x',$1='ambiguous',
 CASE WHEN $1 IN('body','ambiguous') OR ($1='complete' AND k=0) THEN $2::uuid ELSE NULL END
 FROM indexed_pages CROSS JOIN generate_series(0,2) k`, kind, lintReaderID(2000))
	lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value) SELECT node_id,k,notebook_id,CASE WHEN k=1 THEN 'Sources' ELSE 'other' END,'"x"'::jsonb FROM indexed_pages CROSS JOIN generate_series(0,2) k WHERE node_id NOT IN('10000000-0000-0000-0000-000000000001'::uuid,'10000000-0000-0000-0000-000000000002'::uuid)`)
	lintAssertStatistics(t, f, false)
	return f
}
func lintAnalyze(t *testing.T, f fixture) {
	t.Helper()
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		lintExec(t, f, "ANALYZE "+table)
	}
	lintAssertStatistics(t, f, true)
}
func lintWantedStatement(t *testing.T, notebook uuid.UUID, after *app.EdgePos, excluded []uuid.UUID) statement {
	return statementOf(t, func(q *gen.Queries) error {
		if after == nil {
			_, err := q.LintWantedEdgeHeadsFirst(t.Context(), gen.LintWantedEdgeHeadsFirstParams{NotebookID: notebook, Extractor: domain.Extractor, ExcludedIds: excluded, Take: 512})
			return err
		}
		_, err := q.LintWantedEdgeHeadsAfter(t.Context(), gen.LintWantedEdgeHeadsAfterParams{NotebookID: notebook, Extractor: domain.Extractor, ExcludedIds: excluded, Take: 512, AfterSource: after.SourceID, AfterStart: after.Start})
		return err
	})
}
func lintAssertRange(t *testing.T, plan planNode, ev lintPlanEvidence, after, interval bool) {
	lintAssertRangeRows(t, plan, ev, after, interval, 1024)
}
func lintAssertRangeRows(t *testing.T, plan planNode, ev lintPlanEvidence, after, interval bool, maxRows int64) {
	t.Helper()
	scans := plan.all(readsOf("page_links"))
	if len(scans) != 1 {
		t.Fatalf("page_links must have one range driver, got %+v", scans)
	}
	bounded := scans[0].all(func(p *planNode) bool {
		cond := p.IndexCond
		return p.Index != "" && strings.Contains(cond, "notebook_id") && (!after || (strings.Contains(cond, "range_start") && strings.Contains(cond, ">"))) && (!interval || (strings.Contains(cond, ">=") && strings.Contains(cond, "<=")))
	})
	if len(bounded) != 1 {
		t.Errorf("page_links range driver lacks notebook/tuple/batch bounds: %+v", scans[0])
	}
	if ev.ExecutionWork["page_links"].Returned > maxRows || ev.ExecutionWork["page_links"].Fetched > maxRows {
		t.Errorf("one 512-head batch revisited unrelated links: %+v", ev.ExecutionWork["page_links"])
	}
	for name, w := range ev.ExecutionWork {
		if strings.HasPrefix(name, "page_links_") && w.Returned > maxRows {
			t.Errorf("unbounded index traversal %s: %+v", name, w)
		}
	}
}

// lintHiddenFixtures rebuilds pristine relations for each key distribution. The
// reference holds actual execution work, not estimates or a shared analyzed DB.
func lintHiddenFixtures(t *testing.T, kind string, run func(*testing.T, fixture, func(*testing.T, string, lintPlanEvidence))) {
	t.Helper()
	reference := map[string]lintPlanEvidence{}
	for _, hidden := range []struct {
		name, prefix string
		pages        int
	}{{"A1", "00000000", 50000}, {"B", "20000000", 75000}, {"A2", "00000000", 50000}} {
		t.Run(hidden.name, func(t *testing.T) {
			f := lintPlanFixtureKind(t, hidden.prefix, hidden.pages, kind)
			run(t, f, func(t *testing.T, key string, evidence lintPlanEvidence) {
				if hidden.name == "A1" {
					reference[key] = evidence
				} else {
					lintCompareHidden(t, reference[key], evidence)
				}
			})
		})
	}
}

func TestLintPhysicalRanges(t *testing.T) {
	if testing.Short() {
		t.Skip("physical plan fixture")
	}
	t.Run("first-middle-tail-hidden-A-B-A", func(t *testing.T) {
		lintHiddenFixtures(t, "wanted", func(t *testing.T, f fixture, compare func(*testing.T, string, lintPlanEvidence)) {
			for _, statistics := range []string{"no-statistics", "analyzed"} {
				if statistics == "analyzed" {
					lintAnalyze(t, f)
				}
				t.Run(statistics, func(t *testing.T) {
					for _, generic := range []bool{false, true} {
						for _, part := range []struct {
							name  string
							after *app.EdgePos
							rows  int
						}{{"first", nil, 512}, {"middle", &app.EdgePos{SourceID: lintReaderID(1000), Start: 0}, 512}, {"tail", &app.EdgePos{SourceID: lintReaderID(2000), Start: 1}, 1}, {"empty", &app.EdgePos{SourceID: lintReaderID(2000), Start: 2}, 0}} {
							t.Run(fmt.Sprintf("%s/generic=%t", part.name, generic), func(t *testing.T) {
								plan, evidence := lintExecuted(t, f, lintWantedStatement(t, f.eng, part.after, nil), generic)
								if evidence.Rows != part.rows {
									t.Errorf("rows %d, want %d", evidence.Rows, part.rows)
								}
								lintAssertRange(t, plan, evidence, part.after != nil, false)
								key := fmt.Sprintf("%s/%t/%s", statistics, generic, part.name)
								compare(t, key, evidence)
							})
						}
					}
				})
			}
		})
	})
	t.Run("source-slot-disjoint-batch-ranges", func(t *testing.T) {
		lintHiddenFixtures(t, "wanted", func(t *testing.T, f fixture, compare func(*testing.T, string, lintPlanEvidence)) {
			for _, statistics := range []string{"no-statistics", "analyzed"} {
				if statistics == "analyzed" {
					lintAnalyze(t, f)
				}
				t.Run(statistics, func(t *testing.T) {
					for _, generic := range []bool{false, true} {
						for _, first := range []uint64{1, 513, 1025, 1537} {
							ids, keys, arrays, items := []uuid.UUID{}, []string{}, []bool{}, []int32{}
							for n := first; n < first+512 && n <= 2000; n++ {
								ids = append(ids, lintReaderID(n))
								keys = append(keys, "sources")
								arrays = append(arrays, true)
								items = append(items, 1)
							}
							for _, after := range []bool{false, true} {
								t.Run(fmt.Sprintf("%d/after=%t/generic=%t", first, after, generic), func(t *testing.T) {
									s := statementOf(t, func(q *gen.Queries) error {
										if !after {
											_, err := q.LintSourceSlotHeadsFirst(t.Context(), gen.LintSourceSlotHeadsFirstParams{NotebookID: f.eng, Extractor: 2, Take: 512, PageIds: ids, FirstKeys: keys, IsArray: arrays, ItemCounts: items, BatchFirstSource: ids[0], BatchLastSource: ids[len(ids)-1]})
											return err
										}
										_, err := q.LintSourceSlotHeadsAfter(t.Context(), gen.LintSourceSlotHeadsAfterParams{NotebookID: f.eng, Extractor: 2, Take: 512, PageIds: ids, FirstKeys: keys, IsArray: arrays, ItemCounts: items, BatchFirstSource: ids[0], BatchLastSource: ids[len(ids)-1], AfterSource: ids[0], AfterStart: 1})
										return err
									})
									plan, ev := lintExecuted(t, f, s, generic)
									want := len(ids)
									if after {
										want--
									}
									if ev.Rows != want {
										t.Errorf("slot rows %d, want %d", ev.Rows, want)
									}
									// Each source has three edges and only one slot: all three may be
									// visited once in this interval, never any previous JSON interval.
									lintAssertRangeRows(t, plan, ev, after, true, int64(len(ids)*3))
									compare(t, fmt.Sprintf("%s/%t/%d/%t", statistics, generic, first, after), ev)
								})
							}
						}
					}
				})
			}
		})
	})
}

func TestLintPlans(t *testing.T) {
	if testing.Short() {
		t.Skip("plan and physical work fixture")
	}
	t.Run("first-sources-probe", func(t *testing.T) {
		f := lintPlanFixture(t, "00000000", 50000)
		absent, dense := lintReaderID(1), lintReaderID(2)
		lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value) SELECT $1,k,$2,'other','"x"'::jsonb FROM generate_series(0,4999) k`, absent, f.eng)
		lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value) SELECT $1,k,$2,CASE WHEN k>=1000 THEN 'SOURCES' ELSE 'other' END,'"x"'::jsonb FROM generate_series(0,1999) k`, dense, f.eng)
		ids := []uuid.UUID{absent, dense, lintReaderID(3)}
		for _, stats := range []string{"no-statistics", "analyzed"} {
			if stats == "analyzed" {
				lintAnalyze(t, f)
			}
			t.Run(stats, func(t *testing.T) {
				for _, generic := range []bool{false, true} {
					t.Run(fmt.Sprint(generic), func(t *testing.T) {
						s := statementOf(t, func(q *gen.Queries) error {
							_, err := q.LintFirstSourceHeads(t.Context(), gen.LintFirstSourceHeadsParams{NotebookID: f.eng, Extractor: 2, PageIds: ids})
							return err
						})
						plan, ev := lintExecuted(t, f, s, generic)
						if ev.Rows != 2 {
							t.Errorf("first-key probe rows %d, want 2", ev.Rows)
						}
						scans := plan.all(readsOf("page_properties"))
						if len(scans) != 1 || scans[0].Index != "page_properties_sources_idx" || len(plan.unlimited("page_properties")) != 0 {
							t.Errorf("first probe missing partial index / LIMIT 1: %+v", scans)
						}
						if w := ev.ExecutionWork["page_properties_sources_idx"]; w.Returned != 2 || w.Scans != 3 {
							t.Errorf("first probe read extra property rows: %+v", w)
						}
						for name, work := range ev.ExecutionWork {
							if strings.HasPrefix(name, "page_properties") && (work.Returned > 2 || work.Fetched > 2) {
								t.Errorf("first probe traversed unrelated properties via %s: %+v", name, work)
							}
						}
					})
				}
			})
		}
	})
	t.Run("cache-route", func(t *testing.T) {
		f := newFixture(t)
		p := uuid.UUID{}
		target := uuid.NewV7()
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, originFacts(t, "---\nsources: '[[x]]'\n---\n[[y]] [[z]]"))
		f.replace(t, app.Page{ID: target, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
		f.resolve(t, p, domain.Resolution{ID: target, Ambiguous: true}, strings.Index("---\nsources: '[[x]]'\n---\n[[y]] [[z]]", "y"))
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		conn, err := pool.Acquire(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		ctx := postgres.OnConn(t.Context(), conn)
		r := postgresadapter.NewLintReads(pool, 5*1024*1024)
		for i := 0; i < 7; i++ {
			heads, err := r.Heads(ctx, f.eng)
			if err != nil || len(heads) != 2 {
				t.Fatalf("Planned real heads: %+v, %v", heads, err)
			}
			first, err := r.WantedEdgeHeads(ctx, f.eng, nil, nil, 1)
			if err != nil || len(first) != 1 {
				t.Fatalf("Planned wanted first: %+v,%v", first, err)
			}
			rest, err := r.WantedEdgeHeads(ctx, f.eng, nil, &first[0].EdgePos, 512)
			if err != nil || len(rest) != 1 {
				t.Fatalf("Planned wanted after: %+v,%v", rest, err)
			}
			if rows, err := r.Edges(ctx, f.eng, []app.EdgePos{first[0].EdgePos}); err != nil || len(rows) != 1 {
				t.Fatalf("Planned exact: %d %v", len(rows), err)
			}
			sources, err := r.SourcePropertyHeads(ctx, f.eng, []uuid.UUID{p})
			if err != nil || len(sources) != 1 {
				t.Fatalf("Planned JSON head: %+v,%v", sources, err)
			}
			if values, err := r.SourceProperties(ctx, f.eng, []app.SourcePropertyPos{sources[0].SourcePropertyPos}); err != nil || len(values) != 1 {
				t.Fatalf("Planned JSON: %d %v", len(values), err)
			}
			slot, err := r.SourceSlotHeads(ctx, f.eng, []app.SourceSpec{{PageID: p, Key: "sources"}}, nil, 512)
			if err != nil || len(slot) != 1 {
				t.Fatalf("Planned slot: %+v %v", slot, err)
			}
			if rows, err := r.SourceSlotEdges(ctx, f.eng, []app.EdgePos{slot[0].EdgePos}); err != nil || len(rows) != 1 {
				t.Fatalf("Planned slot exact: %d %v", len(rows), err)
			}
			if rows, err := r.BodyReferences(ctx, f.eng, []uuid.UUID{p}, nil, 512); err != nil || len(rows) != 1 {
				t.Fatalf("Planned short body: %d %v", len(rows), err)
			}
			if rows, err := r.AmbiguousEdgeHeads(ctx, f.eng, nil, nil, 512); err != nil || len(rows) != 1 {
				t.Fatalf("Planned ambiguous: %d %v", len(rows), err)
			}
		}
		var cached int
		if err := conn.QueryRow(t.Context(), `SELECT count(*) FROM pg_prepared_statements WHERE statement LIKE '-- name: Lint%'`).Scan(&cached); err != nil {
			t.Fatal(err)
		}
		if cached != 0 || len(tr.calls) != 70 || len(tr.modes) != 70 {
			t.Fatalf("Planned route: cached=%d calls=%d modes=%d", cached, len(tr.calls), len(tr.modes))
		}
		for _, mode := range tr.modes {
			if mode != pgx.QueryExecModeDescribeExec {
				t.Fatalf("mode=%v", mode)
			}
		}
		// A genuine default-cached call is the positive control for named cache.
		if _, err := gen.New(conn).LintHeads(t.Context(), f.eng); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(t.Context(), `SELECT count(*) FROM pg_prepared_statements WHERE statement LIKE '-- name: Lint%'`).Scan(&cached); err != nil || cached == 0 {
			t.Fatalf("cached positive control: %d %v", cached, err)
		}
	})
}

func TestLintPhysicalRangesCompleteSources(t *testing.T) {
	if testing.Short() {
		t.Skip("complete 100k source parameter fixture")
	}
	for _, n := range []int{0, 10000, 100000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			f := lintPlanFixtureKind(t, "00000000", 50000, "complete")
			excluded := make([]uuid.UUID, n)
			for i := range excluded {
				excluded[i] = lintID(uint64(i + 1))
			}
			for _, stats := range []string{"no-statistics", "analyzed"} {
				if stats == "analyzed" {
					lintAnalyze(t, f)
				}
				t.Run(stats, func(t *testing.T) {
					for _, generic := range []bool{false, true} {
						t.Run(fmt.Sprint(generic), func(t *testing.T) {
							wantedPlan, ev := lintExecuted(t, f, lintWantedStatement(t, f.eng, &app.EdgePos{SourceID: lintReaderID(1000), Start: 0}, excluded), generic)
							lintAssertRange(t, wantedPlan, ev, true, false)
							lintAssertMembership(t, ev, "excluded", n)
							if ev.Rows != 512 {
								t.Errorf("complete exclusion relation lost unrelated emitters: %d", ev.Rows)
							}
							// 2,000 actual emitters plus a complete large unrelated set must remain
							// a relation, never a per-edge linear membership scan.
							emitters := append([]uuid.UUID(nil), excluded...)
							for k := uint64(1); k <= 2000; k++ {
								emitters = append(emitters, lintReaderID(k))
							}
							s := statementOf(t, func(q *gen.Queries) error {
								_, err := q.LintBodyReferencesAfter(t.Context(), gen.LintBodyReferencesAfterParams{NotebookID: f.eng, Extractor: 2, WikiPageIds: emitters, AfterSource: lintReaderID(1000), AfterStart: 0, Take: 512})
								return err
							})
							plan, body := lintExecuted(t, f, s, generic)
							lintAssertRangeRows(t, plan, body, true, false, 3*512+2)
							lintAssertMembership(t, body, "emitters", n+2000)
							if body.Rows != 512 {
								t.Errorf("complete emitter relation lost body references: %d", body.Rows)
							}
							if strings.Contains(s.sql, " ALL(") || len(plan.all(func(p *planNode) bool { return p.Type == "CTE Scan" })) == 0 {
								t.Error("complete source IDs were not materialized as a relation")
							}
						})
					}
				})
			}
		})
	}
}

func lintAssertStatistics(t *testing.T, f fixture, analyzed bool) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		var disabled bool
		if err := f.pool.QueryRow(t.Context(), `SELECT coalesce(reloptions,'{}') @> ARRAY['autovacuum_enabled=false'] FROM pg_class WHERE oid=$1::regclass`, table).Scan(&disabled); err != nil || !disabled {
			t.Fatalf("automatic analyze guard %s=%t: %v", table, disabled, err)
		}
		var count int
		if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM pg_stats WHERE schemaname=current_schema() AND tablename=$1`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		out[table] = count
		if (count > 0) != analyzed {
			t.Fatalf("actual statistics for %s=%d, analyzed=%t", table, count, analyzed)
		}
	}
	return out
}
func lintIndexLevels(t *testing.T, tx pgx.Tx) map[string]int64 {
	t.Helper()
	rows, err := tx.Query(t.Context(), `SELECT c.relname,(bt_metap(c.oid::regclass::text)).level FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE i.indrelid IN('indexed_pages'::regclass,'page_links'::regclass,'page_properties'::regclass) ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var name string
		var level int64
		if err := rows.Scan(&name, &level); err != nil {
			t.Fatal(err)
		}
		out[name] = level
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// lintWorkWithoutPlannerProbes adjusts only scan/tuple counts using the
// independently observed non-executing EXPLAIN. Blocks/hits remain raw: cache
// state changes between calls, so subtraction cannot identify exact executor
// blocks. Generic EXECUTE does not repeat the diagnostic's planning probes.
func lintWorkWithoutPlannerProbes(t *testing.T, evidence lintPlanEvidence) map[string]lintWork {
	t.Helper()
	out := make(map[string]lintWork, len(evidence.ExecutionWork))
	for name, work := range evidence.ExecutionWork {
		planning := evidence.PlanningWork[name]
		// The widest slot range has three source endpoints (lower, upper,
		// After). On pristine rows each estimate needs at most one endpoint
		// probe. Count planner work too; its bound has no hidden-row term.
		blocks := int64(3)*(evidence.IndexLevels[name]+1) + 8
		if planning.Scans > 3 || planning.Returned > 3 || planning.Fetched > 3 || planning.Blocks > blocks {
			t.Errorf("unbounded planning work for %s: %+v, block bound=%d", name, planning, blocks)
		}
		if evidence.Mode == "force_custom_plan" {
			work = lintWork{work.Scans - planning.Scans, work.Returned - planning.Returned, work.Fetched - planning.Fetched, work.Blocks, work.Hits}
			if work.Scans < 0 || work.Returned < 0 || work.Fetched < 0 {
				t.Errorf("planning probes do not account for %s request work: %+v minus %+v", name, evidence.ExecutionWork[name], planning)
			}
		}
		out[name] = work
	}
	return out
}

func lintCompareHidden(t *testing.T, a, b lintPlanEvidence) {
	t.Helper()
	if a.Case == "" {
		t.Fatal("missing A baseline")
	}
	aWork, bWork := lintWorkWithoutPlannerProbes(t, a), lintWorkWithoutPlannerProbes(t, b)
	for name, work := range bWork {
		if strings.HasPrefix(name, "pg_toast") {
			continue
		}
		old := aWork[name]
		// Bound growth, not savings: a normal planner may choose Memoize,
		// which reduces repeated current-page probes for the same prefix.
		// At most two extra boundary entries locate the same pristine keys.
		if work.Scans > old.Scans+2 || work.Returned > old.Returned+2 || work.Fetched > old.Fetched+2 {
			t.Errorf("hidden notebook increased %s work: A=%+v, current=%+v", name, old, work)
		}
		// An extra btree level costs at most one block per scan. Eight pages
		// allow the fixed root/leaf boundary layout, never a term in hidden rows.
		extraLevels := max(int64(0), b.IndexLevels[name]-a.IndexLevels[name])
		// Preserve raw total blocks and allow this separately measured,
		// independently bounded planning work in addition to the fixed
		// execution boundary. This is not exact same-call subtraction.
		planningBlocks := int64(0)
		if b.Mode == "force_custom_plan" {
			planningBlocks = b.PlanningWork[name].Blocks
		}
		bound := old.Blocks + extraLevels*work.Scans + 8 + planningBlocks
		if work.Blocks > bound {
			t.Errorf("hidden notebook inflated %s blocks: A=%d, current=%d, bound=%d, levels=%d→%d, scans=%d", name, old.Blocks, work.Blocks, bound, a.IndexLevels[name], b.IndexLevels[name], work.Scans)
		}
	}
	lintEvidence(t, "hidden-comparison", map[string]any{"reference": a.Case, "current": b.Case, "reference_levels": a.IndexLevels, "current_levels": b.IndexLevels, "reference_work": a.ExecutionWork, "current_work": b.ExecutionWork, "reference_planning": a.PlanningWork, "current_planning": b.PlanningWork, "reference_probe_adjusted_counts_raw_blocks": aWork, "current_probe_adjusted_counts_raw_blocks": bWork, "block_allowance": "positive level delta × scans + 8 fixed boundary pages + independently bounded non-executing EXPLAIN planning blocks; no same-call executor timing claim"})
}

func TestLintPhysicalRangesBodyAndAmbiguous(t *testing.T) {
	if testing.Short() {
		t.Skip("body/ambiguous range matrix")
	}
	for _, kind := range []string{"body", "ambiguous"} {
		t.Run(kind, func(t *testing.T) {
			lintHiddenFixtures(t, kind, func(t *testing.T, f fixture, compare func(*testing.T, string, lintPlanEvidence)) {
				emitters := make([]uuid.UUID, 2000)
				for i := range emitters {
					emitters[i] = lintReaderID(uint64(i + 1))
				}
				for _, stats := range []string{"no-statistics", "analyzed"} {
					if stats == "analyzed" {
						lintAnalyze(t, f)
					}
					t.Run(stats, func(t *testing.T) {
						for _, generic := range []bool{false, true} {
							for _, part := range []struct {
								name  string
								after *app.EdgePos
								rows  int
							}{{"first", nil, 512}, {"middle", &app.EdgePos{SourceID: lintReaderID(1000), Start: 0}, 512}, {"tail", &app.EdgePos{SourceID: lintReaderID(2000), Start: 1}, 1}, {"empty", &app.EdgePos{SourceID: lintReaderID(2000), Start: 2}, 0}} {
								t.Run(fmt.Sprintf("%s/generic=%t", part.name, generic), func(t *testing.T) {
									s := statementOf(t, func(q *gen.Queries) error {
										if kind == "body" {
											if part.after == nil {
												_, err := q.LintBodyReferencesFirst(t.Context(), gen.LintBodyReferencesFirstParams{NotebookID: f.eng, Extractor: 2, Take: 512, WikiPageIds: emitters})
												return err
											}
											_, err := q.LintBodyReferencesAfter(t.Context(), gen.LintBodyReferencesAfterParams{NotebookID: f.eng, Extractor: 2, Take: 512, WikiPageIds: emitters, AfterSource: part.after.SourceID, AfterStart: part.after.Start})
											return err
										}
										if part.after == nil {
											_, err := q.LintAmbiguousEdgeHeadsFirst(t.Context(), gen.LintAmbiguousEdgeHeadsFirstParams{NotebookID: f.eng, Extractor: 2, Take: 512})
											return err
										}
										_, err := q.LintAmbiguousEdgeHeadsAfter(t.Context(), gen.LintAmbiguousEdgeHeadsAfterParams{NotebookID: f.eng, Extractor: 2, Take: 512, AfterSource: part.after.SourceID, AfterStart: part.after.Start})
										return err
									})
									plan, ev := lintExecuted(t, f, s, generic)
									if ev.Rows != part.rows {
										t.Errorf("%s rows=%d want=%d", kind, ev.Rows, part.rows)
									}
									lintAssertRange(t, plan, ev, part.after != nil, false)
									compare(t, fmt.Sprintf("%s/%t/%s", stats, generic, part.name), ev)
								})
							}
						}
					})
				}
			})
		})
	}
}

func TestLintPlansExactPositions(t *testing.T) {
	if testing.Short() {
		t.Skip("exact position plan matrix")
	}
	f := lintPlanFixture(t, "00000000", 50000)
	ids, starts, positions := make([]uuid.UUID, 512), make([]int32, 512), make([]int32, 512)
	for i := range ids {
		ids[i] = lintReaderID(uint64(i + 3))
		starts[i] = 1
		positions[i] = 1
	}
	for _, stats := range []string{"no-statistics", "analyzed"} {
		if stats == "analyzed" {
			lintAnalyze(t, f)
		}
		t.Run(stats, func(t *testing.T) {
			for _, generic := range []bool{false, true} {
				for _, kind := range []string{"wanted", "source-slot", "source-JSON"} {
					t.Run(fmt.Sprintf("%s/generic=%t", kind, generic), func(t *testing.T) {
						table := "page_links"
						if kind == "source-JSON" {
							table = "page_properties"
						}
						s := statementOf(t, func(q *gen.Queries) error {
							switch kind {
							case "wanted":
								_, err := q.LintEdgesByPositions(t.Context(), gen.LintEdgesByPositionsParams{NotebookID: f.eng, SourceIds: ids, Starts: starts})
								return err
							case "source-slot":
								_, err := q.LintSourceSlotEdges(t.Context(), gen.LintSourceSlotEdgesParams{NotebookID: f.eng, PageIds: ids, Starts: starts})
								return err
							default:
								_, err := q.LintFirstSourceValues(t.Context(), gen.LintFirstSourceValuesParams{NotebookID: f.eng, Extractor: 2, PageIds: ids, Positions: positions})
								return err
							}
						})
						plan, ev := lintExecuted(t, f, s, generic)
						if ev.Rows != 512 || len(plan.unlimited(table)) != 0 {
							t.Errorf("exact %s is not 512 limited primary-key reads", kind)
						}
						scans := plan.all(readsOf(table))
						if len(scans) != 1 || scans[0].Index != table+"_pkey" {
							t.Errorf("exact %s has wrong physical path: %+v", kind, scans)
						}
						for name, w := range ev.ExecutionWork {
							if strings.HasPrefix(name, table) && (w.Returned > 514 || w.Fetched > 514) {
								t.Errorf("exact %s read unrelated rows via %s: %+v", kind, name, w)
							}
						}
					})
				}
			}
		})
	}
}

func TestLintPhysicalRangesProductionBatches(t *testing.T) {
	if testing.Short() {
		t.Skip("complete source sets through real bounded streams")
	}
	f := lintPlanFixtureKind(t, "00000000", 50000, "complete")
	excluded := make([]uuid.UUID, 100000)
	for i := range excluded {
		excluded[i] = lintID(uint64(i + 1))
	}
	emitters := append([]uuid.UUID(nil), excluded...)
	for k := uint64(1); k <= 2000; k++ {
		emitters = append(emitters, lintReaderID(k))
	}
	tr := &lintTrace{}
	pool := lintPool(t, f, tr, 1)
	r := app.NewLintReads(postgresadapter.NewLintReads(pool, 5*1024*1024), 5*1024*1024)
	for _, stats := range []string{"no-statistics", "analyzed"} {
		if stats == "analyzed" {
			lintAnalyze(t, f)
		}
		t.Run(stats, func(t *testing.T) {
			lintAssertStatistics(t, f, stats == "analyzed")
			startCalls := len(tr.names())
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			started := time.Now()
			wanted := r.Wanted(f.eng, excluded)
			wantedRows, wantedBatches := 0, 0
			for {
				rows, err := wanted.Next(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) == 0 {
					break
				}
				wantedRows += len(rows)
				wantedBatches++
			}
			bodies := r.Bodies(f.eng, emitters)
			bodyRows, bodyBatches := 0, 0
			for {
				rows, err := bodies.Next(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) == 0 {
					break
				}
				bodyRows += len(rows)
				bodyBatches++
			}
			elapsed := time.Since(started)
			runtime.ReadMemStats(&after)
			if wantedRows != 4000 || wantedBatches != 8 || bodyRows != 2000 || bodyBatches != 4 {
				t.Fatalf("complete source streams: wanted=%d/%d body=%d/%d", wantedRows, wantedBatches, bodyRows, bodyBatches)
			}
			calls := tr.names()[startCalls:]
			if len(calls) != 22 {
				t.Fatalf("actual SQL round trips: %d %v", len(calls), calls)
			}
			lintEvidence(t, "production-100k-parameter-batches", map[string]any{"excluded": len(excluded), "emitters": len(emitters), "wanted_rows": wantedRows, "wanted_batches": wantedBatches, "body_rows": bodyRows, "body_batches": bodyBatches, "statement_calls": calls, "go_total_alloc": after.TotalAlloc - before.TotalAlloc, "wall_ns": elapsed.Nanoseconds(), "scope": "complete source slices through app bounded readers and real Planned queries; includes repeated pgx serialization and relation construction; not rule/presence execution"})
		})
	}
}

func lintAssertMembership(t *testing.T, evidence lintPlanEvidence, relation string, n int) {
	t.Helper()
	type node struct {
		Type  string            `json:"Node Type"`
		CTE   string            `json:"CTE Name"`
		Rows  float64           `json:"Actual Rows"`
		Loops float64           `json:"Actual Loops"`
		Plans []json.RawMessage `json:"Plans"`
	}
	var document []struct {
		Plan json.RawMessage `json:"Plan"`
	}
	if err := json.Unmarshal(evidence.Plan, &document); err != nil || len(document) != 1 {
		t.Fatalf("membership plan document: %v", err)
	}
	scans := 0
	var visit func(json.RawMessage)
	visit = func(raw json.RawMessage) {
		var p node
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatal(err)
		}
		if p.Type == "CTE Scan" && p.CTE == relation {
			scans++
			if p.Loops != 1 || p.Rows != float64(n) {
				t.Errorf("%s must be built/read once, got %.0f rows × %.0f loops; want %d × 1", relation, p.Rows, p.Loops, n)
			}
		}
		for _, child := range p.Plans {
			visit(child)
		}
	}
	visit(document[0].Plan)
	if scans != 1 || !strings.Contains(string(evidence.Plan), "hashed SubPlan") {
		t.Errorf("complete %s membership is not an actual once-built hashed subplan (scans=%d)", relation, scans)
	}
}
