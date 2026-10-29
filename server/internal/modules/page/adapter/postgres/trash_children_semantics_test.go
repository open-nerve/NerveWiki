package postgresadapter_test

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func trashChildrenInt32(n int32) *int32 { return &n }

// Map known fixture data to all thirteen public query columns. Neither the
// query under test nor Store's row/member conversion supplies this oracle.
func trashChildrenExpectedRow(n domain.Node, revision, size *int32) gen.TrashChildrenOfAllRow {
	return gen.TrashChildrenOfAllRow{ID: n.ID, NotebookID: n.NotebookID, ParentID: n.ParentID, Kind: string(n.Kind), Name: n.Name, NameKey: n.NameKey,
		SortOrder: n.SortOrder, CreatedByID: n.CreatedBy, UpdatedByID: n.UpdatedBy, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, Revision: revision, ByteSize: size}
}

// Ordinary adapter metadata, created/deleted through existing Store fixture
// methods. No complete source HTTP revision/file history is claimed here.
func TestTrashChildrenKeepTheirFieldsTimesAndGlobalOrder(t *testing.T) {
	f := newFixture(t)
	root := f.page(t, f.eng, nil, "Root", 0)
	parents := []domain.Node{f.page(t, f.eng, &root.ID, "Parent A", 0), f.page(t, f.eng, &root.ID, "Middle parent", 1), f.page(t, f.eng, &root.ID, "Parent B", 2)}
	slices.SortFunc(parents, func(a, b domain.Node) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	a, unselected, b := parents[0], parents[1], parents[2]
	for i, n := range []*domain.Node{&a, &unselected, &b} {
		n.SortOrder = float64(i)
		f.exec(t, "UPDATE nodes SET sort_order=$2 WHERE id=$1", n.ID, n.SortOrder)
	}
	// A and B are UUID extremes. A populated same-book/time parent lies
	// strictly between them: a merged[min,max] envelope must not include it.
	outside := f.page(t, f.eng, &unselected.ID, "Unselected parent child", 2.5)
	low := f.page(t, f.eng, &b.ID, "Low", 0)
	tieA := f.page(t, f.eng, &a.ID, "Tie A", 1)
	tieB := f.page(t, f.eng, &b.ID, "Tie B", 1)
	high := f.page(t, f.eng, &a.ID, "High", 2)
	asset := f.asset(t, f.eng, &b.ID, "photo.png", 3)
	purged := f.page(t, f.eng, &a.ID, "Purged content", 4)
	mismatch := f.page(t, f.eng, &b.ID, "Other content time", 5)
	earlier := f.page(t, f.eng, &a.ID, "Earlier deletion", -1)
	foreign := f.page(t, f.ops, nil, "Foreign parent", 0)
	foreignChild := f.page(t, f.ops, &foreign.ID, "Foreign child", 0)
	f.exec(t, "UPDATE page_contents SET content='abc',byte_size=3,content_hash=sha256(convert_to('abc','UTF8')),revision=4 WHERE node_id=$1", low.ID)
	earlyAt, at := now(), now().Add(time.Minute)
	f.trashed(t, earlyAt, 1, 0, 1, earlier.ID)
	f.trashed(t, at, 11, 1, 3, root.ID, a.ID, unselected.ID, b.ID, low.ID, tieA.ID, tieB.ID, high.ID, outside.ID, asset.ID, purged.ID, mismatch.ID)
	f.trashed(t, at, 2, 0, 2, foreign.ID, foreignChild.ID)
	f.exec(t, "DELETE FROM page_contents WHERE node_id=$1", purged.ID)
	f.exec(t, "UPDATE page_contents SET deleted_at=$2 WHERE node_id=$1", mismatch.ID, earlyAt)
	for _, n := range []*domain.Node{&root, &a, &unselected, &b, &outside, &low, &tieA, &tieB, &high, &asset, &purged, &mismatch, &foreign, &foreignChild} {
		n.UpdatedAt = at
	}
	// The two equal-order rows belong to different parents. UUID byte order,
	// not the input parent order or either parent's local order, breaks ties.
	firstTie, secondTie := tieA, tieB
	if bytes.Compare(firstTie.ID[:], secondTie.ID[:]) > 0 {
		firstTie, secondTie = secondTie, firstTie
	}
	one, zero := trashChildrenInt32(1), trashChildrenInt32(0)
	want := []gen.TrashChildrenOfAllRow{
		trashChildrenExpectedRow(low, trashChildrenInt32(4), trashChildrenInt32(3)),
		trashChildrenExpectedRow(firstTie, one, zero), trashChildrenExpectedRow(secondTie, one, zero),
		trashChildrenExpectedRow(high, one, zero), trashChildrenExpectedRow(asset, nil, nil),
		trashChildrenExpectedRow(purged, nil, nil), trashChildrenExpectedRow(mismatch, nil, nil),
	}
	unknown := uuid.NewV7()
	for _, tc := range []struct {
		name     string
		notebook uuid.UUID
		parents  []uuid.UUID
		want     []gen.TrashChildrenOfAllRow
	}{
		{"selected extremes exclude middle", f.eng, []uuid.UUID{a.ID, b.ID}, want},
		{"parents reversed", f.eng, []uuid.UUID{b.ID, a.ID}, want},
		{"duplicate parents", f.eng, []uuid.UUID{a.ID, b.ID, a.ID, b.ID}, want},
		{"unknown parent", f.eng, []uuid.UUID{unknown}, nil},
		{"nil array", f.eng, nil, nil},
		{"empty array", f.eng, []uuid.UUID{}, nil},
		{"foreign parent", f.eng, []uuid.UUID{foreign.ID}, nil},
		{"mixed parents", f.eng, []uuid.UUID{b.ID, foreign.ID, unknown, a.ID, a.ID}, want},
		{"another notebook", f.ops, []uuid.UUID{a.ID, b.ID}, nil},
		{"other notebook own parent", f.ops, []uuid.UUID{a.ID, foreign.ID}, []gen.TrashChildrenOfAllRow{trashChildrenExpectedRow(foreignChild, one, zero)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := gen.New(postgres.Planned(f.pool)).TrashChildrenOfAll(t.Context(), gen.TrashChildrenOfAllParams{NotebookID: tc.notebook, Parents: tc.parents, DeletedAt: at})
			if err != nil || len(got) != len(tc.want) || len(got) > 0 && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("all13 columns/order = %+v err=%v want %+v", got, err, tc.want)
			}
		})
	}
	wantGroup := []domain.TrashMember{
		{Node: root, Level: 1, Revision: 1}, {Node: a, Level: 2, Revision: 1}, {Node: unselected, Level: 2, Revision: 1}, {Node: b, Level: 2, Revision: 1},
		{Node: low, Level: 3, Revision: 4, Bytes: 3}, {Node: firstTie, Level: 3, Revision: 1}, {Node: secondTie, Level: 3, Revision: 1},
		{Node: high, Level: 3, Revision: 1}, {Node: outside, Level: 3, Revision: 1}, {Node: asset, Level: 3}, {Node: purged, Level: 3}, {Node: mismatch, Level: 3},
	}
	group, err := f.s.TrashGroup(t.Context(), f.eng, root.ID, at)
	if err != nil || !reflect.DeepEqual(group, wantGroup) {
		t.Fatalf("real TrashGroup = %+v err=%v want %+v", group, err, wantGroup)
	}
}

