package migrations_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The trash's migrations mark what was deleted before them (M8/P2 design
// 3.2): each deletion's root, with its group's counts and height, but no
// node deleted with its notebook; and a page's creation items, revision 1.
// The schema goes down to 00034, the data is written as the program before
// wrote it, and up again.
func TestTheTrashMigrationsMarkWhatWasDeleted(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	for {
		back, err := m.Down(ctx)
		if err != nil || back == nil {
			t.Fatalf("Down() = %+v, %v; want down to 00034", back, err)
		}
		if back.Version == 35 {
			break
		}
	}

	const (
		user = "'0199a2b4-0000-7000-8000-000000000001'"
		ws   = "'0199a2b4-0000-7000-8000-000000000002'"
		live = "'0199a2b4-0000-7000-8000-000000000003'" // a notebook
		gone = "'0199a2b4-0000-7000-8000-000000000004'" // a notebook deleted at t3
		// Times: an earlier deletion t0, a subtree's t1, an attachment's t2, the notebook's t3, a page's in it
		// before t4.
		t0 = "'2026-01-01 00:00:00+00'"
		t1 = "'2026-01-02 00:00:00+00'"
		t2 = "'2026-01-03 00:00:00+00'"
		t3 = "'2026-01-05 00:00:00+00'"
		t4 = "'2026-01-04 00:00:00+00'"
	)
	node := func(id, notebook, parent, kind, name, deleted string) string {
		return "INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, " +
			"created_at, updated_at, deleted_at) VALUES ('" + id + "', " + notebook + ", " + parent + ", '" + kind + "', '" + name +
			"', '" + name + "', 0, " + user + ", " + user + ", now(), now(), " + deleted + ")"
	}
	const (
		r = "0199a2b4-0000-7000-8000-000000000010" // a root page deleted at t1 with its subtree
		a = "0199a2b4-0000-7000-8000-000000000011" // its child
		b = "0199a2b4-0000-7000-8000-000000000012" // its grandchild
		x = "0199a2b4-0000-7000-8000-000000000013" // an attachment of a
		e = "0199a2b4-0000-7000-8000-000000000014" // a's child deleted before it, at t0
		p = "0199a2b4-0000-7000-8000-000000000015" // a page alive
		d = "0199a2b4-0000-7000-8000-000000000016" // its attachment deleted at t2
		q = "0199a2b4-0000-7000-8000-000000000017" // a root page of the deleted notebook, deleted with it
		s = "0199a2b4-0000-7000-8000-000000000018" // its child deleted before the notebook, at t4
		f = "0199a2b4-0000-7000-8000-000000000019" // q's child deleted with the notebook
	)
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (" + user +
			", 'a@corp.com', 'x', 'a', now(), now())",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + ws +
			", 'acme', 'Acme', " + user + ", " + user + ", now(), now())",
		"INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at) " +
			"VALUES (" + live + ", " + ws + ", 'Live', 'editor', " + user + ", " + user + ", now(), now())",
		"INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at, " +
			"deleted_at) VALUES (" + gone + ", " + ws + ", 'Gone', 'editor', " + user + ", " + user + ", now(), now(), " + t3 + ")",
		node(r, live, "NULL", "page", "r", t1),
		node(a, live, "'"+r+"'", "page", "a", t1),
		node(b, live, "'"+a+"'", "page", "b", t1),
		node(x, live, "'"+a+"'", "asset", "x.png", t1),
		node(e, live, "'"+a+"'", "page", "e", t0),
		node(p, live, "NULL", "page", "p", "NULL"),
		node(d, live, "'"+p+"'", "asset", "d.png", t2),
		node(q, gone, "NULL", "page", "q", t3),
		node(s, gone, "'"+q+"'", "page", "s", t4),
		node(f, gone, "'"+q+"'", "page", "f", t3),
		// The live notebook's changeset created r (an item, its version written twice: revision 2), a (an item, no
		// version: none of this changeset) and x, and renamed p; the deleted notebook's, deleted with it, created q.
		"INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000020', " + live + ", 'edit', 'web', " + user + ", now(), now())",
		"INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, deleted_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000021', " + gone + ", 'edit', 'web', " + user + ", now(), now(), " + t3 + ")",
		"INSERT INTO changeset_items (id, changeset_id, node_id, before_name, before_sort_order, after_name, after_sort_order, " +
			"created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000030', '0199a2b4-0000-7000-8000-000000000020', '" + r + "', NULL, NULL, 'r', 0, now(), now()), " +
			"('0199a2b4-0000-7000-8000-000000000031', '0199a2b4-0000-7000-8000-000000000020', '" + a + "', NULL, NULL, 'a', 0, now(), now()), " +
			"('0199a2b4-0000-7000-8000-000000000032', '0199a2b4-0000-7000-8000-000000000020', '" + x + "', NULL, NULL, 'x.png', 0, now(), now()), " +
			"('0199a2b4-0000-7000-8000-000000000033', '0199a2b4-0000-7000-8000-000000000020', '" + p + "', 'o', 0, 'p', 0, now(), now()), " +
			"('0199a2b4-0000-7000-8000-000000000034', '0199a2b4-0000-7000-8000-000000000021', '" + q + "', NULL, NULL, 'q', 0, now(), now())",
		"INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision, content, content_hash, byte_size, " +
			"created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000040', '0199a2b4-0000-7000-8000-000000000020', '" + r + "', NULL, 2, 'x', sha256('x'), 1, now(), now()), " +
			"('0199a2b4-0000-7000-8000-000000000041', '0199a2b4-0000-7000-8000-000000000020', '" + p + "', NULL, 1, 'x', sha256('x'), 1, now(), now()), " +
			"('0199a2b4-0000-7000-8000-000000000042', '0199a2b4-0000-7000-8000-000000000021', '" + q + "', NULL, 1, 'x', sha256('x'), 1, now(), now())",
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}

	type marked struct {
		ID                    string
		Root                  bool
		Pages, Assets, Height *int32
	}
	rows, err := pool.Query(ctx, `SELECT id::text, trash_root, trash_pages, trash_assets, trash_height FROM nodes ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[marked])
	if err != nil {
		t.Fatal(err)
	}
	n := func(v int32) *int32 { return &v }
	root := func(id string, pages, assets, height int32) marked {
		return marked{ID: id, Root: true, Pages: n(pages), Assets: n(assets), Height: n(height)}
	}
	want := []marked{
		root(r, 3, 1, 3), // r, a, b and x; e, deleted before, is not of its group
		{ID: a}, {ID: b}, {ID: x},
		root(e, 1, 0, 1),
		{ID: p},
		root(d, 0, 1, 0),
		{ID: q}, // deleted with its notebook
		root(s, 1, 0, 1),
		{ID: f},
	}
	if len(got) != len(want) {
		t.Fatalf("nodes = %+v, want %d", got, len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.ID != w.ID || g.Root != w.Root || !sameInt(g.Pages, w.Pages) || !sameInt(g.Assets, w.Assets) || !sameInt(g.Height, w.Height) {
			t.Errorf("node %s: root %v, counts %v %v %v; want root %v, counts %v %v %v", g.ID, g.Root, deref(g.Pages),
				deref(g.Assets), deref(g.Height), w.Root, deref(w.Pages), deref(w.Assets), deref(w.Height))
		}
	}

	rows, err = pool.Query(ctx, `SELECT node_id::text, revision FROM changeset_items ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	type item struct {
		Node     string
		Revision *int32
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[item])
	if err != nil {
		t.Fatal(err)
	}
	// r's and q's creations with their versions, revision 1 whatever their version's is now; a's without one of its
	// changeset, x's an attachment's and p's a rename: none.
	wantItems := []item{{r, n(1)}, {a, nil}, {x, nil}, {p, nil}, {q, n(1)}}
	if len(items) != len(wantItems) {
		t.Fatalf("items = %+v, want %d", items, len(wantItems))
	}
	for i := range wantItems {
		if items[i].Node != wantItems[i].Node || !sameInt(items[i].Revision, wantItems[i].Revision) {
			t.Errorf("item of %s: revision %v, want %v", items[i].Node, deref(items[i].Revision), deref(wantItems[i].Revision))
		}
	}
}

func sameInt(a, b *int32) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func deref(v *int32) any {
	if v == nil {
		return nil
	}
	return *v
}
