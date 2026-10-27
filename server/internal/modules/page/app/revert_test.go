package app_test

import (
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

// revert runs revertChangeset on id as alice from the web, with message,
// of maxItems (0: the domain's), the trash read at now.
func (f *fixture) revert(id uuid.UUID, message *string, maxItems int) (app.ChangesetReverted, error) {
	return f.revertWithin(id, message, app.RevertBounds{Nodes: maxItems})
}

// revertWithin is revert within bounds.
func (f *fixture) revertWithin(id uuid.UUID, message *string, bounds app.RevertBounds) (app.ChangesetReverted, error) {
	return app.NewRevertChangeset(f.trashReads(now()), f.log, f.writer(), f.parser(), bounds, f.logger()).
		Execute(f.asAlice(), id, message, domain.ClientWeb)
}

// content gives the page id the content text at revision, which its
// version holds.
func (f *fixture) content(id uuid.UUID, text string, revision int) {
	sum := sha256.Sum256([]byte(text))
	f.store.contents[id] = app.Content{NodeID: id, Content: text, Revision: revision, Hash: sum[:], ByteSize: len(text), By: f.alice,
		At: now()}
	f.log.versions[domain.VersionRef{NodeID: id, Revision: revision}] = text
}

// deletion makes the changeset of the deletion of the subtree of id at
// at, as a deletion's unit writes it: each node's item from where it
// was, a page's with its content's revision, whose version the history
// holds; the subtree in the trash.
func (f *fixture) deletion(id uuid.UUID, at time.Time) *fakeChangeset {
	var items []domain.ChangesetItem
	var walk func(uuid.UUID)
	walk = func(id uuid.UUID) {
		n := f.store.nodes[id]
		it := domain.ChangesetItem{NodeID: id, Before: new(n.State())}
		if c, ok := f.store.contents[id]; ok {
			it.Revision = c.Revision
			f.log.versions[domain.VersionRef{NodeID: id, Revision: c.Revision}] = c.Content
		}
		items = append(items, it)
		for _, child := range f.store.nodes {
			if child.ParentID != nil && *child.ParentID == id {
				walk(child.ID)
			}
		}
	}
	walk(id)
	f.trashed(id, at, f.alice)
	return f.log.changeset(domain.ChangesetEdit, f.alice, at, items)
}

// renaming makes the changeset of the rename of the node id to name.
func (f *fixture) renaming(id uuid.UUID, name string) *fakeChangeset {
	n := f.store.nodes[id]
	before := n.State()
	n.Name, n.NameKey = name, shared.TitleKey(name)
	f.store.nodes[id] = n
	return f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{{NodeID: id, Before: &before,
		After: new(n.State())}})
}

// writing makes the changeset of a write of text to the page id, on its
// content's revision.
func (f *fixture) writing(id uuid.UUID, text string) *fakeChangeset {
	base := f.store.contents[id]
	f.log.versions[domain.VersionRef{NodeID: id, Revision: base.Revision}] = base.Content
	f.content(id, text, base.Revision+1)
	return f.log.changeset(domain.ChangesetEdit, f.alice, now(), nil, f.log.version(id, base.Revision, base.Revision+1, text))
}

// revertFixture is a notebook whose history holds the deletion of Box, an
// hour ago: Box, its page Doc and its attachment scan.pdf; Other is a
// page alive at the root.
type revertFixture struct {
	*fixture
	box, doc, scan, other domain.Node
	c                     *fakeChangeset
	deletedAt             time.Time
}

func newRevertFixture() revertFixture {
	f := revertFixture{fixture: newFixture(), deletedAt: now().Add(-time.Hour)}
	f.names[f.alice] = "Alice"
	f.grant(domain.ActionRevert)
	f.box = f.page("Box", nil, 0)
	f.doc = f.page("Doc", &f.box.ID, 0)
	f.content(f.doc.ID, "# Doc [[Other]]", 3)
	f.scan = f.asset("scan.pdf", &f.box.ID, 1)
	f.other = f.page("Other", nil, 1)
	f.c = f.deletion(f.box.ID, f.deletedAt)
	return f
}

// twoSteps adds to C the creation of a page Fresh at the root: its revert
// restores Box and deletes Fresh, two steps of its round.
func (f revertFixture) twoSteps() {
	fresh := f.page("Fresh", nil, 7)
	f.content(fresh.ID, "fresh", 1)
	f.c.items = append(f.c.items, domain.ChangesetItem{NodeID: fresh.ID, After: new(fresh.State()), Revision: 1})
	f.c.versions = append(f.c.versions, f.log.version(fresh.ID, 0, 1, "fresh"))
}

// trashedContent gives the trashed page id the content text, at its
// revision, which its version holds.
func (f revertFixture) trashedContent(id uuid.UUID, text string) {
	c := f.store.trashed[id]
	sum := sha256.Sum256([]byte(text))
	c.Content, c.Hash, c.ByteSize = text, sum[:], len(text)
	f.store.trashed[id] = c
	f.log.versions[domain.VersionRef{NodeID: id, Revision: c.Revision}] = text
}

