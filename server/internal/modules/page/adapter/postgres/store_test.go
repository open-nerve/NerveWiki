package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func now() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 123456000, time.UTC) }

// fixture is a database with alice, the workspace acme and its notebooks
// eng and ops.
type fixture struct {
	s        *postgresadapter.Store
	pool     *pgxpool.Pool
	alice    uuid.UUID
	eng, ops uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 6})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := fixture{s: postgresadapter.New(pool), pool: pool, alice: uuid.NewV7(), eng: uuid.NewV7(), ops: uuid.NewV7()}
	acme := uuid.NewV7()
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', $2, $2)",
		f.alice, now())
	f.exec(t, "INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, $3, $3)",
		acme, f.alice, now())
	for _, id := range []uuid.UUID{f.eng, f.ops} {
		f.exec(t, "INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Notes', $3, $3, $4, $4)",
			id, acme, f.alice, now())
	}
	return f
}

func (f fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (f fixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// page adds a page with its content, through the store.
func (f fixture) page(t *testing.T, notebook uuid.UUID, parent *uuid.UUID, name string, order float64) domain.Node {
	t.Helper()
	ctx := context.Background()
	title, err := domain.CheckTitle("title", name)
	if err != nil {
		t.Fatal(err)
	}
	n := domain.Node{ID: uuid.NewV7(), NotebookID: notebook, ParentID: parent, Kind: domain.KindPage, Name: title.Name,
		NameKey: title.Key, SortOrder: order, CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: now(), UpdatedAt: now()}
	if err := f.s.CreateNode(ctx, n); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(nil)
	if err := f.s.CreateContent(ctx, app.Content{NodeID: n.ID, Revision: 1, Hash: sum[:], By: f.alice, At: now()}); err != nil {
		t.Fatal(err)
	}
	return n
}

func names(nodes []domain.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}

func TestNodesReadBack(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	root := f.page(t, f.eng, nil, "Root", 0)
	b := f.page(t, f.eng, &root.ID, "B", 2)
	a := f.page(t, f.eng, &root.ID, "A", 1)
	gone := f.page(t, f.eng, &root.ID, "Gone", 3)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	f.page(t, f.ops, nil, "Elsewhere", 0)

	got, err := f.s.FindNode(ctx, a.ID)
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Errorf("FindNode(A) = %+v, %v; want %+v", got, err, a)
	}
	if _, err := f.s.FindNode(ctx, gone.ID); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("FindNode(deleted) = %v, want ErrNotFound", err)
	}
	if _, err := f.s.FindNodeIn(ctx, f.ops, a.ID); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("FindNodeIn(ops, eng's A) = %v, want ErrNotFound", err)
	}
	list, err := f.s.ListNodes(ctx, f.eng)
	if err != nil || len(list) != 3 {
		t.Errorf("ListNodes(eng) = %v, %v; want root, A and B", names(list), err)
	}
	children, err := f.s.Children(ctx, f.eng, &root.ID)
	if err != nil || !slices.Equal(names(children), []string{"A", "B"}) {
		t.Errorf("Children(root) = %v, %v; want A, B in order", names(children), err)
	}
	roots, err := f.s.Children(ctx, f.eng, nil)
	if err != nil || !slices.Equal(names(roots), []string{"Root"}) {
		t.Errorf("Children(none) = %v, %v; want eng's root alone", names(roots), err)
	}
	meta, err := f.s.ContentMeta(ctx, b.ID)
	if err != nil || meta != (app.ContentMeta{Revision: 1, UpdatedBy: f.alice, UpdatedAt: now()}) {
		t.Errorf("ContentMeta(B) = %+v, %v", meta, err)
	}
}

