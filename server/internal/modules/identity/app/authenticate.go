package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// lastUsedInterval is how often a personal access token's last_used_at is
// written at most (M1/P3 design 3.2).
const lastUsedInterval = time.Minute

// AuthenticateDeps are Authenticate's collaborators.
type AuthenticateDeps struct {
	AccessTokens AccessTokens
	Sessions     SessionReader
	APITokens    APITokenReader
	Touch        APITokenToucher
	Clock        Clock
	Logger       *slog.Logger
}

// Authenticate turns a bearer token into the request's actor (M1/P1 design
// 3.5, M1/P3 design 3.2). A token that starts with nwk_pat_ is a personal
// access token: one query by its hash. Any other is an access token: its
// signature and expiry, then one primary-key query for its session and the
// session's account.
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
	errPATMalformed    = errors.New("personal access token is malformed")
	errPATUnknown      = errors.New("personal access token does not exist")
	errPATMismatch     = errors.New("personal access token belongs to another account")
	errPATRevoked      = errors.New("personal access token is revoked")
	errPATExpired      = errors.New("personal access token has expired")
	errUserUnknown     = errors.New("account does not exist")
	errUserDeactivated = errors.New("account is deactivated")
)

// Execute returns the actor of token. An invalid credential is a
// *shared.Error of 401 that wraps the reason; an expired access token also
// matches ErrAccessTokenExpired. Any other error is an internal fault: a
// credential that could not be read.
func (a *Authenticate) Execute(ctx context.Context, token string) (shared.Actor, error) {
	now := a.d.Clock.Now()
	if strings.HasPrefix(token, domain.PATPrefix) {
		return a.personal(ctx, token, now)
	}
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

// personal authenticates a personal access token and records its use at
// most once a minute. Recording is best effort: a failed write is a warning
// with the token's id, and the token still authenticates.
func (a *Authenticate) personal(ctx context.Context, token string, now time.Time) (shared.Actor, error) {
	pat, ok := domain.ParsePAT(token)
	if !ok {
		return shared.Actor{}, unauthenticated(errPATMalformed)
	}
	cred, err := a.d.APITokens.APITokenByHash(ctx, pat.Hash())
	if err := tokenInvalid(cred, err, cred.UserID, now); err != nil {
		return shared.Actor{}, err
	}
	staleBefore := now.Add(-lastUsedInterval)
	if cred.LastUsedAt == nil || cred.LastUsedAt.Before(staleBefore) {
		if err := a.d.Touch.TouchAPIToken(ctx, cred.ID, now, staleBefore); err != nil {
			a.d.Logger.WarnContext(ctx, "API token last_used_at not written",
				slog.String("token_id", cred.ID.String()), slog.Any("error", err))
		}
	}
	return shared.Actor{UserID: cred.UserID, APITokenID: cred.ID}, nil
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

// tokenInvalid judges a personal access token of userID the same way. A
// token found by its hash is userID's by construction; the credential lock
// reads it by id and checks that it is still the caller's.
func tokenInvalid(cred APITokenCredential, err error, userID uuid.UUID, now time.Time) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return unauthenticated(errPATUnknown)
	case err != nil:
		return err
	case cred.UserID != userID:
		return unauthenticated(errPATMismatch)
	case cred.Revoked:
		return unauthenticated(errPATRevoked)
	case cred.ExpiresAt != nil && !now.Before(*cred.ExpiresAt):
		return unauthenticated(errPATExpired)
	case !cred.UserActive:
		return unauthenticated(errUserDeactivated)
	}
	return nil
}

func unauthenticated(reason error) error {
	return fmt.Errorf("%w: %w", shared.Unauthenticated(), reason)
}
