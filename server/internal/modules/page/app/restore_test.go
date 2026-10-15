package app_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// restore runs restoreNode on id as alice from the web, of maxNodes (0:
// domain's), the trash read at now.
func (f trashFixture) restore(id uuid.UUID, maxNodes int) (app.NodeRestored, error) {
	return app.NewRestoreNode(f.trashReads(now()), f.writer(), f.parser(), app.RestoreBounds{Nodes: maxNodes}, f.logger()).
		Execute(f.asAlice(), id, domain.ClientWeb)
}

// trashedContent gives the trashed page id the content text, at its
// revision.
func (f trashFixture) trashedContent(id uuid.UUID, text string) {
	c := f.store.trashed[id]
	sum := sha256.Sum256([]byte(text))
	c.Content, c.Hash, c.ByteSize = text, sum[:], len(text)
	f.store.trashed[id] = c
}

// A restore finds its item unlocked, decides, reads its group and parses
// its pages; then in a unit of its notebook, a trash_restore, reads it
// again under the lock and brings it back where it was, its pages with
// their revisions and facts, each node restored from its time with no
// before, an item of the changeset; the participants follow, the
// observers once.
func TestRestoreNodeBringsAnItemBack(t *testing.T) {
	f := newTrashFixture()
	f.grant(domain.ActionHistory, domain.ActionRestore)
	g, p, o := &guard{recorder: f.rec}, &participant{recorder: f.rec}, &observer{recorder: f.rec}
	f.guards, f.partakers, f.observers = []app.WriteGuard{g}, []app.Participant{p}, []app.PageObserver{o}
	f.store.nodes[f.q.ID] = f.store.trash[f.q.ID].Node // Q alive again: Doc goes back under it
	delete(f.store.trash, f.q.ID)
	f.trashedContent(f.doc.ID, "# Doc [[Child]]")
	f.trashedContent(f.child.ID, "child")
	deleted := now().Add(-time.Hour)
	got, err := f.restore(f.doc.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"FindAnyNode", "WorkspaceOf", "Authorize node.restore", "FindTrashRoot", "TrashGroup", "TrashHistory", "TrashContents",
		"Take 15", "Facts", "KeepFacts 15", "Links", "Take 5", "Facts", "KeepFacts 5", "Links",
		"WorkspaceOf", "ShareWorkspace in tx", "LockNotebook in tx", "Authorize node.restore in tx", "FindAnyNode in tx",
		"TrashGroup in tx", "TrashLines 1 in tx", "Children in tx", "GuardWrite restore in tx", "CreateChangeset in tx",
		"RestoreNodes in tx", "RecordItems 3 in tx", "Participate restore in tx", "QueueSearch in tx", "PagesChanged in tx", "Release 15", "Release 5"}
	if !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v\nwant %v", f.rec.calls, want)
	}
	if got.Node.ID != f.doc.ID || got.Node.ParentID == nil || *got.Node.ParentID != f.q.ID || got.Node.Name != "Doc" ||
		got.Node.SortOrder != f.doc.SortOrder || got.Renamed || got.Placement.Reason != domain.PlacedOriginal || got.Node.UpdatedBy != f.alice ||
		got.ChangesetID != f.store.changesets[0].ID {
		t.Errorf("restored = %+v; want Doc back under Q at its order, not renamed, in the changeset", got)
	}
	if cs := f.store.changesets; len(cs) != 1 || cs[0].Kind != domain.ChangesetTrashRestore || cs[0].Client != domain.ClientWeb {
		t.Errorf("changesets = %+v, want one trash_restore of the web", cs)
	}
	if len(g.steps) != 1 || g.steps[0].Options != (app.Options{}) || g.steps[0].Kind != domain.ChangesetTrashRestore {
		t.Errorf("the guard saw %+v, want the restore without options, of a trash_restore", g.steps)
	}
	changes := o.events[0].Changes
	if len(o.events) != 1 || len(changes) != 3 {
		t.Fatalf("the observers saw %+v, want the three nodes once", o.events)
	}
	for _, c := range changes {
		page := c.NodeID != changes[2].NodeID
		if c.Before != nil || c.After == nil || c.Restored == nil || !c.Restored.DeletedAt.Equal(deleted) || c.Restored.Asset == page ||
			page && (c.Revision == 0 || c.Facts == nil) || !page && (c.Revision != 0 || c.Facts != nil) {
			t.Errorf("change %+v, %+v; want restored from %s with no before, a page with its revision and facts", c, c.Restored, deleted)
		}
	}
	if changes[0].Revision != 6 || changes[0].Facts != "# Doc [[Child]]" || *changes[0].After.ParentID != f.q.ID {
		t.Errorf("Doc's change = %+v, want revision 6 with its facts, under Q", changes[0])
	}
	for _, id := range []uuid.UUID{f.doc.ID, f.child.ID} {
		if it := f.store.items[id]; it.Change.Restored == nil || it.Change.Before != nil || it.Change.Revision == 0 {
			t.Errorf("item of %s = %+v, want a restore with its revision", id, it)
		}
		if _, ok := f.store.nodes[id]; !ok || f.store.contents[id].Revision == 0 {
			t.Errorf("node %s is not back with its content", id)
		}
	}
	r := f.store.restorations
	if len(r) != 1 || r[0].Root != f.doc.ID || !r[0].DeletedAt.Equal(deleted) || len(r[0].IDs) != 3 || r[0].By != f.alice ||
		!slices.Equal(r[0].Names, []string{"Doc", "Child", "scan.pdf"}) || r[0].NameKeys[0] != shared.TitleKey("Doc") {
		t.Errorf("restorations = %+v, want Doc's group of its time, named as they were", r)
	}
	logs := f.logs.String()
	for _, part := range []string{"node restored", "nodes=3", "pages=2", "assets=1", "placed=original", "renamed=false", f.doc.ID.String()} {
		if !strings.Contains(logs, part) || strings.Contains(logs, "Child") {
			t.Errorf("log %q, want %s and no name", logs, part)
		}
	}
	if f.budget.held != 0 {
		t.Errorf("the budget holds %d after the restore", f.budget.held)
	}
}

