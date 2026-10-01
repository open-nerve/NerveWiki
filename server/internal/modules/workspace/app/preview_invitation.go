package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// PreviewInvitationDeps are what PreviewInvitation needs.
type PreviewInvitationDeps struct {
	Tokens      InvitationTokens
	Invitations InvitationFinder
	Workspaces  WorkspaceFinder
}

// InvitationPreview is what an invitation invites to, as anyone holding its
// link sees it: never the address.
type InvitationPreview struct {
	Workspace domain.Workspace
	Role      shared.WorkspaceRole
}

// PreviewInvitation shows what an invitation's link invites to, signed in
// or not: POST /api/v0/workspace-invitations/{workspace_invitation_id}/preview
// (M2/P3 design 3.3).
type PreviewInvitation struct {
	d PreviewInvitationDeps
}

// NewPreviewInvitation returns the use case.
func NewPreviewInvitation(d PreviewInvitationDeps) *PreviewInvitation {
	return &PreviewInvitation{d: d}
}

// Execute returns the workspace and the role of the invitation id. The
// token is checked before any read; a wrong one, an invitation no longer
// pending and a deleted workspace are domain.ErrInvitationNotFound alike.
func (p *PreviewInvitation) Execute(ctx context.Context, id uuid.UUID, token string) (InvitationPreview, error) {
	if !p.d.Tokens.Valid(id, token) {
		return InvitationPreview{}, domain.ErrInvitationNotFound
	}
	inv, err := p.d.Invitations.FindPendingInvitation(ctx, id)
	if err != nil {
		return InvitationPreview{}, found(err, domain.ErrInvitationNotFound)
	}
	w, err := p.d.Workspaces.FindWorkspaceByID(ctx, inv.WorkspaceID)
	if err != nil {
		return InvitationPreview{}, found(err, domain.ErrInvitationNotFound)
	}
	return InvitationPreview{Workspace: w, Role: inv.Role}, nil
}
