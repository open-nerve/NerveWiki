//go:build !race

package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// These are the original real request budgets, not a new empirical guard.
// Three samples per shape retain allocation/heap/SQL data for two-run calibration.
// Fixture preparation, response oracles and later EXPLAINs are outside timing.
const ingestCostOrdinaryBudget = 4 * time.Second
const ingestCostBatchBudget = 15 * time.Second

type ingestCostSQL struct {
	Calls, Rows             int64
	WallNS                  int64
	FirstStartNS, LastEndNS int64
	Errors                  int64
}
type ingestCostTrace struct {
	mu         sync.Mutex
	active     bool
	queries    map[string]ingestCostSQL
	statements map[string]virtualCostStatement
	indexAt    time.Time
	started    time.Time
	indexNS    int64
}
type ingestCostTraceKey struct{}
type ingestCostStart struct {
	name string
	at   time.Time
}

func (c *ingestCostTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.active || !strings.HasPrefix(q.SQL, "-- name: ") {
		return ctx
	}
	name := strings.Fields(q.SQL)[2]
	if name == "LockNotebook" && strings.Contains(q.SQL, "pg_advisory_xact_lock") {
		name = "LinkIndexLock"
	}
	if strings.Contains(name, "Ingest") || strings.HasPrefix(name, "LintBodyReferences") || name == "InspectPageLinksForPreparation" || name == "LinksReachedExcept" {
		old, seen := c.statements[name]
		// Capture the actual largest head batch, including the coverage 512
		// parameters. Keep the original first statement for every other query.
		headCount := func(args []any) int {
			for _, arg := range args {
				if ids, ok := arg.([]uuid.UUID); ok {
					return len(ids)
				}
			}
			return 0
		}
		if !seen || name == "IngestIndexedHeads" && headCount(q.Args) > headCount(old.Args) {
			s := virtualCostStatement{Name: name, SQL: q.SQL}
			for _, a := range q.Args {
				if _, mode := a.(pgx.QueryExecMode); mode {
					continue
				}
				switch v := a.(type) {
				case []uuid.UUID:
					a = slices.Clone(v)
				case []int32:
					a = slices.Clone(v)
				case []bool:
					a = slices.Clone(v)
				case []string:
					a = slices.Clone(v)
				}
				s.Args = append(s.Args, a)
			}
			c.statements[name] = s
		}
	}
	return context.WithValue(ctx, ingestCostTraceKey{}, ingestCostStart{name, time.Now()})
}
func (c *ingestCostTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryEndData) {
	start, ok := ctx.Value(ingestCostTraceKey{}).(ingestCostStart)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	old := c.queries[start.name]
	if old.Calls == 0 {
		old.FirstStartNS = int64(start.at.Sub(c.started))
	}
	old.LastEndNS = int64(time.Since(c.started))
	if q.Err != nil {
		old.Errors++
	}
	old.Calls++
	old.Rows += q.CommandTag.RowsAffected()
	old.WallNS += int64(time.Since(start.at))
	c.queries[start.name] = old
	if start.name == "LinkIndexLock" && q.Err == nil {
		c.indexAt = time.Now()
	}
	if start.name == "AddIngestMarks" && !c.indexAt.IsZero() {
		c.indexNS = int64(time.Since(c.indexAt))
	}
}
func (c *ingestCostTrace) start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = true
	c.started = time.Now()
	c.queries = map[string]ingestCostSQL{}
	c.statements = map[string]virtualCostStatement{}
	c.indexAt = time.Time{}
	c.indexNS = 0
}
func (c *ingestCostTrace) stop() (map[string]ingestCostSQL, map[string]virtualCostStatement, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = false
	return maps.Clone(c.queries), maps.Clone(c.statements), c.indexNS
}
func ingestCostEvent(t *testing.T, name string, metric virtualCostMetric, queries map[string]ingestCostSQL, indexNS int64, extra any) {
	t.Helper()
	b, err := json.Marshal(map[string]any{"Schema": "nervewiki.p4-s2.ingest-cost.v1", "Case": name, "Metric": metric, "SQL": queries, "IndexThroughAddNS": indexNS, "Extra": extra, "BodySQLByRole": ingestCostBodySQLRoles(queries),
		"Go": runtime.Version(), "CPUs": runtime.NumCPU()})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("INGESTCOST %s", b)
}

