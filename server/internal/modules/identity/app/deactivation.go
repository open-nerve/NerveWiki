package app

import (
	"context"
	"time"
	"uuid"
)

// The extension point of deactivation (M1 design 8, M1/P3 design 3.6): the
// modules that keep an account's access elsewhere (M2's workspaces) can
// refuse a deactivation and follow one. They are built from the pool alone
// and run in the deactivation's transaction: their statements reach it
// through the context (postgres.DB).

// Deactivation is an account being deactivated: its id and address, read
// under the account row lock, and the deactivation's instant.
type Deactivation struct {
	UserID uuid.UUID
	Email  string
	At     time.Time
}

// DeactivationVetoer may refuse a deactivation. It runs under the account
// row lock, before any write. Returning a *shared.Error rolls the
// deactivation back and answers that error: the registrant adds its code to
// deactivateMe's x-problem-codes. Any other error is a fault, rolled back
// too.
type DeactivationVetoer interface {
	VetoDeactivation(ctx context.Context, d Deactivation) error
}

// DeactivationSubscriber follows a deactivation. It runs after the writes,
// in the same transaction: an error rolls the whole deactivation back.
type DeactivationSubscriber interface {
	AccountDeactivated(ctx context.Context, d Deactivation) error
}
