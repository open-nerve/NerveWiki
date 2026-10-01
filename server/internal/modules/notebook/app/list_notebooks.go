package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListNotebooks lists the notebooks the caller sees in a workspace:
// GET /api/v0/workspaces/{slug}/notebooks (M3/P1 design 3.7).
type ListNotebooks struct {
	workspaces Workspaces
	notebooks  NotebookLister
	auth       shared.Authorizer
}

// NewListNotebooks returns the use case.
func NewListNotebooks(workspaces Workspaces, notebooks NotebookLister, auth shared.Authorizer) *ListNotebooks {
	return &ListNotebooks{workspaces: workspaces, notebooks: notebooks, auth: auth}
}

// Execute returns the notebooks of the workspace of slug the caller has a
// role in, each with that role, by name. The query filters by the rule
// the access module decides by (shared.EffectiveNotebookRole), which gives
// each one's role here. A read takes no lock and opens no transaction.
func (l *ListNotebooks) Execute(ctx context.Context, slug string) ([]View, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	workspaceID, ok, err := l.workspaces.FindBySlug(ctx, slug)
	switch {
	case err != nil:
		return nil, err
	case !ok:
		return nil, domain.ErrWorkspaceNotFound
	}
	grant, err := authorize(ctx, l.auth, actor, domain.ActionList, shared.Target{WorkspaceID: workspaceID}, domain.ErrWorkspaceNotFound)
	if err != nil {
		return nil, err
	}
	listed, err := l.notebooks.ListNotebooks(ctx, workspaceID, actor.UserID, shared.ReachedByAccess(grant.WorkspaceRole))
	if err != nil {
		return nil, err
	}
	views := make([]View, len(listed))
	for i, n := range listed {
		views[i] = View{
			Notebook:    n.Notebook,
			Role:        shared.EffectiveNotebookRole(n.Explicit, n.Notebook.Access, grant.WorkspaceRole),
			MemberCount: n.MemberCount,
		}
	}
	return views, nil
}
