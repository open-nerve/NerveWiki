package migrations_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// 00038 gives a deletion's item before it the revision of its page's
// content in the trash at its time (M8/P3 design 3.2): none for a page
// restored since, or deleted again at another time, nor for an
// attachment, which has no content. The schema goes down to 00037, the
// data is written as the program before wrote it, and up again; then down
// again, the deletions' revisions cleared.
func TestTheDeletionsRevisionsAreBackfilled(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	downTo(t, m, 38)

	const (
		user = "'0199a2b4-0000-7000-8000-000000000001'"
		ws   = "'0199a2b4-0000-7000-8000-000000000002'"
		nb   = "'0199a2b4-0000-7000-8000-000000000003'"
		cs   = "'0199a2b4-0000-7000-8000-000000000004'"
		t1   = "'2026-01-01 00:00:00+00'"
		t2   = "'2026-01-02 00:00:00+00'"

		d = "0199a2b4-0000-7000-8000-000000000010" // a page deleted at t1, in the trash
		r = "0199a2b4-0000-7000-8000-000000000011" // a page deleted at t1, restored since
		a = "0199a2b4-0000-7000-8000-000000000012" // a page deleted at t1, restored, deleted again at t2
		x = "0199a2b4-0000-7000-8000-000000000013" // an attachment deleted at t1, in the trash
		p = "0199a2b4-0000-7000-8000-000000000014" // a page renamed
		c = "0199a2b4-0000-7000-8000-000000000015" // a page created
	)
	node := func(id, kind, name, deleted string) string {
		return "INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, " +
			"created_at, updated_at, deleted_at) VALUES ('" + id + "', " + nb + ", NULL, '" + kind + "', '" + name + "', '" + name +
			"', 0, " + user + ", " + user + ", now(), now(), " + deleted + ")"
	}
	content := func(id, revision, deleted string) string {
		return "INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at, deleted_at) " +
			"VALUES ('" + id + "', 'x', " + revision + ", sha256('x'), 1, " + user + ", now(), " + deleted + ")"
	}
	item := func(n, id, before, after, revision, at, deleted string) string {
		state := "NULL, NULL, '" + after + "', 0"
		if after == "" {
			state = "'" + before + "', 0, NULL, NULL"
		} else if before != "" {
			state = "'" + before + "', 0, '" + after + "', 0"
		}
		return "INSERT INTO changeset_items (id, changeset_id, node_id, before_name, before_sort_order, after_name, after_sort_order, " +
			"revision, created_at, updated_at, deleted_at) VALUES ('0199a2b4-0000-7000-8000-0000000000" + n + "', " + cs + ", '" + id +
			"', " + state + ", " + revision + ", " + at + ", " + at + ", " + deleted + ")"
	}
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (" + user +
			", 'a@corp.com', 'x', 'a', now(), now())",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + ws +
			", 'acme', 'Acme', " + user + ", " + user + ", now(), now())",
		"INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at) " +
			"VALUES (" + nb + ", " + ws + ", 'Live', 'editor', " + user + ", " + user + ", now(), now())",
		"INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at) VALUES (" + cs + ", " + nb +
			", 'edit', 'web', " + user + ", now(), now())",
		node(d, "page", "d", t1), content(d, "3", t1),
		node(r, "page", "r", "NULL"), content(r, "2", "NULL"),
		node(a, "page", "a", t2), content(a, "4", t2),
		node(x, "asset", "x.png", t1),
		node(p, "page", "p", "NULL"), content(p, "1", "NULL"),
		node(c, "page", "c", "NULL"), content(c, "1", "NULL"),
		// The deletions' items at their deletion's time t1: r's restore took it out of the trash, a's later deletion put it
		// back at t2.
		item("20", d, "d", "", "NULL", t1, t1),
		item("21", r, "r", "", "NULL", t1, "NULL"),
		item("22", a, "a", "", "NULL", t1, t2),
		item("23", x, "x.png", "", "NULL", t1, t1),
		item("24", p, "o", "p", "NULL", t1, "NULL"),
		item("25", c, "", "c", "1", t1, "NULL"),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}

	rows, err := pool.Query(ctx, `SELECT node_id::text, revision FROM changeset_items ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	type revised struct {
		Node     string
		Revision *int32
	}
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[revised])
	if err != nil {
		t.Fatal(err)
	}
	n := func(v int32) *int32 { return &v }
	// d's deletion, its page in the trash at its time: the revision of its content there. The rest none, but the creation's
	// own.
	want := []revised{{d, n(3)}, {r, nil}, {a, nil}, {x, nil}, {p, nil}, {c, n(1)}}
	if len(got) != len(want) {
		t.Fatalf("items = %+v, want %d", got, len(want))
	}
	for i := range want {
		if got[i].Node != want[i].Node || !sameInt(got[i].Revision, want[i].Revision) {
			t.Errorf("item of %s: revision %v, want %v", got[i].Node, deref(got[i].Revision), deref(want[i].Revision))
		}
	}

	// Down again over d's revision (M8/P3A review A-4): the deletions' items
	// lose theirs, which 00037's checks forbid; the creation keeps its own.
	downTo(t, m, 38)
	rows, err = pool.Query(ctx, `SELECT node_id::text, revision FROM changeset_items ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	if got, err = pgx.CollectRows(rows, pgx.RowToStructByPos[revised]); err != nil {
		t.Fatal(err)
	}
	want[0].Revision = nil
	for i := range want {
		if i >= len(got) || got[i].Node != want[i].Node || !sameInt(got[i].Revision, want[i].Revision) {
			t.Errorf("after Down, items %+v; want %+v", got, want)
			break
		}
	}
}
