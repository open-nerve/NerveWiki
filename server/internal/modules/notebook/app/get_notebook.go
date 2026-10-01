package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetNotebook reads a notebook the caller has a role in:
// GET /api/v0/notebooks/{notebook_id}.
type GetNotebook struct {
	notebooks NotebookFinder
	auth      shared.Authorizer
}

// NewGetNotebook returns the use case.
func NewGetNotebook(notebooks NotebookFinder, auth shared.Authorizer) *GetNotebook {
	return &GetNotebook{notebooks: notebooks, auth: auth}
}

// Execute returns notebook id, with the caller's effective role. The
// decision needs the notebook's workspace, so the notebook is read first.
// A notebook that does not exist, is deleted, or that the caller has no
// role in is notebook.not_found alike. A read takes no lock and opens no
// transaction.
func (g *GetNotebook) Execute(ctx context.Context, id uuid.UUID) (View, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return View{}, err
	}
	n, err := g.notebooks.FindNotebook(ctx, id)
	if err != nil {
		return View{}, found(err, domain.ErrNotFound)
	}
	grant, err := authorize(ctx, g.auth, actor, domain.ActionRead, shared.Target{WorkspaceID: n.WorkspaceID, NotebookID: n.ID},
		domain.ErrNotFound)
	if err != nil {
		return View{}, err
	}
	count, err := g.notebooks.CountMembers(ctx, n.ID)
	if err != nil {
		return View{}, err
	}
	return View{Notebook: n, Role: grant.NotebookRole, MemberCount: count}, nil
}
