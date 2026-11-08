//go:build !race

package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The guard's notebook: startCostPages pages, raw and startCostArea pages
// in it among them. The design's was of 100,000 pages, 30,000 in raw (M11/P1
// design 7.1); at that scale the start took 2.16-2.41 s here, the medians of
// two rounds, of its 3 s deadline then, lint's, and CI's runners take 1.16
// to 2.33 times as long as this machine over lint's 100,000 pages (its
// guard's summary, 12 runs of main): 2.5-5.6 s. So the start has three
// quarters of the request's (decision 232), 4.5 s here, and the guard half
// the notebook, which the slowest of them start in 2.7 s; the 100,000 pages
// that 4.5 s does not hold on them go to M12's handoff, as 232 says, and
// production's 11.25 s holds them (the S2 record).
const (
	startCostPages = 50000
	startCostArea  = 15000
)

// startCostDeadline is the start's under the guard's 6 s request.
const startCostDeadline = wikiCostRequest * 3 / 4

// startCostBound is the median of three starts the test below allows in
// each state and plan: three times the larger of the medians of two local
// rounds, rounded up to 100 ms, within the start's deadline (decisions
// 232, 236). The rounds' medians, custom then generic, each never analyzed,
// analyzed by a start of the notebook, and with another upgrade's alone,
// were 1.14, 1.12, 1.15, 1.13, 1.14 and 1.11 s, and 1.11, 1.06, 1.07, 1.04,
// 1.04 and 1.05 s; three times 1.15 s is 3.44 s. Of a start, the
// preparation took 0.69-0.80 s (its wanted rule 0.63-0.74 s, the rule's
// second pass 0.17-0.22 s in 322 reads), Capture 0.11-0.14 s, the seed
// 0.10-0.13 s, SetWanted 78-95 ms, and the ANALYZE after the commit 28-53 ms.
const startCostBound = 3500 * time.Millisecond

