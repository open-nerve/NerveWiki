package app

import (
	"context"
	"log/slog"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ActivateDeps are Activate's collaborators.
type ActivateDeps struct {
	Accounts  AccountLocker
	Users     UserActivator
	APITokens UsableAPITokenCounter
	Tx        shared.TxManager
	Clock     Clock
	Logger    *slog.Logger
}

// Activate makes a deactivated account active again for the server's
// administrator: nervewiki users activate (M1/P4 design 3.6). Its personal
// access tokens authenticate again; its sessions, revoked by the
// deactivation, stay revoked, and its owner signs in anew.
type Activate struct {
	d ActivateDeps
}

// NewActivate returns the use case.
func NewActivate(d ActivateDeps) *Activate {
	return &Activate{d: d}
}

// ActivateResult is the account's address and how many of its tokens are
// usable again; Already when it was active before, and nothing was done.
type ActivateResult struct {
	Email     string // normalized
	APITokens int
	Already   bool
}

// Execute activates the account of email under its row lock; one that is
// active already is left as it is. No such account is
// identity.account_not_found.
func (u *Activate) Execute(ctx context.Context, email string) (ActivateResult, error) {
	now := u.d.Clock.Now()
	var result ActivateResult
	var account LockedAccount
	err := u.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if account, err = lockAccount(ctx, u.d.Accounts, email); err != nil {
			return err
		}
		result.Email = account.Email
		if account.Active {
			result.Already = true
			return nil
		}
		if err := u.d.Users.ActivateUser(ctx, account.ID, now); err != nil {
			return err
		}
		result.APITokens, err = u.d.APITokens.CountUsableAPITokens(ctx, account.ID, now)
		return err
	})
	if err != nil {
		return ActivateResult{}, err
	}
	if result.Already {
		return result, nil
	}
	u.d.Logger.InfoContext(ctx, "account activated", slog.String("user_id", account.ID.String()),
		slog.Int("usable_api_tokens", result.APITokens), slog.String("by", byCLI))
	return result, nil
}
