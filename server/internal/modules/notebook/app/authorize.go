package app

import (
	"context"
	"errors"

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
