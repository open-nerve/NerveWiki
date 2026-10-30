package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestTheLintQueuedModeIsReadAgain(t *testing.T) {
	f := newLintRuntime(t, nil)
	nb, _, _ := f.seedPending(t)
	scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
	release := f.holdAssets(t)
	first := f.start(f.actor(t, "alice"), scope, wiki.LintQuery{})
	f.waitAssetLock(t)
	firstEvent := f.event(t, wiki.RunCreated, 0)
	rule := "missing_summary"
	second := f.start(f.actor(t, "bob"), scope, wiki.LintQuery{Rule: &rule})
	secondEvent := f.event(t, wiki.RunCreated, 0)
	if firstEvent.FlightID == secondEvent.FlightID {
		t.Fatal("different actual queries unexpectedly reused one flight")
	}
	if status, body := f.tm.downgrade(t, "alice", nb.id); status != http.StatusOK {
		t.Fatalf("queued downgrade=%d %s", status, body)
	}
	release()
	if got := lintReceive(t, first); got.err != nil || got.result.Mode != wiki.ModeLLMWiki {
		t.Fatalf("first acquired snapshot=%+v %v, want the original LLM mode", got.result, got.err)
	}
	got := lintReceive(t, second)
	lintErrorCode(t, got.err, "notebook.not_llm_wiki")
	queries, _ := f.trace.copy()
	var secondNames []string
	for _, query := range queries {
		if query.Snapshot == 2 && query.Name != "" {
			secondNames = append(secondNames, query.Name)
		}
	}
	if len(secondNames) != 1 || secondNames[0] != "FindNotebook" {
		t.Fatalf("queued worker SQL=%v, want only its slot-time StateOf before mode409", secondNames)
	}
	f.invariants(t)
}

func TestTheLintFollowersSurviveTheirLeader(t *testing.T) {
	t.Run("rest-and-mcp", lintRESTAndMCPFollowers)
	t.Run("real-Fit-preserves-REST", lintActualFitKeepsREST)
	t.Run("pending-same-flight", lintPendingSharesItsRuleFlight)
	t.Run("internal", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb, _, _ := f.seedPending(t)
		scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
		release := f.holdAssets(t)
		leaderCtx, cancelLeader := context.WithCancel(f.actor(t, "alice"))
		defer cancelLeader()
		leader := f.start(leaderCtx, scope, wiki.LintQuery{})
		f.waitAssetLock(t)
		created := f.event(t, wiki.RunCreated, 0)
		follower := f.start(f.actor(t, "bob"), scope, wiki.LintQuery{})
		f.event(t, wiki.RunJoined, created.FlightID)
		cancelLeader()
		if got := lintReceive(t, leader); !errors.Is(got.err, context.Canceled) {
			t.Fatalf("leader=%v, want its own cancellation", got.err)
		}
		release()
		got := lintReceive(t, follower)
		if got.err != nil || lintResultRule(t, got.result, wiki.RuleUndigestedSources).Count.Value != 1 {
			t.Fatalf("actual authorized follower lost shared worker: %+v %v", got.result, got.err)
		}
		f.event(t, wiki.RunFinished, created.FlightID)
		if got := f.pool.Stat().AcquiredConns(); got != 0 {
			t.Fatalf("finished worker retained %d traced connections", got)
		}
	})
	t.Run("join-before-slot", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb, _, _ := f.seedPending(t)
		f.tm.createPageWith(t, "alice", nb.id, "", "Emitter", "---\ntype: note\nsummary: ready\n---\n[[queued-needed]]\n")
		scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
		release := f.holdAssets(t)
		occupier := f.start(f.actor(t, "alice"), scope, wiki.LintQuery{})
		f.waitAssetLock(t)
		occupied := f.event(t, wiki.RunCreated, 0)
		rule := "wanted"
		leaderCtx, cancelLeader := context.WithCancel(f.actor(t, "alice"))
		defer cancelLeader()
		leader := f.start(leaderCtx, scope, wiki.LintQuery{Rule: &rule})
		queued := f.event(t, wiki.RunCreated, 0)
		if queued.FlightID == occupied.FlightID {
			t.Fatal("queued rule was confused with the slot's different summary query")
		}
		follower := f.start(f.actor(t, "bob"), scope, wiki.LintQuery{Rule: &rule})
		f.event(t, wiki.RunJoined, queued.FlightID)
		other := f.start(f.actor(t, "carol"), scope, wiki.LintQuery{Rule: &rule})
		f.event(t, wiki.RunJoined, queued.FlightID)
		queries, _ := f.trace.copy()
		for _, query := range queries {
			if query.Snapshot > 1 {
				t.Fatal("queued worker started a second snapshot before the single occupied slot returned")
			}
		}
		cancelLeader()
		if got := lintReceive(t, leader); !errors.Is(got.err, context.Canceled) {
			t.Fatalf("queued leader cancellation=%v", got.err)
		}
		release()
		if got := lintReceive(t, occupier); got.err != nil {
			t.Fatal(got.err)
		}
		for _, done := range []<-chan lintCallResult{follower, other} {
			got := lintReceive(t, done)
			if got.err != nil {
				t.Fatal(got.err)
			}
			wanted := lintResultRule(t, got.result, wiki.RuleWanted)
			if wanted.Count.Value != 1 || len(wanted.Items) != 1 || wanted.Items[0].Target != "queued-needed" {
				t.Fatalf("actual queued followers did not share their nonempty result: %+v", got.result)
			}
		}
	})
}

