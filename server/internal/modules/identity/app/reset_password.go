package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ResetPasswordDeps are ResetPassword's collaborators.
type ResetPasswordDeps struct {
	Accounts  AccountLocker
	Passwords PasswordHashWriter
	Sessions  SessionRevoker
	APITokens AllAPITokensRevoker
	Hasher    PasswordHasher
	Rules     *domain.PasswordRules
	Tx        shared.TxManager
	Clock     Clock
	Logger    *slog.Logger
}

// ResetPassword sets an account's password for the server's administrator:
// nervewiki users reset-password, the way back into an account whose owner
// forgot the password or whose credentials leaked (M1/P4 design 3.6).
type ResetPassword struct {
	d ResetPasswordDeps
}

// NewResetPassword returns the use case.
func NewResetPassword(d ResetPasswordDeps) *ResetPassword {
	return &ResetPassword{d: d}
}

// ResetPasswordResult is the account's address and what the reset revoked.
type ResetPasswordResult struct {
	Email     string // normalized
	Sessions  int
	APITokens int
}

// Execute resets the password of the account of email:
//
//  1. the password rules check the new password, with the address for the
//     stem rule: 422 validation_failed on password;
//  2. it is hashed outside the transaction;
//  3. one transaction takes the account row lock by the address, writes the
//     hash, then revokes every session (password_reset) and every personal
//     access token, in the global lock order. No such account is
//     identity.account_not_found.
//
// A sign-in or a token creation that verified the old password holds or
// waits for the same lock: what it creates is revoked here, or it finds its
// credential revoked or the hash changed under the lock.
func (u *ResetPassword) Execute(ctx context.Context, email, password string) (ResetPasswordResult, error) {
	if f := u.d.Rules.Check("password", password, shared.NormalizeEmail(email)); f != nil {
		return ResetPasswordResult{}, shared.Invalid(*f)
	}
	hash, err := u.d.Hasher.Hash(ctx, password)
	if err != nil {
		return ResetPasswordResult{}, err
	}
	now := u.d.Clock.Now()
	var result ResetPasswordResult
	var id uuid.UUID
	err = u.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		account, err := lockAccount(ctx, u.d.Accounts, email)
		if err != nil {
			return err
		}
		id, result.Email = account.ID, account.Email
		if err := u.d.Passwords.UpdatePasswordHash(ctx, id, hash, now); err != nil {
			return err
		}
		if result.Sessions, err = u.d.Sessions.RevokeSessions(ctx, id, uuid.Nil(), domain.RevokePasswordReset, now); err != nil {
			return err
		}
		result.APITokens, err = u.d.APITokens.RevokeAllAPITokens(ctx, id, now)
		return err
	})
	if err != nil {
		return ResetPasswordResult{}, err
	}
	u.d.Logger.InfoContext(ctx, "password reset", slog.String("user_id", id.String()),
		slog.Int("revoked_sessions", result.Sessions), slog.Int("revoked_api_tokens", result.APITokens), slog.String("by", byCLI))
	return result, nil
}
