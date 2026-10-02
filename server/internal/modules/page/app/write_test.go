package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// create runs createPage in eng as alice from the web.
func (f *fixture) create(d app.PageDraft) (app.PageView, error) {
	return app.NewCreatePage(f.writer(), f.store, f.logger()).Execute(f.asAlice(), f.eng, d, domain.ClientWeb)
}

// rename runs renameNode as alice from the web.
func (f *fixture) rename(id uuid.UUID, name string) (domain.Node, error) {
	return app.NewRenameNode(f.writer(), f.store, f.logger()).Execute(f.asAlice(), id, name, domain.ClientWeb)
}

// codeOf is err's problem code, or its text.
func codeOf(err error) string {
	var e *shared.Error
	if errors.As(err, &e) {
		if e.Kind == shared.KindInvalid {
			return e.Code + " " + e.Fields[0].Field
		}
		return e.Code
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

// The unit finds the notebook's workspace unlocked, then shares its row,
// locks the notebook's, decides on the target, and only then reads and
// writes, all in its transaction.
func TestCreatePageLocksThenDecides(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	if _, err := f.create(app.PageDraft{Title: "Notes"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"WorkspaceOf", "ShareWorkspace in tx", "LockNotebook in tx", "Authorize page.create in tx", "Children in tx"}
	if !slices.Equal(f.rec.calls[:len(want)], want) {
		t.Errorf("calls = %v, want them to begin %v", f.rec.calls, want)
	}
	if f.auth.targets[0] != (shared.Target{WorkspaceID: f.acme, NotebookID: f.eng}) {
		t.Errorf("decided on %+v, want acme's eng", f.auth.targets[0])
	}
}

// A new page has an empty content at revision 1, and the unit's changeset
// records it: one changeset of the client, the page's item and version;
// all at the clock's one reading.
func TestCreatePageWritesThePageAndItsChangeset(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	parent := f.page("Parent", nil, 0)
	v, err := f.create(app.PageDraft{ParentID: &parent.ID, Title: "  Notes  "})
	if err != nil {
		t.Fatal(err)
	}
	n := v.Node
	if n.Name != "Notes" || n.NameKey != "notes" || *n.ParentID != parent.ID || n.CreatedBy != f.alice || !n.CreatedAt.Equal(now()) {
		t.Errorf("page = %+v, want Notes under Parent, by alice at %v", n, now())
	}
	wantAncestors := []domain.Ancestor{{ID: parent.ID, Name: "Parent"}}
	if !slices.Equal(v.Ancestors, wantAncestors) || v.Content != (app.ContentMeta{Revision: 1, UpdatedBy: f.alice, UpdatedAt: now()}) {
		t.Errorf("view = %+v, %+v; want Parent above, revision 1 by alice at %v", v.Ancestors, v.Content, now())
	}
	if c := f.store.contents[n.ID]; c.Content != "" || len(c.Hash) != 32 {
		t.Errorf("content = %+v, want empty with its SHA-256", c)
	}
	if len(f.store.changesets) != 1 || f.store.changesets[0].Client != domain.ClientWeb || f.store.changesets[0].Kind != "edit" ||
		!f.store.changesets[0].At.Equal(now()) {
		t.Errorf("changesets = %+v, want one edit of the web at %v", f.store.changesets, now())
	}
	cs := f.store.changesets[0].ID
	item := f.store.items[n.ID]
	if item.ChangesetID != cs || item.Change.Before != nil || item.Change.After.Name != "Notes" || !item.At.Equal(now()) {
		t.Errorf("item = %+v, want the page created in the changeset", item)
	}
	if len(f.store.revisions) != 1 || f.store.revisions[0].Base != nil || f.store.revisions[0].Revision != 1 ||
		f.store.revisions[0].ChangesetID != cs || !f.store.revisions[0].At.Equal(now()) {
		t.Errorf("versions = %+v, want revision 1 without a base in the changeset", f.store.revisions)
	}
	if f.clock.reads != 1 {
		t.Errorf("the clock was read %d times, want once", f.clock.reads)
	}
}

// Omitted, the place is after the last sibling; First is before the first;
// After a sibling is right after it. Siblings with no gap left are
// renumbered in their order.
func TestCreatePagePlacesItAmongItsSiblings(t *testing.T) {
	for _, tt := range []struct {
		name  string
		place func(a, b domain.Node) app.Position
		want  []string
	}{
		{"omitted", func(a, b domain.Node) app.Position { return app.Position{} }, []string{"A", "B", "New"}},
		{"first", func(a, b domain.Node) app.Position { return app.First() }, []string{"New", "A", "B"}},
		{"after A", func(a, b domain.Node) app.Position { return app.After(a.ID) }, []string{"A", "New", "B"}},
		{"after B", func(a, b domain.Node) app.Position { return app.After(b.ID) }, []string{"A", "B", "New"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionCreate)
			a, b := f.page("A", nil, 0), f.page("B", nil, 1)
			if _, err := f.create(app.PageDraft{Title: "New", Position: tt.place(a, b)}); err != nil {
				t.Fatal(err)
			}
			if got := orderedNames(t, f); !slices.Equal(got, tt.want) {
				t.Errorf("roots = %v, want %v", got, tt.want)
			}
		})
	}
	t.Run("no gap left", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionCreate)
		a := f.page("A", nil, 1)
		f.page("B", nil, 1+1e-12)
		if _, err := f.create(app.PageDraft{Title: "New", Position: app.After(a.ID)}); err != nil {
			t.Fatal(err)
		}
		if got := orderedNames(t, f); !slices.Equal(got, []string{"A", "New", "B"}) || !f.called("SetSortOrder in tx") {
			t.Errorf("roots = %v, renumbered %v; want A, New, B, renumbered", got, f.called("SetSortOrder in tx"))
		}
	})
}

