package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

const commitTimeout = 2 * time.Second

// newNotes returns a pool of at most maxConns connections on a new database
// holding one table, notes.
func newNotes(t *testing.T, maxConns int32) *pgxpool.Pool {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewEmptyDatabase(t), MaxConns: maxConns})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "CREATE TABLE notes (id int PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	return pool
}

func insertNote(ctx context.Context, pool *pgxpool.Pool, id int) error {
	_, err := postgres.DB(ctx, pool).Exec(ctx, "INSERT INTO notes (id) VALUES ($1)", id)
	return err
}

func countNotes(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM notes").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWithinTxCommits(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)

	err := tm.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := insertNote(ctx, pool, 1); err != nil {
			return err
		}
		return insertNote(ctx, pool, 2)
	})

	if err != nil || countNotes(t, pool) != 2 {
		t.Errorf("WithinTx() = %v with %d notes, want nil and 2", err, countNotes(t, pool))
	}
}

func TestInTxIsTrueOnlyWithinTx(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	inside, method := false, false
	err := tm.WithinTx(context.Background(), func(ctx context.Context) error {
		inside, method = postgres.InTx(ctx), tm.InTx(ctx)
		return nil
	})
	if err != nil || !inside || postgres.InTx(context.Background()) {
		t.Errorf("InTx() = %v within WithinTx (error %v), %v outside; want true, false", inside, err, postgres.InTx(context.Background()))
	}
	if !method || tm.InTx(context.Background()) {
		t.Errorf("TxManager.InTx() = %v within WithinTx, %v outside; want true, false", method, tm.InTx(context.Background()))
	}
}

func TestWithinTxRollsBackOnError(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	boom := errors.New("boom")

	err := tm.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := insertNote(ctx, pool, 1); err != nil {
			return err
		}
		return boom
	})

	if !errors.Is(err, boom) || countNotes(t, pool) != 0 {
		t.Errorf("WithinTx() = %v with %d notes, want boom and 0", err, countNotes(t, pool))
	}
}

func TestWithinTxRollsBackOnPanic(t *testing.T) {
	pool := newNotes(t, 1)
	tm := postgres.NewTxManager(pool, commitTimeout)

	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic did not reach the caller")
			}
		}()
		_ = tm.WithinTx(context.Background(), func(ctx context.Context) error {
			if err := insertNote(ctx, pool, 1); err != nil {
				return err
			}
			panic("boom")
		})
	}()

	// One connection: the count only runs if the rollback released it.
	if n := countNotes(t, pool); n != 0 {
		t.Errorf("notes after a panic = %d, want 0", n)
	}
}

func TestNestedWithinTxJoinsTheOuterTransaction(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	boom := errors.New("boom")

	err := tm.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := insertNote(ctx, pool, 1); err != nil {
			return err
		}
		inner := tm.WithinTx(ctx, func(ctx context.Context) error {
			var seen int
			// The inner call sees the outer insert: it runs in the same transaction.
			if err := postgres.DB(ctx, pool).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&seen); err != nil {
				return err
			}
			if seen != 1 {
				t.Errorf("inner transaction sees %d notes, want the outer insert", seen)
			}
			return insertNote(ctx, pool, 2)
		})
		if inner != nil {
			return inner
		}
		return boom
	})

	if !errors.Is(err, boom) || countNotes(t, pool) != 0 {
		t.Errorf("WithinTx() = %v with %d notes, want boom and 0: the outer rollback undoes the inner insert", err, countNotes(t, pool))
	}
}

func TestDBOutsideATransactionIsThePool(t *testing.T) {
	pool := newNotes(t, 1)

	if got := postgres.DB(context.Background(), pool); got != postgres.Querier(pool) {
		t.Errorf("DB() = %T, want the pool", got)
	}
}

// The request deadline cancels statements, never the COMMIT of a transaction
// whose statements have all finished.
func TestWithinTxCommitsAfterTheContextIsCancelled(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	ctx, cancel := context.WithCancel(context.Background())

	err := tm.WithinTx(ctx, func(ctx context.Context) error {
		if err := insertNote(ctx, pool, 1); err != nil {
			return err
		}
		cancel() // e.g. the request deadline passes after the last statement
		return nil
	})

	if err != nil || countNotes(t, pool) != 1 {
		t.Errorf("WithinTx() = %v with %d notes, want nil and 1", err, countNotes(t, pool))
	}
}

// conflict stands for a module's domain error: a ProblemError by structure,
// which the API would answer as 409.
type conflict struct{}

func (conflict) Error() string       { return "the name is taken" }
func (conflict) ProblemStatus() int  { return 409 }
func (conflict) ProblemCode() string { return "things.taken" }

// A ROLLBACK that fails is an infrastructure fault: the domain error that
// caused it keeps its text but not its identity, so it is not answered as a
// 409 that hides the fault.
func TestWithinTxReportsAFailedRollback(t *testing.T) {
	pool := newNotes(t, 2)
	tm := postgres.NewTxManager(pool, commitTimeout)

	err := tm.WithinTx(context.Background(), func(ctx context.Context) error {
		var backend int
		if err := postgres.DB(ctx, pool).QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&backend); err != nil {
			return err
		}
		// The pool's other connection ends this transaction's backend and
		// waits until it is gone: the ROLLBACK reads the server's FATAL.
		var gone bool
		if err := pool.QueryRow(context.Background(), "SELECT pg_terminate_backend($1, 5000)", backend).Scan(&gone); err != nil || !gone {
			return fmt.Errorf("terminate backend %d: %t, %w", backend, gone, err)
		}
		return conflict{}
	})

	var problem interface{ ProblemStatus() int }
	if err == nil || errors.As(err, &problem) {
		t.Fatalf("WithinTx() = %v, want an error that is no ProblemError", err)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "57P01" {
		t.Errorf("WithinTx() = %v, want it to wrap the ROLLBACK's failure, 57P01 admin_shutdown", err)
	}
	if !strings.Contains(err.Error(), conflict{}.Error()) {
		t.Errorf("WithinTx() = %v, want the text of fn's error kept", err)
	}
}

