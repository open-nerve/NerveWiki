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
	// ErrRefreshTokenInvalid answers every refresh that does not rotate:
	// unknown, expired, revoked, reused or forged (M1/P2 design 3.5).
	ErrRefreshTokenInvalid = shared.NewError(shared.KindUnauthenticated, "identity.refresh_token_invalid", "The refresh token is not valid; sign in again.")
	// ErrCurrentPasswordIncorrect answers a change of password or a new
	// token whose current password is wrong. It is 422, not 401: a client
	// takes a 401 for the end of its session (M1/P3 design 3.4).
	ErrCurrentPasswordIncorrect = shared.NewError(shared.KindInvalid, "identity.current_password_incorrect", "The current password is incorrect.")
	// ErrAPITokenNotFound answers a revocation of a token that does not
	// exist, is revoked already, or is another account's: the three look
	// the same (M1/P3 design 3.2).
	ErrAPITokenNotFound = shared.NewError(shared.KindNotFound, "identity.api_token_not_found", "The API token does not exist.")
	// ErrTooManyOnboardingSteps answers a step beyond MaxOnboardingSteps
	// recorded: users_onboarding_steps_check decides, so concurrent
	// records cannot pass it (M1/P3 design 3.5).
	ErrTooManyOnboardingSteps = shared.Invalid(shared.FieldError{
		Field: "step", Code: shared.FieldOutOfRange, Message: "at most 32 steps can be recorded",
	})
	// ErrAccountNotFound answers a request about an account that does not
	// exist: ShareActiveAccount's, and the administrator's commands' (P4).
	ErrAccountNotFound = shared.NewError(shared.KindNotFound, "identity.account_not_found", "The account does not exist.")
)
