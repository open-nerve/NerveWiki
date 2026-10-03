// Package authn implements the platform's httpserver.Authenticator with the
// identity module's authentication use case (M1/P1 design 3.5).
package authn

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// AuthenticateUseCase is app.Authenticate.
type AuthenticateUseCase interface {
	Execute(ctx context.Context, token string) (app.Authenticated, error)
}

// Authenticator puts the request's actor, and when its credential expires,
// in the context.
type Authenticator struct {
	uc AuthenticateUseCase
}

// New returns the authenticator over uc.
func New(uc AuthenticateUseCase) *Authenticator {
	return &Authenticator{uc: uc}
}

// Authenticate returns a context carrying the actor of token, and when
// token expires (httpserver.CredentialExpiry, M5 design 4.10), and the
// caller's rate-limit key: session:<id>, or pat:<id> for a personal access
// token (M1/P2 design 3.2). An invalid token is the use case's 401
// *shared.Error; for an access token that is valid but for its expiry, that
// error also reports ExpiredCredential() true, so the platform's failure
// gate does not count it. An expired personal access token is not one: it
// cannot be refreshed (M1/P3 design 3.2). Any other error passes through as
// an internal fault.
func (a *Authenticator) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	auth, err := a.uc.Execute(ctx, token)
	if errors.Is(err, app.ErrAccessTokenExpired) {
		return nil, "", expired{err}
	}
	if err != nil {
		return nil, "", err
	}
	key := "session:" + auth.Actor.SessionID.String()
	if auth.Actor.APITokenID != uuid.Nil() {
		key = "pat:" + auth.Actor.APITokenID.String()
	}
	return httpserver.WithCredentialExpiry(shared.WithActor(ctx, auth.Actor), auth.ExpiresAt), key, nil
}

// expired is the 401 of an expired access token: the client's cue to
// refresh.
type expired struct{ error }

func (e expired) Unwrap() error { return e.error }

// ExpiredCredential tells the platform's failure gate to give the unit back.
func (expired) ExpiredCredential() bool { return true }