// A failed statement, then a cancelled context: the ROLLBACK still runs, and
// the pool's only connection comes back usable.
func TestWithinTxRollsBackAfterAFailedStatementAndCancel(t *testing.T) {
	pool := newNotes(t, 1)
	tm := postgres.NewTxManager(pool, commitTimeout)
	ctx, cancel := context.WithCancel(context.Background())
	var backend int

	err := tm.WithinTx(ctx, func(ctx context.Context) error {
		if err := postgres.DB(ctx, pool).QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&backend); err != nil {
			return err
		}
		if err := insertNote(ctx, pool, 1); err != nil {
			return err
		}
		err := insertNote(ctx, pool, 1) // duplicate key: the transaction is now aborted
		cancel()
		return err
	})

	if err == nil {
		t.Fatal("WithinTx() = nil, want the duplicate-key error")
	}
	var again int
	if err := pool.QueryRow(context.Background(), "SELECT pg_backend_pid()").Scan(&again); err != nil {
		t.Fatalf("the pool is not usable after the rollback: %v", err)
	}
	if again != backend {
		t.Errorf("backend %d after the rollback, want %d: the connection was not returned to the pool", again, backend)
	}
	if n := countNotes(t, pool); n != 0 {
		t.Errorf("notes = %d, want 0", n)
	}
}

// A deadline that passes while a statement runs makes pgx close the
// connection, so the ROLLBACK cannot run; the server rolls the transaction
// back itself. The caller still recognises the deadline, rather than a
// rollback fault, and the pool stays usable.
func TestWithinTxReportsADeadlineDuringAStatement(t *testing.T) {
	pool := newNotes(t, 1)
	tm := postgres.NewTxManager(pool, commitTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := tm.WithinTx(ctx, func(ctx context.Context) error {
		if err := insertNote(ctx, pool, 1); err != nil {
			return err
		}
		_, err := postgres.DB(ctx, pool).Exec(ctx, "SELECT pg_sleep(5)")
		return err
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WithinTx() = %v, want it to match context.DeadlineExceeded", err)
	}
	if n := countNotes(t, pool); n != 0 {
		t.Errorf("notes = %d, want 0: the abandoned transaction is rolled back", n)
	}
}

// A snapshot sees the database as of its first statement: a row committed
// meanwhile on another connection is not in what it reads, though it is
// there once the snapshot ends.
func TestWithinSnapshotSeesOneMoment(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	if err := insertNote(context.Background(), pool, 1); err != nil {
		t.Fatal(err)
	}

	var before, after int
	err := tm.WithinSnapshot(context.Background(), func(ctx context.Context) error {
		if err := postgres.DB(ctx, pool).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&before); err != nil {
			return err
		}
		if err := insertNote(context.Background(), pool, 2); err != nil { // the pool's own connection, committed
			return err
		}
		return postgres.DB(ctx, pool).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&after)
	})

	if err != nil || before != 1 || after != 1 {
		t.Errorf("WithinSnapshot() = %v, read %d then %d notes; want 1 and 1", err, before, after)
	}
	if n := countNotes(t, pool); n != 2 {
		t.Errorf("%d notes after the snapshot, want 2", n)
	}
}

// A snapshot only reads: a write in it fails, and nothing is written.
func TestWithinSnapshotIsReadOnly(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)

	err := tm.WithinSnapshot(context.Background(), func(ctx context.Context) error {
		return insertNote(ctx, pool, 1)
	})

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "25006" {
		t.Errorf("WithinSnapshot() = %v, want read_only_sql_transaction (25006)", err)
	}
	if n := countNotes(t, pool); n != 0 {
		t.Errorf("%d notes, want none", n)
	}
}

// A snapshot is the outermost transaction: within another, it would not
// read one moment.
func TestWithinSnapshotRefusesToNest(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	ran := false

	err := tm.WithinTx(context.Background(), func(ctx context.Context) error {
		return tm.WithinSnapshot(ctx, func(context.Context) error { ran = true; return nil })
	})

	if !errors.Is(err, postgres.ErrNestedSnapshot) || ran {
		t.Errorf("WithinSnapshot() within WithinTx = %v, ran %v; want ErrNestedSnapshot, not run", err, ran)
	}
}

// A snapshot holds no transaction of WithinTx's: its writes would fail in
// the read-only one, its reads be mistaken for a transaction's own.
func TestWithinTxRefusesASnapshot(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	ran := false

	err := tm.WithinSnapshot(context.Background(), func(ctx context.Context) error {
		return tm.WithinTx(ctx, func(context.Context) error { ran = true; return nil })
	})

	if !errors.Is(err, postgres.ErrTxInSnapshot) || ran {
		t.Errorf("WithinTx() within WithinSnapshot = %v, ran %v; want ErrTxInSnapshot, not run", err, ran)
	}
}

// TxFrom is the transaction the statements of DB run in, and none outside.
func TestTxFromIsTheTransaction(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	if _, ok := postgres.TxFrom(context.Background()); ok {
		t.Error("TxFrom() outside a transaction found one")
	}

	err := tm.WithinTx(context.Background(), func(ctx context.Context) error {
		tx, ok := postgres.TxFrom(ctx)
		if !ok || tx != postgres.DB(ctx, pool) {
			return fmt.Errorf("TxFrom() = %v, %v; want DB's transaction", tx, ok)
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}