// A revert reads its changeset unlocked: its items and versions, then its
// row; its nodes, the group of the trash item it deleted, the contents,
// the versions' hashes and the tree; plans, reads and parses the contents
// it restores; then in a unit of its notebook, a revert of the changeset
// with the message, reads and plans it again under the lock and brings
// the trash item back where it was, at its time, each node named as it
// was, its pages with their revisions and facts; the participants
// follow, the observers once.
func TestRevertChangesetRestoresADeletion(t *testing.T) {
	f := newRevertFixture()
	g, p, o := &guard{recorder: f.rec}, &participant{recorder: f.rec}, &observer{recorder: f.rec}
	f.guards, f.partakers, f.observers = []app.WriteGuard{g}, []app.Participant{p}, []app.PageObserver{o}
	message := "Undo the clean-up"
	f.rec.calls = nil
	got, err := f.revert(f.c.ID, &message, 0)
	if err != nil {
		t.Fatal(err)
	}
	reads := []string{"ChangesetItems", "ChangesetVersions", "FindChangeset", "NodesNow 3", "TrashGroups 1"}
	state := []string{"ContentsNow 0", "VersionHashes 2", "ListNodes"}
	var want []string
	want = append(want, "FindChangeset", "WorkspaceOf", "Authorize changeset.revert", "CountItems")
	want = append(append(append(want, reads...), "TrashHistory"), state...)
	want = append(want, "StateOf", "TrashContents", "Take 15", "Facts", "KeepFacts 15", "Links", "Facts",
		"WorkspaceOf", "ShareWorkspace in tx", "LockNotebook in tx", "Authorize changeset.revert in tx")
	for _, call := range append(reads, state...) {
		want = append(want, call+" in tx")
	}
	want = append(want, "GuardWrite restore in tx", "CreateChangeset in tx", "RestoreNodes in tx", "RecordItems 3 in tx",
		"Participate restore in tx", "QueueSearch in tx", "PagesChanged in tx", "Release 15")
	if !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v\nwant %v", f.rec.calls, want)
	}
	cs := f.store.changesets
	if len(cs) != 1 || got != (app.ChangesetReverted{ChangesetID: cs[0].ID, Changed: true}) || cs[0].Kind != domain.ChangesetRevert ||
		cs[0].RevertsID != f.c.ID || cs[0].Message == nil || *cs[0].Message != message || cs[0].Client != domain.ClientWeb {
		t.Errorf("reverted %+v, changesets %+v; want one revert of C with its message, of the web", got, cs)
	}
	if len(g.steps) != 1 || g.steps[0].Options != (app.Options{}) || g.steps[0].Kind != domain.ChangesetRevert ||
		g.steps[0].RevertsID != f.c.ID {
		t.Errorf("the guard saw %+v, want the restore without options, of a revert of C", g.steps)
	}
	r := f.store.restorations
	if len(r) != 1 || r[0].Root != f.box.ID || r[0].ParentID != nil || r[0].SortOrder != 0 || !r[0].DeletedAt.Equal(f.deletedAt) ||
		!slices.Equal(r[0].Names, []string{"Box", "Doc", "scan.pdf"}) || r[0].NameKeys[2] != shared.TitleKey("scan.pdf") {
		t.Errorf("restorations = %+v, want Box's group of its time, named as it was", r)
	}
	changes := o.events[0].Changes
	if len(o.events) != 1 || len(changes) != 3 || changes[1].Revision != 3 || changes[1].Facts != "# Doc [[Other]]" ||
		changes[0].Facts != "" || changes[2].Revision != 0 || changes[2].Restored == nil || !changes[2].Restored.Asset {
		t.Fatalf("the observers saw %+v, want the three nodes back, Doc at 3 with its facts", o.events)
	}
	for _, c := range changes {
		if c.Before != nil || c.Restored == nil || !c.Restored.DeletedAt.Equal(f.deletedAt) {
			t.Errorf("change %+v, want restored from %s with no before", c, f.deletedAt)
		}
	}
	logs := f.logs.String()
	for _, part := range []string{"changeset reverted", "restored=1", "placed=0", "deleted=0", "written=0", "aside=0", f.c.ID.String(),
		cs[0].ID.String(), f.eng.String(), "client=web", "workspace_id=" + f.acme.String(), "user_id=" + f.alice.String()} {
		if !strings.Contains(logs, part) || strings.Contains(logs, "Doc") {
			t.Errorf("log %q, want %s and no name", logs, part)
		}
	}
	if f.budget.held != 0 {
		t.Errorf("the budget holds %d after the revert", f.budget.held)
	}
}

// A revert puts each change back: a rename renamed back, a move moved
// back with its subtree, a creation deleted, a write's base written back
// on the revision the unit read; a write of the bytes it found back
// changes nothing, so no changeset is made.
func TestRevertChangesetPutsEachChangeBack(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(f *fixture) (*fakeChangeset, func() string)
		want  string
	}{
		{"a rename", func(f *fixture) (*fakeChangeset, func() string) {
			p := f.page("Old", nil, 0)
			c := f.renaming(p.ID, "New")
			return c, func() string {
				n := f.store.nodes[p.ID]
				return fmt.Sprintf("%s %s %d", n.Name, n.NameKey, len(f.store.items))
			}
		}, "Old old 1"},
		{"a move", func(f *fixture) (*fakeChangeset, func() string) {
			box := f.page("Box", nil, 0)
			p := f.page("P", nil, 5)
			q := f.page("Q", &p.ID, 0)
			before := p.State()
			p.ParentID, p.SortOrder = &box.ID, 0
			f.store.nodes[p.ID] = p
			c := f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{{NodeID: p.ID, Before: &before,
				After: new(p.State())}})
			o := &observer{recorder: f.rec}
			f.observers = []app.PageObserver{o}
			return c, func() string {
				n := f.store.nodes[p.ID]
				return fmt.Sprintf("%v %v under %d, %d changes, Q %v", n.ParentID == nil, n.SortOrder, len(o.events),
					len(o.events[0].Changes), o.events[0].Changes[1].NodeID == q.ID)
			}
		}, "true 5 under 1, 2 changes, Q true"},
		{"a creation", func(f *fixture) (*fakeChangeset, func() string) {
			p := f.page("New", nil, 0)
			f.content(p.ID, "new", 1)
			c := f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{{NodeID: p.ID, After: new(p.State()),
				Revision: 1}}, f.log.version(p.ID, 0, 1, "new"))
			return c, func() string {
				_, alive := f.store.nodes[p.ID]
				return fmt.Sprintf("alive %v, %d deletions", alive, len(f.store.deletions))
			}
		}, "alive false, 1 deletions"},
		{"a write", func(f *fixture) (*fakeChangeset, func() string) {
			p := f.page("P", nil, 0)
			f.content(p.ID, "one", 4)
			c := f.writing(p.ID, "two")
			return c, func() string {
				got := f.store.contents[p.ID]
				r := f.store.revisions[0]
				return fmt.Sprintf("%q at %d, its version on %d", got.Content, got.Revision, *r.Base)
			}
		}, `"one" at 6, its version on 5`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRevert)
			c, state := tt.setup(f)
			got, err := f.revert(c.ID, nil, 0)
			if err != nil || !got.Changed || len(f.store.changesets) != 1 || got.ChangesetID != f.store.changesets[0].ID {
				t.Fatalf("revert = %+v, %v; changesets %+v", got, err, f.store.changesets)
			}
			if s := state(); s != tt.want {
				t.Errorf("after the revert: %s, want %s", s, tt.want)
			}
		})
	}
	f := newFixture()
	f.grant(domain.ActionRevert)
	p := f.page("P", nil, 0)
	f.content(p.ID, "same", 2)
	c := f.writing(p.ID, "other")
	f.content(p.ID, "same", 3)
	c.versions[0].Hash = hashOf("same")
	f.rec.calls = nil
	if got, err := f.revert(c.ID, nil, 0); err != nil || got != (app.ChangesetReverted{}) || len(f.store.changesets) != 0 ||
		f.called("WriteContent in tx") || f.called("Take 4") {
		t.Errorf("a revert of a write of the bytes found back = %+v, %v, changesets %+v, calls %v; want nothing changed",
			got, err, f.store.changesets, f.rec.calls)
	}
}