// Keep prepared source reads, the private fallback old-F, and original
// old-D/candidate Body cohorts separate. These are query-end traces, not a
// substitute for native Scan/EOF/Err/Close or supplied-input observers.
func ingestCostBodySQLRoles(queries map[string]ingestCostSQL) map[string]map[string]ingestCostSQL {
	return map[string]map[string]ingestCostSQL{
		"prepared_source_old_f": {
			"InspectPageLinksForPreparation": queries["InspectPageLinksForPreparation"],
		},
		"prepared_residual": {
			"LinksReachedExcept": queries["LinksReachedExcept"],
		},
		"private_old_f": {
			"IngestOldBodyReferencesFirst": queries["IngestOldBodyReferencesFirst"],
			"IngestOldBodyReferencesAfter": queries["IngestOldBodyReferencesAfter"],
		},
		"legacy_old_d_and_candidates": {
			"LintBodyReferencesFirst": queries["LintBodyReferencesFirst"],
			"LintBodyReferencesAfter": queries["LintBodyReferencesAfter"],
		},
	}
}

func ingestCostSettle(t *testing.T, tm acmeTeam, query string, args ...any) {
	t.Helper()
	if _, err := tm.pool.Exec(t.Context(), query, args...); err != nil {
		t.Fatal(err)
	}
}
func ingestCostMark(t *testing.T, tm acmeTeam, cs string, want ...string) {
	t.Helper()
	slices.Sort(want)
	got := queryStrings(t, tm.pool, "SELECT node_id::text FROM changeset_sources WHERE changeset_id=$1 ORDER BY node_id", cs)
	if !slices.Equal(got, want) || count(t, tm.pool, "SELECT ingest_count FROM changesets WHERE id=$1", cs) != len(want) {
		t.Fatalf("producer marks %v, want %v at %s", got, want, cs)
	}
}
func ingestCostID(t *testing.T, tm acmeTeam, nb, name string) string {
	t.Helper()
	rows := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id=$1 AND name=$2 AND deleted_at IS NULL", nb, name)
	if len(rows) != 1 {
		t.Fatalf("fixture name %q has %d rows", name, len(rows))
	}
	return rows[0]
}
func ingestCostFill(t *testing.T, tm acmeTeam, nb string, total int) {
	t.Helper()
	missing := total - count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1", nb)
	pages := make([]sqlPage, missing)
	for i := range pages {
		pages[i] = sqlPage{"", fmt.Sprintf("Filler %06d", i), ""}
	}
	seedPagesSQL(t, tm.pool, nb, pages)
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1", nb); n != total {
		t.Fatalf("raw nodes %d, want %d", n, total)
	}
}

