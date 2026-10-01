package httpadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
)

// ListInvitationsUseCase is app.ListInvitations.
type ListInvitationsUseCase interface {
	Execute(ctx context.Context, slug string) ([]app.ListedInvitation, error)
}

// CreateInvitationUseCase is app.CreateInvitation.
type CreateInvitationUseCase interface {
	Execute(ctx context.Context, slug, email, role string) (app.ListedInvitation, error)
}

// DeleteInvitationUseCase is app.DeleteInvitation.
type DeleteInvitationUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) error
}

// PreviewInvitationUseCase is app.PreviewInvitation.
type PreviewInvitationUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, token string) (app.InvitationPreview, error)
}

// AcceptInvitationUseCase is app.AcceptInvitation.
type AcceptInvitationUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, token string) (app.Membership, error)
}

// ListWorkspaceInvitations serves GET /api/v0/workspaces/{slug}/invitations.
func (h handler) ListWorkspaceInvitations(ctx context.Context, req gen.ListWorkspaceInvitationsRequestObject) (gen.ListWorkspaceInvitationsResponseObject, error) {
	list, err := h.uc.ListInvitations.Execute(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	data := make([]gen.WorkspaceInvitation, len(list))
	for i, inv := range list {
		data[i] = invitationOf(inv)
	}
	return gen.ListWorkspaceInvitations200JSONResponse{Data: data}, nil
}

// CreateWorkspaceInvitation serves POST /api/v0/workspaces/{slug}/invitations.
func (h handler) CreateWorkspaceInvitation(ctx context.Context, req gen.CreateWorkspaceInvitationRequestObject) (gen.CreateWorkspaceInvitationResponseObject, error) {
	inv, err := h.uc.CreateInvitation.Execute(ctx, req.Slug, req.Body.Email, string(req.Body.Role))
	if err != nil {
		return nil, err
	}
	return gen.CreateWorkspaceInvitation201JSONResponse(invitationOf(inv)), nil
}

// DeleteWorkspaceInvitation serves DELETE /api/v0/workspace-invitations/{workspace_invitation_id}.
func (h handler) DeleteWorkspaceInvitation(ctx context.Context, req gen.DeleteWorkspaceInvitationRequestObject) (gen.DeleteWorkspaceInvitationResponseObject, error) {
	if err := h.uc.DeleteInvitation.Execute(ctx, req.WorkspaceInvitationID); err != nil {
		return nil, err
	}
	return gen.DeleteWorkspaceInvitation204Response{}, nil
}

// PreviewWorkspaceInvitation serves POST /api/v0/workspace-invitations/{workspace_invitation_id}/preview.
func (h handler) PreviewWorkspaceInvitation(ctx context.Context, req gen.PreviewWorkspaceInvitationRequestObject) (gen.PreviewWorkspaceInvitationResponseObject, error) {
	p, err := h.uc.PreviewInvitation.Execute(ctx, req.WorkspaceInvitationID, req.Body.Token)
	if err != nil {
		return nil, err
	}
	return gen.PreviewWorkspaceInvitation200JSONResponse{
		Workspace: gen.InvitedWorkspace{Name: p.Workspace.Name, Slug: p.Workspace.Slug}, Role: gen.WorkspaceRole(p.Role),
	}, nil
}

// AcceptWorkspaceInvitation serves POST /api/v0/workspace-invitations/{workspace_invitation_id}/accept.
func (h handler) AcceptWorkspaceInvitation(ctx context.Context, req gen.AcceptWorkspaceInvitationRequestObject) (gen.AcceptWorkspaceInvitationResponseObject, error) {
	m, err := h.uc.AcceptInvitation.Execute(ctx, req.WorkspaceInvitationID, req.Body.Token)
	if err != nil {
		return nil, err
	}
	return gen.AcceptWorkspaceInvitation200JSONResponse(workspaceOf(m)), nil
}

// invitationOf is a pending invitation as the API shows it.
func invitationOf(inv app.ListedInvitation) gen.WorkspaceInvitation {
	return gen.WorkspaceInvitation{
		ID: inv.ID, Email: inv.Email, Role: gen.WorkspaceRole(inv.Role), Token: inv.Token, CreatedAt: inv.CreatedAt,
	}
}

// PublicOperations are the module's routes that need no token, as the
// generated code registers them: the preview, which anyone holding an
// invitation's link may ask (M2/P3 design 3.7).
func PublicOperations() []string {
	return []string{"POST /api/v0/workspace-invitations/{workspace_invitation_id}/preview"}
}
