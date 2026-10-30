package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Runs owns accepting flights, their one result, and all workers still
// cleaning up. A flight's expiry removes only its permission to be joined.
type Runs struct {
	mu      sync.Mutex
	closed  bool
	flights map[RunKey]*lintFlight
	workers map[*lintFlight]struct{}
	active  int
	drained chan struct{}
	slots   chan struct{}
	timeout time.Duration
	log     Logger
	observe RunObserver
	nextID  uint64
}

type lintFlight struct {
	key        RunKey
	id         uint64
	ctx        context.Context
	cancel     context.CancelFunc
	stopExpiry func() bool
	started    time.Time
	done       chan struct{}
	published  bool
	result     domain.LintResult
	err        error
	outcome    string
}

var errLintWorker = errors.New("internal failure")

// NewRuns constructs an idle owner. Its bounds come from the request pool
// and timeout at the composition root; invalid bounds are assembly errors.
func NewRuns(max int, timeout time.Duration, log Logger, observe RunObserver) *Runs {
	if max <= 0 || timeout <= 0 || log == nil {
		panic("wiki lint runs require positive bounds and a logger")
	}
	return &Runs{
		flights: make(map[RunKey]*lintFlight), workers: make(map[*lintFlight]struct{}),
		drained: make(chan struct{}), slots: make(chan struct{}, max),
		timeout: timeout, log: log, observe: observe,
	}
}

// Do joins before competing for a slot. Every caller has an independent
// wait budget; its cancellation never cancels the shared worker.
func (r *Runs) Do(ctx context.Context, key RunKey, run func(context.Context) (RunOutput, error)) (domain.LintResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.LintResult{}, err
	}
	budget := r.timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return domain.LintResult{}, context.DeadlineExceeded
		}
		budget = min(budget, remaining/2)
	}
	if budget <= 0 {
		return domain.LintResult{}, shared.ServerBusy(time.Second)
	}
	wait, cancelWait := context.WithTimeout(ctx, budget)
	defer cancelWait()
	if !key.Explicit {
		key.Contract, key.HasRoot, key.RootID = "", false, uuid.UUID{}
	} else if !key.HasRoot {
		key.RootID = uuid.UUID{}
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return domain.LintResult{}, shared.ServerBusy(time.Second)
	}
	f, joined := r.flights[key]
	if !joined {
		worker, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.timeout)
		r.nextID++
		f = &lintFlight{key: key, id: r.nextID, ctx: worker, cancel: cancel, started: time.Now(), done: make(chan struct{})}
		r.flights[key] = f
		r.workers[f] = struct{}{}
		r.active++
		f.stopExpiry = context.AfterFunc(worker, func() { r.expire(f) })
	}
	r.mu.Unlock()
	if joined {
		r.notify(f, RunJoined)
	} else {
		r.notify(f, RunCreated)
		go r.work(f, run)
	}
	select {
	case <-wait.Done():
		if err := ctx.Err(); err != nil {
			return domain.LintResult{}, err
		}
		return domain.LintResult{}, shared.ServerBusy(time.Second)
	case <-f.done:
		if err := ctx.Err(); err != nil {
			return domain.LintResult{}, err
		}
		if wait.Err() != nil {
			return domain.LintResult{}, shared.ServerBusy(time.Second)
		}
		if f.err != nil {
			return domain.LintResult{}, f.err
		}
		return domain.CloneLintResult(f.result), nil
	}
}

func (r *Runs) work(f *lintFlight, run func(context.Context) (RunOutput, error)) {
	var output RunOutput
	var runErr error
	var acquired bool
	var waited time.Duration
	defer func() {
		panicked := recover() != nil
		if panicked {
			runErr = errLintWorker
		}
		// All callback defers, including snapshot cleanup, ran before this.
		r.finish(f, output, runErr, acquired, waited, panicked)
	}()
	select {
	case <-f.ctx.Done():
		runErr = f.ctx.Err()
		return
	case r.slots <- struct{}{}:
		acquired = true
	}
	waited = time.Since(f.started)
	if err := f.ctx.Err(); err != nil {
		runErr = err
		return
	}
	r.notify(f, RunAcquired)
	output, runErr = run(f.ctx)
}

func (r *Runs) expire(f *lintFlight) {
	r.mu.Lock()
	expired := !f.published
	if expired {
		r.forget(f)
		r.publish(f, domain.LintResult{}, shared.ServerBusy(time.Second), "expired")
	}
	r.mu.Unlock()
	if expired {
		r.notify(f, RunExpired)
	}
}

