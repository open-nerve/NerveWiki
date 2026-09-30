package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// ActiveAccounts is what identity offers the modules that give an account
// new access, such as joining a workspace (M1 design 8, M1/P3 design 3.6).
type ActiveAccounts struct {
	Accounts AccountSharer
}

// ShareActiveAccount locks account id's row FOR SHARE until the caller's
// transaction ends and confirms that the account is active:
// domain.ErrAccountDeactivated (403) when it is not, domain.ErrAccountNotFound
// (404) when there is none. Call it first in the transaction that gives the
// account the access: a concurrent deactivation waits for that transaction,
// and its vetoers see what it committed; or the transaction waits for the
// deactivation and sees the account inactive.
func (a ActiveAccounts) ShareActiveAccount(ctx context.Context, id uuid.UUID) error {
	active, err := a.Accounts.ShareAccount(ctx, id)
	switch {
	case errors.Is(err, ErrNotFound):
		return domain.ErrAccountNotFound
	case err != nil:
		return err
	case !active:
		return domain.ErrAccountDeactivated
	}
	return nil
}
