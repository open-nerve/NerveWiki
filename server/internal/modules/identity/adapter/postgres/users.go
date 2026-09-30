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

// FindLoginAccount reads the account of email, a normalized address;
// app.ErrNotFound when there is none.
func (s *Store) FindLoginAccount(ctx context.Context, email string) (app.LoginAccount, error) {
	row, err := s.queries(ctx).FindLoginAccount(ctx, email)
	if err != nil {
		return app.LoginAccount{}, notFound(err)
	}
	return app.LoginAccount{ID: row.ID, PasswordHash: row.Password}, nil
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
