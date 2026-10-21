package page_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// pathTree is a notebook's tree as another module reads its paths: the
// pages A, A/B, A/C deleted, A/y.md (a page titled with the .md) and Inbox
// at the root; the attachments A/x.png and r.png; and a page of another
// notebook, Elsewhere.
type pathTree struct {
	pool                    *pgxpool.Pool
	notebook, other         uuid.UUID
	a, b, c, y, inbox, x, r uuid.UUID
	elsewhere               uuid.UUID
	changedA, changedB      time.Time
}

func newPathTree(t *testing.T) pathTree {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	p := pathTree{pool: pool, notebook: uuid.NewV7(), other: uuid.NewV7(), a: uuid.NewV7(), b: uuid.NewV7(), c: uuid.NewV7(),
		y: uuid.NewV7(), inbox: uuid.NewV7(), x: uuid.NewV7(), r: uuid.NewV7(), elsewhere: uuid.NewV7(),
		changedA: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), changedB: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)}
	alice, acme := uuid.NewV7(), uuid.NewV7()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', now(), now())", []any{alice}},
		{"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())", []any{acme, alice}},
		{"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Notes', $3, $3, now(), now())", []any{p.notebook, acme, alice}},
		{"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Other', $3, $3, now(), now())", []any{p.other, acme, alice}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}
	changed := map[uuid.UUID]time.Time{p.a: p.changedA, p.b: p.changedB}
	for _, n := range []struct {
		id, notebook uuid.UUID
		parent       *uuid.UUID
		kind, name   string
		gone         bool
	}{
		{p.a, p.notebook, nil, "page", "A", false}, {p.b, p.notebook, &p.a, "page", "B", false},
		{p.c, p.notebook, &p.a, "page", "C", true}, {p.y, p.notebook, &p.a, "page", "y.md", false},
		{p.x, p.notebook, &p.a, "asset", "x.png", false}, {p.inbox, p.notebook, nil, "page", "Inbox", false},
		{p.r, p.notebook, nil, "asset", "r.png", false}, {p.elsewhere, p.other, nil, "page", "Elsewhere", false},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id,
			updated_by_id, created_at, updated_at, deleted_at)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $7, now(), now(), CASE WHEN $8 THEN now() END)`,
			n.id, n.notebook, n.parent, n.kind, n.name, shared.TitleKey(n.name), alice, n.gone)
		if err != nil {
			t.Fatal(err)
		}
		if n.kind != "page" {
			continue
		}
		at, ok := changed[n.id]
		if !ok {
			at = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		}
		hash := sha256.Sum256([]byte(n.name))
		_, err = pool.Exec(ctx, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id,
			updated_at, deleted_at) VALUES ($1, $2, 1, $3, $4, $5, $6, CASE WHEN $7 THEN now() END)`,
			n.id, n.name, hash[:], len(n.name), alice, at, n.gone)
		if err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// wordsOf is the text a tool's error answers, with its hint when it has
// room: a fit's text with all its items, or the error's own.
func wordsOf(t *testing.T, err error) string {
	t.Helper()
	var te *mcpserver.ToolError
	switch {
	case !errors.As(err, &te):
		t.Fatalf("%v is no tool's error", err)
	case te.Fit != nil:
		text, _ := te.Fit.Build(te.Fit.N)
		return text
	}
	return te.Text
}

