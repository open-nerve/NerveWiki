package app

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// byCLI marks in the logs what the server's administrator did with
// nervewiki users (M1/P4 design 3.6).
const byCLI = "cli"

// lockAccount takes the account row lock of the account of email, first in
// the administrator's transaction: identity.account_not_found when there is
// none. An address that cannot be valid names no account and is not looked
// up, as at sign-in.
func lockAccount(ctx context.Context, accounts AccountLocker, email string) (LockedAccount, error) {
	email = shared.NormalizeEmail(email)
	if !shared.ValidEmail(email) {
		return LockedAccount{}, domain.ErrAccountNotFound
	}
	account, err := accounts.LockAccountByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return LockedAccount{}, domain.ErrAccountNotFound
	}
	return account, err
}
