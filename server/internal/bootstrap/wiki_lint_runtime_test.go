package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// These tests exercise the module root installed by wireApp. Authentication
// tokens seed real users/sessions; the internal caller carries that actor and
// the actual access.Authorizer reads their current memberships for every call.
type lintRuntimeFixture struct {
	tm     acmeTeam
	module *wiki.Module
	pool   *pgxpool.Pool
	trace  *lintRuntimeTrace
	events chan wiki.RunEvent
	logs   syncBuffer
}

func newLintRuntime(t *testing.T, adjust func(*wiki.Deps)) *lintRuntimeFixture {
	t.Helper()
	f, w := newLintRuntimeWiring(t, adjust)
	f.tm = newAcmeTeamOn(t, "member", "member", func(cfg *config.Config) {
		cfg.Database.MaxConns = 6
	}, &w, slog.New(slog.NewJSONHandler(&f.logs, nil)))
	if f.module == nil || f.pool == nil {
		t.Fatal("wireApp did not expose its actual lint module and traced connection pool")
	}
	return f
}

func newLintRuntimeWiring(t *testing.T, adjust func(*wiki.Deps)) (*lintRuntimeFixture, wiring) {
	t.Helper()
	f := &lintRuntimeFixture{trace: &lintRuntimeTrace{}, events: make(chan wiki.RunEvent, 256)}
	var tracePool *pgxpool.Pool
	w := wiring{
		page: func(d *page.Deps) {
			cfg := d.Pool.Config()
			f.trace.previous = cfg.ConnConfig.Tracer
			cfg.ConnConfig.Tracer = f.trace
			var err error
			tracePool, err = pgxpool.NewWithConfig(t.Context(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			f.pool = tracePool
			t.Cleanup(tracePool.Close)
		},
		wiki: func(d *wiki.Deps) {
			if d.MaxLintRuns != 1 || d.LintTimeout != 2*time.Second {
				t.Fatalf("runtime bounds=%d/%s, want the real six-connection/4s request configuration's 1/2s", d.MaxLintRuns, d.LintTimeout)
			}
			d.Snapshots = &lintRuntimeSnapshots{Snapshots: d.Snapshots, pool: tracePool}
			d.Rules = lintRuntimeRules{Rules: d.Rules, pool: tracePool}
			d.Notebooks = lintRuntimeNotebooks{Notebooks: d.Notebooks, trace: f.trace, pool: tracePool}
			d.Paths = lintRuntimePaths{paths: d.Paths, trace: f.trace, pool: tracePool}
			d.RunObserver = func(event wiki.RunEvent) { f.events <- event }
			if adjust != nil {
				adjust(d)
			}
		},
		wikiReady: func(module *wiki.Module) { f.module = module },
	}
	return f, w
}

func (f *lintRuntimeFixture) actor(t *testing.T, name string) context.Context {
	t.Helper()
	var actor shared.Actor
	if err := f.tm.pool.QueryRow(t.Context(), `SELECT u.id, s.id FROM users u
		JOIN auth_sessions s ON s.user_id=u.id WHERE u.email=$1 AND s.expires_at>now()
		ORDER BY s.id LIMIT 1`, name+"@example.com").Scan(&actor.UserID, &actor.SessionID); err != nil {
		t.Fatal(err)
	}
	if !actor.Valid() {
		t.Fatal("seeded user/session is not a valid internal actor")
	}
	return shared.WithActor(t.Context(), actor)
}

type lintCallResult struct {
	result wiki.LintResult
	err    error
}

func (f *lintRuntimeFixture) start(ctx context.Context, scope wiki.LintScope, query wiki.LintQuery) <-chan lintCallResult {
	done := make(chan lintCallResult, 1)
	go func() {
		result, err := f.module.Execute(ctx, scope, query)
		done <- lintCallResult{result: result, err: err}
	}()
	return done
}

func lintReceive(t *testing.T, done <-chan lintCallResult) lintCallResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(interleavingWait):
		t.Fatal("lint caller did not return within the existing interleaving wait")
		return lintCallResult{}
	}
}

