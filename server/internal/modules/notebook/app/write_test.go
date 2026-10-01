package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func (f fixture) create() *app.CreateNotebook {
	return app.NewCreateNotebook(app.CreateNotebookDeps{
		Workspaces: f.workspaces, Notebooks: f.store, Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
	})
}

func (f fixture) update() *app.UpdateNotebook {
	return app.NewUpdateNotebook(app.UpdateNotebookDeps{
		Workspaces: f.workspaces, Finder: f.store, Notebooks: f.store, Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
	})
}

func (f fixture) delete(subscribers ...app.NotebookDeletionSubscriber) *app.DeleteNotebook {
	return app.NewDeleteNotebook(app.DeleteNotebookDeps{
		Workspaces: f.workspaces, Finder: f.store, Notebooks: f.store, Subscribers: subscribers,
		Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
	})
}

func ptr(s string) *string { return &s }

func TestCreateNotebookMakesTheCallerItsAdmin(t *testing.T) {
	f := newFixture()
	alice := uuid.NewV7()
	f.grant(domain.ActionCreate, shared.WorkspaceMember, "")

	got, err := f.create().Execute(as(alice), "acme", "  Notes ", ptr("viewer"))

	if err != nil || len(f.store.created) != 1 || len(f.store.members) != 1 {
		t.Fatalf("Execute() = %+v, %v; created %+v, members %+v", got, err, f.store.created, f.store.members)
	}
	n, m := f.store.created[0], f.store.members[0]
	if want := (domain.Notebook{ID: n.ID, WorkspaceID: f.acme, Name: "Notes", Access: shared.AccessViewer, CreatedAt: now(), UpdatedAt: now()}); n != want || n.ID == uuid.Nil() {
		t.Errorf("created %+v, want %+v", n, want)
	}
	if want := (domain.Member{ID: m.ID, NotebookID: n.ID, UserID: alice, Role: shared.NotebookAdmin, CreatedAt: now()}); m != want || m.ID == uuid.Nil() {
		t.Errorf("member %+v, want %+v", m, want)
	}
	if want := (app.View{Notebook: n, Role: shared.NotebookAdmin, MemberCount: 1}); got != want {
		t.Errorf("Execute() = %+v, want %+v", got, want)
	}
	// The workspace's share, the decision, then the writes, all in the
	// transaction.
	calls := []string{"FindBySlug acme", "ShareByID in tx", "Authorize notebook.create in tx",
		"CreateNotebook by " + alice.String() + " in tx", "AddMember admin by " + alice.String() + " in tx"}
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}
	logs := f.logs.String()
	if !strings.Contains(logs, "notebook created") || !strings.Contains(logs, n.ID.String()) || strings.Contains(logs, "Notes") {
		t.Errorf("logs = %q, want the creation by its ids, not its name", logs)
	}
}

func TestCreateNotebookWithoutAnAccessIsPrivate(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate, shared.WorkspaceAdmin, "")
	if got, err := f.create().Execute(as(uuid.NewV7()), "acme", "Notes", nil); err != nil || got.Notebook.Access != shared.AccessNone {
		t.Errorf("Execute() = %+v, %v; want none", got, err)
	}
}

// The codes come 404, 403, 422 (v0.1 design 13.1, item 4): the values are
// checked under the lock and after the decision, and nothing is written
// when any refuses.
func TestCreateNotebookRefuses(t *testing.T) {
	forbidden := shared.Forbidden()
	for _, tt := range []struct {
		name, slug, value string
		gone              bool
		err               error
		want              error
		calls             []string
	}{
		{"no workspace", "nope", "Notes", false, nil, domain.ErrWorkspaceNotFound, []string{"FindBySlug nope"}},
		{"deleted while it waited", "acme", "Notes", true, nil, domain.ErrWorkspaceNotFound, []string{"FindBySlug acme", "ShareByID in tx"}},
		{"not a member, and a bad name", "acme", "a/b", false, shared.ErrNotVisible, domain.ErrWorkspaceNotFound,
			[]string{"FindBySlug acme", "ShareByID in tx", "Authorize notebook.create in tx"}},
		{"a guest, and a bad name", "acme", "a/b", false, forbidden, forbidden,
			[]string{"FindBySlug acme", "ShareByID in tx", "Authorize notebook.create in tx"}},
	} {
		f := newFixture()
		f.workspaces.gone[f.acme] = tt.gone
		f.auth.err = tt.err
		got, err := f.create().Execute(as(uuid.NewV7()), tt.slug, tt.value, nil)
		if !errors.Is(err, tt.want) || got != (app.View{}) || !slices.Equal(f.rec.calls, tt.calls) {
			t.Errorf("%s: Execute() = %+v, %v, calls %q; want %v after %q", tt.name, got, err, f.rec.calls, tt.want, tt.calls)
		}
	}
	f := newFixture()
	f.grant(domain.ActionCreate, shared.WorkspaceMember, "")
	_, err := f.create().Execute(as(uuid.NewV7()), "acme", "CON", ptr("public"))
	var se *shared.Error
	if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || len(se.Fields) != 2 || len(f.store.created) != 0 || !f.tx.rolledBack {
		t.Errorf("a member's bad values = %v; want both problems, nothing written", err)
	}
}

