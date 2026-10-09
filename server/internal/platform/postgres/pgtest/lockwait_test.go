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
// wait that never came, rather than wait for a connection without end; a
// probe that first looks its table up too, its lookup failing at the
// deadline.
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

	const limit = 300 * time.Millisecond
	tests := []struct {
		name string
		wait func(testing.TB)
		want string
	}{
		{"WaitForLockWaits", func(tb testing.TB) { pgtest.WaitForLockWaits(tb, pool, 1, limit) },
			"0 statement(s) waited for a lock within 300ms, want at least 1"},
		{"WaitForLockWaitsOn", func(tb testing.TB) { pgtest.WaitForLockWaitsOn(tb, pool, "users", 1, limit) },
			"0 statement(s) waited for a row lock of users within 300ms, want at least 1"},
		{"WaitForKeyWaitOn", func(tb testing.TB) { pgtest.WaitForKeyWaitOn(tb, pool, "users", 1, limit) },
			"0 statement(s) waited for a key of users within 300ms, want at least 1"},
		{"WaitForAdvisoryLockWaits", func(tb testing.TB) { pgtest.WaitForAdvisoryLockWaits(tb, pool, 7, 8, 1, limit) },
			"0 statement(s) waited for the advisory lock 7, 8 within 300ms, want at least 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failed := make(chan string, 1)
			go func() { failed <- fatalOf(tt.wait) }()
			select {
			case got := <-failed:
				if got != tt.want {
					t.Errorf("%s failed with %q, want it to fail at its deadline: %q", tt.name, got, tt.want)
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("%s still waits for a connection after 10s, want it failed at its 300ms deadline", tt.name)
			}
		})
	}
}

// WaitForLockWaitsOn counts the waits for the rows of its table only: an
// advisory lock's waits, as River's services may make in a running server,
// do not satisfy it; a wait for a row of the table does.
func TestWaitForLockWaitsOnCountsTheRowsOfItsTable(t *testing.T) {
	t.Parallel()
	url := pgtest.NewDatabase(t)
	pool := newPool(t, url)
	holdAndWait(t, url, 2)

	advisory := fatalOf(func(tb testing.TB) { pgtest.WaitForLockWaitsOn(tb, pool, "users", 1, 300*time.Millisecond) })
	holdRowAndWait(t, url)
	pgtest.WaitForLockWaitsOn(t, pool, "users", 1, 10*time.Second)
	other := fatalOf(func(tb testing.TB) { pgtest.WaitForLockWaitsOn(tb, pool, "auth_sessions", 1, 300*time.Millisecond) })
	missing := fatalOf(func(tb testing.TB) { pgtest.WaitForLockWaitsOn(tb, pool, "nope", 1, 300*time.Millisecond) })

	if advisory != "0 statement(s) waited for a row lock of users within 300ms, want at least 1" ||
		other != "0 statement(s) waited for a row lock of auth_sessions within 300ms, want at least 1" ||
		missing != `pgtest: no table "nope"` {
		t.Errorf("advisory waits failed with %q, auth_sessions with %q, no table with %q; want each to fail", advisory, other, missing)
	}
}

// WaitForTableLockWaits counts the waits for its table's own lock only: not
// a wait for a row of it, nor for another table's lock.
func TestWaitForTableLockWaitsCountsItsTablesLock(t *testing.T) {
	t.Parallel()
	url := pgtest.NewDatabase(t)
	pool := newPool(t, url)
	holdRowAndWait(t, url)
	rows := fatalOf(func(tb testing.TB) { pgtest.WaitForTableLockWaits(tb, pool, "users", 1, 300*time.Millisecond) })
	holdAndWaitIn(t, url, "LOCK TABLE auth_sessions IN ACCESS EXCLUSIVE MODE", "SELECT 1 FROM auth_sessions", 2)

	pgtest.WaitForTableLockWaits(t, pool, "auth_sessions", 2, 10*time.Second)
	three := fatalOf(func(tb testing.TB) { pgtest.WaitForTableLockWaits(tb, pool, "auth_sessions", 3, 300*time.Millisecond) })
	missing := fatalOf(func(tb testing.TB) { pgtest.WaitForTableLockWaits(tb, pool, "nope", 1, 300*time.Millisecond) })
	if rows != "0 statement(s) waited for the lock of the table users within 300ms, want at least 1" ||
		three != "2 statement(s) waited for the lock of the table auth_sessions within 300ms, want at least 3" ||
		missing != `pgtest: no table "nope"` {
		t.Errorf("a row's wait failed with %q, 3 waits with %q, no table with %q; want each to fail", rows, three, missing)
	}
}