// The codes come in M8/P3 design 3.6's order: the message; the changeset
// and the decision; its mark; its items' count; its mark again, read after
// its items, or the changeset gone meanwhile, purged, with its notebook,
// or the caller's role ended, which answers as the first read; then the
// trash items it deleted, read only past their roots' check, the bounds of
// what it parses and restores; the conflicts, the versions the purge took
// and the round's steps, then the contents the purge took, before the
// parse's server_busy; the links past it; then under the lock its mark,
// the changeset purged, its items' count, its conflicts, a version the
// purge took, its steps, a page parsed at another hash, the restore's rows
// and the guards.
func TestRevertChangesetAnswersItsCodesInOrder(t *testing.T) {
	links := strings.Repeat("[[x]]", domain.MaxUnitLinks/2+1)
	empty, bad := "", "a\x00b"
	for _, tt := range []struct {
		name     string
		setup    func(f revertFixture) uuid.UUID
		message  *string
		maxItems int
		maxSteps int
		want     string
	}{
		{"an empty message, of no changeset", func(f revertFixture) uuid.UUID { return uuid.NewV7() }, &empty, 0, 0,
			shared.CodeValidationFailed + " message"},
		{"a message with a NUL", func(f revertFixture) uuid.UUID { return f.c.ID }, &bad, 0, 0, shared.CodeValidationFailed + " message"},
		{"none", func(f revertFixture) uuid.UUID { return uuid.NewV7() }, nil, 0, 0, "page.changeset_not_found"},
		{"of a deleted notebook", func(f revertFixture) uuid.UUID {
			f.c.gone = true
			return f.c.ID
		}, nil, 0, 0, "page.changeset_not_found"},
		{"of a notebook gone from its workspace", func(f revertFixture) uuid.UUID {
			delete(f.notebooks.workspaces, f.eng)
			return f.c.ID
		}, nil, 0, 0, "page.changeset_not_found"},
		{"no role", func(f revertFixture) uuid.UUID {
			delete(f.auth.grants, domain.ActionRevert)
			return f.c.ID
		}, nil, 0, 0, "page.changeset_not_found"},
		{"a reader, of a trimmed changeset", func(f revertFixture) uuid.UUID {
			delete(f.auth.grants, domain.ActionRevert)
			f.auth.forbidden[domain.ActionRevert] = true
			f.c.Trimmed = true
			return f.c.ID
		}, nil, 0, 0, "forbidden"},
		{"trimmed, past the items", func(f revertFixture) uuid.UUID {
			f.c.Trimmed = true
			return f.c.ID
		}, nil, 1, 0, "page.changeset_trimmed"},
		{"past the items", func(f revertFixture) uuid.UUID { return f.c.ID }, nil, 2, 0, "page.revert_too_large"},
		{"at the items", func(f revertFixture) uuid.UUID { return f.c.ID }, nil, 3, 0, ""},
		{"trimmed between its items and its mark", func(f revertFixture) uuid.UUID {
			f.rec.before = map[string]func(){"ChangesetVersions": func() {
				f.c.items, f.c.Trimmed = f.c.items[:1], true
			}}
			return f.c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"purged between its items and its row", func(f revertFixture) uuid.UUID {
			f.rec.before = map[string]func(){"ChangesetVersions": func() { f.c.gone = true }}
			return f.c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"its notebook deleted between its items and its row", func(f revertFixture) uuid.UUID {
			f.rec.before = map[string]func(){"ChangesetVersions": func() {
				f.c.gone = true
				delete(f.notebooks.workspaces, f.eng)
			}}
			return f.c.ID
		}, nil, 0, 0, "page.changeset_not_found"},
		{"its caller's role ended between its items and its row", func(f revertFixture) uuid.UUID {
			f.rec.before = map[string]func(){"ChangesetVersions": func() {
				f.c.gone = true
				delete(f.auth.grants, domain.ActionRevert)
			}}
			return f.c.ID
		}, nil, 0, 0, "page.changeset_not_found"},
		{"its caller a reader between its items and its row", func(f revertFixture) uuid.UUID {
			f.rec.before = map[string]func(){"ChangesetVersions": func() {
				f.c.gone = true
				delete(f.auth.grants, domain.ActionRevert)
				f.auth.forbidden[domain.ActionRevert] = true
			}}
			return f.c.ID
		}, nil, 0, 0, "forbidden"},
		{"its root no longer its group's, past the history", func(f revertFixture) uuid.UUID {
			f.store.counts[f.box.ID] = [3]int{3, 1, 2}
			f.store.history = map[uuid.UUID]int{f.doc.ID: domain.MaxUnitHistory + 1}
			return f.c.ID
		}, nil, 0, 0, "page.revert_conflict"},
		{"past the pages", func(f revertFixture) uuid.UUID {
			box := f.page("Many", nil, 4)
			for i := range domain.MaxUnitPages {
				f.page(fmt.Sprintf("Page %d", i), &box.ID, float64(i))
			}
			return f.deletion(box.ID, now().Add(-time.Minute)).ID
		}, nil, 0, 0, "page.revert_too_large"},
		{"at the pages", func(f revertFixture) uuid.UUID {
			box := f.page("Many", nil, 4)
			for i := range domain.MaxUnitPages - 1 {
				f.page(fmt.Sprintf("Page %d", i), &box.ID, float64(i))
			}
			return f.deletion(box.ID, now().Add(-time.Minute)).ID
		}, nil, 0, 0, ""},
		{"past the history", func(f revertFixture) uuid.UUID {
			f.store.history = map[uuid.UUID]int{f.doc.ID: 1, f.scan.ID: domain.MaxUnitHistory}
			return f.c.ID
		}, nil, 0, 0, "page.revert_too_large"},
		{"at the history", func(f revertFixture) uuid.UUID {
			f.store.history = map[uuid.UUID]int{f.doc.ID: 1, f.scan.ID: domain.MaxUnitHistory - 1}
			return f.c.ID
		}, nil, 0, 0, ""},
		{"past the bytes, a conflict", func(f revertFixture) uuid.UUID {
			f.trashedContent(f.doc.ID, strings.Repeat("a", domain.MaxUnitBytes))
			f.trashedContent(f.box.ID, "b")
			f.store.nodes[f.other.ID] = domain.Node{ID: f.other.ID, NotebookID: f.eng, Kind: domain.KindPage, Name: "box", NameKey: "box"}
			return f.c.ID
		}, nil, 0, 0, "page.revert_too_large"},
		{"a conflict, the budget busy", func(f revertFixture) uuid.UUID {
			f.budget.err = shared.ServerBusy(time.Second)
			f.store.nodes[f.other.ID] = domain.Node{ID: f.other.ID, NotebookID: f.eng, Kind: domain.KindPage, Name: "box", NameKey: "box"}
			return f.c.ID
		}, nil, 0, 0, "page.revert_conflict"},
		{"a version the purge took, the budget busy", func(f revertFixture) uuid.UUID {
			f.budget.err = shared.ServerBusy(time.Second)
			delete(f.log.versions, domain.VersionRef{NodeID: f.doc.ID, Revision: 3})
			return f.c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"a content the purge took, the budget busy", func(f revertFixture) uuid.UUID {
			f.budget.err = shared.ServerBusy(time.Second)
			f.store.lost = map[uuid.UUID]bool{f.doc.ID: true}
			return f.c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"its root's group gone before it is read", func(f revertFixture) uuid.UUID {
			f.rec.before = map[string]func(){"TrashGroups 1": func() { delete(f.store.trash, f.box.ID) }}
			return f.c.ID
		}, nil, 0, 0, "page.revert_conflict"},
		{"a version to write back the purge took between its hash and its content", func(f revertFixture) uuid.UUID {
			c := f.writing(f.other.ID, "two")
			f.rec.before = map[string]func(){"VersionContents 1": func() {
				delete(f.log.versions, domain.VersionRef{NodeID: f.other.ID, Revision: 1})
			}}
			return c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"a version to write back the purge took, the budget busy", func(f revertFixture) uuid.UUID {
			f.budget.err = shared.ServerBusy(time.Second)
			c := f.writing(f.other.ID, "two")
			delete(f.log.versions, domain.VersionRef{NodeID: f.other.ID, Revision: 1})
			return c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"a conflict, past the steps", func(f revertFixture) uuid.UUID {
			f.twoSteps()
			f.store.nodes[f.other.ID] = domain.Node{ID: f.other.ID, NotebookID: f.eng, Kind: domain.KindPage, Name: "box", NameKey: "box"}
			return f.c.ID
		}, nil, 0, 1, "page.revert_conflict"},
		{"a version the purge took, past the steps", func(f revertFixture) uuid.UUID {
			f.twoSteps()
			delete(f.log.versions, domain.VersionRef{NodeID: f.doc.ID, Revision: 3})
			return f.c.ID
		}, nil, 0, 1, "page.changeset_trimmed"},
		{"past the steps, a content the purge took, the budget busy", func(f revertFixture) uuid.UUID {
			f.twoSteps()
			f.budget.err = shared.ServerBusy(time.Second)
			f.store.lost = map[uuid.UUID]bool{f.doc.ID: true}
			return f.c.ID
		}, nil, 0, 1, "page.revert_too_large"},
		{"at the steps", func(f revertFixture) uuid.UUID {
			f.twoSteps()
			return f.c.ID
		}, nil, 0, 2, ""},
		{"the budget busy, past the links", func(f revertFixture) uuid.UUID {
			f.budget.err = shared.ServerBusy(time.Second)
			f.trashedContent(f.doc.ID, links)
			f.trashedContent(f.box.ID, links)
			return f.c.ID
		}, nil, 0, 0, "server_busy"},
		{"past the links", func(f revertFixture) uuid.UUID {
			f.trashedContent(f.doc.ID, links)
			f.trashedContent(f.box.ID, links)
			return f.c.ID
		}, nil, 0, 0, "page.revert_too_large"},
		{"a defect's schedule: a page deeper than MaxDepth renamed", func(f revertFixture) uuid.UUID {
			// No write leaves a page deeper than MaxDepth, which S* keeps:
			// the rename's place back never fits, before or after its step
			// aside (M9/P3 design 3.16.4, (T4)), and no fallback is left.
			var parent *uuid.UUID
			for i := range domain.MaxDepth {
				parent = new(f.page(fmt.Sprintf("D%d", i+1), parent, 0).ID)
			}
			x := f.page("Old", parent, 0)
			before := x.State()
			x.Name, x.NameKey = "New", "new"
			f.store.nodes[x.ID] = x
			return f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{
				{NodeID: x.ID, Before: &before, After: new(x.State())}}).ID
		}, nil, 0, 0, domain.ErrRevertUnordered.Error()},
		{"trimmed meanwhile", func(f revertFixture) uuid.UUID {
			f.notebooks.waited = func() { f.c.Trimmed = true }
			return f.c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"purged meanwhile", func(f revertFixture) uuid.UUID {
			f.notebooks.waited = func() { f.c.gone = true }
			return f.c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"merged into past the items meanwhile", func(f revertFixture) uuid.UUID {
			f.notebooks.waited = func() {
				f.c.items = append(f.c.items, domain.ChangesetItem{NodeID: f.other.ID, After: new(f.other.State()), Revision: 1})
			}
			return f.c.ID
		}, nil, 3, 0, "page.revert_too_large"},
		{"a conflict meanwhile", func(f revertFixture) uuid.UUID {
			f.notebooks.waited = func() {
				f.store.nodes[f.other.ID] = domain.Node{ID: f.other.ID, NotebookID: f.eng, Kind: domain.KindPage, Name: "BOX", NameKey: "box"}
			}
			return f.c.ID
		}, nil, 0, 0, "page.revert_conflict"},
		{"expired while it waited", func(f revertFixture) uuid.UUID {
			f.clock.reads = int((retention - 2*time.Hour) / time.Microsecond)
			return f.c.ID
		}, nil, 0, 0, "page.revert_conflict"},
		{"a version the purge took meanwhile", func(f revertFixture) uuid.UUID {
			f.notebooks.waited = func() { delete(f.log.versions, domain.VersionRef{NodeID: f.doc.ID, Revision: 3}) }
			return f.c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"merged into past the steps meanwhile, a page in the trash at other bytes", func(f revertFixture) uuid.UUID {
			f.notebooks.waited = func() {
				f.twoSteps()
				f.trashedContent(f.doc.ID, "# Doc, other bytes")
			}
			return f.c.ID
		}, nil, 0, 1, "page.revert_too_large"},
		{"a page merged in meanwhile, not parsed", func(f revertFixture) uuid.UUID {
			f.notebooks.waited = func() {
				f.log.versions[domain.VersionRef{NodeID: f.other.ID, Revision: 1}] = ""
				f.content(f.other.ID, "x", 2)
				f.c.versions = append(f.c.versions, f.log.version(f.other.ID, 1, 2, "x"))
			}
			return f.c.ID
		}, nil, 0, 0, "page.revert_conflict"},
		{"a page in the trash at other bytes meanwhile", func(f revertFixture) uuid.UUID {
			f.notebooks.waited = func() { f.trashedContent(f.doc.ID, "# Doc, other bytes") }
			return f.c.ID
		}, nil, 0, 0, "page.revert_conflict"},
		{"a restore's rows the purge took meanwhile", func(f revertFixture) uuid.UUID {
			f.store.rows = &app.RestoredRows{Nodes: 2, Contents: 2, CurrentVersions: 2, Creations: 2}
			return f.c.ID
		}, nil, 0, 0, "page.changeset_trimmed"},
		{"the guard", func(f revertFixture) uuid.UUID {
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: domain.ErrLocked}}
			return f.c.ID
		}, nil, 0, 0, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newRevertFixture()
			id := tt.setup(f)
			_, err := f.revertWithin(id, tt.message, app.RevertBounds{Nodes: tt.maxItems, Steps: tt.maxSteps})
			if got := codeOf(err); got != tt.want {
				t.Errorf("revertChangeset = %q, want %q", got, tt.want)
			}
			if tt.want != "" && (len(f.store.changesets) != 0 && !f.tx.rolledBack || f.budget.held != 0) {
				t.Errorf("a refused revert left changesets %+v, rolled back %v, the budget holding %d; want nothing", f.store.changesets,
					f.tx.rolledBack, f.budget.held)
			}
			if tt.want == "" && len(f.store.changesets) != 1 {
				t.Errorf("changesets %+v, want the revert's", f.store.changesets)
			}
			if tt.name == "its root no longer its group's, past the history" && f.called("TrashGroups 1") {
				t.Errorf("the group of a root that fails its check was read: %v", f.rec.calls)
			}
			if tt.name == "past the items" && f.called("ChangesetItems") {
				t.Errorf("the items of a changeset past their bound were read: %v", f.rec.calls)
			}
			if tt.name == "a version to write back the purge took between its hash and its content" &&
				slices.ContainsFunc(f.rec.calls, func(c string) bool { return strings.HasSuffix(c, " in tx") }) {
				t.Errorf("a version the purge took before the parse went on to the unit: %v", f.rec.calls)
			}
		})
	}
}

// The revert's conflicts name each node and its reason; the unit's are
// authoritative.
func TestRevertChangesetsConflictsNameTheNodes(t *testing.T) {
	f := newRevertFixture()
	f.store.nodes[f.other.ID] = domain.Node{ID: f.other.ID, NotebookID: f.eng, Kind: domain.KindPage, Name: "BOX", NameKey: "box"}
	_, err := f.revert(f.c.ID, nil, 0)
	var e *shared.Error
	if !errors.As(err, &e) || e.Code != "page.revert_conflict" || len(e.Conflicts) != 1 || *e.ConflictCount != 1 ||
		e.Conflicts[0].NodeID != f.box.ID || e.Conflicts[0].Name != "Box" || e.Conflicts[0].Reason != "title_taken" ||
		*e.Conflicts[0].HolderID != f.other.ID {
		t.Errorf("revert = %v, %+v; want Box's title_taken held by BOX", err, e)
	}
}

// A page a changeset wrote and then deleted, a batch's, is restored at
// its content in the trash, then written back on its revision there (M9/P3
// design 3.18): two contents of the page, each parsed, each step with its
// own facts, as the guards see them, by page and hash; past MaxUnitBytes
// together, but a lone page's, refused by no bound (3.18.4).
func TestARevertRestoresAPageThenWritesItBack(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	o, g := &observer{recorder: f.rec}, &guard{recorder: f.rec}
	f.observers, f.guards = []app.PageObserver{o}, []app.WriteGuard{g}
	before := "[[Before]] " + strings.Repeat("b", domain.MaxContentBytes-len("[[Before]] "))
	written := "[[Written]] " + strings.Repeat("w", domain.MaxContentBytes-len("[[Written]] "))
	p := f.page("P", nil, 0)
	f.content(p.ID, before, 1)
	f.content(p.ID, written, 2)
	at := now().Add(-time.Hour)
	f.trashed(p.ID, at, f.alice)
	c := f.log.changeset(domain.ChangesetEdit, f.alice, at, []domain.ChangesetItem{{NodeID: p.ID, Before: new(p.State()), Revision: 2}},
		f.log.version(p.ID, 1, 2, written))
	if _, err := f.revert(c.ID, nil, 0); err != nil {
		t.Fatal(err)
	}
	if got := f.store.contents[p.ID]; got.Content != before || got.Revision != 3 {
		t.Errorf("P = revision %d of %d bytes, want the bytes before the write at revision 3", got.Revision, len(got.Content))
	}
	if len(o.events) != 1 || len(o.events[0].Changes) != 1 {
		t.Fatalf("events = %+v, want one of P", o.events)
	}
	// The unit's change of P: its restore, then its write, the facts of
	// the content written back.
	if c := o.events[0].Changes[0]; c.Restored == nil || c.Revision != 3 || c.Facts != before {
		t.Errorf("P's change = restored %v, revision %d, its facts the content before: %v", c.Restored, c.Revision, c.Facts == before)
	}
	var steps []string
	for _, s := range g.steps {
		for _, c := range s.Changes {
			steps = append(steps, fmt.Sprintf("%s %d %v", s.Operation, c.Revision, c.Facts == written))
		}
	}
	if want := []string{"restore 2 true", "content 3 false"}; !slices.Equal(steps, want) {
		t.Errorf("the guards saw %q, want the restore at 2 with the facts of its content in the trash, then the write back", steps)
	}
}

// Under the lock a page in the trash at another revision of the bytes
// parsed is no conflict: it goes back at the revision the unit reads.
func TestRevertChangesetRestoresAtTheRevisionItReads(t *testing.T) {
	f := newRevertFixture()
	o := &observer{recorder: f.rec}
	f.observers = []app.PageObserver{o}
	f.notebooks.waited = func() {
		c := f.store.trashed[f.doc.ID]
		c.Revision = 9
		f.store.trashed[f.doc.ID] = c
		f.log.versions[domain.VersionRef{NodeID: f.doc.ID, Revision: 9}] = c.Content
	}
	if _, err := f.revert(f.c.ID, nil, 0); err != nil {
		t.Fatal(err)
	}
	if c := o.events[0].Changes[1]; c.NodeID != f.doc.ID || c.Revision != 9 {
		t.Errorf("Doc's change = %+v, want revision 9", c)
	}
}

// A title taken in a place's write, its move's or its rename's, or a
// revision moved in a content's, which the plan found free and in place,
// is a defect: unclassified, no conflict of the revert's contract.
func TestRevertChangesetsDefectsAreUnclassified(t *testing.T) {
	for _, tt := range []struct {
		call string
		err  error
	}{
		{"RenameNode", domain.ErrTitleTaken},
		{"MoveNode", domain.ErrTitleTaken},
		{"RelocateNode", domain.ErrTitleTaken},
		{"WriteContent", domain.ErrRevisionMismatch},
	} {
		f := newFixture()
		f.grant(domain.ActionRevert)
		p := f.page("Old", nil, 0)
		c := f.renaming(p.ID, "New")
		switch tt.call {
		case "WriteContent":
			c = f.writing(p.ID, "two")
		case "MoveNode", "RelocateNode":
			box := f.page("Box", nil, 1)
			n := f.store.nodes[p.ID]
			before := n.State()
			n.ParentID = &box.ID
			if tt.call == "RelocateNode" { // an MCP move's item: a new parent and a new name
				n.Name, n.NameKey = "Moved", "moved"
			}
			f.store.nodes[p.ID] = n
			c = f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{{NodeID: p.ID, Before: &before,
				After: new(n.State())}})
		}
		f.store.errs = map[string]error{tt.call: tt.err}
		_, err := f.revert(c.ID, nil, 0)
		var e *shared.Error
		if err == nil || errors.As(err, &e) || !strings.Contains(err.Error(), tt.err.Error()) {
			t.Errorf("%s answering %v: the revert = %v; want it unclassified", tt.call, tt.err, err)
		}
	}
}

// A revert puts back an item that moved and renamed its node, an MCP
// move's, in one step of OpMove that one RelocateNode writes, its change
// carrying the parent and the name of each side (M9/P2 design 3.5.4); an
// item that moved alone is put back by one MoveNode, one that renamed
// alone by one RenameNode, as before.
func TestARevertPutsBackAMoveThatRenamesInOneStep(t *testing.T) {
	for _, tt := range []struct {
		name           string
		moved, renamed bool
		writes         []string
		op             domain.Operation
	}{
		{"a move that renames", true, true, []string{"RelocateNode"}, domain.OpMove},
		{"a move", true, false, []string{"MoveNode"}, domain.OpMove},
		{"a rename", false, true, []string{"RenameNode"}, domain.OpRename},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRevert)
			g := &guard{recorder: f.rec}
			f.guards = []app.WriteGuard{g}
			drafts, final := f.page("Drafts", nil, 0), f.page("Final", nil, 1)
			f.page("Plan", &drafts.ID, 0)
			x := f.page("Plan v2", &drafts.ID, 1)
			before := x.State()
			if tt.moved {
				x.ParentID, x.SortOrder = &final.ID, 0
			}
			if tt.renamed {
				x.Name, x.NameKey = "Plan", "plan"
			}
			f.store.nodes[x.ID] = x
			c := f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{{NodeID: x.ID, Before: &before,
				After: new(x.State())}})
			if _, err := f.revert(c.ID, nil, 0); err != nil {
				t.Fatal(err)
			}
			if got := f.store.nodes[x.ID]; *got.ParentID != drafts.ID || got.Name != "Plan v2" || got.NameKey != "plan v2" ||
				got.SortOrder != 1 {
				t.Errorf("after the revert, X = %+v, want Plan v2 under Drafts at 1", got)
			}
			if w := f.writes(); !slices.Equal(w, tt.writes) {
				t.Errorf("writes = %v, want %v", w, tt.writes)
			}
			if len(g.steps) != 1 || g.steps[0].Operation != tt.op || g.steps[0].Kind != domain.ChangesetRevert {
				t.Fatalf("the guard saw %+v, want one %s of a revert", g.steps, tt.op)
			}
			back := g.steps[0].Changes[0]
			if back.NodeID != x.ID || *back.Before != x.State() || *back.After != before {
				t.Errorf("the step's change = %+v, want X from %+v back to %+v", back, x.State(), before)
			}
		})
	}
}

