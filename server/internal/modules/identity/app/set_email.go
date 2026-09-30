package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// SetEmailDeps are SetEmail's collaborators.
type SetEmailDeps struct {
	Accounts AccountLocker
	Emails   EmailChanger
	Sessions SessionRevoker
	Tx       shared.TxManager
	Clock    Clock
	Logger   *slog.Logger
}

// SetEmail changes an account's address for the server's administrator:
// nervewiki users set-email (M1/P4 design 3.6). The address is how its
// owner signs in, so every session ends; the personal access tokens stay.
type SetEmail struct {
	d SetEmailDeps
}

// NewSetEmail returns the use case.
func NewSetEmail(d SetEmailDeps) *SetEmail {
	return &SetEmail{d: d}
}

// SetEmailResult is the account's new address and the sessions revoked.
type SetEmailResult struct {
	Email    string // normalized
	Sessions int
}

// Execute gives the account of email the address newEmail: 422
// validation_failed on new_email for an address that cannot be valid,
// identity.email_unchanged for the address it has; then, under the account
// row lock, the address changes (identity.email_taken when another account
// has it) and every session is revoked (email_changed).
func (u *SetEmail) Execute(ctx context.Context, email, newEmail string) (SetEmailResult, error) {
	newEmail, err := domain.NewEmail("new_email", newEmail)
	if err != nil {
		return SetEmailResult{}, err
	}
	if shared.NormalizeEmail(email) == newEmail {
		return SetEmailResult{}, domain.ErrEmailUnchanged
	}
	now := u.d.Clock.Now()
	result := SetEmailResult{Email: newEmail}
	var id uuid.UUID
	err = u.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		account, err := lockAccount(ctx, u.d.Accounts, email)
		if err != nil {
			return err
		}
		id = account.ID
		if err := u.d.Emails.ChangeEmail(ctx, id, newEmail, now); err != nil {
			return err
		}
		result.Sessions, err = u.d.Sessions.RevokeSessions(ctx, id, uuid.Nil(), domain.RevokeEmailChanged, now)
		return err
	})
	if err != nil {
		return SetEmailResult{}, err
	}
	u.d.Logger.InfoContext(ctx, "e-mail address changed", slog.String("user_id", id.String()),
		slog.Int("revoked_sessions", result.Sessions), slog.String("by", byCLI))
	return result, nil
}