// Where an item goes back: under its parent alive at its order, as it was
// at the root; last at the root, its parent in the trash or its pages too
// deep under it; numbered when an alive sibling holds its name there, an
// attachment's before its extension. Its change, and so its item, holds
// the state it went back with.
func TestRestoreNodePlacesAndNumbers(t *testing.T) {
	for _, tt := range []struct {
		name    string
		setup   func(f trashFixture) uuid.UUID
		parent  func(f trashFixture) *uuid.UUID
		reason  domain.PlacementReason
		named   string
		renamed bool
	}{
		{"its parent alive", func(f trashFixture) uuid.UUID { return f.q.ID },
			func(f trashFixture) *uuid.UUID { return &f.p.ID }, domain.PlacedOriginal, "Q", false},
		{"at the root, as it was", func(f trashFixture) uuid.UUID { return f.photo.ID },
			func(trashFixture) *uuid.UUID { return nil }, domain.PlacedOriginal, "photo.png", false},
		{"as deep as it fits, an attachment below its deepest page", func(f trashFixture) uuid.UUID { return f.deep.ID },
			func(f trashFixture) *uuid.UUID { return f.deep.ParentID }, domain.PlacedOriginal, "Deep", false},
		{"its parent in the trash", func(f trashFixture) uuid.UUID { return f.doc.ID },
			func(trashFixture) *uuid.UUID { return nil }, domain.PlacedParentInTrash, "Doc", false},
		{"too deep", func(f trashFixture) uuid.UUID {
			top := f.page("Top", nil, 9)
			level := f.store.nodes[f.level.ID]
			level.ParentID = &top.ID
			f.store.nodes[f.level.ID] = level
			return f.deep.ID
		}, func(trashFixture) *uuid.UUID { return nil }, domain.PlacedTooDeep, "Deep", false},
		{"a sibling of its name", func(f trashFixture) uuid.UUID {
			f.page("q", &f.p.ID, 5)
			return f.q.ID
		}, func(f trashFixture) *uuid.UUID { return &f.p.ID }, domain.PlacedOriginal, "Q 2", true},
		{"an attachment's sibling of its name", func(f trashFixture) uuid.UUID {
			f.asset("Photo.PNG", nil, 7)
			return f.photo.ID
		}, func(trashFixture) *uuid.UUID { return nil }, domain.PlacedOriginal, "photo 2.png", true},
		{"a sibling of its name at the root", func(f trashFixture) uuid.UUID {
			f.page("Doc", nil, 8)
			return f.doc.ID
		}, func(trashFixture) *uuid.UUID { return nil }, domain.PlacedParentInTrash, "Doc 2", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newTrashFixture()
			f.grant(domain.ActionHistory, domain.ActionRestore)
			id := tt.setup(f)
			before := f.store.trash[id].Node
			var last float64
			for _, n := range f.store.nodes {
				if n.ParentID == nil {
					last = max(last, n.SortOrder)
				}
			}
			got, err := f.restore(id, 0)
			if err != nil {
				t.Fatal(err)
			}
			n := f.store.nodes[id]
			parent := tt.parent(f)
			if !domain.SameParent(n.ParentID, parent) || n.Name != tt.named || n.NameKey != shared.TitleKey(tt.named) ||
				got.Placement.Reason != tt.reason || !domain.SameParent(got.Placement.ParentID, parent) || got.Renamed != tt.renamed ||
				got.Node.Name != tt.named {
				t.Errorf("restored %+v as %+v; want under %v as %q (renamed %v), %s", got, n, parent, tt.named, tt.renamed, tt.reason)
			}
			moved := tt.reason != domain.PlacedOriginal
			if moved && n.SortOrder != last+1 || !moved && n.SortOrder != before.SortOrder {
				t.Errorf("order %v, want %v: its own back where it was, after the root's last at the root", n.SortOrder, last+1)
			}
			if after := f.store.items[id].Change.After; after == nil || !domain.SameParent(after.ParentID, parent) ||
				after.Name != tt.named || after.SortOrder != n.SortOrder {
				t.Errorf("its item's after = %+v; want under %v as %q at %v", after, parent, tt.named, n.SortOrder)
			}
		})
	}
}

