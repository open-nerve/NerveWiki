package pgtest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WaitForLockWaits returns once at least n backends connected to pool's
// database wait for a lock of any kind, and fails the test when fewer have
// within limit: a wait for a row, for an advisory lock, or for the
// transaction that inserted the same key all count. Only that database
// counts: every test has its own (NewDatabase), so another test's lock
// waits never satisfy it. A test forces an interleaving with it: it holds a
// lock, starts the statements that must queue behind it, waits for them,
// and only then lets go.
//
// Every poll, its wait for a connection of the pool included, ends limit
// after the start, so an exhausted pool fails the test at the deadline too.
func WaitForLockWaits(t testing.TB, pool *pgxpool.Pool, n int, limit time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	for {
		var waiting int
		err := pool.QueryRow(ctx,
			"SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(&waiting)
		switch {
		case ctx.Err() != nil:
			t.Fatalf("fewer than %d statements waited for a lock within %v", n, limit)
		case err != nil:
			t.Fatal(err)
		case waiting >= n:
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Millisecond):
		}
	}
}
