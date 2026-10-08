package bootstrap

import (
	"context"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The purge's registry against the schema (M2/P4 design 3.4): what a
// module's migration adds without its purger, or a purger listed after the
// table it references, fails here, before any row is lost or kept.

// purgedTables are the tables of the registry, in its order.
func purgedTables(pool *pgxpool.Pool) []string {
	var tables []string
	for _, p := range purgers(pool, nil, nil, nil) {
		tables = append(tables, p.Table)
	}
	return tables
}

// queryStrings runs a query of one text column on pool.
func queryStrings(t *testing.T, pool *pgxpool.Pool, query string, args ...any) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// Every table with deleted_at has exactly one purger (v0.1 design 13.1,
// item 6), and every purger's table has deleted_at.
func TestEverySoftDeletedTableHasAPurger(t *testing.T) {
	pool := connect(t, pgtest.NewDatabase(t))
	tables := purgedTables(pool)
	softDeleted := queryStrings(t, pool, `SELECT table_name FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'deleted_at' ORDER BY table_name`)

	sorted := slices.Sorted(slices.Values(tables))
	if !slices.Equal(sorted, slices.Compact(slices.Clone(sorted))) || !slices.Equal(sorted, softDeleted) {
		t.Errorf("purgers of %q; want one for each table with deleted_at, %q", tables, softDeleted)
	}
}

// Every foreign key into a purged table comes from a table purged before
// it: a parent row goes after its children, or, when a purger skipped one
// of them, in a later run with it.
func TestPurgersComeBeforeTheTablesTheyReference(t *testing.T) {
	pool := connect(t, pgtest.NewDatabase(t))
	tables := purgedTables(pool)
	keys := queryStrings(t, pool, `SELECT conrelid::regclass::text || ' ' || confrelid::regclass::text FROM pg_constraint
		WHERE contype = 'f' AND connamespace = 'public'::regnamespace AND conrelid <> confrelid ORDER BY 1`)

	checked := 0
	for _, key := range keys {
		child, parent, _ := strings.Cut(key, " ")
		at := slices.Index(tables, parent)
		if at < 0 {
			continue
		}
		checked++
		if from := slices.Index(tables, child); from < 0 || from > at {
			t.Errorf("%s references %s, which is purged at %d; want %s purged before it, in %q", child, parent, at, child, tables)
		}
	}
	if checked == 0 {
		t.Error("no foreign key into a purged table: the rule checks nothing")
	}
}

// A foreign key into a purged table from another module's table is ON
// DELETE RESTRICT (v0.1 design 13.1, item 6): each row goes by its own
// module's purger, never by a cascade from another module's, which the
// purger of the child's module would not know of.
func TestCrossModuleForeignKeysToPurgedTablesRestrict(t *testing.T) {
	pool := connect(t, pgtest.NewDatabase(t))
	owners := tableOwners(t)
	tables := purgedTables(pool)
	keys := queryStrings(t, pool, `SELECT conname || ' ' || conrelid::regclass::text || ' ' || confrelid::regclass::text || ' ' ||
		confdeltype::text FROM pg_constraint WHERE contype = 'f' AND connamespace = 'public'::regnamespace ORDER BY 1`)

	checked := 0
	for _, key := range keys {
		f := strings.Fields(key) // the constraint, the child, the parent, ON DELETE
		name, child, parent, onDelete := f[0], f[1], f[2], f[3]
		if owners[child] == "" || owners[parent] == "" {
			t.Errorf("%s: no migration creates %s or %s", name, child, parent)
			continue
		}
		if !slices.Contains(tables, parent) || owners[child] == owners[parent] {
			continue
		}
		checked++
		if onDelete != "r" {
			t.Errorf("%s: %s (%s) references %s (%s) with ON DELETE %s; want RESTRICT (r)",
				name, child, owners[child], parent, owners[parent], onDelete)
		}
	}
	if checked == 0 {
		t.Error("no foreign key from one module's table into another's purged table: the rule checks nothing")
	}
}

// tableOwners maps each table a migration creates to the owner in the
// migration's name, NNNNN_<owner>_<description>.sql: a table's module.
func tableOwners(t *testing.T) map[string]string {
	t.Helper()
	files, err := fs.Glob(migrations.FS(), "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	creates := regexp.MustCompile(`(?m)^CREATE TABLE (\w+)`)
	owners := make(map[string]string)
	for _, file := range files {
		parts := strings.SplitN(file, "_", 3)
		sql, err := fs.ReadFile(migrations.FS(), file)
		if err != nil || len(parts) != 3 {
			t.Fatalf("%s: %v", file, err)
		}
		for _, m := range creates.FindAllStringSubmatch(string(sql), -1) {
			owners[m[1]] = parts[1]
		}
	}
	return owners
}

// The purge runs as the server's jobs start (M2/P4 design 3.4): what was
// deleted longer than the retention ago goes, the workspace with its
// members, invitations and notebooks, an invitation withdrawn and a
// notebook deleted that long ago in a live workspace; what was deleted
// since stays. The notebooks go before their workspace, which their
// foreign key restricts; their pages, a tree of three levels each with
// what follows them, before the notebooks (M4/P1 design 3.10), and so does
// a subtree deleted that long ago in a live notebook. The run has no
// error: one that failed, and passed on a retry, would hide a purger that
// leaves what the next needs gone.
func TestThePurgeDeletesWhatOutlivedTheRetention(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO users (id, email, password, display_name, created_at, updated_at)
			VALUES ('0199a2b4-0000-7000-8000-0000000000a1', 'alice@example.com', 'x', 'Alice', now(), now())`,
		`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at, deleted_at) VALUES
			('0199a2b4-0000-7000-8000-0000000000b1', 'old', 'Old', '0199a2b4-0000-7000-8000-0000000000a1', '0199a2b4-0000-7000-8000-0000000000a1', now(), now(), now() - interval '61 days'),
			('0199a2b4-0000-7000-8000-0000000000b2', 'recent', 'Recent', '0199a2b4-0000-7000-8000-0000000000a1', '0199a2b4-0000-7000-8000-0000000000a1', now(), now(), now() - interval '59 days'),
			('0199a2b4-0000-7000-8000-0000000000b3', 'live', 'Live', '0199a2b4-0000-7000-8000-0000000000a1', '0199a2b4-0000-7000-8000-0000000000a1', now(), now(), NULL)`,
		`INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), id, created_by_id, 'admin', created_by_id, created_by_id, now(), now(), deleted_at FROM workspaces`,
		`INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), id, 'dana@example.com', 'member', created_by_id, created_by_id, now(), now(), deleted_at FROM workspaces`,
		`INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			VALUES (gen_random_uuid(), '0199a2b4-0000-7000-8000-0000000000b3', 'erin@example.com', 'member',
				'0199a2b4-0000-7000-8000-0000000000a1', '0199a2b4-0000-7000-8000-0000000000a1', now(), now(), now() - interval '61 days')`,
		`INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), id, 'Notes', created_by_id, created_by_id, now(), now(), deleted_at FROM workspaces`,
		`INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			VALUES (gen_random_uuid(), '0199a2b4-0000-7000-8000-0000000000b3', 'Gone',
				'0199a2b4-0000-7000-8000-0000000000a1', '0199a2b4-0000-7000-8000-0000000000a1', now(), now(), now() - interval '61 days')`,
		`INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), id, created_by_id, 'admin', created_by_id, created_by_id, now(), now(), deleted_at FROM notebooks`,
		// Each notebook's tree, Root, Child and Grandchild, deleted with it;
		// live's Notes has Trashed and its child, deleted 61 days ago.
		`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), id, 'page', 'Root', 'root', 0, created_by_id, created_by_id, now(), now(), deleted_at FROM notebooks`,
		`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), n.id, 'page', 'Trashed', 'trashed', 1, n.created_by_id, n.created_by_id, now(), now(), now() - interval '61 days'
			FROM notebooks n WHERE n.workspace_id = '0199a2b4-0000-7000-8000-0000000000b3' AND n.name = 'Notes'`,
		`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), notebook_id, id, 'page', 'Child', 'child', 0, created_by_id, created_by_id, now(), now(), deleted_at
			FROM nodes WHERE name IN ('Root', 'Trashed')`,
		`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), c.notebook_id, c.id, 'page', 'Grandchild', 'grandchild', 0, c.created_by_id, c.created_by_id, now(), now(), c.deleted_at
			FROM nodes c JOIN nodes p ON p.id = c.parent_id WHERE p.name = 'Root'`,
		`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at, deleted_at)
			SELECT id, '', 1, sha256(''), 0, created_by_id, now(), deleted_at FROM nodes`,
		`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), id, 'edit', 'web', created_by_id, now(), now(), deleted_at FROM notebooks`,
		`INSERT INTO changeset_items (id, changeset_id, node_id, after_name, after_sort_order, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), s.id, x.id, x.name, x.sort_order, now(), now(), x.deleted_at FROM nodes x JOIN changesets s ON s.notebook_id = x.notebook_id`,
		`INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), s.id, x.id, 1, '', sha256(''), 0, now(), now(), x.deleted_at FROM nodes x JOIN changesets s ON s.notebook_id = x.notebook_id`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	startApp(t, testConfig(t, url, false), migrations.FS())

	// River records the run completed once it has returned: then the
	// purge is over.
	for deadline := time.Now().Add(15 * time.Second); count(t, pool, "SELECT count(*) FROM river_job WHERE kind = $1 AND state = 'completed'",
		jobs.PurgeKind) == 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the purge did not complete; jobs: %v", queryStrings(t, pool, "SELECT kind || ' ' || state FROM river_job"))
		}
	}
	if errs := queryStrings(t, pool, "SELECT array_to_string(errors, ' ') FROM river_job WHERE kind = $1 AND errors IS NOT NULL", jobs.PurgeKind); len(errs) != 0 {
		t.Errorf("the purge failed before it completed: %v", errs)
	}
	got := queryStrings(t, pool, `SELECT w.slug || ' ' || (SELECT count(*) FROM workspace_members m WHERE m.workspace_id = w.id) || ' ' ||
		(SELECT count(*) FROM workspace_invitations i WHERE i.workspace_id = w.id) FROM workspaces w ORDER BY w.slug`)
	if want := []string{"live 1 1", "recent 1 1"}; !slices.Equal(got, want) || count(t, pool, "SELECT count(*) FROM workspace_members") != 2 ||
		count(t, pool, "SELECT count(*) FROM workspace_invitations") != 2 {
		t.Errorf("workspaces with their members and invitations: %q; want %q, nothing else: old and erin's withdrawn invitation purged", got, want)
	}
	got = queryStrings(t, pool, `SELECT w.slug || ' ' || n.name || ' ' || (SELECT count(*) FROM notebook_members m WHERE m.notebook_id = n.id)
		FROM notebooks n JOIN workspaces w ON w.id = n.workspace_id ORDER BY 1`)
	if want := []string{"live Notes 1", "recent Notes 1"}; !slices.Equal(got, want) || count(t, pool, "SELECT count(*) FROM notebook_members") != 2 {
		t.Errorf("notebooks with their members: %q; want %q, nothing else: old's and live's deleted one purged", got, want)
	}
	// Each page with its content, item and version: Trashed's subtree and
	// the purged notebooks' trees are gone.
	got = queryStrings(t, pool, `SELECT w.slug || ' ' || x.name || ' ' ||
			(SELECT count(*) FROM page_contents c WHERE c.node_id = x.id) ||
			(SELECT count(*) FROM changeset_items i WHERE i.node_id = x.id) ||
			(SELECT count(*) FROM page_revisions r WHERE r.node_id = x.id)
		FROM nodes x JOIN notebooks n ON n.id = x.notebook_id JOIN workspaces w ON w.id = n.workspace_id ORDER BY 1`)
	want := []string{"live Child 111", "live Grandchild 111", "live Root 111", "recent Child 111", "recent Grandchild 111", "recent Root 111"}
	if !slices.Equal(got, want) || count(t, pool, "SELECT count(*) FROM changesets") != 2 {
		t.Errorf("pages with their content, item and version: %q, %d changesets; want %q and live's and recent's changesets", got,
			count(t, pool, "SELECT count(*) FROM changesets"), want)
	}
}