func TestUpdateNotebook(t *testing.T) {
	f := newFixture()
	bob := uuid.NewV7()
	f.grant(domain.ActionUpdate, shared.WorkspaceMember, shared.NotebookAdmin)

	got, err := f.update().Execute(as(bob), f.notebook.ID, ptr(" Eng "), ptr("editor"))

	changed := f.notebook
	changed.Name, changed.Access, changed.UpdatedAt = "Eng", shared.AccessEditor, now()
	if want := (app.View{Notebook: changed, Role: shared.NotebookAdmin, MemberCount: 3}); err != nil || got != want {
		t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
	}
	if !slices.Equal(f.store.updated, []domain.Notebook{changed}) {
		t.Errorf("updated %+v, want %+v", f.store.updated, changed)
	}
	// The workspace's share, the notebook's lock, the decision, then the
	// write, all in the transaction.
	calls := []string{"FindNotebook", "ShareByID in tx", "LockNotebook in tx", "Authorize notebook.update in tx",
		"UpdateNotebook by " + bob.String() + " in tx", "CountMembers in tx"}
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}
	if want := (shared.Target{WorkspaceID: f.acme, NotebookID: f.notebook.ID}); f.auth.targets[0] != want {
		t.Errorf("target = %+v, want %+v", f.auth.targets[0], want)
	}
	if !strings.Contains(f.logs.String(), "notebook updated") || strings.Contains(f.logs.String(), "Eng") {
		t.Errorf("logs = %q, want the update by its ids", f.logs.String())
	}
}

// Values that change nothing write nothing: the answer is the notebook as
// it is, updated_at too.
func TestUpdateNotebookWithoutAChange(t *testing.T) {
	for _, tt := range []struct {
		name          string
		value, access *string
	}{
		{"no field", nil, nil},
		{"the same values", ptr(" Engineering "), ptr("none")},
	} {
		f := newFixture()
		f.grant(domain.ActionUpdate, shared.WorkspaceMember, shared.NotebookAdmin)
		got, err := f.update().Execute(as(uuid.NewV7()), f.notebook.ID, tt.value, tt.access)
		if want := (app.View{Notebook: f.notebook, Role: shared.NotebookAdmin, MemberCount: 3}); err != nil || got != want ||
			len(f.store.updated) != 0 || f.logs.Len() != 0 {
			t.Errorf("%s: Execute() = %+v, %v, updated %+v, logs %q; want the notebook unchanged, nothing written", tt.name, got, err, f.store.updated, f.logs)
		}
	}
}

func TestUpdateNotebookRefuses(t *testing.T) {
	forbidden := shared.Forbidden()
	for _, tt := range []struct {
		name  string
		setup func(f fixture) uuid.UUID
		want  error
		calls []string
	}{
		{"no notebook", func(fixture) uuid.UUID { return uuid.NewV7() }, domain.ErrNotFound, []string{"FindNotebook"}},
		{"its workspace deleted while it waited", func(f fixture) uuid.UUID { f.workspaces.gone[f.acme] = true; return f.notebook.ID },
			domain.ErrNotFound, []string{"FindNotebook", "ShareByID in tx"}},
		{"deleted while it waited", func(f fixture) uuid.UUID { f.store.deleted[f.notebook.ID] = true; return f.notebook.ID },
			domain.ErrNotFound, []string{"FindNotebook", "ShareByID in tx", "LockNotebook in tx"}},
		{"no role in it, and a bad name", func(f fixture) uuid.UUID { return f.notebook.ID },
			domain.ErrNotFound, []string{"FindNotebook", "ShareByID in tx", "LockNotebook in tx", "Authorize notebook.update in tx"}},
		{"an editor, and a bad name", func(f fixture) uuid.UUID { f.auth.err = forbidden; return f.notebook.ID },
			forbidden, []string{"FindNotebook", "ShareByID in tx", "LockNotebook in tx", "Authorize notebook.update in tx"}},
	} {
		f := newFixture()
		got, err := f.update().Execute(as(uuid.NewV7()), tt.setup(f), ptr("a/b"), nil)
		if !errors.Is(err, tt.want) || got != (app.View{}) || !slices.Equal(f.rec.calls, tt.calls) {
			t.Errorf("%s: Execute() = %+v, %v, calls %q; want %v after %q", tt.name, got, err, f.rec.calls, tt.want, tt.calls)
		}
	}
	f := newFixture()
	f.grant(domain.ActionUpdate, shared.WorkspaceMember, shared.NotebookAdmin)
	_, err := f.update().Execute(as(uuid.NewV7()), f.notebook.ID, ptr(" "), ptr("Viewer"))
	var se *shared.Error
	if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || len(se.Fields) != 2 || len(f.store.updated) != 0 {
		t.Errorf("an admin's bad values = %v; want both problems, nothing written", err)
	}
}

