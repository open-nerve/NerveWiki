package app

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// UpdateMe changes the caller's profile: PATCH /api/v0/me. It is one
// statement, no transaction.
type UpdateMe struct {
	users  UserUpdater
	reader UserReader
	clock  Clock
}

// NewUpdateMe returns the use case.
func NewUpdateMe(users UserUpdater, reader UserReader, clock Clock) *UpdateMe {
	return &UpdateMe{users: users, reader: reader, clock: clock}
}

// Execute checks p and applies it; a patch that sets nothing writes nothing
// and returns the account as it is.
func (u *UpdateMe) Execute(ctx context.Context, p domain.UserPatch) (domain.User, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return domain.User{}, err
	}
	p, err = domain.CheckUserPatch(p)
	if err != nil {
		return domain.User{}, err
	}
	var user domain.User
	if p.DisplayName == nil {
		user, err = u.reader.GetUser(ctx, actor.UserID)
	} else {
		user, err = u.users.UpdateDisplayName(ctx, actor.UserID, *p.DisplayName, u.clock.Now())
	}
	if errors.Is(err, ErrNotFound) {
		// Authentication found the account a moment ago; it is gone now.
		return domain.User{}, unauthenticated(errUserUnknown)
	}
	return user, err
}