func TestTheLintFollowerCancelsAlone(t *testing.T) {
	t.Run("internal", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb, _, _ := f.seedPending(t)
		scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
		release := f.holdAssets(t)
		leader := f.start(f.actor(t, "alice"), scope, wiki.LintQuery{})
		f.waitAssetLock(t)
		created := f.event(t, wiki.RunCreated, 0)
		ctx, cancel := context.WithCancel(f.actor(t, "bob"))
		defer cancel()
		follower := f.start(ctx, scope, wiki.LintQuery{})
		f.event(t, wiki.RunJoined, created.FlightID)
		other := f.start(f.actor(t, "carol"), scope, wiki.LintQuery{})
		f.event(t, wiki.RunJoined, created.FlightID)
		cancel()
		if got := lintReceive(t, follower); !errors.Is(got.err, context.Canceled) {
			t.Fatalf("cancelled follower=%v, want its own cancellation", got.err)
		}
		release()
		for _, done := range []<-chan lintCallResult{leader, other} {
			got := lintReceive(t, done)
			if got.err != nil || lintResultRule(t, got.result, wiki.RuleUndigestedSources).Count.Value != 1 {
				t.Fatalf("another caller lost the actual worker: %+v %v", got.result, got.err)
			}
		}
		f.event(t, wiki.RunFinished, created.FlightID)
		if got := f.pool.Stat().AcquiredConns(); got != 0 {
			t.Fatalf("one shared worker did not release its connection: %d", got)
		}
	})
}

func TestLintCallersKeepTheirAuthority(t *testing.T) {
	t.Run("postgres", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb, _, _ := f.seedPending(t)
		f.tm.send(t, request("alice", http.MethodPatch, "/api/v0/notebooks/"+nb.id, `{"workspace_access":"none"}`), http.StatusOK)
		membership := f.tm.addNotebookMember(t, "alice", nb.id, "bob", "reader")
		scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
		release := f.holdAssets(t)
		leader := f.start(f.actor(t, "alice"), scope, wiki.LintQuery{})
		f.waitAssetLock(t)
		created := f.event(t, wiki.RunCreated, 0)
		_, err := f.module.Execute(f.actor(t, "carol"), scope, wiki.LintQuery{})
		lintErrorCode(t, err, "notebook.not_found")
		follower := f.start(f.actor(t, "bob"), scope, wiki.LintQuery{})
		f.event(t, wiki.RunJoined, created.FlightID)
		f.tm.send(t, request("alice", http.MethodDelete, "/api/v0/notebook-members/"+membership, ""), http.StatusNoContent)
		release()
		if got := lintReceive(t, leader); got.err != nil || lintResultRule(t, got.result, wiki.RuleUndigestedSources).Count.Value != 1 {
			t.Fatalf("remaining authorized caller=%+v %v", got.result, got.err)
		}
		got := lintReceive(t, follower)
		lintErrorCode(t, got.err, "notebook.not_found")
		if len(got.result.Rules) != 0 {
			t.Fatal("revoked caller received a partial or cached result")
		}
	})
}

