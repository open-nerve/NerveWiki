package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// NotebookGate decides an MCP entry into a notebook (M9 design 4.6):
// node.list, unlocked, notebook.not_found for none, a deleted one, one
// the caller cannot see, and the zero id, without a read. It is no
// security boundary: a personal access token holds all its account may
// do, and the entry is a cd into the notebook (v0.1 design 6.3); each use
// case decides again.
type NotebookGate struct {
	notebooks Notebooks
	auth      shared.Authorizer
}

// NewNotebookGate returns the use case.
func NewNotebookGate(notebooks Notebooks, auth shared.Authorizer) *NotebookGate {
	return &NotebookGate{notebooks: notebooks, auth: auth}
}

// Execute lets the caller into the notebook id, in ListNodes's order: the
// actor, the zero id answered without a read, the notebook's workspace,
// the decision.
func (g *NotebookGate) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	if id == (uuid.UUID{}) {
		return domain.ErrNotebookNotFound
	}
	workspaceID, ok, err := g.notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return err
	case !ok:
		return domain.ErrNotebookNotFound
	}
	target := shared.Target{WorkspaceID: workspaceID, NotebookID: id}
	_, err = authorize(ctx, g.auth, actor, domain.ActionList, target, domain.ErrNotebookNotFound)
	return err
}
