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
	n, err := readable(ctx, g.notebooks, g.nodes, g.auth, id)
	if err != nil {
		return PageView{}, err
	}
	return viewOf(ctx, g.nodes, n)
}

// readable is the node of the page id, if the caller may read it: its node,
// then its notebook's workspace, then the decision, unlocked. A page that
// does not exist, is deleted, or whose notebook the caller has no role in
// is page.not_found.
func readable(ctx context.Context, notebooks Notebooks, nodes Nodes, auth shared.Authorizer, id uuid.UUID) (domain.Node, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return domain.Node{}, err
	}
	n, err := nodes.FindNode(ctx, id)
	switch {
	case err != nil:
		return domain.Node{}, found(err, domain.ErrNotFound)
	case n.Kind != domain.KindPage:
		return domain.Node{}, domain.ErrNotFound
	}
	workspaceID, ok, err := notebooks.WorkspaceOf(ctx, n.NotebookID)
	switch {
	case err != nil:
		return domain.Node{}, err
	case !ok:
		return domain.Node{}, domain.ErrNotFound
	}
	target := shared.Target{WorkspaceID: workspaceID, NotebookID: n.NotebookID}
	if _, err := authorize(ctx, auth, actor, domain.ActionRead, target, domain.ErrNotFound); err != nil {
		return domain.Node{}, err
	}
	return n, nil
}
