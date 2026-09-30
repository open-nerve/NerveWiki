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
// and only then lets go. Where something else may wait on the database
// too, such as River's services in a running server, use
// WaitForLockWaitsOn.
//
// Every poll, its wait for a connection of the pool included, ends limit
// after the start, so an exhausted pool fails the test at the deadline too.
func WaitForLockWaits(t testing.TB, pool *pgxpool.Pool, n int, limit time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	waitForCount(ctx, t, pool, n, limit, "a lock",
		"SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'")
}

// WaitForLockWaitsOn is WaitForLockWaits for the rows of table: only a
// backend waiting for a row lock of table counts. Such a backend holds that
// row's tuple lock on the table while it awaits the holder's transaction,
// or, queued behind another waiter, awaits the tuple lock itself; a wait for
// another table's rows, or for a lock of another kind, such as an advisory
// lock, does not count. PostgreSQL takes no tuple lock, so the probe does
// not see the wait, when a transaction upgrades a row lock it shares with
// another (its FOR SHARE to FOR NO KEY UPDATE while another transaction
// holds FOR SHARE too), and when a FOR KEY SHARE, as a foreign key's check
// takes, follows the row's update chain to a newer version that a live
// transaction has locked or deleted. A database without the table fails
// the test at once.
func WaitForLockWaitsOn(t testing.TB, pool *pgxpool.Pool, table string, n int, limit time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	var relation *uint32
	if err := pool.QueryRow(ctx, "SELECT to_regclass($1)::oid", table).Scan(&relation); err != nil {
		t.Fatal(err)
	}
	if relation == nil {
		t.Fatalf("pgtest: no table %q", table)
	}
	waitForCount(ctx, t, pool, n, limit, "a row lock of "+table, `
		SELECT count(DISTINCT a.pid) FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid
		WHERE a.datname = current_database() AND a.wait_event_type = 'Lock'
			AND l.locktype = 'tuple' AND l.relation = $1`, *relation)
}

// waitForCount polls query, a count of waiting backends, until it reaches
// n; what names the wait in the failure.
func waitForCount(ctx context.Context, t testing.TB, pool *pgxpool.Pool, n int, limit time.Duration, what, query string, args ...any) {
	t.Helper()
	seen := 0 // the last count read: at the deadline, the query itself fails
	for {
		var waiting int
		err := pool.QueryRow(ctx, query, args...).Scan(&waiting)
		if err == nil {
			seen = waiting
		}
		switch {
		case ctx.Err() != nil:
			t.Fatalf("%d statement(s) waited for %s within %v, want at least %d", seen, what, limit, n)
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
