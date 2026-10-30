//go:build !race

package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// These are S1-0a origin samples, not the later sources migration or the
// complete 100k lint cost suite. Timers retain normal request/commit limits.
// CPU, allocation and sampled peak are this Go process's (including the
// test server), not PostgreSQL's. WAL is cluster-wide; relation sizes are
// this test database's linking and queue tables. No production-scale claim
// or new performance threshold is inferred from these representative rows.
func TestLintIndexesCostTheirScanAndWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("representative write/rebuild measurements run alone without race")
	}
	t.Logf("origin cost input Go=%s OS=%s arch=%s CPUs=%d request=4s commit=2s links/save=5000; full migration/100k/production costs remain S1/M12 obligations", runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	for _, shape := range []string{"body", "named", "mixed-named", "mixed-empty"} {
		t.Run("origin-save/"+shape, func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			tm.createPage(t, "alice", nb, "", "Target")
			source := tm.createPage(t, "alice", nb, "", "Source")
			content := strings.Repeat("[[Target]]\n", 5000)
			nulls, empty := 5000, 0
			switch shape {
			case "named":
				content = "---\nlinks:\n" + strings.Repeat("  - '[[Target]]'\n", 5000) + "---\n"
				nulls = 0
			case "mixed-named", "mixed-empty":
				key := "root"
				if shape == "mixed-empty" {
					key = `""`
					empty = 1
				}
				content = "---\n" + key + ": '[[Target]]'\nlinks:\n" + strings.Repeat("  - '[[Target]]'\n", 2499) + "---\n" + strings.Repeat("[[Target]]\n", 2500)
				nulls = 2500
			}
			sourceIndexSettle(t, tm.pool)
			originCost(t, tm, "http-save-"+shape, func() error {
				a := tm.sender(t, contentWrite("alice", source, content, 1, ""))()
				if a.err != nil {
					return a.err
				}
				if a.status != http.StatusOK {
					return fmt.Errorf("save = %d %s", a.status, a.body)
				}
				return nil
			}, false)
			tm.originVersion(t, source, 2)
			if n := count(t, tm.pool, `SELECT count(*) FROM page_links WHERE source_id=$1`, source); n != 5000 {
				t.Fatalf("%d saved links, want 5000", n)
			}
			if n := count(t, tm.pool, `SELECT count(*) FROM page_links WHERE source_id=$1 AND property_key IS NULL`, source); n != nulls {
				t.Errorf("body rows = %d, want %d", n, nulls)
			}
			if n := count(t, tm.pool, `SELECT count(*) FROM page_links WHERE source_id=$1 AND property_key=''`, source); n != empty {
				t.Errorf("empty property rows = %d, want %d", n, empty)
			}
		})
	}
	t.Run("origin-reindex-stages", func(t *testing.T) {
		r := newOriginReindex(t, false)
		ids := r.seedEmptyPages(t, 10002)
		r.originExec(t, `INSERT INTO page_search(node_id,revision,name,tokenizer,built_at,title,body)
   SELECT id,1,name,$2,now(),''::tsvector,''::tsvector FROM nodes WHERE id=$1`, ids[0], "5/"+unicode.Version+"/"+norm.Version)
		r.originExec(t, `INSERT INTO page_search(node_id,revision,name,tokenizer,built_at,title,body)
   SELECT id,1,name,'0/0.0.0',now(),''::tsvector,''::tsvector FROM nodes WHERE id=$1`, r.target)
		trace := &originCostTrace{}
		cfg := r.pool.Config()
		cfg.ConnConfig.Tracer = trace
		pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		admin, err := reindexAdmin(pool, testConfig(t, r.url, false), slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		before := r.contents(t)
		sourceIndexSettle(t, r.pool)
		t.Logf("origin rebuild fixture: 1 notebook, 10004 live pages, 3 links in one %d-byte document and 10003 empty bodies; literal-v1 linking, 1 current/1 old/10002 missing search rows", len(originDocument))
		originCost(t, r.acmeTeam, "diagnostic-linking-commit", func() error {
			got, err := admin.Rebuild(t.Context(), uuid.MustParse(r.nb))
			if err == nil && (got.Pages != 10004 || got.Links != 3 || got.Unresolved != 0) {
				return fmt.Errorf("rebuild = %+v", got)
			}
			return err
		}, false)
		r.rebuilt(t)
		if got := r.contents(t); !reflect.DeepEqual(got, before) {
			t.Fatal("rebuild changed content/history")
		}
		// A traced call to the production stage exposes its actual 10000 + 3
		// statement transactions. The later fault follows two writes of batch 2.
		r.originExec(t, fmt.Sprintf(`CREATE FUNCTION origin_cost_fault() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
   IF NEW.node_id='%s' THEN
    IF (SELECT count(*) FROM page_search_queue WHERE notebook_id=NEW.notebook_id)<>10002 THEN RAISE EXCEPTION 'wrong cost failure position'; END IF;
    RAISE EXCEPTION 'origin cost second batch failure'; END IF; RETURN NEW; END $$;
   CREATE TRIGGER origin_cost_fault BEFORE INSERT ON page_search_queue FOR EACH ROW EXECUTE FUNCTION origin_cost_fault()`, r.source))
		search := page.NewSearchIndex(pool, 2*time.Second)
		trace.reset()
		originCost(t, r.acmeTeam, "diagnostic-queue-second-batch-failure", func() error {
			n, err := search.Requeue(t.Context(), uuid.MustParse(r.nb))
			if n != 10000 || err == nil || !strings.Contains(err.Error(), "origin cost second batch failure") {
				t.Fatalf("requeue = %d, %v", n, err)
			}
			return err
		}, true)
		trace.check(t, []int64{10000, 0})
		if got := r.queued(t, r.nb); !slices.Equal(got, ids[1:10001]) {
			t.Fatal("first batch did not survive independently")
		}
		r.rebuilt(t)
		r.originExec(t, `DROP TRIGGER origin_cost_fault ON page_search_queue; DROP FUNCTION origin_cost_fault()`)
		trace.reset()
		originCost(t, r.acmeTeam, "diagnostic-queue-recovery", func() error {
			n, err := search.Requeue(t.Context(), uuid.MustParse(r.nb))
			if err == nil && n != 10003 {
				return fmt.Errorf("queued %d, want 10003", n)
			}
			return err
		}, false)
		trace.check(t, []int64{10000, 3})
		originCost(t, r.acmeTeam, "later-search-job-build", func() error { _, err := search.Drain(t.Context()); return err }, false)
		if n := count(t, r.pool, `SELECT count(*) FROM page_search_queue`); n != 0 {
			t.Fatalf("search job left %d queue rows", n)
		}
		if n := count(t, r.pool, `SELECT count(*) FROM page_search WHERE tokenizer=$1`, "5/"+unicode.Version+"/"+norm.Version); n != 10004 {
			t.Fatalf("search job built %d current rows", n)
		}
		// Both whole-command samples start from literal old facts.
		// They are separate executions, not sums of the diagnostics above.
		legacy := func() {
			r.originExec(t, `UPDATE indexed_pages SET extractor=1 WHERE notebook_id=$1`, r.nb)
			r.originExec(t, `UPDATE page_links SET property_key=NULL WHERE notebook_id=$1 AND property_key=''`, r.nb)
			if n := count(t, r.pool, `SELECT count(*) FROM indexed_pages WHERE notebook_id=$1 AND extractor=1`, r.nb); n != 10004 {
				t.Fatalf("whole-command literal-v1 headers = %d, want 10004", n)
			}
			old := queryStrings(t, r.pool, `SELECT json_build_array(property_key,resolved_id)::text FROM page_links WHERE source_id=$1 ORDER BY range_start`, r.source)
			want := []string{fmt.Sprintf(`[null, "%s"]`, r.target), fmt.Sprintf(`["named", "%s"]`, r.target), fmt.Sprintf(`[null, "%s"]`, r.target)}
			if !slices.Equal(old, want) {
				t.Fatalf("whole-command legacy origins = %q, want %q", old, want)
			}
		}
		for _, tokenizer := range []string{"current", "missing-old"} {
			if tokenizer == "missing-old" {
				r.originExec(t, `DELETE FROM page_search WHERE node_id=ANY($1::uuid[])`, append(slices.Clone(ids[1:]), r.source))
				r.originExec(t, `UPDATE page_search SET tokenizer='0/0.0.0' WHERE node_id=$1`, r.target)
			}
			legacy()
			wantQueued := 0
			if tokenizer == "missing-old" {
				wantQueued = 10003
			}
			originCost(t, r.acmeTeam, "whole-command-"+tokenizer+"-tokenizer-v1-to-v2", func() error {
				out, errout, err := r.command(t, false)
				if err == nil && (!strings.Contains(out, fmt.Sprintf("%d nodes queued for the search", wantQueued)) || errout != "") {
					return fmt.Errorf("command output %q %q", out, errout)
				}
				return err
			}, false)
			r.rebuilt(t)
			if n := count(t, r.pool, `SELECT count(*) FROM indexed_pages WHERE notebook_id=$1 AND extractor=2`, r.nb); n != 10004 {
				t.Fatalf("whole-command current headers = %d", n)
			}
			if got := r.contents(t); !reflect.DeepEqual(got, before) {
				t.Fatal("whole command changed content/history")
			}
			var want []string
			if tokenizer == "missing-old" {
				want = append(slices.Clone(ids[1:]), r.target, r.source)
			}
			if got := r.queued(t, r.nb); !slices.Equal(got, want) {
				t.Fatalf("whole-command %s queue = %d IDs, want %d exact IDs", tokenizer, len(got), len(want))
			}
		}
	})
}

