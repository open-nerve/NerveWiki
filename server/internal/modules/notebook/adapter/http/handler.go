// Package httpadapter serves the notebook module's API: it implements the
// strict server that oapi-codegen generates from api/modules/notebook.yaml
// into the gen package, and translates between the generated types and the
// use cases.
package httpadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// ListNotebooksUseCase is app.ListNotebooks.
type ListNotebooksUseCase interface {
	Execute(ctx context.Context, slug string) ([]app.View, error)
}

// CreateNotebookUseCase is app.CreateNotebook.
type CreateNotebookUseCase interface {
	Execute(ctx context.Context, slug, name string, access *string) (app.View, error)
}

// GetNotebookUseCase is app.GetNotebook.
type GetNotebookUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (app.View, error)
}

// UpdateNotebookUseCase is app.UpdateNotebook.
type UpdateNotebookUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, name, access *string) (app.View, error)
}

// DeleteNotebookUseCase is app.DeleteNotebook.
type DeleteNotebookUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) error
}

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	ListNotebooks  ListNotebooksUseCase
	CreateNotebook CreateNotebookUseCase
	GetNotebook    GetNotebookUseCase
	UpdateNotebook UpdateNotebookUseCase
	DeleteNotebook DeleteNotebookUseCase
}

// Register mounts the module's routes on router, the root router from
// httpserver.NewRouter, behind the platform's per-route middlewares.
// Binding, decoding and handler errors are answered as problem+json by
// api.Errors.
func Register(router *httpserver.Router, api *httpserver.API, uc UseCases) {
	strict := gen.NewStrictHandlerWithOptions(handler{uc: uc}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  api.Errors.BodyError,
		ResponseErrorHandlerFunc: api.Errors.Write,
	})
	var middlewares []gen.MiddlewareFunc
	for _, m := range api.Middlewares(gen.BodyShapes()) {
		middlewares = append(middlewares, m)
	}
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseRouter:       router,
		Middlewares:      middlewares,
		ErrorHandlerFunc: api.Errors.BadRequest,
	})
}

// handler implements gen.StrictServerInterface: it only translates between
// the generated types and the use cases.
type handler struct {
	uc UseCases
}

// ListNotebooks serves GET /api/v0/workspaces/{slug}/notebooks.
func (h handler) ListNotebooks(ctx context.Context, req gen.ListNotebooksRequestObject) (gen.ListNotebooksResponseObject, error) {
	list, err := h.uc.ListNotebooks.Execute(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Notebook, len(list))
	for i, v := range list {
		data[i] = notebookOf(v)
	}
	return gen.ListNotebooks200JSONResponse{Data: data}, nil
}

// CreateNotebook serves POST /api/v0/workspaces/{slug}/notebooks.
func (h handler) CreateNotebook(ctx context.Context, req gen.CreateNotebookRequestObject) (gen.CreateNotebookResponseObject, error) {
	v, err := h.uc.CreateNotebook.Execute(ctx, req.Slug, req.Body.Name, (*string)(req.Body.WorkspaceAccess))
	if err != nil {
		return nil, err
	}
	return gen.CreateNotebook201JSONResponse(notebookOf(v)), nil
}

// GetNotebook serves GET /api/v0/notebooks/{notebook_id}.
func (h handler) GetNotebook(ctx context.Context, req gen.GetNotebookRequestObject) (gen.GetNotebookResponseObject, error) {
	v, err := h.uc.GetNotebook.Execute(ctx, req.NotebookID)
	if err != nil {
		return nil, err
	}
	return gen.GetNotebook200JSONResponse(notebookOf(v)), nil
}

// UpdateNotebook serves PATCH /api/v0/notebooks/{notebook_id}.
func (h handler) UpdateNotebook(ctx context.Context, req gen.UpdateNotebookRequestObject) (gen.UpdateNotebookResponseObject, error) {
	v, err := h.uc.UpdateNotebook.Execute(ctx, req.NotebookID, req.Body.Name, (*string)(req.Body.WorkspaceAccess))
	if err != nil {
		return nil, err
	}
	return gen.UpdateNotebook200JSONResponse(notebookOf(v)), nil
}

// DeleteNotebook serves DELETE /api/v0/notebooks/{notebook_id}.
func (h handler) DeleteNotebook(ctx context.Context, req gen.DeleteNotebookRequestObject) (gen.DeleteNotebookResponseObject, error) {
	if err := h.uc.DeleteNotebook.Execute(ctx, req.NotebookID); err != nil {
		return nil, err
	}
	return gen.DeleteNotebook204Response{}, nil
}

// notebookOf is a notebook and the caller's role as the API shows them.
func notebookOf(v app.View) gen.Notebook {
	n := v.Notebook
	return gen.Notebook{
		ID: n.ID, WorkspaceID: n.WorkspaceID, Name: n.Name, WorkspaceAccess: gen.WorkspaceAccess(n.Access),
		Role: gen.NotebookRole(v.Role), MemberCount: v.MemberCount, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
	}
}