func (r *Runs) finish(f *lintFlight, output RunOutput, runErr error, acquired bool, waited time.Duration, panicked bool) {
	if acquired {
		<-r.slots
	} else {
		waited = time.Since(f.started)
	}
	f.stopExpiry()
	r.mu.Lock()
	r.forget(f)
	expired := false
	if !f.published {
		switch {
		case f.ctx.Err() != nil:
			expired = true
			r.publish(f, domain.LintResult{}, shared.ServerBusy(time.Second), "expired")
		case runErr != nil:
			r.publish(f, domain.LintResult{}, runErr, "error")
		default:
			r.publish(f, domain.CloneLintResult(output.Result), nil, "success")
		}
	}
	outcome := f.outcome
	r.mu.Unlock()
	if expired {
		r.notify(f, RunExpired)
	}
	f.cancel()
	r.logFinished(f, output.Stats, runErr, waited, outcome, panicked)
	r.mu.Lock()
	delete(r.workers, f)
	r.active--
	if r.closed && r.active == 0 {
		close(r.drained)
	}
	r.mu.Unlock()
	r.notify(f, RunFinished)
}

// forget and publish require mu. A late worker cannot remove a newer
// accepting flight with an equal key or publish a second terminal result.
func (r *Runs) forget(f *lintFlight) {
	if r.flights[f.key] == f {
		delete(r.flights, f.key)
	}
}

func (*Runs) publish(f *lintFlight, result domain.LintResult, err error, outcome string) {
	f.result, f.err, f.outcome, f.published = result, err, outcome, true
	close(f.done)
}

// Stop closes admission once and cancels all workers, including expired
// ones still cleaning up. Repeated callers share one drained channel.
func (r *Runs) Stop(ctx context.Context) error {
	var cancels []context.CancelFunc
	var stopped []*lintFlight
	r.mu.Lock()
	if !r.closed {
		r.closed = true
		for f := range r.workers {
			cancels = append(cancels, f.cancel)
		}
		for _, f := range r.flights {
			r.publish(f, domain.LintResult{}, shared.ServerBusy(time.Second), "stopped")
			stopped = append(stopped, f)
		}
		clear(r.flights)
		if r.active == 0 {
			close(r.drained)
		}
	}
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	for _, f := range stopped {
		r.notify(f, RunClosed)
	}
	select {
	case <-r.drained:
		return nil
	default:
	}
	select {
	case <-r.drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runs) notify(f *lintFlight, kind RunEventKind) {
	if r.observe != nil {
		r.observe(RunEvent{Kind: kind, FlightID: f.id, NotebookID: f.key.NotebookID})
	}
}

func (r *Runs) logFinished(f *lintFlight, stats RunStats, runErr error, waited time.Duration, outcome string, panicked bool) {
	mode := "unknown"
	if stats.Mode == domain.ModeStandard || stats.Mode == domain.ModeLLMWiki {
		mode = string(stats.Mode)
	}
	kind, rule := "unknown", "unknown"
	switch f.key.Kind {
	case domain.LintSummary:
		kind, rule = "summary", "summary"
	case domain.LintItems:
		kind = "items"
		if known, ok := domain.ParseLintRule(string(f.key.Rule)); ok {
			rule = string(known)
		}
	}
	fault, cleanup := "none", "callback_returned"
	if panicked {
		fault, cleanup = "panic", "panic_unwound"
	} else if runErr != nil {
		fault = "error"
	}
	attrs := []any{
		slog.String("notebook_id", f.key.NotebookID.String()), slog.String("mode", mode),
		slog.String("query_kind", kind), slog.String("rule", rule), slog.String("outcome", outcome),
		slog.String("fault", fault), slog.String("cleanup", cleanup),
		slog.Int64("wait_ms", waited.Milliseconds()), slog.Int64("elapsed_ms", time.Since(f.started).Milliseconds()),
	}
	if stats.StalePages != nil {
		attrs = append(attrs, slog.Int64("stale_pages", *stats.StalePages))
	}
	if stats.PrepareElapsed != nil {
		attrs = append(attrs, slog.Int64("prepare_ms", stats.PrepareElapsed.Milliseconds()))
	}
	if stats.SharedSources != nil {
		attrs = append(attrs, slog.Int64("shared_sources_ms", stats.SharedSources.Milliseconds()))
	}
	rules := make([]any, 0, len(stats.Rules))
	for _, stat := range stats.Rules {
		if known, ok := domain.ParseLintRule(string(stat.Rule)); ok {
			rules = append(rules, slog.Group(string(known), slog.Int64("duration_ms", stat.Duration.Milliseconds()),
				slog.Int64("count", stat.Count.Value), slog.Bool("capped", stat.Count.Capped)))
		}
	}
	if len(rules) > 0 {
		attrs = append(attrs, slog.Group("rules", rules...))
	}
	var coded interface{ SQLState() string }
	if !panicked && errors.As(runErr, &coded) {
		if code := coded.SQLState(); lintSQLState(code) {
			attrs = append(attrs, slog.String("worker_sqlstate", code))
		}
	}
	message := "lint stopped"
	if outcome == "success" {
		message = "lint computed"
	}
	r.log.InfoContext(context.WithoutCancel(f.ctx), message, attrs...)
}

func lintSQLState(code string) bool {
	if len(code) != 5 {
		return false
	}
	for _, b := range []byte(code) {
		if (b < '0' || b > '9') && (b < 'A' || b > 'Z') {
			return false
		}
	}
	return true
}