func orderedNames(t *testing.T, f *fixture) []string {
	t.Helper()
	nodes, err := f.store.Children(context.Background(), f.eng, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range nodes {
		out = append(out, n.Name)
	}
	return out
}

// The codes come in order: the notebook's 404, then 403, then the values'
// 422, then the conflicts' 409, then a guard's.
func TestCreatePageAnswersItsCodesInOrder(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	for _, tt := range []struct {
		name  string
		setup func(f *fixture) app.PageDraft
		want  string
	}{
		{"an unknown notebook before all", func(f *fixture) app.PageDraft {
			f.eng = uuid.NewV7()
			return app.PageDraft{Title: ""}
		}, "notebook.not_found"},
		{"a notebook deleted while it waited", func(f *fixture) app.PageDraft {
			f.notebooks.gone[f.eng] = true
			return app.PageDraft{Title: ""}
		}, "notebook.not_found"},
		{"a workspace deleted while it waited", func(f *fixture) app.PageDraft {
			f.workspaces.gone[f.acme] = true
			return app.PageDraft{Title: ""}
		}, "notebook.not_found"},
		{"a notebook the caller cannot see", func(f *fixture) app.PageDraft {
			return app.PageDraft{Title: ""}
		}, "notebook.not_found"},
		{"a reader before the values", func(f *fixture) app.PageDraft {
			f.auth.forbidden[domain.ActionCreate] = true
			return app.PageDraft{Title: ""}
		}, "forbidden"},
		{"the title before the parent", func(f *fixture) app.PageDraft {
			f.grant(domain.ActionCreate)
			missing := uuid.NewV7()
			return app.PageDraft{Title: "a/b", ParentID: &missing}
		}, "validation_failed title"},
		{"a parent of no page of the notebook", func(f *fixture) app.PageDraft {
			f.grant(domain.ActionCreate)
			missing := uuid.NewV7()
			return app.PageDraft{Title: "Notes", ParentID: &missing}
		}, "validation_failed parent_id"},
		{"a sibling to follow of another parent", func(f *fixture) app.PageDraft {
			f.grant(domain.ActionCreate)
			parent := f.page("Parent", nil, 0)
			return app.PageDraft{Title: "Notes", Position: app.After(parent.ID), ParentID: &parent.ID}
		}, "validation_failed after_id"},
		{"the values before a title taken", func(f *fixture) app.PageDraft {
			f.grant(domain.ActionCreate)
			f.page("Notes", nil, 0)
			return app.PageDraft{Title: "NOTES", Position: app.After(uuid.NewV7())}
		}, "validation_failed after_id"},
		{"a title taken before the guard", func(f *fixture) app.PageDraft {
			f.grant(domain.ActionCreate)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			f.page("Notes", nil, 0)
			return app.PageDraft{Title: "NOTES"}
		}, "page.title_taken"},
		{"the guard last", func(f *fixture) app.PageDraft {
			f.grant(domain.ActionCreate)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			return app.PageDraft{Title: "Notes"}
		}, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			d := tt.setup(f)
			before := len(f.store.nodes)
			_, err := f.create(d)
			if got := codeOf(err); got != tt.want {
				t.Errorf("createPage = %q, want %q", got, tt.want)
			}
			if len(f.store.nodes) != before || f.logs.Len() != 0 {
				t.Errorf("a refused createPage wrote %d nodes, logged %q; want neither", len(f.store.nodes)-before, f.logs)
			}
		})
	}
}

