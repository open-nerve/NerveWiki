package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// CreateUser inserts u. A taken address is domain.ErrEmailTaken. The domain
// validated every value, so a CHECK violation is a bug: an internal error
// (500), not a domain error.
func (s *Store) CreateUser(ctx context.Context, u app.NewUser) error {
	err := s.queries(ctx).CreateUser(ctx, gen.CreateUserParams{
		ID: u.ID, Email: u.Email, Password: u.PasswordHash, DisplayName: u.DisplayName, Now: u.Now,
	})
	switch {
	case uniqueViolation(err, "users_email_key"):
		return domain.ErrEmailTaken
	case err != nil:
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUser reads account id; app.ErrNotFound when there is none.
func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := s.queries(ctx).GetUser(ctx, id)
	if err != nil {
		return domain.User{}, notFound(err)
	}
	return domain.User{
		ID:              row.ID,
		Email:           row.Email,
		DisplayName:     row.DisplayName,
		OnboardingSteps: row.OnboardingSteps,
	}, nil
}

// UpdateDisplayName sets account id's display name at now and returns the
// account; app.ErrNotFound when there is none.
func (s *Store) UpdateDisplayName(ctx context.Context, id uuid.UUID, name string, now time.Time) (domain.User, error) {
	row, err := s.queries(ctx).UpdateDisplayName(ctx, gen.UpdateDisplayNameParams{DisplayName: name, Now: now, ID: id})
	if err != nil {
		return domain.User{}, notFound(err)
	}
	return domain.User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, OnboardingSteps: row.OnboardingSteps}, nil
}

// RecordOnboardingStep appends step to account id's completed steps unless
// it is there, at now, and returns the account; app.ErrNotFound when there
// is none. A step beyond the CHECK's bound is
// domain.ErrTooManyOnboardingSteps: the domain checked the step's form, so
// only the count can break the CHECK.
func (s *Store) RecordOnboardingStep(ctx context.Context, id uuid.UUID, step string, now time.Time) (domain.User, error) {
	row, err := s.queries(ctx).RecordOnboardingStep(ctx, gen.RecordOnboardingStepParams{Step: step, Now: now, ID: id})
	switch {
	case checkViolation(err, "users_onboarding_steps_check"):
		return domain.User{}, domain.ErrTooManyOnboardingSteps
	case err != nil:
		return domain.User{}, notFound(err)
	}
	return domain.User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, OnboardingSteps: row.OnboardingSteps}, nil
}

// DeactivateUser sets account id inactive at now.
func (s *Store) DeactivateUser(ctx context.Context, id uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).DeactivateUser(ctx, gen.DeactivateUserParams{Now: now, ID: id}); err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}
	return nil
}

// ShareAccount locks account id's row FOR SHARE until the transaction ends
// and reports whether it is active; app.ErrNotFound when there is none.
// Outside a transaction the lock would end with the statement.
func (s *Store) ShareAccount(ctx context.Context, id uuid.UUID) (bool, error) {
	active, err := s.queries(ctx).ShareAccount(ctx, id)
	if err != nil {
		return false, notFound(err)
	}
	return active, nil
}

// FindLoginAccount reads the account of email, a normalized address;
// app.ErrNotFound when there is none.
func (s *Store) FindLoginAccount(ctx context.Context, email string) (app.LoginAccount, error) {
	row, err := s.queries(ctx).FindLoginAccount(ctx, email)
	if err != nil {
		return app.LoginAccount{}, notFound(err)
	}
	return app.LoginAccount{ID: row.ID, PasswordHash: row.Password}, nil
}

// PasswordAccount reads account id's address and hash; app.ErrNotFound
// when there is none.
func (s *Store) PasswordAccount(ctx context.Context, id uuid.UUID) (app.PasswordAccount, error) {
	row, err := s.queries(ctx).GetPasswordAccount(ctx, id)
	if err != nil {
		return app.PasswordAccount{}, notFound(err)
	}
	return app.PasswordAccount{Email: row.Email, PasswordHash: row.Password}, nil
}

// LockForCredentials locks account id's row until the transaction ends and
// reads it; app.ErrNotFound when there is none. Outside a transaction the
// lock would end with the statement: call it inside one.
func (s *Store) LockForCredentials(ctx context.Context, id uuid.UUID) (app.LockedAccount, error) {
	row, err := s.queries(ctx).LockUserForCredentials(ctx, id)
	if err != nil {
		return app.LockedAccount{}, notFound(err)
	}
	return app.LockedAccount{Email: row.Email, PasswordHash: row.Password, Active: row.IsActive}, nil
}

// UpdatePasswordHash stores hash as account id's password, at now.
func (s *Store) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string, now time.Time) error {
	if err := s.queries(ctx).UpdatePasswordHash(ctx, gen.UpdatePasswordHashParams{Password: hash, Now: now, ID: id}); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	return nil
}
