// Package httpadapter serves the page module's API: it implements the
// strict server that oapi-codegen generates from api/modules/page.yaml into
// the gen package, and translates between the generated types and the use
// cases. It tells the writes where they came from (M4 design 4).
package httpadapter

import (
	"context"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListNodesUseCase is app.ListNodes.
type ListNodesUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID) ([]domain.Node, error)
}

// CreatePageUseCase is app.CreatePage.
type CreatePageUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, d app.PageDraft, client domain.Client) (app.PageView, error)
}

// GetPageUseCase is app.GetPage.
type GetPageUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (app.PageView, error)
}

// GetPageViewUseCase is app.GetPageView.
type GetPageViewUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (app.ReadingView, error)
}

// RenameNodeUseCase is app.RenameNode.
type RenameNodeUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, name string, client domain.Client) (domain.Node, error)
}

// MoveNodeUseCase is app.MoveNode.
type MoveNodeUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, to app.Destination, client domain.Client) (domain.Node, error)
}

// DeleteNodeUseCase is app.DeleteNode.
type DeleteNodeUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, client domain.Client) error
}

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	ListNodes   ListNodesUseCase
	CreatePage  CreatePageUseCase
	GetPage     GetPageUseCase
	GetPageView GetPageViewUseCase
	RenameNode  RenameNodeUseCase
	MoveNode    MoveNodeUseCase
	DeleteNode  DeleteNodeUseCase
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

// ListNodes serves GET /api/v0/notebooks/{notebook_id}/nodes.
func (h handler) ListNodes(ctx context.Context, req gen.ListNodesRequestObject) (gen.ListNodesResponseObject, error) {
	nodes, err := h.uc.ListNodes.Execute(ctx, req.NotebookID)
	if err != nil {
		return nil, err
	}
	data := make([]gen.TreeNode, len(nodes))
	for i, n := range nodes {
		data[i] = treeNodeOf(n)
	}
	return gen.ListNodes200JSONResponse{Data: data}, nil
}

// CreatePage serves POST /api/v0/notebooks/{notebook_id}/pages.
func (h handler) CreatePage(ctx context.Context, req gen.CreatePageRequestObject) (gen.CreatePageResponseObject, error) {
	client, err := clientOf(ctx)
	if err != nil {
		return nil, err
	}
	d := app.PageDraft{ParentID: idOf(req.Body.ParentID), Title: req.Body.Title, Position: positionOf(req.Body.AfterID)}
	v, err := h.uc.CreatePage.Execute(ctx, req.NotebookID, d, client)
	if err != nil {
		return nil, err
	}
	return gen.CreatePage201JSONResponse(pageOf(v)), nil
}

// GetPage serves GET /api/v0/pages/{page_id}.
func (h handler) GetPage(ctx context.Context, req gen.GetPageRequestObject) (gen.GetPageResponseObject, error) {
	v, err := h.uc.GetPage.Execute(ctx, req.PageID)
	if err != nil {
		return nil, err
	}
	return gen.GetPage200JSONResponse(pageOf(v)), nil
}

// GetPageView serves GET /api/v0/pages/{page_id}/view.
func (h handler) GetPageView(ctx context.Context, req gen.GetPageViewRequestObject) (gen.GetPageViewResponseObject, error) {
	v, err := h.uc.GetPageView.Execute(ctx, req.PageID)
	if err != nil {
		return nil, err
	}
	return gen.GetPageView200JSONResponse{HTML: v.HTML, Revision: v.Revision}, nil
}

// RenameNode serves PATCH /api/v0/nodes/{node_id}.
func (h handler) RenameNode(ctx context.Context, req gen.RenameNodeRequestObject) (gen.RenameNodeResponseObject, error) {
	client, err := clientOf(ctx)
	if err != nil {
		return nil, err
	}
	n, err := h.uc.RenameNode.Execute(ctx, req.NodeID, req.Body.Name, client)
	if err != nil {
		return nil, err
	}
	return gen.RenameNode200JSONResponse(treeNodeOf(n)), nil
}

// MoveNode serves POST /api/v0/nodes/{node_id}/move.
func (h handler) MoveNode(ctx context.Context, req gen.MoveNodeRequestObject) (gen.MoveNodeResponseObject, error) {
	client, err := clientOf(ctx)
	if err != nil {
		return nil, err
	}
	to := app.Destination{ParentID: idOf(req.Body.ParentID), Position: positionOf(req.Body.AfterID)}
	n, err := h.uc.MoveNode.Execute(ctx, req.NodeID, to, client)
	if err != nil {
		return nil, err
	}
	return gen.MoveNode200JSONResponse(treeNodeOf(n)), nil
}

// DeleteNode serves DELETE /api/v0/nodes/{node_id}.
func (h handler) DeleteNode(ctx context.Context, req gen.DeleteNodeRequestObject) (gen.DeleteNodeResponseObject, error) {
	client, err := clientOf(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.uc.DeleteNode.Execute(ctx, req.NodeID, client); err != nil {
		return nil, err
	}
	return gen.DeleteNode204Response{}, nil
}

// clientOf is where the request came from: a personal access token's is
// the API's, a sign-in session's access token the web's.
func clientOf(ctx context.Context) (domain.Client, error) {
	actor, err := shared.RequireActor(ctx)
	switch {
	case err != nil:
		return "", err
	case actor.APITokenID != uuid.UUID{}:
		return domain.ClientAPI, nil
	}
	return domain.ClientWeb, nil
}

// idOf is a required id that may be null; the body's check made sure it
// was sent.
func idOf(id nullable.Nullable[uuid.UUID]) *uuid.UUID {
	if id.IsNull() || !id.IsSpecified() {
		return nil
	}
	v := id.MustGet()
	return &v
}

// positionOf is after_id: absent puts the page last, null first, an id
// right after that sibling.
func positionOf(after nullable.Nullable[uuid.UUID]) app.Position {
	switch {
	case !after.IsSpecified():
		return app.Position{}
	case after.IsNull():
		return app.First()
	}
	return app.After(after.MustGet())
}

// parentOf is a node's parent as a required field that may be null: the
// zero Nullable would be no field at all.
func parentOf(id *uuid.UUID) nullable.Nullable[uuid.UUID] {
	if id == nil {
		return nullable.NewNullNullable[uuid.UUID]()
	}
	return nullable.NewNullableWithValue(*id)
}

func treeNodeOf(n domain.Node) gen.TreeNode {
	return gen.TreeNode{
		ID: n.ID, NotebookID: n.NotebookID, ParentID: parentOf(n.ParentID), Kind: gen.NodeKind(n.Kind), Name: n.Name,
		CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
	}
}

func pageOf(v app.PageView) gen.Page {
	n := v.Node
	ancestors := make([]gen.Ancestor, len(v.Ancestors))
	for i, a := range v.Ancestors {
		ancestors[i] = gen.Ancestor{ID: a.ID, Name: a.Name}
	}
	return gen.Page{
		ID: n.ID, NotebookID: n.NotebookID, ParentID: parentOf(n.ParentID), Kind: gen.NodeKind(n.Kind), Name: n.Name,
		CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, Ancestors: ancestors, Revision: v.Content.Revision,
		ByteSize: v.Content.ByteSize, ContentUpdatedAt: v.Content.UpdatedAt, ContentUpdatedBy: v.Content.UpdatedBy,
	}
}
