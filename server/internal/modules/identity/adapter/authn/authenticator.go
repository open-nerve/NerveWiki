// Package authn implements the platform's httpserver.Authenticator with the
// identity module's authentication use case (M1/P1 design 3.5).
package authn

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// AuthenticateUseCase is app.Authenticate.
type AuthenticateUseCase interface {
	Execute(ctx context.Context, token string) (shared.Actor, error)
}

// Authenticator puts the request's actor in the context.
type Authenticator struct {
	uc AuthenticateUseCase
}

// New returns the authenticator over uc.
func New(uc AuthenticateUseCase) *Authenticator {
	return &Authenticator{uc: uc}
}

// Authenticate returns a context carrying the actor of token and the
// caller's rate-limit key, session:<id> (M1/P2 design 3.3). An invalid
// token is the use case's 401 *shared.Error; for an access token that is
// valid but for its expiry, that error also reports ExpiredCredential()
// true, so the platform's failure gate does not count it. Any other error
// passes through as an internal fault.
func (a *Authenticator) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	actor, err := a.uc.Execute(ctx, token)
	if errors.Is(err, app.ErrAccessTokenExpired) {
		return nil, "", expired{err}
	}
	if err != nil {
		return nil, "", err
	}
	return shared.WithActor(ctx, actor), "session:" + actor.SessionID.String(), nil
}

// expired is the 401 of an expired access token: the client's cue to
// refresh.
type expired struct{ error }

func (e expired) Unwrap() error { return e.error }

// ExpiredCredential tells the platform's failure gate to give the unit back.
func (expired) ExpiredCredential() bool { return true }
