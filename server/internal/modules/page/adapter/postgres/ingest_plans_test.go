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
)

func ingestPlanID(k int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("7a210000-0000-7000-8001-%012x", k))
}

// These are additional physical legs of the original index guard. All planner
// switches remain on, and PREPARE/EXECUTE counters prove both real cache modes.
func testIngestHistoryPlans(t *testing.T) {
	for _, foreign := range []int{0, 80000} {
		t.Run(fmt.Sprintf("foreign_%d", foreign), func(t *testing.T) {
			ctx := context.Background()
			f := newFixture(t)
			for _, table := range []string{"changesets", "changeset_sources", "changeset_items", "page_revisions", "nodes"} {
				f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
			}
			bob := f.user(t, "ingest-plan@corp.com")
			hot, rest, away := f.page(t, f.eng, nil, "Hot", 0), f.page(t, f.eng, nil, "Other", 1), f.page(t, f.ops, nil, "Away", 0)
			f.exec(t, `INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at,ingest_count)
    SELECT ('7a210000-0000-7000-8001-'||lpad(to_hex(k),12,'0'))::uuid,$1,'edit','web',
     CASE WHEN k%3=0 THEN $2::uuid ELSE $3::uuid END,$4::timestamptz+k*interval '1 second',
     $4::timestamptz+k*interval '1 second',CASE WHEN k%10=0 THEN 1 ELSE 0 END FROM generate_series(1,20000) k`, f.eng, f.alice, bob, now())
			f.exec(t, `INSERT INTO changeset_items(id,changeset_id,node_id,after_name,after_sort_order,created_at,updated_at)
    SELECT uuidv7(),('7a210000-0000-7000-8001-'||lpad(to_hex(k),12,'0'))::uuid,
     CASE WHEN k%20<2 THEN $1::uuid ELSE $2::uuid END,'N',0,$3,$3 FROM generate_series(1,20000) k WHERE k%2=1`, hot.ID, rest.ID, now())
			f.exec(t, `INSERT INTO page_revisions(id,changeset_id,node_id,revision,content,content_hash,byte_size,created_at,updated_at)
    SELECT uuidv7(),('7a210000-0000-7000-8001-'||lpad(to_hex(k),12,'0'))::uuid,
     CASE WHEN k%20<2 THEN $1::uuid ELSE $2::uuid END,k,'',sha256(''),0,$3,$3 FROM generate_series(1,20000) k WHERE k%2=0`, hot.ID, rest.ID, now())
			if foreign > 0 {
				f.exec(t, `INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at,ingest_count)
     SELECT ('7a220000-0000-7000-8001-'||lpad(to_hex(k),12,'0'))::uuid,$1,'edit','web',$2,
      $3::timestamptz+k*interval '1 second',$3::timestamptz+k*interval '1 second',1 FROM generate_series(1,$4::integer) k`, f.ops, bob, now(), foreign)
				f.exec(t, `INSERT INTO changeset_items(id,changeset_id,node_id,after_name,after_sort_order,created_at,updated_at)
     SELECT uuidv7(),('7a220000-0000-7000-8001-'||lpad(to_hex(k),12,'0'))::uuid,$1,'Away',0,$2,$2
      FROM generate_series(1,$3::integer) k WHERE k%2=1`, away.ID, now(), foreign)
				f.exec(t, `INSERT INTO page_revisions(id,changeset_id,node_id,revision,content,content_hash,byte_size,created_at,updated_at)
     SELECT uuidv7(),('7a220000-0000-7000-8001-'||lpad(to_hex(k),12,'0'))::uuid,$1,k,'',sha256(''),0,$2,$2
      FROM generate_series(1,$3::integer) k WHERE k%2=0`, away.ID, now(), foreign)
			}
			f.exec(t, `INSERT INTO changeset_sources(changeset_id,seq,node_id)
    SELECT id,1,CASE WHEN notebook_id=$1 THEN $2::uuid ELSE $3::uuid END FROM changesets WHERE ingest_count=1`, f.eng, hot.ID, away.ID)
			end := now().Add(30000 * time.Second)
			queries := []struct {
				name    string
				s       statement
				indexes []string
			}{
				{"notebook", statementOf(t, func(q *gen.Queries) error {
					_, err := q.ListIngestChangesets(ctx, gen.ListIngestChangesetsParams{NotebookID: f.eng, BelowAt: end, BelowID: uuid.Max(), MaxRows: 101})
					return err
				}), []string{"changesets_notebook_id_created_at_ingest_idx"}},
				{"actor", statementOf(t, func(q *gen.Queries) error {
					_, err := q.ListIngestChangesetsBy(ctx, gen.ListIngestChangesetsByParams{NotebookID: f.eng, Actor: f.alice, BelowAt: end, BelowID: uuid.Max(), MaxRows: 101})
					return err
				}), []string{"changesets_notebook_id_created_by_id_created_at_ingest_idx"}},
				{"page", statementOf(t, func(q *gen.Queries) error {
					_, err := q.ListIngestChangesetsOfPage(ctx, gen.ListIngestChangesetsOfPageParams{NotebookID: f.eng, PageID: hot.ID, BelowAt: end, BelowID: uuid.Max(), MaxRows: 101})
					return err
				}), []string{"changeset_items_node_id_idx", "page_revisions_node_id_revision_key", "changesets_pkey"}},
				// Other owns 90% of each target history relation. Keep Hot's
				// sparse range as well as this majority-hit comparison.
				{"page_majority", statementOf(t, func(q *gen.Queries) error {
					_, err := q.ListIngestChangesetsOfPage(ctx, gen.ListIngestChangesetsOfPageParams{NotebookID: f.eng, PageID: rest.ID, BelowAt: end, BelowID: uuid.Max(), MaxRows: 101})
					return err
				}), []string{"changeset_items_node_id_idx", "page_revisions_node_id_revision_key", "changesets_pkey"}},
			}
			for _, stats := range []string{"no_statistics", "analyzed"} {
				if stats == "analyzed" {
					f.exec(t, "ANALYZE")
				}
				n := f.count(t, "SELECT count(*) FROM pg_stats WHERE schemaname='public' AND tablename IN ('changesets','changeset_sources','changeset_items','page_revisions','nodes')")
				if (stats == "no_statistics" && n != 0) || (stats == "analyzed" && n == 0) {
					t.Fatalf("%s actual stats columns%d", stats, n)
				}
				for _, mode := range []string{"custom", "generic"} {
					for _, q := range queries {
						t.Run(stats+"/"+mode+"/"+q.name, func(t *testing.T) {
							want := make([]uuid.UUID, 0, 101)
							for k := 20000; k > 0 && len(want) < 101; k-- {
								if k%10 != 0 || q.name == "actor" && k%3 != 0 || q.name == "page" && k%20 >= 2 || q.name == "page_majority" && k%20 < 2 {
									continue
								}
								want = append(want, ingestPlanID(k))
							}
							ingestHistoryPlan(t, f, q.name, stats, mode, q.s, q.indexes, want, foreign)
						})
					}
				}
			}
		})
	}
}

