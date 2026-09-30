package postgresadapter

import (
	"context"
	"fmt"
	"net/netip"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// CreateSession inserts a sign-in at generation 0. An unknown client IP is
// stored as NULL.
func (s *Store) CreateSession(ctx context.Context, n app.NewSession) error {
	var ip *netip.Addr
	if n.IP.IsValid() {
		ip = &n.IP
	}
	err := s.queries(ctx).CreateSession(ctx, gen.CreateSessionParams{
		ID: n.ID, UserID: n.UserID, TokenHash: n.TokenHash, UserAgent: n.UserAgent, Ip: ip, ExpiresAt: n.ExpiresAt, Now: n.Now,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionCredential reads what authentication checks of session id;
// app.ErrNotFound when there is none.
func (s *Store) SessionCredential(ctx context.Context, id uuid.UUID) (app.SessionCredential, error) {
	row, err := s.queries(ctx).GetSessionCredential(ctx, id)
	if err != nil {
		return app.SessionCredential{}, notFound(err)
	}
	return app.SessionCredential{
		UserID:     row.UserID,
		ExpiresAt:  row.ExpiresAt,
		Revoked:    row.RevokedAt != nil,
		UserActive: row.UserActive,
	}, nil
}

// SessionForRefresh reads what a refresh judges of session id;
// app.ErrNotFound when there is none.
func (s *Store) SessionForRefresh(ctx context.Context, id uuid.UUID) (app.RefreshSession, error) {
	row, err := s.queries(ctx).GetSessionForRefresh(ctx, id)
	if err != nil {
		return app.RefreshSession{}, notFound(err)
	}
	return app.RefreshSession{UserID: row.UserID, State: domain.SessionState{
		Generation: uint32(row.Generation), // CHECK (generation >= 0)
		TokenHash:  row.TokenHash,
		Revoked:    row.RevokedAt != nil,
		ExpiresAt:  row.ExpiresAt,
	}}, nil
}

// RotateSession moves the session from g to the next generation with
// newHash, stamping last_refreshed_at; false when it is no longer at g.
// g.Generation fits the column's integer: domain.ParseRefreshToken bounds
// it.
func (s *Store) RotateSession(ctx context.Context, g app.SessionGeneration, newHash []byte) (bool, error) {
	n, err := s.queries(ctx).RotateSession(ctx, gen.RotateSessionParams{
		Now: g.Now, NewTokenHash: newHash, ID: g.ID, Generation: int32(g.Generation), TokenHash: g.TokenHash,
	})
	if err != nil {
		return false, fmt.Errorf("rotate session: %w", err)
	}
	return n == 1, nil
}

// RevokeForReuse revokes session id with reason reuse_detected, unless it
// is revoked already.
func (s *Store) RevokeForReuse(ctx context.Context, id uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).RevokeSessionForReuse(ctx, gen.RevokeSessionForReuseParams{Now: now, ID: id}); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// EndSession revokes the session with reason logout while it is at g;
// false when it is not.
func (s *Store) EndSession(ctx context.Context, g app.SessionGeneration) (bool, error) {
	n, err := s.queries(ctx).EndSession(ctx, gen.EndSessionParams{
		Now: g.Now, ID: g.ID, Generation: int32(g.Generation), TokenHash: g.TokenHash,
	})
	if err != nil {
		return false, fmt.Errorf("end session: %w", err)
	}
	return n == 1, nil
}

// RevokeSessions revokes at now, with reason, every live session of userID
// but keep, and returns how many.
func (s *Store) RevokeSessions(ctx context.Context, userID, keep uuid.UUID, reason domain.RevokeReason, now time.Time) (int, error) {
	n, err := s.queries(ctx).RevokeSessions(ctx, gen.RevokeSessionsParams{Now: now, Reason: string(reason), UserID: userID, Keep: keep})
	if err != nil {
		return 0, fmt.Errorf("revoke sessions: %w", err)
	}
	return int(n), nil
}