// nextScopeEvent waits until a caller has actually been admitted. Waiting for
// either outcome makes a wrong join observable instead of racing the release.
func (f *lintRuntimeFixture) nextScopeEvent(t *testing.T) wiki.RunEvent {
	t.Helper()
	timer := time.NewTimer(interleavingWait)
	defer timer.Stop()
	for {
		select {
		case event := <-f.events:
			if event.Kind == wiki.RunCreated || event.Kind == wiki.RunJoined {
				return event
			}
		case <-timer.C:
			t.Fatal("scope caller neither created nor joined a real flight")
			return wiki.RunEvent{}
		}
	}
}

func TestLintScopesNeverShare(t *testing.T) {
	t.Run("postgres", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		nb := f.tm.openNotebook(t, "alice", "Proposed roots")
		a := f.tm.createPageWith(t, "alice", nb, "", "A", "")
		b := f.tm.createPageWith(t, "alice", nb, "", "B", "")
		assetA := f.tm.upload(t, "alice", nb, a, "a.txt", "only A")
		assetB := f.tm.upload(t, "alice", nb, b, "b.txt", "only B bytes")
		rootA, rootB := uuid.MustParse(a), uuid.MustParse(b)
		llm, standard := wiki.ModeLLMWiki, wiki.ModeStandard
		scopeA := wiki.LintScope{NotebookID: uuid.MustParse(nb), Contract: &llm, AreaRootID: &rootA}
		scopeB := wiki.LintScope{NotebookID: uuid.MustParse(nb), Contract: &llm, AreaRootID: &rootB}
		release := f.holdAssets(t)
		first := f.start(f.actor(t, "alice"), scopeA, wiki.LintQuery{})
		f.waitAssetLock(t)
		created := f.event(t, wiki.RunCreated, 0)
		copyRoot, copyMode := rootA, llm
		same := f.start(f.actor(t, "bob"), wiki.LintScope{NotebookID: scopeA.NotebookID, Contract: &copyMode, AreaRootID: &copyRoot}, wiki.LintQuery{})
		f.event(t, wiki.RunJoined, created.FlightID)
		other := f.start(f.actor(t, "carol"), scopeB, wiki.LintQuery{})
		otherEvent := f.nextScopeEvent(t)
		implicit := f.start(f.actor(t, "alice"), wiki.LintScope{NotebookID: scopeA.NotebookID}, wiki.LintQuery{})
		implicitEvent := f.nextScopeEvent(t)
		explicitStandard := f.start(f.actor(t, "bob"), wiki.LintScope{NotebookID: scopeA.NotebookID, Contract: &standard}, wiki.LintQuery{})
		standardEvent := f.nextScopeEvent(t)
		release()
		for _, done := range []<-chan lintCallResult{first, same} {
			got := lintReceive(t, done)
			if got.err != nil {
				t.Fatal(got.err)
			}
			pending := lintResultRule(t, got.result, wiki.RuleUndigestedSources)
			if got.result.Mode != wiki.ModeLLMWiki || len(pending.Items) != 1 || pending.Items[0].NodeID == nil || *pending.Items[0].NodeID != uuid.MustParse(assetA.ID) || pending.Items[0].Bytes != 6 {
				t.Fatalf("root A result=%+v, want only its actual asset", got.result)
			}
		}
		got := lintReceive(t, other)
		if got.err != nil {
			t.Fatal(got.err)
		}
		pending := lintResultRule(t, got.result, wiki.RuleUndigestedSources)
		if otherEvent.Kind != wiki.RunCreated || otherEvent.FlightID == created.FlightID || len(pending.Items) != 1 || pending.Items[0].NodeID == nil || *pending.Items[0].NodeID != uuid.MustParse(assetB.ID) || pending.Items[0].Bytes != 12 {
			t.Fatalf("root B shared root A's computation: event=%+v result=%+v", otherEvent, got.result)
		}
		if implicitEvent.Kind != wiki.RunCreated || standardEvent.Kind != wiki.RunCreated || implicitEvent.FlightID == standardEvent.FlightID {
			t.Fatalf("implicit and explicit standard did not keep their scope identities: %+v %+v", implicitEvent, standardEvent)
		}
		for _, done := range []<-chan lintCallResult{implicit, explicitStandard} {
			got := lintReceive(t, done)
			if got.err != nil || got.result.Mode != wiki.ModeStandard || len(got.result.Rules) != 4 {
				t.Fatalf("standard scope inherited LLM preparation: %+v %v", got.result, got.err)
			}
		}
	})
}

