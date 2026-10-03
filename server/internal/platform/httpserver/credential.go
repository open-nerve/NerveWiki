package httpserver

import (
	"context"
	"time"
)

type expiryKey struct{}

// WithCredentialExpiry returns ctx carrying when the request's credential
// expires. An Authenticator sets it on the context it returns; the zero
// time is never. A long-lived response ends there (M5 design 4.10).
func WithCredentialExpiry(ctx context.Context, at time.Time) context.Context {
	return context.WithValue(ctx, expiryKey{}, at)
}

// CredentialExpiry returns when the request's credential expires; ok is
// false when it never does, or when the Authenticator did not say.
func CredentialExpiry(ctx context.Context) (time.Time, bool) {
	at, _ := ctx.Value(expiryKey{}).(time.Time)
	return at, !at.IsZero()
}
