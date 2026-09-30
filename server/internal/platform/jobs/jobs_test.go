package jobs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// logBuffer collects the log lines that River's goroutines and the runner
// write concurrently.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newLogger(b *logBuffer) *slog.Logger { return slog.New(slog.NewTextHandler(b, nil)) }

// newPool connects to a database of its own, migrated, River's tables too.
func newPool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: url, MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type probeArgs struct{}

func (probeArgs) Kind() string { return "jobs_test.probe" }

// probeWorker runs work for every probe job.
type probeWorker struct {
	river.WorkerDefaults[probeArgs]
	work func(ctx context.Context) error
}

func (w *probeWorker) Work(ctx context.Context, _ *river.Job[probeArgs]) error { return w.work(ctx) }

// probeJob is a periodic job every interval, the first at once.
func probeJob(interval time.Duration, work func(ctx context.Context) error) Job {
	return Job{
		Add: func(w *river.Workers) error { return river.AddWorkerSafely(w, &probeWorker{work: work}) },
		Periodic: river.NewPeriodicJob(river.PeriodicInterval(interval),
			func() (river.JobArgs, *river.InsertOpts) { return probeArgs{}, nil },
			&river.PeriodicJobOpts{ID: "jobs_test.probe", RunOnStart: true}),
	}
}

// receive fails the test unless ch yields within limit.
func receive[T any](t *testing.T, ch <-chan T, limit time.Duration, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(limit):
		t.Fatalf("no %s within %s", what, limit)
	}
	var zero T
	return zero
}

// stop runs r.Stop(ctx) and fails the test unless it returns within limit.
func stop(t *testing.T, r *Runner, ctx context.Context, limit time.Duration) error {
	t.Helper()
	stopped := make(chan error, 1)
	go func() { stopped <- r.Stop(ctx) }()
	return receive(t, stopped, limit, "return from Stop")
}

func mustStart(t *testing.T, r *Runner) {
	t.Helper()
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start() = %v", err)
	}
}

// The periodic job runs at once, then every interval, until Stop; Stop
// returns once the client has stopped, and nothing runs after it. River's
// own lines go to the runner's logger.
func TestRunnerWorksAPeriodicJobUntilStopped(t *testing.T) {
	t.Parallel()
	runs := make(chan time.Time, 100)
	var logs logBuffer
	r, err := New(newPool(t, pgtest.NewDatabase(t)), Config{ShutdownTimeout: 5 * time.Second, Logger: newLogger(&logs)},
		[]Job{probeJob(time.Second, func(context.Context) error { runs <- time.Now(); return nil })})
	if err != nil {
		t.Fatal(err)
	}
	mustStart(t, r)

	first := receive(t, runs, 15*time.Second, "first run")
	second := receive(t, runs, 10*time.Second, "second run")
	if gap := second.Sub(first); gap < 500*time.Millisecond {
		t.Errorf("runs %s apart, want about the 1s interval", gap)
	}
	if err := stop(t, r, context.Background(), 5*time.Second+cancelGrace+2*time.Second); err != nil {
		t.Fatalf("Stop() = %v", err)
	}
	for len(runs) > 0 { // a run that was fetched before Stop
		<-runs
	}
	select {
	case at := <-runs:
		t.Errorf("a run at %v after Stop returned", at)
	case <-time.After(1500 * time.Millisecond):
	}
	out := logs.String()
	if !strings.Contains(out, `msg="jobs started" shutdown_timeout=5s`) || !strings.Contains(out, `msg="jobs stopped"`) {
		t.Errorf("logs lack the start and the stop:\n%s", out)
	}
	// River v0.47.0's start line; on a River upgrade, check its wording here.
	if !strings.Contains(out, `msg="River client started"`) {
		t.Errorf("logs lack River's own lines, want them in the runner's logger:\n%s", out)
	}
}

// Stop lets a running job finish for jobs.shutdown_timeout, then cancels
// its context and returns once it has returned. Two settings, so the value
// reaches River whichever it is.
func TestStopCancelsARunningJobAfterTheShutdownTimeout(t *testing.T) {
	t.Parallel()
	for _, timeout := range []time.Duration{time.Second, 3 * time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			t.Parallel()
			running, cancelled := make(chan struct{}, 1), make(chan time.Time, 1)
			r, err := New(newPool(t, pgtest.NewDatabase(t)), Config{ShutdownTimeout: timeout, Logger: slog.New(slog.DiscardHandler)},
				[]Job{probeJob(time.Hour, func(ctx context.Context) error {
					running <- struct{}{}
					select {
					case <-ctx.Done():
						cancelled <- time.Now()
						return ctx.Err()
					case <-time.After(15 * time.Second): // never cancelled: end, so the test fails instead of hanging
						return errors.New("the job's context was never cancelled")
					}
				})})
			if err != nil {
				t.Fatal(err)
			}
			mustStart(t, r)
			receive(t, running, 15*time.Second, "running job")

			stopping := time.Now()
			err = stop(t, r, context.Background(), timeout+cancelGrace+2*time.Second)
			took := time.Since(stopping)

			at := receive(t, cancelled, time.Second, "cancellation")
			if err != nil || took < timeout || took > timeout+cancelGrace {
				t.Errorf("Stop() = %v after %s, want nil after %s to %s", err, took, timeout, timeout+cancelGrace)
			}
			if waited := at.Sub(stopping); waited < timeout || waited > timeout+500*time.Millisecond {
				t.Errorf("the job's context was cancelled %s after Stop began, want %s", waited, timeout)
			}
		})
	}
}

