package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetPage reads a page: GET /api/v0/pages/{page_id} (M4/P1 design 3.7).
type GetPage struct {
	notebooks Notebooks
	nodes     Nodes
	auth      shared.Authorizer
}

// NewGetPage returns the use case.
func NewGetPage(notebooks Notebooks, nodes Nodes, auth shared.Authorizer) *GetPage {
	return &GetPage{notebooks: notebooks, nodes: nodes, auth: auth}
}

// Execute reads the page id with its ancestors, without a transaction or a
// lock: its node first, then the decision on its notebook, then the rest. A
// page that does not exist, is deleted, or whose notebook the caller has no
// role in is page.not_found.
func (g *GetPage) Execute(ctx context.Context, id uuid.UUID) (PageView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return PageView{}, err
	}
	n, err := g.nodes.FindNode(ctx, id)
	switch {
	case err != nil:
		return PageView{}, found(err, domain.ErrNotFound)
	case n.Kind != domain.KindPage:
		return PageView{}, domain.ErrNotFound
	}
	workspaceID, ok, err := g.notebooks.WorkspaceOf(ctx, n.NotebookID)
	switch {
	case err != nil:
		return PageView{}, err
	case !ok:
		return PageView{}, domain.ErrNotFound
	}
	target := shared.Target{WorkspaceID: workspaceID, NotebookID: n.NotebookID}
	if _, err := authorize(ctx, g.auth, actor, domain.ActionRead, target, domain.ErrNotFound); err != nil {
		return PageView{}, err
	}
	return viewOf(ctx, g.nodes, n)
}
