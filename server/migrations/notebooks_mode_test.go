package migrations_test

import (
	"context"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// 00048 gives a notebook its mode and an LLM Wiki its source root (M10/P1
// design 3.2): the check takes an LLM Wiki with its source root and a
// standard notebook with one, which M11's upgrade may set before the mode.
// Down drops both columns and keeps the notebooks, an LLM Wiki read as a
// standard one.
func TestTheNotebooksModeGoesDown(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}

	const (
		user = "'0199a2b4-0000-7000-8000-000000000001'"
		ws   = "'0199a2b4-0000-7000-8000-000000000002'"
	)
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (" + user +
			", 'a@corp.com', 'x', 'a', now(), now())",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + ws +
			", 'acme', 'Acme', " + user + ", " + user + ", now(), now())",
		"INSERT INTO notebooks (id, workspace_id, name, mode, source_root_id, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000003', " + ws + ", 'Wiki', 'llm_wiki', gen_random_uuid(), " + user + ", " + user +
			", now(), now()), ('0199a2b4-0000-7000-8000-000000000004', " + ws + ", 'Notes', 'standard', gen_random_uuid(), " + user +
			", " + user + ", now(), now())",
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	// columns counts the two columns; the notebooks are both still there.
	columns := func() (columns, notebooks int) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM information_schema.columns
			WHERE table_name = 'notebooks' AND column_name IN ('mode', 'source_root_id')), (SELECT count(*) FROM notebooks)`).
			Scan(&columns, &notebooks); err != nil {
			t.Fatal(err)
		}
		return columns, notebooks
	}
	if c, n := columns(); c != 2 || n != 2 {
		t.Fatalf("before Down, %d of the columns and %d notebooks, want both columns and both notebooks", c, n)
	}

	downTo(t, m, 48)
	if c, n := columns(); c != 0 || n != 2 {
		t.Errorf("after Down, %d of the columns and %d notebooks, want no column and both notebooks", c, n)
	}
}