// Under the lock, the pages parsed at another hash than the trash holds
// now are conflicts in their order, by node id, not the group's: Early,
// created first, moved under Box before Box's deletion, comes first.
func TestRevertChangesetsLockedConflictsComeByNodeID(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	early := f.page("Early", nil, 0)
	box := f.page("Box", nil, 1)
	n := f.store.nodes[early.ID]
	n.ParentID = &box.ID
	f.store.nodes[early.ID] = n
	f.content(early.ID, "early", 1)
	f.content(box.ID, "box", 1)
	rf := revertFixture{fixture: f, box: box}
	c := f.deletion(box.ID, now().Add(-time.Hour))
	f.notebooks.waited = func() {
		rf.trashedContent(box.ID, "box, other bytes")
		rf.trashedContent(early.ID, "early, other bytes")
	}
	_, err := f.revert(c.ID, nil, 0)
	var e *shared.Error
	if !errors.As(err, &e) || e.Code != "page.revert_conflict" || len(e.Conflicts) != 2 || e.Conflicts[0].NodeID != early.ID ||
		e.Conflicts[1].NodeID != box.ID {
		t.Errorf("revert = %v, %+v; want Early, then Box, changed", err, e)
	}
}

// A deletion's revert compares its top where and as the deletion left it,
// unless the top's deletion undid the restore of the deletion's own trash
// item (M8/P3A fix checks 1 and 2). X created as A, renamed B, deleted (D)
// and restored; the rename reverted, then the creation, which deletes X as
// A where D left it: D's revert finds X changed and writes nothing, as it
// would undo the rename's revert. Had that deletion been the revert of a
// changeset that restored D's item after D, a trash restore or a revert,
// D's revert would bring X back as B; of one after D whose item of X is
// no restore, of one that restored a later item of X, a deletion between
// them, or of one older than D, it finds X changed.
func TestRevertChangesetComparesATopAnotherRevertDeleted(t *testing.T) {
	for _, tt := range []struct {
		name string
		// undid gives the creation's revert's deletion of X another
		// changeset it reverts, made in the history, at an hour from D.
		undid   func(f *fixture, x domain.Node, from time.Time) uuid.UUID
		changed bool
	}{
		{"its creation's revert", nil, true},
		{"the revert of the restore of its item", func(f *fixture, x domain.Node, from time.Time) uuid.UUID {
			return f.log.changeset(domain.ChangesetTrashRestore, f.alice, from.Add(30*time.Minute),
				[]domain.ChangesetItem{{NodeID: x.ID, After: new(x.State()), Restored: true, Revision: 1}}).ID
		}, false},
		{"the revert of a revert's restore of its item", func(f *fixture, x domain.Node, from time.Time) uuid.UUID {
			return f.log.changeset(domain.ChangesetRevert, f.alice, from.Add(30*time.Minute),
				[]domain.ChangesetItem{{NodeID: x.ID, After: new(x.State()), Restored: true, Revision: 1}}).ID
		}, false},
		{"the revert of a changeset after it that brought it back, not restored", func(f *fixture, x domain.Node, from time.Time) uuid.UUID {
			return f.log.changeset(domain.ChangesetEdit, f.alice, from.Add(30*time.Minute),
				[]domain.ChangesetItem{{NodeID: x.ID, After: new(x.State()), Revision: 1}}).ID
		}, true},
		{"the revert of the restore of a later item", func(f *fixture, x domain.Node, from time.Time) uuid.UUID {
			f.log.changeset(domain.ChangesetEdit, f.alice, from.Add(20*time.Minute), []domain.ChangesetItem{{NodeID: x.ID, Before: new(x.State())}})
			return f.log.changeset(domain.ChangesetTrashRestore, f.alice, from.Add(30*time.Minute),
				[]domain.ChangesetItem{{NodeID: x.ID, After: new(x.State()), Restored: true, Revision: 1}}).ID
		}, true},
		{"the revert of a restore older than the deletion", func(f *fixture, x domain.Node, from time.Time) uuid.UUID {
			return f.log.changeset(domain.ChangesetTrashRestore, f.alice, from.Add(-30*time.Minute),
				[]domain.ChangesetItem{{NodeID: x.ID, After: new(x.State()), Restored: true, Revision: 1}}).ID
		}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.names[f.alice] = "Alice"
			f.grant(domain.ActionRevert)
			x := f.page("A", nil, 0)
			f.content(x.ID, "x", 1)
			creation := f.log.changeset(domain.ChangesetEdit, f.alice, now().Add(-3*time.Hour),
				[]domain.ChangesetItem{{NodeID: x.ID, After: new(x.State()), Revision: 1}}, f.log.version(x.ID, 0, 1, "x"))
			rename := f.renaming(x.ID, "B")
			deletedAt := now().Add(-2 * time.Hour)
			d := f.deletion(x.ID, deletedAt)
			b := f.store.trash[x.ID].Node
			if _, err := f.store.RestoreNodes(f.asAlice(), app.Restoration{Root: x.ID, IDs: []uuid.UUID{x.ID}, Names: []string{b.Name},
				NameKeys: []string{b.NameKey}, DeletedAt: deletedAt, By: f.alice, At: now().Add(-time.Hour)}); err != nil {
				t.Fatal(err)
			}
			f.log.changeset(domain.ChangesetTrashRestore, f.alice, now().Add(-time.Hour),
				[]domain.ChangesetItem{{NodeID: x.ID, After: new(b.State()), Restored: true, Revision: 1}})
			for _, c := range []*fakeChangeset{rename, creation} {
				if _, err := f.revert(c.ID, nil, 0); err != nil {
					t.Fatalf("the revert of %s: %v", c.ID, err)
				}
			}
			if n := f.store.trash[x.ID]; n.Name != "A" || !n.TrashRoot || len(f.store.changesets) != 2 {
				t.Fatalf("X = %+v, changesets %+v; want X deleted as A by the creation's revert", n, f.store.changesets)
			}
			if tt.undid != nil {
				f.store.changesets[1].RevertsID = tt.undid(f, b, deletedAt)
			}
			_, err := f.revert(d.ID, nil, 0)
			var e *shared.Error
			switch {
			case tt.changed && (!errors.As(err, &e) || e.Code != "page.revert_conflict" || len(e.Conflicts) != 1 ||
				e.Conflicts[0].NodeID != x.ID || e.Conflicts[0].Reason != "changed" || len(f.store.changesets) != 2):
				t.Errorf("D's revert = %v, %+v, changesets %+v; want X changed, nothing written", err, e, f.store.changesets)
			case !tt.changed && (err != nil || f.store.nodes[x.ID].Name != "B"):
				t.Errorf("D's revert = %v, X %+v; want X back as B", err, f.store.nodes[x.ID])
			}
		})
	}
}

