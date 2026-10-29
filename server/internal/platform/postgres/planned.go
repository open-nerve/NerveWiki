package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Planned sends and describes an unnamed statement on every call, a round
// trip more than pgx's cached statement, requiring none of pgx's caches
// (database.url may disable them: M6 FA6-M1). This avoids reusing a named
// statement's generic plan under auto mode. It does not force custom plans:
// PostgreSQL's plan_cache_mode=force_generic_plan also applies to unnamed
// statements, so the queries must work with that mode (M10/P2 CI fix).
// Historically a cached statement could switch to a generic plan after
// five custom plans, comparing each row with every array element instead
// of hashing a constant array: 50,000 subtree parents took 3.5 s (FA4-M1).
// Describing every statement added about a third to rename/move costs
// (FA5-Q1), so only selected queries use this mode (FA6-N2).
func Planned(q Querier) Querier {
	return planned{q}
}

type planned struct {
	q Querier
}

func (p planned) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return p.q.Exec(ctx, sql, append([]any{pgx.QueryExecModeDescribeExec}, args...)...)
}

func (p planned) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.q.Query(ctx, sql, append([]any{pgx.QueryExecModeDescribeExec}, args...)...)
}

func (p planned) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.q.QueryRow(ctx, sql, append([]any{pgx.QueryExecModeDescribeExec}, args...)...)
}

// SendBatch sends b as q does: a batch's statements are queued with their
// modes, which Planned does not choose. A batch writes rows; a statement
// that reads by an array argument is no batch's.
func (p planned) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return p.q.SendBatch(ctx, b)
}
