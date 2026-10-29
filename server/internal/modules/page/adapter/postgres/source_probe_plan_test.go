package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const sourceHashIndex = "nodes_notebook_id_kind_source_hash_idx"

// These IDs interleave notebooks and kinds in the primary index. Only the
// source index's full namespace may bound a continuation in a converged key.
func sourcePlanID(book, kind, ordinal int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("70000000-0000-7000-8000-%04x%02x%06x", book, kind, ordinal))
}

func (f fixture) sourceProbeCorpus(t *testing.T) {
	t.Helper()
	f.exec(t, "ALTER TABLE nodes SET (autovacuum_enabled=false)")
	hash := sourceSum("converged bytes")
	for book, nb := range []uuid.UUID{f.eng, f.ops} {
		raw := f.page(t, nb, nil, "raw", 0)
		f.exec(t, "UPDATE notebooks SET mode='llm_wiki', source_root_id=$2 WHERE id=$1", nb, raw.ID)
		count := (book + 1) * 100000
		// This port fixture seeds current nodes/content only, like the existing
		// adapter fixtures; full history/file invariants belong to bootstrap.
		for start := 0; start < count; start += 10000 {
			f.exec(t, `INSERT INTO nodes (id,notebook_id,parent_id,kind,name,name_key,sort_order,source_hash,
    created_by_id,updated_by_id,created_at,updated_at)
    SELECT ('70000000-0000-7000-8000-' || lpad(to_hex($5::int),4,'0') || '00' || lpad(to_hex(k),6,'0'))::uuid,
     $1,$2,'page','Source'||k,'source'||k,k,$3,$4,$4,$8,$8
    FROM generate_series($6::int,$7::int) k`, nb, raw.ID, hash, f.alice, book+1, start+1, start+10000, now())
		}
		f.exec(t, `INSERT INTO page_contents (node_id,revision,content,content_hash,byte_size,updated_by_id,updated_at)
   SELECT id,1,'converged bytes',$2,15,$3,$4 FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL`, nb, hash, f.alice, now())
		asset := f.asset(t, nb, &raw.ID, "same-bytes.bin", -1)
		f.exec(t, "UPDATE nodes SET source_hash=$2 WHERE id=$1", asset.ID, hash)
		f.exec(t, `INSERT INTO asset_blobs (id,node_id,notebook_id,mime,byte_size,sha256,created_by_id,created_at)
		 VALUES ($1,$2,$3,'application/octet-stream',15,$4,$5,$6)`, uuid.NewV7(), asset.ID, nb, hash, f.alice, now())
		f.page(t, nb, &raw.ID, "Empty", -2)
		ordinary := f.page(t, nb, nil, "Ordinary", 1)
		trash := f.page(t, nb, &raw.ID, "Trash", -3)
		for _, id := range []uuid.UUID{ordinary.ID, trash.ID} {
			f.exec(t, "UPDATE page_contents SET content='converged bytes',content_hash=$2,byte_size=15 WHERE node_id=$1", id, hash)
		}
		f.exec(t, "UPDATE nodes SET deleted_at=$2 WHERE id=$1", trash.ID, now())
		f.exec(t, "UPDATE page_contents SET deleted_at=$2 WHERE node_id=$1", trash.ID, now())
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND kind='page' AND source_hash IS NOT NULL", f.eng); got != 100000 {
		t.Fatalf("source pages=%d want100000", got)
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND kind='page' AND source_hash IS NOT NULL", f.ops); got != 200000 {
		t.Fatalf("foreign source pages=%d want200000", got)
	}
}

func sourceProbeExecute(t *testing.T, name string, args []any) string {
	t.Helper()
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	var parts []string
	for _, arg := range args {
		var values []string
		var kind string
		switch v := arg.(type) {
		case uuid.UUID:
			parts = append(parts, quote(v.String())+"::uuid")
			continue
		case []uuid.UUID:
			kind = "uuid"
			for _, x := range v {
				values = append(values, quote(x.String()))
			}
		case []string:
			kind = "text"
			for _, x := range v {
				values = append(values, quote(x))
			}
		case [][]byte:
			kind = "bytea"
			for _, x := range v {
				values = append(values, fmt.Sprintf("decode('%x','hex')", x))
			}
		case []int32:
			kind = "integer"
			for _, x := range v {
				values = append(values, fmt.Sprint(x))
			}
		case []bool:
			kind = "boolean"
			for _, x := range v {
				values = append(values, fmt.Sprint(x))
			}
		default:
			t.Fatalf("unexpected ProbeSources argument %T", arg)
		}
		parts = append(parts, "ARRAY["+strings.Join(values, ",")+"]::"+kind+"[]")
	}
	return "EXECUTE " + pgx.Identifier{name}.Sanitize() + " (" + strings.Join(parts, ",") + ")"
}

