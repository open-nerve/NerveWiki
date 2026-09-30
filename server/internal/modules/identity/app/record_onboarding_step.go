package app

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// RecordOnboardingStep records a completed onboarding step of the caller:
// POST /api/v0/me/onboarding-steps. The web app's registry knows the steps;
// the server checks only the id's form and the count (M1 design 8).
type RecordOnboardingStep struct {
	users UserUpdater
	clock Clock
}

// NewRecordOnboardingStep returns the use case.
func NewRecordOnboardingStep(users UserUpdater, clock Clock) *RecordOnboardingStep {
	return &RecordOnboardingStep{users: users, clock: clock}
}

// Execute records step once: recording it again changes nothing and
// answers the same account.
func (r *RecordOnboardingStep) Execute(ctx context.Context, step string) (domain.User, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return domain.User{}, err
	}
	if err := domain.CheckOnboardingStep(step); err != nil {
		return domain.User{}, err
	}
	user, err := r.users.RecordOnboardingStep(ctx, actor.UserID, step, r.clock.Now())
	if errors.Is(err, ErrNotFound) {
		return domain.User{}, unauthenticated(errUserUnknown)
	}
	return user, err
}
