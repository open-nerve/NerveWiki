package page_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// linkTree is a notebook's tree as the link index reads it: A, A/B, A/C
// deleted, and A/b.png, an attachment.
type linkTree struct {
	pool                 *pgxpool.Pool
	notebook, a, b, c, x uuid.UUID
	alice, acme          uuid.UUID
}

func newLinkTree(t *testing.T) linkTree {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	l := linkTree{pool: pool, notebook: uuid.NewV7(), a: uuid.NewV7(), b: uuid.NewV7(), c: uuid.NewV7(), x: uuid.NewV7()}
	alice, acme := uuid.NewV7(), uuid.NewV7()
	l.alice, l.acme = alice, acme
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', now(), now())", []any{alice}},
		{"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())", []any{acme, alice}},
		{"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Notes', $3, $3, now(), now())", []any{l.notebook, acme, alice}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}
	for _, n := range []struct {
		id     uuid.UUID
		parent *uuid.UUID
		kind   string
		name   string
		gone   bool
	}{{l.a, nil, "page", "A", false}, {l.b, &l.a, "page", "B", false}, {l.c, &l.a, "page", "C", true}, {l.x, &l.a, "asset", "b.png", false}} {
		_, err := pool.Exec(ctx, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id,
			updated_by_id, created_at, updated_at, deleted_at)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $7, now(), now(), CASE WHEN $8 THEN now() END)`,
			n.id, l.notebook, n.parent, n.kind, n.name, shared.TitleKey(n.name), alice, n.gone)
		if err != nil {
			t.Fatal(err)
		}
	}
	return l
}

// The link index reads the pages and attachments by key and by id with
// their paths and kinds, and a node with the pages and attachments under it:
// never a deleted one (M7/P3 design 4.3).
func TestTheLinkIndexReadsTheNotebooksNodes(t *testing.T) {
	l := newLinkTree(t)
	ctx := context.Background()
	targets := page.NewLinkTargets(l.pool)
	a := page.LinkStep{ID: l.a, Key: "a", Name: "A"}
	b := page.LinkStep{ID: l.b, Key: "b", Name: "B"}
	x := page.LinkStep{ID: l.x, Key: "b.png", Name: "b.png"}

	got, err := targets.ByKeys(ctx, l.notebook, []string{"b", "c", "b.png"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []page.LinkNode{{ID: l.b, Path: []page.LinkStep{a, b}}, {ID: l.x, Path: []page.LinkStep{a, x}, Asset: true}}; !reflect.DeepEqual(got, want) {
		t.Errorf("by keys = %+v\nwant %+v", got, want)
	}
	got, err = targets.Paths(ctx, l.notebook, []uuid.UUID{l.a, l.c, l.x})
	if err != nil {
		t.Fatal(err)
	}
	if want := []page.LinkNode{{ID: l.a, Path: []page.LinkStep{a}}, {ID: l.x, Path: []page.LinkStep{a, x}, Asset: true}}; !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %+v\nwant %+v", got, want)
	}
	sub, err := targets.Subtree(ctx, l.notebook, l.a)
	if err != nil {
		t.Fatal(err)
	}
	if want := []page.LinkStep{a, b, x}; !reflect.DeepEqual(sub, want) {
		t.Errorf("subtree = %+v\nwant %+v", sub, want)
	}
	if sub, err := targets.Subtree(ctx, l.notebook, l.x); err != nil || !reflect.DeepEqual(sub, []page.LinkStep{x}) {
		t.Errorf("an attachment's subtree = %+v, %v; want itself", sub, err)
	}
	if sub, err := targets.Subtree(ctx, l.notebook, l.c); err == nil {
		t.Errorf("a deleted page's subtree = %+v, want an error", sub)
	}
}

// For the link targets and the reads by page (M6/P5 design 7; M7/P3 design
// 4.3): the notebook's pages and attachments, each with its path and kind;
// a page's notebook, never an attachment's. Never a deleted node.
func TestTheLinkIndexsReadsReadTheWholeTreeAndAPagesNotebook(t *testing.T) {
	l := newLinkTree(t)
	ctx := context.Background()
	targets := page.NewLinkTargets(l.pool)
	a := page.LinkStep{ID: l.a, Key: "a", Name: "A"}
	b := page.LinkStep{ID: l.b, Key: "b", Name: "B"}
	x := page.LinkStep{ID: l.x, Key: "b.png", Name: "b.png"}

	got, err := targets.All(ctx, l.notebook)
	if want := []page.LinkNode{{ID: l.a, Path: []page.LinkStep{a}}, {ID: l.b, Path: []page.LinkStep{a, b}},
		{ID: l.x, Path: []page.LinkStep{a, x}, Asset: true}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("All = %+v, %v\nwant %+v", got, err, want)
	}
	if got, err := targets.All(ctx, uuid.NewV7()); err != nil || len(got) != 0 {
		t.Errorf("All of no notebook = %+v, %v", got, err)
	}
	if nb, ok, err := targets.NotebookOf(ctx, l.b); err != nil || !ok || nb != l.notebook {
		t.Errorf("NotebookOf(B) = %v, %t, %v", nb, ok, err)
	}
	for _, id := range []uuid.UUID{l.c, l.x, uuid.NewV7()} {
		if nb, ok, err := targets.NotebookOf(ctx, id); err != nil || ok || nb != (uuid.UUID{}) {
			t.Errorf("NotebookOf(%v) = %v, %t, %v; want none", id, nb, ok, err)
		}
	}
}

// An attachment is no level (M7 decision 1): All reads one under a page as
// deep as pages nest, its path a step longer than a page's may be; a page
// there is a defect, a path with no root within a page's depth.
func TestTheLinkTargetsTakeAnAttachmentUnderTheDeepestPage(t *testing.T) {
	l := newLinkTree(t)
	ctx := context.Background()
	targets := page.NewLinkTargets(l.pool)
	insert := func(id uuid.UUID, parent *uuid.UUID, kind, name string) {
		t.Helper()
		_, err := l.pool.Exec(ctx, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id,
			updated_by_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $7, now(), now())`,
			id, l.notebook, parent, kind, name, shared.TitleKey(name), l.alice)
		if err != nil {
			t.Fatal(err)
		}
	}
	var parent *uuid.UUID
	for i := range page.MaxDepth {
		id := uuid.NewV7()
		insert(id, parent, "page", fmt.Sprintf("d%d", i+1))
		parent = &id
	}
	deep := uuid.NewV7()
	insert(deep, parent, "asset", "deep.png")
	all, err := targets.All(ctx, l.notebook)
	if err != nil {
		t.Fatal(err)
	}
	at := slices.IndexFunc(all, func(n page.LinkNode) bool { return n.ID == deep })
	if at < 0 || !all[at].Asset || len(all[at].Path) != page.MaxDepth+1 {
		t.Fatalf("the deepest attachment in All = %+v, want its path of %d steps", all, page.MaxDepth+1)
	}
	insert(uuid.NewV7(), parent, "page", "too deep")
	if got, err := targets.All(ctx, l.notebook); err == nil {
		t.Errorf("All with a page deeper than pages nest = %+v, want an error", got)
	}
}

