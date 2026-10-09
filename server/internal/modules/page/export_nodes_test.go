package page_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// content gives the page id content, written at at.
func (l linkTree) content(t *testing.T, id uuid.UUID, content string, at time.Time) {
	t.Helper()
	_, err := l.pool.Exec(context.Background(), `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		VALUES ($1, $2::text, 1, sha256(convert_to($2::text, 'UTF8')), octet_length($2::text), $3, $4)`, id, content, l.alice, at)
	if err != nil {
		t.Fatal(err)
	}
}

// An export reads its scope's nodes not deleted, the whole notebook's or a
// page's subtree: a page's content's bytes, none for a page without
// content or without a content's row; a page's last write is its node's or
// its content's, the later.
// A subtree of what is no page not deleted of the notebook is empty.
func TestAnExportReadsItsScope(t *testing.T) {
	l := newLinkTree(t)
	ctx := context.Background()
	nodes := page.NewExportNodes(l.pool)
	later := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	l.content(t, l.a, "hello", later)
	l.content(t, l.b, "", time.Unix(0, 0))

	byID := func(got []page.ExportNode) map[uuid.UUID]page.ExportNode {
		out := map[uuid.UUID]page.ExportNode{}
		for _, n := range got {
			out[n.ID] = n
		}
		return out
	}
	all, err := nodes.Scope(ctx, l.notebook, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := byID(all)
	a, b, x := got[l.a], got[l.b], got[l.x]
	if len(all) != 3 || a.Name != "A" || a.ParentID != nil || a.Asset || a.Bytes != 5 || !a.Modified.Equal(later) ||
		b.Name != "B" || *b.ParentID != l.a || b.Bytes != 0 || b.Modified.Equal(time.Unix(0, 0)) || !x.Asset || x.Name != "b.png" {
		t.Errorf("Scope(the notebook) = %+v", all)
	}
	for _, tt := range []struct {
		name string
		root uuid.UUID
		want []uuid.UUID
	}{
		{"a page", l.a, []uuid.UUID{l.a, l.b, l.x}},
		{"a leaf", l.b, []uuid.UUID{l.b}},
		{"an attachment", l.x, nil},
		{"a deleted page", l.c, nil},
		{"no node", uuid.NewV7(), nil},
	} {
		sub, err := nodes.Scope(ctx, l.notebook, &tt.root)
		var ids []uuid.UUID
		for _, n := range sub {
			ids = append(ids, n.ID)
		}
		slices.SortFunc(ids, func(p, q uuid.UUID) int { return p.Compare(q) })
		slices.SortFunc(tt.want, func(p, q uuid.UUID) int { return p.Compare(q) })
		if err != nil || !slices.Equal(ids, tt.want) {
			t.Errorf("Scope(%s) = %v, %v; want %v", tt.name, ids, err, tt.want)
		}
	}
	if sub, err := nodes.Scope(ctx, uuid.NewV7(), &l.a); err != nil || len(sub) != 0 {
		t.Errorf("Scope() of a page of another notebook = %v, %v; want none", sub, err)
	}

	for _, tt := range []struct {
		name string
		id   uuid.UUID
		want bool
	}{{"a page", l.a, true}, {"an attachment", l.x, false}, {"a deleted page", l.c, false}, {"no node", uuid.NewV7(), false}} {
		name, ok, err := nodes.Page(ctx, l.notebook, tt.id)
		if err != nil || ok != tt.want || ok && name != "A" {
			t.Errorf("Page(%s) = %q, %v, %v; want %v", tt.name, name, ok, err, tt.want)
		}
	}

	contents, err := nodes.Contents(ctx, []uuid.UUID{l.a, l.b, l.c, l.x})
	if err != nil || len(contents) != 2 || contents[l.a] != "hello" || contents[l.b] != "" {
		t.Errorf("Contents() = %q, %v", contents, err)
	}
}
