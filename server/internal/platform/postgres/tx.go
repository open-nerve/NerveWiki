package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier runs statements: the transaction WithinTx opened, the connection
// OnConn named, or the pool.
// sqlc's generated DBTX interface has the same methods, SendBatch where a
// module's queries hold a batch, so repositories pass DB(ctx, pool) to their
// generated queries.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

type txKey struct{}

// connKey marks a context whose statements run on one connection, OnConn's.
type connKey struct{}

// snapshotKey marks a context whose transaction WithinSnapshot opened.
type snapshotKey struct{}

// DB returns the transaction that TxManager.WithinTx put in ctx; else the
// connection OnConn put there; else pool.
func DB(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	if conn, ok := ctx.Value(connKey{}).(*pgxpool.Conn); ok {
		return conn
	}
	return pool
}

// OnConn returns ctx whose statements, DB's and those of the transactions
// TxManager opens in it, run on conn, not on any connection of the pool: a
// job that holds a session's lock on conn does its work there, so that the
// lock is its work's own and the work takes one connection of the pool
// (M8/P5A review B-M4). conn runs one statement at a time: the caller runs
// them in turn.
func OnConn(ctx context.Context, conn *pgxpool.Conn) context.Context {
	return context.WithValue(ctx, connKey{}, conn)
}

// InTx reports whether ctx carries a transaction that TxManager.WithinTx
// opened: a statement whose lock must last until the transaction ends checks
// it, since on the pool the lock would end with the statement.
func InTx(ctx context.Context) bool {
	_, ok := TxFrom(ctx)
	return ok
}

// TxFrom returns the transaction that TxManager.WithinTx or WithinSnapshot
// put in ctx, for an adapter that hands it to another library: the
// transfer module enqueues a job in the request's transaction (M7/P5
// design 3.3).
func TxFrom(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// ErrNestedSnapshot is WithinSnapshot's error when ctx already carries a
// transaction: what it would read would not be one moment of the database.
var ErrNestedSnapshot = errors.New("postgres: a snapshot within a transaction")

// ErrTxInSnapshot is WithinTx's error when ctx carries a snapshot: fn's
// writes would fail in its read-only transaction, or, read alone, be
// mistaken for a transaction of their own.
var ErrTxInSnapshot = errors.New("postgres: a transaction within a snapshot")

// TxManager runs functions in transactions on one pool. It satisfies
// shared.TxManager by structure; the platform does not import shared.
//
// The statements of a transaction run under the caller's context and are
// cancelled with it, e.g. when the request deadline passes. COMMIT and
// ROLLBACK are not: they run under context.WithoutCancel, bounded by their own
// commitTimeout (database.commit_timeout). A transaction whose statements
// have all finished therefore commits even if the request is cancelled
// meanwhile, and a failed one is rolled back and its connection returned to
// the pool in a known state. A COMMIT that gets no answer within
// commitTimeout has an unknown outcome and returns an error.
//
// A ROLLBACK that fails is an infrastructure fault: WithinTx returns an error
// that wraps the failure and keeps only the text of fn's error, so a domain
// error behind it is answered as a logged 500, not as itself. One case is
// not a fault: when the caller's context ends while a statement runs, pgx
// closes the connection, the server rolls the abandoned transaction back, and
// the ROLLBACK fails for want of a connection. The error then wraps the
// context's error, so the caller recognises a deadline or a disconnect.
type TxManager struct {
	pool          *pgxpool.Pool
	commitTimeout time.Duration
}

// NewTxManager returns a TxManager on pool.
func NewTxManager(pool *pgxpool.Pool, commitTimeout time.Duration) *TxManager {
	return &TxManager{pool: pool, commitTimeout: commitTimeout}
}

// InTx is the package's InTx: a use case that must open the outermost
// transaction asks it through its port.
func (m *TxManager) InTx(ctx context.Context) bool {
	return InTx(ctx)
}

// WithinTx runs fn in a transaction and commits it when fn returns nil. The
// context fn receives carries the transaction; a nested WithinTx on it runs
// fn in the same transaction. fn's error, or a panic, rolls it back. On a
// ctx that carries a snapshot it returns ErrTxInSnapshot.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if ctx.Value(snapshotKey{}) != nil {
		return ErrTxInSnapshot
	}
	if InTx(ctx) {
		return fn(ctx)
	}
	return m.run(ctx, pgx.TxOptions{}, fn)
}

// WithinSnapshot runs fn in a read-only transaction at REPEATABLE READ: its
// statements see the database as of its first, whatever commits meanwhile
// (M7/P5 design 3.3). The context fn receives carries it, as WithinTx's
// does; a write in it fails, and a WithinTx in it is refused. It ends as
// WithinTx's does. It must be the outermost transaction: on a ctx that
// carries one it returns ErrNestedSnapshot.
func (m *TxManager) WithinSnapshot(ctx context.Context, fn func(ctx context.Context) error) error {
	if InTx(ctx) {
		return ErrNestedSnapshot
	}
	return m.run(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(ctx context.Context) error {
		return fn(context.WithValue(ctx, snapshotKey{}, true))
	})
}

// run runs fn in a new transaction of opts, as WithinTx describes: on the
// connection OnConn put in ctx, else on one of the pool.
func (m *TxManager) run(ctx context.Context, opts pgx.TxOptions, fn func(ctx context.Context) error) error {
	begin := m.pool.BeginTx
	if conn, ok := ctx.Value(connKey{}).(*pgxpool.Conn); ok {
		begin = conn.BeginTx
	}
	tx, err := begin(ctx, opts)
	if err != nil {
		return &beginError{err}
	}
	returned := false
	defer func() {
		if !returned { // fn panicked: roll back, then let the panic go on
			_ = m.end(ctx, tx.Rollback)
		}
	}()
	err = fn(context.WithValue(ctx, txKey{}, tx))
	returned = true
	if err != nil {
		if rbErr := m.end(ctx, tx.Rollback); rbErr != nil {
			// fn's error keeps only its text: a domain error must not
			// hide the infrastructure fault behind its own answer.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("transaction abandoned after %v: %w", err, errors.Join(ctxErr, rbErr)) //nolint:errorlint // see above
			}
			return fmt.Errorf("roll back transaction after %v: %w", err, rbErr) //nolint:errorlint // see above
		}
		return err
	}
	if err := m.end(ctx, tx.Commit); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// beginError is run's error when its transaction cannot begin, before fn
// runs: the connection it was to run on is broken, or the pool closed, or
// ctx ended. TxNotBegun marks it so, for a caller that tells it from a
// failure of fn's statements without importing the platform
// (shared.TxNotBegun): a job whose connection is gone fails its run, not
// each of its items (M8/P5A fix check M-1).
type beginError struct{ err error }

func (e *beginError) Error() string { return "begin transaction: " + e.err.Error() }

func (e *beginError) Unwrap() error { return e.err }

// TxNotBegun marks the error as a transaction's that did not begin.
func (*beginError) TxNotBegun() bool { return true }

// end runs COMMIT or ROLLBACK detached from ctx's cancellation, within the
// commit timeout.
func (m *TxManager) end(ctx context.Context, finish func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.commitTimeout)
	defer cancel()
	return finish(ctx)
}
