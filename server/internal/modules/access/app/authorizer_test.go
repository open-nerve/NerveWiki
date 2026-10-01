package app_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/access/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type ctxKey struct{}

type membership struct{ workspace, user uuid.UUID }

// fakeMemberships answers from roles, and records each call with the value
// its context carried under ctxKey.
type fakeMemberships struct {
	roles map[membership]shared.WorkspaceRole
	err   error
	calls []string
}

func (f *fakeMemberships) RoleOf(ctx context.Context, workspaceID, userID uuid.UUID) (shared.WorkspaceRole, bool, error) {
	value, _ := ctx.Value(ctxKey{}).(string)
	f.calls = append(f.calls, workspaceID.String()+" "+userID.String()+" "+value)
	if f.err != nil {
		return "", false, f.err
	}
	role, ok := f.roles[membership{workspaceID, userID}]
	return role, ok, nil
}

// fakeNotebooks answers from facts, and records each call as fakeMemberships
// does.
type fakeNotebooks struct {
	facts map[membership]app.NotebookFact // keyed by notebook and user
	err   error
	calls []string
}

func (f *fakeNotebooks) NotebookFacts(ctx context.Context, notebookID, userID uuid.UUID) (app.NotebookFact, error) {
	value, _ := ctx.Value(ctxKey{}).(string)
	f.calls = append(f.calls, notebookID.String()+" "+userID.String()+" "+value)
	return f.facts[membership{notebookID, userID}], f.err
}

func TestAuthorizeReadsTheCallersRoleInTheTargetWorkspace(t *testing.T) {
	w1, w2 := uuid.NewV7(), uuid.NewV7()
	a, b := uuid.NewV7(), uuid.NewV7()
	memberships := &fakeMemberships{roles: map[membership]shared.WorkspaceRole{
		{w1, a}: shared.WorkspaceAdmin, {w2, a}: shared.WorkspaceGuest, {w1, b}: shared.WorkspaceMember,
	}}
	auth := app.NewAuthorizer(memberships, &fakeNotebooks{})
	tests := []struct {
		user, workspace uuid.UUID
		want            shared.WorkspaceRole // "": not visible
	}{
		{a, w1, shared.WorkspaceAdmin},
		{a, w2, shared.WorkspaceGuest},
		{b, w1, shared.WorkspaceMember},
		{b, w2, ""},
	}
	for _, tt := range tests {
		memberships.calls = nil
		ctx := context.WithValue(context.Background(), ctxKey{}, "tx")
		grant, err := auth.Authorize(ctx, shared.Actor{UserID: tt.user}, "workspace.read", shared.Target{WorkspaceID: tt.workspace})
		want := tt.workspace.String() + " " + tt.user.String() + " tx"
		if len(memberships.calls) != 1 || memberships.calls[0] != want {
			t.Errorf("RoleOf calls = %q, want [%q]", memberships.calls, want)
		}
		if tt.want == "" {
			if !errors.Is(err, shared.ErrNotVisible) {
				t.Errorf("user %s in %s: Authorize() = %+v, %v; want ErrNotVisible", tt.user, tt.workspace, grant, err)
			}
			continue
		}
		if err != nil || grant != (shared.Grant{WorkspaceRole: tt.want}) {
			t.Errorf("user %s in %s: Authorize() = %+v, %v; want %s", tt.user, tt.workspace, grant, err, tt.want)
		}
	}
}

func TestAuthorizeReadsTheNotebookAndTheWorkspaceAtTheNotebookLevel(t *testing.T) {
	w1, w2 := uuid.NewV7(), uuid.NewV7()
	n1, n2 := uuid.NewV7(), uuid.NewV7()
	a, b := uuid.NewV7(), uuid.NewV7()
	memberships := &fakeMemberships{roles: map[membership]shared.WorkspaceRole{
		{w1, a}: shared.WorkspaceMember, {w2, a}: shared.WorkspaceMember, {w1, b}: shared.WorkspaceGuest,
	}}
	notebooks := &fakeNotebooks{facts: map[membership]app.NotebookFact{
		{n1, a}: {Found: true, WorkspaceID: w1, Access: shared.AccessViewer},
		{n1, b}: {Found: true, WorkspaceID: w1, Access: shared.AccessViewer, Role: shared.NotebookAdmin},
		{n2, a}: {Found: true, WorkspaceID: w2, Access: shared.AccessEditor, Role: shared.NotebookAdmin},
	}}
	auth := app.NewAuthorizer(memberships, notebooks)
	tests := []struct {
		name                      string
		user, workspace, notebook uuid.UUID
		want                      shared.NotebookRole // "": not visible
	}{
		{"a member by the access", a, w1, n1, shared.NotebookReader},
		{"a guest, its admin", b, w1, n1, shared.NotebookAdmin},
		{"its admin", a, w2, n2, shared.NotebookAdmin},
		// The path names another workspace than the notebook's.
		{"its admin, through another workspace", a, w1, n2, ""},
		{"not found", b, w1, n2, ""},
	}
	for _, tt := range tests {
		memberships.calls, notebooks.calls = nil, nil
		ctx := context.WithValue(context.Background(), ctxKey{}, "tx")
		grant, err := auth.Authorize(ctx, shared.Actor{UserID: tt.user}, "notebook.read",
			shared.Target{WorkspaceID: tt.workspace, NotebookID: tt.notebook})
		if want := tt.notebook.String() + " " + tt.user.String() + " tx"; len(notebooks.calls) != 1 || notebooks.calls[0] != want {
			t.Errorf("%s: NotebookFacts calls = %q, want [%q]", tt.name, notebooks.calls, want)
		}
		if want := tt.workspace.String() + " " + tt.user.String() + " tx"; len(memberships.calls) != 1 || memberships.calls[0] != want {
			t.Errorf("%s: RoleOf calls = %q, want [%q]", tt.name, memberships.calls, want)
		}
		if tt.want == "" {
			if !errors.Is(err, shared.ErrNotVisible) {
				t.Errorf("%s: Authorize() = %+v, %v; want ErrNotVisible", tt.name, grant, err)
			}
			continue
		}
		role := memberships.roles[membership{tt.workspace, tt.user}]
		if err != nil || grant != (shared.Grant{WorkspaceRole: role, NotebookRole: tt.want}) {
			t.Errorf("%s: Authorize() = %+v, %v; want %s", tt.name, grant, err, tt.want)
		}
	}
}

