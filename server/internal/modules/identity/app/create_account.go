package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// accountCreator is the step that registration and the administrator's
// create share (M1/P4 design 3.6): it checks a new account and hashes its
// password.
type accountCreator struct {
	rules  *domain.PasswordRules
	hasher PasswordHasher
	clock  Clock
}

// prepare checks email and password, every problem at once (422
// validation_failed), hashes the password, outside any transaction, and
// returns the account to insert, named after its address, at the clock's
// now once the hash is done.
func (c accountCreator) prepare(ctx context.Context, email, password string) (NewUser, error) {
	email, err := domain.NewAccount(c.rules, email, password)
	if err != nil {
		return NewUser{}, err
	}
	hash, err := c.hasher.Hash(ctx, password)
	if err != nil {
		return NewUser{}, err
	}
	return NewUser{
		ID: uuid.NewV7(), Email: email, PasswordHash: hash, DisplayName: domain.DisplayNameFromEmail(email), Now: c.clock.Now(),
	}, nil
}
