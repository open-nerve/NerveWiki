package postgres_test

import (
	"context"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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

// The pool sends no parameter at startup but those database.url and the
// PG* environment variables ask for: a connection pooler refuses those it
// does not track (M6 closeout FA5-M2, FA7-N2). What a connection needs set
// it sets once it starts (afterConnect).
func TestNewPoolSendsNoStartupParameters(t *testing.T) {
	const url = "postgres://nervewiki:secret@127.0.0.1:1/nervewiki?application_name=wiki"
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: url, MaxConns: 1})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()
	asked, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pool.Config().ConnConfig.RuntimeParams, asked.ConnConfig.RuntimeParams; !maps.Equal(got, want) {
		t.Errorf("startup parameters %v, want %v", got, want)
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

// Every connection of the pool, the one a listener takes for its own too,
// has jit off (M6 closeout FA5-M1).
func TestPoolTurnsJITOffOnEachConnection(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	jit := func(q interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}) string {
		var v string
		if err := q.QueryRow(ctx, "SHOW jit").Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	conns := make([]*pgxpool.Conn, pool.Config().MaxConns)
	for i := range conns {
		c, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns[i] = c
		if v := jit(c); v != "off" {
			t.Errorf("connection %d has jit %s, want off", i+1, v)
		}
	}
	own := conns[0].Hijack()
	t.Cleanup(func() {
		if err := own.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	if v := jit(own); v != "off" {
		t.Errorf("a connection taken from the pool has jit %s, want off", v)
	}
	for _, c := range conns[1:] {
		c.Release()
	}
}