// WaitForAdvisoryLockWaits counts the waits for its key pair only, a
// negative key too: not those for another pair, nor for a row.
func TestWaitForAdvisoryLockWaitsCountsItsKeys(t *testing.T) {
	t.Parallel()
	url := pgtest.NewDatabase(t)
	pool := newPool(t, url)
	holdAndWaitIn(t, url, "SELECT pg_advisory_xact_lock(7, -9)", "SELECT pg_advisory_xact_lock(7, -9)", 2)
	holdAndWaitIn(t, url, "SELECT pg_advisory_xact_lock(7, 8)", "SELECT pg_advisory_xact_lock(7, 8)", 1)
	holdRowAndWait(t, url)

	pgtest.WaitForAdvisoryLockWaits(t, pool, 7, -9, 2, 10*time.Second)
	three := fatalOf(func(tb testing.TB) { pgtest.WaitForAdvisoryLockWaits(tb, pool, 7, -9, 3, 300*time.Millisecond) })
	other := fatalOf(func(tb testing.TB) { pgtest.WaitForAdvisoryLockWaits(tb, pool, 9, -9, 1, 300*time.Millisecond) })
	if three != "2 statement(s) waited for the advisory lock 7, -9 within 300ms, want at least 3" ||
		other != "0 statement(s) waited for the advisory lock 9, -9 within 300ms, want at least 1" {
		t.Errorf("3 waits failed with %q, another pair with %q; want both to fail at their deadline", three, other)
	}
}

