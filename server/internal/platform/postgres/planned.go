package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Planned is q running each statement planned with its arguments: sent as
// the unnamed statement, its description cached, never pgx's cached
// statement. From a cached statement's sixth run the server may plan it
// once for any arguments, and such a plan compares each row with an array
// argument's elements one by one where a plan with the array hashes them:
// a subtree's level of 50,000 parents took 3.5 s (M6 closeout FA4-M1).
// Planning costs each run a little, about a third of a rename's or a
// move's statements had every statement been planned so (FA5-Q1): it is
// for the statements whose arrays grow with the data.
func Planned(q Querier) Querier {
	return planned{q}
}

type planned struct {
	q Querier
}

func (p planned) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return p.q.Exec(ctx, sql, append([]any{pgx.QueryExecModeCacheDescribe}, args...)...)
}

func (p planned) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.q.Query(ctx, sql, append([]any{pgx.QueryExecModeCacheDescribe}, args...)...)
}

func (p planned) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.q.QueryRow(ctx, sql, append([]any{pgx.QueryExecModeCacheDescribe}, args...)...)
}
