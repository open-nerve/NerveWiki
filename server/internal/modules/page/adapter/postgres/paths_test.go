package postgresadapter_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A path's segment is a node not deleted by its title's key: a root at the
// root alone, a child under its parent alone, each in its notebook (M9/P1
// design 3.9.2).
func TestThePathsLookUpByKey(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	guide := f.page(t, f.eng, nil, "Guide", 0)
	setup := f.page(t, f.eng, &guide.ID, "Setup", 0)
	f.page(t, f.eng, &guide.ID, "Straße", 1)
	f.page(t, f.ops, nil, "Elsewhere", 0)
	gone := f.page(t, f.eng, nil, "Gone", 1)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		parent   *uuid.UUID
		key      string
		want     string
	}{
		{"a root", f.eng, nil, "guide", "Guide"},
		{"a root by its key, not its title", f.eng, nil, "Guide", ""},
		{"a child is no root", f.eng, nil, "setup", ""},
		{"a child", f.eng, &guide.ID, "setup", "Setup"},
		{"a child by its folded key", f.eng, &guide.ID, shared.TitleKey("STRASSE"), "Straße"},
		{"a root is no child", f.eng, &guide.ID, "guide", ""},
		{"a child of another parent", f.eng, &setup.ID, "setup", ""},
		{"another notebook's root", f.eng, nil, "elsewhere", ""},
		{"a root in another notebook", f.ops, nil, "guide", ""},
		{"a child in another notebook", f.ops, &guide.ID, "setup", ""},
		{"a deleted root", f.eng, nil, "gone", ""},
	} {
		var name string
		var ok bool
		var err error
		if tt.parent == nil {
			n, found, ferr := f.s.RootByKey(ctx, tt.notebook, tt.key)
			name, ok, err = n.Name, found, ferr
		} else {
			n, found, ferr := f.s.ChildByKey(ctx, tt.notebook, *tt.parent, tt.key)
			name, ok, err = n.Name, found, ferr
		}
		if err != nil || ok != (tt.want != "") || name != tt.want {
			t.Errorf("%s: %q, %v, %v; want %q", tt.name, name, ok, err, tt.want)
		}
	}
}

// PageStates reads each page's revision, size and last write: not of a
// deleted content, nor of an attachment, which has none.
func TestPageStates(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a, b := f.page(t, f.eng, nil, "A", 0), f.page(t, f.eng, nil, "B", 1)
	gone := f.page(t, f.eng, nil, "Gone", 2)
	photo := f.asset(t, f.eng, nil, "photo.png", 3)
	later := now().Add(time.Hour)
	f.exec(t, "UPDATE page_contents SET revision = 7, content = 'twelve bytes', byte_size = 12, updated_at = $2 WHERE node_id = $1", b.ID, later)
	f.exec(t, "UPDATE page_contents SET deleted_at = $2 WHERE node_id = $1", gone.ID, now())
	got, err := f.s.PageStates(ctx, []uuid.UUID{a.ID, b.ID, gone.ID, photo.ID, uuid.NewV7()})
	switch {
	case err != nil:
		t.Fatal(err)
	case len(got) != 2:
		t.Errorf("PageStates = %+v, want A's and B's", got)
	case got[a.ID] != (app.PageState{Revision: 1, Bytes: 0, UpdatedAt: now()}):
		t.Errorf("A's state is %+v", got[a.ID])
	case got[b.ID].Revision != 7 || got[b.ID].Bytes != 12 || !got[b.ID].UpdatedAt.Equal(later):
		t.Errorf("B's state is %+v", got[b.ID])
	}
}

// The path's lookups read a notebook's nodes of the one title key, by an
// index whose condition is the notebook and the key, without statistics:
// PostgreSQL 18 takes the titles' index (notebook_id, name_key), the
// smaller of two equal costs, over the siblings' unique index, and checks
// the parent on the rows (a root's IS NULL, which IS NOT DISTINCT FROM
// would not narrow, M9/P1 design 2.1 item 13, 3.9.2); list's pages'
// states read by the contents' key, a page of 1,000 (3.10.3). Scans of
// whole tables off, as the other plans' tests have them: the tables here
// are small.
func TestThePathLookupsArePointLookups(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ids := f.nodes(t, f.eng, "P", 2000)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
			created_at, updated_at)
		SELECT uuidv7(), $1, $2, 'page', 'C' || i, 'c' || i, i, $3, $3, $4, $4 FROM generate_series(1, 2000) i`, f.eng, ids[0], f.alice, now())
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, '', 1, sha256(''), 0, $2, $3 FROM unnest($1::uuid[]) id`, ids, f.alice, now())
	byKey := []string{"nodes_notebook_id_name_key_idx", "nodes_notebook_id_parent_id_name_key_idx"}
	for _, tt := range []struct {
		name    string
		s       statement
		table   string
		indexes []string
		columns []string
	}{
		{"RootByKey", statementOf(t, func(q *gen.Queries) error {
			_, err := q.RootByKey(ctx, gen.RootByKeyParams{NotebookID: f.eng, NameKey: "p7"})
			return err
		}), "nodes", byKey, []string{"notebook_id", "name_key"}},
		{"ChildByKey", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ChildByKey(ctx, gen.ChildByKeyParams{NotebookID: f.eng, ParentID: &ids[0], NameKey: "c7"})
			return err
		}), "nodes", byKey, []string{"notebook_id", "name_key"}},
		{"PageStates", statementOf(t, func(q *gen.Queries) error {
			_, err := q.PageStates(ctx, ids[:1000])
			return err
		}), "page_contents", []string{"page_contents_pkey"}, []string{"node_id"}},
	} {
		plan, raw := f.planTreeOf(t, tt.s)
		var scans []*planNode
		for _, index := range tt.indexes {
			scans = append(scans, plan.all(reads(tt.table, index, ""))...)
		}
		if len(scans) != 1 || len(plan.all(seqScan)) > 0 {
			t.Errorf("%s does not read %s by one of %q:\n%s", tt.name, tt.table, tt.indexes, raw)
			continue
		}
		cond := scans[0].IndexCond
		for _, bitmap := range scans[0].all(func(n *planNode) bool { return n.IndexCond != "" }) {
			cond += " " + bitmap.IndexCond
		}
		for _, column := range tt.columns {
			if !strings.Contains(cond, "."+column+" ") {
				t.Errorf("%s's index condition %q lacks %s", tt.name, cond, column)
			}
		}
	}
}