// The start's cost guard (M11/P1 design 7.1, decision 232): a notebook of
// startCostPages pages, startCostArea of them in raw, the source root the
// start proposes, each with ten wanted links, and one in ten of the others
// with one, of 1,000 targets; the start under lint's cost configuration, a
// 6 s request, with its own deadline of three quarters of it.
// upgrade_nodes never analyzed, analyzed by a start of this notebook, and
// with another upgrade's rows alone, each in custom and in generic plans,
// set for the database. Plan first: in the first start of each, inside its
// transaction, SetBaselineWanted's plans for its arguments and generic read
// upgrade_nodes once, or by its primary key for each id. Time second: each
// start answers 200, the median of three within startCostBound; -v prints
// each start's parts: Capture, the seed, the preparation, its wanted rule
// and the rule's second pass, SetWanted, and the ANALYZE after the commit.
func TestUpgradeStartTakesItsTimeAtAnyStatistics(t *testing.T) {
	if testing.Short() {
		t.Skip("the start's cost guard runs alone, without race")
	}
	contract := apitest.Load(t)
	url, keyFile := pgtest.NewDatabase(t), signingKeyFile(t)
	pool := connect(t, url)
	lintCostExec(t, pool, "ALTER TABLE upgrade_nodes SET (autovacuum_enabled = false)")
	var database string
	if err := pool.QueryRow(t.Context(), "SELECT current_database()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	var token, nb, raw string
	var medians []time.Duration
	for _, mode := range []string{"force_custom_plan", "force_generic_plan"} {
		t.Run(mode, func(t *testing.T) {
			// The app's connections open after this and take it.
			lintCostExec(t, pool, "ALTER DATABASE "+pgx.Identifier{database}.Sanitize()+" SET plan_cache_mode = "+mode)
			trace := &startCostTrace{}
			cfg := testConfig(t, url, false)
			cfg.Auth.JWT.PrivateKeyFile, cfg.Server.RequestTimeout, cfg.Server.WriteTimeout = keyFile, wikiCostRequest, 15*time.Second
			a, err := wireApp(context.Background(), cfg, slog.New(slog.DiscardHandler), migrations.FS(), testWebUI(), wiring{
				queryTracer: trace,
				wiki: func(d *wiki.Deps) {
					if d.StartTimeout != startCostDeadline {
						t.Errorf("the start's deadline is %v, want %v under a %v request", d.StartTimeout, startCostDeadline, wikiCostRequest)
					}
					d.Rules = startCostRules{d.Rules, trace}
					d.Upgrades = startCostUpgrades{d.Upgrades, trace}
				}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(a.close)
			base := runApp(t, a)
			if token == "" {
				token, nb, raw = startCostSeed(t, contract, base, pool)
			}
			for _, statistics := range []string{"never analyzed", "analyzed by a start of this notebook", "another upgrade's alone"} {
				t.Run(statistics, func(t *testing.T) {
					var took []time.Duration
					for sample := range 3 {
						other := uuid.NewV7()
						switch statistics {
						case "never analyzed":
							lintCostExec(t, pool, "SELECT pg_clear_relation_stats('public', 'upgrade_nodes')")
							lintCostExec(t, pool, `SELECT pg_clear_attribute_stats('public', 'upgrade_nodes', attname, false)
								FROM pg_attribute WHERE attrelid = 'upgrade_nodes'::regclass AND attnum > 0 AND NOT attisdropped`)
						case "another upgrade's alone":
							lintCostExec(t, pool, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name,
									sort_order, revision, content_hash, byte_size, wanted, accepted_hash)
								SELECT $1, gen_random_uuid(), $2, true, 'page', NULL, 'p' || g, g, 1, sha256(g::text::bytea), 1, g % 3,
									sha256(g::text::bytea)
								FROM generate_series(1, 100000) g`, other, uuid.NewV7())
							lintCostExec(t, pool, "ANALYZE upgrade_nodes")
						}
						trace.begin(sample == 0)
						started := time.Now()
						status, answer := ask(t, contract, http.MethodPost, base+"/api/v0/notebooks/"+nb+"/upgrade", token, `{}`)
						elapsed := time.Since(started)
						s := trace.end()
						if status != http.StatusOK || s.PlanErr != nil {
							t.Fatalf("the start = %d %s after %v; its plans: %v", status, answer, elapsed, s.PlanErr)
						}
						var u upgradeAnswer
						decodeAnswer(t, answer, &u)
						startCostCheck(t, pool, u, raw, s, sample == 0)
						// The plans' EXPLAINs ran inside the first start: not its time.
						elapsed -= s.Explain
						took = append(took, elapsed)
						lintCostLog(t, "upgrade start", map[string]any{"mode": mode, "statistics": statistics, "sample": sample,
							"elapsed_ms": ms(elapsed), "explain_ms": ms(s.Explain), "capture_ms": ms(s.Capture), "seed_ms": ms(s.Seed),
							"prepare_ms": ms(s.Prepare), "wanted_rule_ms": ms(s.WantedRule), "area_pass_ms": ms(s.AreaPass),
							"area_reads": s.AreaReads, "set_wanted_ms": ms(s.SetWanted), "analyze_ms": ms(s.Analyze),
							"nodes": u.Baseline.Nodes, "wanted": u.Baseline.Wanted, "plans": s.Plans})
						// The upgrade ends as its rollback's ending would leave it;
						// AGENTS goes, so that the next start writes it again.
						lintCostExec(t, pool, "DELETE FROM upgrade_nodes WHERE upgrade_id = $1 OR upgrade_id = $2", u.ID, other)
						lintCostExec(t, pool, "DELETE FROM notebook_upgrades WHERE id = $1", u.ID)
						lintCostExec(t, pool, `UPDATE notebooks SET upgrade_id = NULL, upgrade_state = NULL, upgrade_by_id = NULL,
							upgrade_at = NULL WHERE id = $1`, nb)
						lintCostExec(t, pool, `UPDATE nodes SET deleted_at = now()
							WHERE notebook_id = $1 AND parent_id IS NULL AND name_key = 'agents' AND deleted_at IS NULL`, nb)
						// As autovacuum would; the table keeps its statistics.
						lintCostExec(t, pool, "VACUUM upgrade_nodes")
					}
					slices.Sort(took)
					medians = append(medians, took[1])
					if took[1] > startCostBound {
						t.Errorf("the median of three starts took %v, past %v: %v", took[1], startCostBound, took)
					}
				})
			}
		})
	}
	lintCostLog(t, "upgrade start medians", medians)
}

// startCostSeed makes alice's notebook through the API and SQL: raw at its
// root, startCostArea pages in raw with ten wanted links each, the others at
// the root, one in ten of them with one, of 1,000 targets.
func startCostSeed(t *testing.T, contract *apitest.Contract, base string, pool *pgxpool.Pool) (token, nb, raw string) {
	t.Helper()
	started := time.Now()
	token = registerAccount(t, contract, base, "alice@example.com").AccessToken
	if status, answer := ask(t, contract, http.MethodPost, base+"/api/v0/workspaces", token, `{"name":"Acme","slug":"acme"}`); status != http.StatusCreated {
		t.Fatalf("create acme = %d %s", status, answer)
	}
	var created struct{ ID string }
	status, answer := ask(t, contract, http.MethodPost, base+"/api/v0/workspaces/acme/notebooks", token, `{"name":"A hundred thousand pages"}`)
	if status != http.StatusCreated {
		t.Fatalf("create the notebook = %d %s", status, answer)
	}
	decodeAnswer(t, answer, &created)
	nb = created.ID
	status, answer = ask(t, contract, http.MethodPost, base+"/api/v0/notebooks/"+nb+"/pages", token, `{"parent_id":null,"title":"raw"}`)
	if status != http.StatusCreated {
		t.Fatalf("create raw = %d %s", status, answer)
	}
	decodeAnswer(t, answer, &created)
	raw = created.ID
	lintCostExec(t, pool, `INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id,
			created_at, updated_at)
		SELECT gen_random_uuid(), b.id, 'page', CASE WHEN k <= $4 THEN $2::uuid END, s.name, s.name, k, b.created_by_id,
			b.created_by_id, now(), now()
		FROM notebooks b, generate_series(1, $3::integer - 1) k,
			LATERAL (SELECT CASE WHEN k <= $4 THEN 's' ELSE 'p' END || k AS name) s
		WHERE b.id = $1`, nb, raw, startCostPages, startCostArea)
	lintCostExec(t, pool, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, 'x', 1, sha256('x'), 1, created_by_id, now() FROM nodes WHERE notebook_id = $1 AND id <> $2`, nb, raw)
	lintCostExec(t, pool, `INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
		SELECT id, notebook_id, 1, $3, true FROM nodes WHERE notebook_id = $1 AND id <> $2`, nb, raw, linking.Extractor)
	lintCostExec(t, pool, `INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind, target, target_key, ambiguous)
		SELECT n.id, k, k + 1, n.notebook_id, 'wikilink', t.key, t.key, false
		FROM nodes n, generate_series(0, 9) k, LATERAL (SELECT 'w' || n.sort_order::integer || '-' || k AS key) t
		WHERE n.notebook_id = $1 AND n.parent_id = $2`, nb, raw)
	lintCostExec(t, pool, `INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind, target, target_key, ambiguous)
		SELECT n.id, 0, 1, n.notebook_id, 'wikilink', t.key, t.key, false
		FROM nodes n, LATERAL (SELECT 'missing' || (n.sort_order::integer / 10 % 1000) AS key) t
		WHERE n.notebook_id = $1 AND n.parent_id IS NULL AND n.id <> $2 AND n.sort_order::integer % 10 = 0`, nb, raw)
	lintCostExec(t, pool, "ANALYZE nodes, page_contents, indexed_pages, page_links")
	lintCostLog(t, "upgrade start seed", map[string]any{"elapsed_ms": ms(time.Since(started))})
	return token, nb, raw
}

// startCostCheck fails t unless u is the start of the seeded notebook:
// its baseline of startCostPages pages, AGENTS written, raw the source
// root, 1,000 wanted, each of raw's pages with its ten wanted targets, each
// of the others with a link with its one; and the start read the area, checked its plans
// when asked, and ran its ANALYZE, which the next state takes.
func startCostCheck(t *testing.T, pool *pgxpool.Pool, u upgradeAnswer, raw string, s startCostParts, plans bool) {
	t.Helper()
	if u.Baseline.Nodes != startCostPages || u.Baseline.Pages != startCostPages || u.Baseline.Wanted != 1000 ||
		u.SourceRootID != raw || len(u.SeededPageIDs) != 1 {
		t.Errorf("the start %+v, want %d pages, raw its source root, AGENTS written, 1,000 wanted", u, startCostPages)
	}
	var area, outside int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FILTER (WHERE n.parent_id = $2 AND u.wanted = 10),
			count(*) FILTER (WHERE n.parent_id IS NULL AND u.wanted = 1)
		FROM upgrade_nodes u JOIN nodes n ON n.id = u.node_id WHERE u.upgrade_id = $1`, u.ID, raw).Scan(&area, &outside); err != nil {
		t.Fatal(err)
	}
	if outside != (startCostPages-1)/10-startCostArea/10 || area != startCostArea {
		t.Errorf("wanted by page: %d of raw's pages with 10, %d outside with 1; want %d and %d", area, outside, startCostArea,
			(startCostPages-1)/10-startCostArea/10)
	}
	if s.Analyzed != 1 || s.AreaReads == 0 || s.Planned != plans {
		t.Errorf("the start analyzed %d times, read the area %d times, checked its plans %v, want once, some, %v", s.Analyzed,
			s.AreaReads, s.Planned, plans)
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// startCostTrace sums the start's statements by their sqlc names and keeps
// SetBaselineWanted's text and arguments for its plans.
type startCostTrace struct {
	mu         sync.Mutex
	plans      bool
	parts      startCostParts
	captured   time.Time // CaptureBaseline's end
	wanted     string
	wantedArgs []any
}

// startCostParts are a start's parts, as its ports and statements took
// them.
type startCostParts struct {
	Capture, Seed, Prepare, WantedRule, AreaPass, SetWanted, Analyze, Explain time.Duration
	AreaReads, Analyzed                                                       int
	Planned                                                                   bool
	PlanErr                                                                   error
	Plans                                                                     []string // custom, generic
}

type startCostKey struct{}

type startCostQuery struct {
	name string
	at   time.Time
	sql  string
	args []any
}

func (s *startCostTrace) begin(plans bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plans, s.parts = plans, startCostParts{}
}

func (s *startCostTrace) end() startCostParts {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parts
}

func (s *startCostTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	rest, ok := strings.CutPrefix(data.SQL, "-- name: ")
	if !ok {
		return ctx
	}
	name, _, _ := strings.Cut(rest, " ")
	return context.WithValue(ctx, startCostKey{}, startCostQuery{name, time.Now(), data.SQL, data.Args})
}

func (s *startCostTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	q, ok := ctx.Value(startCostKey{}).(startCostQuery)
	if !ok || data.Err != nil {
		return
	}
	took := time.Since(q.at)
	s.mu.Lock()
	defer s.mu.Unlock()
	switch q.name {
	case "CaptureBaseline":
		s.parts.Capture, s.captured = took, time.Now()
	case "LintAreaWantedEdgeHeads":
		s.parts.AreaPass += took
		s.parts.AreaReads++
	case "SetBaselineWanted":
		s.parts.SetWanted, s.wanted, s.wantedArgs = took, q.sql, q.args
	case "AnalyzeUpgradeNodes":
		s.parts.Analyze = took
		s.parts.Analyzed++
	}
}

// startCostRules times the start's preparation, the one that asks for the
// wanted counts by page, and its wanted rule, both passes.
type startCostRules struct {
	wiki.Rules
	s *startCostTrace
}

func (r startCostRules) PrepareRules(ctx context.Context, in wiki.ReadInput, q wiki.RuleQuery) (wiki.PreparedRules, error) {
	started := time.Now()
	prepared, err := r.Rules.PrepareRules(ctx, in, q)
	if q.WantedByPage {
		r.s.mu.Lock()
		r.s.parts.Seed, r.s.parts.Prepare = started.Sub(r.s.captured), time.Since(started)
		if err == nil {
			for _, rule := range prepared.Timings().Rules {
				r.s.parts.WantedRule += rule.Duration
			}
		}
		r.s.mu.Unlock()
	}
	return prepared, err
}

// startCostUpgrades checks SetBaselineWanted's plans in the start's
// transaction, after it ran, when the trace asks.
type startCostUpgrades struct {
	wiki.UpgradePages
	s *startCostTrace
}

func (p startCostUpgrades) SetWanted(ctx context.Context, upgradeID uuid.UUID, wanted map[uuid.UUID]int) error {
	if err := p.UpgradePages.SetWanted(ctx, upgradeID, wanted); err != nil {
		return err
	}
	p.s.mu.Lock()
	plans, sql, args := p.s.plans, p.s.wanted, p.s.wantedArgs
	p.s.mu.Unlock()
	if !plans {
		return nil
	}
	started := time.Now()
	shapes, err := startCostPlans(ctx, sql, args)
	p.s.mu.Lock()
	p.s.parts.Planned, p.s.parts.Explain, p.s.parts.PlanErr, p.s.parts.Plans = true, time.Since(started), err, shapes
	p.s.mu.Unlock()
	return nil // the test reports err; an EXPLAIN's error leaves the transaction failed
}

// startCostPlans returns the shapes of sql's plans in ctx's transaction,
// for args and generic, and an error, which the test reports, unless each
// reads upgrade_nodes in one scan, once or by upgrade_nodes_pkey for each
// id with both of its columns in the index's condition.
func startCostPlans(ctx context.Context, sql string, args []any) ([]string, error) {
	tx, ok := postgres.TxFrom(ctx)
	if !ok || !strings.HasPrefix(sql, "-- name: SetBaselineWanted ") {
		return nil, errors.New("SetWanted ran outside a transaction or before SetBaselineWanted")
	}
	var shapes []string
	for _, generic := range []bool{false, true} {
		var raw []byte
		if generic {
			// A generic plan's parameters have no values: the simple protocol.
			results, err := tx.Conn().PgConn().Exec(ctx, "EXPLAIN (GENERIC_PLAN, FORMAT JSON) "+sql).ReadAll()
			if err != nil {
				return shapes, fmt.Errorf("SetBaselineWanted's generic plan: %w", err)
			}
			if len(results) != 1 || len(results[0].Rows) != 1 {
				return shapes, fmt.Errorf("SetBaselineWanted's generic plan: %d results", len(results))
			}
			raw = results[0].Rows[0][0]
		} else if err := tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
			return shapes, fmt.Errorf("SetBaselineWanted's plan: %w", err)
		}
		var plans []struct {
			Plan startCostNode `json:"Plan"`
		}
		if err := json.Unmarshal(raw, &plans); err != nil {
			return shapes, fmt.Errorf("SetBaselineWanted's plan, generic %v: %s: %w", generic, raw, err)
		}
		if len(plans) != 1 {
			return shapes, fmt.Errorf("SetBaselineWanted's plan, generic %v: %d plans: %s", generic, len(plans), raw)
		}
		shapes = append(shapes, plans[0].Plan.shape())
		var scans []startCostScan
		plans[0].Plan.scans(false, &scans)
		probe := func(s startCostScan) bool {
			return s.Index == "upgrade_nodes_pkey" && strings.Contains(s.Cond, "upgrade_id = ") && strings.Contains(s.Cond, "node_id = ")
		}
		if len(scans) != 1 || scans[0].repeated && !probe(scans[0]) {
			return shapes, fmt.Errorf("SetBaselineWanted's plan, generic %v, reads upgrade_nodes other than once or by its key for each id: %s",
				generic, raw)
		}
	}
	return shapes, nil
}

// shape is n and the nodes below it in brackets, each with its relation
// and index.
func (n startCostNode) shape() string {
	s := n.Type
	if n.Relation != "" {
		s += " on " + n.Relation
	}
	if n.Index != "" {
		s += " by " + n.Index
	}
	var below []string
	for _, m := range n.Plans {
		below = append(below, m.shape())
	}
	if len(below) > 0 {
		s += " [" + strings.Join(below, "; ") + "]"
	}
	return s
}

// startCostNode is a node of a plan in JSON, as much as the plans' checks
// read.
type startCostNode struct {
	Type     string          `json:"Node Type"`
	Relation string          `json:"Relation Name"`
	Index    string          `json:"Index Name"`
	Cond     string          `json:"Index Cond"`
	Parent   string          `json:"Parent Relationship"`
	Plans    []startCostNode `json:"Plans"`
}

// startCostScan is a scan of upgrade_nodes, repeated when it is below a
// Nested Loop's inner side with no Materialize or Hash between.
type startCostScan struct {
	startCostNode
	repeated bool
}

func (n startCostNode) scans(repeated bool, into *[]startCostScan) {
	if n.Type == "Materialize" || n.Type == "Hash" {
		repeated = false
	}
	if n.Relation == "upgrade_nodes" && n.Type != "ModifyTable" {
		scan := startCostScan{n, repeated}
		for _, below := range n.Plans { // a Bitmap Heap Scan's index is its child's
			if below.Index != "" {
				scan.Index, scan.Cond = below.Index, below.Cond
			}
		}
		*into = append(*into, scan)
	}
	for _, below := range n.Plans {
		below.scans(repeated || n.Type == "Nested Loop" && below.Parent == "Inner", into)
	}
}
