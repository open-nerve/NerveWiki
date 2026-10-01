package app

import (
	"context"
	"log/slog"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ChangePasswordDeps are ChangePassword's collaborators.
type ChangePasswordDeps struct {
	Password  CurrentPassword
	Rules     *domain.PasswordRules
	Hasher    PasswordHasher
	Passwords PasswordHashWriter
	Sessions  SessionRevoker
	Clock     Clock
	Logger    *slog.Logger
}

// ChangePassword changes the caller's password:
// POST /api/v0/me/change-password.
type ChangePassword struct {
	d ChangePasswordDeps
}

// NewChangePassword returns the use case.
func NewChangePassword(d ChangePasswordDeps) *ChangePassword {
	return &ChangePassword{d: d}
}

// ChangePasswordInput is the current password and the new one.
type ChangePasswordInput struct {
	Current string
	New     string
}

// Execute changes the caller's password (M1/P3 design 3.5): the new one is
// checked by the password rules, with the account's address (422 on
// new_password); the current one is confirmed (3.4) and the new one hashed
// once, outside the transaction; under the credential lock the new hash is
// written and the account's other sessions are revoked with reason
// password_changed: all of them when the caller is a personal access token,
// which has no session. The tokens stay (M1 design 4). An address changed
// meanwhile (the administrator's set-email keeps the tokens) is checked
// again under the lock: the rules judge the address the account has.
func (c *ChangePassword) Execute(ctx context.Context, in ChangePasswordInput) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	account, err := c.d.Password.Account(ctx, actor)
	if err != nil {
		return err
	}
	if f := c.d.Rules.Check("new_password", in.New, account.Email); f != nil {
		return shared.Invalid(*f)
	}
	now := c.d.Clock.Now()
	var hash string
	revoked := 0
	err = c.d.Password.Confirm(ctx, actor, in.Current, account.PasswordHash, now,
		func() error {
			var err error
			hash, err = c.d.Hasher.Hash(ctx, in.New)
			return err
		},
		func(ctx context.Context, locked LockedAccount) error {
			if locked.Email != account.Email {
				if f := c.d.Rules.Check("new_password", in.New, locked.Email); f != nil {
					return shared.Invalid(*f)
				}
			}
			if err := c.d.Passwords.UpdatePasswordHash(ctx, actor.UserID, hash, now); err != nil {
				return err
			}
			var err error
			revoked, err = c.d.Sessions.RevokeSessions(ctx, actor.UserID, actor.SessionID, domain.RevokePasswordChanged, now)
			return err
		})
	if err != nil {
		return err
	}
	c.d.Logger.InfoContext(ctx, "password changed", slog.String("user_id", actor.UserID.String()), slog.Int("revoked_sessions", revoked))
	return nil
}
