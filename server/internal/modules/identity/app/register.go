package app

import (
	"context"
	"log/slog"
	"net/netip"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// RegisterDeps are Register's collaborators and settings.
type RegisterDeps struct {
	Policy   SignupPolicy
	Rules    *domain.PasswordRules
	Hasher   PasswordHasher
	Tx       shared.TxManager
	Users    UserCreator
	Sessions SessionCreator
	Issuance Issuance
	Clock    Clock
	Logger   *slog.Logger
}

// Register creates an account and signs it in: POST /api/v0/auth/register.
type Register struct {
	d RegisterDeps
}

// NewRegister returns the use case.
func NewRegister(d RegisterDeps) *Register {
	return &Register{d: d}
}

// RegisterInput is a registration and where it comes from.
type RegisterInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        netip.Addr
}

// Execute registers in.Email (M1/P1 design 3.7):
//
//  1. closed sign-up answers 403 before any other check;
//  2. the address and the password are validated, all fields at once (422);
//  3. the password is hashed outside the transaction;
//  4. the session and its tokens are made outside it too, so nothing can
//     fail after it commits;
//  5. one transaction inserts the account and its first session; an
//     address in use is 409 identity.email_taken.
func (r *Register) Execute(ctx context.Context, in RegisterInput) (Tokens, error) {
	allowed, err := r.d.Policy.AllowSignup(ctx)
	if err != nil {
		return Tokens{}, err
	}
	if !allowed {
		return Tokens{}, domain.ErrSignupDisabled
	}
	email, err := domain.NewAccount(r.d.Rules, in.Email, in.Password)
	if err != nil {
		return Tokens{}, err
	}
	hash, err := r.d.Hasher.Hash(ctx, in.Password)
	if err != nil {
		return Tokens{}, err
	}
	now := r.d.Clock.Now()
	user := NewUser{ID: uuid.NewV7(), Email: email, PasswordHash: hash, DisplayName: domain.DisplayNameFromEmail(email), Now: now}
	session, tokens, err := r.d.Issuance.newSession(user.ID, in.UserAgent, in.IP, now)
	if err != nil {
		return Tokens{}, err
	}

	err = r.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := r.d.Users.CreateUser(ctx, user); err != nil {
			return err
		}
		return r.d.Sessions.CreateSession(ctx, session)
	})
	if err != nil {
		return Tokens{}, err
	}
	r.d.Logger.InfoContext(ctx, "account registered",
		slog.String("user_id", user.ID.String()), slog.String("session_id", session.ID.String()))
	return tokens, nil
}