// The bounded source reader and ordinary whole-trash reader have different
// contracts. Currently MaxUnitNodes+1 is10001 children, complete and ordered;
// it cannot accidentally reuse the source reader's10000-node unit limit.
func TestOrdinaryTrashGroupIsCompletePastSourceLimit(t *testing.T) {
	f := newFixture(t)
	root := f.page(t, f.eng, nil, "Wide root", 0)
	const children = domain.MaxUnitNodes + 1
	f.sourceChildren(t, f.eng, root.ID, children) // adapter metadata; these pages have no content rows
	bounded, complete, err := f.s.SubtreeWithin(t.Context(), f.eng, root.ID, domain.MaxUnitNodes)
	if err != nil || complete || bounded != nil {
		t.Fatalf("bounded source read=%d complete=%t err=%v wantnil/false/nil", len(bounded), complete, err)
	}
	rows, err := f.pool.Query(t.Context(), "SELECT id FROM nodes WHERE notebook_id=$1 AND parent_id=$2 ORDER BY sort_order,id", f.eng, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil || len(ids) != children {
		t.Fatalf("fixture children=%d err=%v", len(ids), err)
	}
	at := now().Add(time.Minute)
	f.trashed(t, at, children+1, 0, 2, append([]uuid.UUID{root.ID}, ids...)...)
	root.UpdatedAt = at
	s, tr, _ := trashChildrenPool(t, f, "auto")
	group, err := s.TrashGroup(t.Context(), f.eng, root.ID, at)
	if err != nil || len(group) != children+1 {
		t.Fatalf("whole TrashGroup nodes=%d err=%v want%d", len(group), err, children+1)
	}
	if !reflect.DeepEqual(group[0], domain.TrashMember{Node: root, Level: 1, Revision: 1}) {
		t.Fatalf("wrong wide root: %+v", group[0])
	}
	height := 1
	for i, member := range group[1:] {
		want := domain.Node{ID: ids[i], NotebookID: f.eng, ParentID: &root.ID, Kind: domain.KindPage, Name: fmt.Sprintf("Child %d", children-i), NameKey: fmt.Sprintf("child %08d", children-i), SortOrder: float64(i), CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: now(), UpdatedAt: at}
		if !reflect.DeepEqual(member, domain.TrashMember{Node: want, Level: 2}) {
			t.Fatalf("wide member%d = %+v want %+v", i+1, member, want)
		}
		if member.Level > height {
			height = member.Level
		}
	}
	if height != 2 || len(tr.reads) != 2 {
		t.Fatalf("wide height=%d child reads=%d want2/2", height, len(tr.reads))
	}
	for i, read := range tr.reads {
		parents := []uuid.UUID{root.ID}
		if i == 1 {
			parents = ids
		}
		want := statementOf(t, func(q *gen.Queries) error {
			_, err := q.TrashChildrenOfAll(context.Background(), gen.TrashChildrenOfAllParams{NotebookID: f.eng, Parents: parents, DeletedAt: at})
			return err
		})
		if read.SQL != want.sql || !reflect.DeepEqual(read.Args, want.args) || read.Mode != pgx.QueryExecModeDescribeExec || read.ModeArguments != 1 || read.Rows != []int64{children, 0}[i] || read.Err != nil {
			t.Fatalf("wide Store level%d routed/capped its real generated query: %+v", i+2, read)
		}
	}
}
