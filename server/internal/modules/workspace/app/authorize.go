package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// authorize asks auth whether actor may do action in workspaceID, and
// answers notFound, the operation's own 404, when the caller cannot see it.
func authorize(ctx context.Context, auth shared.Authorizer, actor shared.Actor, action shared.Action,
	workspaceID uuid.UUID, notFound error,
) (shared.Grant, error) {
	grant, err := auth.Authorize(ctx, actor, action, shared.Target{WorkspaceID: workspaceID})
	if errors.Is(err, shared.ErrNotVisible) {
		return shared.Grant{}, notFound
	}
	return grant, err
}

// found is err, or notFound for a repository's ErrNotFound.
func found(err, notFound error) error {
	if errors.Is(err, ErrNotFound) {
		return notFound
	}
	return err
}