// Another module checks a path as page's tools do, at its field, before
// any read; it finds a page by its path in page's tools' words: nothing
// there with the hint page's tools give, and an attachment's path, which
// names no page (M9/P4 design 3.9.1).
func TestNodePathsCheckAndFindAPageAsPagesToolsDo(t *testing.T) {
	p := newPathTree(t)
	ctx := context.Background()
	paths := page.NewNodePaths(p.pool)

	if err := paths.Check("parent", "A/B.md"); err != nil {
		t.Errorf("a path = %v", err)
	}
	for _, bad := range []string{"a//b.md", "A/\x00.md", "A/B.md/", "", "/"} {
		err := paths.Check("parent", bad)
		var se *shared.Error
		if !errors.As(err, &se) || len(se.Fields) != 1 || se.Fields[0].Field != "parent" {
			t.Errorf("Check(%q) = %v, want 422 at parent", bad, err)
		}
	}

	for path, want := range map[string]uuid.UUID{"A/B.md": p.b, "/A.md": p.a, "a/b.MD": p.b, "Inbox.md": p.inbox, "A/y.md.md": p.y} {
		if got, err := paths.FindPage(ctx, p.notebook, "path", path); err != nil || got != want {
			t.Errorf("FindPage(%q) = %v, %v; want %v", path, got, err, want)
		}
	}
	for _, tt := range []struct{ path, want string }{
		{"A/C.md", `page.not_found: Nothing is at "A/C.md".`},
		{"A", `page.not_found: Nothing is at "A". A page's path ends with .md: "A.md".`},
		{"A/y.md", `page.not_found: Nothing is at "A/y.md". A page there is titled with the .md: its path is "A/y.md.md".`},
		{"A/x.png.md", `page.not_found: Nothing is at "A/x.png.md". That is an attachment: its path is "A/x.png".`},
		{"A/x.png", `page.not_found: "A/x.png" is an attachment; a page's path ends with .md.`},
		{"/r.png", `page.not_found: "/r.png" is an attachment; a page's path ends with .md.`},
		{"Elsewhere.md", `page.not_found: Nothing is at "Elsewhere.md".`},
		{"A/x.png/B.md", `page.not_found: Nothing is at "A/x.png/B.md".`},
	} {
		_, err := paths.FindPage(ctx, p.notebook, "path", tt.path)
		if !errors.Is(err, page.ErrNotFound) {
			t.Errorf("FindPage(%q) = %v, want page.not_found", tt.path, err)
			continue
		}
		if got := wordsOf(t, err); got != tt.want {
			t.Errorf("FindPage(%q) says %q, want %q", tt.path, got, tt.want)
		}
	}
	_, err := paths.FindPage(ctx, p.notebook, "parent", "a//b.md")
	var se *shared.Error
	if !errors.As(err, &se) || len(se.Fields) != 1 || se.Fields[0].Field != "parent" {
		t.Errorf("FindPage of a bad path = %v, want 422 at parent", err)
	}
}

// Another module reads the paths of the nodes alive in the notebook, a
// page's with when its content last changed, and the child pages of a
// page or of the root: never a deleted node, an attachment among the
// pages, or another notebook's node (M9/P4 design 3.9.1).
func TestNodePathsReadThePathsPagesAndChildPages(t *testing.T) {
	p := newPathTree(t)
	ctx := context.Background()
	paths := page.NewNodePaths(p.pool)
	ids := []uuid.UUID{p.a, p.b, p.c, p.x, p.r, p.elsewhere, uuid.NewV7()}

	got, err := paths.Paths(ctx, p.notebook, ids)
	want := map[uuid.UUID]string{p.a: "A.md", p.b: "A/B.md", p.x: "A/x.png", p.r: "r.png"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Paths = %v, %v; want %v", got, err, want)
	}

	pages, err := paths.Pages(ctx, p.notebook, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || pages[p.a].Path != "A.md" || pages[p.b].Path != "A/B.md" ||
		!pages[p.a].ChangedAt.Equal(p.changedA) || !pages[p.b].ChangedAt.Equal(p.changedB) {
		t.Errorf("Pages = %+v, want A.md and A/B.md with their contents' last changes", pages)
	}
	if pages, err := paths.Pages(ctx, p.notebook, []uuid.UUID{p.x, p.c}); err != nil || len(pages) != 0 {
		t.Errorf("Pages of an attachment and a deleted page = %+v, %v; want none", pages, err)
	}

	for _, tt := range []struct {
		name   string
		parent uuid.UUID
		want   []uuid.UUID
	}{
		{"the root", uuid.UUID{}, []uuid.UUID{p.a, p.inbox}},
		{"A", p.a, []uuid.UUID{p.b, p.y}},
		{"B", p.b, nil},
		{"an attachment", p.x, nil},
	} {
		got, err := paths.ChildPages(ctx, p.notebook, tt.parent)
		slices.SortFunc(got, uuid.UUID.Compare)
		slices.SortFunc(tt.want, uuid.UUID.Compare)
		if err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("ChildPages of %s = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}
	if got, err := paths.ChildPages(ctx, p.other, uuid.UUID{}); err != nil || !slices.Equal(got, []uuid.UUID{p.elsewhere}) {
		t.Errorf("ChildPages of the other notebook's root = %v, %v", got, err)
	}
}
