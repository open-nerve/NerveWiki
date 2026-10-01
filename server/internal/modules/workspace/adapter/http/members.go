package httpadapter

import (
	"context"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
)

// ListMembersUseCase is app.ListMembers.
type ListMembersUseCase interface {
	Execute(ctx context.Context, slug string) ([]app.ListedMember, error)
}

// UpdateMemberUseCase is app.UpdateMember.
type UpdateMemberUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, role string) (app.ListedMember, error)
}

// RemoveMemberUseCase is app.RemoveMember.
type RemoveMemberUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) error
}

// LeaveWorkspaceUseCase is app.LeaveWorkspace.
type LeaveWorkspaceUseCase interface {
	Execute(ctx context.Context, slug string) error
}

// ListWorkspaceMembers serves GET /api/v0/workspaces/{slug}/members.
func (h handler) ListWorkspaceMembers(ctx context.Context, req gen.ListWorkspaceMembersRequestObject) (gen.ListWorkspaceMembersResponseObject, error) {
	list, err := h.uc.ListMembers.Execute(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	data := make([]gen.WorkspaceMember, len(list))
	for i, m := range list {
		data[i] = memberOf(m)
	}
	return gen.ListWorkspaceMembers200JSONResponse{Data: data}, nil
}

// UpdateWorkspaceMember serves PATCH /api/v0/workspace-members/{workspace_member_id}.
func (h handler) UpdateWorkspaceMember(ctx context.Context, req gen.UpdateWorkspaceMemberRequestObject) (gen.UpdateWorkspaceMemberResponseObject, error) {
	m, err := h.uc.UpdateMember.Execute(ctx, req.WorkspaceMemberID, string(req.Body.Role))
	if err != nil {
		return nil, err
	}
	return gen.UpdateWorkspaceMember200JSONResponse(memberOf(m)), nil
}

// RemoveWorkspaceMember serves DELETE /api/v0/workspace-members/{workspace_member_id}.
func (h handler) RemoveWorkspaceMember(ctx context.Context, req gen.RemoveWorkspaceMemberRequestObject) (gen.RemoveWorkspaceMemberResponseObject, error) {
	if err := h.uc.RemoveMember.Execute(ctx, req.WorkspaceMemberID); err != nil {
		return nil, err
	}
	return gen.RemoveWorkspaceMember204Response{}, nil
}

// LeaveWorkspace serves POST /api/v0/workspaces/{slug}/leave.
func (h handler) LeaveWorkspace(ctx context.Context, req gen.LeaveWorkspaceRequestObject) (gen.LeaveWorkspaceResponseObject, error) {
	if err := h.uc.LeaveWorkspace.Execute(ctx, req.Slug); err != nil {
		return nil, err
	}
	return gen.LeaveWorkspace204Response{}, nil
}

// memberOf is a member as the API shows it. The email is a required field
// that may be null: the zero Nullable would marshal as "", so a hidden one
// is set to null explicitly.
func memberOf(m app.ListedMember) gen.WorkspaceMember {
	email := nullable.NewNullNullable[string]()
	if m.Email != nil {
		email = nullable.NewNullableWithValue(*m.Email)
	}
	return gen.WorkspaceMember{
		ID: m.ID, UserID: m.UserID, Role: gen.WorkspaceRole(m.Role), DisplayName: m.DisplayName, Email: email, CreatedAt: m.CreatedAt,
	}
}