func TestIngestOfAHundredThousandPagesTakesItsTime(t *testing.T) {
	trace := &ingestCostTrace{}
	tm := newAcmeTeamOn(t, "", "", func(c *config.Config) {
		if c.Server.RequestTimeout != ingestCostOrdinaryBudget || c.Database.CommitTimeout != 2*time.Second {
			t.Fatal("ordinary budgets changed")
		}
	}, &wiring{queryTracer: trace}, slog.New(slog.DiscardHandler))
	nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Ingest 100k","mode":"llm_wiki"}`)
	var sources, outputs []string
	for i := range 3 {
		sources = append(sources, tm.createPageWith(t, "alice", nb.ID, *nb.SourceRootID, fmt.Sprintf("T%d", i), fmt.Sprintf("Independent evidence %d.", i)))
		outputs = append(outputs, tm.createPage(t, "alice", nb.ID, "", fmt.Sprintf("P%d", i)))
	}
	ingestCostFill(t, tm, nb.ID, 100000)
	// A real changed save without a candidate must not scan this notebook.
	trace.start()
	status, body := ask(t, tm.contract, http.MethodPut, tm.base+"/api/v0/pages/"+outputs[0]+"/content", tm.tokens["alice"], `{"content":"plain changed","base_revision":1}`)
	q, _, _ := trace.stop()
	if status != http.StatusOK {
		t.Fatalf("noncandidate save = %d %s", status, body)
	}
	for _, name := range []string{"IngestHeadsFirst", "IngestHeadsAfter", "IngestNovelSources", "LintSourceNodes"} {
		if q[name].Calls != 0 {
			t.Fatalf("noncandidate save scanned %s: %+v", name, q[name])
		}
	}
	for sample := range 3 {
		content := fmt.Sprintf("[[raw/T%d]]", sample)
		base := 1
		if sample == 0 {
			base = 2
		}
		payload, _ := json.Marshal(map[string]any{"content": content, "base_revision": base})
		trace.start()
		metric, err := virtualCostMeasure(t, func() error {
			status, body = ask(t, tm.contract, http.MethodPut, tm.base+"/api/v0/pages/"+outputs[sample]+"/content", tm.tokens["alice"], string(payload))
			if status != http.StatusOK {
				return fmt.Errorf("PUT = %d %s", status, body)
			}
			return nil
		})
		queries, statements, indexNS := trace.stop()
		ingestCostEvent(t, fmt.Sprintf("healthy_hundred_k/%d", sample), metric, queries, indexNS, map[string]any{"raw": 100000, "request_budget_ns": int64(ingestCostOrdinaryBudget)})
		if err != nil {
			t.Fatal(err)
		}
		if time.Duration(metric.WallNS) >= ingestCostOrdinaryBudget {
			t.Fatal("healthy 100k exceeded original request budget")
		}
		if queries["IngestHeadsFirst"].Rows+queries["IngestHeadsAfter"].Rows != 100000 || queries["IngestNovelSources"].Calls != 1 {
			t.Fatalf("incomplete raw/evidence work: %v", queries)
		}
		cs := queryStrings(t, tm.pool, "SELECT changeset_id::text FROM page_revisions WHERE node_id=$1 AND revision=$2", outputs[sample], base+1)
		if len(cs) != 1 {
			t.Fatalf("exact revision changesets %v", cs)
		}
		ingestCostMark(t, tm, cs[0], sources[sample])
		ingestCostPlans(t, tm, statements, sample == 0)
	}
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
}