// Siblings' names differ by their keys: under one parent, at the root
// too, while a deleted sibling holds no name.
func TestSiblingNamesAreUnique(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	root := f.page(t, f.eng, nil, "Straße", 0)
	child := f.page(t, f.eng, &root.ID, "Notes", 0)
	for _, tt := range []struct {
		name   string
		parent *uuid.UUID
		title  string
	}{{"a root", nil, "STRASSE"}, {"a child", &root.ID, "notes"}} {
		title, _ := domain.CheckTitle("title", tt.title)
		n := domain.Node{ID: uuid.NewV7(), NotebookID: f.eng, ParentID: tt.parent, Kind: domain.KindPage, Name: title.Name,
			NameKey: title.Key, CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: now(), UpdatedAt: now()}
		if err := f.s.CreateNode(ctx, n); !errors.Is(err, domain.ErrTitleTaken) {
			t.Errorf("%s named %q beside its sibling = %v, want ErrTitleTaken", tt.name, tt.title, err)
		}
	}
	other := f.page(t, f.eng, &root.ID, "Other", 1)
	renamed := other
	renamed.Name, renamed.NameKey = "NOTES", shared.TitleKey("NOTES")
	if err := f.s.RenameNode(ctx, renamed); !errors.Is(err, domain.ErrTitleTaken) {
		t.Errorf("RenameNode(Other to NOTES) = %v, want ErrTitleTaken", err)
	}
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", child.ID, now())
	if err := f.s.RenameNode(ctx, renamed); err != nil {
		t.Errorf("RenameNode(Other to NOTES) after Notes' deletion = %v, want no error", err)
	}
	f.page(t, f.ops, nil, "Straße", 0)
}

// A parent is in its child's notebook: the foreign key on both columns
// keeps a tree in one notebook.
func TestAParentInAnotherNotebook(t *testing.T) {
	f := newFixture(t)
	elsewhere := f.page(t, f.ops, nil, "Elsewhere", 0)
	n := domain.Node{ID: uuid.NewV7(), NotebookID: f.eng, ParentID: &elsewhere.ID, Kind: domain.KindPage, Name: "A",
		NameKey: "a", CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: now(), UpdatedAt: now()}
	err := f.s.CreateNode(context.Background(), n)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != "nodes_notebook_id_parent_id_fkey" {
		t.Errorf("CreateNode(in eng under ops' page) = %v, want foreign_key_violation (23503) of nodes_notebook_id_parent_id_fkey", err)
	}
}

func TestAncestors(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a := f.page(t, f.eng, nil, "A", 0)
	b := f.page(t, f.eng, &a.ID, "B", 0)
	c := f.page(t, f.eng, &b.ID, "C", 0)
	got, err := f.s.Ancestors(ctx, c.ID)
	if err != nil || !slices.Equal(got, []domain.Ancestor{{ID: a.ID, Name: "A"}, {ID: b.ID, Name: "B"}}) {
		t.Errorf("Ancestors(C) = %+v, %v; want A then B", got, err)
	}
	if got, err := f.s.Ancestors(ctx, a.ID); err != nil || len(got) != 0 {
		t.Errorf("Ancestors(root) = %+v, %v; want none", got, err)
	}
	// Only a defect makes a loop; the chain then never reaches a root.
	f.exec(t, "UPDATE nodes SET parent_id = $2 WHERE id = $1", a.ID, c.ID)
	if _, err := f.s.Ancestors(ctx, c.ID); err == nil {
		t.Error("Ancestors(C) in a loop = nil error")
	}
}

