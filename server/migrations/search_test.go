package migrations_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// 00040 queues every live node for the search's job, in bulk, each with a
// version of its own and its notebook; a node in the trash is not queued
// (M8/P5 design 3.2). The schema goes down to 00039, the nodes are written
// as the program before wrote them, and up again; then down again with a
// row built and the queue as it was, both tables, the sequence and the
// function of the body tier's candidates gone.
func TestTheLiveNodesAreQueuedForTheSearch(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	downTo(t, m, 40)

	const (
		user = "'0199a2b4-0000-7000-8000-000000000001'"
		ws   = "'0199a2b4-0000-7000-8000-000000000002'"
		nb1  = "0199a2b4-0000-7000-8000-000000000003"
		nb2  = "0199a2b4-0000-7000-8000-000000000004"

		p = "0199a2b4-0000-7000-8000-000000000010" // a page of nb1
		x = "0199a2b4-0000-7000-8000-000000000011" // an attachment of nb1
		d = "0199a2b4-0000-7000-8000-000000000012" // a page of nb1 in the trash
		q = "0199a2b4-0000-7000-8000-000000000013" // a page of nb2
	)
	notebook := func(id, name string) string {
		return "INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at) " +
			"VALUES ('" + id + "', " + ws + ", '" + name + "', 'editor', " + user + ", " + user + ", now(), now())"
	}
	node := func(id, nb, kind, name, deleted string) string {
		return "INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, " +
			"created_at, updated_at, deleted_at) VALUES ('" + id + "', '" + nb + "', NULL, '" + kind + "', '" + name + "', '" + name +
			"', 0, " + user + ", " + user + ", now(), now(), " + deleted + ")"
	}
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (" + user +
			", 'a@corp.com', 'x', 'a', now(), now())",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + ws +
			", 'acme', 'Acme', " + user + ", " + user + ", now(), now())",
		notebook(nb1, "One"), notebook(nb2, "Two"),
		node(p, nb1, "page", "p", "NULL"),
		node(x, nb1, "asset", "x.png", "NULL"),
		node(d, nb1, "page", "d", "now()"),
		node(q, nb2, "page", "q", "NULL"),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}

	type queued struct {
		Node, Notebook string
		Version        int64
		Bulk           bool
	}
	rows, err := pool.Query(ctx, `SELECT node_id::text, notebook_id::text, version, bulk FROM page_search_queue ORDER BY node_id`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[queued])
	if err != nil {
		t.Fatal(err)
	}
	// The live nodes, pages and attachments of both notebooks, in bulk; not d.
	want := []queued{{Node: p, Notebook: nb1}, {Node: x, Notebook: nb1}, {Node: q, Notebook: nb2}}
	versions := map[int64]bool{}
	if len(got) != len(want) {
		t.Fatalf("queued = %+v, want p, x and q", got)
	}
	for i, w := range want {
		g := got[i]
		if g.Node != w.Node || g.Notebook != w.Notebook || !g.Bulk {
			t.Errorf("queued %+v, want %s of %s in bulk", g, w.Node, w.Notebook)
		}
		versions[g.Version] = true
	}
	if len(versions) != len(want) {
		t.Errorf("queued versions %+v: want one each", got)
	}
	var built int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM page_search").Scan(&built); err != nil || built != 0 {
		t.Errorf("page_search holds %d rows (%v), want none: the job builds them", built, err)
	}
	if hits := function(t, pool, "page_search_hits(tsquery, tsquery)"); !hits {
		t.Error("page_search_hits(tsquery, tsquery) is missing after Up")
	}

	// Down again with a row built and the queue as it was: both tables and
	// their sequence go.
	if _, err := pool.Exec(ctx, "INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body) "+
		"VALUES ($1, 1, 'q', '1/17.0.0', now(), 'q', 'q')", q); err != nil {
		t.Fatal(err)
	}
	downTo(t, m, 40)
	var left []string
	rows, err = pool.Query(ctx, `SELECT relname::text FROM pg_class WHERE relnamespace = 'public'::regnamespace AND relname LIKE 'page\_search%' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	if left, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil || len(left) != 0 {
		t.Errorf("after Down, %q (%v) left; want no table, index or sequence of the search", left, err)
	}
	if function(t, pool, "page_search_hits(tsquery, tsquery)") {
		t.Error("page_search_hits(tsquery, tsquery) left after Down")
	}
}

// function reports whether the function of signature exists.
func function(t *testing.T, pool *pgxpool.Pool, signature string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(), "SELECT to_regprocedure($1) IS NOT NULL", signature).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}
