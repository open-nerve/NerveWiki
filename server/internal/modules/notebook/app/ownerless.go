package app

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// lockOwnerless locks n, read before the transaction ctx carries, for
// actor to do action, a workspace admin's, on it while it is ownerless:
// the workspace's row FOR SHARE, the notebook's FOR NO KEY UPDATE, the
// decision at the workspace level, then the notebook still ownerless under
// its lock. Each refusal, the decision's 403 too, is notebook.not_found:
// no one but the workspace's admins learns whether the notebook is there
// (M3 design 4).
func lockOwnerless(ctx context.Context, workspaces Workspaces, notebooks NotebookWriter, auth shared.Authorizer, actor shared.Actor,
	action shared.Action, n domain.Notebook,
) (domain.Notebook, error) {
	if ok, err := workspaces.ShareByID(ctx, n.WorkspaceID); err != nil || !ok {
		return domain.Notebook{}, orNotFound(err, domain.ErrNotFound)
	}
	locked, err := notebooks.LockNotebook(ctx, n.ID)
	if err != nil {
		return domain.Notebook{}, found(err, domain.ErrNotFound)
	}
	_, err = auth.Authorize(ctx, actor, action, shared.Target{WorkspaceID: locked.WorkspaceID})
	switch {
	case errors.Is(err, shared.ErrNotVisible), errors.Is(err, shared.Forbidden()):
		return domain.Notebook{}, domain.ErrNotFound
	case err != nil:
		return domain.Notebook{}, err
	case locked.Ownerless == nil:
		return domain.Notebook{}, domain.ErrNotFound
	}
	return locked, nil
}

// profilesOf is the profiles of ids, each of which an account: a missing
// one is a fault, as withProfiles's.
func profilesOf(ctx context.Context, profiles MemberProfiles, ids []uuid.UUID) (map[uuid.UUID]Profile, error) {
	byID, err := profiles.MemberProfiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("no profile of account %s", id)
		}
	}
	return byID, nil
}
