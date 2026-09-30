package app

import (
	"context"
	"errors"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CurrentPassword is the check of the caller's current password that
// changing the password and creating a token share (M1/P3 design 3.4).
type CurrentPassword struct {
	Accounts PasswordAccountReader
	Verifier PasswordVerifier
	Lock     CredentialLock
	Tx       shared.TxManager
}

// Account reads the caller's account before the transaction. An account
// that authentication found a moment ago and that is gone now is 401.
func (c CurrentPassword) Account(ctx context.Context, actor shared.Actor) (PasswordAccount, error) {
	account, err := c.Accounts.PasswordAccount(ctx, actor.UserID)
	if errors.Is(err, ErrNotFound) {
		return PasswordAccount{}, unauthenticated(errUserUnknown)
	}
	return account, err
}

// Confirm verifies password against snapshot, the hash Account read, then
// runs prepare once, outside any transaction (changing the password hashes
// the new one there), then, in one transaction, takes the credential lock
// and runs write while the hash under the lock is still the one verified.
// When it changed in between (a concurrent login hashed the password again,
// or the password changed), password is verified against the new hash and
// the transaction runs once more; a second change, like a wrong password,
// is domain.ErrCurrentPasswordIncorrect. prepare may be nil.
func (c CurrentPassword) Confirm(ctx context.Context, actor shared.Actor, password, snapshot string, now time.Time,
	prepare func() error, write func(ctx context.Context) error,
) error {
	for range 2 {
		ok, _, err := c.Verifier.Verify(ctx, password, snapshot)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		if prepare != nil {
			if err := prepare(); err != nil {
				return err
			}
			prepare = nil
		}
		found := snapshot
		err = c.Tx.WithinTx(ctx, func(ctx context.Context) error {
			locked, err := c.Lock.Lock(ctx, actor, now)
			if err != nil {
				return err
			}
			if found = locked.PasswordHash; found != snapshot {
				return nil
			}
			return write(ctx)
		})
		if err != nil || found == snapshot {
			return err
		}
		snapshot = found
	}
	return domain.ErrCurrentPasswordIncorrect
}
