package app_test

import (
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Each notebook listed has the caller's effective role: the higher of the
// explicit role and the one the access gives a member of the workspace; an
// explicit reader of a notebook open to edit is its editor (M3 Codex
// review R5).
func TestListNotebooksGivesEachTheCallersRole(t *testing.T) {
	f := newFixture()
	alice := uuid.NewV7()
	open := f.notebook
	open.Access = shared.AccessEditor
	f.store.listed = []app.Listed{
		{Notebook: f.notebook, Explicit: shared.NotebookReader, MemberCount: 2},
		{Notebook: open, MemberCount: 1},
		{Notebook: open, Explicit: shared.NotebookAdmin, MemberCount: 1},
		{Notebook: open, Explicit: shared.NotebookReader, MemberCount: 2},
	}
	f.grant(domain.ActionList, shared.WorkspaceMember, "")
	got, err := app.NewListNotebooks(f.workspaces, f.store, f.auth).Execute(as(alice), "acme")

	want := []app.View{
		{Notebook: f.notebook, Role: shared.NotebookReader, MemberCount: 2},
		{Notebook: open, Role: shared.NotebookEditor, MemberCount: 1},
		{Notebook: open, Role: shared.NotebookAdmin, MemberCount: 1},
		{Notebook: open, Role: shared.NotebookEditor, MemberCount: 2},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
	}
	if calls := []string{"FindBySlug acme", "Authorize notebook.list", "ListNotebooks reached"}; !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q: a read opens no transaction", f.rec.calls, calls)
	}
	if f.auth.targets[0] != (shared.Target{WorkspaceID: f.acme}) {
		t.Errorf("target = %+v, want acme", f.auth.targets[0])
	}
}

// A guest has no role by the access: an explicit reader of a notebook open
// to edit stays its reader.
func TestListNotebooksGivesAGuestItsExplicitRole(t *testing.T) {
	f := newFixture()
	open := f.notebook
	open.Access = shared.AccessEditor
	f.store.listed = []app.Listed{{Notebook: open, Explicit: shared.NotebookReader, MemberCount: 2}}
	f.grant(domain.ActionList, shared.WorkspaceGuest, "")
	got, err := app.NewListNotebooks(f.workspaces, f.store, f.auth).Execute(as(uuid.NewV7()), "acme")

	if want := []app.View{{Notebook: open, Role: shared.NotebookReader, MemberCount: 2}}; err != nil || !slices.Equal(got, want) {
		t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
	}
}

// The workspace access reaches its admins and members, not its guests: the
// query asks only for their memberships then.
func TestListNotebooksAsksForTheAccessByTheWorkspaceRole(t *testing.T) {
	for _, tt := range []struct {
		role shared.WorkspaceRole
		call string
	}{
		{shared.WorkspaceAdmin, "ListNotebooks reached"},
		{shared.WorkspaceMember, "ListNotebooks reached"},
		{shared.WorkspaceGuest, "ListNotebooks"},
	} {
		f := newFixture()
		f.grant(domain.ActionList, tt.role, "")
		if _, err := app.NewListNotebooks(f.workspaces, f.store, f.auth).Execute(as(uuid.NewV7()), "acme"); err != nil ||
			f.rec.calls[len(f.rec.calls)-1] != tt.call {
			t.Errorf("%s: calls = %q, %v; want %s last", tt.role, f.rec.calls, err, tt.call)
		}
	}
}

func TestListNotebooksOfAWorkspaceNotSeen(t *testing.T) {
	for _, tt := range []struct {
		name, slug string
		calls      []string
	}{
		{"no workspace", "nope", []string{"FindBySlug nope"}},
		{"not a member", "acme", []string{"FindBySlug acme", "Authorize notebook.list"}},
	} {
		f := newFixture()
		got, err := app.NewListNotebooks(f.workspaces, f.store, f.auth).Execute(as(uuid.NewV7()), tt.slug)
		if !errors.Is(err, domain.ErrWorkspaceNotFound) || got != nil || !slices.Equal(f.rec.calls, tt.calls) {
			t.Errorf("%s: Execute() = %+v, %v, calls %q; want workspace.not_found after %q", tt.name, got, err, f.rec.calls, tt.calls)
		}
	}
}

func TestGetNotebook(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead, shared.WorkspaceGuest, shared.NotebookEditor)
	got, err := app.NewGetNotebook(f.store, f.auth).Execute(as(uuid.NewV7()), f.notebook.ID)

	if want := (app.View{Notebook: f.notebook, Role: shared.NotebookEditor, MemberCount: 3}); err != nil || got != want {
		t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
	}
	if want := (shared.Target{WorkspaceID: f.acme, NotebookID: f.notebook.ID}); f.auth.targets[0] != want {
		t.Errorf("target = %+v, want %+v: the notebook's workspace", f.auth.targets[0], want)
	}
	if calls := []string{"FindNotebook", "Authorize notebook.read", "CountMembers"}; !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}
}

func TestGetNotebookNotSeen(t *testing.T) {
	for _, tt := range []struct {
		name string
		id   func(f fixture) uuid.UUID
	}{
		{"no notebook", func(fixture) uuid.UUID { return uuid.NewV7() }},
		{"no role in it", func(f fixture) uuid.UUID { return f.notebook.ID }},
	} {
		f := newFixture()
		got, err := app.NewGetNotebook(f.store, f.auth).Execute(as(uuid.NewV7()), tt.id(f))
		if !errors.Is(err, domain.ErrNotFound) || got != (app.View{}) {
			t.Errorf("%s: Execute() = %+v, %v; want notebook.not_found", tt.name, got, err)
		}
	}
}