func (f *lintRuntimeFixture) event(t *testing.T, kind wiki.RunEventKind, flight uint64) wiki.RunEvent {
	t.Helper()
	timer := time.NewTimer(interleavingWait)
	defer timer.Stop()
	for {
		select {
		case event := <-f.events:
			if event.Kind == kind && (flight == 0 || event.FlightID == flight) {
				return event
			}
		case <-timer.C:
			t.Fatalf("no actual run event kind=%d flight=%d", kind, flight)
			return wiki.RunEvent{}
		}
	}
}

func lintErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	var problem *shared.Error
	if !errors.As(err, &problem) || problem.Code != want {
		t.Fatalf("lint error=%v, want code %s", err, want)
	}
}

func lintResultRule(t *testing.T, result wiki.LintResult, rule wiki.LintRule) wiki.LintRuleResult {
	t.Helper()
	for _, got := range result.Rules {
		if got.Rule == rule {
			return got
		}
	}
	t.Fatalf("real result has no %s rule: %+v", rule, result)
	return wiki.LintRuleResult{}
}

type lintSnapshotKey struct{}

// OnConn makes the original TxManager open the real transaction on this
// traced connection. Its complete panic/error cleanup precedes Release.
type lintRuntimeSnapshots struct {
	shared.Snapshots
	pool *pgxpool.Pool
	next atomic.Uint64
}

func (s *lintRuntimeSnapshots) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	ctx = context.WithValue(ctx, lintSnapshotKey{}, s.next.Add(1))
	return s.Snapshots.WithinSnapshot(postgres.OnConn(ctx, conn), fn)
}

type lintRuntimeNotebooks struct {
	wiki.Notebooks
	trace *lintRuntimeTrace
	pool  *pgxpool.Pool
}

// A snapshot-removal control must still reach an observed real asset wait.
// With no transaction, OnConn merely traces the original autocommit reads; it
// does not add a snapshot or make the broken operation transactionally sound.
type lintRuntimeRules struct {
	wiki.Rules
	pool *pgxpool.Pool
}

func (r lintRuntimeRules) PrepareRules(ctx context.Context, input wiki.ReadInput, query wiki.RuleQuery) (wiki.PreparedRules, error) {
	if postgres.InTx(ctx) {
		return r.Rules.PrepareRules(ctx, input, query)
	}
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	return r.Rules.PrepareRules(postgres.OnConn(ctx, conn), input, query)
}

func (n lintRuntimeNotebooks) StateOf(ctx context.Context, id uuid.UUID) (wiki.NotebookState, bool, error) {
	state, found, err := n.Notebooks.StateOf(ctx, id)
	if err == nil && postgres.InTx(ctx) {
		err = n.trace.observe(ctx, n.pool, "state")
	}
	return state, found, err
}

type lintRuntimePaths struct {
	paths wiki.Paths
	trace *lintRuntimeTrace
	pool  *pgxpool.Pool
}

func (p lintRuntimePaths) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	paths, err := p.paths.Paths(ctx, notebookID, ids)
	if err == nil {
		err = p.trace.observe(ctx, p.pool, "paths")
	}
	return paths, err
}

type lintSQLRecord struct {
	Snapshot uint64
	PID      uint32
	Name     string
	SQL      string
}

type lintSnapshotObservation struct {
	Snapshot                   uint64
	PID                        uint32
	Phase, Isolation, ReadOnly string
	View                       string
}

type lintRuntimeTrace struct {
	previous     pgx.QueryTracer
	mu           sync.Mutex
	queries      []lintSQLRecord
	observations []lintSnapshotObservation
}