// The full plan's contract is checked before locks or any unit's first
// step, including when the actor loses the extra grant while it waits.
func TestARevertsChecksRefuseTheContract(t *testing.T) {
	for _, route := range []string{"sync", "start", "plan"} {
		for _, tt := range []struct {
			name                         string
			seed, held, limited, demoted bool
			want                         string
		}{
			{name: "template", seed: true, want: "page.protected"},
			{name: "held template", seed: true, held: true, want: "page.protected"},
			{name: "source rename", want: "page.source_admin_only"},
			{name: "steps before contract", seed: true, limited: true, want: "page.revert_too_large"},
			{name: "locked denial", demoted: true, want: "page.source_admin_only"},
		} {
			if route != "sync" && (tt.limited || tt.demoted) {
				continue
			}
			t.Run(route+"/"+tt.name, func(t *testing.T) {
				f := newFixture()
				f.grant(domain.ActionRevert)
				raw, agents := f.page("raw", nil, 0), f.page("AGENTS", nil, 1)
				f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
				f.auth.forbidden[domain.ActionManageSource] = true
				c := f.renaming(raw.ID, "sources")
				if tt.seed {
					// The seed has two creations, so its undo exceeds a one-step bound.
					n := f.store.nodes[raw.ID]
					n.Name, n.NameKey = "raw", "raw"
					f.store.nodes[raw.ID] = n
					c = f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{
						{NodeID: raw.ID, After: new(n.State()), Revision: 1},
						{NodeID: agents.ID, After: new(agents.State()), Revision: 1},
					}, f.log.version(raw.ID, 0, 1, ""), f.log.version(agents.ID, 0, 1, ""))
				}
				if tt.held {
					f.session(agents.ID, uuid.NewV7(), now().Add(time.Minute))
				}
				// This guard records and permits everything: only Unit.Revert's
				// precheck can refuse before a step is seen.
				g := &guard{recorder: f.rec}
				f.guards = []app.WriteGuard{g}
				if tt.demoted {
					f.grant(domain.ActionManageSource)
					f.notebooks.waited = func() { delete(f.auth.grants, domain.ActionManageSource) }
				}
				var err error
				switch route {
				case "sync":
					bounds := app.RevertBounds{}
					if tt.limited {
						bounds.Steps = 1
					}
					_, err = f.revertWithin(c.ID, nil, bounds)
				case "start":
					_, err = f.jobs(now(), 0, 0).CheckRevert(f.asAlice(), f.eng, c.ID, nil, nil, allJobBatches)
				case "plan":
					_, err = f.jobs(now(), 0, 0).PlanRevert(f.asJob(), f.jobSpec(), c.ID, allJobBatches)
				}
				if codeOf(err) != tt.want {
					t.Errorf("revert = %v, want %s", err, tt.want)
				}
				if !tt.limited {
					var refusal *domain.Refused
					if !errors.As(err, &refusal) || refusal.NodeID != raw.ID {
						t.Errorf("refusal = %#v, want raw %s", err, raw.ID)
					}
					if f.count("StateOf") != 1 {
						t.Errorf("calls = %v, want one state read", f.rec.calls)
					}
				}
				asked := 0
				if !tt.seed {
					asked = 1
				}
				locked := 0
				if tt.demoted {
					locked = 1
				}
				if f.count("Authorize page.manage_source") != asked || f.count("Authorize page.manage_source in tx") != locked ||
					f.count("LockNotebook in tx") != locked || len(g.steps) != 0 || len(f.store.changesets) != 0 {
					t.Errorf("calls = %v, steps = %+v, changesets = %+v; want %d unlocked grants, %d locked, no steps or writes", f.rec.calls, g.steps, f.store.changesets, asked, locked)
				}
			})
		}
	}
}