// The letters occupy three positions around one dot: 26^3 distinct legal
// four-byte asset names. The first 10000, in lexical order, are all linkable.
func ingestCostAssetNames(n int) []string {
	out := make([]string, 0, n)
	for a := 'a'; a <= 'z' && len(out) < n; a++ {
		for b := 'a'; b <= 'z' && len(out) < n; b++ {
			for c := 'a'; c <= 'z' && len(out) < n; c++ {
				out = append(out, string([]rune{a, b, '.', c}))
			}
		}
	}
	return out
}
func TestIngestAtTheBatchBoundaryTakesItsTime(t *testing.T) {
	for _, shape := range []struct {
		name             string
		assets, ops, raw int
	}{
		{"maximum_old_f_hundred_k_100_ops", 10000, 100, 100000},
		{"old_f_120000", 300, 2, 0},
	} {
		t.Run(shape.name, func(t *testing.T) {
			trace := &ingestCostTrace{}
			logs := &syncBuffer{}
			tm := newAcmeTeamOn(t, "", "", func(c *config.Config) {
				c.Server.WriteTimeout, c.Server.RequestTimeout = 60*time.Second, ingestCostBatchBudget
				if c.Database.CommitTimeout != 2*time.Second {
					t.Fatal("changed commit budget")
				}
			}, &wiring{queryTracer: trace}, slog.New(slog.NewJSONHandler(logs, nil)))
			nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Ingest retention","mode":"llm_wiki"}`)
			names := ingestCostAssetNames(shape.assets)
			ingestCostSettle(t, tm, `INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,source_hash,created_by_id,updated_by_id,created_at,updated_at)
                SELECT gen_random_uuid(),b.id,$2,'asset',n,n,ord,sha256(convert_to(n,'UTF8')),b.created_by_id,b.created_by_id,now(),now()
                FROM notebooks b,unnest($3::text[]) WITH ORDINALITY a(n,ord) WHERE b.id=$1`, nb.ID, *nb.SourceRootID, names)
			ingestCostSettle(t, tm, `INSERT INTO asset_blobs(id,node_id,notebook_id,mime,byte_size,sha256,created_by_id,created_at)
                SELECT gen_random_uuid(),id,notebook_id,'application/octet-stream',4,source_hash,created_by_id,created_at FROM nodes
                WHERE notebook_id=$1 AND kind='asset'`, nb.ID)
			ingestCostSettle(t, tm, `INSERT INTO page_search_queue(node_id,notebook_id,bulk)
                SELECT id,notebook_id,false FROM nodes WHERE notebook_id=$1 AND kind='asset'`, nb.ID)
			var links strings.Builder
			for _, name := range names {
				links.WriteString("[](" + name + ")")
			}
			pages := make([]sqlPage, 0, 800)
			for i := range 400 {
				pages = append(pages, sqlPage{*nb.SourceRootID, fmt.Sprintf("S%03d", i), fmt.Sprintf("S%03d\n", i) + links.String()})
			}
			for i := range 400 {
				pages = append(pages, sqlPage{"", fmt.Sprintf("Q%03d", i), fmt.Sprintf("[[raw/S%03d]]", i)})
			}
			seedPagesSQL(t, tm.pool, nb.ID, pages)
			ingestCostSettle(t, tm, `UPDATE nodes n SET source_hash=c.content_hash FROM page_contents c WHERE c.node_id=n.id AND n.notebook_id=$1 AND n.parent_id=$2 AND c.byte_size>0`, nb.ID, *nb.SourceRootID)
			tm.reindex(t, nb.ID) // real parser/extractor/resolver, outside the request timer
			if n := count(t, tm.pool, `SELECT count(*) FROM page_links l JOIN nodes n ON n.id=l.source_id JOIN indexed_pages i ON i.node_id=n.id
                JOIN page_contents c ON c.node_id=n.id WHERE n.notebook_id=$1 AND n.parent_id=$2 AND n.name LIKE 'S%'
                AND i.extractor=2 AND i.revision=c.revision AND c.byte_size=$3 AND l.property_key IS NULL AND l.kind='link'
                AND l.resolved_asset AND l.resolved_id IS NOT NULL`, nb.ID, *nb.SourceRootID, 5+8*shape.assets); n != 400*shape.assets {
				t.Fatalf("real parsed/resolved old F = %d, want %d", n, 400*shape.assets)
			}
			if n := count(t, tm.pool, "SELECT count(DISTINCT source_hash) FROM nodes WHERE notebook_id=$1 AND name LIKE 'S%'", nb.ID); n != 400 {
				t.Fatalf("distinct source hashes %d", n)
			}
			var sources []string
			for sample := range 3 {
				sources = append(sources, tm.createPageWith(t, "alice", nb.ID, *nb.SourceRootID, fmt.Sprintf("T%d", sample), fmt.Sprintf("Independent T%d evidence", sample)))
				var outputs []sqlPage
				for i := range shape.ops - 1 {
					outputs = append(outputs, sqlPage{"", fmt.Sprintf("P%d_%02d", sample, i), ""})
				}
				seedPagesSQL(t, tm.pool, nb.ID, outputs)
			}
			if shape.raw > 0 {
				ingestCostFill(t, tm, nb.ID, shape.raw)
			}
			m := tm.agent(t, "alice", nb.ID)
			m.handshakeAs(t, "ingest-cost")
			beforeName := names[0]
			for sample := range 3 {
				nextName := []string{"zz.x", "zz.y", "zz.z"}[sample]
				ops := []map[string]any{opMove("raw/"+beforeName, "raw/"+nextName)}
				for i := range shape.ops - 1 {
					text := fmt.Sprintf("sample %d item %d", sample, i)
					if i == 0 {
						text = fmt.Sprintf("[asset](raw/%s) [[raw/T%d]]", names[1+sample], sample)
					}
					name := fmt.Sprintf("P%d_%02d", sample, i)
					id := ingestCostID(t, tm, nb.ID, name)
					ops = append(ops, opWriteOn(name+".md", text, versionOf(id, 1)))
				}
				if len(ops) != shape.ops {
					t.Fatal("wrong actual batch operation count")
				}
				trace.start()
				var answer mcpAnswer
				metric, err := virtualCostMeasure(t, func() error {
					answer = m.batch(t, ops...)
					if answer.status != http.StatusOK {
						return fmt.Errorf("batch failed: %d %.1000s", answer.status, answer.body)
					}
					return nil
				})
				queries, statements, indexNS := trace.stop()
				ingestCostEvent(t, fmt.Sprintf("%s/%d", shape.name, sample), metric, queries, indexNS, map[string]any{"old_f_pairs": 400 * shape.assets, "source_body_bytes": 5 + 8*shape.assets, "ops": len(ops), "raw": shape.raw, "request_budget_ns": int64(ingestCostBatchBudget)})
				// Preserve the exact transport/tool reply and read plans before any
				// fatal request/budget assertion. These rereads are outside timing.
				t.Logf("INGESTRESPONSE case=%s/%d status=%d body=%s", shape.name, sample, answer.status, answer.body)
				if err != nil || time.Duration(metric.WallNS) >= ingestCostBatchBudget || answer.result(t)["isError"] == true {
					ingestCostPlans(t, tm, statements, true)
				}
				if err != nil {
					t.Fatal(err)
				}
				if time.Duration(metric.WallNS) >= ingestCostBatchBudget {
					t.Fatal("maximum batch exceeded original request budget")
				}
				if answer.result(t)["isError"] == true {
					t.Fatalf("batch failure after preserved metrics: %.1000s", answer.body)
				}
				line := (&mcpApp{logs: logs}).loggedLast(t, "batch applied")
				if line == nil || line["rewritten"] != float64(400) {
					t.Fatalf("actual rewrite line %v, want400", line)
				}
				if ms, ok := line["unit_ms"].(float64); !ok || ms >= 11000 {
					t.Fatalf("batch unit exceeds original 11s guard: %v", line)
				}
				result := answerOf[mcpBatched](t, answer)
				if result.ChangesetID == nil || !result.Changed || result.ResultsCount != shape.ops {
					t.Fatalf("batch result %+v", result)
				}
				ingestCostMark(t, tm, *result.ChangesetID, sources[sample])
				// All 400 source rewrites take the prepared route. The ordinary
				// wiki outputs retain their original sentinel Inspect; residual
				// SQL returns the 400 Q links and two new output links.
				for _, want := range []struct {
					name        string
					calls, rows int64
				}{
					{"InspectPageLinksForPreparation", 400, int64(400 * shape.assets)},
					{"InspectPageLinksForRetention", int64(shape.ops - 1), int64(shape.ops - 1)},
					{"LinksReachedExcept", 1, 402},
					{"IngestOldBodyReferencesFirst", 0, 0},
					{"IngestOldBodyReferencesAfter", 0, 0},
				} {
					got := queries[want.name]
					if got.Calls != want.calls || got.Rows != want.rows || got.Errors != 0 {
						t.Fatalf("prepared/fallback work %s: %+v, want calls=%d rows=%d errors=0", want.name, got, want.calls, want.rows)
					}
				}
				if queries["IngestNovelSources"].Calls != 1 {
					t.Fatalf("novel evidence not fully read: %+v", queries)
				}
				if n := count(t, tm.pool, `SELECT count(*) FROM page_contents c JOIN nodes n ON n.id=c.node_id
                    WHERE n.notebook_id=$1 AND n.name LIKE 'S%' AND c.revision=$2 AND c.byte_size=$3`, nb.ID, sample+2, 5+8*shape.assets); n != 400 {
					t.Fatalf("real source rewrites=%d, want400", n)
				}
				ingestCostPlans(t, tm, statements, sample == 0)
				beforeName = nextName
			}
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
		})
	}
}