// A job that ignores its cancelled context does not hold nervewiki's
// shutdown: Stop gives up cancelGrace after the timeout.
func TestStopGivesUpOnAJobThatIgnoresCancellation(t *testing.T) {
	t.Parallel()
	running, release := make(chan struct{}, 1), make(chan struct{})
	r, err := New(newPool(t, pgtest.NewDatabase(t)), Config{ShutdownTimeout: time.Second, Logger: slog.New(slog.DiscardHandler)},
		[]Job{probeJob(time.Hour, func(context.Context) error {
			running <- struct{}{}
			<-release
			return nil
		})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { close(release) }) // before the pool closes: cleanups run last first
	mustStart(t, r)
	receive(t, running, 15*time.Second, "running job")

	stopping := time.Now()
	err = stop(t, r, context.Background(), time.Second+cancelGrace+2*time.Second)
	took := time.Since(stopping)

	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "jobs still running 1s after jobs.shutdown_timeout (1s)") ||
		took < time.Second+cancelGrace || took > time.Second+cancelGrace+500*time.Millisecond {
		t.Errorf("Stop() = %v after %s, want the deadline after %s", err, took, time.Second+cancelGrace)
	}
}

// River's Start fails while the database cannot be reached: Start answers
// the error, and Stop has nothing to stop.
func TestStartWithoutADatabase(t *testing.T) {
	var logs logBuffer
	r, err := New(newPool(t, "postgres://nobody@127.0.0.1:1/nowhere"), Config{ShutdownTimeout: 5 * time.Second, Logger: newLogger(&logs)},
		[]Job{probeJob(time.Hour, func(context.Context) error { return nil })})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := r.Start(ctx); err == nil || !strings.Contains(err.Error(), "start the jobs") {
		t.Errorf("Start() = %v, want the failure to reach the database", err)
	}
	if err := stop(t, r, context.Background(), time.Second); err != nil {
		t.Errorf("Stop() = %v, want nil: nothing started", err)
	}
	if strings.Contains(logs.String(), `msg="jobs started"`) {
		t.Errorf("logs claim the jobs started:\n%s", logs.String())
	}
}

func TestNewRejectsTwoWorkersOfOneKind(t *testing.T) {
	job := probeJob(time.Hour, func(context.Context) error { return nil })
	_, err := New(newPool(t, "postgres://nobody@127.0.0.1:1/nowhere"), Config{ShutdownTimeout: time.Second, Logger: slog.New(slog.DiscardHandler)},
		[]Job{job, {Add: job.Add}})
	if err == nil || !strings.Contains(err.Error(), "register a job") {
		t.Errorf("New() = %v, want the second worker of jobs_test.probe refused", err)
	}
}

// At most two jobs run at once: a third waits until one of them returns.
func TestRunnerWorksTwoJobsAtOnce(t *testing.T) {
	t.Parallel()
	running, release := make(chan struct{}, 3), make(chan struct{})
	job := probeJob(time.Hour, func(context.Context) error {
		running <- struct{}{}
		<-release
		return nil
	})
	r, err := New(newPool(t, pgtest.NewDatabase(t)), Config{ShutdownTimeout: time.Second, Logger: slog.New(slog.DiscardHandler)},
		[]Job{{Add: job.Add}})
	if err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	releaseAll := func() { once.Do(func() { close(release) }) }
	t.Cleanup(releaseAll) // before the pool closes: cleanups run last first
	for range 3 {
		if _, err := r.client.(*river.Client[pgx.Tx]).Insert(context.Background(), probeArgs{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	mustStart(t, r)
	receive(t, running, 15*time.Second, "first job")
	receive(t, running, 5*time.Second, "second job")
	select {
	case <-running:
		t.Errorf("a third job runs alongside the first two, want at most two at once")
	case <-time.After(1500 * time.Millisecond):
	}
	releaseAll()
	receive(t, running, 5*time.Second, "third job, once the first two returned")
	if err := stop(t, r, context.Background(), 5*time.Second); err != nil {
		t.Errorf("Stop() = %v", err)
	}
}
