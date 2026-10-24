package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func (f fixture) create(seeders ...app.Seeder) *app.CreateNotebook {
	return app.NewCreateNotebook(app.CreateNotebookDeps{
		Workspaces: f.workspaces, Notebooks: f.store, Seeders: seeders, Subscribers: []app.VisibilitySubscriber{f.visibility},
		Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
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
	for _, tt := range []struct {
		mode   *string
		locale *string
		want   domain.Mode
	}{{nil, nil, domain.ModeStandard}, {ptr("llm_wiki"), ptr("zh-CN"), domain.ModeLLMWiki}} {
		f := newFixture()
		alice := uuid.NewV7()
		f.grant(domain.ActionCreate, shared.WorkspaceMember, "")
		seeder := &fakeSeeder{recorder: f.rec, name: "template", root: itsRoot}

		got, err := f.create(seeder).Execute(as(alice), "acme", app.NewNotebook{Name: "  Notes ", Access: ptr("viewer"), Mode: tt.mode,
			Locale: tt.locale, Client: "api"})

		if err != nil || len(f.store.created) != 1 || len(f.store.members) != 1 {
			t.Fatalf("%s: Execute() = %+v, %v; created %+v, members %+v", tt.want, got, err, f.store.created, f.store.members)
		}
		n, m := f.store.created[0], f.store.members[0]
		want := domain.Notebook{ID: n.ID, WorkspaceID: f.acme, Name: "Notes", Access: shared.AccessViewer, CreatedAt: now(), UpdatedAt: now(),
			Mode: tt.want}
		if tt.want == domain.ModeLLMWiki {
			// The source root's id is decided after the notebook's.
			want.SourceRootID = n.SourceRootID
			if n.SourceRootID == (uuid.UUID{}) || n.SourceRootID.Compare(n.ID) <= 0 {
				t.Errorf("%s: the source root %s, want an id after the notebook's %s", tt.want, n.SourceRootID, n.ID)
			}
		}
		if n != want || n.ID == uuid.Nil() {
			t.Errorf("%s: created %+v, want %+v", tt.want, n, want)
		}
		if want := (domain.Member{ID: m.ID, NotebookID: n.ID, UserID: alice, Role: shared.NotebookAdmin, CreatedAt: now()}); m != want || m.ID == uuid.Nil() {
			t.Errorf("%s: member %+v, want %+v", tt.want, m, want)
		}
		if want := (app.View{Notebook: n, Role: shared.NotebookAdmin, MemberCount: 1}); got != want {
			t.Errorf("%s: Execute() = %+v, want %+v", tt.want, got, want)
		}
		// The seeder gets the creation as it is, at the clock's one read.
		locale := domain.LocaleEn
		if tt.locale != nil {
			locale = domain.Locale(*tt.locale)
		}
		creation := app.NotebookCreation{NotebookID: n.ID, WorkspaceID: f.acme, Mode: tt.want, SourceRootID: n.SourceRootID, Locale: locale,
			By: alice, Client: "api", At: now()}
		if len(seeder.got) != 1 || seeder.got[0] != creation {
			t.Errorf("%s: the seeder got %+v, want %+v", tt.want, seeder.got, creation)
		}
		// The workspace's share, the decision, the writes, the seeders after
		// the admin, then the visibility, all in the transaction.
		calls := []string{"FindBySlug acme", "ShareByID in tx", "Authorize notebook.create in tx",
			"CreateNotebook by " + alice.String() + " in tx", "AddMember admin by " + alice.String() + " in tx",
			"Seed template " + string(tt.want) + " in tx", "VisibilityChanged visibility in tx"}
		if !slices.Equal(f.rec.calls, calls) {
			t.Errorf("%s: calls = %q, want %q", tt.want, f.rec.calls, calls)
		}
		logs := f.logs.String()
		if !strings.Contains(logs, "notebook created") || !strings.Contains(logs, n.ID.String()) || !strings.Contains(logs, "mode="+string(tt.want)) ||
			strings.Contains(logs, "Notes") {
			t.Errorf("%s: logs = %q, want the creation by its ids and its mode, not its name", tt.want, logs)
		}
	}
}

// Two seeders (v0.1 design 13.1, item 21): both follow an LLM Wiki's
// creation, in the order registered; the first error stops the dispatch
// and is the use case's, whose transaction rolls back. A seeder's problem
// is a defect, its code in the error's text and no problem left in it;
// ctx's cancellation and deadline pass as they are.
func TestTheSeedersRunInTheirOrder(t *testing.T) {
	failure := errors.New("the seeder failed")
	protected := shared.NewError(shared.KindConflict, "page.protected", "The guard refused it.")
	deadline := fmt.Errorf("the unit: %w", context.DeadlineExceeded)
	for _, tt := range []struct {
		name    string
		failing string
		err     error
		defect  bool
		told    []string
	}{
		{"both follow", "", nil, false, []string{"Seed a llm_wiki in tx", "Seed b llm_wiki in tx"}},
		{"the first fails", "a", failure, false, []string{"Seed a llm_wiki in tx"}},
		{"the second fails", "b", failure, false, []string{"Seed a llm_wiki in tx", "Seed b llm_wiki in tx"}},
		{"the first refuses", "a", protected, true, []string{"Seed a llm_wiki in tx"}},
		{"the first is cancelled", "a", context.Canceled, false, []string{"Seed a llm_wiki in tx"}},
		{"the first's deadline passes", "a", deadline, false, []string{"Seed a llm_wiki in tx"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionCreate, shared.WorkspaceMember, "")
			a, b := &fakeSeeder{recorder: f.rec, name: "a", root: itsRoot}, &fakeSeeder{recorder: f.rec, name: "b"}
			for _, s := range []*fakeSeeder{a, b} {
				if s.name == tt.failing {
					s.err = tt.err
				}
			}

			_, err := f.create(a, b).Execute(as(f.alice), "acme", app.NewNotebook{Name: "Notes", Mode: ptr("llm_wiki")})

			got := slices.DeleteFunc(slices.Clone(f.rec.calls), func(c string) bool { return !strings.HasPrefix(c, "Seed ") })
			if !slices.Equal(got, tt.told) || f.tx.rolledBack != (tt.failing != "") {
				t.Errorf("seeded %q, rolled back %v; want %q", got, f.tx.rolledBack, tt.told)
			}
			var problem *shared.Error
			switch {
			case tt.err == nil && err != nil:
				t.Errorf("Execute() = %v, want nil", err)
			case tt.defect && (errors.As(err, &problem) || err == nil || !strings.Contains(err.Error(), "page.protected")):
				t.Errorf("Execute() = %v; want a defect naming page.protected, no problem in it", err)
			case tt.err != nil && !tt.defect && (!errors.Is(err, tt.err) || strings.Contains(err.Error(), "defect")):
				t.Errorf("Execute() = %v; want %v as it is", err, tt.err)
			}
		})
	}
}

func TestCreateNotebookWithoutAnAccessIsPrivate(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate, shared.WorkspaceAdmin, "")
	if got, err := f.create().Execute(as(uuid.NewV7()), "acme", app.NewNotebook{Name: "Notes"}); err != nil || got.Notebook.Access != shared.AccessNone {
		t.Errorf("Execute() = %+v, %v; want none", got, err)
	}
}