func (s *lintRuntimeTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if s.previous != nil {
		ctx = s.previous.TraceQueryStart(ctx, conn, data)
	}
	id, _ := ctx.Value(lintSnapshotKey{}).(uint64)
	fields := strings.Fields(data.SQL)
	name := ""
	if len(fields) > 2 && fields[0] == "--" && fields[1] == "name:" {
		name = fields[2]
	}
	s.mu.Lock()
	s.queries = append(s.queries, lintSQLRecord{Snapshot: id, PID: conn.PgConn().PID(), Name: name, SQL: data.SQL})
	s.mu.Unlock()
	return ctx
}

func (s *lintRuntimeTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if s.previous != nil {
		s.previous.TraceQueryEnd(ctx, conn, data)
	}
}

func (s *lintRuntimeTrace) observe(ctx context.Context, pool *pgxpool.Pool, phase string) error {
	id, _ := ctx.Value(lintSnapshotKey{}).(uint64)
	observation := lintSnapshotObservation{Snapshot: id, Phase: phase}
	// The first observation runs only after StateOf completed, so it cannot
	// accidentally establish the worker's snapshot before its first real read.
	err := postgres.DB(ctx, pool).QueryRow(ctx, `/* lint-test-observation */ SELECT pg_backend_pid(),
		current_setting('transaction_isolation'), current_setting('transaction_read_only'), pg_current_snapshot()::text`).Scan(
		&observation.PID, &observation.Isolation, &observation.ReadOnly, &observation.View)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.observations = append(s.observations, observation)
	s.mu.Unlock()
	return nil
}

func (s *lintRuntimeTrace) copy() ([]lintSQLRecord, []lintSnapshotObservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.queries), slices.Clone(s.observations)
}

func (f *lintRuntimeFixture) assertSnapshot(t *testing.T, wantedPID uint32) {
	t.Helper()
	queries, observations := f.trace.copy()
	var names []string
	var snapshot uint64
	for _, query := range queries {
		if query.Name == "" {
			continue
		}
		if query.PID != wantedPID || query.Snapshot == 0 {
			t.Fatalf("payload query escaped the actual snapshot backend: %+v, want PID %d", query, wantedPID)
		}
		if snapshot == 0 {
			snapshot = query.Snapshot
		} else if snapshot != query.Snapshot {
			t.Fatal("one lint worker used more than one transaction")
		}
		names = append(names, query.Name)
	}
	if len(names) < 4 || names[0] != "FindNotebook" || names[len(names)-1] != "LinkTargetsByIDs" ||
		!slices.Contains(names, "AttachmentInfos") {
		t.Fatalf("snapshot's payload SQL=%v, want StateOf first, real asset read and Paths last", names)
	}
	if len(observations) != 2 || observations[0].Phase != "state" || observations[1].Phase != "paths" {
		t.Fatalf("snapshot observations=%+v, want StateOf then Paths", observations)
	}
	for _, got := range observations {
		if got.PID != wantedPID || got.Snapshot != snapshot || got.Isolation != "repeatable read" || got.ReadOnly != "on" || got.View != observations[0].View {
			t.Fatalf("snapshot changed or was not RR/read-only: %+v", observations)
		}
	}
	t.Logf("actual lint backend=%d, snapshot=%s, first/last=%s/%s, payload SQL=%v", wantedPID, observations[0].View, names[0], names[len(names)-1], names)
}

// A lock witness identifies this worker's PID and this relation. Background
// jobs may wait too; their waits cannot stand in for the lint reader's wait.
func (f *lintRuntimeFixture) waitAssetLock(t *testing.T) uint32 {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(interleavingWait)
	defer deadline.Stop()
	for {
		queries, _ := f.trace.copy()
		for _, query := range queries {
			if query.Name != "AttachmentInfos" {
				continue
			}
			var waiting bool
			if err := f.tm.pool.QueryRow(t.Context(), `SELECT EXISTS (
				SELECT 1 FROM pg_locks l JOIN pg_stat_activity a USING(pid)
				WHERE l.pid=$1 AND l.relation='asset_blobs'::regclass AND NOT l.granted
				AND l.locktype='relation' AND a.wait_event_type='Lock' AND a.state='active'
				AND a.query LIKE '-- name: AttachmentInfos%')`, query.PID).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				t.Logf("actual pid=%d waits on asset_blobs relation lock in AttachmentInfos", query.PID)
				return query.PID
			}
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("actual lint backend never waited on asset_blobs; trace=%+v", queries)
			return 0
		}
	}
}

