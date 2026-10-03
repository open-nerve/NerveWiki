package authn_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/authn"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The platform's port, satisfied by structure.
var _ httpserver.Authenticator = (*authn.Authenticator)(nil)

type fakeUseCase struct {
	actor   shared.Actor
	expires time.Time
	err     error
}

func (f fakeUseCase) Execute(context.Context, string) (app.Authenticated, error) {
	return app.Authenticated{Actor: f.actor, ExpiresAt: f.expires}, f.err
}

// The actor and the credential's expiry go into the context; the session
// is the rate-limit key.
func TestAuthenticatePutsTheActorInTheContext(t *testing.T) {
	actor := shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()}
	expires := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

	ctx, key, err := authn.New(fakeUseCase{actor: actor, expires: expires}).Authenticate(context.Background(), "token")

	got, gotErr := shared.RequireActor(ctx)
	if err != nil || gotErr != nil || got != actor || key != "session:"+actor.SessionID.String() {
		t.Errorf("Authenticate() = key %q, %v; actor %+v, %v; want session:%s and %+v", key, err, got, gotErr, actor.SessionID, actor)
	}
	if at, ok := httpserver.CredentialExpiry(ctx); !ok || !at.Equal(expires) {
		t.Errorf("CredentialExpiry() = %v, %v; want %v", at, ok, expires)
	}
}

// A personal access token is its own rate-limit key.
func TestAPersonalAccessTokenIsItsOwnKey(t *testing.T) {
	actor := shared.Actor{UserID: uuid.NewV7(), APITokenID: uuid.NewV7()}

	ctx, key, err := authn.New(fakeUseCase{actor: actor}).Authenticate(context.Background(), "nwk_pat_x")

	got, _ := shared.RequireActor(ctx)
	if err != nil || got != actor || key != "pat:"+actor.APITokenID.String() {
		t.Errorf("Authenticate() = key %q, %v; actor %+v; want pat:%s and %+v", key, err, got, actor.APITokenID, actor)
	}
	if at, ok := httpserver.CredentialExpiry(ctx); ok {
		t.Errorf("CredentialExpiry() of a token that never expires = %v", at)
	}
}

// expiredCredential is the platform's optional interface on a 401.
type expiredCredential interface{ ExpiredCredential() bool }

// A 401 stays a 401 for the platform to answer, and only an expired access
// token says so; any other failure passes through as the internal fault it
// is.
func TestAuthenticatePassesFailuresThrough(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		unauthorized bool
		expired      bool
	}{
		{"invalid credential", fmt.Errorf("%w: %w", shared.Unauthenticated(), errors.New("session is revoked")), true, false},
		{"expired access token", fmt.Errorf("%w: %w", shared.Unauthenticated(), app.ErrAccessTokenExpired), true, true},
		{"expired personal access token", fmt.Errorf("%w: %w", shared.Unauthenticated(), errors.New("personal access token has expired")), true, false},
		{"internal fault", errors.New("database is down"), false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, key, err := authn.New(fakeUseCase{err: tt.err}).Authenticate(context.Background(), "token")

			var ec expiredCredential
			isExpired := errors.As(err, &ec) && ec.ExpiredCredential()
			if ctx != nil || key != "" || !errors.Is(err, tt.err) || err.Error() != tt.err.Error() || isExpired != tt.expired {
				t.Errorf("Authenticate() = %v, %q, %v (expired %v); want nil, \"\", %v (expired %v)", ctx, key, err, isExpired, tt.err, tt.expired)
			}
			var pe httpserver.ProblemError
			if is401 := errors.As(err, &pe) && pe.ProblemStatus() == 401; is401 != tt.unauthorized {
				t.Errorf("Authenticate() error %v is a 401: %v, want %v", err, is401, tt.unauthorized)
			}
		})
	}
}
