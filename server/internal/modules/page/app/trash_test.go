package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// retention is the trash fixture's purge retention.
const retention = 30 * 24 * time.Hour

// trashed moves the subtree of the node id to the trash at at, as a
// deletion's unit does, by by, without recording a call: its root an item
// with its counts, its pages' contents kept.
func (f *fixture) trashed(id uuid.UUID, at time.Time, by uuid.UUID) {
	var ids []uuid.UUID
	var walk func(uuid.UUID)
	walk = func(id uuid.UUID) {
		ids = append(ids, id)
		for _, n := range f.store.nodes {
			if n.ParentID != nil && *n.ParentID == id {
				walk(n.ID)
			}
		}
	}
	walk(id)
	pages, assets, height := 0, 0, 0
	depth := map[uuid.UUID]int{id: 1}
	for _, n := range ids {
		node := f.store.nodes[n]
		if node.ParentID != nil && n != id {
			depth[n] = depth[*node.ParentID] + 1
		}
		if node.Kind == domain.KindAsset {
			assets++
		} else {
			pages++
			height = max(height, depth[n])
		}
	}
	for _, n := range ids {
		node := f.store.nodes[n]
		node.UpdatedBy, node.UpdatedAt = by, at
		f.store.nodes[n] = node
	}
	calls := len(f.rec.calls)
	if err := f.store.DeleteNodes(f.asAlice(), app.Deletion{Root: id, IDs: ids, Pages: pages, Assets: assets, Height: height, By: by,
		At: at}); err != nil {
		panic(err)
	}
	f.rec.calls = f.rec.calls[:calls]
	f.store.deletions = nil
}

func (f *fixture) trashReads(at time.Time) app.TrashReads {
	return app.TrashReads{Notebooks: f.notebooks, Nodes: f.store, Trash: f.store, Names: f.names,
		Attachments: fakeAttachments{recorder: f.rec, infos: f.infos}, Auth: f.auth, Clock: fixedClock{at}, Retention: retention}
}

func (f *fixture) listTrash(at time.Time, limit int, cursor string) (app.TrashList, error) {
	var c *string
	if cursor != "" {
		c = &cursor
	}
	return app.NewListTrash(f.trashReads(at)).Execute(f.asAlice(), f.eng, &limit, c)
}

func (f *fixture) trashItem(at time.Time, id uuid.UUID) (app.TrashItem, error) {
	return app.NewGetTrashItem(f.trashReads(at)).Execute(f.asAlice(), id)
}

func itemNames(items []app.TrashItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Root.Node.Name
	}
	return out
}

// trashFixture is a notebook's trash: P/Q/Doc, deleted an hour ago with
// its child and attachment by bob, Q since; photo.png at the root, two
// hours ago; Deep, as deep as its child Leaf fits under its parent, an
// attachment under Leaf, the top of whose line, Level a, may move deeper;
// and an item past the time it could be restored in.
type trashFixture struct {
	*fixture
	bob                                       uuid.UUID
	p, q, doc, child, photo, deep, old, level domain.Node
}

func newTrashFixture() trashFixture {
	f := trashFixture{fixture: newFixture(), bob: uuid.NewV7()}
	f.names[f.alice], f.names[f.bob] = "Alice", "Bob"
	f.p = f.page("P", nil, 0)
	f.q = f.page("Q", &f.p.ID, 0)
	f.doc = f.page("Doc", &f.q.ID, 0)
	f.child = f.page("Child", &f.doc.ID, 0)
	f.asset("scan.pdf", &f.doc.ID, 1)
	c := f.store.contents[f.doc.ID]
	c.Revision = 6
	f.store.contents[f.doc.ID] = c
	f.photo = f.asset("photo.png", nil, 1)
	f.infos = map[uuid.UUID]app.AttachmentInfo{f.photo.ID: {MIME: "image/png", Bytes: 70}}
	var parent *uuid.UUID
	for i := range domain.MaxDepth - 2 {
		n := f.page("Level "+string(rune('a'+i)), parent, 2)
		if i == 0 {
			f.level = n
		}
		parent = &n.ID
	}
	f.deep = f.page("Deep", parent, 0)
	leaf := f.page("Leaf", &f.deep.ID, 0)
	f.asset("leaf.png", &leaf.ID, 1) // below the deepest page: no level
	f.old = f.page("Old", nil, 3)
	f.trashed(f.doc.ID, now().Add(-time.Hour), f.bob)
	f.trashed(f.q.ID, now().Add(-time.Minute), f.alice)
	f.trashed(f.photo.ID, now().Add(-2*time.Hour), f.alice)
	f.trashed(f.deep.ID, now().Add(-3*time.Hour), f.alice)
	f.trashed(f.old.ID, now().Add(-retention+domain.TrashMargin), f.alice)
	return f
}