func TestRecordItemAndRevisionMerge(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "A", 0)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientAPI, By: f.alice, At: now()}
	if err := f.s.CreateChangeset(ctx, cs); err != nil {
		t.Fatal(err)
	}
	first, second := domain.TreeState{Name: "A", SortOrder: 0}, domain.TreeState{Name: "B", SortOrder: 0}
	for _, c := range []domain.Change{{NodeID: n.ID, After: &first}, {NodeID: n.ID, Before: &first, After: &second}} {
		if err := f.s.RecordItem(ctx, app.Item{ID: uuid.NewV7(), ChangesetID: cs.ID, Change: c, At: now()}); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.count(t, "SELECT count(*) FROM changeset_items WHERE node_id = $1 AND before_name IS NULL AND after_name = 'B'", n.ID); got != 1 {
		t.Errorf("items of a page created then renamed in one changeset = %d with no before and after B, want 1", got)
	}
	base := 1
	for _, r := range []app.Revision{{Base: &base, Revision: 2, Content: "x"}, {Revision: 3, Content: "xy"}} {
		sum := sha256.Sum256([]byte(r.Content))
		r.ID, r.ChangesetID, r.NodeID, r.Hash, r.ByteSize, r.At = uuid.NewV7(), cs.ID, n.ID, sum[:], len(r.Content), now()
		if err := f.s.RecordRevision(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.count(t, "SELECT count(*) FROM page_revisions WHERE node_id = $1 AND base_revision = 1 AND revision = 3 AND content = 'xy'", n.ID); got != 1 {
		t.Errorf("versions of a page written twice in one changeset = %d from base 1 at revision 3, want 1", got)
	}
}

// An item keeps where its node was and is: the parent and the order, not
// the name alone; M8 reads them back.
func TestRecordItemKeepsTheTreeStates(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	p, q := f.page(t, f.eng, nil, "P", 0), f.page(t, f.eng, nil, "Q", 1)
	n := f.page(t, f.eng, &p.ID, "N", 1)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientWeb, By: f.alice, At: now()}
	if err := f.s.CreateChangeset(ctx, cs); err != nil {
		t.Fatal(err)
	}
	before, after := n.State(), domain.TreeState{ParentID: &q.ID, Name: "N", SortOrder: 2.5}
	if err := f.s.RecordItem(ctx, app.Item{ID: uuid.NewV7(), ChangesetID: cs.ID, Change: domain.Change{NodeID: n.ID, Before: &before, After: &after}, At: now()}); err != nil {
		t.Fatal(err)
	}
	if got := f.count(t, `SELECT count(*) FROM changeset_items WHERE node_id = $1 AND before_parent_id = $2 AND before_name = 'N'
		AND before_sort_order = 1 AND after_parent_id = $3 AND after_name = 'N' AND after_sort_order = 2.5`, n.ID, p.ID, q.ID); got != 1 {
		t.Error("the item does not keep the node's parent and order, before and after")
	}
}

// A rename writes who and when, and keeps the creation's.
func TestRenameNodeWritesItsAuthor(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	bob := uuid.NewV7()
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'bob@corp.com', 'x', 'x', $2, $2)",
		bob, now())
	n := f.page(t, f.eng, nil, "Notes", 0)
	renamed := n
	renamed.Name, renamed.NameKey, renamed.UpdatedBy, renamed.UpdatedAt = "Journal", "journal", bob, now().Add(time.Minute)
	if err := f.s.RenameNode(ctx, renamed); err != nil {
		t.Fatal(err)
	}
	if got, err := f.s.FindNode(ctx, n.ID); err != nil || !reflect.DeepEqual(got, renamed) {
		t.Errorf("FindNode after the rename = %+v, %v; want %+v", got, err, renamed)
	}
}