// Losing visibility during the unlocked source decision answers the
// operation's 404; a database failure stays itself, with no refusal mark.
func TestARevertsExtraDecisionKeepsItsErrors(t *testing.T) {
	db := errors.New("source authorization database unavailable")
	for _, route := range []string{"sync", "start", "plan"} {
		for _, tt := range []struct {
			name string
			err  error
		}{{"not visible", shared.ErrNotVisible}, {"database", db}} {
			t.Run(route+"/"+tt.name, func(t *testing.T) {
				f := newFixture()
				f.grant(domain.ActionRevert)
				raw := f.page("raw", nil, 0)
				f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
				c := f.renaming(raw.ID, "sources")
				f.auth.failing = map[shared.Action]error{domain.ActionManageSource: tt.err}
				var err error
				switch route {
				case "sync":
					_, err = f.revert(c.ID, nil, 0)
				case "start":
					_, err = f.jobs(now(), 0, 0).CheckRevert(f.asAlice(), f.eng, c.ID, nil, nil, allJobBatches)
				case "plan":
					_, err = f.jobs(now(), 0, 0).PlanRevert(f.asJob(), f.jobSpec(), c.ID, allJobBatches)
				}
				want := tt.err
				if errors.Is(tt.err, shared.ErrNotVisible) {
					want = domain.ErrNotebookNotFound
					if route == "sync" {
						want = domain.ErrChangesetNotFound
					}
				}
				var refusal *domain.Refused
				if !errors.Is(err, want) || errors.As(err, &refusal) {
					t.Errorf("decision = %v, want unmarked %v", err, want)
				}
				if f.count("Authorize page.manage_source") != 1 || f.called("LockNotebook in tx") {
					t.Errorf("calls = %v, want one extra decision, no unit", f.rec.calls)
				}
				for _, target := range f.auth.targets {
					if target != (shared.Target{WorkspaceID: f.acme, NotebookID: f.eng}) {
						t.Errorf("target = %+v", target)
					}
				}
			})
		}
	}
}

