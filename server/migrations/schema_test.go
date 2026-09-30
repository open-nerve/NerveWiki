package migrations_test

import (
	"context"
	"io/fs"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// objectsQuery lists the schema's objects: tables but goose's own, and
// extensions but the built-in plpgsql.
const objectsQuery = `
	SELECT 'table ' || table_name FROM information_schema.tables
	WHERE table_schema = 'public' AND table_name <> 'goose_db_version'
	UNION ALL
	SELECT 'extension ' || extname FROM pg_extension WHERE extname <> 'plpgsql'
	ORDER BY 1`

func objects(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), objectsQuery)
	if err != nil {
		t.Fatal(err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// Every migration goes up, and down again to an empty schema; going up once
// more rebuilds the same schema.
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
	files, err := fs.ReadDir(migrations.FS(), ".")
	if err != nil {
		t.Fatal(err)
	}

	up, err := m.Up(ctx)
	if err != nil || len(up) != len(files) {
		t.Fatalf("Up() = %d migrations, %v; want all %d", len(up), err, len(files))
	}
	schema := objects(t, pool)
	if !slices.Contains(schema, "extension pg_trgm") {
		t.Errorf("after Up, the schema holds %q, want pg_trgm among it", schema)
	}
	for range up {
		if _, err := m.Down(ctx); err != nil {
			t.Fatalf("Down() error = %v", err)
		}
	}
	if left := objects(t, pool); len(left) != 0 {
		t.Errorf("after every Down, the schema holds %q, want nothing", left)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up() again: %v", err)
	}
	if again := objects(t, pool); !slices.Equal(again, schema) {
		t.Errorf("after Up again, the schema holds %q, want %q as after the first Up", again, schema)
	}
}