// The trash lists a notebook's items restorable at now, newer first, a
// page at a time: each with where it was, its counts, its deleter's name,
// until when it can be restored, where it goes back and an attachment's
// type and size; the lines, the names and the infos each read once.
func TestListTrashListsTheItems(t *testing.T) {
	f := newTrashFixture()
	f.grant(domain.ActionHistory)
	reads := 0
	r := f.trashReads(now())
	r.Names = countingNames{f.names, &reads}
	list, err := app.NewListTrash(r).Execute(f.asAlice(), f.eng, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Q", "Doc", "photo.png", "Deep"}; !slices.Equal(itemNames(list.Items), want) || list.NextCursor != "" {
		t.Fatalf("ListTrash() = %v, cursor %q; want %v, the last page", itemNames(list.Items), list.NextCursor, want)
	}
	q, doc, photo, deep := list.Items[0], list.Items[1], list.Items[2], list.Items[3]
	if doc.Root.Pages != 2 || doc.Root.Assets != 1 || doc.Root.Height != 2 || doc.Root.Revision != 6 || doc.DeletedByName != "Bob" ||
		!doc.Root.DeletedAt.Equal(now().Add(-time.Hour)) || !doc.RestorableUntil.Equal(now().Add(-time.Hour+retention-time.Hour)) {
		t.Errorf("Doc's item = %+v; want 2 pages, an attachment, 2 levels, revision 6, by Bob an hour ago", doc)
	}
	if !slices.Equal(doc.Ancestors, []domain.Ancestor{{ID: f.p.ID, Name: "P"}, {ID: f.q.ID, Name: "Q"}}) ||
		doc.Placement.Reason != domain.PlacedParentInTrash || doc.Placement.ParentID != nil {
		t.Errorf("Doc's place = %+v, back to %+v; want P / Q, back at the root as Q is in the trash", doc.Ancestors, doc.Placement)
	}
	if q.Placement.Reason != domain.PlacedOriginal || q.Placement.ParentID == nil || *q.Placement.ParentID != f.p.ID ||
		q.DeletedByName != f.names[f.alice] || q.Asset != nil {
		t.Errorf("Q's item = %+v; want back under P", q)
	}
	if photo.Asset == nil || *photo.Asset != (app.AttachmentInfo{MIME: "image/png", Bytes: 70}) || photo.Placement.Reason != domain.PlacedOriginal ||
		photo.Placement.ParentID != nil || len(photo.Ancestors) != 0 || photo.Root.Revision != 0 {
		t.Errorf("the photo's item = %+v; want its type and size, back at the root", photo)
	}
	if deep.Placement.Reason != domain.PlacedOriginal || deep.Root.Height != 2 || len(deep.Ancestors) != domain.MaxDepth-2 {
		t.Errorf("Deep's item = %+v; want back under its parent, 2 levels as deep as they go", deep)
	}
	for _, call := range []string{"TrashLines 4", "AttachmentInfos 1"} {
		if n := slices.Index(f.rec.calls, call); n < 0 || slices.Contains(f.rec.calls[n+1:], call) {
			t.Errorf("calls = %v, want %s once", f.rec.calls, call)
		}
	}
	if reads != 1 {
		t.Errorf("display names read %d times, want once", reads)
	}
}