// For nervewiki reindex: the notebook's pages by id, a page's content and
// revision, and the title keys taken anew from the names: a stale one is
// set; when siblings would share one, each key stays and they are told.
func TestTheLinkIndexsRebuildReadsAndRekeysThePages(t *testing.T) {
	l := newLinkTree(t)
	ctx := context.Background()
	targets := page.NewLinkTargets(l.pool)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := l.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	key := func(id uuid.UUID) string {
		t.Helper()
		var k string
		if err := l.pool.QueryRow(ctx, "SELECT name_key FROM nodes WHERE id = $1", id).Scan(&k); err != nil {
			t.Fatal(err)
		}
		return k
	}

	if ids, err := targets.PageIDs(ctx, l.notebook); err != nil || !reflect.DeepEqual(ids, []uuid.UUID{l.a, l.b}) {
		t.Errorf("page ids = %v, %v; want A and B", ids, err)
	}
	exec(`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, '# A', 3, sha256('# A'), 3, created_by_id, now() FROM nodes WHERE id = $1`, l.a)
	if content, revision, ok, err := targets.Content(ctx, l.a); err != nil || !ok || content != "# A" || revision != 3 {
		t.Errorf("A's content = %q, %d, %t, %v; want # A at 3", content, revision, ok, err)
	}
	if _, _, ok, err := targets.Content(ctx, l.c); err != nil || ok {
		t.Errorf("a deleted page's content was read: %t, %v", ok, err)
	}

	exec("UPDATE nodes SET name_key = 'stale' WHERE id = $1", l.b)
	if clashes, err := targets.Rekey(ctx, l.notebook); err != nil || clashes != nil {
		t.Fatalf("Rekey = %v, %v; want no clash", clashes, err)
	}
	if got := key(l.b); got != "b" {
		t.Errorf("B's key = %q, want b", got)
	}
	exec("UPDATE nodes SET name = 'B.PNG', name_key = 'z' WHERE id = $1", l.b)
	clashes, err := targets.Rekey(ctx, l.notebook)
	if want := [][]uuid.UUID{{l.b, l.x}}; err != nil || !reflect.DeepEqual(clashes, want) {
		t.Errorf("Rekey of a clash = %v, %v; want %v", clashes, err, want)
	}
	if got := key(l.b); got != "z" {
		t.Errorf("after a clash, B's key = %q, want it kept", got)
	}
}