// WaitForKeyWaitOn counts the INSERTs that wait for the transaction that
// inserted the same key into its table: a wait WaitForLockWaitsOn does not
// see, having no tuple lock. It does not count a wait for a row of its
// table, though the waiter has written the table; a key's wait on another
// table by a transaction that has read its table; or a wait for an advisory
// lock by a transaction that has written its table. Each case waits in a
// database of its own, and WaitForLockWaits proves it waits, so it fails
// for the kind of its wait.
func TestWaitForKeyWaitOnCountsTheKeysWaitsOnItsTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	prepared := pgtest.NewDatabase(t)
	setup := connect(t, prepared)
	if _, err := setup.Exec(ctx, "CREATE TABLE a (id int PRIMARY KEY, n int); CREATE TABLE b (id int PRIMARY KEY); INSERT INTO a VALUES (1, 0)"); err != nil {
		t.Fatal(err)
	}
	if err := setup.Close(ctx); err != nil {
		t.Fatal(err)
	}
	key, row, other, advisory := pgtest.NewDatabaseFrom(t, prepared), pgtest.NewDatabaseFrom(t, prepared),
		pgtest.NewDatabaseFrom(t, prepared), pgtest.NewDatabaseFrom(t, prepared)
	holdAndWaitIn(t, key, "INSERT INTO a VALUES (2, 0)", "INSERT INTO a VALUES (2, 0)", 2)
	holdAndWaitIn(t, row, "UPDATE a SET n = 1 WHERE id = 1", "INSERT INTO a VALUES (3, 0); UPDATE a SET n = 2 WHERE id = 1", 1)
	holdAndWaitIn(t, other, "INSERT INTO b VALUES (2)", "SELECT count(*) FROM a; INSERT INTO b VALUES (2)", 1)
	holdAndWaitIn(t, advisory, "SELECT pg_advisory_xact_lock(1)", "INSERT INTO a VALUES (3, 0); SELECT pg_advisory_xact_lock(1)", 1)

	keyPool := newPool(t, key)
	pgtest.WaitForKeyWaitOn(t, keyPool, "a", 2, 10*time.Second)
	three := fatalOf(func(tb testing.TB) { pgtest.WaitForKeyWaitOn(tb, keyPool, "a", 3, 300*time.Millisecond) })
	rows := fatalOf(func(tb testing.TB) { pgtest.WaitForLockWaitsOn(tb, keyPool, "a", 1, 300*time.Millisecond) })
	if three != "2 statement(s) waited for a key of a within 300ms, want at least 3" ||
		rows != "0 statement(s) waited for a row lock of a within 300ms, want at least 1" {
		t.Errorf("3 key waits failed with %q, a row lock with %q; want both to fail at their deadline", three, rows)
	}
	for _, tt := range []struct{ name, url string }{
		{"a row of the table, by a writer of it", row},
		{"a key of another table, by a reader of it", other},
		{"an advisory lock, by a writer of it", advisory},
	} {
		pool := newPool(t, tt.url)
		pgtest.WaitForLockWaits(t, pool, 1, 10*time.Second)
		if failed := fatalOf(func(tb testing.TB) { pgtest.WaitForKeyWaitOn(tb, pool, "a", 1, 300*time.Millisecond) }); failed !=
			"0 statement(s) waited for a key of a within 300ms, want at least 1" {
			t.Errorf("%s: WaitForKeyWaitOn failed with %q, want it to fail at its deadline", tt.name, failed)
		}
	}
	if failed := fatalOf(func(tb testing.TB) { pgtest.WaitForKeyWaitOn(tb, keyPool, "nope", 1, 300*time.Millisecond) }); failed !=
		`pgtest: no table "nope"` {
		t.Errorf("WaitForKeyWaitOn(nope) failed with %q, want no table", failed)
	}
}

// holdAndWaitIn makes a connection to url run held in a transaction, and n
// others run waits each in its own after it, until the test ends.
func holdAndWaitIn(t *testing.T, url, held, waits string, n int) {
	t.Helper()
	tx, err := connect(t, url).Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), held); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done := make(chan error, n)
	for range n {
		waiter := connect(t, url)
		go func() {
			wtx, err := waiter.Begin(ctx)
			if err == nil {
				_, err = wtx.Exec(ctx, waits)
			}
			if err == nil {
				err = wtx.Rollback(ctx)
			}
			done <- err
		}()
	}
	// Runs before the connections close: the holder rolls back, and the
	// waiters go on.
	t.Cleanup(func() {
		defer cancel()
		if err := tx.Rollback(context.Background()); err != nil {
			t.Error(err)
		}
		for range n {
			if err := <-done; err != nil {
				t.Errorf("a waiting connection: %v", err)
			}
		}
	})
}

// holdRowAndWait makes a connection to url wait for a row of users that
// another one holds, until the test ends.
func holdRowAndWait(t *testing.T, url string) {
	t.Helper()
	ctx := context.Background()
	holder := connect(t, url)
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (gen_random_uuid(), 'alice@corp.com', 'x', 'alice', now(), now())",
		"BEGIN",
		"SELECT 1 FROM users FOR NO KEY UPDATE",
	} {
		if _, err := holder.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	waiter := connect(t, url)
	done := make(chan error, 1)
	go func() {
		_, err := waiter.Exec(ctx, "SELECT 1 FROM users FOR NO KEY UPDATE")
		done <- err
	}()
	// Runs before the connections close.
	t.Cleanup(func() {
		if _, err := holder.Exec(ctx, "ROLLBACK"); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil {
			t.Errorf("the waiting connection: %v", err)
		}
	})
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