type originCostFootprint struct {
	WAL                                          string
	LinkingBytes, QueueBytes, PropertyIndexBytes int64
}

func originFootprint(t *testing.T, tm acmeTeam) originCostFootprint {
	t.Helper()
	var f originCostFootprint
	err := tm.pool.QueryRow(t.Context(), `SELECT pg_current_wal_insert_lsn()::text,
  (SELECT sum(pg_total_relation_size(x::regclass))::bigint FROM unnest(ARRAY['indexed_pages','page_links','page_properties','page_tags','page_aliases']) x),
  pg_total_relation_size('page_search_queue'),pg_relation_size('page_links_source_id_range_start_idx')`).Scan(&f.WAL, &f.LinkingBytes, &f.QueueBytes, &f.PropertyIndexBytes)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func originCPU(t *testing.T) int64 {
	t.Helper()
	var r syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &r); err != nil {
		t.Fatal(err)
	}
	return r.Utime.Nano() + r.Stime.Nano()
}

func originCost(t *testing.T, tm acmeTeam, stage string, run func() error, wantFailure bool) {
	t.Helper()
	before := originFootprint(t, tm)
	runtime.GC()
	var m0 runtime.MemStats
	runtime.ReadMemStats(&m0)
	stop, peak := make(chan struct{}), make(chan uint64, 1)
	stopSampling := sync.OnceFunc(func() { close(stop) })
	defer stopSampling()
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		high := m0.HeapAlloc
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			high = max(high, m.HeapAlloc)
			select {
			case <-stop:
				peak <- high
				return
			case <-ticker.C:
			}
		}
	}()
	cpu, start := originCPU(t), time.Now()
	err := run()
	elapsed := time.Since(start)
	cpu = originCPU(t) - cpu
	stopSampling()
	high := <-peak
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	after := originFootprint(t, tm)
	row := struct {
		Stage                                string
		ElapsedNS, ProcessCPUNS              int64
		AllocatedBytes, SampledPeakHeapBytes uint64
		WALBytes                             int64
		Before, After                        originCostFootprint
		Failed                               bool
	}{
		stage, int64(elapsed), cpu, m1.TotalAlloc - m0.TotalAlloc, high, sourceIndexWAL(t, tm.pool, before.WAL, after.WAL), before, after, err != nil}
	b, jsonErr := json.Marshal(row)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	t.Logf("origin cost %s", b)
	if (err != nil) != wantFailure {
		t.Fatalf("%s = %v, expected failure %t", stage, err, wantFailure)
	}
}

