package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type lintRunAnswer struct {
	result domain.LintResult
	err    error
}

type lintRunLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lintRunLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lintRunLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

type lintRunEvents struct {
	c       chan RunEvent
	pending []RunEvent
}

func (e *lintRunEvents) take(t *testing.T, kind RunEventKind, id uint64) RunEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	for {
		for i, event := range e.pending {
			if event.Kind == kind && (id == 0 || event.FlightID == id) {
				e.pending = append(e.pending[:i], e.pending[i+1:]...)
				return event
			}
		}
		select {
		case event := <-e.c:
			e.pending = append(e.pending, event)
		case <-ctx.Done():
			t.Fatalf("missing run event %d for flight %d; pending %v", kind, id, e.pending)
		}
	}
}

func lintRunFixture(t *testing.T, max int, timeout time.Duration) (*Runs, *lintRunEvents, *lintRunLog) {
	t.Helper()
	events := &lintRunEvents{c: make(chan RunEvent, 128)}
	log := &lintRunLog{}
	runs := NewRuns(max, timeout, slog.New(slog.NewJSONHandler(log, nil)), func(event RunEvent) { events.c <- event })
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := runs.Stop(ctx); err != nil {
			t.Errorf("fixture owner did not drain: %v", err)
		}
	})
	return runs, events, log
}

func lintRunKey() RunKey {
	return RunKey{NotebookID: uuid.MustParse("11111111-1111-4111-8111-111111111111"), Kind: domain.LintItems, Rule: domain.RuleWanted, Limit: 50}
}

func lintRunAsync(r *Runs, ctx context.Context, key RunKey, run func(context.Context) (RunOutput, error)) <-chan lintRunAnswer {
	done := make(chan lintRunAnswer, 1)
	go func() {
		result, err := r.Do(ctx, key, run)
		done <- lintRunAnswer{result, err}
	}()
	return done
}

func lintRunTake(t *testing.T, done <-chan lintRunAnswer) lintRunAnswer {
	t.Helper()
	select {
	case answer := <-done:
		return answer
	case <-time.After(2 * time.Second):
		t.Fatal("caller did not return within the test's bound")
		return lintRunAnswer{}
	}
}

func lintRunWait(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("did not observe %s", what)
	}
}

func lintRunBusy(t *testing.T, err error) {
	t.Helper()
	var problem *shared.Error
	if !errors.As(err, &problem) || problem.Kind != shared.KindUnavailable || problem.Code != "server_busy" || problem.RetryDelay != time.Second {
		t.Fatalf("want busy with one-second retry, got %v", err)
	}
}

func TestLintFollowersSurviveTheirLeader(t *testing.T) {
	t.Run("join-before-slot", func(t *testing.T) {
		r, events, _ := lintRunFixture(t, 1, time.Second)
		hold := make(chan struct{})
		defer close(hold)
		blockKey := lintRunKey()
		blockKey.Rule = domain.RuleOrphans
		block := lintRunAsync(r, t.Context(), blockKey, func(context.Context) (RunOutput, error) { <-hold; return RunOutput{}, nil })
		events.take(t, RunAcquired, 0)
		leaderCtx, cancelLeader := context.WithCancel(t.Context())
		defer cancelLeader()
		var calls atomic.Int32
		work := func(ctx context.Context) (RunOutput, error) {
			calls.Add(1)
			return RunOutput{Result: domain.LintResult{Mode: domain.ModeStandard}}, ctx.Err()
		}
		leader := lintRunAsync(r, leaderCtx, lintRunKey(), work)
		created := events.take(t, RunCreated, 0)
		if created.NotebookID != lintRunKey().NotebookID {
			t.Fatal("created event names another notebook")
		}
		// Discard the blocker's creation if it preceded the acquired event.
		if created.FlightID == 1 {
			created = events.take(t, RunCreated, 0)
		}
		follower := lintRunAsync(r, t.Context(), lintRunKey(), work)
		third := lintRunAsync(r, t.Context(), lintRunKey(), work)
		events.take(t, RunJoined, created.FlightID)
		events.take(t, RunJoined, created.FlightID)
		cancelLeader()
		if answer := lintRunTake(t, leader); !errors.Is(answer.err, context.Canceled) {
			t.Fatalf("leader cancel = %v", answer.err)
		}
		// The occupied slot is released only after all three callers joined.
		hold <- struct{}{}
		if answer := lintRunTake(t, block); answer.err != nil {
			t.Fatal(answer.err)
		}
		for _, done := range []<-chan lintRunAnswer{follower, third} {
			answer := lintRunTake(t, done)
			if answer.err != nil || answer.result.Mode != domain.ModeStandard {
				t.Fatalf("surviving follower = %+v", answer)
			}
		}
		if calls.Load() != 1 {
			t.Fatalf("same flight ran %d callbacks, want one", calls.Load())
		}
	})
}