// A notebook's deletion takes its pages not deleted, what follows them and
// its changesets, at its time; what the trash holds keeps its own.
func TestDeleteNotebooksPages(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	root := f.page(t, f.eng, nil, "Root", 0)
	child := f.page(t, f.eng, &root.ID, "Child", 0)
	trashed := f.page(t, f.eng, nil, "Trashed", 1)
	kept := f.page(t, f.ops, nil, "Kept", 0)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientWeb, By: f.alice, At: now()}
	if err := f.s.CreateChangeset(ctx, cs); err != nil {
		t.Fatal(err)
	}
	for _, n := range []domain.Node{root, child, trashed} {
		state := n.State()
		if err := f.s.RecordItem(ctx, app.Item{ID: uuid.NewV7(), ChangesetID: cs.ID, Change: domain.Change{NodeID: n.ID, After: &state}, At: now()}); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(nil)
		r := app.Revision{ID: uuid.NewV7(), ChangesetID: cs.ID, NodeID: n.ID, Revision: 1, Hash: sum[:], At: now()}
		if err := f.s.RecordRevision(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	earlier := now().Add(-time.Hour)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", trashed.ID, earlier)
	for _, table := range []string{"page_contents", "page_revisions", "changeset_items"} {
		f.exec(t, "UPDATE "+table+" SET deleted_at = $2 WHERE node_id = $1", trashed.ID, earlier)
	}
	at := now().Add(time.Minute)
	if err := f.s.DeleteNotebooksPages(ctx, []uuid.UUID{f.eng}, f.alice, at); err != nil {
		t.Fatal(err)
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at = $2 AND updated_at = $2", f.eng, at); got != 2 {
		t.Errorf("eng's nodes deleted at the deletion's time = %d, want root and child", got)
	}
	for _, table := range []string{"page_contents", "page_revisions", "changeset_items"} {
		if got := f.count(t, "SELECT count(*) FROM "+table+" WHERE deleted_at = $1", at); got != 2 {
			t.Errorf("%s deleted at the deletion's time = %d, want root's and child's", table, got)
		}
		if got := f.count(t, "SELECT count(*) FROM "+table+" WHERE node_id = $1 AND deleted_at = $2", trashed.ID, earlier); got != 1 {
			t.Errorf("the trashed page's row of %s lost its own time", table)
		}
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND deleted_at = $2", cs.ID, at); got != 1 {
		t.Error("eng's changeset is not deleted at the deletion's time")
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at = $2", trashed.ID, earlier); got != 1 {
		t.Error("the trashed page lost its own time")
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", kept.ID); got != 1 {
		t.Error("ops' page is deleted with eng")
	}
}

// A subtree is the node and its descendants not deleted, level by level and
// in order within a level, in its notebook alone.
func TestSubtree(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	r := f.page(t, f.eng, nil, "R", 0)
	a, b := f.page(t, f.eng, &r.ID, "A", 1), f.page(t, f.eng, &r.ID, "B", 0)
	f.page(t, f.eng, &a.ID, "A1", 0)
	gone := f.page(t, f.eng, &b.ID, "B1", 0)
	f.page(t, f.eng, nil, "S", 1)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	sub, err := f.s.Subtree(ctx, f.eng, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range sub {
		got = append(got, fmt.Sprintf("%s@%d", n.Node.Name, n.Level))
	}
	if want := []string{"R@1", "B@2", "A@2", "A1@3"}; !slices.Equal(got, want) {
		t.Errorf("Subtree(R) = %v, want %v", got, want)
	}
	if sub[0].Node != r {
		t.Errorf("Subtree(R)[0] = %+v, want R as it reads", sub[0].Node)
	}
	for what, id := range map[string]struct{ notebook, node uuid.UUID }{
		"in another notebook": {f.ops, r.ID}, "deleted": {f.eng, gone.ID}, "missing": {f.eng, uuid.NewV7()},
	} {
		if _, err := f.s.Subtree(ctx, id.notebook, id.node); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("Subtree of a node %s = %v, want app.ErrNotFound", what, err)
		}
	}
}

// A subtree goes as deep as pages nest: a chain of MaxDepth levels is read
// to its last page.
func TestSubtreeReachesTheDeepestLevel(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	var parent *uuid.UUID
	var ids []uuid.UUID
	for i := range domain.MaxDepth {
		n := f.page(t, f.eng, parent, fmt.Sprintf("Level %d", i+1), 0)
		parent, ids = &n.ID, append(ids, n.ID)
	}
	sub, err := f.s.Subtree(ctx, f.eng, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) != domain.MaxDepth || sub.Height() != domain.MaxDepth || sub[len(sub)-1].Node.ID != ids[len(ids)-1] {
		t.Fatalf("Subtree of a chain of %d = %d nodes, height %d; want all of them, the last at level %d", domain.MaxDepth, len(sub),
			sub.Height(), domain.MaxDepth)
	}
}

// A move writes the parent, the order and who and when; a title a sibling
// holds under the new parent is page.title_taken.
func TestMoveNode(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	bob := uuid.NewV7()
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'bob@corp.com', 'x', 'x', $2, $2)",
		bob, now())
	p, q := f.page(t, f.eng, nil, "P", 0), f.page(t, f.eng, nil, "Q", 1)
	n := f.page(t, f.eng, &p.ID, "N", 0)
	f.page(t, f.eng, &q.ID, "n", 0)
	moved := n
	moved.ParentID, moved.SortOrder, moved.UpdatedBy, moved.UpdatedAt = nil, 2.5, bob, now().Add(time.Minute)
	if err := f.s.MoveNode(ctx, moved); err != nil {
		t.Fatal(err)
	}
	if got, err := f.s.FindNode(ctx, n.ID); err != nil || !reflect.DeepEqual(got, moved) {
		t.Errorf("FindNode after the move to the root = %+v, %v; want %+v", got, err, moved)
	}
	moved.ParentID = &q.ID
	if err := f.s.MoveNode(ctx, moved); !errors.Is(err, domain.ErrTitleTaken) {
		t.Errorf("MoveNode under a sibling's title = %v, want page.title_taken", err)
	}
}

