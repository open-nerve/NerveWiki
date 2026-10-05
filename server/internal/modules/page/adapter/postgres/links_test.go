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
		p.Steps = append(p.Steps, postgresadapter.LinkStep{ID: n.ID, Key: n.NameKey})
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

// A path that loops never reaches a root: a defect, which the read reports
// rather than answering a path cut short.
func TestALinkTargetsPathThatLoopsIsAnError(t *testing.T) {
	f := newFixture(t)
	a := f.page(t, f.eng, nil, "A", 0)
	b := f.page(t, f.eng, &a.ID, "B", 0)
	f.exec(t, "UPDATE nodes SET parent_id = $1 WHERE id = $2", b.ID, a.ID)
	if got, err := f.s.LinkTargetsByIDs(context.Background(), f.eng, []uuid.UUID{b.ID}); err == nil {
		t.Errorf("a loop's path = %+v, want an error", got)
	}
}
