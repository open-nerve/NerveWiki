package shared_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestRequireActor(t *testing.T) {
	want := shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()}

	got, err := shared.RequireActor(shared.WithActor(context.Background(), want))

	if err != nil || got != want {
		t.Errorf("RequireActor() = %+v, %v; want %+v", got, err, want)
	}
}

func TestRequireActorWithoutActorIsUnauthenticated(t *testing.T) {
	_, err := shared.RequireActor(context.Background())

	if !errors.Is(err, shared.Unauthenticated()) {
		t.Errorf("RequireActor() error = %v, want 401 unauthorized", err)
	}
}
