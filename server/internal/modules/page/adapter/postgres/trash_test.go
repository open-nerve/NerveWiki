package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// trashed deletes the subtree ids, the first its root, at at, through the
// store, with counts.
func (f fixture) trashed(t *testing.T, at time.Time, pages, assets, height int, ids ...uuid.UUID) {
	t.Helper()
	d := app.Deletion{Root: ids[0], IDs: ids, Pages: pages, Assets: assets, Height: height, By: f.alice, At: at}
	if err := f.s.DeleteNodes(context.Background(), d); err != nil {
		t.Fatal(err)
	}
}

// asset adds an attachment of notebook under parent, through the store.
func (f fixture) asset(t *testing.T, notebook uuid.UUID, parent *uuid.UUID, name string, order float64) domain.Node {
	t.Helper()
	title, err := domain.CheckAssetName("name", name)
	if err != nil {
		t.Fatal(err)
	}
	n := domain.Node{ID: uuid.NewV7(), NotebookID: notebook, ParentID: parent, Kind: domain.KindAsset, Name: title.Name,
		NameKey: title.Key, SortOrder: order, CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: now(), UpdatedAt: now()}
	if err := f.s.CreateNode(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	return n
}

func rootNames(roots []domain.TrashRoot) []string {
	out := make([]string, len(roots))
	for i, r := range roots {
		out[i] = r.Node.Name
	}
	return out
}

// The trash lists a notebook's roots alone, deleted after since, newer
// first and by id at one time, below the cursor; each with its counts, its
// deleter and a page's content's revision at its deletion, none for an
// attachment or a content the purge took.
func TestListTrash(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a, b, c, d := f.page(t, f.eng, nil, "A", 0), f.page(t, f.eng, nil, "B", 1), f.page(t, f.eng, nil, "C", 2), f.page(t, f.eng, nil, "D", 3)
	child := f.page(t, f.eng, &a.ID, "Child", 0)
	photo := f.asset(t, f.eng, nil, "photo.png", 4)
	elsewhere := f.page(t, f.ops, nil, "Elsewhere", 0)
	f.page(t, f.eng, nil, "Alive", 5)
	f.exec(t, "UPDATE page_contents SET revision = 7 WHERE node_id = $1", a.ID)
	t0 := now()
	f.trashed(t, t0, 2, 0, 2, a.ID, child.ID)
	f.trashed(t, t0.Add(time.Minute), 1, 0, 1, b.ID)
	f.trashed(t, t0.Add(time.Minute), 1, 0, 1, c.ID)
	f.trashed(t, t0.Add(2*time.Minute), 0, 1, 0, photo.ID)
	f.trashed(t, t0.Add(3*time.Minute), 1, 0, 1, d.ID)
	f.trashed(t, t0.Add(3*time.Minute), 1, 0, 1, elsewhere.ID)
	f.exec(t, "DELETE FROM page_contents WHERE node_id = $1", d.ID)
	// b and c at one time: the larger id first.
	bc := []string{"B", "C"}
	if b.ID.Compare(c.ID) < 0 {
		bc = []string{"C", "B"}
	}

	all, err := f.s.ListTrash(ctx, f.eng, t0.Add(-time.Hour), domain.TrashFrom(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := slices.Concat([]string{"D", "photo.png"}, bc, []string{"A"}); !slices.Equal(rootNames(all), want) {
		t.Fatalf("ListTrash() = %v, want %v", rootNames(all), want)
	}
	first := all[len(all)-1]
	if first.Node.ID != a.ID || first.Node.UpdatedBy != f.alice || !first.DeletedAt.Equal(t0) || first.Pages != 2 || first.Assets != 0 ||
		first.Height != 2 || first.Revision != 7 {
		t.Errorf("A's item = %+v; want deleted by alice at %s, 2 pages, 0 attachments, 2 levels, revision 7", first, t0)
	}
	if all[0].Revision != 0 || all[1].Revision != 0 || all[1].Assets != 1 || all[1].Node.Kind != domain.KindAsset {
		t.Errorf("D's and the photo's items = %+v, %+v; want no revision, the photo an attachment", all[0], all[1])
	}

	since, err := f.s.ListTrash(ctx, f.eng, t0, domain.TrashFrom(), 10)
	if err != nil || slices.Contains(rootNames(since), "A") || len(since) != 4 {
		t.Errorf("ListTrash(after A's time) = %v, %v; want all but A", rootNames(since), err)
	}
	page, err := f.s.ListTrash(ctx, f.eng, t0.Add(-time.Hour), domain.TrashFrom(), 2)
	if err != nil || len(page) != 2 {
		t.Fatalf("ListTrash(2) = %v, %v", rootNames(page), err)
	}
	below := domain.TrashCursor{At: page[1].DeletedAt, ID: page[1].Node.ID}
	next, err := f.s.ListTrash(ctx, f.eng, t0.Add(-time.Hour), below, 2)
	if want := slices.Concat(bc, nil); err != nil || !slices.Equal(rootNames(next), want) {
		t.Errorf("ListTrash(below the photo, 2) = %v, %v; want %v", rootNames(next), err, want)
	}
	below = domain.TrashCursor{At: next[0].DeletedAt, ID: next[0].Node.ID}
	if after, err := f.s.ListTrash(ctx, f.eng, t0.Add(-time.Hour), below, 10); err != nil || !slices.Equal(rootNames(after), []string{bc[1], "A"}) {
		t.Errorf("ListTrash(below %s, at its time) = %v, %v; want %s, A", bc[0], rootNames(after), err, bc[1])
	}

	if root, err := f.s.FindTrashRoot(ctx, a.ID); err != nil || root.Node.ID != a.ID || root.Pages != 2 || root.Revision != 7 {
		t.Errorf("FindTrashRoot(A) = %+v, %v; want A's item", root, err)
	}
	for what, id := range map[string]uuid.UUID{"a member": child.ID, "alive": f.page(t, f.eng, nil, "E", 6).ID, "none": uuid.NewV7()} {
		if _, err := f.s.FindTrashRoot(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("FindTrashRoot(%s) = %v, want ErrNotFound", what, err)
		}
	}
	if n, err := f.s.FindAnyNode(ctx, a.ID); err != nil || !n.TrashRoot {
		t.Errorf("FindAnyNode(A) = %+v, %v; want a trash root", n, err)
	}
	if n, err := f.s.FindAnyNode(ctx, child.ID); err != nil || n.TrashRoot {
		t.Errorf("FindAnyNode(A's child) = %+v, %v; want no trash root", n, err)
	}
}

// A deleted node's item is its group's root, up its parents deleted at its
// time; a descendant deleted before is an item of its own.
func TestTrashRootOf(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	r := f.page(t, f.eng, nil, "R", 0)
	c := f.page(t, f.eng, &r.ID, "C", 0)
	g := f.page(t, f.eng, &c.ID, "G", 0)
	e := f.page(t, f.eng, &g.ID, "E", 0)
	alive := f.page(t, f.eng, nil, "Alive", 1)
	f.trashed(t, now(), 1, 0, 1, e.ID)
	f.trashed(t, now().Add(time.Minute), 3, 0, 3, r.ID, c.ID, g.ID)
	for _, tt := range []struct {
		name     string
		id, root uuid.UUID
	}{{"the root", r.ID, r.ID}, {"its grandchild", g.ID, r.ID}, {"a descendant deleted before", e.ID, e.ID}} {
		if got, err := f.s.TrashRootOf(ctx, tt.id); err != nil || got != tt.root {
			t.Errorf("TrashRootOf(%s) = %s, %v; want %s", tt.name, got, err, tt.root)
		}
	}
	for what, id := range map[string]uuid.UUID{"alive": alive.ID, "none": uuid.NewV7()} {
		if _, err := f.s.TrashRootOf(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("TrashRootOf(%s) = %v, want ErrNotFound", what, err)
		}
	}
}

// An item's line is its root's ancestors from the notebook's root down,
// deleted or not, each with when it was deleted.
func TestTrashLines(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	p := f.page(t, f.eng, nil, "P", 0)
	q := f.page(t, f.eng, &p.ID, "Q", 0)
	d := f.page(t, f.eng, &q.ID, "D", 0)
	top := f.page(t, f.eng, nil, "Top", 1)
	f.trashed(t, now(), 1, 0, 1, d.ID)
	at := now().Add(time.Minute)
	f.trashed(t, at, 1, 0, 1, q.ID)
	f.trashed(t, at, 1, 0, 1, top.ID)
	lines, err := f.s.TrashLines(ctx, []uuid.UUID{d.ID, q.ID, top.ID})
	if err != nil {
		t.Fatal(err)
	}
	line := func(id uuid.UUID) string {
		var out []string
		for _, a := range lines[id] {
			deleted := "alive"
			if a.DeletedAt != nil {
				deleted = "deleted " + a.DeletedAt.UTC().Format(time.TimeOnly)
			}
			out = append(out, fmt.Sprintf("%s %s %s", a.ID, a.Name, deleted))
		}
		return fmt.Sprint(out)
	}
	if got, want := line(d.ID), fmt.Sprint([]string{p.ID.String() + " P alive", q.ID.String() + " Q deleted " + at.UTC().Format(time.TimeOnly)}); got != want {
		t.Errorf("D's line = %s, want %s", got, want)
	}
	if got, want := line(q.ID), fmt.Sprint([]string{p.ID.String() + " P alive"}); got != want {
		t.Errorf("Q's line = %s, want %s", got, want)
	}
	if _, ok := lines[top.ID]; ok {
		t.Errorf("Top's line = %s, want none: it was at the root", line(top.ID))
	}
}

// An item's group is its root and the descendants deleted at its time,
// level by level in order, each page with its content's revision and size
// at that time; one of another notebook or time is none.
func TestTrashGroupAndContents(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	r := f.page(t, f.eng, nil, "R", 0)
	b, a := f.page(t, f.eng, &r.ID, "B", 1), f.page(t, f.eng, &r.ID, "A", 0)
	photo := f.asset(t, f.eng, &a.ID, "photo.png", 0)
	earlier := f.page(t, f.eng, &b.ID, "Earlier", 0)
	f.exec(t, "UPDATE page_contents SET content = 'abc', byte_size = 3, content_hash = sha256('abc'), revision = 4 WHERE node_id = $1", a.ID)
	f.trashed(t, now(), 1, 0, 1, earlier.ID)
	at := now().Add(time.Minute)
	f.trashed(t, at, 3, 1, 2, r.ID, b.ID, a.ID, photo.ID, earlier.ID)
	group, err := f.s.TrashGroup(ctx, f.eng, r.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range group {
		got = append(got, fmt.Sprintf("%s@%d r%d %db", m.Node.Name, m.Level, m.Revision, m.Bytes))
	}
	if want := []string{"R@1 r1 0b", "A@2 r4 3b", "B@2 r1 0b", "photo.png@3 r0 0b"}; !slices.Equal(got, want) {
		t.Errorf("TrashGroup(R) = %v, want %v", got, want)
	}
	if group[0].Node.UpdatedBy != f.alice || group[0].Node.NotebookID != f.eng {
		t.Errorf("TrashGroup(R)'s root = %+v, want R as it reads", group[0].Node)
	}
	for what, args := range map[string]struct {
		notebook uuid.UUID
		at       time.Time
	}{"another notebook": {f.ops, at}, "another time": {f.eng, now()}} {
		if _, err := f.s.TrashGroup(ctx, args.notebook, r.ID, args.at); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("TrashGroup(R of %s) = %v, want ErrNotFound", what, err)
		}
	}
	f.exec(t, "DELETE FROM page_contents WHERE node_id = $1", b.ID)
	group, err = f.s.TrashGroup(ctx, f.eng, r.ID, at)
	if err != nil || len(group) != 4 || group[2].Node.ID != b.ID || group[2].Revision != 0 {
		t.Errorf("TrashGroup(R) without B's content = %+v, %v; want B at revision 0", group, err)
	}
	contents, err := f.s.TrashContents(ctx, []uuid.UUID{r.ID, a.ID, b.ID, earlier.ID}, at)
	if err != nil || len(contents) != 2 || contents[a.ID] != "abc" || contents[r.ID] != "" {
		t.Errorf("TrashContents() = %q, %v; want R's and A's, not B's taken nor Earlier's of another time", contents, err)
	}
}

// restoration is the restore of the trash item ids, the first its root,
// deleted at at, under parent at order, each by its name: a test's.
func restoration(at time.Time, parent *uuid.UUID, order float64, by uuid.UUID, nodes ...domain.Node) app.Restoration {
	r := app.Restoration{Root: nodes[0].ID, ParentID: parent, SortOrder: order, DeletedAt: at, By: by, At: at.Add(time.Hour)}
	for _, n := range nodes {
		r.IDs, r.Names, r.NameKeys = append(r.IDs, n.ID), append(r.Names, n.Name), append(r.NameKeys, n.NameKey)
	}
	return r
}

// A restore brings back the nodes deleted at the item's time, each with
// its name, the root under its parent at its order, by its caller at its
// time, no item any more; and what followed them at that time, a content
// keeping its writer; what the trash held before keeps its own time. It
// counts what came back.
func TestRestoreNodesBringsTheGroupBack(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	bob := uuid.NewV7()
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'bob@corp.com', 'x', 'x', $2, $2)",
		bob, now())
	p := f.page(t, f.eng, nil, "P", 0)
	r := f.page(t, f.eng, nil, "R", 1)
	a := f.page(t, f.eng, &r.ID, "A", 0)
	photo := f.asset(t, f.eng, &a.ID, "photo.png", 0)
	earlier := f.page(t, f.eng, &r.ID, "Earlier", 1)
	f.versions(t, r.ID, 3)
	f.versions(t, a.ID, 2)
	f.versions(t, earlier.ID, 1)
	f.exec(t, "UPDATE page_contents SET revision = CASE node_id WHEN $1 THEN 3 ELSE 2 END WHERE node_id IN ($1, $2)", r.ID, a.ID)
	cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientWeb, By: f.alice, At: now()}
	if err := f.s.CreateChangeset(ctx, cs); err != nil {
		t.Fatal(err)
	}
	for _, n := range []domain.Node{r, a, photo, earlier} {
		state := n.State()
		if err := f.s.RecordItems(ctx, []app.Item{{ID: uuid.NewV7(), ChangesetID: cs.ID, Change: domain.Change{NodeID: n.ID, After: &state},
			At: now()}}); err != nil {
			t.Fatal(err)
		}
	}
	f.trashed(t, now(), 1, 0, 1, earlier.ID)
	at := now().Add(time.Minute)
	f.trashed(t, at, 2, 1, 2, r.ID, a.ID, photo.ID, earlier.ID)
	r.Name, r.NameKey = "R 2", "r 2"
	back := restoration(at, &p.ID, 7, bob, r, a, photo)
	rows, err := f.s.RestoreNodes(ctx, back)
	if err != nil {
		t.Fatal(err)
	}
	if want := (app.RestoredRows{Nodes: 3, Contents: 2, CurrentVersions: 2, Creations: 2}); rows != want {
		t.Errorf("RestoreNodes() = %+v, want %+v", rows, want)
	}
	if got := f.count(t, `SELECT count(*) FROM nodes WHERE id = $1 AND parent_id = $2 AND sort_order = 7 AND name = 'R 2'
		AND name_key = 'r 2'`, r.ID, p.ID); got != 1 {
		t.Error("R is not back under P at its order, by its new name")
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND parent_id = $2 AND sort_order = 0", a.ID, r.ID); got != 1 {
		t.Error("A is not back under R at its order")
	}
	for _, n := range []domain.Node{r, a, photo} {
		if got := f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL AND updated_by_id = $2 AND updated_at = $3",
			n.ID, bob, back.At); got != 1 {
			t.Errorf("%s is not back by bob at the restore's time", n.Name)
		}
		if got := f.trashMark(t, n.ID); got != "false -1 -1 -1" {
			t.Errorf("%s's mark = %s, want none", n.Name, got)
		}
		if got := f.count(t, "SELECT count(*) FROM changeset_items WHERE node_id = $1 AND deleted_at IS NULL", n.ID); got != 1 {
			t.Errorf("%s's items back = %d, want its edit's", n.Name, got)
		}
	}
	if got := f.count(t, "SELECT count(*) FROM page_revisions WHERE node_id IN ($1, $2) AND deleted_at IS NULL", r.ID, a.ID); got != 5 {
		t.Errorf("versions back = %d, want R's 3 and A's 2", got)
	}
	if got := f.count(t, "SELECT count(*) FROM page_contents WHERE node_id IN ($1, $2) AND deleted_at IS NULL AND updated_by_id = $3",
		r.ID, a.ID, f.alice); got != 2 {
		t.Error("the contents are not back with their writer")
	}
	for _, table := range append([]string{"nodes"}, followers()...) {
		column := "node_id"
		if table == "nodes" {
			column = "id"
		}
		if got := f.count(t, "SELECT count(*) FROM "+table+" WHERE "+column+" = $1 AND deleted_at = $2", earlier.ID, now()); got == 0 {
			t.Errorf("Earlier's row of %s lost its own time", table)
		}
	}
}