func TestLintFollowerCancelsAlone(t *testing.T) {
	r, events, _ := lintRunFixture(t, 1, time.Hour)
	release := make(chan struct{})
	defer close(release)
	workerCtx := make(chan context.Context, 1)
	work := func(ctx context.Context) (RunOutput, error) {
		workerCtx <- ctx
		<-release
		return RunOutput{}, ctx.Err()
	}
	leader := lintRunAsync(r, t.Context(), lintRunKey(), work)
	flight := events.take(t, RunAcquired, 0)
	fctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	follower := lintRunAsync(r, fctx, lintRunKey(), work)
	other := lintRunAsync(r, t.Context(), lintRunKey(), work)
	events.take(t, RunJoined, flight.FlightID)
	events.take(t, RunJoined, flight.FlightID)
	cancel()
	if answer := lintRunTake(t, follower); !errors.Is(answer.err, context.Canceled) {
		t.Fatalf("follower cancellation = %v", answer.err)
	}
	if ctx := <-workerCtx; ctx.Err() != nil {
		t.Fatalf("follower canceled worker: %v", ctx.Err())
	}
	select {
	case <-leader:
		t.Fatal("leader finished while its worker was still blocked")
	case <-other:
		t.Fatal("other follower finished while its worker was still blocked")
	default:
	}
	release <- struct{}{}
	for _, done := range []<-chan lintRunAnswer{leader, other} {
		if answer := lintRunTake(t, done); answer.err != nil {
			t.Fatal(answer.err)
		}
	}
}

func TestLintFollowersOwnTheirProjection(t *testing.T) {
	r, events, _ := lintRunFixture(t, 1, time.Second)
	release := make(chan struct{})
	defer close(release)
	work := func(context.Context) (RunOutput, error) {
		<-release
		id := uuid.UUID{}
		path, next, start := "original", "after", "begin"
		return RunOutput{Result: domain.LintResult{Rules: []domain.LintRuleResult{{
			Items: []domain.LintItem{{PageID: &id, NodeID: &id, Path: &path}}, NextCursor: &next, StartCursor: &start,
		}}}}, nil
	}
	a := lintRunAsync(r, t.Context(), lintRunKey(), work)
	flight := events.take(t, RunAcquired, 0)
	b := lintRunAsync(r, t.Context(), lintRunKey(), work)
	events.take(t, RunJoined, flight.FlightID)
	release <- struct{}{}
	x, y := lintRunTake(t, a), lintRunTake(t, b)
	if x.err != nil || y.err != nil {
		t.Fatalf("followers failed: %v / %v", x.err, y.err)
	}
	*x.result.Rules[0].Items[0].Path = "cut"
	*x.result.Rules[0].Items[0].PageID = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	*x.result.Rules[0].NextCursor = "cut"
	*x.result.Rules[0].StartCursor = "cut"
	x.result.Rules[0].Items = nil
	if len(y.result.Rules) != 1 || len(y.result.Rules[0].Items) != 1 {
		t.Fatal("one caller's Fit-like projection changed another caller's result")
	}
	if *y.result.Rules[0].Items[0].Path != "original" || *y.result.Rules[0].Items[0].PageID != (uuid.UUID{}) ||
		*y.result.Rules[0].NextCursor != "after" || *y.result.Rules[0].StartCursor != "begin" {
		t.Fatal("one caller's Fit-like projection changed another caller's result")
	}
}

