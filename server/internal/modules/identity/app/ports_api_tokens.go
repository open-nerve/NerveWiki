package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// The ports of personal access tokens (M1/P3 design 3.2).

// NewAPIToken is a personal access token to insert, at Now.
type NewAPIToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte // domain.PAT.Hash
	Name      string
	ExpiresAt *time.Time
	Now       time.Time
}

// APITokenCreator inserts personal access tokens.
type APITokenCreator interface {
	CreateAPIToken(ctx context.Context, t NewAPIToken) error
}

// APITokenLister reads an account's tokens.
type APITokenLister interface {
	// ListAPITokens returns userID's unrevoked tokens, newest first and
	// then by id.
	ListAPITokens(ctx context.Context, userID uuid.UUID) ([]domain.APIToken, error)
}

// APITokenRevoker revokes personal access tokens.
type APITokenRevoker interface {
	// RevokeAPIToken revokes token id of userID at now; false when userID
	// has no such unrevoked token.
	RevokeAPIToken(ctx context.Context, id, userID uuid.UUID, now time.Time) (bool, error)
}

// APITokenCredential is what authentication and the credential lock check
// of a personal access token.
type APITokenCredential struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	ExpiresAt  *time.Time // nil: never expires
	LastUsedAt *time.Time
	Revoked    bool
	UserActive bool
}

// APITokenReader reads what authentication and the credential lock need.
type APITokenReader interface {
	// APITokenByHash returns ErrNotFound when no token has hash.
	APITokenByHash(ctx context.Context, hash []byte) (APITokenCredential, error)
	// APITokenByID returns ErrNotFound when there is no token id.
	APITokenByID(ctx context.Context, id uuid.UUID) (APITokenCredential, error)
}

// APITokenToucher records that a token was used.
type APITokenToucher interface {
	// TouchAPIToken sets token id's last_used_at to now when it is unset or
	// older than staleBefore.
	TouchAPIToken(ctx context.Context, id uuid.UUID, now, staleBefore time.Time) error
}
