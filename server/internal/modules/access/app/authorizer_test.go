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

func TestAuthorizeReadsTheCallersRoleInTheTargetWorkspace(t *testing.T) {
	w1, w2 := uuid.NewV7(), uuid.NewV7()
	a, b := uuid.NewV7(), uuid.NewV7()
	memberships := &fakeMemberships{roles: map[membership]shared.WorkspaceRole{
		{w1, a}: shared.WorkspaceAdmin, {w2, a}: shared.WorkspaceGuest, {w1, b}: shared.WorkspaceMember,
	}}
	auth := app.NewAuthorizer(memberships)
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

func TestAuthorizeReadsOnEveryCall(t *testing.T) {
	w, a := uuid.NewV7(), uuid.NewV7()
	memberships := &fakeMemberships{roles: map[membership]shared.WorkspaceRole{{w, a}: shared.WorkspaceAdmin}}
	auth := app.NewAuthorizer(memberships)
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
	grant, err := app.NewAuthorizer(memberships).Authorize(context.Background(), shared.Actor{UserID: a}, "no.such.action", shared.Target{WorkspaceID: w})
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
	auth := app.NewAuthorizer(&fakeMemberships{err: failure})
	if _, err := auth.Authorize(context.Background(), shared.Actor{UserID: uuid.NewV7()}, "workspace.read",
		shared.Target{WorkspaceID: uuid.NewV7()}); !errors.Is(err, failure) {
		t.Errorf("Authorize() = %v, want %v", err, failure)
	}
}