func (f *lintRuntimeFixture) holdAssets(t *testing.T) func() {
	t.Helper()
	tx, err := f.tm.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), "LOCK TABLE asset_blobs IN ACCESS EXCLUSIVE MODE"); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatal(err)
	}
	var once sync.Once
	release := func() { once.Do(func() { _ = tx.Rollback(context.Background()) }) }
	t.Cleanup(release)
	return release
}

func (f *lintRuntimeFixture) seedPending(t *testing.T) (guardedNotebook, string, string) {
	t.Helper()
	nb := f.tm.llmWiki(t, "Runtime wiki")
	orphan := f.tm.createPageWith(t, "alice", nb.id, "", "Outside", "---\ntype: note\nsummary: outside\n---\nbody\n")
	asset := f.tm.upload(t, "alice", nb.id, nb.raw, "pending.txt", "source bytes")
	return nb, orphan, asset.ID
}

func (f *lintRuntimeFixture) invariants(t *testing.T) {
	t.Helper()
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
	assertSourceHashesMatchRebuild(t, t.Context(), f.tm.pool)
}

func TestLintReadsBothModesWithRealAuthority(t *testing.T) {
	f := newLintRuntime(t, nil)
	f.tm.join(t, "dana", "admin")
	for _, mode := range []string{"standard", "llm_wiki"} {
		t.Run(mode, func(t *testing.T) {
			nb := f.tm.createWith(t, f.tm.tokens["alice"], `{"name":"`+mode+`","mode":"`+mode+`","workspace_access":"none"}`)
			f.tm.addNotebookMember(t, "alice", nb.ID, "bob", "reader")
			f.tm.addNotebookMember(t, "alice", nb.ID, "carol", "editor")
			f.tm.createPageWith(t, "alice", nb.ID, "", "Emitter", "---\ntype: note\nsummary: ready\n---\n[[truly-wanted]]\n")
			scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.ID)}
			for _, actor := range []string{"alice", "bob", "carol"} {
				result, err := f.module.Execute(f.actor(t, actor), scope, wiki.LintQuery{})
				if err != nil {
					t.Fatalf("real %s member %s was denied: %v", mode, actor, err)
				}
				wanted := lintResultRule(t, result, wiki.RuleWanted)
				wantRules := 4
				if mode == "llm_wiki" {
					wantRules = 8
				}
				if result.Mode != wiki.Mode(mode) || len(result.Rules) != wantRules || wanted.Count.Value != 1 || len(wanted.Items) != 1 || wanted.Items[0].Target != "truly-wanted" {
					t.Fatalf("real %s/%s result=%+v, want %d rules with the independently seeded wanted item", mode, actor, result, wantRules)
				}
				if actor == "alice" {
					if wanted.StartCursor == nil || wanted.Items[0].Cursor == "" {
						t.Fatal("actual summary omitted its dedicated start or item cursor")
					}
					rule, limit := "wanted", 1
					page, err := f.module.Execute(f.actor(t, actor), scope, wiki.LintQuery{Rule: &rule, Limit: &limit, Cursor: wanted.StartCursor})
					if err != nil {
						t.Fatal(err)
					}
					first := lintResultRule(t, page, wiki.RuleWanted)
					if first.Count.Value != 1 || len(first.Items) != 1 || first.Items[0].ItemKey != wanted.Items[0].ItemKey {
						t.Fatalf("actual Decode/PositionFor start cursor lost its item identity: %+v", first)
					}
					end, err := f.module.Execute(f.actor(t, actor), scope, wiki.LintQuery{Rule: &rule, Limit: &limit, Cursor: &first.Items[0].Cursor})
					if err != nil {
						t.Fatal(err)
					}
					last := lintResultRule(t, end, wiki.RuleWanted)
					if last.Count.Value != 1 || len(last.Items) != 0 || last.NextCursor != nil {
						t.Fatalf("actual cursor sorting fields did not terminate: %+v", last)
					}
				}
			}
			_, err := f.module.Execute(f.actor(t, "dana"), scope, wiki.LintQuery{})
			lintErrorCode(t, err, "notebook.not_found")
		})
	}
	f.invariants(t)
}

