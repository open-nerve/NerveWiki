package postgresadapter_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// A missing field, asset, root, or unrelated live witness changes the tree a
// revert plans by. The explicit expected set also catches a shared filter bug
// if the public and dedicated reads ever both omit the same member.
func TestListRevertTreeReadsEveryLiveShape(t *testing.T) {
	f := newFixture(t)
	root := f.page(t, f.eng, nil, "Folder", 2.5)
	page := f.page(t, f.eng, &root.ID, "ÉTÉ", 7.25)
	asset := f.asset(t, f.eng, &root.ID, "Image.PNG", 3.75)
	f.exec(t, "UPDATE nodes SET name_key=$2 WHERE id=$1", asset.ID, "legacy-image-key")
	other := f.page(t, f.eng, nil, "Untouched", 9.5)
	gone := f.page(t, f.eng, nil, "Gone", 5)
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	f.page(t, f.ops, nil, "Elsewhere", 1)
	want := map[uuid.UUID]domain.RevertTreeNode{
		root.ID:  {ID: root.ID, Kind: domain.KindPage, Name: "Folder", NameKey: "folder", SortOrder: 2.5},
		page.ID:  {ID: page.ID, ParentID: &root.ID, Kind: domain.KindPage, Name: "ÉTÉ", NameKey: "été", SortOrder: 7.25},
		asset.ID: {ID: asset.ID, ParentID: &root.ID, Kind: domain.KindAsset, Name: "Image.PNG", NameKey: "legacy-image-key", SortOrder: 3.75},
		other.ID: {ID: other.ID, Kind: domain.KindPage, Name: "Untouched", NameKey: "untouched", SortOrder: 9.5},
	}
	read := func() {
		t.Helper()
		rows, err := f.s.ListRevertTree(t.Context(), f.eng)
		if err != nil {
			t.Fatal(err)
		}
		got := make(map[uuid.UUID]domain.RevertTreeNode, len(rows))
		for _, n := range rows {
			got[n.ID] = n
		}
		if len(rows) != len(want) || !reflect.DeepEqual(got, want) {
			t.Fatalf("revert tree = %+v; want every live shape %+v", rows, want)
		}
	}
	read()
	// Audit data is not a planning-tree input, but remains observable through
	// public ListNodes. Changing it must neither leak into nor shrink this read.
	at := now().Add(time.Hour)
	f.exec(t, "UPDATE nodes SET updated_at=$2 WHERE notebook_id=$1 AND deleted_at IS NULL", f.eng, at)
	read()
	public, err := f.s.ListNodes(t.Context(), f.eng)
	if err != nil || len(public) != len(want) {
		t.Fatalf("ListNodes = %+v, %v; want four complete nodes", public, err)
	}
	for _, n := range public {
		w, ok := want[n.ID]
		if !ok || !reflect.DeepEqual(domain.RevertTreeNode{ID: n.ID, ParentID: n.ParentID, Kind: n.Kind,
			Name: n.Name, NameKey: n.NameKey, SortOrder: n.SortOrder}, w) || n.NotebookID != f.eng ||
			n.CreatedBy != f.alice || n.UpdatedBy != f.alice || !n.CreatedAt.Equal(now()) || !n.UpdatedAt.Equal(at) {
			t.Errorf("public node = %+v; want its complete structural and audit fields", n)
		}
	}
}

func TestListRevertTreeEmptyIsRead(t *testing.T) {
	f := newFixture(t)
	got, err := f.s.ListRevertTree(t.Context(), f.eng)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty read = %#v, %v; want non-nil empty tree", got, err)
	}
}

func TestListRevertTreeUsesContextTransaction(t *testing.T) {
	f := newFixture(t)
	n := domain.Node{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.KindAsset, Name: "Pending.PNG", NameKey: "pending.png",
		SortOrder: 4.5, CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: now(), UpdatedAt: now()}
	want := []domain.RevertTreeNode{{ID: n.ID, Kind: domain.KindAsset, Name: "Pending.PNG", NameKey: "pending.png", SortOrder: 4.5}}
	rollback := errors.New("roll back the uncommitted tree")
	err := postgres.NewTxManager(f.pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
		if err := f.s.CreateNode(ctx, n); err != nil {
			return err
		}
		inside, err := f.s.ListRevertTree(ctx, f.eng)
		if err != nil || !reflect.DeepEqual(inside, want) {
			t.Errorf("transaction tree = %+v, %v; want uncommitted member %+v", inside, err, want)
		}
		outside, err := f.s.ListRevertTree(t.Context(), f.eng)
		if err != nil || outside == nil || len(outside) != 0 {
			t.Errorf("outside transaction = %#v, %v; want empty tree", outside, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE id=$1", n.ID); got != 0 {
		t.Fatalf("rollback kept %d rows", got)
	}
}

func TestListRevertTreeKeepsCancellation(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	rows, err := f.s.ListRevertTree(ctx, f.eng)
	if !errors.Is(err, context.Canceled) || rows != nil {
		t.Fatalf("cancelled tree = %#v, %v; want context.Canceled without partial tree", rows, err)
	}
}
