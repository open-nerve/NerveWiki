package pgtest_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// WaitForLockWaits counts the waits in its pool's database only: every test
// has its own database, and another test's waits must not count.
func TestWaitForLockWaitsCountsItsOwnDatabase(t *testing.T) {
	t.Parallel()
	waiting, idle := pgtest.NewDatabase(t), pgtest.NewDatabase(t)
	holdAndWait(t, waiting, 2)
	pool, idlePool := newPool(t, waiting), newPool(t, idle)

	pgtest.WaitForLockWaits(t, pool, 2, 10*time.Second)
	three := fatalOf(func(tb testing.TB) { pgtest.WaitForLockWaits(tb, pool, 3, 300*time.Millisecond) })
	other := fatalOf(func(tb testing.TB) { pgtest.WaitForLockWaits(tb, idlePool, 1, 300*time.Millisecond) })

	if three != "2 statement(s) waited for a lock within 300ms, want at least 3" ||
		other != "0 statement(s) waited for a lock within 300ms, want at least 1" {
		t.Errorf("3 waits failed with %q, the other database with %q; want both to fail at their deadline", three, other)
	}
}

// A probe whose pool has no connection free fails at its deadline, as a
// wait that never came, rather than wait for a connection without end.
func TestWaitForLockWaitsFailsAtItsDeadlineOnAnExhaustedPool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(pgtest.NewDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	held, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(held.Release) // runs before pool.Close

	failed := make(chan string, 1)
	go func() {
		failed <- fatalOf(func(tb testing.TB) { pgtest.WaitForLockWaits(tb, pool, 1, 300*time.Millisecond) })
	}()
	select {
	case got := <-failed:
		if got != "0 statement(s) waited for a lock within 300ms, want at least 1" {
			t.Errorf("WaitForLockWaits failed with %q, want it to fail at its deadline", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("WaitForLockWaits still waits for a connection after 10s, want it failed at its 300ms deadline")
	}
}

// holdAndWait makes n connections to url wait for an advisory lock that
// another one holds, until the test ends.
func holdAndWait(t *testing.T, url string, n int) {
	t.Helper()
	holder := connect(t, url)
	if _, err := holder.Exec(context.Background(), "SELECT pg_advisory_lock(1)"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done := make(chan error, n)
	for range n {
		waiter := connect(t, url)
		go func() {
			_, err := waiter.Exec(ctx, "SELECT pg_advisory_lock(1)")
			if err == nil {
				_, err = waiter.Exec(ctx, "SELECT pg_advisory_unlock(1)")
			}
			done <- err
		}()
	}
	// Runs before the connections close: each waiter gets the lock in turn,
	// or its context ends the wait.
	t.Cleanup(func() {
		defer cancel()
		if _, err := holder.Exec(context.Background(), "SELECT pg_advisory_unlock(1)"); err != nil {
			t.Error(err)
		}
		for range n {
			if err := <-done; err != nil {
				t.Errorf("a waiting connection: %v", err)
			}
		}
	})
}

func newPool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// fatalProbe is a testing.TB whose Fatal and Fatalf record the message and
// end the goroutine, as testing.T's do, without failing the test.
type fatalProbe struct {
	testing.TB
	message string
}

func (p *fatalProbe) Helper() {}

func (p *fatalProbe) Fatal(args ...any) {
	p.message = fmt.Sprint(args...)
	runtime.Goexit()
}

func (p *fatalProbe) Fatalf(format string, args ...any) {
	p.message = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// fatalOf runs f with a fatalProbe on a goroutine of its own and returns
// what f failed with, "" when it did not fail.
func fatalOf(f func(testing.TB)) string {
	p := &fatalProbe{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(p)
	}()
	<-done
	return p.message
}
