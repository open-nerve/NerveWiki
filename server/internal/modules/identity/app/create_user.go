package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// CreateUserDeps are CreateUser's collaborators.
type CreateUserDeps struct {
	Rules  *domain.PasswordRules
	Hasher PasswordHasher
	Users  UserCreator
	Clock  Clock
	Logger *slog.Logger
}

// CreateUser creates an account for the server's administrator: nervewiki
// users create (M1/P4 design 3.6). Unlike registration it does not ask
// whether sign-up is open, and signs nobody in.
type CreateUser struct {
	accounts accountCreator
	users    UserCreator
	logger   *slog.Logger
}

// NewCreateUser returns the use case.
func NewCreateUser(d CreateUserDeps) *CreateUser {
	return &CreateUser{accounts: accountCreator{rules: d.Rules, hasher: d.Hasher, clock: d.Clock}, users: d.Users, logger: d.Logger}
}

// CreatedUser is the account the administrator created.
type CreatedUser struct {
	ID    uuid.UUID
	Email string // normalized
}

// Execute creates the account of email with password: the checks and the
// hash of registration (422 validation_failed), then one insert; an address
// in use is identity.email_taken.
func (u *CreateUser) Execute(ctx context.Context, email, password string) (CreatedUser, error) {
	user, err := u.accounts.prepare(ctx, email, password)
	if err != nil {
		return CreatedUser{}, err
	}
	if err := u.users.CreateUser(ctx, user); err != nil {
		return CreatedUser{}, err
	}
	u.logger.InfoContext(ctx, "account created", slog.String("user_id", user.ID.String()), slog.String("by", byCLI))
	return CreatedUser{ID: user.ID, Email: user.Email}, nil
}
