// Package httpadapter serves the workspace module's API: it implements the
// strict server that oapi-codegen generates from api/modules/workspace.yaml
// into the gen package, and translates between the generated types and the
// use cases.
package httpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// ListWorkspacesUseCase is app.ListWorkspaces.
type ListWorkspacesUseCase interface {
	Execute(ctx context.Context) ([]app.Membership, error)
}

// CreateWorkspaceUseCase is app.CreateWorkspace.
type CreateWorkspaceUseCase interface {
	Execute(ctx context.Context, name, slug string) (app.Membership, error)
}

// GetWorkspaceUseCase is app.GetWorkspace.
type GetWorkspaceUseCase interface {
	Execute(ctx context.Context, slug string) (app.Membership, error)
}

// CheckSlugUseCase is app.CheckSlug.
type CheckSlugUseCase interface {
	Execute(ctx context.Context, slug string) (string, error)
}

// UpdateWorkspaceUseCase is app.UpdateWorkspace.
type UpdateWorkspaceUseCase interface {
	Execute(ctx context.Context, slug, name string) (app.Membership, error)
}

// DeleteWorkspaceUseCase is app.DeleteWorkspace.
type DeleteWorkspaceUseCase interface {
	Execute(ctx context.Context, slug string) error
}

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	ListWorkspaces  ListWorkspacesUseCase
	CreateWorkspace CreateWorkspaceUseCase
	GetWorkspace    GetWorkspaceUseCase
	CheckSlug       CheckSlugUseCase
	UpdateWorkspace UpdateWorkspaceUseCase
	DeleteWorkspace DeleteWorkspaceUseCase
	ListMembers     ListMembersUseCase
	UpdateMember    UpdateMemberUseCase
	RemoveMember    RemoveMemberUseCase
	LeaveWorkspace  LeaveWorkspaceUseCase

	ListInvitations   ListInvitationsUseCase
	CreateInvitation  CreateInvitationUseCase
	DeleteInvitation  DeleteInvitationUseCase
	PreviewInvitation PreviewInvitationUseCase
	AcceptInvitation  AcceptInvitationUseCase
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

// ListWorkspaces serves GET /api/v0/workspaces.
func (h handler) ListWorkspaces(ctx context.Context, _ gen.ListWorkspacesRequestObject) (gen.ListWorkspacesResponseObject, error) {
	list, err := h.uc.ListWorkspaces.Execute(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Workspace, len(list))
	for i, m := range list {
		data[i] = workspaceOf(m)
	}
	return gen.ListWorkspaces200JSONResponse{Data: data}, nil
}

// CreateWorkspace serves POST /api/v0/workspaces.
func (h handler) CreateWorkspace(ctx context.Context, req gen.CreateWorkspaceRequestObject) (gen.CreateWorkspaceResponseObject, error) {
	m, err := h.uc.CreateWorkspace.Execute(ctx, req.Body.Name, req.Body.Slug)
	if err != nil {
		return nil, err
	}
	return gen.CreateWorkspace201JSONResponse(workspaceOf(m)), nil
}

// GetWorkspace serves GET /api/v0/workspaces/{slug}.
func (h handler) GetWorkspace(ctx context.Context, req gen.GetWorkspaceRequestObject) (gen.GetWorkspaceResponseObject, error) {
	m, err := h.uc.GetWorkspace.Execute(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	return gen.GetWorkspace200JSONResponse(workspaceOf(m)), nil
}

// UpdateWorkspace serves PATCH /api/v0/workspaces/{slug}.
func (h handler) UpdateWorkspace(ctx context.Context, req gen.UpdateWorkspaceRequestObject) (gen.UpdateWorkspaceResponseObject, error) {
	m, err := h.uc.UpdateWorkspace.Execute(ctx, req.Slug, req.Body.Name)
	if err != nil {
		return nil, err
	}
	return gen.UpdateWorkspace200JSONResponse(workspaceOf(m)), nil
}

// DeleteWorkspace serves DELETE /api/v0/workspaces/{slug}.
func (h handler) DeleteWorkspace(ctx context.Context, req gen.DeleteWorkspaceRequestObject) (gen.DeleteWorkspaceResponseObject, error) {
	if err := h.uc.DeleteWorkspace.Execute(ctx, req.Slug); err != nil {
		return nil, err
	}
	return gen.DeleteWorkspace204Response{}, nil
}

// CheckWorkspaceSlug serves GET /api/v0/workspace-slugs/{slug}.
func (h handler) CheckWorkspaceSlug(ctx context.Context, req gen.CheckWorkspaceSlugRequestObject) (gen.CheckWorkspaceSlugResponseObject, error) {
	reason, err := h.uc.CheckSlug.Execute(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	if reason == "" {
		return gen.CheckWorkspaceSlug200JSONResponse{Available: true}, nil
	}
	r := gen.SlugAvailabilityReason(reason)
	return gen.CheckWorkspaceSlug200JSONResponse{Available: false, Reason: &r}, nil
}

// workspaceOf is a workspace and the caller's role as the API shows them.
func workspaceOf(m app.Membership) gen.Workspace {
	w := m.Workspace
	return gen.Workspace{
		ID: w.ID, Slug: w.Slug, Name: w.Name, Role: gen.WorkspaceRole(m.Role),
		CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
}
