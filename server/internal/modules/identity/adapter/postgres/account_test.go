package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

func updatedAt(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	if err := pool.QueryRow(context.Background(), `SELECT updated_at FROM users WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestUpdateDisplayName(t *testing.T) {
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)

	got, err := s.UpdateDisplayName(context.Background(), u.ID, "Alice Stone", later())

	if err != nil || got.ID != u.ID || got.Email != u.Email || got.DisplayName != "Alice Stone" || !updatedAt(t, pool, u.ID).Equal(later()) {
		t.Errorf("UpdateDisplayName() = %+v, %v; want Alice Stone, updated at %v", got, err, later())
	}
	if _, err := s.UpdateDisplayName(context.Background(), uuid.NewV7(), "x", later()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("UpdateDisplayName(unknown) = %v, want app.ErrNotFound", err)
	}
}

// A step is appended once, in the order recorded; recording it again
// leaves the row, updated_at included, as it was.
func TestRecordOnboardingStep(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)

	if _, err := s.RecordOnboardingStep(ctx, u.ID, "profile", later()); err != nil {
		t.Fatal(err)
	}
	got, err := s.RecordOnboardingStep(ctx, u.ID, "workspace", later())
	again, againErr := s.RecordOnboardingStep(ctx, u.ID, "profile", later().Add(time.Hour))

	if err != nil || againErr != nil || !slices.Equal(got.OnboardingSteps, []string{"profile", "workspace"}) ||
		!slices.Equal(again.OnboardingSteps, got.OnboardingSteps) || !updatedAt(t, pool, u.ID).Equal(later()) {
		t.Errorf("steps = %q, then %q (%v, %v), updated at %v; want profile and workspace once, updated at %v",
			got.OnboardingSteps, again.OnboardingSteps, err, againErr, updatedAt(t, pool, u.ID), later())
	}
	if _, err := s.RecordOnboardingStep(ctx, uuid.NewV7(), "profile", later()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("RecordOnboardingStep(unknown) = %v, want app.ErrNotFound", err)
	}
}

// The 33rd step breaks the CHECK: the store answers the domain's 422, and
// a step recorded already still answers.
func TestRecordOnboardingStepBeyondTheBound(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	for i := range domain.MaxOnboardingSteps {
		if _, err := s.RecordOnboardingStep(ctx, u.ID, fmt.Sprintf("s%d", i), later()); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	_, err := s.RecordOnboardingStep(ctx, u.ID, "one_more", later())
	got, known := s.RecordOnboardingStep(ctx, u.ID, "s0", later())

	if !errors.Is(err, domain.ErrTooManyOnboardingSteps) || known != nil || len(got.OnboardingSteps) != domain.MaxOnboardingSteps {
		t.Errorf("the 33rd step = %v, a known one = %v with %d steps; want ErrTooManyOnboardingSteps, then 32", err, known, len(got.OnboardingSteps))
	}
}