func TestLintRunsStop(t *testing.T) {
	t.Run("running-and-queued", func(t *testing.T) {
		r, events, _ := lintRunFixture(t, 1, time.Hour)
		clean := make(chan struct{})
		defer close(clean)
		canceled := make(chan struct{})
		running := lintRunAsync(r, t.Context(), lintRunKey(), func(ctx context.Context) (RunOutput, error) {
			<-ctx.Done()
			close(canceled)
			<-clean
			return RunOutput{}, ctx.Err()
		})
		flight := events.take(t, RunAcquired, 0)
		events.take(t, RunCreated, flight.FlightID)
		other := lintRunKey()
		other.Rule = domain.RuleOrphans
		var queuedCalls atomic.Int32
		queued := lintRunAsync(r, t.Context(), other, func(context.Context) (RunOutput, error) { queuedCalls.Add(1); return RunOutput{}, nil })
		queuedFlight := events.take(t, RunCreated, 0)
		stopCtx, cancel := context.WithCancel(t.Context())
		stop := make(chan error, 1)
		go func() { stop <- r.Stop(stopCtx) }()
		lintRunWait(t, canceled, "running worker cancellation before its one-hour deadline")
		lintRunBusy(t, lintRunTake(t, running).err)
		lintRunBusy(t, lintRunTake(t, queued).err)
		events.take(t, RunFinished, queuedFlight.FlightID)
		cancel()
		if err := <-stop; !errors.Is(err, context.Canceled) {
			t.Fatalf("Stop returned before running cleanup: %v", err)
		}
		closedCtx, cancelClosed := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancelClosed()
		_, err := r.Do(closedCtx, lintRunKey(), func(context.Context) (RunOutput, error) { t.Error("closed owner ran work"); return RunOutput{}, nil })
		lintRunBusy(t, err)
		clean <- struct{}{}
		if err := r.Stop(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := r.Stop(t.Context()); err != nil || queuedCalls.Load() != 0 {
			t.Fatalf("repeat Stop = %v, queued callbacks = %d", err, queuedCalls.Load())
		}
	})
	t.Run("queued-canceled-before-deadline", func(t *testing.T) {
		r, events, _ := lintRunFixture(t, 1, 100*time.Millisecond)
		clean := make(chan struct{})
		defer close(clean)
		first := lintRunAsync(r, t.Context(), lintRunKey(), func(ctx context.Context) (RunOutput, error) {
			<-ctx.Done()
			<-clean
			return RunOutput{}, ctx.Err()
		})
		old := events.take(t, RunAcquired, 0)
		events.take(t, RunCreated, old.FlightID)
		events.take(t, RunExpired, old.FlightID)
		lintRunBusy(t, lintRunTake(t, first).err)
		key := lintRunKey()
		key.Rule = domain.RuleOrphans
		second := lintRunAsync(r, t.Context(), key, func(context.Context) (RunOutput, error) {
			t.Error("Stop let queued worker enter the callback")
			return RunOutput{}, nil
		})
		fresh := events.take(t, RunCreated, 0)
		r.mu.Lock()
		queuedCtx := r.flights[key].ctx
		r.mu.Unlock()
		stopCtx, cancelStop := context.WithCancel(t.Context())
		defer cancelStop()
		stopped := make(chan error, 1)
		go func() { stopped <- r.Stop(stopCtx) }()
		lintRunWait(t, queuedCtx.Done(), "Stop canceling a queued worker")
		if !errors.Is(queuedCtx.Err(), context.Canceled) {
			t.Fatalf("queued worker reached its own deadline instead of Stop cancellation: %v", queuedCtx.Err())
		}
		events.take(t, RunFinished, fresh.FlightID)
		lintRunBusy(t, lintRunTake(t, second).err)
		cancelStop()
		if err := <-stopped; !errors.Is(err, context.Canceled) {
			t.Fatalf("Stop omitted expired cleanup while canceling the queue: %v", err)
		}
		clean <- struct{}{}
	})
	t.Run("expired-cleanup-and-concurrent-stop", func(t *testing.T) {
		r, events, _ := lintRunFixture(t, 1, 50*time.Millisecond)
		clean := make(chan struct{})
		defer close(clean)
		done := lintRunAsync(r, t.Context(), lintRunKey(), func(ctx context.Context) (RunOutput, error) {
			<-ctx.Done()
			<-clean
			return RunOutput{}, ctx.Err()
		})
		flight := events.take(t, RunAcquired, 0)
		events.take(t, RunExpired, flight.FlightID)
		lintRunBusy(t, lintRunTake(t, done).err)
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		if err := r.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Stop omitted expired worker still in cleanup: %v", err)
		}
		stops := make(chan error, 2)
		for range 2 {
			go func() { stops <- r.Stop(t.Context()) }()
		}
		clean <- struct{}{}
		for range 2 {
			if err := <-stops; err != nil {
				t.Fatalf("concurrent Stop after cleanup: %v", err)
			}
		}
	})
}

