package page_test

import (
	"context"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// node adds a node of kind named name under parent to notebook, deleted
// when gone, and returns its id.
func (l linkTree) node(t *testing.T, notebook uuid.UUID, parent *uuid.UUID, kind, name string, gone bool) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	_, err := l.pool.Exec(context.Background(), `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
		created_by_id, updated_by_id, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $7, now(), now(), CASE WHEN $8 THEN now() END)`,
		id, notebook, parent, kind, name, shared.TitleKey(name), l.alice, gone)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// The asset module reads a node of any kind not deleted; a parent that is
// a page not deleted of the notebook; and a parent's attachments not
// deleted, by title key and id, a page at a time.
func TestTheAssetModuleReadsTheTree(t *testing.T) {
	l := newLinkTree(t)
	ctx := context.Background()
	nodes := page.NewAssetNodes(l.pool)
	other := uuid.NewV7()
	if _, err := l.pool.Exec(ctx, `INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, $2, 'Other', $3, $3, now(), now())`, other, l.acme, l.alice); err != nil {
		t.Fatal(err)
	}
	first := l.node(t, l.notebook, &l.a, "asset", "A.png", false)
	second := l.node(t, l.notebook, &l.a, "asset", "b.jpg", false)
	l.node(t, l.notebook, &l.a, "asset", "c.png", true)
	root := l.node(t, l.notebook, nil, "asset", "root.png", false)
	l.node(t, other, nil, "asset", "elsewhere.png", false)

	n, ok, err := nodes.Node(ctx, l.x)
	if err != nil || !ok || !n.Asset || n.Name != "b.png" || n.NameKey != "b.png" || *n.ParentID != l.a || n.NotebookID != l.notebook ||
		n.CreatedBy != l.alice {
		t.Errorf("Node(b.png) = %+v, %v, %v; want the attachment", n, ok, err)
	}
	if n, ok, err := nodes.Node(ctx, l.a); err != nil || !ok || n.Asset || n.Name != "A" {
		t.Errorf("Node(A) = %+v, %v, %v; want the page", n, ok, err)
	}
	for _, id := range []uuid.UUID{l.c, uuid.NewV7()} {
		if _, ok, err := nodes.Node(ctx, id); err != nil || ok {
			t.Errorf("Node(deleted or missing) = %v, %v; want none", ok, err)
		}
	}

	for _, tt := range []struct {
		name             string
		notebook, parent uuid.UUID
		want             bool
	}{
		{"a page", l.notebook, l.a, true},
		{"an attachment", l.notebook, l.x, false},
		{"a deleted page", l.notebook, l.c, false},
		{"a page of another notebook", other, l.a, false},
		{"no node", l.notebook, uuid.NewV7(), false},
	} {
		if got, err := nodes.Parent(ctx, tt.notebook, tt.parent); err != nil || got != tt.want {
			t.Errorf("Parent(%s) = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}

	ids := func(list []page.NodeInfo) []uuid.UUID {
		out := make([]uuid.UUID, len(list))
		for i, n := range list {
			out[i] = n.ID
		}
		return out
	}
	page1, err := nodes.Assets(ctx, l.notebook, &l.a, nil, 2)
	if err != nil || !slices.Equal(ids(page1), []uuid.UUID{first, second}) {
		t.Errorf("Assets(A, 2) = %v, %v; want A.png, b.jpg", ids(page1), err)
	}
	after := &page.AssetCursor{NameKey: page1[1].NameKey, ID: page1[1].ID}
	page2, err := nodes.Assets(ctx, l.notebook, &l.a, after, 2)
	if err != nil || !slices.Equal(ids(page2), []uuid.UUID{l.x}) {
		t.Errorf("Assets(A, after b.jpg) = %v, %v; want b.png", ids(page2), err)
	}
	atRoot, err := nodes.Assets(ctx, l.notebook, nil, nil, 10)
	if err != nil || !slices.Equal(ids(atRoot), []uuid.UUID{root}) {
		t.Errorf("Assets(root) = %v, %v; want root.png, not the page A or another notebook's", ids(atRoot), err)
	}
}
