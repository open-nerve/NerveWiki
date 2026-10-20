package migrations_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// 00045 gives each changeset whether it renames a node, or moves one to
// another parent, of all its items (M9/P3 design 3.20.1): a rename, a
// move, a move and rename, a creation, a deletion and a reorder, and one
// whose rename is its 1,001st item by node id, which the read before took
// from the first 1,000 alone. The schema goes down to 00044, the data is
// written as the program before wrote it, and up again; then down again,
// the columns gone.
func TestTheChangesetsMovesAreBackfilled(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	downTo(t, m, 45)

	const (
		user   = "'0199a2b4-0000-7000-8000-000000000001'"
		ws     = "'0199a2b4-0000-7000-8000-000000000002'"
		nb     = "'0199a2b4-0000-7000-8000-000000000003'"
		parent = "'0199a2b4-0000-7000-8000-000000000004'"
	)
	// The changesets, in id order, and what each of their items does.
	changesets := []struct {
		name  string
		items []string // its item's before and after: name, under the parent "P/name", at order 2 "name@2"; "" for none
	}{
		{"renamed", []string{"a", "b"}},
		{"moved", []string{"a", "P/a"}},
		{"moved and renamed", []string{"a", "P/b"}},
		{"created", []string{"", "a"}},
		{"deleted", []string{"a", ""}},
		{"reordered", []string{"a", "a@2"}},
	}
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	exec("INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (" + user +
		", 'a@corp.com', 'x', 'a', now(), now())")
	exec("INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + ws +
		", 'acme', 'Acme', " + user + ", " + user + ", now(), now())")
	exec("INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at) " +
		"VALUES (" + nb + ", " + ws + ", 'Live', 'editor', " + user + ", " + user + ", now(), now())")
	exec("INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, " +
		"updated_at) VALUES (" + parent + ", " + nb + ", NULL, 'page', 'P', 'p', 0, " + user + ", " + user + ", now(), now())")
	// state is an item's before or after: its parent, name and order, or none.
	state := func(s string) string {
		if s == "" {
			return "NULL, NULL, NULL"
		}
		under, order := "NULL", "1"
		if rest, ok := strings.CutPrefix(s, "P/"); ok {
			under, s = parent, rest
		}
		if name, at, ok := strings.Cut(s, "@"); ok {
			s, order = name, at
		}
		return under + ", '" + s + "', " + order
	}
	csID := func(i int) string { return fmt.Sprintf("'0199a2b4-0000-7000-8000-0000000001%02d'", i) }
	item := func(cs string, n int, before, after string) {
		node := fmt.Sprintf("'0199a2b4-0000-7000-8001-%012d'", n)
		exec("INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, " +
			"created_at, updated_at) VALUES (" + node + ", " + nb + fmt.Sprintf(", NULL, 'page', 'n%d', 'n%d', 0, ", n, n) + user + ", " + user +
			", now(), now())")
		exec("INSERT INTO changeset_items (id, changeset_id, node_id, before_parent_id, before_name, before_sort_order, after_parent_id, " +
			"after_name, after_sort_order, created_at, updated_at) VALUES (gen_random_uuid(), " + cs + ", " + node + ", " + state(before) +
			", " + state(after) + ", now(), now())")
	}
	newChangeset := func(cs string) {
		exec("INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at) VALUES (" + cs + ", " + nb +
			", 'edit', 'web', " + user + ", now(), now())")
	}
	for i, c := range changesets {
		newChangeset(csID(i))
		item(csID(i), i, c.items[0], c.items[1])
	}
	// 1,001 items of the same before and after, the last by node id renamed.
	last := csID(len(changesets))
	newChangeset(last)
	exec("INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, " +
		"updated_at) SELECT ('0199a2b4-0000-7000-8001-' || lpad(g::text, 12, '0'))::uuid, " + nb + ", NULL, 'page', 'n' || g, 'n' || g, 0, " + user +
		", " + user + ", now(), now() FROM generate_series(1000, 2000) g")
	exec("INSERT INTO changeset_items (id, changeset_id, node_id, before_parent_id, before_name, before_sort_order, after_parent_id, " +
		"after_name, after_sort_order, created_at, updated_at) SELECT gen_random_uuid(), " + last +
		", ('0199a2b4-0000-7000-8001-' || lpad(g::text, 12, '0'))::uuid, NULL, 'a', 1, NULL, CASE WHEN g = 2000 THEN 'b' ELSE 'a' END, 1, " +
		"now(), now() FROM generate_series(1000, 2000) g")
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}

	rows, err := pool.Query(ctx, `SELECT renames, moves FROM changesets ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	type moved struct{ Renames, Moves bool }
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[moved])
	if err != nil {
		t.Fatal(err)
	}
	want := []moved{{true, false}, {false, true}, {true, true}, {}, {}, {}, {true, false}}
	if len(got) != len(want) {
		t.Fatalf("changesets = %+v, want %d", got, len(want))
	}
	for i := range want {
		name := "the 1,001st item renamed"
		if i < len(changesets) {
			name = changesets[i].name
		}
		if got[i] != want[i] {
			t.Errorf("%s: %+v, want %+v", name, got[i], want[i])
		}
	}

	downTo(t, m, 45)
	var columns int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'changesets' AND column_name IN ('renames', 'moves')`).Scan(&columns); err != nil || columns != 0 {
		t.Errorf("after Down, %d of the columns (%v), want none", columns, err)
	}
}