// A page's cursor goes on below its last item; an item deleted the
// retention less the margin before now is not listed, a microsecond later
// it is; a page without an attachment reads no infos.
func TestListTrashPagesAndBounds(t *testing.T) {
	f := newTrashFixture()
	f.grant(domain.ActionHistory)
	first, err := f.listTrash(now(), 2, "")
	if err != nil || !slices.Equal(itemNames(first.Items), []string{"Q", "Doc"}) || first.NextCursor == "" {
		t.Fatalf("ListTrash(2) = %v, %q, %v; want Q, Doc and a cursor", itemNames(first.Items), first.NextCursor, err)
	}
	if slices.ContainsFunc(f.rec.calls, func(c string) bool { return strings.HasPrefix(c, "AttachmentInfos") }) {
		t.Errorf("calls = %v, want no infos read for pages alone", f.rec.calls)
	}
	next, err := f.listTrash(now(), 2, first.NextCursor)
	if err != nil || !slices.Equal(itemNames(next.Items), []string{"photo.png", "Deep"}) {
		t.Errorf("ListTrash(2, after Doc) = %v, %v; want photo.png, Deep", itemNames(next.Items), err)
	}
	oldAt := now().Add(-retention + domain.TrashMargin)
	if list, err := f.listTrash(now().Add(-time.Microsecond), 10, ""); err != nil || !slices.Contains(itemNames(list.Items), "Old") {
		t.Errorf("ListTrash(a microsecond before Old's time) = %v, %v; want Old", itemNames(list.Items), err)
	}
	if list, err := f.listTrash(now(), 10, ""); err != nil || slices.Contains(itemNames(list.Items), "Old") {
		t.Errorf("ListTrash(at Old's time) = %v, %v; want it gone (deleted at %s)", itemNames(list.Items), err, oldAt)
	}
}

// The list's codes: a cursor that does not decode before anything; then
// the notebook, not found when unknown, deleted, or without a role; then
// the limit.
func TestListTrashAnswersItsCodesInOrder(t *testing.T) {
	for _, tt := range []struct {
		name   string
		setup  func(f *fixture) (uuid.UUID, int, string)
		want   string
		before bool
	}{
		{"a cursor that does not decode", func(f *fixture) (uuid.UUID, int, string) { return uuid.NewV7(), 0, "x" }, "bad_request", true},
		{"an unknown notebook", func(f *fixture) (uuid.UUID, int, string) {
			f.grant(domain.ActionHistory)
			return uuid.NewV7(), 0, ""
		}, "notebook.not_found", false},
		{"a notebook without a role", func(f *fixture) (uuid.UUID, int, string) { return f.eng, 0, "" }, "notebook.not_found", false},
		{"a limit of 0", func(f *fixture) (uuid.UUID, int, string) {
			f.grant(domain.ActionHistory)
			return f.eng, 0, ""
		}, "validation_failed limit", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			id, limit, cursor := tt.setup(f)
			var c *string
			if cursor != "" {
				c = &cursor
			}
			_, err := app.NewListTrash(f.trashReads(now())).Execute(f.asAlice(), id, &limit, c)
			if got := codeOf(err); got != tt.want {
				t.Errorf("listTrash = %q, want %q", got, tt.want)
			}
			if tt.before && len(f.rec.calls) != 0 || f.called("ListTrash") {
				t.Errorf("calls = %v, want none read", f.rec.calls)
			}
		})
	}
}

