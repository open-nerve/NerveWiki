package migrations_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The schema's objects: tables (but goose's own) and extensions (but the
// built-in plpgsql).
const (
	tablesQuery     = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name <> 'goose_db_version' ORDER BY 1"
	extensionsQuery = "SELECT extname FROM pg_extension WHERE extname <> 'plpgsql' ORDER BY 1"
)

func names(t *testing.T, pool *pgxpool.Pool, query string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		found = append(found, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return found
}

// Every migration can go up, down and up again.
func TestMigrationsGoUpDownAndUpAgain(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewEmptyDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })

	up, err := m.Up(ctx)
	if err != nil || len(up) != 1 {
		t.Fatalf("Up() = %d migrations, %v; want 1", len(up), err)
	}
	for _, want := range []struct {
		query string
		names []string
	}{
		{tablesQuery, nil},
		{extensionsQuery, []string{"pg_trgm"}},
	} {
		if got := names(t, pool, want.query); !slices.Equal(got, want.names) {
			t.Errorf("after Up, %s = %q, want %q", want.query, got, want.names)
		}
	}
	for range up {
		if _, err := m.Down(ctx); err != nil {
			t.Fatalf("Down() error = %v", err)
		}
	}
	for _, query := range []string{tablesQuery, extensionsQuery} {
		if got := names(t, pool, query); len(got) != 0 {
			t.Errorf("after every Down, %s = %q, want none", query, got)
		}
	}
	if again, err := m.Up(ctx); err != nil || len(again) != 1 {
		t.Errorf("Up() again = %d migrations, %v; want 1", len(again), err)
	}
}