func TestDeleteNotebookTellsTheSubscribers(t *testing.T) {
	f := newFixture()
	bob := uuid.NewV7()
	f.grant(domain.ActionDelete, shared.WorkspaceMember, shared.NotebookAdmin)
	first, second := &fakeSubscriber{recorder: f.rec, name: "first"}, &fakeSubscriber{recorder: f.rec, name: "second"}

	if err := f.delete(first, second).Execute(as(bob), f.notebook.ID); err != nil {
		t.Fatal(err)
	}
	calls := []string{"FindNotebook", "ShareByID in tx", "LockNotebook in tx", "Authorize notebook.delete in tx",
		"DeleteNotebook by " + bob.String() + " at 2026-10-02T10:00:00Z in tx", "NotebookDeleted first in tx", "NotebookDeleted second in tx"}
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}
	want := app.NotebookDeletion{WorkspaceID: f.acme, NotebookIDs: []uuid.UUID{f.notebook.ID}, By: bob, At: now()}
	for _, s := range []*fakeSubscriber{first, second} {
		if len(s.got) != 1 || s.got[0].WorkspaceID != want.WorkspaceID || !slices.Equal(s.got[0].NotebookIDs, want.NotebookIDs) ||
			s.got[0].By != want.By || !s.got[0].At.Equal(want.At) {
			t.Errorf("%s got %+v, want %+v", s.name, s.got, want)
		}
	}
	if !strings.Contains(f.logs.String(), "notebook deleted") {
		t.Errorf("logs = %q, want the deletion", f.logs.String())
	}
}

// A subscriber's error rolls the deletion back, and stops the ones after
// it.
func TestDeleteNotebookFailsWithASubscriber(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionDelete, shared.WorkspaceMember, shared.NotebookAdmin)
	failure := errors.New("disk full")
	first, second := &fakeSubscriber{recorder: f.rec, name: "first", err: failure}, &fakeSubscriber{recorder: f.rec, name: "second"}

	if err := f.delete(first, second).Execute(as(uuid.NewV7()), f.notebook.ID); !errors.Is(err, failure) || !f.tx.rolledBack ||
		len(second.got) != 0 || f.logs.Len() != 0 {
		t.Errorf("Execute() = %v, rolled back %v, second %+v, logs %q; want the failure, rolled back, second not called, no log",
			err, f.tx.rolledBack, second.got, f.logs)
	}
}

func TestDeleteNotebookRefuses(t *testing.T) {
	forbidden := shared.Forbidden()
	for _, tt := range []struct {
		name string
		err  error
		want error
	}{
		{"no role in it", nil, domain.ErrNotFound},
		{"an editor", forbidden, forbidden},
	} {
		f := newFixture()
		f.auth.err = tt.err
		s := &fakeSubscriber{recorder: f.rec}
		if err := f.delete(s).Execute(as(uuid.NewV7()), f.notebook.ID); !errors.Is(err, tt.want) || len(s.got) != 0 ||
			slices.ContainsFunc(f.rec.calls, func(c string) bool { return strings.HasPrefix(c, "DeleteNotebook") }) {
			t.Errorf("%s: Execute() = %v, calls %q; want %v, nothing deleted", tt.name, err, f.rec.calls, tt.want)
		}
	}
}

// The registrant of the workspace's deletion: the workspace's notebooks at
// its time, and one call of the subscribers with every id, none without.
func TestWorkspaceDeletion(t *testing.T) {
	ws, by := uuid.NewV7(), uuid.NewV7()
	d := app.WorkspaceDeleted{WorkspaceID: ws, By: by, At: now()}
	for _, tt := range []struct {
		name  string
		ids   []uuid.UUID
		calls []string
	}{
		{"no notebook", nil, []string{"DeleteNotebooksOf by " + by.String() + " at 2026-10-02T10:00:00Z"}},
		{"two notebooks", []uuid.UUID{uuid.NewV7(), uuid.NewV7()},
			[]string{"DeleteNotebooksOf by " + by.String() + " at 2026-10-02T10:00:00Z", "NotebookDeleted s"}},
	} {
		f := newFixture()
		f.store.ofWorkspace = tt.ids
		s := &fakeSubscriber{recorder: f.rec, name: "s"}
		err := app.WorkspaceDeletion{Notebooks: f.store, Subscribers: []app.NotebookDeletionSubscriber{s}}.WorkspaceDeleted(as(by), d)
		if err != nil || !slices.Equal(f.rec.calls, tt.calls) {
			t.Errorf("%s: WorkspaceDeleted() = %v, calls %q; want %q", tt.name, err, f.rec.calls, tt.calls)
		}
		if tt.ids != nil && (len(s.got) != 1 || !slices.Equal(s.got[0].NotebookIDs, tt.ids) || s.got[0].WorkspaceID != ws ||
			s.got[0].By != by || !s.got[0].At.Equal(now())) {
			t.Errorf("%s: the subscriber got %+v, want the ids of %s by its deleter at its time", tt.name, s.got, ws)
		}
	}
}
