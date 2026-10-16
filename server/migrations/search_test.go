package migrations_test

import (
	"context"
	"fmt"
	"slices"
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
// as the program before wrote them, and up again (00041 queues them again,
// as they were); then down again with a row built and the queue as it was,
// the tables, the sequences and the functions of the search gone.
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
	if hits := function(t, pool, "page_search_hits(uuid[], text[], text[])"); !hits {
		t.Error("page_search_hits(uuid[], text[], text[]) is missing after Up")
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
	for _, f := range []string{"page_search_hits(tsquery, tsquery)", "page_search_hits(uuid[], text[], text[])",
		"page_search_titles(uuid[], text[])", "page_search_queries(uuid[], text[])", "page_search_prefix(bigint)"} {
		if function(t, pool, f) {
			t.Errorf("%s left after Down", f)
		}
	}
}

// 00041 keys page_search by notebook (M8 Codex review R1): the rows, whose
// lexemes have no prefix, go, and every live node is queued in bulk again,
// a node queued exact staying exact, the trash's not; the GIN indexes keep
// no pending list; a notebook's prefix is its number in base 62 and '#';
// the queries are each notebook's with a prefix, its keys quoted, none for
// one without, nothing without notebooks or keys. Down, 00040's function
// is back, the rows go and the live nodes are queued in bulk again, the
// indexes' pending lists back, the prefixes gone.
func TestTheSearchsKeysAreEachNotebooks(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	downTo(t, m, 41)

	const (
		user = "'0199a2b4-0000-7000-8000-000000000001'"
		ws   = "'0199a2b4-0000-7000-8000-000000000002'"
		nb1  = "0199a2b4-0000-7000-8000-000000000003"
		nb2  = "0199a2b4-0000-7000-8000-000000000004"
		nb3  = "0199a2b4-0000-7000-8000-000000000005" // no prefix

		p = "0199a2b4-0000-7000-8000-000000000010" // a page of nb1, with its row
		e = "0199a2b4-0000-7000-8000-000000000011" // a page of nb1, with its row, queued exact
		d = "0199a2b4-0000-7000-8000-000000000012" // a page of nb1 in the trash, with its row
		q = "0199a2b4-0000-7000-8000-000000000013" // a page of nb2, queued in bulk
	)
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (" + user +
			", 'a@corp.com', 'x', 'a', now(), now())",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES (" + ws +
			", 'acme', 'Acme', " + user + ", " + user + ", now(), now())",
		"INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at) " +
			"SELECT x.id, " + ws + ", x.id::text, 'editor', " + user + ", " + user + ", now(), now() FROM unnest(ARRAY['" + nb1 +
			"', '" + nb2 + "', '" + nb3 + "']::uuid[]) x (id)",
		"INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, " +
			"updated_at, deleted_at) SELECT x.id, x.nb, NULL, 'page', x.id::text, x.id::text, 0, " + user + ", " + user +
			", now(), now(), CASE WHEN x.id = '" + d + "' THEN now() END FROM unnest(ARRAY['" + p + "', '" + e + "', '" + d + "', '" +
			q + "']::uuid[], ARRAY['" + nb1 + "', '" + nb1 + "', '" + nb1 + "', '" + nb2 + "']::uuid[]) x (id, nb)",
		"INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body) SELECT x, 1, 'n', '2/17.0.0/17.0.0', " +
			"now(), 'kakapo', 'kakapo' FROM unnest(ARRAY['" + p + "', '" + e + "', '" + d + "']::uuid[]) x",
		"INSERT INTO page_search_queue (node_id, notebook_id, bulk) VALUES ('" + e + "', '" + nb1 + "', false), ('" + q + "', '" +
			nb2 + "', true)",
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	// queue is the queue as node:bulk, by node.
	queue := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT node_id::text || ':' || bulk FROM page_search_queue ORDER BY node_id`)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	// state is the rows' count and the GIN indexes' options.
	state := func() string {
		t.Helper()
		var rows int
		var options string
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM page_search), (SELECT string_agg(relname || '=' ||
				coalesce(array_to_string(reloptions, ','), '-'), ' ' ORDER BY relname) FROM pg_class
				WHERE relname IN ('page_search_title_idx', 'page_search_body_idx'))`).Scan(&rows, &options); err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%d rows, %s", rows, options)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}
	if want := []string{p + ":true", e + ":false", q + ":true"}; !slices.Equal(queue(), want) {
		t.Errorf("queued after Up = %q, want %q: the live nodes in bulk, e exact", queue(), want)
	}
	if got, want := state(), "0 rows, page_search_body_idx=fastupdate=off page_search_title_idx=fastupdate=off"; got != want {
		t.Errorf("after Up: %s, want %s", got, want)
	}
	for _, f := range []string{"page_search_hits(uuid[], text[], text[])", "page_search_titles(uuid[], text[])"} {
		if !function(t, pool, f) {
			t.Errorf("%s is missing after Up", f)
		}
	}
	if function(t, pool, "page_search_hits(tsquery, tsquery)") {
		t.Error("00040's page_search_hits(tsquery, tsquery) left after Up")
	}

	// The prefixes: base 62, then '#'.
	var prefixes string
	if err := pool.QueryRow(ctx, `SELECT string_agg(page_search_prefix(n), ' ' ORDER BY n)
		FROM unnest(ARRAY[0, 9, 10, 61, 62, 3843, 3844, 1000000, 916132832]::bigint[]) n`).Scan(&prefixes); err != nil {
		t.Fatal(err)
	}
	if want := "0# 9# A# z# 10# zz# 100# 4C92# 100000#"; prefixes != want {
		t.Errorf("page_search_prefix = %s, want %s", prefixes, want)
	}
	// The queries: nb1's and nb2's, each key behind the notebook's prefix,
	// one quoted as a tsquery's operand matching its lexeme; none of nb3.
	if _, err := pool.Exec(ctx, "INSERT INTO page_search_tags (notebook_id, prefix) VALUES ($1, 'A#'), ($2, 'B#')", nb1, nb2); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		notebooks, keys []string
		want            []string // nil: NULL
	}{
		{[]string{nb1, nb2, nb3}, []string{"ka", "po"}, []string{"'A#ka' & 'A#po'", "'B#ka' & 'B#po'"}},
		{[]string{nb3}, []string{"ka"}, []string{}},
		{[]string{}, []string{"ka"}, []string{}},
		{nil, []string{"ka"}, nil},
		{[]string{nb1}, nil, nil},
	} {
		var null bool
		var got []string
		if err := pool.QueryRow(ctx, `SELECT q IS NULL, coalesce(q::text[], '{}') FROM page_search_queries($1::uuid[], $2::text[]) q`,
			tt.notebooks, tt.keys).Scan(&null, &got); err != nil {
			t.Fatal(err)
		}
		slices.Sort(got)
		if null != (tt.want == nil) || !slices.Equal(got, tt.want) && tt.want != nil {
			t.Errorf("page_search_queries(%v, %q) = %q (NULL %v), want %q (NULL %v)", tt.notebooks, tt.keys, got, null, tt.want,
				tt.want == nil)
		}
	}
	var matched bool
	if err := pool.QueryRow(ctx, `SELECT array_to_tsvector(ARRAY['A#it''s \ odd']) @@ ALL (page_search_queries($1::uuid[], ARRAY['it''s \ odd']))
		AND cardinality(page_search_queries($1::uuid[], ARRAY['it''s \ odd'])) = 1`, []string{nb1}).Scan(&matched); err != nil || !matched {
		t.Errorf("a key with a quote and a backslash, behind its prefix, matched %v, %v; want it matched", matched, err)
	}

	downTo(t, m, 41)
	if want := []string{p + ":true", e + ":false", q + ":true"}; !slices.Equal(queue(), want) {
		t.Errorf("queued after Down = %q, want %q", queue(), want)
	}
	if got, want := state(), "0 rows, page_search_body_idx=- page_search_title_idx=-"; got != want {
		t.Errorf("after Down: %s, want %s", got, want)
	}
	if !function(t, pool, "page_search_hits(tsquery, tsquery)") {
		t.Error("00040's page_search_hits(tsquery, tsquery) is missing after Down")
	}
	for _, f := range []string{"page_search_hits(uuid[], text[], text[])", "page_search_titles(uuid[], text[])",
		"page_search_queries(uuid[], text[])", "page_search_prefix(bigint)"} {
		if function(t, pool, f) {
			t.Errorf("%s left after Down", f)
		}
	}
	var tags *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('page_search_tags')::text").Scan(&tags); err != nil || tags != nil {
		t.Errorf("page_search_tags after Down = %v, %v; want gone", tags, err)
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