// The group's names are keyed again by today's rules, and one whose key
// another of the group under the same parent holds, after it by order and
// id, is numbered: refused, the item could never come back. Names alike
// under two parents of the group are not. The others renamed do not count
// the root, numbered or not.
func TestRestoreNodeKeysTheGroupAgain(t *testing.T) {
	for _, rootRenamed := range []bool{false, true} {
		t.Run(fmt.Sprintf("the root renamed %v", rootRenamed), func(t *testing.T) {
			f := newTrashFixture()
			f.grant(domain.ActionHistory, domain.ActionRestore)
			box := f.page("Box", nil, 4)
			notes := f.page("Notes", &box.ID, 0)
			other := f.page("NOTES", &box.ID, 1)
			a, b := f.page("A", &box.ID, 2), f.page("B", &box.ID, 3)
			aNotes, bNotes := f.page("Notes", &a.ID, 0), f.page("Notes", &b.ID, 0)
			f.trashed(box.ID, now().Add(-time.Minute), f.alice)
			for id, key := range map[uuid.UUID]string{notes.ID: "stale", other.ID: "keyed apart"} {
				n := f.store.trash[id]
				n.NameKey = key
				f.store.trash[id] = n
			}
			name := "Box"
			if rootRenamed {
				f.page("box", nil, 5)
				name = "Box 2"
			}
			got, err := f.restore(box.ID, 0)
			if err != nil {
				t.Fatal(err)
			}
			if n := f.store.nodes[notes.ID]; n.Name != "Notes" || n.NameKey != shared.TitleKey("Notes") {
				t.Errorf("Notes = %+v, want its name keyed again", n)
			}
			if n, after := f.store.nodes[other.ID], f.store.items[other.ID].Change.After; n.Name != "NOTES 2" ||
				n.NameKey != shared.TitleKey("NOTES 2") || after == nil || after.Name != "NOTES 2" {
				t.Errorf("NOTES = %+v, its item's after %+v; want it numbered after Notes", n, after)
			}
			for _, id := range []uuid.UUID{aNotes.ID, bNotes.ID} {
				if n := f.store.nodes[id]; n.Name != "Notes" {
					t.Errorf("Notes under %v = %+v, want it as it was: its parent's alone", *n.ParentID, n)
				}
			}
			if got.Renamed != rootRenamed || got.Node.Name != name || !strings.Contains(f.logs.String(), "others_renamed=1") {
				t.Errorf("restored = %+v, log %q; want the root as %s, one other renamed", got, f.logs, name)
			}
		})
	}
}

