package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestUpdateWorkspaceRenamesUnderTheLock(t *testing.T) {
	tm := newTeam()

	got, err := tm.update().Execute(tm.as(tm.alice), "acme", "  Acme Labs ")

	want := tm.acme
	want.Name, want.UpdatedAt = "Acme Labs", firstTick()
	if err != nil || got != (app.Membership{Workspace: want, Role: shared.WorkspaceAdmin}) {
		t.Errorf("Execute() = %+v, %v; want acme renamed, as its admin", got, err)
	}
	wantCalls := inTxCalls("LockWorkspaceBySlug acme", "Authorize workspace.update", "RenameWorkspace Acme Labs"+at(tm.alice))
	if !slices.Equal(tm.store.calls, wantCalls) {
		t.Errorf("calls = %q, want %q", tm.store.calls, wantCalls)
	}
	if !strings.Contains(tm.logs.String(), "workspace renamed") || strings.Contains(tm.logs.String(), "Acme Labs") {
		t.Errorf("logs = %s, want the rename without the name", tm.logs)
	}
}

// Each refusal comes before the write: 404 for a workspace the caller cannot
// see, whatever the name; 403, then 422, for one it can.
func TestUpdateWorkspaceRefusals(t *testing.T) {
	tests := []struct {
		name, slug, newName string
		caller              func(tm *team) context.Context
		want                error
		calls               []string
	}{
		{"a slug spelled as none", "\x00", "Acme", func(tm *team) context.Context { return tm.as(tm.alice) }, domain.ErrNotFound, nil},
		{"no such workspace", "nowhere", "Acme", func(tm *team) context.Context { return tm.as(tm.alice) }, domain.ErrNotFound,
			inTxCalls("LockWorkspaceBySlug nowhere")},
		{"not a member, a bad name", "acme", " ", func(tm *team) context.Context { return tm.asStranger() }, domain.ErrNotFound,
			inTxCalls("LockWorkspaceBySlug acme", "Authorize workspace.update")},
		{"a member", "acme", "Acme", func(tm *team) context.Context { return tm.as(tm.bob) }, shared.Forbidden(),
			inTxCalls("LockWorkspaceBySlug acme", "Authorize workspace.update")},
		{"the admin, a bad name", "acme", " ", func(tm *team) context.Context { return tm.as(tm.alice) }, shared.Invalid(),
			inTxCalls("LockWorkspaceBySlug acme", "Authorize workspace.update")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			ctx := tt.caller(tm)
			if errors.Is(tt.want, shared.Forbidden()) {
				tm.auth.err = shared.Forbidden()
			}

			_, err := tm.update().Execute(ctx, tt.slug, tt.newName)

			if !errors.Is(err, tt.want) || !slices.Equal(tm.store.calls, tt.calls) || tm.logs.Len() != 0 {
				t.Errorf("Execute() = %v after %q; want %v after %q, nothing logged", err, tm.store.calls, tt.want, tt.calls)
			}
		})
	}
}

// The members, then the workspace, at one time, then the subscribers with
// that time, all in the transaction.
func TestDeleteWorkspaceDeletesTheMembersThenTheWorkspace(t *testing.T) {
	tm := newTeam()

	err := tm.delete().Execute(tm.as(tm.alice), "acme")

	wantCalls := inTxCalls("LockWorkspaceBySlug acme", "Authorize workspace.delete",
		"DeleteMembersOf"+at(tm.alice), "DeleteWorkspace"+at(tm.alice), "WorkspaceDeleted")
	if err != nil || !slices.Equal(tm.store.calls, wantCalls) {
		t.Errorf("Execute() = %v after %q; want %q", err, tm.store.calls, wantCalls)
	}
	want := []app.WorkspaceDeletion{{WorkspaceID: tm.acme.ID, By: tm.alice.UserID, At: firstTick()}}
	if !slices.Equal(tm.sub.deleted, want) {
		t.Errorf("the subscriber saw %+v, want %+v", tm.sub.deleted, want)
	}
	if !strings.Contains(tm.logs.String(), "workspace deleted") {
		t.Errorf("logs = %s, want the deletion", tm.logs)
	}
}

func TestDeleteWorkspaceRefusals(t *testing.T) {
	tests := []struct {
		name, slug string
		caller     func(tm *team) context.Context
		want       error
		calls      []string
	}{
		{"a slug spelled as none", "\xff", func(tm *team) context.Context { return tm.as(tm.alice) }, domain.ErrNotFound, nil},
		{"no such workspace", "nowhere", func(tm *team) context.Context { return tm.as(tm.alice) }, domain.ErrNotFound,
			inTxCalls("LockWorkspaceBySlug nowhere")},
		{"not a member", "acme", func(tm *team) context.Context { return tm.asStranger() }, domain.ErrNotFound,
			inTxCalls("LockWorkspaceBySlug acme", "Authorize workspace.delete")},
		{"a guest", "acme", func(tm *team) context.Context { return tm.as(tm.carol) }, shared.Forbidden(),
			inTxCalls("LockWorkspaceBySlug acme", "Authorize workspace.delete")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			ctx := tt.caller(tm)
			if errors.Is(tt.want, shared.Forbidden()) {
				tm.auth.err = shared.Forbidden()
			}

			err := tm.delete().Execute(ctx, tt.slug)

			if !errors.Is(err, tt.want) || !slices.Equal(tm.store.calls, tt.calls) || len(tm.sub.deleted) != 0 {
				t.Errorf("Execute() = %v after %q; want %v after %q", err, tm.store.calls, tt.want, tt.calls)
			}
		})
	}
}

func TestDeleteWorkspaceFailsWithItsSubscriber(t *testing.T) {
	tm := newTeam()
	refused := errors.New("the subscriber failed")
	tm.sub.err = refused

	err := tm.delete().Execute(tm.as(tm.alice), "acme")

	if !errors.Is(err, refused) || !tm.tx.rolledBack || tm.logs.Len() != 0 {
		t.Errorf("Execute() = %v, rolled back %v, logs %s; want the subscriber's error, rolled back", err, tm.tx.rolledBack, tm.logs)
	}
}