// Unlike a text comparison of SQL, these guards measure executed relation
// ranges and exact nonempty answers. The foreign notebook may grow every
// relation; it may not add target reads.
func ingestHistoryPlan(t *testing.T, f fixture, name, stats, mode string, s statement, indexes []string, want []uuid.UUID, foreign int) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode=force_"+mode+"_plan"); err != nil {
		t.Fatal(err)
	}
	const prepared = "ingest_history_physical"
	if _, err := tx.Exec(ctx, "PREPARE "+prepared+" AS "+s.sql); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = tx.Exec(ctx, "DEALLOCATE "+prepared) }()
	settings := map[string]string{}
	for _, key := range []string{"plan_cache_mode", "enable_seqscan", "enable_bitmapscan", "enable_indexscan", "enable_indexonlyscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin", "enable_sort", "jit"} {
		var value string
		if err := tx.QueryRow(ctx, "SELECT current_setting($1)", key).Scan(&value); err != nil {
			t.Fatal(err)
		}
		settings[key] = value
		if key != "plan_cache_mode" && key != "jit" && value != "on" {
			t.Fatalf("planner switch %s=%s", key, value)
		}
		if key == "jit" && value != "off" {
			t.Fatalf("production pool jit=%s", value)
		}
	}
	execute := ingestPlanExecute(t, prepared, s.args)
	rows, err := tx.Query(ctx, execute)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]uuid.UUID, 0, 101)
	for rows.Next() {
		var id, nb uuid.UUID
		var kind string
		var count int32
		if err := rows.Scan(&id, &nb, &kind, nil, nil, nil, nil, nil, nil, nil, &count, nil, nil, nil); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if nb != f.eng || kind != "edit" || count != 1 {
			t.Errorf("actual row belongs to %s kind%s count%d", nb, kind, count)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, want) {
		t.Errorf("nonempty ordered IDs=%v, want%v", ids, want)
	}
	var raw json.RawMessage
	if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,VERBOSE,FORMAT JSON) "+execute).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var generic, custom int64
	if err := tx.QueryRow(ctx, "SELECT generic_plans,custom_plans FROM pg_prepared_statements WHERE name=$1", prepared).Scan(&generic, &custom); err != nil {
		t.Fatal(err)
	}
	if mode == "generic" && (generic != 2 || custom != 0) || mode == "custom" && (custom != 2 || generic != 0) {
		t.Errorf("actual cache counters generic/custom=%d/%d", generic, custom)
	}
	var plan []struct{ Plan sourceExecutedPlan }
	if err := json.Unmarshal(raw, &plan); err != nil || len(plan) != 1 {
		t.Fatalf("decode actual plan: %v", err)
	}
	used := map[string]bool{}
	work := map[string]float64{}
	var visit func(sourceExecutedPlan)
	visit = func(p sourceExecutedPlan) {
		if p.ActualLoops > 0 {
			if p.IndexName != "" {
				used[p.IndexName] = true
			}
			if p.RelationName != "" {
				work[p.RelationName] += (p.ActualRows + p.RowsRemovedFilter) * p.ActualLoops
				if p.NodeType == "Seq Scan" {
					t.Errorf("history scans whole relation %s", p.RelationName)
				}
			}
			if !strings.HasPrefix(name, "page") && p.NodeType == "Sort" {
				t.Errorf("%s sorted instead of reading its ingest index", name)
			}
		}
		for _, child := range p.Plans {
			visit(child)
		}
	}
	visit(plan[0].Plan)
	for _, index := range indexes {
		if !used[index] {
			t.Errorf("executed %s did not use %s", name, index)
		}
	}
	if strings.HasPrefix(name, "page") {
		rangeRows := 1000
		if name == "page_majority" {
			rangeRows = 9000
		}
		if work["changeset_items"] > float64(rangeRows) || work["page_revisions"] > float64(rangeRows) || work["changesets"] > float64(2*rangeRows+101) {
			t.Errorf("changed-page ranges grew outside %d items+%d revisions+%d parents: %v", rangeRows, rangeRows, 2*rangeRows, work)
		}
	} else if work["changesets"] > 202 {
		t.Errorf("ingest indexed range exceeded 101 rows and at most101 revert probes: %v", work)
	}
	evidence := map[string]any{"name": name, "statistics": stats, "mode": mode, "target_changesets": 20000, "foreign_changesets": foreign,
		"sql": s.sql, "arguments": s.args, "execute": execute, "settings": settings, "generic_plans": generic, "custom_plans": custom,
		"result_ids": ids, "relation_rows_and_filtered": work, "plan": raw,
		"pool_query_mode": f.pool.Config().ConnConfig.DefaultQueryExecMode.String(), "pool_statement_cache": f.pool.Config().ConnConfig.StatementCacheCapacity}
	data, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ingest physical evidence: %s", data)
}

