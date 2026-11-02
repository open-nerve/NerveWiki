//go:build !race

package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type virtualRelationWork struct{ Scans, Returned, Fetched, Blocks, Hits int64 }

func virtualPhysicalWork(t *testing.T, tx pgx.Tx) map[string]virtualRelationWork {
	t.Helper()
	rows, err := tx.Query(t.Context(), `WITH tables AS (
 SELECT oid FROM pg_class WHERE relname=ANY($1::text[])
 UNION SELECT reltoastrelid FROM pg_class WHERE relname=ANY($1::text[]) AND reltoastrelid<>0),rels AS (
 SELECT oid FROM tables UNION SELECT indexrelid FROM pg_index WHERE indrelid IN(SELECT oid FROM tables))
 SELECT c.relname,pg_stat_get_xact_numscans(c.oid),pg_stat_get_xact_tuples_returned(c.oid),pg_stat_get_xact_tuples_fetched(c.oid),pg_stat_get_xact_blocks_fetched(c.oid),pg_stat_get_xact_blocks_hit(c.oid)
 FROM rels r JOIN pg_class c ON c.oid=r.oid ORDER BY c.relname`, virtualCostTables())
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]virtualRelationWork{}
	for rows.Next() {
		var name string
		var w virtualRelationWork
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
func virtualWorkDelta(before, after map[string]virtualRelationWork) map[string]virtualRelationWork {
	out := map[string]virtualRelationWork{}
	for name, w := range after {
		old := before[name]
		out[name] = virtualRelationWork{w.Scans - old.Scans, w.Returned - old.Returned, w.Fetched - old.Fetched, w.Blocks - old.Blocks, w.Hits - old.Hits}
	}
	return out
}

// The complete captured arrays are encoded independently after measurement.
// Actual custom and generic executions then consume the same SQL/parameters;
// named prepared counters distinguish them. EXPLAIN is a separate re-read,
// with its physical work kept separate from the production/plan execution.
func (f *virtualCostFixture) plans(t *testing.T, name, state string, statements []virtualCostStatement) {
	t.Helper()
	virtualPlannedQueries := []string{"IndexCatalogueHeads", "IndexPages", "IndexItems", "InboundCounts", "ListNodes", "LintNodeMetadata", "PageStates", "ListChangesets", "ChangesetFacts", "FirstChanges", "LinkTargetsByIDs", "ContentSizes", "Contents"}
	selected := map[string]virtualCostStatement{}
	var parameterStatements []virtualCostStatement
	for _, s := range statements {
		if !slices.Contains(virtualPlannedQueries, s.Name) {
			continue
		}
		parameterStatements = append(parameterStatements, s)
		key := s.Name
		for _, arg := range s.Args {
			if ids, ok := arg.([]uuid.UUID); ok {
				key += fmt.Sprintf("/%d", len(ids))
			}
		}
		if _, ok := selected[key]; !ok {
			selected[key] = s
		}
	}
	required := []string{"IndexPages", "IndexItems", "InboundCounts"}
	if name == "virtual-read-log" {
		required = []string{"ListChangesets", "ChangesetFacts", "FirstChanges", "LinkTargetsByIDs"}
	}
	if name == "virtual-export-whole" {
		required = []string{"IndexPages", "IndexItems", "ListNodes", "LintNodeMetadata", "PageStates", "ListChangesets", "ChangesetFacts", "FirstChanges", "Contents"}
	}
	if name == "virtual-directory" {
		required = append(required, "IndexCatalogueHeads", "LintNodeMetadata")
	}
	for _, q := range required {
		if !slices.ContainsFunc(parameterStatements, func(s virtualCostStatement) bool { return s.Name == q && s.Rows > 0 }) {
			t.Fatalf("actual %s never executed nonempty %s", name, q)
		}
	}
	conn, err := f.tm.pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	described := map[string][]uint32{}
	var evidence []map[string]any
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for _, s := range parameterStatements {
		oids := described[s.SQL]
		if oids == nil {
			d, e := conn.Conn().PgConn().Prepare(t.Context(), "", s.SQL, nil)
			if e != nil {
				t.Fatal(e)
			}
			oids = d.ParamOIDs
			described[s.SQL] = oids
		}
		if len(oids) != len(s.Args) {
			t.Fatalf("actual %s SQL/args mismatch %d/%d", s.Name, len(oids), len(s.Args))
		}
		hash := sha256.New()
		var sizes []int
		var arrays []int
		for i, a := range s.Args {
			b, e := conn.Conn().TypeMap().Encode(oids[i], pgtype.BinaryFormatCode, a, nil)
			if e != nil {
				t.Fatalf("actual %s parameter%d: %v", s.Name, i, e)
			}
			sizes = append(sizes, len(b))
			_, _ = hash.Write(b)
			if ids, ok := a.([]uuid.UUID); ok {
				arrays = append(arrays, len(ids))
			}
		}
		evidence = append(evidence, map[string]any{"Name": s.Name, "OIDs": oids, "ParameterBytes": sizes, "ArrayElements": arrays, "EncodedParametersSHA256": fmt.Sprintf("%x", hash.Sum(nil))})
	}
	runtime.ReadMemStats(&after)
	conn.Release()
	virtualCostEvent(t, "parameters", name, state, map[string]any{"Calls": evidence, "EncodingTotalAllocBytes": after.TotalAlloc - before.TotalAlloc, "Scope": "all captured service arrays; diagnostic encoding outside algorithm sample; PostgreSQL binary framing included"})
	keys := make([]string, 0, len(selected))
	for key := range selected {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		for _, mode := range []string{"force_custom_plan", "force_generic_plan"} {
			t.Run("plan/"+key+"/"+mode, func(t *testing.T) { f.explain(t, name, state, mode, selected[key]) })
		}
	}
}

func (f *virtualCostFixture) explain(t *testing.T, name, state, mode string, s virtualCostStatement) {
	t.Helper()
	tx, err := f.tm.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if mode != "force_custom_plan" && mode != "force_generic_plan" {
		t.Fatal("unreviewed plan mode")
	}
	if _, err := tx.Exec(t.Context(), "SET LOCAL plan_cache_mode="+mode); err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{}
	for _, setting := range []string{"plan_cache_mode", "enable_seqscan", "enable_indexscan", "enable_bitmapscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin", "track_counts"} {
		var v string
		if err := tx.QueryRow(t.Context(), "SHOW "+setting).Scan(&v); err != nil {
			t.Fatal(err)
		}
		settings[setting] = v
		if setting == "plan_cache_mode" && v != mode || setting != "plan_cache_mode" && v != "on" {
			t.Fatalf("actual planner setting %s=%s", setting, v)
		}
	}
	desc, err := tx.Conn().Prepare(t.Context(), "p4_virtual_cost", s.SQL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Conn().Deallocate(context.Background(), "p4_virtual_cost") }()
	prepared := func() [2]int64 {
		var v [2]int64
		if err := tx.QueryRow(t.Context(), `SELECT custom_plans,generic_plans FROM pg_prepared_statements WHERE name='p4_virtual_cost'`).Scan(&v[0], &v[1]); err != nil {
			t.Fatal(err)
		}
		return v
	}
	beforePlanning := virtualPhysicalWork(t, tx)
	var planning json.RawMessage
	if err := postgres.Planned(tx).QueryRow(t.Context(), "EXPLAIN (FORMAT JSON) "+s.SQL, s.Args...).Scan(&planning); err != nil {
		t.Fatal(err)
	}
	before := virtualPhysicalWork(t, tx)
	plansBefore := prepared()
	at := time.Now()
	rows, err := tx.Query(t.Context(), "p4_virtual_cost", s.Args...)
	if err != nil {
		t.Fatal(err)
	}
	count := int64(0)
	for rows.Next() {
		if _, err := rows.Values(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		count++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	elapsed := int64(time.Since(at))
	plansAfter := prepared()
	after := virtualPhysicalWork(t, tx)
	if count != s.Rows || count == 0 {
		t.Fatalf("actual %s %s rows=%d, observed call=%d", s.Name, mode, count, s.Rows)
	}
	if mode == "force_custom_plan" && (plansAfter[0]-plansBefore[0] != 1 || plansAfter[1] != plansBefore[1]) || mode == "force_generic_plan" && (plansAfter[1]-plansBefore[1] != 1 || plansAfter[0] != plansBefore[0]) {
		t.Fatalf("not the actual named %s execution: %v -> %v", mode, plansBefore, plansAfter)
	}
	var raw json.RawMessage
	if err := postgres.Planned(tx).QueryRow(t.Context(), "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+s.SQL, s.Args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	work := virtualWorkDelta(before, after)
	virtualCostEvent(t, "plan", name, state, map[string]any{"Name": s.Name, "SQL": s.SQL, "Args": s.Args, "OIDs": desc.ParamOIDs, "Mode": mode, "Settings": settings, "PreparedBefore": plansBefore, "PreparedAfter": plansAfter, "Rows": count, "ExecutionWallNS": elapsed, "PlanningOnlyWork": virtualWorkDelta(beforePlanning, before), "ExecutionWork": work, "ExplainWork": virtualWorkDelta(after, virtualPhysicalWork(t, tx)), "Plan": raw, "Scope": "actual named prepared execution; separate raw SQL EXPLAIN ANALYZE with identical parameters/mode, including buffers/temp; SQL+Scan wall is not PG CPU"})
	observed := false
	maxRows := int64(200004)
	if s.Name != "ListNodes" && s.Name != "LintNodeMetadata" && s.Name != "IndexPages" {
		var ids int
		for _, arg := range s.Args {
			if v, ok := arg.([]uuid.UUID); ok {
				ids = max(ids, len(v))
			}
		}
		if ids > 0 {
			maxRows = int64(max(1000, ids*20))
			if s.Name == "LinkTargetsByIDs" {
				// The existing notebook + ANY query can choose a notebook
				// index scan without statistics. The target has 100002 nodes;
				// each requested node can additionally visit at most ten
				// ancestors in this valid-depth fixture. The larger foreign
				// notebook is not part of this allowance.
				maxRows = 100002 + int64(ids)*10
			}
			if s.Name == "InboundCounts" {
				// Each recursive seek returns at most one distinct source. The
				// fixed count stops at 1000 other pages; one self source can
				// additionally be visited without advancing that count.
				if len(s.Args) != 2 || s.Args[1] != int32(1000) {
					t.Fatalf("actual inbound SQL cap: %#v", s.Args)
				}
				maxRows = int64(ids) * 1001
			}
		}
	}
	for relation, w := range work {
		observed = observed || w.Fetched > 0 || w.Returned > 0
		if s.Name == "LinkTargetsByIDs" && !strings.HasPrefix(relation, "nodes") && (w.Returned != 0 || w.Fetched != 0) {
			t.Fatalf("path lookup queried unrelated relation %s: %+v", relation, w)
		}
		if s.Name == "InboundCounts" && !strings.HasPrefix(relation, "page_links") && (w.Returned != 0 || w.Fetched != 0) {
			t.Fatalf("inbound queried unrelated relation %s: %+v", relation, w)
		}
		if !strings.HasPrefix(relation, "pg_toast") && (w.Returned > maxRows || w.Fetched > maxRows) {
			t.Fatalf("actual %s read outside fixture-shaped physical bound %d through %s: %+v", s.Name, maxRows, relation, w)
		}
	}
	if !observed {
		t.Fatal("physical observer saw no tuple/index work")
	}
}