// An item goes back at the root when its pages would go too deep under
// its parent: its line read as it is now.
func TestATrashItemTooDeepGoesBackAtTheRoot(t *testing.T) {
	f := newTrashFixture()
	f.grant(domain.ActionHistory)
	top := f.page("Top", nil, 9)
	level := f.store.nodes[f.level.ID]
	level.ParentID = &top.ID
	f.store.nodes[f.level.ID] = level
	deep, err := f.trashItem(now(), f.deep.ID)
	if err != nil || deep.Placement.Reason != domain.PlacedTooDeep || deep.Placement.ParentID != nil ||
		len(deep.Ancestors) != domain.MaxDepth-1 || deep.Ancestors[0].ID != top.ID {
		t.Errorf("GetTrashItem(Deep) = %+v, %v; want back at the root, too deep under Top / Level a / …", deep, err)
	}
}

// An item reads as the trash lists it, a page's or an attachment's, with
// its notebook.
func TestGetTrashItem(t *testing.T) {
	f := newTrashFixture()
	f.grant(domain.ActionHistory)
	doc, err := f.trashItem(now(), f.doc.ID)
	if err != nil || doc.Root.Node.ID != f.doc.ID || doc.Root.Node.NotebookID != f.eng || doc.Root.Revision != 6 || doc.Root.Pages != 2 ||
		doc.DeletedByName != "Bob" || doc.Placement.Reason != domain.PlacedParentInTrash || len(doc.Ancestors) != 2 {
		t.Errorf("GetTrashItem(Doc) = %+v, %v; want Doc's item as listed", doc, err)
	}
	photo, err := f.trashItem(now(), f.photo.ID)
	if err != nil || photo.Asset == nil || photo.Asset.MIME != "image/png" || photo.Root.Node.Kind != domain.KindAsset {
		t.Errorf("GetTrashItem(photo) = %+v, %v; want its type and size", photo, err)
	}
}

// The item's codes come in their order: none, a deleted notebook and no
// role are page.not_found; a node alive page.not_in_trash; a member of
// another root's item page.not_trash_root with that root; then
// page.restore_expired, at the time and after, not a microsecond before.
func TestGetTrashItemAnswersItsCodesInOrder(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(f trashFixture) (uuid.UUID, time.Time)
		want  string
	}{
		{"none", func(f trashFixture) (uuid.UUID, time.Time) { return uuid.NewV7(), now() }, "page.not_found"},
		{"no role", func(f trashFixture) (uuid.UUID, time.Time) {
			delete(f.auth.grants, domain.ActionHistory)
			return f.doc.ID, now()
		}, "page.not_found"},
		{"a deleted notebook", func(f trashFixture) (uuid.UUID, time.Time) {
			delete(f.notebooks.workspaces, f.eng)
			return f.doc.ID, now()
		}, "page.not_found"},
		{"alive", func(f trashFixture) (uuid.UUID, time.Time) { return f.p.ID, now() }, "page.not_in_trash"},
		{"a member", func(f trashFixture) (uuid.UUID, time.Time) { return f.child.ID, now() }, "page.not_trash_root"},
		{"a member of an expired item", func(f trashFixture) (uuid.UUID, time.Time) {
			return f.child.ID, now().Add(retention)
		}, "page.not_trash_root"},
		{"at its time", func(f trashFixture) (uuid.UUID, time.Time) { return f.old.ID, now() }, "page.restore_expired"},
		{"after it", func(f trashFixture) (uuid.UUID, time.Time) { return f.old.ID, now().Add(time.Hour) }, "page.restore_expired"},
		{"a microsecond before", func(f trashFixture) (uuid.UUID, time.Time) { return f.old.ID, now().Add(-time.Microsecond) }, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newTrashFixture()
			f.grant(domain.ActionHistory)
			id, at := tt.setup(f)
			_, err := f.trashItem(at, id)
			if got := codeOf(err); got != tt.want {
				t.Errorf("getTrashItem = %q, want %q", got, tt.want)
			}
			var e *shared.Error
			if tt.want == "page.not_trash_root" && (!errors.As(err, &e) || e.RootID == nil || *e.RootID != f.doc.ID) {
				t.Errorf("page.not_trash_root = %+v, want Doc as its root", err)
			}
		})
	}
}
