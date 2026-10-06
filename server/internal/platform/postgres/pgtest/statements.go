package pgtest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CachedStatements is the names sqlc gives ("-- name: X") of the statements
// pgx has cached on pool's one connection: those it prepared, which the
// server may plan once for any arguments from their sixth run.
func CachedStatements(t testing.TB, pool *pgxpool.Pool) []string {
	t.Helper()
	if pool.Config().MaxConns != 1 {
		t.Fatalf("a pool of %d connections: the statements cached are each connection's own", pool.Config().MaxConns)
	}
	rows, err := pool.Query(context.Background(), `SELECT substring(statement FROM '^-- name: (\w+)') FROM pg_prepared_statements
		WHERE statement LIKE '-- name: %' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return names
}