// EXECUTE accepts SQL literals, not wire parameters. Every value is a typed
// internal fixture value; the query itself is caught from the generated method.
func ingestPlanExecute(t *testing.T, name string, args []any) string {
	t.Helper()
	literals := make([]string, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case uuid.UUID:
			literals[i] = "'" + v.String() + "'::uuid"
		case time.Time:
			literals[i] = "'" + v.Format(time.RFC3339Nano) + "'::timestamptz"
		case *uuid.UUID:
			if v == nil {
				literals[i] = "NULL"
			} else {
				literals[i] = "'" + v.String() + "'::uuid"
			}
		case *string:
			if v == nil {
				literals[i] = "NULL"
			} else {
				literals[i] = "'" + strings.ReplaceAll(*v, "'", "''") + "'::text"
			}
		case bool:
			literals[i] = fmt.Sprintf("%t", v)
		case int32:
			literals[i] = fmt.Sprintf("%d::integer", v)
		default:
			t.Fatalf("unexpected ingest plan parameter %T", arg)
		}
	}
	return "EXECUTE " + pgx.Identifier{name}.Sanitize() + " (" + strings.Join(literals, ",") + ")"
}

func testIngestPreviewPlans(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	for _, table := range []string{"changesets", "changeset_sources", "nodes"} {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	nodes := f.nodes(t, f.eng, "Preview", 50000)
	ids := make([]uuid.UUID, 100)
	for i := range ids {
		ids[i] = ingestPlanID(i + 1)
	}
	f.exec(t, `INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at,ingest_count)
  SELECT ('7a210000-0000-7000-8001-'||lpad(to_hex(k),12,'0'))::uuid,$1,'edit','web',$2,$3,$3,
   CASE WHEN k<=100 THEN 25 ELSE 0 END FROM generate_series(1,20000) k`, f.eng, f.alice, now())
	missing := uuid.NewV7()
	f.exec(t, `INSERT INTO changeset_sources(changeset_id,seq,node_id)
  SELECT c.id,k,CASE WHEN k=1 THEN $3::uuid ELSE ($2::uuid[])[k] END
  FROM changesets c CROSS JOIN LATERAL generate_series(1,c.ingest_count) k WHERE c.id=ANY($1::uuid[])`, ids, nodes[:25], missing)
	for _, stats := range []string{"no_statistics", "analyzed"} {
		if stats == "analyzed" {
			f.exec(t, "ANALYZE")
		}
		columns := f.count(t, "SELECT count(*) FROM pg_stats WHERE schemaname='public' AND tablename IN ('changesets','changeset_sources','nodes')")
		if stats == "no_statistics" && columns != 0 || stats == "analyzed" && columns == 0 {
			t.Fatalf("%s statistics columns%d", stats, columns)
		}
		for _, mode := range []string{"custom", "generic"} {
			for _, n := range []int{3, 5, 20} {
				t.Run(fmt.Sprintf("%s/%s/n%d", stats, mode, n), func(t *testing.T) {
					s := statementOf(t, func(q *gen.Queries) error {
						_, err := q.FirstIngestSources(ctx, gen.FirstIngestSourcesParams{ChangesetIds: ids, PreviewCount: int32(n)})
						return err
					})
					tx, err := f.pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = tx.Rollback(ctx) }()
					if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode=force_"+mode+"_plan"); err != nil {
						t.Fatal(err)
					}
					const name = "ingest_preview_physical"
					if _, err := tx.Exec(ctx, "PREPARE "+name+" AS "+s.sql); err != nil {
						t.Fatal(err)
					}
					defer func() { _, _ = tx.Exec(ctx, "DEALLOCATE "+name) }()
					settings := map[string]string{}
					for _, key := range []string{"plan_cache_mode", "enable_seqscan", "enable_indexscan", "enable_indexonlyscan", "enable_bitmapscan", "enable_hashjoin", "enable_nestloop", "enable_mergejoin", "enable_sort", "jit"} {
						var value string
						if err := tx.QueryRow(ctx, "SELECT current_setting($1)", key).Scan(&value); err != nil {
							t.Fatal(err)
						}
						settings[key] = value
						if key != "plan_cache_mode" && key != "jit" && value != "on" {
							t.Fatalf("planner %s=%s", key, value)
						}
						if key == "jit" && value != "off" {
							t.Fatalf("production pool jit=%s", value)
						}
					}
					execute := sourceExecute(t, name, s.args)
					rows, err := tx.Query(ctx, execute)
					if err != nil {
						t.Fatal(err)
					}
					got := map[uuid.UUID]int{}
					for rows.Next() {
						var cs uuid.UUID
						var seq int32
						var node *uuid.UUID
						if err := rows.Scan(&cs, &seq, &node); err != nil {
							rows.Close()
							t.Fatal(err)
						}
						got[cs]++
						if int(seq) != got[cs] || seq < 1 || int(seq) > n || !slices.Contains(ids, cs) {
							t.Errorf("unexpected ordered position %s/%d", cs, seq)
						}
						if seq == 1 && node != nil || seq > 1 && (node == nil || *node != nodes[seq-1]) {
							t.Errorf("nullable source %s/%d=%v", cs, seq, node)
						}
					}
					rows.Close()
					if err := rows.Err(); err != nil {
						t.Fatal(err)
					}
					if len(got) != 100 {
						t.Errorf("preview has%d changesets, want100", len(got))
					}
					for _, id := range ids {
						if got[id] != n {
							t.Errorf("%s positions%d, want%d", id, got[id], n)
						}
					}
					var raw json.RawMessage
					if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,VERBOSE,FORMAT JSON) "+execute).Scan(&raw); err != nil {
						t.Fatal(err)
					}
					var generic, custom int64
					if err := tx.QueryRow(ctx, "SELECT generic_plans,custom_plans FROM pg_prepared_statements WHERE name=$1", name).Scan(&generic, &custom); err != nil {
						t.Fatal(err)
					}
					if mode == "generic" && (generic != 2 || custom != 0) || mode == "custom" && (custom != 2 || generic != 0) {
						t.Errorf("actual prepared cache=%d/%d", generic, custom)
					}
					var plans []struct{ Plan sourceExecutedPlan }
					if err := json.Unmarshal(raw, &plans); err != nil || len(plans) != 1 {
						t.Fatalf("decode preview plan=%v", err)
					}
					work := map[string]float64{}
					used := map[string]bool{}
					var visit func(sourceExecutedPlan)
					visit = func(p sourceExecutedPlan) {
						if p.ActualLoops > 0 && p.RelationName != "" {
							work[p.RelationName] += (p.ActualRows + p.RowsRemovedFilter) * p.ActualLoops
							if p.IndexName != "" {
								used[p.IndexName] = true
							}
							if p.NodeType == "Seq Scan" {
								t.Errorf("preview scans whole relation %s", p.RelationName)
							}
						}
						for _, child := range p.Plans {
							visit(child)
						}
					}
					visit(plans[0].Plan)
					for _, index := range []string{"changeset_sources_pkey", "changesets_pkey", "nodes_pkey"} {
						if !used[index] {
							t.Errorf("preview misses index%s", index)
						}
					}
					if work["changesets"] > 100 || work["nodes"] > float64(100*n) || work["changeset_sources"] > 2500 {
						t.Errorf("preview touched unrelated parents/nodes or repeated the persistent source relation: %v", work)
					}
					data, err := json.Marshal(map[string]any{"statistics": stats, "mode": mode, "n": n, "sql": s.sql, "arguments": s.args, "execute": execute,
						"settings": settings, "generic_plans": generic, "custom_plans": custom, "returned_changesets": len(got), "rows_per_changeset": n,
						"relation_rows_and_filtered": work, "plan": raw})
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("ingest preview physical evidence: %s", data)
				})
			}
		}
	}
}
