package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// RevokeAPIToken revokes one of the caller's tokens:
// DELETE /api/v0/api-tokens/{token_id}. It is one statement, no
// transaction: revoking only takes access away.
type RevokeAPIToken struct {
	tokens APITokenRevoker
	clock  Clock
	logger *slog.Logger
}

// NewRevokeAPIToken returns the use case.
func NewRevokeAPIToken(tokens APITokenRevoker, clock Clock, logger *slog.Logger) *RevokeAPIToken {
	return &RevokeAPIToken{tokens: tokens, clock: clock, logger: logger}
}

// Execute revokes token id of the caller. A token that does not exist, is
// revoked already or is another account's is identity.api_token_not_found.
// The token the request came with may revoke itself.
func (r *RevokeAPIToken) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	revoked, err := r.tokens.RevokeAPIToken(ctx, id, actor.UserID, r.clock.Now())
	switch {
	case err != nil:
		return err
	case !revoked:
		return domain.ErrAPITokenNotFound
	}
	r.logger.InfoContext(ctx, "API token revoked", slog.String("user_id", actor.UserID.String()), slog.String("token_id", id.String()))
	return nil
}
