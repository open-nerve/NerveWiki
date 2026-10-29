package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// This regression measures executed join work, not time or a preferred plan
// shape. With no column statistics, joining two materialized preimage/change
// CTEs estimated at one row compared all 10,000 by 10,000 actual rows.
// TestSourceWritesReturnOnlyActualOldKeys independently checks the returned
// old values, NULLs, kinds, no-op result and transaction rollback.
func TestSourceHashWritesAvoidQuadraticPreimageJoin(t *testing.T) {
	const corpus, changed = 100000, 10000
	ctx := t.Context()
	f := newFixture(t)
	for _, table := range []string{"nodes", "page_contents"} {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	raw := f.page(t, f.eng, nil, "raw", 0)
	f.exec(t, "UPDATE notebooks SET mode='llm_wiki',source_root_id=$2 WHERE id=$1", f.eng, raw.ID)
	oldHash, newHash := sourceSum("before source set"), sourceSum("after source set")
	// Current rows/content are legal adapter input. History and attachment
	// files are immaterial to this one derived-column statement.
	for start := 0; start < corpus; start += changed {
		f.exec(t, `INSERT INTO nodes (id,notebook_id,parent_id,kind,name,name_key,sort_order,source_hash,
 created_by_id,updated_by_id,created_at,updated_at)
 SELECT ('70000000-0000-7000-8000-000300' || lpad(to_hex(k),6,'0'))::uuid,
 $1,$2,'page','Source'||k,'source'||k,k,$3,$4,$4,$7,$7
 FROM generate_series($5::int,$6::int) k`, f.eng, raw.ID, oldHash, f.alice, start+1, start+changed, now())
	}
	f.exec(t, `INSERT INTO page_contents (node_id,revision,content,content_hash,byte_size,updated_by_id,updated_at)
 SELECT id,1,'before source set',$2,17,$3,$4 FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL`,
		f.eng, oldHash, f.alice, now())
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash=$2", f.eng, oldHash); got != corpus {
		t.Fatalf("source set corpus=%d want%d", got, corpus)
	}
	if got := f.count(t, "SELECT count(*) FROM pg_stats WHERE schemaname=current_schema() AND tablename IN ('nodes','page_contents')"); got != 0 {
		t.Fatalf("no-statistics source set fixture has %d column statistics", got)
	}
	p := gen.SetSourceHashesParams{NotebookID: f.eng}
	for n := 1; n <= changed; n++ {
		p.NodeIds = append(p.NodeIds, sourcePlanID(3, 0, n))
		p.Hashes = append(p.Hashes, newHash)
	}
	s := statementOf(t, func(q *gen.Queries) error { _, err := q.SetSourceHashes(ctx, p); return err })
	parameters, err := json.Marshal(s.args)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	settings := map[string]string{}
	for _, setting := range []string{"plan_cache_mode", "enable_seqscan", "enable_bitmapscan", "enable_indexscan", "enable_indexonlyscan", "enable_sort", "enable_incremental_sort", "enable_nestloop", "enable_material"} {
		var value string
		if err := tx.QueryRow(ctx, "SELECT current_setting($1)", setting).Scan(&value); err != nil {
			t.Fatal(err)
		}
		settings[setting] = value
		want := "on"
		if setting == "plan_cache_mode" {
			want = "auto"
		}
		if value != want {
			t.Fatalf("source set nondefault planner setting %s=%s want%s", setting, value, want)
		}
	}
	var plan json.RawMessage
	if err := postgres.Planned(tx).QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,VERBOSE,FORMAT JSON) "+s.sql, s.args...).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	var plans []struct{ Plan sourceSetExecutedPlan }
	if err := json.Unmarshal(plan, &plans); err != nil || len(plans) != 1 {
		t.Fatalf("source set plan decode=%v documents=%d", err, len(plans))
	}
	var joinRejected, cteRows float64
	var walk func(sourceSetExecutedPlan)
	walk = func(n sourceSetExecutedPlan) {
		joinRejected += n.JoinRowsRemoved * n.ActualLoops
		if n.NodeType == "CTE Scan" {
			cteRows += n.ActualRows * n.ActualLoops
		}
		for _, child := range n.Plans {
			walk(child)
		}
	}
	walk(plans[0].Plan)
	sourceEvidence(t, "source-set-plan", struct {
		SQL, ParametersSHA256 string
		NotebookID            uuid.UUID
		IDRecipe, HashHex     string
		Corpus, Changed       int
		Settings              map[string]string
		JoinRejected, CTERows float64
		Plan                  json.RawMessage
	}{s.sql, fmt.Sprintf("%x", sha256.Sum256(parameters)), f.eng,
		"sourcePlanID(3, 0, ordinal), ordinal=1..10000, same order in both zipped arrays", fmt.Sprintf("%x", newHash),
		corpus, changed, settings, joinRejected, cteRows, plan})
	if plans[0].Plan.ActualRows != changed {
		t.Errorf("source set actual returned rows=%.0f want%d", plans[0].Plan.ActualRows, changed)
	}
	// Allow corpus-linear joins/scans, but reject the observed quadratic
	// crossmatch or equivalent repeated materialized CTE consumption.
	if joinRejected > corpus || cteRows > corpus {
		t.Errorf("source set repeated preimage work: join-rejected=%.0f CTE-emitted=%.0f, each must be <=%d for %d changes", joinRejected, cteRows, corpus, changed)
	}
	var written int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash=$2", f.eng, newHash).Scan(&written); err != nil {
		t.Fatal(err)
	}
	if written != changed {
		t.Errorf("source set executed update changed=%d want%d", written, changed)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash=$2", f.eng, oldHash); got != corpus {
		t.Errorf("source set EXPLAIN rollback preserved=%d want%d", got, corpus)
	}
}

type sourceSetExecutedPlan struct {
	NodeType        string                  `json:"Node Type"`
	ActualRows      float64                 `json:"Actual Rows"`
	ActualLoops     float64                 `json:"Actual Loops"`
	JoinRowsRemoved float64                 `json:"Rows Removed by Join Filter"`
	Plans           []sourceSetExecutedPlan `json:"Plans"`
}
