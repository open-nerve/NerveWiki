package postgresadapter_test

import (
	"context"
	"reflect"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// pathOf is the path the link index reads of nodes, from the root down.
func pathOf(nodes ...domain.Node) postgresadapter.LinkPath {
	p := postgresadapter.LinkPath{ID: nodes[len(nodes)-1].ID}
	for _, n := range nodes {
		p.Steps = append(p.Steps, postgresadapter.LinkStep{ID: n.ID, Key: n.NameKey, Name: n.Name})
	}
	return p
}

// The link index's candidates are the pages of the notebook whose title key
// it names, each with its path from the root: not another notebook's, not an
// attachment, not a deleted page; and the pages of the notebook among ids.
func TestLinkTargetsAreTheNotebooksPagesWithTheirPaths(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	note := f.page(t, f.eng, nil, "Note", 0)
	a := f.page(t, f.eng, nil, "A", 1)
	aNote := f.page(t, f.eng, &a.ID, "NOTE", 0)
	b := f.page(t, f.eng, &a.ID, "B", 1)
	bNote := f.page(t, f.eng, &b.ID, "Note", 0)
	c := f.page(t, f.eng, nil, "C", 2)
	gone := f.page(t, f.eng, &c.ID, "Note", 0)
	other := f.page(t, f.ops, nil, "Note", 0)
	if err := f.s.DeleteNodes(ctx, []uuid.UUID{gone.ID}, f.alice, now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	asset := domain.Node{ID: uuid.NewV7(), NotebookID: f.eng, ParentID: &c.ID, Kind: domain.KindAsset, Name: "note",
		NameKey: "note", SortOrder: 1, CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: now(), UpdatedAt: now()}
	if err := f.s.CreateNode(ctx, asset); err != nil {
		t.Fatal(err)
	}

	got, err := f.s.LinkTargetsByKeys(ctx, f.eng, []string{"note", "b"})
	if err != nil {
		t.Fatal(err)
	}
	want := []postgresadapter.LinkPath{pathOf(note), pathOf(a, aNote), pathOf(a, b), pathOf(a, b, bNote)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("by keys = %+v\nwant %+v", got, want)
	}
	if got, err := f.s.LinkTargetsByKeys(ctx, f.eng, nil); err != nil || got != nil {
		t.Errorf("by no keys = %+v, %v", got, err)
	}

	got, err = f.s.LinkTargetsByIDs(ctx, f.eng, []uuid.UUID{bNote.ID, other.ID, gone.ID, asset.ID, a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if want := []postgresadapter.LinkPath{pathOf(a), pathOf(a, b, bNote)}; !reflect.DeepEqual(got, want) {
		t.Errorf("by ids = %+v\nwant %+v", got, want)
	}
	if got, err := f.s.LinkTargetsByIDs(ctx, f.eng, nil); err != nil || got != nil {
		t.Errorf("by no ids = %+v, %v", got, err)
	}
}

// A path that loops, or goes through a deleted page, never reaches a root:
// a defect, which the read reports rather than answering a path cut short
// or one through the trash.
func TestALinkTargetsPathThatReachesNoRootIsAnError(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.page(t, f.eng, nil, "A", 0)
	b := f.page(t, f.eng, &a.ID, "B", 0)
	f.exec(t, "UPDATE nodes SET parent_id = $1 WHERE id = $2", b.ID, a.ID)
	if got, err := f.s.LinkTargetsByIDs(ctx, f.eng, []uuid.UUID{b.ID}); err == nil {
		t.Errorf("a loop's path = %+v, want an error", got)
	}
	c := f.page(t, f.eng, nil, "C", 1)
	d := f.page(t, f.eng, &c.ID, "D", 0)
	f.exec(t, "UPDATE nodes SET deleted_at = now() WHERE id = $1", c.ID)
	if got, err := f.s.LinkTargetsByKeys(ctx, f.eng, []string{"d"}); err == nil {
		t.Errorf("the path of %s under a deleted page = %+v, want an error", d.ID, got)
	}
}

// The title keys reindex takes anew are set on the nodes not deleted; a
// node deleted, or none, is a defect: an error, which rolls the unit back.
func TestSetNameKeysSetsTheNodesNotDeleted(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.page(t, f.eng, nil, "A", 0)
	gone := f.page(t, f.eng, nil, "Gone", 1)
	if err := f.s.DeleteNodes(ctx, []uuid.UUID{gone.ID}, f.alice, now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	a.NameKey = "new"
	if err := f.s.SetNameKeys(ctx, []domain.Node{a}); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND name_key = 'new'", a.ID); n != 1 {
		t.Errorf("A's key was not set")
	}
	gone.NameKey = "new"
	if err := f.s.SetNameKeys(ctx, []domain.Node{gone}); err == nil {
		t.Error("a deleted node's key was set")
	}
}

// Siblings' keys set at once may each take another's old one, x's y while
// y's goes to z, in either order: no key is held twice on the way.
func TestSetNameKeysPassesKeysOnAmongSiblings(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	x := f.page(t, f.eng, nil, "X", 0)
	y := f.page(t, f.eng, nil, "Y", 1)
	x.NameKey, y.NameKey = "y", "z"
	for _, nodes := range [][]domain.Node{{x, y}, {y, x}} {
		f.exec(t, "UPDATE nodes SET name_key = lower(name) WHERE id = ANY($1)", []uuid.UUID{x.ID, y.ID})
		if err := f.s.SetNameKeys(ctx, nodes); err != nil {
			t.Fatalf("set the keys in the order %s, %s: %v", nodes[0].Name, nodes[1].Name, err)
		}
		if n := f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND name_key = 'y' OR id = $2 AND name_key = 'z'", x.ID, y.ID); n != 2 {
			t.Errorf("in the order %s, %s, %d keys set, want 2", nodes[0].Name, nodes[1].Name, n)
		}
	}
}
