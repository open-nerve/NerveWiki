// Package authn implements the platform's httpserver.Authenticator with the
// identity module's authentication use case (M1/P1 design 3.5).
package authn

import (
	"context"

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

// Authenticate returns a context carrying the actor of token. An invalid
// token is the use case's 401 *shared.Error; any other error passes through
// as an internal fault.
func (a *Authenticator) Authenticate(ctx context.Context, token string) (context.Context, error) {
	actor, err := a.uc.Execute(ctx, token)
	if err != nil {
		return nil, err
	}
	return shared.WithActor(ctx, actor), nil
}