// What the purge took of an item, or a restore meanwhile, shows in the
// counts: a content taken, a current version, a creation, a version
// between, a node no longer deleted at the item's time. A name an alive
// sibling holds is a title taken.
func TestRestoreNodesCountsWhatCameBack(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	for _, tt := range []struct {
		name string
		take string
		want app.RestoredRows
	}{
		{"nothing", "", app.RestoredRows{Nodes: 1, Contents: 1, CurrentVersions: 1, Creations: 1}},
		{"the content", "DELETE FROM page_contents WHERE node_id = $1", app.RestoredRows{Nodes: 1, Creations: 1}},
		{"the current version", "DELETE FROM page_revisions WHERE node_id = $1 AND revision = 3",
			app.RestoredRows{Nodes: 1, Contents: 1, Creations: 1}},
		{"the creation", "DELETE FROM page_revisions WHERE node_id = $1 AND revision = 1",
			app.RestoredRows{Nodes: 1, Contents: 1, CurrentVersions: 1, Gaps: 1}},
		{"a version between", "DELETE FROM page_revisions WHERE node_id = $1 AND revision = 2",
			app.RestoredRows{Nodes: 1, Contents: 1, CurrentVersions: 1, Creations: 1, Gaps: 1}},
		{"the node restored", "UPDATE nodes SET deleted_at = NULL, trash_root = false, trash_pages = NULL, trash_assets = NULL, " +
			"trash_height = NULL WHERE id = $1", app.RestoredRows{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			x := f.page(t, f.eng, nil, tt.name, 0)
			f.versions(t, x.ID, 3)
			f.exec(t, "UPDATE page_contents SET revision = 3 WHERE node_id = $1", x.ID)
			f.trashed(t, now(), 1, 0, 1, x.ID)
			if tt.take != "" {
				f.exec(t, tt.take, x.ID)
			}
			x.Name = tt.name + " back"
			if rows, err := f.s.RestoreNodes(ctx, restoration(now(), nil, 0, f.alice, x)); err != nil || rows != tt.want {
				t.Errorf("RestoreNodes(%s taken) = %+v, %v; want %+v", tt.name, rows, err, tt.want)
			}
		})
	}
	taken := f.page(t, f.eng, nil, "Taken", 1)
	f.trashed(t, now(), 1, 0, 1, taken.ID)
	f.page(t, f.eng, nil, "taken", 2)
	if _, err := f.s.RestoreNodes(ctx, restoration(now(), nil, 1, f.alice, taken)); !errors.Is(err, domain.ErrTitleTaken) {
		t.Errorf("RestoreNodes(a name a sibling holds) = %v, want page.title_taken", err)
	}
}
