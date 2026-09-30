package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The identity module's errors. api/modules/identity.yaml declares the codes
// of those the API answers with in x-problem-codes.
var (
	// ErrSignupDisabled answers a well-formed registration while sign-up is
	// off, before the address or the password is looked at; the platform's
	// structural 400 or 413 can come first.
	ErrSignupDisabled = shared.NewError(shared.KindForbidden, "identity.signup_disabled", "Sign-up is disabled on this instance.")
	// ErrEmailTaken answers a registration with an address in use.
	ErrEmailTaken = shared.NewError(shared.KindConflict, "identity.email_taken", "An account with this e-mail address already exists.")
	// ErrInvalidCredentials answers a login with an unknown address or a
	// wrong password alike (M1/P2 design 3.4).
	ErrInvalidCredentials = shared.NewError(shared.KindUnauthenticated, "identity.invalid_credentials", "The e-mail address or the password is incorrect.")
	// ErrAccountDeactivated answers a login with the right password for a
	// deactivated account; only then is the state revealed (M1/P2 design
	// 3.4).
	ErrAccountDeactivated = shared.NewError(shared.KindForbidden, "identity.account_deactivated", "This account is deactivated.")
)