func TestAuthorizeReadsNoNotebookAtTheWorkspaceLevel(t *testing.T) {
	w, a := uuid.NewV7(), uuid.NewV7()
	notebooks := &fakeNotebooks{}
	auth := app.NewAuthorizer(&fakeMemberships{roles: map[membership]shared.WorkspaceRole{{w, a}: shared.WorkspaceMember}}, notebooks)
	if _, err := auth.Authorize(context.Background(), shared.Actor{UserID: a}, "notebook.create",
		shared.Target{WorkspaceID: w, NotebookID: uuid.NewV7()}); err != nil {
		t.Fatalf("Authorize() = %v", err)
	}
	if len(notebooks.calls) != 0 {
		t.Errorf("NotebookFacts calls = %q, want none", notebooks.calls)
	}
}

func TestAuthorizeReadsOnEveryCall(t *testing.T) {
	w, a := uuid.NewV7(), uuid.NewV7()
	memberships := &fakeMemberships{roles: map[membership]shared.WorkspaceRole{{w, a}: shared.WorkspaceAdmin}}
	auth := app.NewAuthorizer(memberships, &fakeNotebooks{})
	target := shared.Target{WorkspaceID: w}
	if _, err := auth.Authorize(context.Background(), shared.Actor{UserID: a}, "workspace.read", target); err != nil {
		t.Fatalf("first Authorize() = %v", err)
	}
	delete(memberships.roles, membership{w, a})
	if _, err := auth.Authorize(context.Background(), shared.Actor{UserID: a}, "workspace.read", target); !errors.Is(err, shared.ErrNotVisible) {
		t.Errorf("Authorize() after the membership ended = %v, want ErrNotVisible", err)
	}
}

func TestAuthorizeRefusesAnActionWithoutARule(t *testing.T) {
	w, a := uuid.NewV7(), uuid.NewV7()
	memberships := &fakeMemberships{roles: map[membership]shared.WorkspaceRole{{w, a}: shared.WorkspaceAdmin}}
	grant, err := app.NewAuthorizer(memberships, &fakeNotebooks{}).Authorize(context.Background(), shared.Actor{UserID: a}, "no.such.action", shared.Target{WorkspaceID: w})
	var se *shared.Error
	if err == nil || errors.As(err, &se) || grant != (shared.Grant{}) {
		t.Errorf("Authorize() = %+v, %v; want an internal error", grant, err)
	}
	if len(memberships.calls) != 0 {
		t.Errorf("RoleOf calls = %q, want none", memberships.calls)
	}
}

func TestAuthorizeReturnsThePortsError(t *testing.T) {
	failure := errors.New("connection reset")
	for _, tt := range []struct {
		name        string
		memberships *fakeMemberships
		notebooks   *fakeNotebooks
		action      shared.Action
	}{
		{"the memberships", &fakeMemberships{err: failure}, &fakeNotebooks{}, "workspace.read"},
		{"the memberships, at the notebook level", &fakeMemberships{err: failure}, &fakeNotebooks{}, "notebook.read"},
		{"the notebooks", &fakeMemberships{}, &fakeNotebooks{err: failure}, "notebook.read"},
	} {
		auth := app.NewAuthorizer(tt.memberships, tt.notebooks)
		grant, err := auth.Authorize(context.Background(), shared.Actor{UserID: uuid.NewV7()}, tt.action,
			shared.Target{WorkspaceID: uuid.NewV7(), NotebookID: uuid.NewV7()})
		if !errors.Is(err, failure) || grant != (shared.Grant{}) {
			t.Errorf("%s: Authorize() = %+v, %v; want %v", tt.name, grant, err, failure)
		}
	}
}
