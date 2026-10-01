package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
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
// and reads, under the lock, whether it is active and its address;
// app.ErrNotFound when there is none. Outside a transaction the lock would
// end with the statement, and the caller's access would no longer wait for
// a deactivation: that is a fault (M1 handoff to M2, item 3).
func (s *Store) ShareAccount(ctx context.Context, id uuid.UUID) (app.SharedAccount, error) {
	if !postgres.InTx(ctx) {
		return app.SharedAccount{}, errors.New("share the account row: not in a transaction, the lock would end with the statement")
	}
	row, err := s.queries(ctx).ShareAccount(ctx, id)
	if err != nil {
		return app.SharedAccount{}, notFound(err)
	}
	return app.SharedAccount{Active: row.IsActive, Email: row.Email}, nil
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
	return app.LockedAccount{ID: id, Email: row.Email, PasswordHash: row.Password, Active: row.IsActive}, nil
}

// LockAccountByEmail takes the account row lock of the account of email,
// normalized, until the transaction ends, and returns what it read under
// it; app.ErrNotFound when there is none.
func (s *Store) LockAccountByEmail(ctx context.Context, email string) (app.LockedAccount, error) {
	row, err := s.queries(ctx).LockUserByEmail(ctx, email)
	if err != nil {
		return app.LockedAccount{}, notFound(err)
	}
	return app.LockedAccount{ID: row.ID, Email: row.Email, PasswordHash: row.Password, Active: row.IsActive}, nil
}

// ActivateUser makes account id active again, at now.
func (s *Store) ActivateUser(ctx context.Context, id uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).ActivateUser(ctx, gen.ActivateUserParams{Now: now, ID: id}); err != nil {
		return fmt.Errorf("activate user: %w", err)
	}
	return nil
}

// ChangeEmail sets account id's address to email, normalized, at now;
// domain.ErrEmailTaken when another account has it.
func (s *Store) ChangeEmail(ctx context.Context, id uuid.UUID, email string, now time.Time) error {
	err := s.queries(ctx).ChangeEmail(ctx, gen.ChangeEmailParams{Email: email, Now: now, ID: id})
	switch {
	case uniqueViolation(err, "users_email_key"):
		return domain.ErrEmailTaken
	case err != nil:
		return fmt.Errorf("change email: %w", err)
	}
	return nil
}

// UpdatePasswordHash stores hash as account id's password, at now.
func (s *Store) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string, now time.Time) error {
	if err := s.queries(ctx).UpdatePasswordHash(ctx, gen.UpdatePasswordHashParams{Password: hash, Now: now, ID: id}); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	return nil
}

// AccountIDByEmail returns the id of the account of email, a normalized
// address, and whether there is one.
func (s *Store) AccountIDByEmail(ctx context.Context, email string) (uuid.UUID, bool, error) {
	id, err := s.queries(ctx).AccountIDByEmail(ctx, email)
	if errors.Is(notFound(err), app.ErrNotFound) {
		return uuid.UUID{}, false, nil
	}
	if err != nil {
		return uuid.UUID{}, false, fmt.Errorf("account by email: %w", err)
	}
	return id, true, nil
}

// Profiles reads the profiles of the accounts ids, by id; an id of no
// account is left out.
func (s *Store) Profiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Profile, error) {
	rows, err := s.queries(ctx).ProfilesByID(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("profiles: %w", err)
	}
	profiles := make(map[uuid.UUID]domain.Profile, len(rows))
	for _, r := range rows {
		profiles[r.ID] = domain.Profile{DisplayName: r.DisplayName, Email: r.Email}
	}
	return profiles, nil
}
