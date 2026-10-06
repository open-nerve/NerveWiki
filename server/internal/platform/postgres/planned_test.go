package postgres_test

import (
	"context"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// A statement run through Planned is never pgx's cached one, in a
// transaction or not: the server plans it with its arguments each time,
// through Exec, Query and QueryRow alike (M6 closeout FA4-M1). Run as they
// are, the same statements are cached.
func TestPlannedStatementsAreNotCached(t *testing.T) {
	ctx := context.Background()
	pool := newNotes(t, 1)
	cached := func() int {
		var n int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_prepared_statements
			WHERE statement LIKE '%notes%' AND statement NOT LIKE '%pg_prepared_statements%'`).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	run := func(q postgres.Querier) {
		for i := range 8 {
			if _, err := q.Exec(ctx, "INSERT INTO notes (id) VALUES ($1) ON CONFLICT DO NOTHING", i); err != nil {
				t.Fatal(err)
			}
			rows, err := q.Query(ctx, "SELECT id FROM notes WHERE id = ANY($1::int[])", []int32{int32(i)})
			if err != nil {
				t.Fatal(err)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			var n int
			if err := q.QueryRow(ctx, "SELECT count(*) FROM notes WHERE id <= $1", i).Scan(&n); err != nil {
				t.Fatal(err)
			}
		}
	}
	run(postgres.Planned(pool))
	err := postgres.NewTxManager(pool, commitTimeout).WithinTx(ctx, func(ctx context.Context) error {
		run(postgres.Planned(postgres.DB(ctx, pool)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := cached(); n != 0 {
		t.Fatalf("%d statements on notes cached through Planned, want none", n)
	}
	run(pool)
	if n := cached(); n != 3 {
		t.Fatalf("%d statements on notes cached as they are, want 3", n)
	}
}