func TestTheLintSnapshotAndRename(t *testing.T) {
	t.Run("internal", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb, orphan, asset := f.seedPending(t)
		release := f.holdAssets(t)
		done := f.start(f.actor(t, "alice"), wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}, wiki.LintQuery{})
		pid := f.waitAssetLock(t)
		f.tm.send(t, nodeRename("alice", orphan, "Renamed"), http.StatusOK)
		release()
		got := lintReceive(t, done)
		if got.err != nil {
			t.Fatal(got.err)
		}
		orphans := lintResultRule(t, got.result, wiki.RuleOrphans)
		found := false
		for _, item := range orphans.Items {
			if item.PageID != nil && *item.PageID == uuid.MustParse(orphan) {
				found = item.Path != nil && *item.Path == "Outside.md"
			}
		}
		pending := lintResultRule(t, got.result, wiki.RuleUndigestedSources)
		if !found || pending.Count.Value != 1 || len(pending.Items) != 1 || pending.Items[0].NodeID == nil || *pending.Items[0].NodeID != uuid.MustParse(asset) || pending.Items[0].Bytes != 12 {
			t.Fatalf("snapshot lost its old orphan path or real pending asset: orphans=%+v pending=%+v", orphans, pending)
		}
		f.assertSnapshot(t, pid)
		f.invariants(t)
	})
}

func TestTheLintSnapshotAndDowngrade(t *testing.T) {
	t.Run("internal", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb, _, _ := f.seedPending(t)
		release := f.holdAssets(t)
		done := f.start(f.actor(t, "alice"), wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}, wiki.LintQuery{})
		pid := f.waitAssetLock(t)
		if status, body := f.tm.downgrade(t, "alice", nb.id); status != http.StatusOK {
			t.Fatalf("real downgrade=%d %s", status, body)
		}
		release()
		got := lintReceive(t, done)
		if got.err != nil || got.result.Mode != wiki.ModeLLMWiki || len(got.result.Rules) != 8 || lintResultRule(t, got.result, wiki.RuleUndigestedSources).Count.Value != 1 {
			t.Fatalf("in-flight downgrade changed the snapshot's result=%+v error=%v", got.result, got.err)
		}
		f.assertSnapshot(t, pid)
		fresh, err := f.module.Execute(f.actor(t, "alice"), wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}, wiki.LintQuery{})
		if err != nil || fresh.Mode != wiki.ModeStandard || len(fresh.Rules) != 4 {
			t.Fatalf("new request did not see committed downgrade: %+v %v", fresh, err)
		}
		f.invariants(t)
	})
}

