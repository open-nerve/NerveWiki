package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// pgx returns timestamptz in time.Local by default; the pool returns UTC.
func TestPoolScansTimestamptzInUTC(t *testing.T) {
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	var got time.Time

	err := pool.QueryRow(context.Background(), "SELECT '2026-09-25 18:00:00+08'::timestamptz").Scan(&got)

	want := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	if err != nil || got.Location() != time.UTC || !got.Equal(want) {
		t.Errorf("scanned %v (%v), want %v in UTC", got, err, want)
	}
}

func TestNewPoolAppliesMaxConns(t *testing.T) {
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{
		URL:      "postgres://nervewiki:secret@127.0.0.1:1/nervewiki",
		MaxConns: 3,
	})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()
	if got := pool.Config().MaxConns; got != 3 {
		t.Errorf("MaxConns = %d, want 3", got)
	}
}

// pgx quotes the connection string in its parse error and masks the password
// only on a best-effort basis, e.g. not in the legal key/value form
// "password = secret"; NewPool shows none of that text, whether the string is
// malformed or names a file that cannot be read.
func TestNewPoolRejectsUnusableURLWithoutLeakingPassword(t *testing.T) {
	const want = "database.url: pgx cannot use it; check its syntax, the files it names (sslrootcert, sslcert, sslkey) " +
		"and any PG* environment variables (details not shown, as they may contain the password)"
	for _, url := range []string{
		"postgres://nervewiki:secret@localhost:notaport/nervewiki",
		"host=localhost port=1 password = secret sslmode=bogus",
		`host=localhost password=sec\ secret sslmode=bogus`,
		"postgres://nervewiki:secret@localhost/nervewiki?sslmode=verify-full&sslrootcert=/nonexistent/ca.pem",
	} {
		_, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: url, MaxConns: 1})
		if err == nil || err.Error() != want || strings.Contains(err.Error(), "secret") {
			t.Errorf("NewPool(%q) error = %v, want %q", url, err, want)
		}
	}
}

// The server plans each statement with its arguments, though pgx caches it
// (M6 closeout FA4-M1): from a cached statement's sixth run it would plan
// it once for any arguments.
func TestPoolPlansEachStatementWithItsArguments(t *testing.T) {
	ctx := context.Background()
	pool := newNotes(t, 1)
	for i := range 8 {
		rows, err := pool.Query(ctx, "SELECT id FROM notes WHERE id = $1", i)
		if err != nil {
			t.Fatal(err)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
	var generic, custom int
	err := pool.QueryRow(ctx, `SELECT generic_plans, custom_plans FROM pg_prepared_statements
		WHERE statement LIKE '%FROM notes%' AND statement NOT LIKE '%pg_prepared_statements%'`).Scan(&generic, &custom)
	if err != nil || generic != 0 || custom != 8 {
		t.Errorf("a statement run 8 times was planned for any arguments %d times, with its own %d, %v; want 8 with its own",
			generic, custom, err)
	}
}
