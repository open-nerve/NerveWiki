package app

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// AuthenticateDeps are Authenticate's collaborators.
type AuthenticateDeps struct {
	AccessTokens AccessTokens
	Sessions     SessionReader
	Clock        Clock
}

// Authenticate turns a bearer token into the request's actor (M1/P1 design
// 3.5): the access token's signature and expiry, then one primary-key query
// for its session and the session's account. Personal access tokens come
// in P3.
type Authenticate struct {
	d AuthenticateDeps
}

// NewAuthenticate returns the use case.
func NewAuthenticate(d AuthenticateDeps) *Authenticate {
	return &Authenticate{d: d}
}

// The reasons a credential fails. They go to the debug log only; the
// caller always sees 401 unauthorized.
var (
	errSessionUnknown  = errors.New("session does not exist")
	errSessionMismatch = errors.New("session belongs to another account")
	errSessionRevoked  = errors.New("session is revoked")
	errSessionExpired  = errors.New("session has expired")
	errUserDeactivated = errors.New("account is deactivated")
)

// Execute returns the actor of token. An invalid credential is a
// *shared.Error of 401 that wraps the reason; an expired access token also
// matches ErrAccessTokenExpired. Any other error is an internal fault: a
// credential that could not be read.
func (a *Authenticate) Execute(ctx context.Context, token string) (shared.Actor, error) {
	now := a.d.Clock.Now()
	claims, err := a.d.AccessTokens.Verify(token, now)
	if err != nil {
		return shared.Actor{}, unauthenticated(err)
	}
	cred, err := a.d.Sessions.SessionCredential(ctx, claims.SessionID)
	if err := sessionInvalid(cred, err, claims.UserID, now); err != nil {
		return shared.Actor{}, err
	}
	return shared.Actor{UserID: claims.UserID, SessionID: claims.SessionID}, nil
}

// sessionInvalid judges the session that the access token of userID names,
// as SessionReader read it with err, at now: nil when it is valid, a 401
// with the reason when it is not, err itself when the read failed.
func sessionInvalid(cred SessionCredential, err error, userID uuid.UUID, now time.Time) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return unauthenticated(errSessionUnknown)
	case err != nil:
		return err
	case cred.UserID != userID:
		return unauthenticated(errSessionMismatch)
	case cred.Revoked:
		return unauthenticated(errSessionRevoked)
	case !now.Before(cred.ExpiresAt):
		return unauthenticated(errSessionExpired)
	case !cred.UserActive:
		return unauthenticated(errUserDeactivated)
	}
	return nil
}

func unauthenticated(reason error) error {
	return fmt.Errorf("%w: %w", shared.Unauthenticated(), reason)
}
