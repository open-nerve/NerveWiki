package shared

import (
	"context"
	"uuid"
)

// Actor is the account a request acts as. Authentication puts it in the
// request context; handlers of every module read it with RequireActor. It
// tells which credential authenticated the request, never what the account
// may do: that is each module's to decide (v0.1 design 6.2).
type Actor struct {
	UserID uuid.UUID
	// The credential: the sign-in session of an access token, or a personal
	// access token. Exactly one of them is set.
	SessionID  uuid.UUID
	APITokenID uuid.UUID
}

type actorKey struct{}

// WithActor returns ctx carrying a.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// RequireActor returns the actor of the request, or 401 unauthorized when
// there is none. With the deny-by-default authentication (M1/P1 design 3.5)
// the error only happens when an operation is wired wrongly.
func RequireActor(ctx context.Context) (Actor, error) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	if !ok {
		return Actor{}, Unauthenticated()
	}
	return a, nil
}
