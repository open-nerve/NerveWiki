package postgresadapter

import (
	"context"
	"fmt"
	"net/netip"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
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