// A swap makes the source root step aside and then return. Both steps
// need the same grant, asked once by each precheck and shared with the
// real mode guard only inside the unit.
func TestARevertsSourceGrantIsAskedOnceEachPass(t *testing.T) {
	for _, route := range []string{"sync", "start", "plan"} {
		t.Run(route, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRevert, domain.ActionManageSource)
			other, raw := f.page("sources", nil, 0), f.page("raw", nil, 1)
			f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID}}
			c := f.renaming(raw.ID, "sources")
			before := other.State()
			other.Name, other.NameKey = "raw", "raw"
			f.store.nodes[other.ID] = other
			c.items = append(c.items, domain.ChangesetItem{NodeID: other.ID, Before: &before, After: new(other.State())})
			g := &guard{recorder: f.rec}
			f.guards = []app.WriteGuard{app.NewModeGuard(f.auth, f.store), g}
			var err error
			switch route {
			case "sync":
				_, err = f.revert(c.ID, nil, 0)
			case "start":
				_, err = f.jobs(now(), 0, 0).CheckRevert(f.asAlice(), f.eng, c.ID, nil, nil, allJobBatches)
			case "plan":
				_, err = f.jobs(now(), 0, 0).PlanRevert(f.asJob(), f.jobSpec(), c.ID, allJobBatches)
			}
			locked := 0
			if route == "sync" {
				locked = 1
			}
			if err != nil || f.count("Authorize page.manage_source") != 1 || f.count("Authorize page.manage_source in tx") != locked {
				t.Errorf("%s = %v after %v; want one unlocked and %d locked source decisions", route, err, f.rec.calls, locked)
			}
			if route == "sync" && (len(g.steps) != 3 || g.steps[0].Changes[0].NodeID != raw.ID || g.steps[2].Changes[0].NodeID != raw.ID ||
				f.store.nodes[raw.ID].Name != "raw" || f.store.nodes[other.ID].Name != "sources") {
				t.Errorf("swap's undo saw steps %+v, nodes %+v; want source aside, other back, source back", g.steps, f.store.nodes)
			}
		})
	}
}
