package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DeactivateDeps are Deactivate's collaborators.
type DeactivateDeps struct {
	Lock        CredentialLock
	Users       UserDeactivator
	Sessions    SessionRevoker
	Vetoers     []DeactivationVetoer
	Subscribers []DeactivationSubscriber
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
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

// Execute takes the credential lock, then deactivates as deactivate does,
// in one transaction.
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
		revoked, err = u.deactivate(ctx, Deactivation{UserID: actor.UserID, Email: locked.Email, At: now})
		return err
	})
	var refused *shared.Error
	switch {
	case errors.As(err, &refused) && refused.Kind != shared.KindUnauthenticated:
		u.d.Logger.InfoContext(ctx, "account deactivation refused", slog.String("user_id", actor.UserID.String()),
			slog.String("code", refused.Code))
		return err
	case err != nil:
		return err
	}
	u.d.Logger.InfoContext(ctx, "account deactivated", slog.String("user_id", actor.UserID.String()),
		slog.Int("revoked_sessions", revoked), slog.String("by", "self"))
	return nil
}

// deactivate is the deactivation under the account row lock, the
// administrator's command's too (P4): the vetoers in order, the first
// refusal ending it; the account inactive and every session revoked with
// reason deactivated; the subscribers in order. It returns how many
// sessions it revoked. The personal access tokens and the onboarding steps
// stay: authentication refuses the tokens while the account is inactive.
func (u *Deactivate) deactivate(ctx context.Context, d Deactivation) (int, error) {
	for _, v := range u.d.Vetoers {
		if err := v.VetoDeactivation(ctx, d); err != nil {
			return 0, err
		}
	}
	if err := u.d.Users.DeactivateUser(ctx, d.UserID, d.At); err != nil {
		return 0, err
	}
	revoked, err := u.d.Sessions.RevokeSessions(ctx, d.UserID, uuid.Nil(), domain.RevokeDeactivated, d.At)
	if err != nil {
		return 0, err
	}
	for _, s := range u.d.Subscribers {
		if err := s.AccountDeactivated(ctx, d); err != nil {
			return 0, err
		}
	}
	return revoked, nil
}