type lintAfterRules struct {
	wiki.Rules
	after func(context.Context) error
}

func (r lintAfterRules) PrepareRules(ctx context.Context, input wiki.ReadInput, query wiki.RuleQuery) (wiki.PreparedRules, error) {
	prepared, err := r.Rules.PrepareRules(ctx, input, query)
	if err != nil {
		return nil, err
	}
	if err := r.after(ctx); err != nil {
		return nil, err
	}
	return prepared, nil
}

func TestTheLintWorkerRecoversThroughModule(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: needs Docker")
	}
	if os.Getenv("NERVEWIKI_LINT_PANIC_CHILD") != "1" {
		const name = "TestTheLintWorkerRecoversThroughModule"
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+name+"$", "-test.v", "-test.count=1", "-test.timeout=15m")
		cmd.Env = append(os.Environ(), "NERVEWIKI_LINT_PANIC_CHILD=1")
		output, err := cmd.CombinedOutput()
		t.Logf("actual module child output:\n%s", output)
		if !strings.Contains(string(output), "=== RUN   "+name) {
			t.Fatalf("module child did not actually enter its designated catcher: %v", err)
		}
		if err != nil || !strings.Contains(string(output), "--- PASS: "+name) {
			t.Fatalf("actual module child failed; expected its recovered-panic assertions and PASS (child error=%v)", err)
		}
		return
	}
	const secret = "panic-only-private-lint-target"
	entered, release := make(chan struct{}), make(chan struct{})
	var once atomic.Bool
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	f := newLintRuntime(t, func(d *wiki.Deps) {
		d.Rules = lintAfterRules{Rules: d.Rules, after: func(ctx context.Context) error {
			if !once.CompareAndSwap(false, true) {
				return nil
			}
			close(entered)
			<-release
			panic(secret)
		}}
	})
	nb, _, _ := f.seedPending(t)
	scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
	first := f.start(f.actor(t, "alice"), scope, wiki.LintQuery{})
	select {
	case <-entered:
	case <-time.After(interleavingWait):
		t.Fatal("real Rules.Prepare never reached the panic barrier")
	}
	created := f.event(t, wiki.RunCreated, 0)
	second := f.start(f.actor(t, "bob"), scope, wiki.LintQuery{})
	f.event(t, wiki.RunJoined, created.FlightID)
	// The real preparation is still inside its callback and transaction.
	// A third caller must exhaust only its private wait; publishing the
	// eventual internal error early must not look like completed cleanup.
	probeCtx, cancelProbe := context.WithTimeout(f.actor(t, "carol"), 300*time.Millisecond)
	defer cancelProbe()
	probe := f.start(probeCtx, scope, wiki.LintQuery{})
	f.event(t, wiki.RunJoined, created.FlightID)
	probeResult := lintReceive(t, probe)
	var busy *shared.Error
	if !errors.As(probeResult.err, &busy) || busy.Code != "server_busy" || len(probeResult.result.Rules) != 0 {
		t.Fatalf("panic flight delivered before its actual callback and transaction cleanup: %+v %v", probeResult.result, probeResult.err)
	}
	if probeCtx.Err() != nil || f.pool.Stat().AcquiredConns() != 1 {
		t.Fatal("panic cleanup probe did not return on its private half-budget while the actual snapshot remained acquired")
	}
	releaseOnce.Do(func() { close(release) })
	for _, done := range []<-chan lintCallResult{first, second} {
		got := lintReceive(t, done)
		if got.err == nil || got.err.Error() != "internal failure" || len(got.result.Rules) != 0 {
			t.Fatalf("panic result=%+v error=%v, want fixed internal and no partial answer", got.result, got.err)
		}
	}
	f.event(t, wiki.RunFinished, created.FlightID)
	if f.pool.Stat().AcquiredConns() != 0 {
		t.Fatal("panic cleanup did not release the actual traced connection")
	}
	result, err := f.module.Execute(f.actor(t, "carol"), scope, wiki.LintQuery{})
	if err != nil || lintResultRule(t, result, wiki.RuleUndigestedSources).Count.Value != 1 {
		t.Fatalf("same-key real read after panic=%+v %v", result, err)
	}
	if strings.Contains(f.logs.String(), secret) {
		t.Fatal("runtime log exposed the original panic value")
	}
	// The platform deliberately discards a panic rollback's error. Success of
	// this subsequent real query proves usable connections, not a rollback code.
	if n := count(t, f.pool, "SELECT 1"); n != 1 {
		t.Fatal("real query after panic did not succeed")
	}
}

