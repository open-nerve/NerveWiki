package postgres_test

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// sampleMigrations is a test-only migration set, independent of the
// production schema (whose own up/down test is in server/migrations).
func sampleMigrations() fstest.MapFS {
	return fstest.MapFS{
		"00001_probe_create_widgets.sql": {Data: []byte(`-- +goose Up
CREATE TABLE widgets (id bigint PRIMARY KEY);

-- +goose Down
DROP TABLE widgets;
`)},
		"00002_probe_add_color.sql": {Data: []byte(`-- +goose Up
ALTER TABLE widgets ADD COLUMN color text;

-- +goose Down
ALTER TABLE widgets DROP COLUMN color;
`)},
	}
}

func newPool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: url, MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newMigrator(t *testing.T, pool *pgxpool.Pool, fsys fstest.MapFS) *postgres.Migrator {
	t.Helper()
	m, err := postgres.NewMigrator(pool, fsys)
	if err != nil {
		t.Fatalf("NewMigrator() error = %v", err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return m
}

func hasColumn(t *testing.T, pool *pgxpool.Pool, table, column string) bool {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2",
		table, column).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func applied(t *testing.T, m *postgres.Migrator) []bool {
	t.Helper()
	statuses, err := m.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	out := make([]bool, len(statuses))
	for i, s := range statuses {
		out[i] = s.Applied
		if s.Applied == s.AppliedAt.IsZero() {
			t.Errorf("status %+v: AppliedAt must be set exactly when applied", s)
		}
	}
	return out
}

func TestMigratorUpStatusDown(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	m := newMigrator(t, pool, sampleMigrations())

	if got := applied(t, m); len(got) != 2 || got[0] || got[1] {
		t.Fatalf("applied before Up = %v, want [false false]", got)
	}
	if err := m.CheckUpToDate(ctx); !errors.Is(err, postgres.ErrPendingMigrations) {
		t.Errorf("CheckUpToDate() before Up = %v, want ErrPendingMigrations", err)
	}

	ran, err := m.Up(ctx)
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	want := []postgres.Migration{
		{Version: 1, Source: "00001_probe_create_widgets.sql"},
		{Version: 2, Source: "00002_probe_add_color.sql"},
	}
	if len(ran) != 2 || ran[0] != want[0] || ran[1] != want[1] {
		t.Errorf("Up() = %+v, want %+v", ran, want)
	}
	if !hasColumn(t, pool, "widgets", "color") {
		t.Error("widgets.color missing after Up")
	}
	if got := applied(t, m); !got[0] || !got[1] {
		t.Errorf("applied after Up = %v, want [true true]", got)
	}
	if err := m.CheckUpToDate(ctx); err != nil {
		t.Errorf("CheckUpToDate() after Up = %v, want nil", err)
	}
	if again, err := m.Up(ctx); err != nil || len(again) != 0 {
		t.Errorf("second Up() = %+v, %v; want nothing to do", again, err)
	}

	back, err := m.Down(ctx)
	if err != nil || back == nil || *back != want[1] {
		t.Fatalf("Down() = %+v, %v; want %+v", back, err, want[1])
	}
	if hasColumn(t, pool, "widgets", "color") {
		t.Error("widgets.color still present after Down")
	}
	if got := applied(t, m); !got[0] || got[1] {
		t.Errorf("applied after Down = %v, want [true false]", got)
	}

	if back, err := m.Down(ctx); err != nil || back == nil || *back != want[0] {
		t.Fatalf("second Down() = %+v, %v; want %+v", back, err, want[0])
	}
	if back, err := m.Down(ctx); err != nil || back != nil {
		t.Errorf("Down() with nothing applied = %+v, %v; want nil, nil", back, err)
	}
}

func TestMigratorReportsFailingMigration(t *testing.T) {
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	m := newMigrator(t, pool, fstest.MapFS{
		"00001_probe_create_widgets.sql": sampleMigrations()["00001_probe_create_widgets.sql"],
		"00002_probe_broken.sql":         {Data: []byte("-- +goose Up\nCREATE TABLE broken (;\n")},
	})

	ran, err := m.Up(context.Background())
	if err == nil {
		t.Error("Up() error = nil, want the SQL error")
	}
	want := postgres.Migration{Version: 1, Source: "00001_probe_create_widgets.sql"}
	if len(ran) != 1 || ran[0] != want {
		t.Errorf("Up() applied %+v, want only %+v, the migration before the failure", ran, want)
	}
	if !hasColumn(t, pool, "widgets", "id") {
		t.Error("widgets table missing: the migration before the failure must stay applied")
	}
}

// The production set is never empty (server/migrations tests it), so an
// empty set is a wiring mistake.
func TestNewMigratorRejectsAnEmptySet(t *testing.T) {
	pool := newPool(t, "postgres://nobody@127.0.0.1:1/nowhere")

	if _, err := postgres.NewMigrator(pool, fstest.MapFS{".gitkeep": {}}); err == nil {
		t.Error("NewMigrator() error = nil, want an error for no migrations")
	}
}

// Processes that migrate one database at once, such as several instances
// starting with auto_migrate, take turns on a lock: each migration is
// applied once, and neither process fails. The database is already at
// version 1, and migration 2 takes half a second, so the two runs overlap:
// without the lock, both would see it pending and the second would fail on
// the table the first created.
func TestConcurrentUpAppliesEachMigrationOnce(t *testing.T) {
	url := pgtest.NewEmptyDatabase(t)
	first := fstest.MapFS{"00001_probe_create_widgets.sql": sampleMigrations()["00001_probe_create_widgets.sql"]}
	if _, err := newMigrator(t, newPool(t, url), first).Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	both := fstest.MapFS{
		"00001_probe_create_widgets.sql": first["00001_probe_create_widgets.sql"],
		"00002_probe_slow_gadgets.sql": {Data: []byte(
			"-- +goose Up\nSELECT pg_sleep(0.5);\nCREATE TABLE gadgets (id bigint);\n-- +goose Down\nDROP TABLE gadgets;\n")},
	}
	type result struct {
		ran []postgres.Migration
		err error
	}
	start, results := make(chan struct{}), make(chan result, 2)
	for range 2 {
		m := newMigrator(t, newPool(t, url), both)
		go func() {
			<-start
			ran, err := m.Up(context.Background())
			results <- result{ran, err}
		}()
	}
	close(start)

	applied := 0
	for range 2 {
		r := <-results
		if r.err != nil {
			t.Errorf("Up() error = %v, want both processes to succeed", r.err)
		}
		applied += len(r.ran)
	}
	if applied != 1 {
		t.Errorf("the two Up() calls applied %d migrations between them, want migration 2 once", applied)
	}
}

func TestCheckUpToDateFailsWhenDatabaseIsUnreachable(t *testing.T) {
	pool := newPool(t, "postgres://nobody@127.0.0.1:1/nowhere")
	m := newMigrator(t, pool, sampleMigrations())

	err := m.CheckUpToDate(context.Background())
	if err == nil || errors.Is(err, postgres.ErrPendingMigrations) {
		t.Errorf("CheckUpToDate() = %v, want a connection error", err)
	}
}