type originCostQuery struct {
	start       time.Time
	queue, lock bool
}
type originCostQueryKey struct{}
type originCostBatch struct {
	ElapsedNS, Rows int64
	Failed          bool
}
type originCostTrace struct {
	mu          sync.Mutex
	batches     []originCostBatch
	lockQueryNS int64
}

func (p *originCostTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, originCostQueryKey{}, originCostQuery{time.Now(), strings.HasPrefix(q.SQL, "-- name: QueueSearch :"), strings.Contains(q.SQL, "pg_advisory_xact_lock(") || strings.Contains(q.SQL, "FOR NO KEY UPDATE")})
}
func (p *originCostTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryEndData) {
	v, ok := ctx.Value(originCostQueryKey{}).(originCostQuery)
	if !ok {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if v.queue {
		p.batches = append(p.batches, originCostBatch{int64(time.Since(v.start)), q.CommandTag.RowsAffected(), q.Err != nil})
	}
	if v.lock {
		p.lockQueryNS += int64(time.Since(v.start))
	}
}
func (p *originCostTrace) reset() { p.mu.Lock(); defer p.mu.Unlock(); p.batches = nil }
func (p *originCostTrace) check(t *testing.T, want []int64) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	var got []int64
	for _, b := range p.batches {
		got = append(got, b.Rows)
	}
	if !slices.Equal(got, want) {
		t.Errorf("QueueSearch statement rows = %v, want %v", got, want)
	}
	b, err := json.Marshal(p.batches)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("origin queue statement batches %s; earlier linking lock acquisition query wall=%dns (SQL round trips included, not pure contention wait; no lock spans queue)", b, p.lockQueryNS)
}