func TestLintBusyReturnsBeforeCleanup(t *testing.T) {
	t.Run("slot-stays-owned", func(t *testing.T) {
		r, events, _ := lintRunFixture(t, 1, 100*time.Millisecond)
		clean := make(chan struct{})
		defer close(clean)
		first := lintRunAsync(r, t.Context(), lintRunKey(), func(ctx context.Context) (RunOutput, error) {
			<-ctx.Done()
			<-clean
			return RunOutput{}, ctx.Err()
		})
		flight := events.take(t, RunAcquired, 0)
		events.take(t, RunCreated, flight.FlightID)
		events.take(t, RunExpired, flight.FlightID)
		lintRunBusy(t, lintRunTake(t, first).err)
		var ran atomic.Bool
		key := lintRunKey()
		key.Rule = domain.RuleOrphans
		second := lintRunAsync(r, t.Context(), key, func(context.Context) (RunOutput, error) { ran.Store(true); return RunOutput{}, nil })
		queued := events.take(t, RunCreated, 0)
		lintRunBusy(t, lintRunTake(t, second).err)
		events.take(t, RunFinished, queued.FlightID)
		if ran.Load() {
			t.Fatal("another worker acquired the slot before the expired snapshot finished cleanup")
		}
		clean <- struct{}{}
		events.take(t, RunFinished, flight.FlightID)
	})
	t.Run("old-worker-keeps-new-flight", func(t *testing.T) {
		r, events, _ := lintRunFixture(t, 1, 200*time.Millisecond)
		oldClean, newClean := make(chan struct{}), make(chan struct{})
		defer close(oldClean)
		defer close(newClean)
		first := lintRunAsync(r, t.Context(), lintRunKey(), func(ctx context.Context) (RunOutput, error) {
			<-ctx.Done()
			<-oldClean
			return RunOutput{}, ctx.Err()
		})
		old := events.take(t, RunAcquired, 0)
		events.take(t, RunCreated, old.FlightID)
		events.take(t, RunExpired, old.FlightID)
		lintRunBusy(t, lintRunTake(t, first).err)
		var calls atomic.Int32
		work := func(context.Context) (RunOutput, error) {
			calls.Add(1)
			<-newClean
			return RunOutput{Result: domain.LintResult{Mode: domain.ModeLLMWiki}}, nil
		}
		second := lintRunAsync(r, t.Context(), lintRunKey(), work)
		fresh := events.take(t, RunCreated, 0)
		if fresh.FlightID == old.FlightID {
			t.Fatal("new caller joined an expired flight")
		}
		oldClean <- struct{}{}
		events.take(t, RunFinished, old.FlightID)
		events.take(t, RunAcquired, fresh.FlightID)
		third := lintRunAsync(r, t.Context(), lintRunKey(), work)
		events.take(t, RunJoined, fresh.FlightID)
		newClean <- struct{}{}
		for _, done := range []<-chan lintRunAnswer{second, third} {
			answer := lintRunTake(t, done)
			if answer.err != nil || answer.result.Mode != domain.ModeLLMWiki {
				t.Fatalf("fresh caller = %+v", answer)
			}
		}
		if calls.Load() != 1 {
			t.Fatalf("old cleanup lost the accepting replacement: %d callbacks", calls.Load())
		}
	})
}

