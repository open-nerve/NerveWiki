package jobs

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

type longArgs struct{ N int }

func (longArgs) Kind() string { return "jobs_test.long" }

type longWorker struct {
	river.WorkerDefaults[longArgs]
	ran chan int
}

func (w *longWorker) Work(_ context.Context, j *river.Job[longArgs]) error {
	w.ran <- j.Args.N
	return nil
}

// A job enqueued in a transaction exists exactly when it commits: rolled
// back, River has nothing; committed, the server's client works it in the
// queue it was enqueued in, a queue of its own besides the default.
func TestTheInserterEnqueuesInTheTransaction(t *testing.T) {
	t.Parallel()
	pool := newPool(t, pgtest.NewDatabase(t))
	ctx := context.Background()
	inserter, err := NewInserter(pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	enqueue := func(n int, commit bool) {
		t.Helper()
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if err := inserter.InsertTx(ctx, tx, longArgs{N: n}, &river.InsertOpts{Queue: "jobs_test_long", MaxAttempts: 1}); err != nil {
				return err
			}
			if !commit {
				return errRollBack
			}
			return nil
		})
		if err != nil && !errors.Is(err, errRollBack) {
			t.Fatal(err)
		}
	}
	enqueue(1, false)
	enqueue(2, true)
	var queue string
	var attempts int
	if err := pool.QueryRow(ctx, `SELECT queue, max_attempts FROM river_job WHERE kind = 'jobs_test.long'`).Scan(&queue, &attempts); err != nil {
		t.Fatalf("the committed job: %v (and none rolled back)", err)
	}
	if queue != "jobs_test_long" || attempts != 1 {
		t.Errorf("the job is in %q with %d attempts, want jobs_test_long and 1", queue, attempts)
	}

	worker := &longWorker{ran: make(chan int, 2)}
	r, err := New(pool, Config{ShutdownTimeout: time.Second, Queues: map[string]int{"jobs_test_long": 1}, Logger: slog.New(slog.DiscardHandler)},
		[]Job{{Add: func(w *river.Workers) error { return river.AddWorkerSafely(w, worker) }}})
	if err != nil {
		t.Fatal(err)
	}
	mustStart(t, r)
	if n := receive(t, worker.ran, 15*time.Second, "the committed job's run"); n != 2 {
		t.Errorf("ran job %d, want 2", n)
	}
	select {
	case n := <-worker.ran:
		t.Errorf("job %d ran too, want only the committed one", n)
	case <-time.After(500 * time.Millisecond):
	}
	if err := stop(t, r, ctx, 5*time.Second); err != nil {
		t.Errorf("Stop() = %v", err)
	}
}

var errRollBack = errors.New("roll back")

// The default queue is the periodic jobs': no module takes it over.
func TestNewRefusesTheDefaultQueue(t *testing.T) {
	_, err := New(newPool(t, "postgres://nobody@127.0.0.1:1/nowhere"),
		Config{ShutdownTimeout: time.Second, Queues: map[string]int{river.QueueDefault: 5}, Logger: slog.New(slog.DiscardHandler)}, nil)
	if err == nil || !strings.Contains(err.Error(), "River's default") {
		t.Errorf("New() = %v, want the default queue refused", err)
	}
}

// RescueAfter reaches River, which refuses one shorter than its jobs'
// default timeout, a minute.
func TestRescueAfterReachesRiver(t *testing.T) {
	_, err := New(newPool(t, "postgres://nobody@127.0.0.1:1/nowhere"),
		Config{ShutdownTimeout: time.Second, RescueAfter: 30 * time.Second, Logger: slog.New(slog.DiscardHandler)}, nil)
	if err == nil || !strings.Contains(err.Error(), "RescueStuckJobsAfter") {
		t.Errorf("New() = %v, want River's refusal of a rescue before the timeout", err)
	}
}