// Pages nest MaxDepth levels at most: a child of the tenth level is 409.
func TestCreatePageRefusesAnEleventhLevel(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	var parent *uuid.UUID
	var levels []domain.Node
	for i := range domain.MaxDepth {
		n := f.page("Level "+string(rune('A'+i)), parent, 0)
		levels = append(levels, n)
		parent = &n.ID
	}
	if _, err := f.create(app.PageDraft{ParentID: &levels[domain.MaxDepth-2].ID, Title: "Tenth"}); err != nil {
		t.Errorf("a page at the tenth level = %v, want it created", err)
	}
	if _, err := f.create(app.PageDraft{ParentID: &levels[domain.MaxDepth-1].ID, Title: "Eleventh"}); codeOf(err) != "page.too_deep" {
		t.Errorf("a page at the eleventh level = %v, want page.too_deep", err)
	}
}

// The guard sees the operation as the checks left it: the title checked,
// the order placed, the node not yet written.
func TestTheGuardSeesTheCheckedChange(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	f.page("A", nil, 3)
	v, err := f.create(app.PageDraft{Title: "  Notes  "})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.steps) != 1 {
		t.Fatalf("the guard saw %d steps, want 1", len(g.steps))
	}
	s := g.steps[0]
	want := app.Write{WorkspaceID: f.acme, NotebookID: f.eng, By: f.alice, Client: domain.ClientWeb, Options: app.Options{UpdateLinks: true}, At: now()}
	if s.Write != want || s.Operation != domain.OpCreate || len(s.Changes) != 1 || s.Changes[0].NodeID != v.Node.ID ||
		s.Changes[0].Before != nil || s.Changes[0].After.Name != "Notes" || s.Changes[0].After.SortOrder != 4 {
		t.Errorf("the guard saw %+v (after %+v), want the creation of Notes at order 4 by %+v", s, s.Changes[0].After, want)
	}
	if i, j := slices.Index(f.rec.calls, "GuardWrite create in tx"), slices.Index(f.rec.calls, "CreateNode in tx"); i < 0 || i > j {
		t.Errorf("calls = %v, want the guard before the write", f.rec.calls)
	}
}

// Its log tells the ids of the write, never the title; a refused write
// logs nothing (TestCreatePageAnswersItsCodesInOrder).
func TestCreatePageLogsTheIDsNotTheTitle(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	v, err := f.create(app.PageDraft{Title: "Zebrafish Ω"})
	if err != nil {
		t.Fatal(err)
	}
	logs := f.logs.String()
	for _, want := range []string{"page created", f.acme.String(), f.eng.String(), v.Node.ID.String(), f.store.changesets[0].ID.String(),
		f.alice.String(), "client=web"} {
		if !strings.Contains(logs, want) {
			t.Errorf("log %q lacks %q", logs, want)
		}
	}
	if strings.Contains(logs, "Zebrafish") {
		t.Errorf("log %q tells the title", logs)
	}
}

