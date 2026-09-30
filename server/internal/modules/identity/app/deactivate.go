package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DeactivationSteps are what a deactivation does under the account row
// lock (M1/P3 design 3.6), the caller's own and the administrator's alike
// (M1/P4 design 3.6).
type DeactivationSteps struct {
	Users       UserDeactivator
	Sessions    SessionRevoker
	Vetoers     []DeactivationVetoer
	Subscribers []DeactivationSubscriber
}

// run deactivates d.UserID: the vetoers in order, the first refusal ending
// it; the account inactive and every session revoked with reason
// deactivated; the subscribers in order. It returns how many sessions it
// revoked. The personal access tokens and the onboarding steps stay:
// authentication refuses the tokens while the account is inactive.
func (s DeactivationSteps) run(ctx context.Context, d Deactivation) (int, error) {
	for _, v := range s.Vetoers {
		if err := v.VetoDeactivation(ctx, d); err != nil {
			return 0, err
		}
	}
	if err := s.Users.DeactivateUser(ctx, d.UserID, d.At); err != nil {
		return 0, err
	}
	revoked, err := s.Sessions.RevokeSessions(ctx, d.UserID, uuid.Nil(), domain.RevokeDeactivated, d.At)
	if err != nil {
		return 0, err
	}
	for _, sub := range s.Subscribers {
		if err := sub.AccountDeactivated(ctx, d); err != nil {
			return 0, err
		}
	}
	return revoked, nil
}

// logDeactivation logs how a deactivation of userID by by ended once it had
// the lock: a refusal with its code, or the deactivation with the sessions
// it revoked. A failure of the caller's credential is not a refusal, and a
// fault is the caller's to report.
func logDeactivation(ctx context.Context, logger *slog.Logger, userID uuid.UUID, by string, revoked int, err error) {
	var refused *shared.Error
	switch {
	case errors.As(err, &refused) && refused.Kind != shared.KindUnauthenticated:
		logger.InfoContext(ctx, "account deactivation refused", slog.String("user_id", userID.String()),
			slog.String("code", refused.Code), slog.String("by", by))
	case err == nil:
		logger.InfoContext(ctx, "account deactivated", slog.String("user_id", userID.String()),
			slog.Int("revoked_sessions", revoked), slog.String("by", by))
	}
}

// DeactivateDeps are Deactivate's collaborators.
type DeactivateDeps struct {
	Lock   CredentialLock
	Steps  DeactivationSteps
	Tx     shared.TxManager
	Clock  Clock
	Logger *slog.Logger
}

// Deactivate deactivates the caller's account: POST /api/v0/me/deactivate.
// Any credential may, and no password is asked for: the server's
// administrator can activate the account again, and nothing but its
// sessions is destroyed (M1/P3 design 3.6).
type Deactivate struct {
	d DeactivateDeps
}

// NewDeactivate returns the use case.
func NewDeactivate(d DeactivateDeps) *Deactivate {
	return &Deactivate{d: d}
}

// Execute takes the credential lock, then runs the deactivation's steps, in
// one transaction.
func (u *Deactivate) Execute(ctx context.Context) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	now := u.d.Clock.Now()
	revoked := 0
	err = u.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := u.d.Lock.Lock(ctx, actor, now)
		if err != nil {
			return err
		}
		revoked, err = u.d.Steps.run(ctx, Deactivation{UserID: actor.UserID, Email: locked.Email, At: now})
		return err
	})
	logDeactivation(ctx, u.d.Logger, actor.UserID, "self", revoked, err)
	return err
}

// DeactivateAccountDeps are DeactivateAccount's collaborators.
type DeactivateAccountDeps struct {
	Accounts AccountLocker
	Steps    DeactivationSteps
	Tx       shared.TxManager
	Clock    Clock
	Logger   *slog.Logger
}

// DeactivateAccount deactivates an account for the server's administrator:
// nervewiki users deactivate (M1/P4 design 3.6), with the steps, the
// vetoers and subscribers too, of the caller's own.
type DeactivateAccount struct {
	d DeactivateAccountDeps
}

// NewDeactivateAccount returns the use case.
func NewDeactivateAccount(d DeactivateAccountDeps) *DeactivateAccount {
	return &DeactivateAccount{d: d}
}

// DeactivateAccountResult is the account's address and the sessions the
// deactivation revoked; Already when the account was inactive before, and
// nothing was done.
type DeactivateAccountResult struct {
	Email    string // normalized
	Sessions int
	Already  bool
}

// Execute deactivates the account of email under its row lock. One that is
// inactive already is left as it is: the vetoers and the subscribers see
// each deactivation once. No such account is identity.account_not_found.
func (u *DeactivateAccount) Execute(ctx context.Context, email string) (DeactivateAccountResult, error) {
	now := u.d.Clock.Now()
	var result DeactivateAccountResult
	var id uuid.UUID
	err := u.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		account, err := lockAccount(ctx, u.d.Accounts, email)
		if err != nil {
			return err
		}
		id, result.Email = account.ID, account.Email
		if !account.Active {
			result.Already = true
			return nil
		}
		result.Sessions, err = u.d.Steps.run(ctx, Deactivation{UserID: account.ID, Email: account.Email, At: now})
		return err
	})
	if id != uuid.Nil() && !result.Already {
		logDeactivation(ctx, u.d.Logger, id, byCLI, result.Sessions, err)
	}
	if err != nil {
		return DeactivateAccountResult{}, err
	}
	return result, nil
}
