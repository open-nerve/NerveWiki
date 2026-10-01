package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// RemoveMemberDeps are what RemoveMember needs.
type RemoveMemberDeps struct {
	Locker WorkspaceLocker
	Finder MemberFinder
	Ender  MembershipEnder
	Auth   shared.Authorizer
	Tx     shared.TxManager
	Clock  Clock
	Logger *slog.Logger
}

// RemoveMember ends another account's membership:
// DELETE /api/v0/workspace-members/{workspace_member_id} (M2/P2 design 3.2).
type RemoveMember struct {
	d RemoveMemberDeps
}

// NewRemoveMember returns the use case.
func NewRemoveMember(d RemoveMemberDeps) *RemoveMember {
	return &RemoveMember{d: d}
}

// Execute ends the membership id through the extension point, under its
// workspace's row lock and the decision. An admin cannot remove their own
// membership, so the workspace keeps an admin.
func (r *RemoveMember) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	m, err := r.d.Finder.FindActiveMember(ctx, id)
	if err != nil {
		return found(err, domain.ErrMemberNotFound)
	}
	err = r.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if m, err = lockMember(ctx, r.d.Locker, r.d.Finder, m); err != nil {
			return err
		}
		if _, err := authorize(ctx, r.d.Auth, actor, domain.ActionRemoveMember, m.WorkspaceID, domain.ErrMemberNotFound); err != nil {
			return err
		}
		if m.UserID == actor.UserID {
			return domain.ErrOwnMembership
		}
		return r.d.Ender.End(ctx, MembershipEnd{
			UserID: m.UserID, WorkspaceIDs: []uuid.UUID{m.WorkspaceID}, Cause: EndRemoved, By: actor.UserID, At: r.d.Clock.Now(),
		})
	})
	if err != nil {
		return err
	}
	r.d.Logger.InfoContext(ctx, "workspace member removed", slog.String("workspace_id", m.WorkspaceID.String()),
		slog.String("member_id", m.ID.String()), slog.String("user_id", actor.UserID.String()))
	return nil
}
