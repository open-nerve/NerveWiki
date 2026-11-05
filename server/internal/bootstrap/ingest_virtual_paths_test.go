//go:build !race

package bootstrap

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The real first write of a source overlaps the three changed pages and
// five preview sources: eight entries must become seven path IDs. A sixth
// live source remains outside the preview. The larger other book is real.
func TestIngestVirtualLogPaths(t *testing.T) {
	trace := &virtualCostTrace{}
	load := newVirtualExportLoadConfigured(t, "BoundedIngestWiki", nil, trace, nil)
	tables := append(virtualCostTables(), "changeset_sources")
	for _, table := range tables {
		load.tm.originExec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	load.seed(t, 10000, false)
	other := &virtualExportLoad{tm: load.tm}
	other.book = load.tm.llmWiki(t, "ForeignIngestWiki")
	other.prototype = load.tm.createPageWith(t, "alice", other.book.id, "", "Prototype", "---\ntype: note\nsummary: bounded fixture\n---\nbody")
	other.seed(t, 20000, false)
	f := ingestVirtualSources(t, load.tm, load.book, true)
	f.m.handshakeAs(t, "whole-program")
	for _, id := range f.ids[1:] {
		written := answerOf[mcpWritten](t, f.m.use(t, "write", map[string]any{"path": "raw/" + f.titles[id] + ".md", "content": "source:" + f.titles[id] + "\n", "base_version": id + ":1"}))
		if !written.Changed || written.NodeID != id || written.Version != id+":2" {
			t.Fatalf("legal source first write: %+v", written)
		}
	}
	helper := load.tm.createPageWith(t, "alice", f.book.id, "", "Helper", "helper before")
	batch := answerOf[mcpBatched](t, f.m.use(t, "batch", map[string]any{
		"message": "bounded source paths", "ops": []map[string]any{
			opWriteOn("raw/"+f.titles[f.ids[0]]+".md", "source:"+f.titles[f.ids[0]]+"\n", f.ids[0]+":1"),
			opWriteOn("Helper.md", "helper after", helper+":1"),
			opWriteOn("Summary.md", "[[raw/S00]]\n[[raw/S01]]\n[[raw/S02]]\n[[raw/S03]]\n[[raw/S04]]\n[[raw/S05]]\n", f.note+":1"),
		},
	}))
	if !batch.Changed || batch.ChangesetID == nil || batch.At == nil || batch.ResultsCount != 3 || len(batch.Results) != 3 {
		t.Fatalf("real 3+5 producer: %+v", batch)
	}
	for i, id := range []string{f.ids[0], helper, f.note} {
		r := batch.Results[i]
		if !r.Changed || r.NodeID != id || r.Version == nil || *r.Version != id+":2" {
			t.Fatalf("real batch op%d: %+v", i, r)
		}
	}
	f.cs = *batch.ChangesetID
	f.marks(t)
	if count(t, f.tm.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id=$1", f.cs) != 0 {
		t.Fatal("content-only batch unexpectedly wrote node items")
	}
	for _, pair := range []struct {
		book string
		rows int
	}{{f.book.id, 10011}, {other.book.id, 20003}} {
		if got := count(t, f.tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND deleted_at IS NULL", pair.book); got != pair.rows {
			t.Fatalf("actual notebook rows=%d, want%d", got, pair.rows)
		}
	}
	if count(t, f.tm.pool, "SELECT count(*) FROM nodes") != 30014 {
		t.Fatal("actual main+foreign fixture is not30014 nodes")
	}
	if count(t, f.tm.pool, "SELECT count(*) FROM page_revisions WHERE changeset_id=$1 AND deleted_at IS NULL", f.cs) != 3 {
		t.Fatal("batch did not retain exactly three real content changes")
	}

	wantIDs := append(slices.Clone(f.ids[:5]), helper, f.note)
	slices.Sort(wantIDs)
	wantChanges := []page.VirtualLogChange{}
	for _, id := range []string{f.ids[0], helper, f.note} {
		name, path := "Summary", "Summary.md"
		if id == helper {
			name, path = "Helper", "Helper.md"
		}
		if id == f.ids[0] {
			name, path = f.titles[id], "raw/"+f.titles[id]+".md"
		}
		wantChanges = append(wantChanges, page.VirtualLogChange{NodeID: uuid.MustParse(id), Name: name, Kind: "content", Path: &path, Asset: false})
	}
	slices.SortFunc(wantChanges, func(a, b page.VirtualLogChange) int { return strings.Compare(a.NodeID.String(), b.NodeID.String()) })
	wantSources := make([]page.VirtualLogSource, 5)
	for i, id := range f.ids[:5] {
		u, path := uuid.MustParse(id), "raw/"+f.titles[id]+".md"
		wantSources[i] = page.VirtualLogSource{NodeID: &u, Path: &path, Asset: false}
	}
	actor := queryStrings(t, f.tm.pool, "SELECT display_name FROM users WHERE email='alice@example.com'")
	if len(actor) != 1 || actor[0] == "" {
		t.Fatal("real actor fixture is absent")
	}
	message := "bounded source paths"
	want := []page.VirtualLogChangeset{{ID: uuid.MustParse(f.cs), Kind: "edit", Client: "mcp:whole-program", ByName: actor[0], CreatedAt: batch.At.UTC(),
		Message: &message, NodeCount: 0, ContentCount: 3, IngestCount: 6, Changes: wantChanges, IngestSources: wantSources}}

	for _, state := range []string{"no_statistics", "analyzed"} {
		t.Run(state, func(t *testing.T) {
			stats := map[string]int{}
			for _, table := range tables {
				if state == "analyzed" {
					f.tm.originExec(t, "ANALYZE "+table)
				}
				var disabled bool
				if err := f.tm.pool.QueryRow(t.Context(), "SELECT coalesce(reloptions,'{}') @> ARRAY['autovacuum_enabled=false'] FROM pg_class WHERE oid=$1::regclass", table).Scan(&disabled); err != nil || !disabled {
					t.Fatalf("actual autovacuum %s=%t: %v", table, disabled, err)
				}
				n := count(t, f.tm.pool, "SELECT count(*) FROM pg_stats WHERE schemaname=current_schema() AND tablename=$1", table)
				stats[table] = n
				if state == "no_statistics" && n != 0 || state == "analyzed" && count(t, f.tm.pool, "SELECT count(*) FROM "+table) > 0 && n == 0 {
					t.Fatalf("actual %s statistics %s=%d", state, table, n)
				}
			}
			var got []page.VirtualLogChangeset
			priorSQL, _ := load.trace.copy()
			trace.start()
			err := load.read.WithinSnapshot(t.Context(), func(ctx context.Context) error {
				var err error
				got, err = page.NewVirtualLogReads(f.tm.pool, displayNames{identity.NewDirectory(f.tm.pool)}).Recent(ctx, uuid.MustParse(f.book.id), 1)
				return err
			})
			queries, statements := trace.stop()
			if len(got) == 1 {
				got[0].CreatedAt = got[0].CreatedAt.UTC()
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("real Recent full3+5 result=%+v, want%+v: %v", got, want, err)
			}
			var path virtualCostStatement
			calls := map[string]int{}
			var pid uint32
			var snapshot uint64
			for _, q := range queries {
				if pid == 0 {
					pid, snapshot = q.PID, q.Snapshot
				}
				if q.Failed || q.PID != pid || q.Snapshot != snapshot || snapshot == 0 {
					t.Fatalf("Recent left one actual successful snapshot: %+v", queries)
				}
			}
			allSQL, _ := load.trace.copy()
			begins, commits := 0, 0
			for _, q := range allSQL[len(priorSQL):] {
				if q.PID != pid || q.Snapshot != snapshot {
					continue
				}
				if strings.HasPrefix(q.SQL, "begin") {
					begins++
					if !strings.Contains(q.SQL, "repeatable read") || !strings.Contains(q.SQL, "read only") {
						t.Fatalf("actual Recent snapshot=%s", q.SQL)
					}
				}
				if q.SQL == "commit" {
					commits++
				}
				if q.SQL == "rollback" {
					t.Fatal("healthy Recent rolled back")
				}
			}
			if begins != 1 || commits != 1 {
				t.Fatalf("actual Recent begin/commit=%d/%d", begins, commits)
			}
			for _, s := range statements {
				calls[s.Name]++
				switch s.Name {
				case "ListChangesets":
					if len(s.Args) != 6 || s.Args[0] != uuid.MustParse(f.book.id) || s.Args[5] != int32(1) {
						t.Fatalf("actual Recent list arguments=%#v", s.Args)
					}
				case "FirstChanges", "FirstIngestSources":
					n := int32(3)
					if s.Name == "FirstIngestSources" {
						n = 5
					}
					if len(s.Args) != 2 || !reflect.DeepEqual(s.Args[0], []uuid.UUID{uuid.MustParse(f.cs)}) || s.Args[1] != n {
						t.Fatalf("actual %s arguments=%#v", s.Name, s.Args)
					}
				case "LinkTargetsByIDs":
					path = s
				}
			}
			if calls["ListChangesets"] != 1 || calls["FirstChanges"] != 1 || calls["FirstIngestSources"] != 1 || calls["LinkTargetsByIDs"] != 1 || calls["ListNodes"] != 0 {
				t.Fatalf("Recent bounded calls=%v", calls)
			}
			if len(path.Args) != 2 || path.Args[0] != uuid.MustParse(f.book.id) || path.Rows != 12 {
				t.Fatalf("actual path SQL/rows=%+v", path)
			}
			ids, ok := path.Args[1].([]uuid.UUID)
			actual := make([]string, len(ids))
			for i, id := range ids {
				actual[i] = id.String()
			}
			slices.Sort(actual)
			if !ok || !slices.Equal(actual, wantIDs) {
				t.Fatalf("actual unique path IDs=%q, want exact seven %q", actual, wantIDs)
			}
			virtualCostEvent(t, "service", "ingest-recent-paths", state, map[string]any{"Queries": queries, "SQL": path.SQL, "Args": path.Args, "Statistics": stats, "PoolDefaultQueryExecMode": load.read.pool.Config().ConnConfig.DefaultQueryExecMode.String(), "Scope": "actual Recent(1) in original RR; trace does not retain the leading QueryExecMode option; static Store.planned contract is DescribeExec; no forced plan_cache_mode"})
			for _, mode := range []string{"force_custom_plan", "force_generic_plan"} {
				ingestVirtualPathPlan(t, f, helper, state, mode, path, wantIDs)
			}
		})
	}
}

func ingestVirtualPathPlan(t *testing.T, f ingestVirtualFixture, helper, state, mode string, s virtualCostStatement, ids []string) {
	t.Helper()
	if mode != "force_custom_plan" && mode != "force_generic_plan" {
		t.Fatal("unreviewed path replay mode")
	}
	tx, err := f.tm.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(t.Context(), "SET LOCAL plan_cache_mode="+mode); err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{}
	for _, name := range []string{"plan_cache_mode", "enable_seqscan", "enable_indexscan", "enable_bitmapscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin", "track_counts"} {
		var value string
		if err := tx.QueryRow(t.Context(), "SHOW "+name).Scan(&value); err != nil {
			t.Fatal(err)
		}
		settings[name] = value
		if name == "plan_cache_mode" && value != mode || name != "plan_cache_mode" && value != "on" {
			t.Fatalf("planner %s=%s", name, value)
		}
	}
	prepareBefore := virtualPhysicalWork(t, tx)
	desc, err := tx.Conn().Prepare(t.Context(), "p4_ingest_paths", s.SQL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Conn().Deallocate(context.Background(), "p4_ingest_paths") }()
	if !slices.Equal(desc.ParamOIDs, []uint32{2950, 2951}) {
		t.Fatalf("actual path OIDs=%v", desc.ParamOIDs)
	}
	prepared := func() [2]int64 {
		var n [2]int64
		if err := tx.QueryRow(t.Context(), "SELECT custom_plans,generic_plans FROM pg_prepared_statements WHERE name='p4_ingest_paths'").Scan(&n[0], &n[1]); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := virtualPhysicalWork(t, tx)
	plansBefore := prepared()
	rows, err := tx.Query(t.Context(), "p4_ingest_paths", s.Args...)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[gen.LinkTargetsByIDsRow])
	if err != nil {
		t.Fatal(err)
	}
	after := virtualPhysicalWork(t, tx)
	plansAfter := prepared()
	if mode == "force_custom_plan" && (plansAfter[0]-plansBefore[0] != 1 || plansAfter[1] != plansBefore[1]) || mode == "force_generic_plan" && (plansAfter[1]-plansBefore[1] != 1 || plansAfter[0] != plansBefore[0]) {
		t.Fatalf("actual %s counters=%v->%v", mode, plansBefore, plansAfter)
	}
	want := []gen.LinkTargetsByIDsRow{}
	raw := uuid.MustParse(f.book.raw)
	for _, id := range ids {
		u := uuid.MustParse(id)
		if title, source := f.titles[id]; source {
			want = append(want, gen.LinkTargetsByIDsRow{PageID: u, ID: raw, Name: "raw", NameKey: "raw", Kind: "page", Up: 1},
				gen.LinkTargetsByIDsRow{PageID: u, ID: u, ParentID: &raw, Name: title, NameKey: strings.ToLower(title), Kind: "page"})
		} else {
			name, key := "Summary", "summary"
			if id == helper {
				name, key = "Helper", "helper"
			}
			want = append(want, gen.LinkTargetsByIDsRow{PageID: u, ID: u, Name: name, NameKey: key, Kind: "page"})
		}
	}
	if len(got) != 12 || !reflect.DeepEqual(got, want) {
		t.Fatalf("actual named path rows=%+v, want all twelve independent rows%+v", got, want)
	}
	work := virtualWorkDelta(before, after)
	virtualCostEvent(t, "execution", "ingest-recent-paths", state, map[string]any{"SQL": s.SQL, "Args": s.Args, "OIDs": desc.ParamOIDs, "Mode": mode, "Settings": settings,
		"PrepareWork": virtualWorkDelta(prepareBefore, before), "PreparedBefore": plansBefore, "PreparedAfter": plansAfter, "ExecutionWork": work,
		"ReturnedRows": len(got), "MainPages": 10011, "ForeignPages": 20003, "PerRelationRowBound": 10081,
		"Scope": "extra named PREPARE/EXECUTE; execution includes any execute-time planning, without a blanket extra-pass allowance; separate raw SQL EXPLAIN has not run"})
	var plan json.RawMessage
	if err := postgres.Planned(tx).QueryRow(t.Context(), "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+s.SQL, s.Args...).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	explainWork := virtualWorkDelta(after, virtualPhysicalWork(t, tx))
	virtualCostEvent(t, "plan", "ingest-recent-paths", state, map[string]any{"SQL": s.SQL, "Args": s.Args, "OIDs": desc.ParamOIDs, "Mode": mode, "Settings": settings,
		"PrepareWork": virtualWorkDelta(prepareBefore, before), "PreparedBefore": plansBefore, "PreparedAfter": plansAfter, "ExecutionWork": work, "ExplainWork": explainWork, "Plan": plan,
		"ReturnedRows": len(got), "MainPages": 10011, "ForeignPages": 20003, "PerRelationRowBound": 10081,
		"Scope": "extra named PREPARE/EXECUTE; execution includes any execute-time planning, without a blanket extra-pass allowance; raw SQL EXPLAIN is separate and is not the named cached plan"})
	ingestVirtualPathWork(t, "named "+mode, work)
	ingestVirtualPathWork(t, "separate EXPLAIN", explainWork)
}

func ingestVirtualPathWork(t *testing.T, phase string, work map[string]virtualRelationWork) {
	t.Helper()
	observed := false
	for relation, w := range work {
		observed = observed || w.Returned > 0 || w.Fetched > 0
		if !strings.HasPrefix(relation, "nodes") && (w.Returned != 0 || w.Fetched != 0) {
			t.Fatalf("%s path read unrelated %s: %+v", phase, relation, w)
		}
		// This deliberately permits the old notebook-index scan: main10011
		// plus seven paths' existing ten-ancestor allowance. Twelve returned
		// path steps are not the physical-work limit; foreign20003 is excluded.
		if w.Returned > 10081 || w.Fetched > 10081 {
			t.Fatalf("%s read beyond main+ancestors through %s: %+v", phase, relation, w)
		}
	}
	if !observed {
		t.Fatalf("%s observer saw no physical path work", phase)
	}
}
