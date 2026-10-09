package shared

import (
	"context"
	"uuid"
)

// Actor is the account a request acts as. Authentication puts it in the
// request context; handlers of every module read it with RequireActor. A
// background job acting for the account that started it acts as one, its
// JobID set (M7/P5 design 3.4). It tells which credential the account
// acts with, never what the account may do: that is each module's to
// decide (v0.1 design 6.2), on the account's roles as they are when it
// acts.
type Actor struct {
	UserID uuid.UUID
	// The credential: the sign-in session of an access token, a personal
	// access token, or the background job that acts for the account. Exactly
	// one of them is set.
	SessionID  uuid.UUID
	APITokenID uuid.UUID
	JobID      uuid.UUID
}

// Valid reports whether a is an account acting with exactly one
// credential, as authentication and the jobs make it.
func (a Actor) Valid() bool {
	held := 0
	for _, id := range []uuid.UUID{a.SessionID, a.APITokenID, a.JobID} {
		if id != (uuid.UUID{}) {
			held++
		}
	}
	return a.UserID != (uuid.UUID{}) && held == 1
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
