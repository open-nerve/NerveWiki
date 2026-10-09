package pgtest

import (
	"context"
	"fmt"
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
	waitForCount(ctx, t, pool, n, limit, "a row lock of "+table, `
		SELECT count(DISTINCT a.pid) FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid
		WHERE a.datname = current_database() AND a.wait_event_type = 'Lock'
			AND l.locktype = 'tuple' AND l.relation = $1`, relation(ctx, t, pool, table))
}

// WaitForKeyWaitOn is WaitForLockWaitsOn for the wait of a unique index's
// check: an INSERT whose key a live transaction has inserted too waits for
// that transaction to end, with no row to lock, so WaitForLockWaitsOn does
// not see it (M2/P3 design 3.9, interleaving 7). It counts the backends
// that have written table in their transaction and wait for another
// transaction to end without holding a tuple lock. A wait for a row of the
// table holds its tuple lock and does not count; nor does a wait of a
// transaction that has not written the table, nor one for a lock of another
// kind, such as an advisory lock. By a transaction that has written table,
// it cannot tell a key's wait on table from one on another table, or from
// either of the two row waits PostgreSQL makes without a tuple lock
// (WaitForLockWaitsOn): use it where the waiting transaction writes no
// other table with a unique key and neither of those two waits can occur.
func WaitForKeyWaitOn(t testing.TB, pool *pgxpool.Pool, table string, n int, limit time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	waitForCount(ctx, t, pool, n, limit, "a key of "+table, `
		SELECT count(*) FROM pg_stat_activity a
		WHERE a.datname = current_database() AND a.wait_event_type = 'Lock' AND a.wait_event = 'transactionid'
			AND EXISTS (SELECT 1 FROM pg_locks l WHERE l.pid = a.pid AND l.locktype = 'relation' AND l.relation = $1
				AND l.mode = 'RowExclusiveLock' AND l.granted)
			AND NOT EXISTS (SELECT 1 FROM pg_locks l WHERE l.pid = a.pid AND l.locktype = 'tuple')`,
		relation(ctx, t, pool, table))
}

// WaitForAdvisoryLockWaits returns once at least n backends connected to
// pool's database wait for the advisory lock of the key pair space, key
// (pg_advisory_xact_lock(space, key) and its kin), and fails the test when
// fewer have within limit. A wait for another key, or for a lock of
// another kind, does not count: other waits in a running server, such as
// River's, are not mistaken for it.
func WaitForAdvisoryLockWaits(t testing.TB, pool *pgxpool.Pool, space, key int32, n int, limit time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	waitForCount(ctx, t, pool, n, limit, fmt.Sprintf("the advisory lock %d, %d", space, key), `
		SELECT count(DISTINCT l.pid) FROM pg_locks l JOIN pg_database d ON d.oid = l.database
		WHERE d.datname = current_database() AND l.locktype = 'advisory' AND NOT l.granted
			AND l.classid::bigint = $1 AND l.objid::bigint = $2 AND l.objsubid = 2`,
		int64(uint32(space)), int64(uint32(key)))
}

// WaitForTableLockWaits returns once at least n backends wait for a lock
// of table itself, as LOCK TABLE takes one, and fails the test when fewer
// have within limit. A wait for a row of the table, or for another table,
// does not count: a test that holds a table's lock stops whatever reads it,
// such as a background job that holds no row, at its read. A database
// without the table fails the test at once.
func WaitForTableLockWaits(t testing.TB, pool *pgxpool.Pool, table string, n int, limit time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	waitForCount(ctx, t, pool, n, limit, "the lock of the table "+table, `
		SELECT count(DISTINCT l.pid) FROM pg_locks l JOIN pg_database d ON d.oid = l.database
		WHERE d.datname = current_database() AND l.locktype = 'relation' AND NOT l.granted AND l.relation = $1`,
		relation(ctx, t, pool, table))
}

// relation is the OID of table in pool's database. A database without the
// table fails the test at once: a misspelled name can never be waited on.
// A lookup that fails at the deadline, the pool's connection slow to come
// or none free, leaves the failure to waitForCount, whose first poll fails
// at once with no wait seen.
func relation(ctx context.Context, t testing.TB, pool *pgxpool.Pool, table string) uint32 {
	t.Helper()
	var oid *uint32
	if err := pool.QueryRow(ctx, "SELECT to_regclass($1)::oid", table).Scan(&oid); err != nil {
		if ctx.Err() != nil {
			return 0
		}
		t.Fatal(err)
	}
	if oid == nil {
		t.Fatalf("pgtest: no table %q", table)
	}
	return *oid
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
