package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// A rollback's reads (M11/P1 design 3.18): what is left, the baseline
// nodes not in place (moved, renamed, reordered only, or not alive), the
// baseline pages of another content and the live nodes with no baseline
// row, a protected row's node among them, a deleted one not; the free
// nodes, new ones alive holding no baseline node, with their parents; the
// holders, the new ancestors of baseline nodes alive; the baseline, each
// row as captured. Another upgrade's rows, of the same notebook, change
// none of it; DeleteUpgradeNodes deletes the upgrade's rows and no other.
func TestARollbacksReads(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	guide := f.page(t, f.eng, nil, "Guide", 0)
	install := f.page(t, f.eng, &guide.ID, "Install", 0)
	photo := f.asset(t, f.eng, &guide.ID, "photo.png", 1)
	logo := f.asset(t, f.eng, nil, "logo.png", 1)
	notes := f.page(t, f.eng, nil, "Notes", 2)
	old := f.page(t, f.eng, nil, "Old", 3)
	f.write(t, notes.ID, "notes", 2, now())
	f.page(t, f.ops, nil, "Ops", 0)
	upgrade, other := uuid.NewV7(), uuid.NewV7()
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	capture := func(id uuid.UUID) {
		t.Helper()
		if err := tx.WithinTx(ctx, func(ctx context.Context) error {
			_, err := f.s.CaptureBaseline(ctx, id, f.eng)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	capture(upgrade)

	// The agent's: a free subtree at the root, protected as a source
	// root is; a holder of Install, a free page under it; a free page
	// under Guide; a new page deleted; Guide written; photo renamed;
	// Notes reordered; Old deleted by a hand's SQL.
	scratch := f.page(t, f.eng, nil, "Scratch", 10)
	scratch1 := f.page(t, f.eng, &scratch.ID, "Scratch 1", 0)
	topics := f.page(t, f.eng, nil, "Topics", 11)
	folder := f.page(t, f.eng, &topics.ID, "Folder", 1)
	child := f.page(t, f.eng, &guide.ID, "Child", 5)
	gone := f.page(t, f.eng, nil, "Gone", 12)
	f.exec(t, "UPDATE nodes SET deleted_at = now() WHERE id = ANY($1)", []uuid.UUID{gone.ID, old.ID})
	f.exec(t, "UPDATE page_contents SET deleted_at = now() WHERE node_id = ANY($1)", []uuid.UUID{gone.ID, old.ID})
	f.exec(t, "UPDATE nodes SET parent_id = $2 WHERE id = $1", install.ID, topics.ID)
	f.exec(t, "UPDATE nodes SET name = 'shot.png', name_key = 'shot.png' WHERE id = $1", photo.ID)
	f.exec(t, "UPDATE nodes SET sort_order = 2.5 WHERE id = $1", notes.ID)
	f.write(t, guide.ID, "guide, the agent's", 2, now())
	if err := tx.WithinTx(ctx, func(ctx context.Context) error {
		return f.s.ProtectSourceRoot(ctx, upgrade, f.eng, scratch.ID)
	}); err != nil {
		t.Fatal(err)
	}
	// Another upgrade of the notebook, its baseline the notebook now: by
	// it nothing is left, every node in place.
	capture(other)

	left, err := f.s.RollbackLeft(ctx, upgrade, f.eng)
	if want := (domain.RollbackLeft{Nodes: 4, Pages: 1, Created: 5}); err != nil || left != want {
		t.Errorf("RollbackLeft = %+v, %v; want %+v: Install, photo, Notes, Old; Guide; five new", left, err, want)
	}
	if left, err := f.s.RollbackLeft(ctx, other, f.eng); err != nil || !left.Zero() {
		t.Errorf("the other upgrade's RollbackLeft = %+v, %v; want nothing", left, err)
	}
	if left, err := f.s.RollbackLeft(ctx, upgrade, f.ops); err != nil || left != (domain.RollbackLeft{Nodes: 4, Pages: 1, Created: 1}) {
		t.Errorf("RollbackLeft of another notebook = %+v, %v; want its one node new, the baseline's rows its own", left, err)
	}

	forest, err := f.s.RollbackFreeForest(ctx, upgrade, f.eng)
	slices.SortFunc(forest, func(a, b domain.FreeNode) int { return a.ID.Compare(b.ID) })
	wantForest := []domain.FreeNode{{ID: scratch.ID}, {ID: scratch1.ID, ParentID: &scratch.ID}, {ID: folder.ID, ParentID: &topics.ID},
		{ID: child.ID, ParentID: &guide.ID}}
	slices.SortFunc(wantForest, func(a, b domain.FreeNode) int { return a.ID.Compare(b.ID) })
	if err != nil || !slices.EqualFunc(forest, wantForest, func(a, b domain.FreeNode) bool {
		return a.ID == b.ID && domain.SameParent(a.ParentID, b.ParentID)
	}) {
		t.Errorf("RollbackFreeForest = %+v, %v; want Scratch, Scratch 1, Folder, Child", forest, err)
	}
	if holders, err := f.s.RollbackHolders(ctx, upgrade); err != nil || len(holders) != 1 || holders[0].ID != topics.ID ||
		holders[0].Name != "Topics" || holders[0].ParentID != nil || holders[0].SortOrder != 11 || holders[0].NotebookID != f.eng {
		t.Errorf("RollbackHolders = %+v, %v; want Topics %s alone, as it is", holders, err, topics.ID)
	}
	if holders, err := f.s.RollbackHolders(ctx, other); err != nil || len(holders) != 0 {
		t.Errorf("the other upgrade's holders = %v, %v; want none", holders, err)
	}

	baseline, err := f.s.RollbackBaseline(ctx, upgrade)
	slices.SortFunc(baseline, func(a, b domain.BaselineNode) int { return a.NodeID.Compare(b.NodeID) })
	empty := sha256.Sum256(nil)
	page := func(n domain.Node, revision int, hash []byte, size int) domain.BaselineNode {
		return domain.BaselineNode{NodeID: n.ID, Kind: domain.KindPage, ParentID: n.ParentID, Name: n.Name, SortOrder: n.SortOrder,
			Revision: revision, Hash: hash, Bytes: size}
	}
	asset := func(n domain.Node) domain.BaselineNode {
		return domain.BaselineNode{NodeID: n.ID, Kind: domain.KindAsset, ParentID: n.ParentID, Name: n.Name, SortOrder: n.SortOrder}
	}
	notesHash := sha256.Sum256([]byte("notes"))
	want := []domain.BaselineNode{page(guide, 1, empty[:], 0), page(install, 1, empty[:], 0), asset(photo), asset(logo),
		page(notes, 2, notesHash[:], 5), page(old, 1, empty[:], 0)}
	slices.SortFunc(want, func(a, b domain.BaselineNode) int { return a.NodeID.Compare(b.NodeID) })
	if err != nil || !slices.EqualFunc(baseline, want, func(a, b domain.BaselineNode) bool {
		return a.NodeID == b.NodeID && a.Kind == b.Kind && domain.SameParent(a.ParentID, b.ParentID) && a.Name == b.Name &&
			a.SortOrder == b.SortOrder && a.Revision == b.Revision && slices.Equal(a.Hash, b.Hash) && a.Bytes == b.Bytes
	}) {
		t.Errorf("RollbackBaseline = %+v, %v;\nwant %+v", baseline, err, want)
	}

	now, contents, err := f.s.RollbackNow(ctx, upgrade)
	at := func(id uuid.UUID) string {
		n := now[id]
		return n.Name + "@" + fmt.Sprint(n.SortOrder)
	}
	if err != nil || len(now) != 5 || now[install.ID].ParentID == nil || *now[install.ID].ParentID != topics.ID ||
		at(photo.ID) != "shot.png@1" || at(notes.ID) != "Notes@2.5" || at(guide.ID) != "Guide@0" || at(logo.ID) != "logo.png@1" ||
		now[guide.ID].NotebookID != f.eng || now[guide.ID].Kind != domain.KindPage {
		t.Errorf("RollbackNow's nodes = %+v, %v; want the five alive as they are, Old not", now, err)
	}
	agents := sha256.Sum256([]byte("guide, the agent's"))
	content := func(id uuid.UUID, revision int, hash []byte, size int) bool {
		c, ok := contents[id]
		return ok && c.Revision == revision && slices.Equal(c.Hash, hash) && c.Bytes == size
	}
	if len(contents) != 3 || !content(guide.ID, 2, agents[:], 18) || !content(notes.ID, 2, notesHash[:], 5) ||
		!content(install.ID, 1, empty[:], 0) {
		t.Errorf("RollbackNow's contents = %+v; want Guide's, Notes's and Install's, Old's deleted", contents)
	}

	if n, err := f.s.DeleteUpgradeNodes(ctx, upgrade); err != nil || n != 7 {
		t.Errorf("DeleteUpgradeNodes = %d, %v; want the six baseline rows and the protected one", n, err)
	}
	if n := f.count(t, "SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1", upgrade); n != 0 {
		t.Errorf("the upgrade has %d rows left, want none", n)
	}
	if n := f.count(t, "SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1", other); n != 10 {
		t.Errorf("the other upgrade has %d rows, want its 10", n)
	}
}