func TestLintBusyReturnsBeforeCleanup(t *testing.T) {
	t.Run("postgres", func(t *testing.T) {
		unwinding, release := make(chan struct{}), make(chan struct{})
		nextPrepared, releaseNext := make(chan struct{}), make(chan struct{})
		var calls atomic.Uint64
		var releaseOnce sync.Once
		var releaseNextOnce sync.Once
		defer releaseOnce.Do(func() { close(release) })
		defer releaseNextOnce.Do(func() { close(releaseNext) })
		var f *lintRuntimeFixture
		var cleanup *lintCleanupOrder
		f = newLintRuntime(t, func(d *wiki.Deps) {
			cleanup = &lintCleanupOrder{Snapshots: d.Snapshots}
			d.Snapshots = cleanup
			d.Rules = lintAfterRules{Rules: d.Rules, after: func(ctx context.Context) error {
				switch calls.Add(1) {
				case 2:
					close(nextPrepared)
					<-releaseNext
					return nil
				case 1:
				default:
					return nil
				}
				rows, err := postgres.DB(ctx, f.pool).Query(ctx, "SELECT pg_sleep(30)")
				if err == nil {
					for rows.Next() {
					}
					err = rows.Err()
					rows.Close()
				}
				close(unwinding)
				// The actual cancelled query has unwound its Rows. Hold the
				// callback before the original TxManager's cleanup and Release.
				<-release
				return err
			}}
		})
		nb, _, _ := f.seedPending(t)
		scope := wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}
		first := f.start(f.actor(t, "alice"), scope, wiki.LintQuery{})
		created := f.event(t, wiki.RunCreated, 0)
		f.waitSleep(t)
		got := lintReceive(t, first)
		lintErrorCode(t, got.err, "server_busy")
		select {
		case <-unwinding:
		case <-time.After(interleavingWait):
			t.Fatal("expired real pg_sleep did not cancel and unwind its Rows")
		}
		if f.pool.Stat().AcquiredConns() != 1 {
			t.Fatalf("busy was delivered after actual cleanup/release: acquired=%d", f.pool.Stat().AcquiredConns())
		}
		next := f.start(f.actor(t, "bob"), scope, wiki.LintQuery{})
		nextEvent := f.nextScopeEvent(t)
		if nextEvent.Kind != wiki.RunCreated || nextEvent.FlightID == created.FlightID {
			t.Fatalf("new same-key caller joined an expired worker: %+v", nextEvent)
		}
		probeCtx, cancelProbe := context.WithTimeout(f.actor(t, "carol"), 300*time.Millisecond)
		defer cancelProbe()
		probe := f.start(probeCtx, scope, wiki.LintQuery{})
		f.event(t, wiki.RunJoined, nextEvent.FlightID)
		lintErrorCode(t, lintReceive(t, probe).err, "server_busy")
		if cleanup.early.Load() || cleanup.firstDone.Load() || f.pool.Stat().AcquiredConns() != 1 {
			t.Fatal("short follower waited while a new snapshot entered before old cleanup; slot was released early")
		}
		releaseOnce.Do(func() { close(release) })
		select {
		case <-nextPrepared:
		case <-time.After(interleavingWait):
			t.Fatal("new same-key flight did not execute its actual Rules.Prepare")
		}
		last := f.start(f.actor(t, "carol"), scope, wiki.LintQuery{})
		lastEvent := f.nextScopeEvent(t)
		releaseNextOnce.Do(func() { close(releaseNext) })
		if lastEvent.Kind != wiki.RunJoined || lastEvent.FlightID != nextEvent.FlightID {
			t.Fatalf("old cleanup deleted the new flight before its real follower joined: %+v", lastEvent)
		}
		got = lintReceive(t, next)
		if got.err != nil || lintResultRule(t, got.result, wiki.RuleUndigestedSources).Count.Value != 1 {
			t.Fatalf("real read after expired worker cleanup=%+v %v", got.result, got.err)
		}
		if got := lintReceive(t, last); got.err != nil || lintResultRule(t, got.result, wiki.RuleUndigestedSources).Count.Value != 1 {
			t.Fatalf("new flight's actual follower=%+v %v", got.result, got.err)
		}
		f.event(t, wiki.RunFinished, nextEvent.FlightID)
		if cleanup.early.Load() || !cleanup.firstDone.Load() {
			t.Fatal("a new snapshot entered before the old worker's real transaction cleanup and connection Release")
		}
		if n := count(t, f.pool, "SELECT 1"); n != 1 || f.pool.Stat().AcquiredConns() != 0 {
			t.Fatal("real pool was unusable or retained a connection after cleanup")
		}
	})
}