// The codes come 404, 403, 422 (v0.1 design 13.1, item 4): the values, the
// mode and the template's language among them, are checked under the lock
// and after the decision, and nothing is written when any refuses.
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
		got, err := f.create().Execute(as(uuid.NewV7()), tt.slug, app.NewNotebook{Name: tt.value, Mode: ptr("LLM_WIKI"), Locale: ptr("fr")})
		if !errors.Is(err, tt.want) || got != (app.View{}) || !slices.Equal(f.rec.calls, tt.calls) {
			t.Errorf("%s: Execute() = %+v, %v, calls %q; want %v after %q", tt.name, got, err, f.rec.calls, tt.want, tt.calls)
		}
	}
	f := newFixture()
	f.grant(domain.ActionCreate, shared.WorkspaceMember, "")
	_, err := f.create().Execute(as(uuid.NewV7()), "acme", app.NewNotebook{Name: "CON", Access: ptr("public"), Mode: ptr("wiki"), Locale: ptr("fr")})
	var se *shared.Error
	if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || len(se.Fields) != 4 || len(f.store.created) != 0 || !f.tx.rolledBack {
		t.Errorf("a member's bad values = %v; want the four problems, nothing written", err)
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
	logs := f.logs.String()
	if !strings.Contains(logs, "notebook updated") || !strings.Contains(logs, f.acme.String()) ||
		!strings.Contains(logs, f.notebook.ID.String()) || strings.Contains(logs, "Eng") {
		t.Errorf("logs = %q, want the update by its ids", logs)
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

// The registrant of the workspace's deletion: the workspace's notebooks and
// audit events at its time, the events even with no notebook, and one call
// of the subscribers with every id, none without.
func TestWorkspaceDeletion(t *testing.T) {
	ws, by := uuid.NewV7(), uuid.NewV7()
	d := app.WorkspaceDeleted{WorkspaceID: ws, By: by, At: now()}
	deleted := []string{"DeleteNotebooksOf by " + by.String() + " at 2026-10-02T10:00:00Z", "DeleteAuditEventsOf by " + by.String() + " at 2026-10-02T10:00:00Z"}
	for _, tt := range []struct {
		name  string
		ids   []uuid.UUID
		calls []string
	}{
		{"no notebook", nil, deleted},
		{"two notebooks", []uuid.UUID{uuid.NewV7(), uuid.NewV7()}, append(slices.Clone(deleted), "NotebookDeleted s")},
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
