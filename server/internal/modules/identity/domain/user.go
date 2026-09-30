// Package domain holds the identity module's rules: pure functions and
// values. The rules of e-mail addresses, which other modules share, are in
// internal/shared.
package domain

import "uuid"

// User is an account as the API shows it.
type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	// OnboardingSteps are the ids of the onboarding steps the account has
	// completed; the web app's registry defines the steps (M1 design 8).
	OnboardingSteps []string
}
