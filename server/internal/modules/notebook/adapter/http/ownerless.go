package httpadapter

import (
	"context"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
)

// ListOwnerlessUseCase is app.ListOwnerlessNotebooks.
type ListOwnerlessUseCase interface {
	Execute(ctx context.Context, slug string) ([]app.OwnerlessNotebook, error)
}

// TakeOverUseCase is app.TakeOverNotebook.
type TakeOverUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (app.View, error)
}

// DeleteOwnerlessUseCase is app.DeleteOwnerlessNotebook.
type DeleteOwnerlessUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) error
}

// ListAuditEventsUseCase is app.ListNotebookAuditEvents.
type ListAuditEventsUseCase interface {
	Execute(ctx context.Context, slug string, limit *int, cursor *string) (app.AuditPage, error)
}

// ListOwnerlessNotebooks serves GET /api/v0/workspaces/{slug}/ownerless-notebooks.
func (h handler) ListOwnerlessNotebooks(ctx context.Context, req gen.ListOwnerlessNotebooksRequestObject,
) (gen.ListOwnerlessNotebooksResponseObject, error) {
	list, err := h.uc.ListOwnerless.Execute(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	data := make([]gen.OwnerlessNotebook, len(list))
	for i, o := range list {
		n := o.Notebook
		data[i] = gen.OwnerlessNotebook{
			ID: n.ID, Name: n.Name, WorkspaceAccess: gen.WorkspaceAccess(n.Access), MemberCount: o.MemberCount,
			FormerOwner: profileOf(n.Ownerless.FormerOwner, o.FormerOwner), OwnerlessSince: n.Ownerless.Since,
			LastActivityAt: o.LastActivityAt, SizeBytes: o.SizeBytes,
		}
	}
	return gen.ListOwnerlessNotebooks200JSONResponse{Data: data}, nil
}

// TakeOverNotebook serves POST /api/v0/ownerless-notebooks/{notebook_id}/take-over.
func (h handler) TakeOverNotebook(ctx context.Context, req gen.TakeOverNotebookRequestObject) (gen.TakeOverNotebookResponseObject, error) {
	v, err := h.uc.TakeOver.Execute(ctx, req.NotebookID)
	if err != nil {
		return nil, err
	}
	return gen.TakeOverNotebook200JSONResponse(notebookOf(v)), nil
}

// DeleteOwnerlessNotebook serves DELETE /api/v0/ownerless-notebooks/{notebook_id}.
func (h handler) DeleteOwnerlessNotebook(ctx context.Context, req gen.DeleteOwnerlessNotebookRequestObject,
) (gen.DeleteOwnerlessNotebookResponseObject, error) {
	if err := h.uc.DeleteOwnerless.Execute(ctx, req.NotebookID); err != nil {
		return nil, err
	}
	return gen.DeleteOwnerlessNotebook204Response{}, nil
}

// ListNotebookAuditEvents serves GET /api/v0/workspaces/{slug}/notebook-audit-events.
func (h handler) ListNotebookAuditEvents(ctx context.Context, req gen.ListNotebookAuditEventsRequestObject,
) (gen.ListNotebookAuditEventsResponseObject, error) {
	page, err := h.uc.ListAuditEvents.Execute(ctx, req.Slug, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	out := gen.ListNotebookAuditEvents200JSONResponse{Data: make([]gen.NotebookAuditEvent, len(page.Events)), NextCursor: nullable.NewNullNullable[string]()}
	if page.NextCursor != "" {
		out.NextCursor = nullable.NewNullableWithValue(page.NextCursor)
	}
	for i, l := range page.Events {
		e := l.Event
		out.Data[i] = gen.NotebookAuditEvent{
			ID: e.ID, Action: gen.NotebookAuditAction(e.Action), NotebookID: e.NotebookID, NotebookName: e.NotebookName,
			FormerOwner: profileOf(e.FormerOwnerID, l.FormerOwner), Actor: profileOf(e.ActorID, l.Actor), CreatedAt: e.At,
		}
	}
	return out, nil
}

// profileOf is account id's profile as the workspace's admins see it.
func profileOf(id uuid.UUID, p app.Profile) gen.AccountProfile {
	return gen.AccountProfile{UserID: id, DisplayName: p.DisplayName, Email: p.Email}
}
