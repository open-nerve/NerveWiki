package migrations_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// downTo takes the schema down until the migration version is undone.
func downTo(t *testing.T, m *postgres.Migrator, version int64) {
	t.Helper()
	for {
		back, err := m.Down(context.Background())
		if err != nil || back == nil {
			t.Fatalf("Down() = %+v, %v; want down to %05d", back, err, version-1)
		}
		if back.Version == version {
			return
		}
	}
}

// 00039 gives a queued import the changeset it will write (M8/P4 design
// 3.2): its id decided as a new row's is, which its first unit that writes
// makes the changeset with; an import that ran or ended keeps none. The
// schema goes down to 00038, the jobs are written as the program before
// wrote them, and up again. Down again with a restore's job and an import
// that has its changeset (M8/P3A review A-4): the restore goes, the import
// stays without it.
func TestTheQueuedImportsGetTheirChangesets(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	downTo(t, m, 39)

	const (
		user = "'0199a2b4-0000-7000-8000-000000000001'"
		ws   = "'0199a2b4-0000-7000-8000-000000000002'"
		nb   = "'0199a2b4-0000-7000-8000-000000000003'"
		nb2  = "'0199a2b4-0000-7000-8000-000000000004'"

		queued   = "0199a2b4-0000-7000-8000-000000000010" // an import queued
		ended    = "0199a2b4-0000-7000-8000-000000000011" // an import that succeeded
		exported = "0199a2b4-0000-7000-8000-000000000012" // an export queued
	)
	job := func(id, notebook, kind, state string) string {
		ended := "NULL, NULL, NULL, NULL"
		if state != "queued" {
			ended = "now(), now(), now(), '{}'"
		}
		return "INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, started_at, heartbeat_at, " +
			"finished_at, report, created_at) VALUES ('" + id + "', " + notebook + ", '" + kind + "', '" + state + "', 'n', " + user +
			", 'web', " + ended + ", now())"
	}
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (" + user +
			", 'a@corp.com', 'x', 'a', now(), now())",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + ws +
			", 'acme', 'Acme', " + user + ", " + user + ", now(), now())",
		"INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at) " +
			"VALUES (" + nb + ", " + ws + ", 'Live', 'editor', " + user + ", " + user + ", now(), now()), (" + nb2 + ", " + ws +
			", 'Other', 'editor', " + user + ", " + user + ", now(), now())",
		job(queued, nb, "import", "queued"),
		job(ended, nb, "import", "succeeded"),
		job(exported, nb, "export", "queued"),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}

	type changeset struct {
		ID        string
		Changeset *string
	}
	read := func() []changeset {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT id::text, changeset_id::text FROM transfer_jobs ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[changeset])
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	got := read()
	if len(got) != 3 || got[0].ID != queued || got[0].Changeset == nil || got[1].Changeset != nil || got[2].Changeset != nil {
		t.Fatalf("after Up, the jobs' changesets %+v; want the queued import's alone", got)
	}
	// A new one, the program's: a queued revert in the notebook the queued
	// import holds is one writing job too many.
	revert := "INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, created_at, changeset_id, " +
		"target_changeset_id) VALUES ('0199a2b4-0000-7000-8000-000000000013', " + nb + ", 'revert', 'queued', 'import', " + user +
		", 'web', now(), gen_random_uuid(), gen_random_uuid())"
	if _, err := pool.Exec(ctx, revert); err == nil {
		t.Errorf("a revert queued beside the queued import was written, want the writing key's refusal")
	}
	for _, stmt := range []string{
		"INSERT INTO transfer_jobs (id, notebook_id, root_id, kind, state, name, created_by_id, client, created_at, changeset_id) " +
			"VALUES ('0199a2b4-0000-7000-8000-000000000014', " + nb2 + ", gen_random_uuid(), 'trash_restore', 'queued', 'x', " + user +
			", 'web', now(), gen_random_uuid())",
		"UPDATE transfer_jobs SET changeset_id = gen_random_uuid() WHERE id = '" + ended + "'",
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	downTo(t, m, 39)
	rows, err := pool.Query(ctx, `SELECT id::text FROM transfer_jobs ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != queued || ids[1] != ended || ids[2] != exported {
		t.Errorf("after Down, the jobs %q; want the imports and the export, the restore gone", ids)
	}
	// The import's key again: a second import queued in the notebook is
	// refused.
	if _, err := pool.Exec(ctx, job("0199a2b4-0000-7000-8000-000000000015", nb, "import", "queued")); err == nil {
		t.Errorf("a second import queued after Down was written, want the importing key's refusal")
	}
}