func TestLintLogsItsRunNotItsInputs(t *testing.T) {
	for _, name := range []string{"success", "database-error"} {
		t.Run(name, func(t *testing.T) {
			suffix := strings.ReplaceAll(name, "-", "_")
			targetSecret := "private_lint_target_46_" + suffix
			pathSecret := "private_lint_path_46_" + suffix
			jsonSecret := "private_lint_json_46_" + suffix
			errorSecret := "private_lint_error_column_46_" + suffix
			var f *lintRuntimeFixture
			f = newLintRuntime(t, func(d *wiki.Deps) {
				if name == "database-error" {
					d.Rules = lintAfterRules{Rules: d.Rules, after: func(ctx context.Context) error {
						var unused int
						return postgres.DB(ctx, f.pool).QueryRow(ctx, "SELECT "+errorSecret).Scan(&unused)
					}}
				}
			})
			nb, _, _ := f.seedPending(t)
			f.tm.createPageWith(t, "alice", nb.id, "", pathSecret,
				"---\ntype: note\nsummary: ready\nsources: ['"+jsonSecret+"']\n---\n[["+targetSecret+"]]\n")
			offset := len(f.logs.String())
			release := f.holdAssets(t)
			scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
			first := f.start(f.actor(t, "alice"), scope, wiki.LintQuery{})
			f.waitAssetLock(t)
			created := f.event(t, wiki.RunCreated, 0)
			second := f.start(f.actor(t, "bob"), scope, wiki.LintQuery{})
			f.event(t, wiki.RunJoined, created.FlightID)
			release()
			for _, done := range []<-chan lintCallResult{first, second} {
				got := lintReceive(t, done)
				if name == "success" {
					if got.err != nil || lintResultRule(t, got.result, wiki.RuleWanted).Count.Value != 1 || lintResultRule(t, got.result, wiki.RuleInvalidSources).Count.Value != 1 {
						t.Fatalf("real private-data run=%+v %v", got.result, got.err)
					}
				} else if got.err == nil || !strings.Contains(got.err.Error(), errorSecret) || len(got.result.Rules) != 0 {
					t.Fatalf("test did not receive the actual PostgreSQL error with private input: %v", got.err)
				}
			}
			f.event(t, wiki.RunFinished, created.FlightID)
			logs := f.logs.String()[offset:]
			for _, secret := range []string{targetSecret, pathSecret, jsonSecret, errorSecret} {
				if strings.Contains(logs, secret) {
					t.Fatalf("runtime log exposed fixture input %q", secret)
				}
			}
			var runs []map[string]any
			for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
				var entry map[string]any
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatal(err)
				}
				if entry["msg"] == "lint computed" || entry["msg"] == "lint stopped" {
					runs = append(runs, entry)
				}
			}
			if len(runs) != 1 || runs[0]["notebook_id"] != nb.id || runs[0]["mode"] != "llm_wiki" || runs[0]["query_kind"] != "summary" || runs[0]["rule"] != "summary" {
				t.Fatalf("actual two-caller flight logs=%v, want one sanitized owner record", runs)
			}
			if name == "success" {
				rules, ok := runs[0]["rules"].(map[string]any)
				if !ok || len(rules) != 8 || runs[0]["prepare_ms"] == nil || runs[0]["shared_sources_ms"] == nil || runs[0]["stale_pages"] != float64(0) {
					t.Fatalf("real successful preparation lost its observed statistics: %+v", runs[0])
				}
			} else if runs[0]["worker_sqlstate"] != "42703" || runs[0]["rules"] != nil || runs[0]["stale_pages"] != nil {
				t.Fatalf("failed preparation invented statistics or lost actual SQLSTATE: %+v", runs[0])
			}
		})
	}
}

