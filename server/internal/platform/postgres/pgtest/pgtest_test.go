package pgtest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

func connect(t *testing.T, url string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func TestDatabasesAreIsolated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := connect(t, pgtest.NewDatabase(t))
	b := connect(t, pgtest.NewDatabase(t))

	if _, err := a.Exec(ctx, "CREATE TABLE only_in_a (id int)"); err != nil {
		t.Fatal(err)
	}
	var found bool
	if err := b.QueryRow(ctx, "SELECT to_regclass('only_in_a') IS NOT NULL").Scan(&found); err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("a table created in one test database is visible in another")
	}
}

func TestEmptyDatabaseHasNoTables(t *testing.T) {
	t.Parallel()
	conn := connect(t, pgtest.NewEmptyDatabase(t))

	var tables int
	err := conn.QueryRow(context.Background(),
		"SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tables)
	if err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Errorf("empty database has %d tables, want 0", tables)
	}
}

func TestDatabaseIsDroppedAfterTheTest(t *testing.T) {
	ctx := context.Background()
	var url string
	var open *pgx.Conn
	t.Run("uses a database", func(t *testing.T) {
		url = pgtest.NewDatabase(t)
		// No cleanup closes this connection, so it is still open when
		// NewDatabase's cleanup drops the database: the drop must force it.
		conn, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		open = conn
	})
	if url == "" {
		t.Skip("the subtest was skipped")
	}
	if open == nil {
		t.FailNow() // the subtest could not connect and has reported why
	}

	_, err := pgx.Connect(ctx, url)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "3D000" { // invalid_catalog_name
		t.Errorf("connect after the test = %v, want database does not exist (3D000)", err)
	}
	_ = open.Close(ctx) // only now, after the drop has been asserted
}
