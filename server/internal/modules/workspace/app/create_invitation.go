package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CreateInvitationDeps are what CreateInvitation needs.
type CreateInvitationDeps struct {
	Sharer      WorkspaceSharer
	Accounts    AccountFinder
	Members     MemberFinder
	Invitations InvitationUpdater
	Tokens      InvitationTokens
	Auth        shared.Authorizer
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// CreateInvitation invites an address to a workspace:
// POST /api/v0/workspaces/{slug}/invitations (M2/P3 design 3.3).
type CreateInvitation struct {
	d CreateInvitationDeps
}

// NewCreateInvitation returns the use case.
func NewCreateInvitation(d CreateInvitationDeps) *CreateInvitation {
	return &CreateInvitation{d: d}
}

// Execute invites email to the workspace of slug with role. Under the
// workspace's share lock, which two admins inviting hold together and which
// holds a change of the memberships back, the decision comes first, then
// the values (422): an active member's address is not_allowed; an address
// with a pending invitation is a duplicate, as the unique index tells when
// two admins invite it at once.
func (c *CreateInvitation) Execute(ctx context.Context, slug, email, role string) (ListedInvitation, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return ListedInvitation{}, err
	}
	if !domain.ValidSlug(slug) {
		return ListedInvitation{}, domain.ErrNotFound
	}
	var inv domain.Invitation
	err = c.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		w, err := c.d.Sharer.ShareWorkspaceBySlug(ctx, slug)
		if err != nil {
			return found(err, domain.ErrNotFound)
		}
		if _, err := authorize(ctx, c.d.Auth, actor, domain.ActionCreateInvitation, w.ID, domain.ErrNotFound); err != nil {
			return err
		}
		draft, err := domain.CheckInvitation(email, role)
		if err != nil {
			return err
		}
		member, err := c.activeMember(ctx, w.ID, draft.Email)
		if err != nil {
			return err
		}
		if member {
			return domain.ErrAlreadyMember
		}
		inv = domain.Invitation{ID: uuid.NewV7(), WorkspaceID: w.ID, Email: draft.Email, Role: draft.Role, CreatedAt: c.d.Clock.Now()}
		return c.d.Invitations.CreateInvitation(ctx, inv, actor.UserID)
	})
	if err != nil {
		return ListedInvitation{}, err
	}
	c.d.Logger.InfoContext(ctx, "workspace invitation created", slog.String("workspace_id", inv.WorkspaceID.String()),
		slog.String("invitation_id", inv.ID.String()), slog.String("role", string(inv.Role)), slog.String("user_id", actor.UserID.String()))
	return listed(c.d.Tokens, inv), nil
}

// activeMember reports whether the account of email, if there is one, is
// an active member of workspaceID: an invitation is for those who are not.
func (c *CreateInvitation) activeMember(ctx context.Context, workspaceID uuid.UUID, email string) (bool, error) {
	userID, ok, err := c.d.Accounts.AccountIDByEmail(ctx, email)
	if err != nil || !ok {
		return false, err
	}
	_, active, err := c.d.Members.FindMembership(ctx, workspaceID, userID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return active, err
}
