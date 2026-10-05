package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// NotebookWorkspaces reads the notebooks' workspaces: the notebook
// module's, which bootstrap wires.
type NotebookWorkspaces interface {
	// WorkspaceOf is the workspace of the notebook id; false for no such
	// notebook, or a deleted one.
	WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
}

// PageTree is what the reads of the index read of a notebook's tree, on
// the pool: the page module's, which bootstrap wires (M6/P5 design 7; the
// landing's, M6/P6 design 2). Attachments and deleted pages are never
// among them.
type PageTree interface {
	// NotebookOf is the notebook of the page id; false for no such page.
	NotebookOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
	// All is the pages of notebookID, by id, each with its path from the
	// root.
	All(ctx context.Context, notebookID uuid.UUID) ([]domain.Node, error)
	// ByKeys is the pages of notebookID whose title key is one of keys,
	// each with its path from the root.
	ByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]domain.Node, error)
	// Paths is the pages of notebookID among ids, each with its path from
	// the root.
	Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]domain.Node, error)
}

// Access decides the reads of the index (M6/P5 design 2): by the decision
// on the notebook, which hides what the caller cannot see behind the
// not-found of what it named. Every role reads, so none is forbidden.
type Access struct {
	Notebooks NotebookWorkspaces
	Pages     PageTree
	Auth      shared.Authorizer
}

// page is the notebook of the page id, if actor may do action there: a
// page that does not exist, is deleted, is in a deleted notebook, or in a
// notebook actor has no role in, is page.not_found.
func (a Access) page(ctx context.Context, actor shared.Actor, id uuid.UUID, action shared.Action) (uuid.UUID, error) {
	notebookID, ok, err := a.Pages.NotebookOf(ctx, id)
	switch {
	case err != nil:
		return uuid.UUID{}, err
	case !ok:
		return uuid.UUID{}, domain.ErrPageNotFound
	}
	return notebookID, a.decide(ctx, actor, notebookID, action, domain.ErrPageNotFound)
}

// notebook tells whether actor may do action in the notebook id: one that
// does not exist, is deleted, or that actor has no role in, is
// notebook.not_found.
func (a Access) notebook(ctx context.Context, actor shared.Actor, id uuid.UUID, action shared.Action) error {
	return a.decide(ctx, actor, id, action, domain.ErrNotebookNotFound)
}

// decide is the decision on action in the notebook id, notFound when it is
// deleted or actor cannot see it.
func (a Access) decide(ctx context.Context, actor shared.Actor, id uuid.UUID, action shared.Action, notFound error) error {
	workspaceID, ok, err := a.Notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return err
	case !ok:
		return notFound
	}
	_, err = a.Auth.Authorize(ctx, actor, action, shared.Target{WorkspaceID: workspaceID, NotebookID: id})
	if errors.Is(err, shared.ErrNotVisible) {
		return notFound
	}
	return err
}
