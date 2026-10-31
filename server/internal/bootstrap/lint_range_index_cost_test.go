//go:build !race

package bootstrap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

const lintRangeIndexName = "page_links_notebook_id_source_id_range_start_idx"

// Production52 supplies the full range index in the seed. Only the off
// copy drops it; Sources51 and every other index stay present in each copy.
// The seed's real content, history and linking rows are identical before
// each measured update. Input is warm; this is not read-side admission.
func TestLintRangeIndexCostsItsNormalWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("range index write comparisons run alone without race")
	}
	t.Logf("range index warm write input Go=%s OS=%s arch=%s CPUs=%d request=4s commit=2s links=5000 unrelated_properties=1000", runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	for _, shape := range []string{"sources", "nonsources", "mixed-empty"} {
		t.Run(shape, func(t *testing.T) {
			database, seed, notebook := lintCostSeedApp(t)
			tm := seed.team
			tm.createPage(t, "alice", notebook, "", "Target")
			source := tm.createPage(t, "alice", notebook, "", "Source")
			content, bodyRows, emptyRows := lintRangeContent(shape)
			if a := tm.sender(t, contentWrite("alice", source, content, 1, ""))(); a.err != nil || a.status != http.StatusOK {
				t.Fatalf("seed save = %+v", a)
			}
			sourceIndexSettle(t, tm.pool)
			lintSourcesDefinition(t, tm.pool)
			lintRangeDefinition(t, tm.pool, true)
			fingerprint := lintSaveFingerprint(t, tm.pool, source)
			indexes := lintRangeOtherIndexes(t, tm.pool)
			var settings json.RawMessage
			if err := tm.pool.QueryRow(t.Context(), `SELECT jsonb_build_object(
 'server_version',current_setting('server_version'),'shared_buffers',current_setting('shared_buffers'),
 'effective_cache_size',current_setting('effective_cache_size'),'maintenance_work_mem',current_setting('maintenance_work_mem'))`).Scan(&settings); err != nil {
				t.Fatal(err)
			}
			lintCostLog(t, "range-index-seed", struct {
				Shape, Fingerprint string
				OtherIndexes       []string
				DatabaseSettings   json.RawMessage
			}{shape, fingerprint, indexes, settings})
			seed.stop()
			sourceIndexDisconnected(t, database)
			for _, stage := range []string{"no_statistics", "analyzed", "force_generic_setting"} {
				var offStatistics string
				for _, enabled := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/index-%t", stage, enabled), func(t *testing.T) {
						clone := pgtest.NewDatabaseFrom(t, database)
						setup := connect(t, clone)
						if !enabled {
							lintCostExec(t, setup, "DROP INDEX "+lintRangeIndexName)
						}
						lintRangeDefinition(t, setup, enabled)
						lintSourcesDefinition(t, setup)
						if got := lintRangeOtherIndexes(t, setup); !slices.Equal(got, indexes) {
							t.Fatalf("candidate toggle changed another index: %v want %v", got, indexes)
						}
						mode := "auto"
						if stage == "force_generic_setting" {
							mode = "force_generic_plan"
						}
						a := sourceHTTPStartApp(t, clone, mode, "")
						defer a.stop()
						status, body := ask(t, a.team.contract, http.MethodPost, a.team.base+"/api/v0/auth/login", "", `{"email":"alice@example.com","password":"Tr0ub4dor&3"}`)
						var tokens authTokens
						if status != http.StatusOK || json.Unmarshal([]byte(body), &tokens) != nil || tokens.AccessToken == "" {
							t.Fatalf("clone login = %d %s", status, body)
						}
						a.team.tokens["alice"] = tokens.AccessToken
						sourceIndexSettle(t, a.team.pool)
						if stage != "no_statistics" {
							lintCostExec(t, setup, "ANALYZE")
						}
						boundary := sourceHTTPBoundary(t, a, stage)
						statisticsBefore := lintCostStatistics(t, setup, stage)
						statisticsSignature := lintRangeStatisticsSignature(t, setup)
						if !enabled {
							offStatistics = statisticsSignature
						} else if statisticsSignature != offStatistics {
							t.Fatalf("candidate on/off heap-column statistics differ: %s want %s", statisticsSignature, offStatistics)
						}
						if got := lintSaveFingerprint(t, setup, source); got != fingerprint {
							t.Fatal("candidate preparation changed initial content, history or linking rows")
						}
						before := lintRangeFootprintOf(t, setup)
						originCost(t, a.team, fmt.Sprintf("range-index-%s-%s-%t", shape, stage, enabled), func() error {
							answer := a.team.sender(t, contentWrite("alice", source, content+"\nafter\n", 2, ""))()
							if answer.err != nil {
								return answer.err
							}
							if answer.status != http.StatusOK {
								return fmt.Errorf("save = %d %s", answer.status, answer.body)
							}
							return nil
						}, false)
						after := lintRangeFootprintOf(t, setup)
						lintRangeSaved(t, a.team, source, shape, content, bodyRows, emptyRows)
						lintSourcesDefinition(t, setup)
						lintRangeDefinition(t, setup, enabled)
						if got := lintRangeOtherIndexes(t, setup); !slices.Equal(got, indexes) {
							t.Fatal("save changed a noncandidate index")
						}
						statisticsAfter := lintCostStatistics(t, setup, stage)
						if got := lintRangeStatisticsSignature(t, setup); got != statisticsSignature {
							t.Fatal("automatic analysis changed measured heap-column statistics")
						}
						lintCostLog(t, "range-index-write", struct {
							Shape, State, Fingerprint, StatisticsSignature string
							Enabled                                        bool
							Before, After                                  lintRangeFootprint
							Boundary                                       sourceHTTPState
							StatisticsBefore, StatisticsAfter              []sourceHTTPTableState
						}{shape, stage, fingerprint, statisticsSignature, enabled, before, after, boundary, statisticsBefore, statisticsAfter})
					})
				}
			}
		})
	}
}