// This observer never replaces a read or changes the result. It records entry
// into the second actual snapshot relative to return of the first one, whose
// original TxManager and traced-connection Release have both completed.
type lintCleanupOrder struct {
	shared.Snapshots
	calls     atomic.Uint64
	firstDone atomic.Bool
	early     atomic.Bool
}

func (s *lintCleanupOrder) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	if s.calls.Add(1) == 1 {
		defer s.firstDone.Store(true)
	} else if !s.firstDone.Load() {
		s.early.Store(true)
	}
	return s.Snapshots.WithinSnapshot(ctx, fn)
}

func (f *lintRuntimeFixture) waitSleep(t *testing.T) {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(interleavingWait)
	defer deadline.Stop()
	for {
		queries, _ := f.trace.copy()
		for _, query := range queries {
			if query.SQL != "SELECT pg_sleep(30)" {
				continue
			}
			var sleeping bool
			if err := f.tm.pool.QueryRow(t.Context(), `SELECT EXISTS (SELECT 1 FROM pg_stat_activity
				WHERE pid=$1 AND query='SELECT pg_sleep(30)' AND state='active' AND wait_event='PgSleep')`, query.PID).Scan(&sleeping); err != nil {
				t.Fatal(err)
			}
			if sleeping {
				t.Logf("actual lint backend=%d is executing pg_sleep before worker expiry", query.PID)
				return
			}
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("worker never entered the actual PostgreSQL slow query")
			return
		}
	}
}