// The codes come in their order, and a refusal writes nothing.
func TestRestoreNodeAnswersItsCodesInOrder(t *testing.T) {
	links := strings.Repeat("[[x]]", domain.MaxUnitLinks/2+1)
	for _, tt := range []struct {
		name     string
		setup    func(f trashFixture) uuid.UUID
		maxNodes int
		want     string
	}{
		{"none", func(f trashFixture) uuid.UUID { return uuid.NewV7() }, 0, "page.not_found"},
		{"a deleted notebook", func(f trashFixture) uuid.UUID {
			delete(f.notebooks.workspaces, f.eng)
			return f.doc.ID
		}, 0, "page.not_found"},
		{"no role", func(f trashFixture) uuid.UUID {
			delete(f.auth.grants, domain.ActionRestore)
			return f.doc.ID
		}, 0, "page.not_found"},
		{"a reader", func(f trashFixture) uuid.UUID {
			delete(f.auth.grants, domain.ActionRestore)
			f.auth.forbidden[domain.ActionRestore] = true
			return f.old.ID // expired: forbidden first
		}, 0, "forbidden"},
		{"alive", func(f trashFixture) uuid.UUID { return f.p.ID }, 0, "page.not_in_trash"},
		{"a member", func(f trashFixture) uuid.UUID { return f.child.ID }, 0, "page.not_trash_root"},
		{"expired", func(f trashFixture) uuid.UUID { return f.old.ID }, 1, "page.restore_expired"},
		{"past the nodes", func(f trashFixture) uuid.UUID { return f.doc.ID }, 2, "page.restore_too_large"},
		{"at the nodes", func(f trashFixture) uuid.UUID { return f.doc.ID }, 3, ""},
		{"a content the purge took, past the nodes", func(f trashFixture) uuid.UUID {
			delete(f.store.trashed, f.child.ID)
			return f.doc.ID
		}, 2, "page.restore_too_large"},
		{"a content the purge took", func(f trashFixture) uuid.UUID {
			delete(f.store.trashed, f.child.ID)
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"past the pages", func(f trashFixture) uuid.UUID {
			box := f.page("Box", nil, 4)
			for i := range domain.MaxUnitPages {
				f.page(fmt.Sprintf("Page %d", i), &box.ID, float64(i))
			}
			f.trashed(box.ID, now(), f.alice)
			return box.ID
		}, 1000, "page.restore_too_large"},
		{"at the pages", func(f trashFixture) uuid.UUID {
			box := f.page("Box", nil, 4)
			for i := range domain.MaxUnitPages - 1 {
				f.page(fmt.Sprintf("Page %d", i), &box.ID, float64(i))
			}
			f.trashed(box.ID, now(), f.alice)
			return box.ID
		}, 1000, ""},
		{"at the pages, with attachments", func(f trashFixture) uuid.UUID {
			box := f.page("Box", nil, 4)
			for i := range domain.MaxUnitPages - 1 {
				page := f.page(fmt.Sprintf("Page %d", i), &box.ID, float64(i))
				f.asset(fmt.Sprintf("a%d.png", i), &page.ID, 0)
			}
			f.trashed(box.ID, now(), f.alice)
			return box.ID
		}, 1000, ""},
		{"past the history", func(f trashFixture) uuid.UUID {
			f.store.history = map[uuid.UUID]int{f.doc.ID: 1, f.child.ID: domain.MaxUnitHistory}
			return f.doc.ID
		}, 0, "page.restore_too_large"},
		{"past the history, by its attachment's", func(f trashFixture) uuid.UUID {
			for id, n := range f.store.trash {
				if n.Kind == domain.KindAsset && n.ParentID != nil && *n.ParentID == f.doc.ID {
					f.store.history = map[uuid.UUID]int{id: domain.MaxUnitHistory + 1}
				}
			}
			return f.doc.ID
		}, 0, "page.restore_too_large"},
		{"at the history", func(f trashFixture) uuid.UUID {
			f.store.history = map[uuid.UUID]int{f.doc.ID: 1, f.child.ID: domain.MaxUnitHistory - 1}
			return f.doc.ID
		}, 0, ""},
		{"a content the purge took, past the history", func(f trashFixture) uuid.UUID {
			f.store.history = map[uuid.UUID]int{f.child.ID: domain.MaxUnitHistory + 1}
			delete(f.store.trashed, f.child.ID)
			return f.doc.ID
		}, 0, "page.restore_too_large"},
		{"past the bytes", func(f trashFixture) uuid.UUID {
			f.trashedContent(f.doc.ID, strings.Repeat("a", domain.MaxUnitBytes))
			f.trashedContent(f.child.ID, "b")
			return f.doc.ID
		}, 0, "page.restore_too_large"},
		{"a content the purge took, past the bytes", func(f trashFixture) uuid.UUID {
			f.trashedContent(f.doc.ID, strings.Repeat("a", domain.MaxUnitBytes+1))
			delete(f.store.trashed, f.child.ID)
			return f.doc.ID
		}, 0, "page.restore_too_large"},
		{"a lone page past the bytes and links", func(f trashFixture) uuid.UUID {
			f.trashedContent(f.q.ID, strings.Repeat("a", domain.MaxUnitBytes)+links+links)
			return f.q.ID
		}, 0, ""},
		{"the budget busy, past the links", func(f trashFixture) uuid.UUID {
			f.budget.err = shared.ServerBusy(time.Second)
			f.trashedContent(f.doc.ID, links)
			f.trashedContent(f.child.ID, links)
			return f.doc.ID
		}, 0, "server_busy"},
		{"past the links", func(f trashFixture) uuid.UUID {
			f.trashedContent(f.doc.ID, links)
			f.trashedContent(f.child.ID, links)
			return f.doc.ID
		}, 0, "page.restore_too_large"},
		{"restored meanwhile", func(f trashFixture) uuid.UUID {
			f.notebooks.waited = func() {
				f.store.nodes[f.doc.ID] = f.store.trash[f.doc.ID].Node
				delete(f.store.trash, f.doc.ID)
			}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"deleted again meanwhile", func(f trashFixture) uuid.UUID {
			f.notebooks.waited = func() {
				n := f.store.trash[f.doc.ID]
				at := now()
				n.DeletedAt = &at
				f.store.trash[f.doc.ID] = n
			}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"a member restored meanwhile", func(f trashFixture) uuid.UUID {
			f.notebooks.waited = func() { delete(f.store.trash, f.child.ID) }
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"its last member restored meanwhile", func(f trashFixture) uuid.UUID {
			f.notebooks.waited = func() {
				for id, n := range f.store.trash {
					if n.Kind == domain.KindAsset && n.ParentID != nil && *n.ParentID == f.doc.ID {
						delete(f.store.trash, id)
					}
				}
			}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"no root meanwhile", func(f trashFixture) uuid.UUID {
			f.notebooks.waited = func() {
				n := f.store.trash[f.doc.ID]
				n.TrashRoot = false
				f.store.trash[f.doc.ID] = n
			}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"a content written meanwhile", func(f trashFixture) uuid.UUID {
			f.notebooks.waited = func() {
				c := f.store.trashed[f.child.ID]
				c.Revision++
				f.store.trashed[f.child.ID] = c
			}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"a content purged after the group was read", func(f trashFixture) uuid.UUID {
			f.store.lost = map[uuid.UUID]bool{f.child.ID: true}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"a content purged meanwhile", func(f trashFixture) uuid.UUID {
			f.notebooks.waited = func() { delete(f.store.trashed, f.child.ID) }
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"restored before its item was read", func(f trashFixture) uuid.UUID {
			f.rec.before = map[string]func(){"FindTrashRoot": func() { f.untrash(f.doc.ID) }}
			return f.doc.ID
		}, 0, "page.not_in_trash"},
		{"restored before its group was read", func(f trashFixture) uuid.UUID {
			f.rec.before = map[string]func(){"TrashGroup": func() { f.untrash(f.doc.ID) }}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"purged before the lock", func(f trashFixture) uuid.UUID {
			at := *f.store.trash[f.doc.ID].DeletedAt
			f.rec.before = map[string]func(){"FindAnyNode in tx": func() {
				for id, n := range f.store.trash {
					if n.DeletedAt.Equal(at) {
						delete(f.store.trash, id)
					}
				}
			}}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"the current version purged meanwhile", func(f trashFixture) uuid.UUID {
			f.store.unversioned = map[uuid.UUID]bool{f.child.ID: true}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"a version purged meanwhile", func(f trashFixture) uuid.UUID {
			f.store.rows = &app.RestoredRows{Nodes: 3, Contents: 2, CurrentVersions: 2, Creations: 2, Gaps: 1}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"a node gone meanwhile", func(f trashFixture) uuid.UUID {
			f.store.rows = &app.RestoredRows{Nodes: 2, Contents: 2, CurrentVersions: 2, Creations: 2}
			return f.doc.ID
		}, 0, "page.trash_changed"},
		{"expired while it waited", func(f trashFixture) uuid.UUID {
			f.clock.reads = int((retention - 2*time.Hour) / time.Microsecond) // the unit's time: Doc's restorable until
			return f.doc.ID
		}, 0, "page.restore_expired"},
		{"the guard", func(f trashFixture) uuid.UUID {
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: domain.ErrLocked}}
			return f.doc.ID
		}, 0, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newTrashFixture()
			f.grant(domain.ActionHistory, domain.ActionRestore)
			id := tt.setup(f)
			trash := len(f.store.trash)
			_, err := f.restore(id, tt.maxNodes)
			if got := codeOf(err); got != tt.want {
				t.Errorf("restoreNode = %q, want %q", got, tt.want)
			}
			if tt.want != "" && (len(f.store.changesets) != 0 && !f.tx.rolledBack || f.budget.held != 0) {
				t.Errorf("a refused restore left changesets %+v, rolled back %v, the budget holding %d; want nothing", f.store.changesets,
					f.tx.rolledBack, f.budget.held)
			}
			if tt.want == "" && len(f.store.trash) >= trash {
				t.Errorf("the trash holds %d nodes, want fewer than %d", len(f.store.trash), trash)
			}
		})
	}
}

// The contents are parsed largest first, their links counted as they go:
// a lone page is never past them, more past them are too large, given
// back. The takes wait, together, the budget's wait at most: a take that
// runs out of what is left while the request has time is server_busy,
// one whose request runs out first its request's error.
func TestDecidedAllTakesTheLargestFirstWithinOneWait(t *testing.T) {
	f := newFixture()
	a, b, c := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	tooLarge := errors.New("too large")
	facts, release, err := f.parser().DecidedAll(context.Background(), map[uuid.UUID]string{a: "aa", b: "bbbb", c: ""}, tooLarge)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Take 4", "Facts", "KeepFacts 4", "Links", "Take 2", "Facts", "KeepFacts 2", "Links", "Facts"}; !slices.Equal(f.rec.calls, want) ||
		facts[a] != "aa" || facts[b] != "bbbb" || facts[c] != "" {
		t.Errorf("calls = %v, facts %v; want the largest first, the empty one without the budget", f.rec.calls, facts)
	}
	release()
	if f.budget.held != 0 {
		t.Errorf("the budget holds %d after the release", f.budget.held)
	}

	links := strings.Repeat("[[x]]", domain.MaxUnitLinks/2+1)
	if _, release, err := f.parser().DecidedAll(context.Background(), map[uuid.UUID]string{a: links + links}, tooLarge); err != nil {
		t.Errorf("DecidedAll(a lone page past the links) = %v, want its facts", err)
	} else {
		release()
	}
	if _, _, err := f.parser().DecidedAll(context.Background(), map[uuid.UUID]string{a: links, b: links}, tooLarge); !errors.Is(err, tooLarge) ||
		f.budget.held != 0 {
		t.Errorf("DecidedAll(two pages past the links) = %v, holding %d; want too large, nothing held", err, f.budget.held)
	}

	f.wait = 200 * time.Millisecond
	f.budget.waits = []time.Duration{150 * time.Millisecond, 100 * time.Millisecond}
	if _, _, err := f.parser().DecidedAll(context.Background(), map[uuid.UUID]string{a: "aa", b: "bbbb"}, tooLarge); codeOf(err) != "server_busy" ||
		f.budget.held != 0 {
		t.Errorf("DecidedAll(two takes waiting past the wait together) = %v, holding %d; want server_busy, nothing held", err, f.budget.held)
	}
	f.budget.waits = []time.Duration{150 * time.Millisecond}
	if _, release, err := f.parser().DecidedAll(context.Background(), map[uuid.UUID]string{a: "aa", b: "bbbb"}, tooLarge); err != nil {
		t.Errorf("DecidedAll(two takes within the wait together) = %v, want the facts", err)
	} else {
		release()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	f.budget.waits = []time.Duration{150 * time.Millisecond}
	if _, _, err := f.parser().DecidedAll(ctx, map[uuid.UUID]string{a: "aa"}, tooLarge); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("DecidedAll(a request that runs out first) = %v, want its deadline", err)
	}
}