// A subtree's deletion takes its nodes not deleted and what follows them,
// at its time, by its caller; what the trash holds keeps its own time, and
// the rest of the notebook stays.
func TestDeleteNodes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	bob := uuid.NewV7()
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'bob@corp.com', 'x', 'x', $2, $2)",
		bob, now())
	root := f.page(t, f.eng, nil, "Root", 0)
	child := f.page(t, f.eng, &root.ID, "Child", 0)
	trashed := f.page(t, f.eng, &root.ID, "Trashed", 1)
	sibling := f.page(t, f.eng, nil, "Sibling", 1)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientWeb, By: f.alice, At: now()}
	if err := f.s.CreateChangeset(ctx, cs); err != nil {
		t.Fatal(err)
	}
	for _, n := range []domain.Node{root, child, trashed, sibling} {
		state := n.State()
		if err := f.s.RecordItem(ctx, app.Item{ID: uuid.NewV7(), ChangesetID: cs.ID, Change: domain.Change{NodeID: n.ID, After: &state}, At: now()}); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(nil)
		if err := f.s.RecordRevision(ctx, app.Revision{ID: uuid.NewV7(), ChangesetID: cs.ID, NodeID: n.ID, Revision: 1, Hash: sum[:], At: now()}); err != nil {
			t.Fatal(err)
		}
	}
	earlier := now().Add(-time.Hour)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", trashed.ID, earlier)
	for _, table := range followers() {
		f.exec(t, "UPDATE "+table+" SET deleted_at = $2 WHERE node_id = $1", trashed.ID, earlier)
	}
	at := now().Add(time.Minute)
	if err := f.s.DeleteNodes(ctx, []uuid.UUID{root.ID, child.ID, trashed.ID}, bob, at); err != nil {
		t.Fatal(err)
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE deleted_at = $1 AND updated_at = $1 AND updated_by_id = $2", at, bob); got != 2 {
		t.Errorf("nodes deleted at the deletion's time by bob = %d, want root and child", got)
	}
	for _, table := range followers() {
		if got := f.count(t, "SELECT count(*) FROM "+table+" WHERE deleted_at = $1", at); got != 2 {
			t.Errorf("%s deleted at the deletion's time = %d, want root's and child's", table, got)
		}
		if got := f.count(t, "SELECT count(*) FROM "+table+" WHERE node_id = $1 AND deleted_at = $2", trashed.ID, earlier); got != 1 {
			t.Errorf("the trashed page's row of %s lost its own time", table)
		}
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at = $2 AND updated_by_id = $3", trashed.ID, earlier, f.alice); got != 1 {
		t.Error("the trashed page lost its own time or author")
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE deleted_at IS NULL"); got != 1 {
		t.Error("the changeset is deleted with the subtree, want it kept with its notebook")
	}
	for _, table := range append([]string{"nodes"}, followers()...) {
		column := "node_id"
		if table == "nodes" {
			column = "id"
		}
		if got := f.count(t, "SELECT count(*) FROM "+table+" WHERE "+column+" = $1 AND deleted_at IS NULL", sibling.ID); got != 1 {
			t.Errorf("the sibling's row of %s is deleted with the subtree", table)
		}
	}
}

// An item that deletes its node goes to the trash with it, at its time:
// inserted so, or merged so after a rename in the same changeset, keeping
// the first before.
func TestADeletionsItemIsDeletedWithItsNode(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n, m := f.page(t, f.eng, nil, "N", 0), f.page(t, f.eng, nil, "M", 1)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientWeb, By: f.alice, At: now()}
	if err := f.s.CreateChangeset(ctx, cs); err != nil {
		t.Fatal(err)
	}
	at := now().Add(time.Minute)
	nState, mState, renamed := n.State(), m.State(), domain.TreeState{Name: "M2", SortOrder: 1}
	for _, c := range []domain.Change{
		{NodeID: n.ID, Before: &nState},
		{NodeID: m.ID, Before: &mState, After: &renamed},
		{NodeID: m.ID, Before: &renamed},
	} {
		if err := f.s.RecordItem(ctx, app.Item{ID: uuid.NewV7(), ChangesetID: cs.ID, Change: c, At: at}); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.count(t, `SELECT count(*) FROM changeset_items WHERE deleted_at = $1 AND after_name IS NULL
		AND before_name = CASE node_id WHEN $2 THEN 'N' WHEN $3 THEN 'M' END`, at, n.ID, m.ID); got != 2 {
		t.Errorf("deletions' items in the trash at their time, keeping the first before = %d, want 2", got)
	}
}