type sourceProbePlanEvidence struct {
	Case, Mode, SQL, ExecutedSQL string
	Arguments                    []any
	Rows                         []uuid.UUID
	Samples                      []time.Duration
	ExecutionWork, ExplainWork   map[string]sourceRelationWork
	Plan                         json.RawMessage
	Settings                     map[string]string
	GenericPlans, CustomPlans    int64
}

func (f fixture) sourceProbeRange(t *testing.T, label string, p gen.ProbeSourcesParams, generic bool, want []uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	s := statementOf(t, func(q *gen.Queries) error { _, err := q.ProbeSources(ctx, p); return err })
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	e := sourceProbePlanEvidence{Case: label, Mode: "custom", SQL: s.sql, ExecutedSQL: s.sql, Arguments: s.args, Settings: map[string]string{}}
	args := s.args
	const prepared = "source_probe_range"
	if generic {
		e.Mode = "generic"
		if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode=force_generic_plan"); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, "PREPARE "+prepared+" AS "+s.sql); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = tx.Exec(context.Background(), "DEALLOCATE "+prepared) }()
		e.ExecutedSQL, args = sourceProbeExecute(t, prepared, args), nil
	} else if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode=force_custom_plan"); err != nil {
		t.Fatal(err)
	}
	for _, setting := range []string{"plan_cache_mode", "enable_seqscan", "enable_bitmapscan", "enable_indexscan", "enable_indexonlyscan", "enable_sort", "enable_incremental_sort", "enable_nestloop", "enable_material", "track_counts"} {
		var value string
		if err := tx.QueryRow(ctx, "SELECT current_setting($1)", setting).Scan(&value); err != nil {
			t.Fatal(err)
		}
		e.Settings[setting] = value
		if setting != "plan_cache_mode" && value != "on" {
			t.Fatalf("nondefault planner setting %s=%s", setting, value)
		}
	}
	for sample := 0; sample < 3; sample++ {
		before := sourceWork(t, tx)
		started := time.Now()
		rows, err := postgres.Planned(tx).Query(ctx, e.ExecutedSQL, args...)
		if err != nil {
			t.Fatal(err)
		}
		var ids []uuid.UUID
		for rows.Next() {
			var ordinal int64
			var id uuid.UUID
			if err := rows.Scan(&ordinal, &id); err != nil {
				t.Fatal(err)
			}
			if ordinal != 1 {
				t.Errorf("probe ordinal=%d want1", ordinal)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		e.Samples = append(e.Samples, time.Since(started))
		if !slices.Equal(ids, want) {
			t.Fatalf("probe %s returned %d/%v want%d/%v", label, len(ids), ids, len(want), want)
		}
		e.Rows = ids
		e.ExecutionWork = sourceWorkDelta(before, sourceWork(t, tx))
		assertSourceProbeWork(t, label, e.ExecutionWork, len(want), p.HasAfters[0])
	}
	before := sourceWork(t, tx)
	if err := postgres.Planned(tx).QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,VERBOSE,FORMAT JSON) "+e.ExecutedSQL, args...).Scan(&e.Plan); err != nil {
		t.Fatal(err)
	}
	e.ExplainWork = sourceWorkDelta(before, sourceWork(t, tx))
	assertSourceProbeWork(t, label, e.ExplainWork, len(want), p.HasAfters[0])
	if generic {
		if err := tx.QueryRow(ctx, "SELECT generic_plans,custom_plans FROM pg_prepared_statements WHERE name=$1", prepared).Scan(&e.GenericPlans, &e.CustomPlans); err != nil {
			t.Fatal(err)
		}
		if e.GenericPlans != 4 || e.CustomPlans != 0 {
			t.Errorf("prepared generic/custom=%d/%d want4/0", e.GenericPlans, e.CustomPlans)
		}
	}
	sourceEvidence(t, "source-probe-range", e)
	var plans []struct{ Plan sourceExecutedPlan }
	if err := json.Unmarshal(e.Plan, &plans); err != nil || len(plans) != 1 {
		t.Fatalf("plan decode=%v documents%d", err, len(plans))
	}
	found := false
	var walk func(sourceExecutedPlan, bool)
	walk = func(n sourceExecutedPlan, limited bool) {
		if n.NodeType == "Limit" {
			limited = true
		}
		if limited && (strings.Contains(n.NodeType, "Sort") || strings.Contains(n.NodeType, "Bitmap") || n.NodeType == "Seq Scan" || n.NodeType == "Materialize") {
			t.Errorf("unbounded work below per-key limit: %+v", n)
		}
		if n.RelationName == "nodes" && strings.Contains(n.NodeType, "Index") {
			found = true
			if n.IndexName != sourceHashIndex {
				t.Errorf("source index=%s want%s", n.IndexName, sourceHashIndex)
			}
			for _, key := range []string{"notebook_id", "kind", "source_hash", "id >=", "ROW(n.kind, n.source_hash, n.id) >=", "ROW(n.kind, n.source_hash) <="} {
				if !strings.Contains(n.IndexCond, key) {
					t.Errorf("index condition misses %s: %s", key, n.IndexCond)
				}
			}
		}
		for _, child := range n.Plans {
			walk(child, limited)
		}
	}
	walk(plans[0].Plan, false)
	if !found {
		t.Error("no actual source index access")
	}
}

