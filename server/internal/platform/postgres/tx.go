package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier runs statements: the transaction WithinTx opened, or the pool.
// sqlc's generated DBTX interface has the same methods, so repositories pass
// DB(ctx, pool) to their generated queries.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// DB returns the transaction that TxManager.WithinTx put in ctx, or pool
// when ctx carries none.
func DB(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// TxManager runs functions in transactions on one pool. It satisfies
// shared.TxManager by structure; the platform does not import shared.
//
// The statements of a transaction run under the caller's context and are
// cancelled with it, e.g. when the request deadline passes. COMMIT and
// ROLLBACK are not: they run under context.WithoutCancel, bounded by their own
// commitTimeout (database.commit_timeout). A transaction whose statements
// have all finished therefore commits even if the request is cancelled
// meanwhile, and a failed one is rolled back and its connection returned to
// the pool in a known state. A COMMIT that gets no answer
// within commitTimeout has an unknown outcome and returns an error. A ROLLBACK
// that fails is an infrastructure fault: WithinTx returns an error that wraps
// the failure and keeps only the text of fn's error, so a domain error behind
// it is answered as a logged 500, not as itself.
type TxManager struct {
	pool          *pgxpool.Pool
	commitTimeout time.Duration
}

// NewTxManager returns a TxManager on pool.
func NewTxManager(pool *pgxpool.Pool, commitTimeout time.Duration) *TxManager {
	return &TxManager{pool: pool, commitTimeout: commitTimeout}
}

// WithinTx runs fn in a transaction and commits it when fn returns nil. The
// context fn receives carries the transaction; a nested WithinTx on it runs
// fn in the same transaction. fn's error, or a panic, rolls it back.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
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
			return fmt.Errorf("roll back transaction after %v: %w", err, rbErr) //nolint:errorlint // see above
		}
		return err
	}
	if err := m.end(ctx, tx.Commit); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// end runs COMMIT or ROLLBACK detached from ctx's cancellation, within the
// commit timeout.
func (m *TxManager) end(ctx context.Context, finish func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.commitTimeout)
	defer cancel()
	return finish(ctx)
}
