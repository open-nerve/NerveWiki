package app

import (
	"context"
	"time"
	"uuid"
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
