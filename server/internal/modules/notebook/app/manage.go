package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// manager takes the locks of a notebook's management write and decides
// (M3/P1 design 3.10).
type manager struct {
	workspaces Workspaces
	notebooks  NotebookWriter
	auth       shared.Authorizer
}

// lock locks n, read before the transaction ctx carries, for actor to do
// action: the workspace's row FOR SHARE, which runs the write before or
// after a change of the workspace's memberships, then the notebook's FOR
// NO KEY UPDATE, then the decision. It returns the notebook read under its
// lock. A notebook or a workspace deleted meanwhile, and one the caller
// cannot see, is notFound: notebook.not_found, or, for the operations that
// name a membership, notebook.member_not_found.
func (m manager) lock(ctx context.Context, actor shared.Actor, action shared.Action, n domain.Notebook, notFound error,
) (domain.Notebook, shared.Grant, error) {
	if ok, err := m.workspaces.ShareByID(ctx, n.WorkspaceID); err != nil || !ok {
		return domain.Notebook{}, shared.Grant{}, orNotFound(err, notFound)
	}
	locked, err := m.notebooks.LockNotebook(ctx, n.ID)
	if err != nil {
		return domain.Notebook{}, shared.Grant{}, found(err, notFound)
	}
	grant, err := authorize(ctx, m.auth, actor, action, shared.Target{WorkspaceID: locked.WorkspaceID, NotebookID: locked.ID},
		notFound)
	if err != nil {
		return domain.Notebook{}, shared.Grant{}, err
	}
	return locked, grant, nil
}

// find reads notebook id unlocked, for its workspace: the first lock is
// the workspace's.
func find(ctx context.Context, notebooks NotebookFinder, id uuid.UUID) (domain.Notebook, error) {
	n, err := notebooks.FindNotebook(ctx, id)
	if err != nil {
		return domain.Notebook{}, found(err, domain.ErrNotFound)
	}
	return n, nil
}
