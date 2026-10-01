package app

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CredentialLock is the account row lock protocol's first step (M1/P3 design
// 3.3): a transaction that issues or changes a credential with the caller's
// credential (changing the password, creating a token, deactivating) locks
// the account row first, then checks again, under the lock, that the
// caller's credential still holds. Authentication ran before the
// transaction: a concurrent change that revoked the credential in between
// is seen here.
type CredentialLock struct {
	Locker    CredentialLocker
	Sessions  SessionReader
	APITokens APITokenReader
}

// Lock locks actor's account row until the transaction ends and returns
// it, once the account is active and actor's credential valid at now: its
// session unrevoked and unexpired, or its personal access token unrevoked
// and unexpired. Otherwise it is 401 unauthorized. Call it first inside the
// transaction. now is the request's time, the one authentication judged the
// credential at: the check under the lock looks for what changed meanwhile
// (a revocation, a deactivation), as the request was allowed when it came;
// a credential that expires while the request waits for the lock still
// finishes it, as any request authenticated before the expiry does.
func (c CredentialLock) Lock(ctx context.Context, actor shared.Actor, now time.Time) (LockedAccount, error) {
	account, err := c.Locker.LockForCredentials(ctx, actor.UserID)
	switch {
	case errors.Is(err, ErrNotFound):
		return LockedAccount{}, unauthenticated(errUserUnknown)
	case err != nil:
		return LockedAccount{}, err
	case !account.Active:
		return LockedAccount{}, unauthenticated(errUserDeactivated)
	}
	if actor.APITokenID != uuid.Nil() {
		cred, err := c.APITokens.APITokenByID(ctx, actor.APITokenID)
		if err := tokenInvalid(cred, err, actor.UserID, now); err != nil {
			return LockedAccount{}, err
		}
		return account, nil
	}
	cred, err := c.Sessions.SessionCredential(ctx, actor.SessionID)
	if err := sessionInvalid(cred, err, actor.UserID, now); err != nil {
		return LockedAccount{}, err
	}
	return account, nil
}
