package migrations_test

import (
	"context"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// 00055 gives a notebook its upgrade under way, 00056 the upgrade's
// baseline and a changeset the upgrade it was written in (M11/P1 design
// 3.2), 00057 the wiki module's row of the upgrade (3.10). Down goes back
// one at a time: 00057's drops the wiki module's table and keeps the
// rest; 00056's drops the baseline and the changesets' column and keeps
// the changeset; 00055's drops the four columns and keeps the notebook,
// standard as it was, read as one by the program before. Up again
// rebuilds them.
func TestTheUpgradeColumnsGoDown(t *testing.T) {
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
		user     = "'0199a2b4-0000-7000-8000-000000000001'"
		ws       = "'0199a2b4-0000-7000-8000-000000000002'"
		notebook = "'0199a2b4-0000-7000-8000-000000000003'"
		upgrade  = "'0199a2b4-0000-7000-8000-000000000004'"
		node     = "'0199a2b4-0000-7000-8000-000000000005'"
	)
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (" + user +
			", 'a@corp.com', 'x', 'a', now(), now())",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + ws +
			", 'acme', 'Acme', " + user + ", " + user + ", now(), now())",
		"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at, upgrade_id, upgrade_state, " +
			"upgrade_by_id, upgrade_at) VALUES (" + notebook + ", " + ws + ", 'Notes', " + user + ", " + user + ", now(), now(), " + upgrade +
			", 'upgrading', " + user + ", now())",
		"INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision, content_hash, " +
			"byte_size, wanted, accepted_hash) VALUES (" + upgrade + ", " + node + ", " + notebook +
			", true, 'page', NULL, 'Guide', 0, 1, sha256('x'), 1, 0, sha256('x'))",
		"INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, upgrade_id) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000006', " + notebook + ", 'edit', 'mcp:claude-code', " + user + ", now(), now(), " + upgrade + ")",
		"INSERT INTO notebook_upgrades (id, notebook_id, state, source_root_id, seeded_root, locale, baseline_nodes, baseline_pages, " +
			"baseline_wanted, baseline_wanted_capped, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + upgrade + ", " +
			notebook + ", 'upgrading', " + node + ", false, 'en', 1, 1, 0, false, " + user + ", " + user + ", now(), now())",
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	// shape counts the four columns of notebooks, changesets' column, the
	// baseline's table and the wiki module's; the notebooks, standard, and
	// the changesets.
	type shape struct{ columns, changesetColumn, tables, wikiTables, standard, changesets int }
	read := func() shape {
		t.Helper()
		var s shape
		if err := pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM information_schema.columns WHERE table_name = 'notebooks'
				AND column_name IN ('upgrade_id', 'upgrade_state', 'upgrade_by_id', 'upgrade_at')),
			(SELECT count(*) FROM information_schema.columns WHERE table_name = 'changesets' AND column_name = 'upgrade_id'),
			(SELECT count(*) FROM information_schema.tables WHERE table_name = 'upgrade_nodes'),
			(SELECT count(*) FROM information_schema.tables WHERE table_name = 'notebook_upgrades'),
			(SELECT count(*) FROM notebooks WHERE mode = 'standard'), (SELECT count(*) FROM changesets)`).
			Scan(&s.columns, &s.changesetColumn, &s.tables, &s.wikiTables, &s.standard, &s.changesets); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if got, want := read(), (shape{4, 1, 1, 1, 1, 1}); got != want {
		t.Fatalf("before Down, %+v, want %+v", got, want)
	}

	downTo(t, m, 57)
	if got, want := read(), (shape{4, 1, 1, 0, 1, 1}); got != want {
		t.Errorf("after 00057's Down, %+v, want %+v: the wiki module's table gone, the rest kept", got, want)
	}
	downTo(t, m, 56)
	if got, want := read(), (shape{4, 0, 0, 0, 1, 1}); got != want {
		t.Errorf("after 00056's Down, %+v, want %+v: the baseline and the changesets' column gone, the changeset kept", got, want)
	}
	downTo(t, m, 55)
	if got, want := read(), (shape{0, 0, 0, 0, 1, 1}); got != want {
		t.Errorf("after 00055's Down, %+v, want %+v: the four columns gone, the notebook kept and standard", got, want)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up() again: %v", err)
	}
	if got, want := read(), (shape{4, 1, 1, 1, 1, 1}); got != want {
		t.Errorf("after Up again, %+v, want %+v", got, want)
	}
}

// 00058 gives a changeset the rollback's kind, 00059 a job the rollback's
// kind, its upgrade and its place among the writing jobs (M11/P1 design
// 3.17). Down goes back one at a time: 00059's deletes the rollbacks' jobs
// and keeps the rest, the column and the rollback gone from the writing
// key and from the jobs' kind and root checks; 00058's makes the
// rollbacks' changesets edits, their writes kept, as 00034's Down does,
// and the rollback gone from the changesets' kind check (M11/P1 S3 review
// B-N1). Up again takes a rollback's job and changeset.
func TestTheRollbackKindsGoDown(t *testing.T) {
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
		user     = "'0199a2b4-0000-7000-8000-000000000001'"
		ws       = "'0199a2b4-0000-7000-8000-000000000002'"
		notebook = "'0199a2b4-0000-7000-8000-000000000003'"
		upgrade  = "'0199a2b4-0000-7000-8000-000000000004'"
	)
	job := func(kind, columns, values string) string {
		return "INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, finished_at, report, created_at, " +
			"changeset_id" + columns + ") VALUES (gen_random_uuid(), " + notebook + ", '" + kind + "', 'failed', 'n', " + user +
			", 'web', now(), '{}', now(), gen_random_uuid()" + values + ")"
	}
	rollbackChangeset := "INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, message, " +
		"upgrade_id) VALUES (gen_random_uuid(), " + notebook + ", 'rollback', 'web', " + user + ", now(), now(), 'Upgrade rollback', " +
		upgrade + ")"
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (" + user +
			", 'a@corp.com', 'x', 'a', now(), now())",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + ws +
			", 'acme', 'Acme', " + user + ", " + user + ", now(), now())",
		"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + notebook +
			", " + ws + ", 'Notes', " + user + ", " + user + ", now(), now())",
		job("rollback", ", upgrade_id", ", "+upgrade),
		job("import", "", ""),
		rollbackChangeset,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	// shape counts the jobs by kind, the jobs' upgrade column, the writing
	// key's rollback, the changesets by kind and the checks that take the
	// rollback: the jobs' kind and root, the changesets' kind.
	type shape struct{ rollbackJobs, imports, column, writingRollback, rollbackChangesets, edits, checks int }
	read := func() shape {
		t.Helper()
		var s shape
		if err := pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM transfer_jobs WHERE kind = 'rollback'), (SELECT count(*) FROM transfer_jobs WHERE kind = 'import'),
			(SELECT count(*) FROM information_schema.columns WHERE table_name = 'transfer_jobs' AND column_name = 'upgrade_id'),
			(SELECT count(*) FROM pg_indexes WHERE indexname = 'transfer_jobs_writing_key' AND indexdef LIKE '%rollback%'),
			(SELECT count(*) FROM changesets WHERE kind = 'rollback'), (SELECT count(*) FROM changesets WHERE kind = 'edit'),
			(SELECT count(*) FROM pg_constraint WHERE conname IN ('transfer_jobs_kind_check', 'transfer_jobs_root_check',
				'changesets_kind_check') AND pg_get_constraintdef(oid) LIKE '%rollback%')`).
			Scan(&s.rollbackJobs, &s.imports, &s.column, &s.writingRollback, &s.rollbackChangesets, &s.edits, &s.checks); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if got, want := read(), (shape{1, 1, 1, 1, 1, 0, 3}); got != want {
		t.Fatalf("before Down, %+v, want %+v", got, want)
	}
	downTo(t, m, 59)
	if got, want := read(), (shape{0, 1, 0, 0, 1, 0, 1}); got != want {
		t.Errorf("after 00059's Down, %+v, want %+v: the rollback's job, its column and its place in the writing key and the jobs' "+
			"checks gone, the import kept", got, want)
	}
	downTo(t, m, 58)
	if got, want := read(), (shape{0, 1, 0, 0, 0, 1, 0}); got != want {
		t.Errorf("after 00058's Down, %+v, want %+v: the rollback's changeset an edit, the changesets' check without it", got, want)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up() again: %v", err)
	}
	for _, stmt := range []string{job("rollback", ", upgrade_id", ", "+upgrade), rollbackChangeset} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Errorf("after Up again, %s: %v", stmt, err)
		}
	}
	if got, want := read(), (shape{1, 1, 1, 1, 1, 1, 3}); got != want {
		t.Errorf("after Up again, %+v, want %+v", got, want)
	}
}