// Each pair starts from independent copies of the same real 5000-link
// save, including its content and history. Only the sources index differs.
// The final state sets force_generic_plan on every application connection;
// it does not claim that Planned statements become prepared generic plans.
// These are warm application updates, not cold or production guarantees.
func TestLintSourcesIndexCostsItsNormalWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("sources index write comparisons run alone without race")
	}
	for _, shape := range []string{"sources", "nonsources", "mixed-empty"} {
		t.Run(shape, func(t *testing.T) {
			database, seed, notebook := lintCostSeedApp(t)
			tm := seed.team
			tm.createPage(t, "alice", notebook, "", "Target")
			source := tm.createPage(t, "alice", notebook, "", "Source")
			key := "sources"
			if shape == "nonsources" {
				key = "nonsources"
			}
			var leading, trailing strings.Builder
			for i := range 1000 {
				part := &leading
				if i >= 500 {
					part = &trailing
				}
				fmt.Fprintf(part, "unrelated%04d: x\n", i)
			}
			content := "---\n" + leading.String() + key + ":\n" + strings.Repeat("  - '[[Target]]'\n", 5000) + trailing.String() + "---\n"
			bodyRows, emptyRows := 0, 0
			if shape == "mixed-empty" {
				content = "---\n\"\": '[[Target]]'\n" + leading.String() + "sources:\n" + strings.Repeat("  - '[[Target]]'\n", 2499) + trailing.String() + "---\n" + strings.Repeat("[[Target]]\n", 2500)
				bodyRows, emptyRows = 2500, 1
			}
			if a := tm.sender(t, contentWrite("alice", source, content, 1, ""))(); a.err != nil || a.status != http.StatusOK {
				t.Fatalf("seed save = %+v", a)
			}
			sourceIndexSettle(t, tm.pool)
			fingerprint := lintSaveFingerprint(t, tm.pool, source)
			seed.stop()
			sourceIndexDisconnected(t, database)
			for _, stage := range []string{"no_statistics", "analyzed", "force_generic_setting"} {
				for _, enabled := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/index-%t", stage, enabled), func(t *testing.T) {
						clone := pgtest.NewDatabaseFrom(t, database)
						setup := connect(t, clone)
						if !enabled {
							lintCostExec(t, setup, "DROP INDEX page_properties_sources_idx")
						}
						if got := lintSaveFingerprint(t, setup, source); got != fingerprint {
							t.Fatal("index comparison changed initial content, history or linking rows")
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
						if got := count(t, setup, "SELECT count(*) FROM pg_index WHERE indexrelid=to_regclass('page_properties_sources_idx')"); (got == 1) != enabled {
							t.Fatal("wrong sources index toggle")
						}
						before := lintMigrationFootprintOf(t, setup)
						originCost(t, a.team, fmt.Sprintf("sources-index-%s-%s-%t", shape, stage, enabled), func() error {
							answer := a.team.sender(t, contentWrite("alice", source, content+"\nafter\n", 2, ""))()
							if answer.err != nil {
								return answer.err
							}
							if answer.status != http.StatusOK {
								return fmt.Errorf("save = %d %s", answer.status, answer.body)
							}
							return nil
						}, false)
						after := lintMigrationFootprintOf(t, setup)
						a.team.originVersion(t, source, 2)
						if n := count(t, setup, "SELECT count(*) FROM page_properties WHERE source_id=$1 AND key LIKE 'unrelated%'", source); n != 1000 {
							t.Fatalf("saved unrelated properties = %d want 1000", n)
						}
						wantSources := 1
						if shape == "nonsources" {
							wantSources = 0
						}
						if n := count(t, setup, "SELECT count(*) FROM page_properties WHERE source_id=$1 AND translate(key,'SOURCE','source')='sources'", source); n != wantSources {
							t.Fatalf("saved sources properties = %d want %d", n, wantSources)
						}
						for _, check := range []struct {
							where string
							want  int
						}{{"", 5000}, {" AND property_key IS NULL", bodyRows}, {" AND property_key=''", emptyRows}} {
							if n := count(t, setup, "SELECT count(*) FROM page_links WHERE source_id=$1"+check.where, source); n != check.want {
								t.Fatalf("saved links%s = %d want %d", check.where, n, check.want)
							}
						}
						if n := count(t, setup, "SELECT count(*) FROM page_contents WHERE node_id=$1 AND revision=3 AND content=$2", source, content+"\nafter\n"); n != 1 {
							t.Fatal("production update did not commit exact content and revision")
						}
						if n := count(t, setup, "SELECT count(*) FROM page_revisions WHERE node_id=$1", source); n != 3 {
							t.Fatalf("save history = %d want 3", n)
						}
						statisticsAfter := lintCostStatistics(t, setup, stage)
						lintCostLog(t, "write-index-boundary", struct {
							Shape, State, Fingerprint         string
							Enabled                           bool
							Before, After                     lintMigrationFootprint
							Boundary                          sourceHTTPState
							StatisticsBefore, StatisticsAfter []sourceHTTPTableState
						}{shape, stage, fingerprint, enabled, before, after, boundary, statisticsBefore, statisticsAfter})
					})
				}
			}
		})
	}
}

func lintSaveFingerprint(t *testing.T, pool *pgxpool.Pool, source string) string {
	// JSON of the actual source rows includes every field, ordered by its
	// stable primary key; the index itself is deliberately not part of it.
	t.Helper()
	var fingerprint string
	err := pool.QueryRow(t.Context(), `SELECT md5(concat(
 (SELECT jsonb_agg(to_jsonb(p) ORDER BY position) FROM page_properties p WHERE source_id=$1),
 (SELECT jsonb_agg(to_jsonb(l) ORDER BY range_start) FROM page_links l WHERE source_id=$1),
 (SELECT to_jsonb(c) FROM page_contents c WHERE node_id=$1),
 (SELECT jsonb_agg(to_jsonb(r) ORDER BY revision) FROM page_revisions r WHERE node_id=$1)))`, source).Scan(&fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}
