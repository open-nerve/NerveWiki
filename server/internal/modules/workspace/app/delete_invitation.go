package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DeleteInvitationDeps are what DeleteInvitation needs.
type DeleteInvitationDeps struct {
	Finder      InvitationFinder
	Sharer      WorkspaceSharer
	Invitations InvitationUpdater
	Auth        shared.Authorizer
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// DeleteInvitation withdraws a pending invitation:
// DELETE /api/v0/workspace-invitations/{workspace_invitation_id} (M2/P3
// design 3.3).
type DeleteInvitation struct {
	d DeleteInvitationDeps
}

// NewDeleteInvitation returns the use case.
func NewDeleteInvitation(d DeleteInvitationDeps) *DeleteInvitation {
	return &DeleteInvitation{d: d}
}

// Execute deletes the invitation id softly. The invitation read first names
// its workspace; under the workspace's share lock the invitation is read
// again, locked, where an acceptance or a deletion committed meanwhile
// leaves none; then the decision.
func (d *DeleteInvitation) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	inv, err := d.d.Finder.FindPendingInvitation(ctx, id)
	if err != nil {
		return found(err, domain.ErrInvitationNotFound)
	}
	err = d.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := d.d.Sharer.ShareWorkspaceByID(ctx, inv.WorkspaceID); err != nil {
			return found(err, domain.ErrInvitationNotFound)
		}
		if inv, err = d.d.Invitations.LockPendingInvitation(ctx, id); err != nil {
			return found(err, domain.ErrInvitationNotFound)
		}
		if _, err := authorize(ctx, d.d.Auth, actor, domain.ActionDeleteInvitation, inv.WorkspaceID, domain.ErrInvitationNotFound); err != nil {
			return err
		}
		return d.d.Invitations.DeleteInvitation(ctx, id, actor.UserID, d.d.Clock.Now())
	})
	if err != nil {
		return err
	}
	d.d.Logger.InfoContext(ctx, "workspace invitation deleted", slog.String("workspace_id", inv.WorkspaceID.String()),
		slog.String("invitation_id", id.String()), slog.String("user_id", actor.UserID.String()))
	return nil
}
