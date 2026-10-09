package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
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
	var n, attempts int
	var queue string
	err = pool.QueryRow(ctx, `SELECT count(*), min(queue), min(max_attempts) FROM river_job WHERE kind = 'jobs_test.long'`).Scan(&n, &queue, &attempts)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || queue != "jobs_test_long" || attempts != 1 {
		t.Errorf("%d jobs, in %q with %d attempts; want the committed one alone, in jobs_test_long, 1", n, queue, attempts)
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

// The jobs of a kind that River has not finished, a page at a time: those
// to work and those it works, not those it completed or discarded, nor
// another kind's.
func TestUnfinishedAreTheJobsRiverHolds(t *testing.T) {
	t.Parallel()
	pool := newPool(t, pgtest.NewDatabase(t))
	ctx := context.Background()
	inserter, err := NewInserter(pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		for n := 1; n <= 5; n++ {
			if err := inserter.InsertTx(ctx, tx, longArgs{N: n}, &river.InsertOpts{Queue: "jobs_test_long"}); err != nil {
				return err
			}
		}
		return inserter.InsertTx(ctx, tx, probeArgs{}, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`UPDATE river_job SET state = 'running', attempt = 1, attempted_at = now() WHERE args->>'N' = '2'`,
		`UPDATE river_job SET state = 'completed', attempt = 1, attempted_at = now(), finalized_at = now() WHERE args->>'N' = '3'`,
		`UPDATE river_job SET state = 'discarded', attempt = 1, attempted_at = now(), finalized_at = now() WHERE args->>'N' = '4'`,
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	got, err := inserter.unfinished(ctx, longArgs{}.Kind(), 2)
	if err != nil {
		t.Fatal(err)
	}
	var ns []int
	for _, args := range got {
		var a longArgs
		if err := json.Unmarshal(args, &a); err != nil {
			t.Fatal(err)
		}
		ns = append(ns, a.N)
	}
	slices.Sort(ns)
	if !slices.Equal(ns, []int{1, 2, 5}) {
		t.Errorf("Unfinished() = jobs %v, want 1, 2 and 5", ns)
	}
}
