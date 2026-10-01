package app_test

import (
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestGetWorkspaceAnswersWithTheCallersRole(t *testing.T) {
	acme := domain.Workspace{ID: uuid.NewV7(), Slug: "acme", Name: "Acme", CreatedAt: now(), UpdatedAt: now()}
	store := &fakeStore{workspaces: map[string]domain.Workspace{"acme": acme}}
	auth := &fakeAuthorizer{roles: map[uuid.UUID]shared.WorkspaceRole{acme.ID: shared.WorkspaceGuest}}

	got, err := app.NewGetWorkspace(store, auth).Execute(as(uuid.NewV7()), "acme")

	if err != nil || got != (app.Membership{Workspace: acme, Role: shared.WorkspaceGuest}) {
		t.Errorf("Execute() = %+v, %v; want acme as a guest", got, err)
	}
	if !slices.Equal(auth.calls, []shared.Action{domain.ActionRead}) {
		t.Errorf("decisions asked = %q, want workspace.read", auth.calls)
	}
}

func TestGetWorkspaceAnswersNotFoundAlike(t *testing.T) {
	acme := domain.Workspace{ID: uuid.NewV7(), Slug: "acme", Name: "Acme"}
	for _, tt := range []struct {
		name, slug string
		decided    bool
	}{
		{"no such workspace", "nowhere", false},
		{"a spelling no slug has", "Acme", false},
		{"not a member", "acme", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{workspaces: map[string]domain.Workspace{"acme": acme}}
			auth := &fakeAuthorizer{roles: map[uuid.UUID]shared.WorkspaceRole{}}

			_, err := app.NewGetWorkspace(store, auth).Execute(as(uuid.NewV7()), tt.slug)

			if !errors.Is(err, domain.ErrNotFound) || (len(auth.calls) == 1) != tt.decided {
				t.Errorf("Execute(%q) = %v after %d decisions; want workspace.not_found", tt.slug, err, len(auth.calls))
			}
		})
	}
}

func TestGetWorkspacePassesAFaultOn(t *testing.T) {
	acme := domain.Workspace{ID: uuid.NewV7(), Slug: "acme"}
	fault := errors.New("connection reset")
	store := &fakeStore{workspaces: map[string]domain.Workspace{"acme": acme}}

	_, err := app.NewGetWorkspace(store, &fakeAuthorizer{err: fault}).Execute(as(uuid.NewV7()), "acme")

	if !errors.Is(err, fault) {
		t.Errorf("Execute() = %v, want the fault", err)
	}
}

func TestListWorkspacesIsTheCallersMemberships(t *testing.T) {
	alice := uuid.NewV7()
	list := []app.Membership{{Workspace: domain.Workspace{ID: uuid.NewV7(), Slug: "acme"}, Role: shared.WorkspaceMember}}
	store := &fakeStore{memberships: list}

	got, err := app.NewListWorkspaces(store).Execute(as(alice))

	if err != nil || !slices.Equal(got, list) || !slices.Equal(store.calls, []string{"ListWorkspacesOf " + alice.String()}) {
		t.Errorf("Execute() = %+v, %v after %q; want alice's memberships", got, err, store.calls)
	}
}

func TestCheckSlug(t *testing.T) {
	store := &fakeStore{workspaces: map[string]domain.Workspace{"acme": {}}}
	uc := app.NewCheckSlug(store)
	for slug, want := range map[string]string{
		"acme":      domain.SlugTaken,
		"acme-labs": "",
		"Acme":      domain.SlugInvalid,
		"settings":  domain.SlugReserved,
	} {
		store.calls = nil
		got, err := uc.Execute(as(uuid.NewV7()), slug)
		if err != nil || got != want {
			t.Errorf("Execute(%q) = %q, %v; want %q", slug, got, err, want)
		}
		// Only a slug that could be taken is looked up.
		if asked := len(store.calls) == 1; asked != (want == "" || want == domain.SlugTaken) {
			t.Errorf("Execute(%q) asked the store %q", slug, store.calls)
		}
	}
}
