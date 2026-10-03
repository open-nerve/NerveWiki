package pgtest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TerminateListeners ends, with pg_terminate_backend, every backend of
// pool's database whose last statement was LISTEN on channel, as a
// listener's idle connection is, and returns how many it ended. A test
// cuts a listener's connection with it to see the listener reconnect. It
// fails the test when it cannot ask within limit.
func TerminateListeners(t testing.TB, pool *pgxpool.Pool, channel string, limit time.Duration) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM (
		SELECT pg_terminate_backend(pid) FROM pg_stat_activity
		WHERE datname = current_database() AND pid <> pg_backend_pid() AND query = $1) ended`,
		"LISTEN "+pgx.Identifier{channel}.Sanitize()).Scan(&n)
	if err != nil {
		t.Fatalf("pgtest: terminate the listeners on %s: %v", channel, err)
	}
	return n
}