// A rename finds the node, then runs a unit of its notebook that finds it
// again under the lock.
func TestRenameNodeRenamesUnderTheLock(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	n := f.page("Notes", nil, 0)
	got, err := f.rename(n.ID, "Journal")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Journal" || got.NameKey != "journal" || got.UpdatedBy != f.alice || !got.UpdatedAt.Equal(now()) {
		t.Errorf("renamed = %+v, want Journal by alice at %v", got, now())
	}
	want := []string{"FindNode", "WorkspaceOf", "ShareWorkspace in tx", "LockNotebook in tx", "Authorize node.rename in tx", "FindNodeIn in tx"}
	if !slices.Equal(f.rec.calls[:len(want)], want) {
		t.Errorf("calls = %v, want them to begin %v", f.rec.calls, want)
	}
	item := f.store.items[n.ID]
	if item.Change.Before.Name != "Notes" || item.Change.After.Name != "Journal" || len(f.store.revisions) != 0 {
		t.Errorf("item = %+v, versions %v; want Notes to Journal, no version", item, f.store.revisions)
	}
	if !strings.Contains(f.logs.String(), "node renamed") || strings.Contains(f.logs.String(), "Journal") {
		t.Errorf("log %q, want the rename without its name", f.logs)
	}
}

// The same name writes nothing: no changeset, no event, no log. A name
// that differs in case alone is the same key, no conflict with the node
// itself, and is written.
func TestRenameNodeToItsOwnName(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	o := &observer{recorder: f.rec}
	f.observers = []app.PageObserver{o}
	n := f.page("Notes", nil, 0)
	if _, err := f.rename(n.ID, " Notes "); err != nil || f.called("RenameNode in tx") || len(f.store.changesets) != 0 ||
		len(o.events) != 0 || f.logs.Len() != 0 {
		t.Errorf("renaming Notes to Notes = %v, renamed %v, %d changesets, %d events, log %q; want nothing", err,
			f.called("RenameNode in tx"), len(f.store.changesets), len(o.events), f.logs)
	}
	got, err := f.rename(n.ID, "NOTES")
	if err != nil || got.Name != "NOTES" || !f.called("RenameNode in tx") || len(o.events) != 1 {
		t.Errorf("renaming Notes to NOTES = %+v, %v, %d events; want NOTES written and told", got, err, len(o.events))
	}
}

func TestRenameNodeAnswersItsCodesInOrder(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	for _, tt := range []struct {
		name  string
		setup func(f *fixture, n domain.Node) (uuid.UUID, string)
		want  string
	}{
		{"an unknown node", func(f *fixture, n domain.Node) (uuid.UUID, string) {
			return uuid.NewV7(), ""
		}, "page.not_found"},
		{"a notebook deleted while it waited", func(f *fixture, n domain.Node) (uuid.UUID, string) {
			f.grant(domain.ActionRename)
			f.notebooks.gone[f.eng] = true
			return n.ID, ""
		}, "page.not_found"},
		{"a notebook the caller cannot see", func(f *fixture, n domain.Node) (uuid.UUID, string) {
			return n.ID, ""
		}, "page.not_found"},
		{"a reader", func(f *fixture, n domain.Node) (uuid.UUID, string) {
			f.auth.forbidden[domain.ActionRename] = true
			return n.ID, ""
		}, "forbidden"},
		{"a name that breaks the rules", func(f *fixture, n domain.Node) (uuid.UUID, string) {
			f.grant(domain.ActionRename)
			return n.ID, "CON"
		}, "validation_failed name"},
		{"a sibling's name", func(f *fixture, n domain.Node) (uuid.UUID, string) {
			f.grant(domain.ActionRename)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			f.page("Journal", nil, 1)
			return n.ID, "journal"
		}, "page.title_taken"},
		{"the guard last", func(f *fixture, n domain.Node) (uuid.UUID, string) {
			f.grant(domain.ActionRename)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			return n.ID, "Journal"
		}, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			n := f.page("Notes", nil, 0)
			id, name := tt.setup(f, n)
			_, err := f.rename(id, name)
			if got := codeOf(err); got != tt.want {
				t.Errorf("renameNode = %q, want %q", got, tt.want)
			}
			if f.store.nodes[n.ID].Name != "Notes" || f.logs.Len() != 0 {
				t.Errorf("a refused rename left %q, logged %q; want Notes, nothing", f.store.nodes[n.ID].Name, f.logs)
			}
		})
	}
}