func TestLintTimeoutFollowsTheRequest(t *testing.T) {
	t.Run("remaining-caller", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb, _, _ := f.seedPending(t)
		scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
		release := f.holdAssets(t)
		ctx, cancel := context.WithTimeout(f.actor(t, "alice"), 800*time.Millisecond)
		defer cancel()
		first := f.start(ctx, scope, wiki.LintQuery{})
		f.waitAssetLock(t)
		created := f.event(t, wiki.RunCreated, 0)
		got := lintReceive(t, first)
		lintErrorCode(t, got.err, "server_busy")
		if ctx.Err() != nil {
			t.Fatal("busy waited for the caller deadline instead of its remaining half-budget")
		}
		// The worker has its own full configured budget. A shorter caller
		// expiring must not cancel the computation for a real later follower.
		follower := f.start(f.actor(t, "bob"), scope, wiki.LintQuery{})
		f.event(t, wiki.RunJoined, created.FlightID)
		release()
		got = lintReceive(t, follower)
		if got.err != nil || lintResultRule(t, got.result, wiki.RuleUndigestedSources).Count.Value != 1 {
			t.Fatalf("caller wait expiry cancelled its independent actual worker: %+v %v", got.result, got.err)
		}
		f.event(t, wiki.RunFinished, created.FlightID)
		if f.pool.Stat().AcquiredConns() != 0 {
			t.Fatal("finished worker did not release its actual snapshot")
		}
	})
	t.Run("queued-caller", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb, _, _ := f.seedPending(t)
		f.tm.createPageWith(t, "alice", nb.id, "", "Emitter", "---\ntype: note\nsummary: ready\n---\n[[queued-budget]]\n")
		scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
		release := f.holdAssets(t)
		first := f.start(f.actor(t, "alice"), scope, wiki.LintQuery{})
		f.waitAssetLock(t)
		f.event(t, wiki.RunCreated, 0)
		rule := "wanted"
		ctx, cancel := context.WithTimeout(f.actor(t, "bob"), 800*time.Millisecond)
		defer cancel()
		queued := f.start(ctx, scope, wiki.LintQuery{Rule: &rule})
		created := f.event(t, wiki.RunCreated, 0)
		lintErrorCode(t, lintReceive(t, queued).err, "server_busy")
		if ctx.Err() != nil {
			t.Fatal("waiting for an occupied slot did not consume the caller's half-budget")
		}
		follower := f.start(f.actor(t, "carol"), scope, wiki.LintQuery{Rule: &rule})
		f.event(t, wiki.RunJoined, created.FlightID)
		queries, _ := f.trace.copy()
		for _, query := range queries {
			if query.Snapshot > 1 {
				t.Fatal("queued caller's expired wait had already entered the occupied slot")
			}
		}
		release()
		if got := lintReceive(t, first); got.err != nil {
			t.Fatal(got.err)
		}
		got := lintReceive(t, follower)
		if got.err != nil {
			t.Fatal(got.err)
		}
		wanted := lintResultRule(t, got.result, wiki.RuleWanted)
		if wanted.Count.Value != 1 || len(wanted.Items) != 1 || wanted.Items[0].Target != "queued-budget" {
			t.Fatalf("queued worker did not keep its own remaining budget: %+v", got.result)
		}
	})
	t.Run("queued-worker", func(t *testing.T) {
		ready, release := make(chan struct{}), make(chan struct{})
		var entered atomic.Bool
		defer close(release)
		f := newLintRuntime(t, func(d *wiki.Deps) {
			d.Rules = lintAfterRules{Rules: d.Rules, after: func(ctx context.Context) error {
				if !entered.CompareAndSwap(false, true) {
					return nil
				}
				close(ready)
				<-ctx.Done()
				<-release
				return ctx.Err()
			}}
		})
		nb, _, _ := f.seedPending(t)
		scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
		first := f.start(f.actor(t, "alice"), scope, wiki.LintQuery{})
		select {
		case <-ready:
		case <-time.After(interleavingWait):
			t.Fatal("actual first preparation never occupied its slot")
		}
		f.event(t, wiki.RunCreated, 0)
		rule := "wanted"
		queued := f.start(f.actor(t, "bob"), scope, wiki.LintQuery{Rule: &rule})
		created := f.event(t, wiki.RunCreated, 0)
		request, cancel := context.WithTimeout(t.Context(), 4*time.Second)
		defer cancel()
		lintErrorCode(t, lintReceive(t, first).err, "server_busy")
		lintErrorCode(t, lintReceive(t, queued).err, "server_busy")
		finished := false
		for !finished {
			select {
			case event := <-f.events:
				if event.FlightID == created.FlightID {
					if event.Kind == wiki.RunAcquired {
						t.Fatal("queued worker acquired a slot held by actual transaction cleanup")
					}
					finished = event.Kind == wiki.RunExpired || event.Kind == wiki.RunFinished
				}
			case <-request.Done():
				t.Fatal("worker's configured budget excluded time spent waiting for the slot")
			}
		}
		queries, _ := f.trace.copy()
		for _, query := range queries {
			if query.Snapshot > 1 {
				t.Fatal("expired queued worker performed database work")
			}
		}
	})
}