// Parameters come from the actual successful request. EXPLAIN is separately
// reported; it does not inflate the measured request nor masquerade as PG CPU.
func ingestCostPlans(t *testing.T, tm acmeTeam, statements map[string]virtualCostStatement, run bool) {
	t.Helper()
	if !run {
		return
	}
	for _, name := range []string{"IngestNodesNow", "IngestHeadsFirst", "IngestHeadsAfter", "IngestIndexedHeads", "IngestNovelSources", "LintBodyReferencesFirst", "LintBodyReferencesAfter", "IngestOldBodyReferencesFirst", "IngestOldBodyReferencesAfter", "InspectPageLinksForPreparation", "LinksReachedExcept"} {
		s, ok := statements[name]
		if !ok {
			continue
		}
		for _, mode := range []string{"force_custom_plan", "force_generic_plan"} {
			tx, err := tm.pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(t.Context(), "SET LOCAL plan_cache_mode="+mode); err != nil {
				_ = tx.Rollback(t.Context())
				t.Fatal(err)
			}
			desc, err := tx.Conn().Prepare(t.Context(), "ingest_cost", s.SQL)
			if err != nil {
				_ = tx.Rollback(t.Context())
				t.Fatal(err)
			}
			// Actual pgx binary payload bytes for each captured argument, using
			// server-described OIDs. Zero bytes for nil is legal SQL NULL; these
			// are neither network frames nor the separate plan JSON length.
			if len(desc.ParamOIDs) != len(s.Args) {
				t.Fatal("captured parameter/OID arity differs")
			}
			parameterBytes := make([]int, len(s.Args))
			var encodingBefore, encodingAfter runtime.MemStats
			runtime.ReadMemStats(&encodingBefore)
			for i, arg := range s.Args {
				encoded, encodeErr := tx.Conn().TypeMap().Encode(desc.ParamOIDs[i], pgtype.BinaryFormatCode, arg, nil)
				if encodeErr != nil {
					_ = tx.Rollback(t.Context())
					t.Fatal(encodeErr)
				}
				parameterBytes[i] = len(encoded)
			}
			runtime.ReadMemStats(&encodingAfter)
			t.Logf("INGESTPARAM name=%s mode=%s oids=%v binary_payload_bytes=%v encoding_process_alloc_bytes=%d; outside request timing, not network frames or plan JSON", name, mode, desc.ParamOIDs, parameterBytes, encodingAfter.TotalAlloc-encodingBefore.TotalAlloc)
			prepared := func() [2]int64 {
				var out [2]int64
				if err := tx.QueryRow(t.Context(), "SELECT custom_plans,generic_plans FROM pg_prepared_statements WHERE name='ingest_cost'").Scan(&out[0], &out[1]); err != nil {
					t.Fatal(err)
				}
				return out
			}
			before := prepared()
			at := time.Now()
			rows, err := tx.Query(t.Context(), "ingest_cost", s.Args...)
			if err != nil {
				_ = tx.Rollback(t.Context())
				t.Fatal(err)
			}
			rowCount := 0
			for rows.Next() {
				if _, err := rows.Values(); err != nil {
					rows.Close()
					t.Fatal(err)
				}
				rowCount++
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			elapsed := time.Since(at)
			after := prepared()
			if mode == "force_custom_plan" && (after[0]-before[0] != 1 || after[1] != before[1]) ||
				mode == "force_generic_plan" && (after[1]-before[1] != 1 || after[0] != before[0]) {
				t.Fatalf("not an actual %s execution: %v -> %v", mode, before, after)
			}
			var plan json.RawMessage
			err = postgres.Planned(tx).QueryRow(t.Context(), "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+s.SQL, s.Args...).Scan(&plan)
			_ = tx.Conn().Deallocate(t.Context(), "ingest_cost")
			_ = tx.Rollback(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("INGESTPREPARED name=%s mode=%s before=%v after=%v rows=%d wall=%s oids=%v; following EXPLAIN is a separate reread with the same parameters", name, mode, before, after, rowCount, elapsed, desc.ParamOIDs)
			t.Logf("INGESTPLAN name=%s mode=%s parameters=%d bytes=%d plan=%s", name, mode, len(s.Args), len(plan), plan)
		}
	}
}