func TestLintTimeoutFollowsTheRequest(t *testing.T) {
	t.Run("caller-budget", func(t *testing.T) {
		r, events, _ := lintRunFixture(t, 1, time.Second)
		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()
		clean := make(chan struct{})
		defer close(clean)
		done := lintRunAsync(r, ctx, lintRunKey(), func(context.Context) (RunOutput, error) { <-clean; return RunOutput{}, nil })
		events.take(t, RunAcquired, 0)
		lintRunBusy(t, lintRunTake(t, done).err)
		if ctx.Err() != nil {
			t.Fatal("caller wait consumed the entire request deadline instead of half")
		}
		clean <- struct{}{}
	})
	t.Run("configured-worker-bound", func(t *testing.T) {
		r, _, _ := lintRunFixture(t, 1, 100*time.Millisecond)
		_, err := r.Do(t.Context(), lintRunKey(), func(ctx context.Context) (RunOutput, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 100*time.Millisecond {
				t.Error("worker ignored the configured short request-derived timeout")
			}
			return RunOutput{}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("already-canceled", func(t *testing.T) {
		r, _, _ := lintRunFixture(t, 1, time.Second)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := r.Do(ctx, lintRunKey(), func(context.Context) (RunOutput, error) {
			t.Error("canceled caller created work")
			return RunOutput{}, nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("already canceled = %v", err)
		}
	})
}

func TestLintWorkerRecovers(t *testing.T) {
	for _, tc := range []struct {
		name         string
		cleanupFault bool
		expire       bool
	}{{name: "panic-cleanup"}, {name: "panic-and-cleanup-fault", cleanupFault: true}, {name: "panic-and-expiry", expire: true}} {
		t.Run(tc.name, func(t *testing.T) {
			bound := time.Hour
			if tc.expire {
				bound = 100 * time.Millisecond
			}
			r, events, log := lintRunFixture(t, 1, bound)
			trigger, cleaning, cleaned := make(chan struct{}), make(chan struct{}), make(chan struct{})
			cleanup := make(chan struct{})
			defer close(cleanup)
			secret := "panic-private-" + tc.name
			work := func(context.Context) (RunOutput, error) {
				defer func() {
					close(cleaning)
					<-cleanup
					close(cleaned)
					if tc.cleanupFault {
						panic("cleanup-private-" + tc.name)
					}
				}()
				<-trigger
				panic(secret)
			}
			leader := lintRunAsync(r, t.Context(), lintRunKey(), work)
			flight := events.take(t, RunAcquired, 0)
			follower := lintRunAsync(r, t.Context(), lintRunKey(), work)
			events.take(t, RunJoined, flight.FlightID)
			close(trigger)
			lintRunWait(t, cleaning, "callback's real defer entering cleanup")
			if tc.expire {
				events.take(t, RunExpired, flight.FlightID)
				lintRunBusy(t, lintRunTake(t, leader).err)
				lintRunBusy(t, lintRunTake(t, follower).err)
			} else {
				// The callback is still in its own defer. A published result here
				// would let a caller escape before snapshot cleanup returned.
				r.mu.Lock()
				published := r.flights[lintRunKey()] == nil || r.flights[lintRunKey()].published
				r.mu.Unlock()
				if published {
					t.Fatal("panic result was published before callback cleanup returned")
				}
			}
			cleanup <- struct{}{}
			lintRunWait(t, cleaned, "completed callback cleanup")
			if !tc.expire {
				for _, done := range []<-chan lintRunAnswer{leader, follower} {
					answer := lintRunTake(t, done)
					if answer.err == nil || answer.err.Error() != "internal failure" || answer.result.Rules != nil {
						t.Fatalf("panic did not yield a fixed internal failure without partial data: %+v", answer)
					}
				}
			}
			events.take(t, RunFinished, flight.FlightID)
			before := log.String()
			if strings.Contains(before, secret) || strings.Contains(before, "cleanup-private-") ||
				strings.Contains(before, "worker_sqlstate") || strings.Contains(before, "\"count\"") ||
				strings.Count(before, "\"msg\":\"lint stopped\"") != 1 || !strings.Contains(before, "\"fault\":\"panic\"") {
				t.Fatalf("panic log leaked data or invented observations: %s", before)
			}
			if tc.expire && !strings.Contains(before, "\"outcome\":\"expired\"") {
				t.Fatalf("late panic overwrote expiry: %s", before)
			}
			result, err := r.Do(t.Context(), lintRunKey(), func(context.Context) (RunOutput, error) {
				return RunOutput{Result: domain.LintResult{Mode: domain.ModeStandard}}, nil
			})
			if err != nil || result.Mode != domain.ModeStandard {
				t.Fatalf("owner did not recover its slot/key after panic: %+v, %v", result, err)
			}
		})
	}
}

type lintTestDatabaseError struct{ code string }

func (lintTestDatabaseError) Error() string      { return "private query argument" }
func (e lintTestDatabaseError) SQLState() string { return e.code }

func TestLintWorkerErrorsStayErrors(t *testing.T) {
	for _, code := range []string{"XX000", "bad-private-code"} {
		t.Run(code, func(t *testing.T) {
			r, events, log := lintRunFixture(t, 1, time.Second)
			want := lintTestDatabaseError{code: code}
			result, err := r.Do(t.Context(), lintRunKey(), func(context.Context) (RunOutput, error) {
				return RunOutput{Result: domain.LintResult{Mode: domain.ModeLLMWiki, Rules: []domain.LintRuleResult{{Rule: domain.RuleWanted}}}}, want
			})
			if !errors.Is(err, want) || result.Mode != "" || result.Rules != nil {
				t.Fatalf("ordinary error became busy/success or exposed partial data: %+v, %v", result, err)
			}
			events.take(t, RunFinished, 0)
			line := log.String()
			if strings.Contains(line, "private query argument") || strings.Contains(line, "bad-private-code") || strings.Contains(line, "rollback") ||
				strings.Contains(line, "\"count\"") || strings.Contains(line, "\"stale_pages\"") || !strings.Contains(line, "\"mode\":\"unknown\"") {
				t.Fatalf("error log exposed raw data or guessed statistics/cleanup provenance: %s", line)
			}
			if (code == "XX000") != strings.Contains(line, "\"worker_sqlstate\":\"XX000\"") {
				t.Fatalf("safe worker SQLSTATE observation = %s", line)
			}
			if _, err := r.Do(t.Context(), lintRunKey(), func(context.Context) (RunOutput, error) { return RunOutput{}, nil }); err != nil {
				t.Fatalf("ordinary failure leaked slot: %v", err)
			}
		})
	}
}

func TestLintImplicitFlightIgnoresPrecheckedMode(t *testing.T) {
	r, events, _ := lintRunFixture(t, 1, time.Second)
	release := make(chan struct{})
	defer close(release)
	work := func(context.Context) (RunOutput, error) {
		<-release
		return RunOutput{Result: domain.LintResult{Mode: domain.ModeLLMWiki}}, nil
	}
	key := lintRunKey()
	key.Contract = domain.ModeStandard
	a := lintRunAsync(r, t.Context(), key, work)
	flight := events.take(t, RunAcquired, 0)
	key.Contract, key.HasRoot = domain.ModeLLMWiki, true
	key.RootID = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	b := lintRunAsync(r, t.Context(), key, work)
	events.take(t, RunJoined, flight.FlightID)
	release <- struct{}{}
	for _, done := range []<-chan lintRunAnswer{a, b} {
		answer := lintRunTake(t, done)
		if answer.err != nil || answer.result.Mode != domain.ModeLLMWiki {
			t.Fatalf("implicit prechecked values bound shared computation: %+v", answer)
		}
	}
}

func TestLintExpiryIsObservedOnce(t *testing.T) {
	for _, winner := range []string{"finish-wins", "afterfunc-wins"} {
		t.Run(winner, func(t *testing.T) {
			r, events, _ := lintRunFixture(t, 1, 100*time.Millisecond)
			entered := make(chan context.Context, 1)
			deadlineReached, release := make(chan struct{}), make(chan struct{})
			defer close(release)
			done := lintRunAsync(r, t.Context(), lintRunKey(), func(ctx context.Context) (RunOutput, error) {
				entered <- ctx
				<-ctx.Done()
				close(deadlineReached)
				<-release
				return RunOutput{}, ctx.Err()
			})
			flight := events.take(t, RunAcquired, 0)
			workerCtx := <-entered
			if winner == "finish-wins" {
				// Deliberately keep AfterFunc from being scheduled. The real
				// worker still reaches its real deadline, then finishes its
				// callback. This exercises that winning path without a race.
				r.mu.Lock()
				stopExpiry := r.flights[lintRunKey()].stopExpiry
				r.mu.Unlock()
				if workerCtx.Err() != nil || !stopExpiry() {
					t.Fatal("fixture did not stop AfterFunc before the worker deadline")
				}
			}
			lintRunWait(t, deadlineReached, "the real worker deadline")
			if !errors.Is(workerCtx.Err(), context.DeadlineExceeded) {
				t.Fatalf("worker ended without reaching its deadline: %v", workerCtx.Err())
			}
			observed := 0
			if winner == "afterfunc-wins" {
				events.take(t, RunExpired, flight.FlightID)
				observed++
			}
			lintRunBusy(t, lintRunTake(t, done).err)
			release <- struct{}{}
			events.take(t, RunFinished, flight.FlightID)
			if err := r.Stop(t.Context()); err != nil {
				t.Fatal(err)
			}
			// Only this flight's expiry matters. No total order is imposed
			// on events emitted by different goroutines.
			for _, event := range events.pending {
				if event.Kind == RunExpired && event.FlightID == flight.FlightID {
					observed++
				}
			}
			for {
				select {
				case event := <-events.c:
					if event.Kind == RunExpired && event.FlightID == flight.FlightID {
						observed++
					}
				default:
					if observed != 1 {
						t.Fatalf("%s published expired but emitted %d RunExpired events; want exactly one", winner, observed)
					}
					return
				}
			}
		})
	}
}
