package authn_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/authn"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type fakeUseCase struct {
	actor shared.Actor
	err   error
}

func (f fakeUseCase) Execute(context.Context, string) (shared.Actor, error) { return f.actor, f.err }

func TestAuthenticatePutsTheActorInTheContext(t *testing.T) {
	actor := shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()}

	ctx, err := authn.New(fakeUseCase{actor: actor}).Authenticate(context.Background(), "token")

	got, gotErr := shared.RequireActor(ctx)
	if err != nil || gotErr != nil || got != actor {
		t.Errorf("Authenticate() = %v; actor %+v, %v; want %+v", err, got, gotErr, actor)
	}
}

// A 401 stays a 401 for the platform to answer; any other failure passes
// through as the internal fault it is.
func TestAuthenticatePassesFailuresThrough(t *testing.T) {
	for _, want := range []error{shared.Unauthenticated(), errors.New("database is down")} {
		ctx, err := authn.New(fakeUseCase{err: want}).Authenticate(context.Background(), "token")
		if ctx != nil || !errors.Is(err, want) {
			t.Errorf("Authenticate() = %v, %v; want no context and %v", ctx, err, want)
		}
	}
}
