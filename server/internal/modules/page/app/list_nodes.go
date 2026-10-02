package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListNodes lists a notebook's tree: GET /api/v0/notebooks/{notebook_id}/nodes
// (M4/P1 design 3.7).
type ListNodes struct {
	notebooks Notebooks
	nodes     Nodes
	auth      shared.Authorizer
}

// NewListNodes returns the use case.
func NewListNodes(notebooks Notebooks, nodes Nodes, auth shared.Authorizer) *ListNodes {
	return &ListNodes{notebooks: notebooks, nodes: nodes, auth: auth}
}

// Execute lists the nodes not deleted of the notebook id, each parent
// before its children, siblings in order. It reads without a transaction
// or a lock. A notebook that does not exist, is deleted, or that the caller
// has no role in is notebook.not_found.
func (l *ListNodes) Execute(ctx context.Context, id uuid.UUID) ([]domain.Node, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	workspaceID, ok, err := l.notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return nil, err
	case !ok:
		return nil, domain.ErrNotebookNotFound
	}
	target := shared.Target{WorkspaceID: workspaceID, NotebookID: id}
	if _, err := authorize(ctx, l.auth, actor, domain.ActionList, target, domain.ErrNotebookNotFound); err != nil {
		return nil, err
	}
	nodes, err := l.nodes.ListNodes(ctx, id)
	if err != nil {
		return nil, err
	}
	return domain.PreOrder(nodes), nil
}