func lintRangeContent(shape string) (string, int, int) {
	var leading, trailing strings.Builder
	for i := range 1000 {
		part := &leading
		if i >= 500 {
			part = &trailing
		}
		fmt.Fprintf(part, "unrelated%04d: x\n", i)
	}
	if shape == "mixed-empty" {
		return "---\n\"\": '[[Target]]'\n" + leading.String() + "sources:\n" + strings.Repeat("  - '[[Target]]'\n", 2499) + trailing.String() + "---\n" + strings.Repeat("[[Target]]\n", 2500), 2500, 1
	}
	return "---\n" + leading.String() + shape + ":\n" + strings.Repeat("  - '[[Target]]'\n", 5000) + trailing.String() + "---\n", 0, 0
}

func lintRangeSaved(t *testing.T, tm acmeTeam, source, shape, content string, bodyRows, emptyRows int) {
	t.Helper()
	tm.originVersion(t, source, 2)
	for _, check := range []struct {
		where string
		want  int
	}{{"", 5000}, {" AND property_key IS NULL", bodyRows}, {" AND property_key=''", emptyRows}} {
		if n := count(t, tm.pool, "SELECT count(*) FROM page_links WHERE source_id=$1"+check.where, source); n != check.want {
			t.Fatalf("saved links%s = %d want %d", check.where, n, check.want)
		}
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM page_properties WHERE source_id=$1 AND key LIKE 'unrelated%'", source); n != 1000 {
		t.Fatalf("saved unrelated properties = %d want 1000", n)
	}
	wantSources := 1
	if shape == "nonsources" {
		wantSources = 0
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM page_properties WHERE source_id=$1 AND translate(key,'SOURCE','source')='sources'", source); n != wantSources {
		t.Fatalf("saved sources properties = %d want %d", n, wantSources)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM page_contents WHERE node_id=$1 AND revision=3 AND content=$2", source, content+"\nafter\n"); n != 1 {
		t.Fatal("production update did not commit exact content and revision")
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM page_revisions WHERE node_id=$1", source); n != 3 {
		t.Fatalf("save history = %d want 3", n)
	}
}

func lintRangeOtherIndexes(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	return queryStrings(t, pool, `SELECT schemaname||'.'||tablename||'.'||indexname||' '||indexdef
 FROM pg_indexes WHERE schemaname='public' AND indexname<>$1 ORDER BY tablename,indexname`, lintRangeIndexName)
}

func lintRangeStatisticsSignature(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var signature string
	err := pool.QueryRow(t.Context(), `SELECT encode(sha256(convert_to(
 COALESCE(jsonb_agg(to_jsonb(s) ORDER BY tablename,attname,inherited),'[]'::jsonb)::text,'UTF8')),'hex')
 FROM pg_stats s WHERE schemaname='public' AND tablename=ANY($1::text[])`, lintCostTables()).Scan(&signature)
	if err != nil {
		t.Fatal(err)
	}
	return signature
}

func lintRangeDefinition(t *testing.T, pool *pgxpool.Pool, enabled bool) {
	t.Helper()
	if n := count(t, pool, "SELECT count(*) FROM pg_index WHERE indexrelid=to_regclass($1)", lintRangeIndexName); (n == 1) != enabled {
		t.Fatalf("candidate range index count = %d, enabled=%t", n, enabled)
	}
	if !enabled {
		return
	}
	var keys []string
	var attrs int
	var valid, unique, full bool
	var method string
	err := pool.QueryRow(t.Context(), `SELECT i.indnatts,i.indisvalid,i.indisunique,i.indpred IS NULL,am.amname,
 ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(attnum,ord)
 JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum ORDER BY k.ord)
 FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid JOIN pg_am am ON am.oid=c.relam
 WHERE i.indexrelid=to_regclass($1)`, lintRangeIndexName).Scan(&attrs, &valid, &unique, &full, &method, &keys)
	if err != nil {
		t.Fatal(err)
	}
	if attrs != 3 || !valid || unique || !full || method != "btree" || !slices.Equal(keys, []string{"notebook_id", "source_id", "range_start"}) {
		t.Fatalf("candidate range index attrs=%d valid=%t unique=%t full=%t method=%s keys=%v", attrs, valid, unique, full, method, keys)
	}
}

type lintRangeFootprint struct {
	WAL                                                                 string
	HeapBytes, AllIndexBytes, TotalBytes, RangeBytes, SourcesIndexBytes int64
}

func lintRangeFootprintOf(t *testing.T, pool *pgxpool.Pool) lintRangeFootprint {
	t.Helper()
	var f lintRangeFootprint
	err := pool.QueryRow(t.Context(), `SELECT pg_current_wal_insert_lsn()::text,
 pg_relation_size('page_links'),pg_indexes_size('page_links'),pg_total_relation_size('page_links'),
 COALESCE(pg_relation_size(to_regclass($1)),0),pg_relation_size('page_properties_sources_idx')`, lintRangeIndexName).
		Scan(&f.WAL, &f.HeapBytes, &f.AllIndexBytes, &f.TotalBytes, &f.RangeBytes, &f.SourcesIndexBytes)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
