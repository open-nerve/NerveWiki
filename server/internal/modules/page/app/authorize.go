package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// authorize asks auth whether actor may do action on target, and answers
// notFound, the operation's own 404, when the caller cannot see it.
func authorize(ctx context.Context, auth shared.Authorizer, actor shared.Actor, action shared.Action,
	target shared.Target, notFound error,
) (shared.Grant, error) {
	grant, err := auth.Authorize(ctx, actor, action, target)
	if errors.Is(err, shared.ErrNotVisible) {
		return shared.Grant{}, notFound
	}
	return grant, err
}

// isForbidden reports whether err is the authorizer's denial, 403
// forbidden.
func isForbidden(err error) bool {
	return errors.Is(err, shared.Forbidden())
}

// decideNotebook decides action on the notebook id for actor, as MCP's
// entries and tools decide a notebook (M9 design 4.6): notebook.not_found
// for the zero id, an account entry's notebook that is no UUID, without a
// read; for none, a deleted one, and one the caller cannot see.
func decideNotebook(ctx context.Context, notebooks Notebooks, auth shared.Authorizer, actor shared.Actor, action shared.Action,
	id uuid.UUID,
) error {
	if id == (uuid.UUID{}) {
		return domain.ErrNotebookNotFound
	}
	workspaceID, ok, err := notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return err
	case !ok:
		return domain.ErrNotebookNotFound
	}
	_, err = authorize(ctx, auth, actor, action, shared.Target{WorkspaceID: workspaceID, NotebookID: id}, domain.ErrNotebookNotFound)
	return err
}

// found is err, or notFound for a repository's ErrNotFound.
func found(err, notFound error) error {
	if errors.Is(err, ErrNotFound) {
		return notFound
	}
	return err
}

// orNotFound is err, or notFound when there is none: a lock that found no
// row.
func orNotFound(err, notFound error) error {
	if err != nil {
		return err
	}
	return notFound
}
