package app

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// RefreshDeps are Refresh's collaborators and settings.
type RefreshDeps struct {
	Sessions SessionRotator
	Tx       shared.TxManager
	Issuance Issuance
	Clock    Clock
	Logger   *slog.Logger
}

// Refresh exchanges a refresh token for the next pair: POST
// /api/v0/auth/refresh (M1/P2 design 3.5).
type Refresh struct {
	d RefreshDeps
}

// NewRefresh returns the use case.
func NewRefresh(d RefreshDeps) *Refresh {
	return &Refresh{d: d}
}

// errRotatedTwice means the conditional rotation missed after a re-read
// that judged it possible: the session cannot move back to a generation,
// so this is a fault, not an answer.
var errRotatedTwice = errors.New("refresh: the session changed under the rotation twice")

// Execute rotates the session of token and returns the next generation's
// tokens. Every other outcome is 401 identity.refresh_token_invalid; only an
// older generation that the session did issue revokes the session first
// (reuse), with a warning naming the account, the session and ip.
//
// Parsing looks nothing up. One transaction reads the session by its id,
// judges the token (domain.JudgeRefresh), and rotates with a conditional
// UPDATE. When the UPDATE misses, a concurrent refresh or revocation got
// there first: the row is read and judged again in the same transaction.
// The revocation of a reuse commits before the 401 goes out.
func (r *Refresh) Execute(ctx context.Context, token string, ip netip.Addr) (Tokens, error) {
	presented, ok := domain.ParseRefreshToken(token)
	if !ok {
		return Tokens{}, domain.ErrRefreshTokenInvalid
	}
	now := r.d.Clock.Now()
	tagValid := r.d.Issuance.MAC.Verify(presented.MACMessage(), presented.Tag)
	next := r.d.Issuance.refreshToken(presented.SessionID, presented.Generation+1)
	current := SessionGeneration{ID: presented.SessionID, Generation: presented.Generation, TokenHash: presented.SecretHash(), Now: now}

	var verdict domain.Verdict
	var userID uuid.UUID
	var tokens Tokens
	err := r.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		for range 2 {
			s, err := r.d.Sessions.SessionForRefresh(ctx, presented.SessionID)
			if errors.Is(err, ErrNotFound) {
				verdict = domain.Reject
				return nil
			}
			if err != nil {
				return err
			}
			userID = s.UserID
			switch verdict = domain.JudgeRefresh(s.State, presented, tagValid, now); verdict {
			case domain.Reject:
				return nil
			case domain.Reuse:
				return r.d.Sessions.RevokeForReuse(ctx, presented.SessionID, now)
			}
			if tokens, err = r.d.Issuance.tokens(s.UserID, next, now, s.State.ExpiresAt); err != nil {
				return err
			}
			rotated, err := r.d.Sessions.RotateSession(ctx, current, next.SecretHash())
			if err != nil || rotated {
				return err
			}
		}
		return errRotatedTwice
	})
	switch {
	case err != nil:
		return Tokens{}, err
	case verdict == domain.Rotate:
		return tokens, nil
	case verdict == domain.Reuse:
		r.d.Logger.WarnContext(ctx, "refresh token reused: session revoked",
			slog.String("user_id", userID.String()), slog.String("session_id", presented.SessionID.String()), slog.String("ip", ip.String()))
	}
	return Tokens{}, domain.ErrRefreshTokenInvalid
}
