package httpadapter

import (
	"context"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
)

// ListMembersUseCase is app.ListMembers.
type ListMembersUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID) ([]app.ListedMember, error)
}

// AddMemberUseCase is app.AddMember.
type AddMemberUseCase interface {
	Execute(ctx context.Context, notebookID, userID uuid.UUID, role string) (app.ListedMember, error)
}

// UpdateMemberUseCase is app.UpdateMember.
type UpdateMemberUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, role string) (app.ListedMember, error)
}

// RemoveMemberUseCase is app.RemoveMember.
type RemoveMemberUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) error
}

// LeaveNotebookUseCase is app.LeaveNotebook.
type LeaveNotebookUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID) error
}

// ListNotebookMembers serves GET /api/v0/notebooks/{notebook_id}/members.
func (h handler) ListNotebookMembers(ctx context.Context, req gen.ListNotebookMembersRequestObject) (gen.ListNotebookMembersResponseObject, error) {
	list, err := h.uc.ListMembers.Execute(ctx, req.NotebookID)
	if err != nil {
		return nil, err
	}
	data := make([]gen.NotebookMember, len(list))
	for i, m := range list {
		data[i] = memberOf(m)
	}
	return gen.ListNotebookMembers200JSONResponse{Data: data}, nil
}

// AddNotebookMember serves POST /api/v0/notebooks/{notebook_id}/members.
func (h handler) AddNotebookMember(ctx context.Context, req gen.AddNotebookMemberRequestObject) (gen.AddNotebookMemberResponseObject, error) {
	m, err := h.uc.AddMember.Execute(ctx, req.NotebookID, req.Body.UserID, string(req.Body.Role))
	if err != nil {
		return nil, err
	}
	return gen.AddNotebookMember201JSONResponse(memberOf(m)), nil
}

// UpdateNotebookMember serves PATCH /api/v0/notebook-members/{notebook_member_id}.
func (h handler) UpdateNotebookMember(ctx context.Context, req gen.UpdateNotebookMemberRequestObject) (gen.UpdateNotebookMemberResponseObject, error) {
	m, err := h.uc.UpdateMember.Execute(ctx, req.NotebookMemberID, string(req.Body.Role))
	if err != nil {
		return nil, err
	}
	return gen.UpdateNotebookMember200JSONResponse(memberOf(m)), nil
}

// RemoveNotebookMember serves DELETE /api/v0/notebook-members/{notebook_member_id}.
func (h handler) RemoveNotebookMember(ctx context.Context, req gen.RemoveNotebookMemberRequestObject) (gen.RemoveNotebookMemberResponseObject, error) {
	if err := h.uc.RemoveMember.Execute(ctx, req.NotebookMemberID); err != nil {
		return nil, err
	}
	return gen.RemoveNotebookMember204Response{}, nil
}

// LeaveNotebook serves POST /api/v0/notebooks/{notebook_id}/leave.
func (h handler) LeaveNotebook(ctx context.Context, req gen.LeaveNotebookRequestObject) (gen.LeaveNotebookResponseObject, error) {
	if err := h.uc.LeaveNotebook.Execute(ctx, req.NotebookID); err != nil {
		return nil, err
	}
	return gen.LeaveNotebook204Response{}, nil
}

// memberOf is a membership as the API shows it: the email null for a
// caller who may not see it.
func memberOf(m app.ListedMember) gen.NotebookMember {
	email := nullable.NewNullNullable[string]()
	if m.Email != nil {
		email = nullable.NewNullableWithValue(*m.Email)
	}
	return gen.NotebookMember{
		ID: m.ID, UserID: m.UserID, Role: gen.NotebookRole(m.Role), DisplayName: m.DisplayName, Email: email, CreatedAt: m.CreatedAt,
	}
}
