package bootstrap

import (
	"context"
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
	for _, p := range purgers(pool) {
		tables = append(tables, p.Table)
	}
	return tables
}

// queryStrings runs a query of one text column on pool.
func queryStrings(t *testing.T, pool *pgxpool.Pool, query string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), query)
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
// it: a parent row goes after its children, which a purger may have had to
// skip for a run.
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

// The purge runs as the server's jobs start (M2/P4 design 3.4): what was
// deleted longer than the retention ago goes, the workspace with its
// members and invitations, and an invitation withdrawn that long ago from
// a live workspace; what was deleted since stays.
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
	got := queryStrings(t, pool, `SELECT w.slug || ' ' || (SELECT count(*) FROM workspace_members m WHERE m.workspace_id = w.id) || ' ' ||
		(SELECT count(*) FROM workspace_invitations i WHERE i.workspace_id = w.id) FROM workspaces w ORDER BY w.slug`)
	if want := []string{"live 1 1", "recent 1 1"}; !slices.Equal(got, want) || count(t, pool, "SELECT count(*) FROM workspace_members") != 2 ||
		count(t, pool, "SELECT count(*) FROM workspace_invitations") != 2 {
		t.Errorf("workspaces with their members and invitations: %q; want %q, nothing else: old and erin's withdrawn invitation purged", got, want)
	}
}