func assertSourceProbeWork(t *testing.T, label string, work map[string]sourceRelationWork, rows int, after bool) {
	t.Helper()
	maxRows := int64(rows)
	if after {
		maxRows++
	}
	w := work[sourceHashIndex]
	if w.Scans != 1 || w.Returned < int64(rows) || w.Returned > maxRows {
		t.Errorf("%s physical source index work=%+v want1scan and %d..%d entries", label, w, rows, maxRows)
	}
	if w := work["nodes"]; w.Scans != 0 || w.Returned != 0 {
		t.Errorf("%s sequential heap work=%+v", label, w)
	}
	for name, w := range work {
		if name != "nodes" && name != sourceHashIndex && w.Returned != 0 {
			t.Errorf("%s unrelated index %s work=%+v", label, name, w)
		}
	}
}

// The outer sort sees only the bounded result. A continuation near the end
// of a 100k converged key must not read/sort its preceding rows or the other
// notebook's 200k same-key rows. Every timing sample is an actual read.
func TestSourceProbesKeepTheirPhysicalRanges(t *testing.T) {
	f := newFixture(t)
	f.sourceProbeCorpus(t)
	hash := sourceSum("converged bytes")
	for _, stage := range []string{"no_statistics", "analyzed", "generic"} {
		if stage == "analyzed" {
			f.exec(t, "ANALYZE nodes,page_contents")
		}
		stats := f.count(t, "SELECT count(*) FROM pg_stats WHERE tablename='nodes'")
		if (stage == "no_statistics") != (stats == 0) {
			t.Fatalf("%s actual statistics columns=%d", stage, stats)
		}
		for _, tc := range []struct {
			name        string
			after, take int
		}{{"first", 0, 1}, {"after_three", 3, 4}, {"after_sixty_thousand", 60000, 10000}, {"last", 99999, 1}, {"exhausted", 100000, 1}} {
			t.Run(stage+"/"+tc.name, func(t *testing.T) {
				cursor := uuid.Nil()
				if tc.after != 0 {
					cursor = sourcePlanID(1, 0, tc.after)
				}
				p := gen.ProbeSourcesParams{NotebookID: f.eng, Kinds: []string{"page"}, Hashes: [][]byte{hash}, AfterIds: []uuid.UUID{cursor}, HasAfters: []bool{tc.after != 0}, Takes: []int32{int32(tc.take)}}
				var want []uuid.UUID
				for n := tc.after + 1; n <= min(100000, tc.after+tc.take); n++ {
					want = append(want, sourcePlanID(1, 0, n))
				}
				f.sourceProbeRange(t, stage+"/"+tc.name, p, stage == "generic", want)
			})
		}
	}
}
