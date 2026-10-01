package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ActiveAccounts is what identity offers the modules that give an account
// new access, such as joining a workspace (M1 design 8, M1/P3 design 3.6).
type ActiveAccounts struct {
	Accounts AccountSharer
}

// ShareActiveAccount locks account id's row FOR SHARE until the caller's
// transaction ends, confirms that the account is active and returns its
// address, read under the lock: domain.ErrAccountDeactivated (403) when it
// is not active, domain.ErrAccountNotFound (404) when there is none. Call it
// first in the transaction that gives the account the access: a concurrent
// deactivation, or change of the address, waits for that transaction, and
// its vetoers see what it committed; or the transaction waits for it and
// sees the account inactive, or its new address.
func (a ActiveAccounts) ShareActiveAccount(ctx context.Context, id uuid.UUID) (string, error) {
	account, err := active(a.Accounts.ShareAccount(ctx, id))
	return account.Email, err
}

// ShareActiveAccountByEmail is ShareActiveAccount for the administrator's
// commands, which name the account by its address (M2/P4 design 3.3): it
// returns the account's id. An address that cannot be valid names no
// account and is not looked up, as at sign-in.
func (a ActiveAccounts) ShareActiveAccountByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	email = shared.NormalizeEmail(email)
	if !shared.ValidEmail(email) {
		return uuid.Nil(), domain.ErrAccountNotFound
	}
	account, err := active(a.Accounts.ShareAccountByEmail(ctx, email))
	return account.ID, err
}

// active is the account a share read, when it is active.
func active(account SharedAccount, err error) (SharedAccount, error) {
	switch {
	case errors.Is(err, ErrNotFound):
		return SharedAccount{}, domain.ErrAccountNotFound
	case err != nil:
		return SharedAccount{}, err
	case !account.Active:
		return SharedAccount{}, domain.ErrAccountDeactivated
	}
	return account, nil
}
