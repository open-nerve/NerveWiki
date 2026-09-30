// Package app holds the identity module's use cases, one per file, and the
// ports they need (v0.1 design 8.1). Each use case depends on the few methods
// it calls; the postgres adapter's Store satisfies every repository port.
package app

import (
	"context"
	"errors"
	"net/netip"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// ErrNotFound is what a repository returns for a missing row.
var ErrNotFound = errors.New("not found")

// Clock tells the time: every business time comes from it and goes to SQL
// as a parameter (M1/P1 design 3.7). platform/clock implements it.
type Clock interface {
	Now() time.Time
}

// SignupPolicy decides whether registration is open (M1/P1 design 3.7).
// From bootstrap it is auth.signup_enabled; M2 extends it to invitations.
type SignupPolicy interface {
	AllowSignup(ctx context.Context) (bool, error)
}

// NewUser is an account to insert. Its audit columns are Now.
type NewUser struct {
	ID           uuid.UUID
	Email        string // normalized
	PasswordHash string // argon2id PHC string
	DisplayName  string
	Now          time.Time
}

// UserCreator inserts accounts.
type UserCreator interface {
	// CreateUser returns domain.ErrEmailTaken when the address is in use.
	CreateUser(ctx context.Context, u NewUser) error
}

// UserReader reads accounts.
type UserReader interface {
	// GetUser returns ErrNotFound when there is no such account.
	GetUser(ctx context.Context, id uuid.UUID) (domain.User, error)
}

// NewSession is a sign-in to insert, at generation 0.
type NewSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte // SHA-256 of the refresh token's secret
	UserAgent string
	IP        netip.Addr // the zero Addr when unknown
	ExpiresAt time.Time
	Now       time.Time
}

// SessionCreator inserts sessions.
type SessionCreator interface {
	CreateSession(ctx context.Context, s NewSession) error
}

// SessionCredential is what authentication checks of a session.
type SessionCredential struct {
	UserID     uuid.UUID
	ExpiresAt  time.Time
	Revoked    bool
	UserActive bool
}

// SessionReader reads what authentication needs, by primary key.
type SessionReader interface {
	// SessionCredential returns ErrNotFound when there is no such session.
	SessionCredential(ctx context.Context, id uuid.UUID) (SessionCredential, error)
}

// PasswordHasher hashes and verifies passwords with argon2id. Both return a
// *shared.Error of 503 server_busy when no slot frees up within the wait
// limit (M1/P1 design 3.4).
type PasswordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
	// Verify reports whether password matches hash, and whether hash has
	// other parameters than the current ones: then login hashes the
	// password again.
	Verify(ctx context.Context, password, hash string) (ok, rehash bool, err error)
}

// AccessClaims are the claims of an access token: nothing about permissions
// (M1/P1 design 3.4).
type AccessClaims struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	ExpiresAt time.Time
}

// ErrAccessTokenExpired is what AccessTokens.Verify returns for a token
// whose signature is valid but whose exp has passed: the client's cue to
// refresh. Any other failure is a different error.
var ErrAccessTokenExpired = errors.New("access token expired")

// AccessTokens signs and verifies access tokens (JWT, EdDSA).
type AccessTokens interface {
	Issue(c AccessClaims) (string, error)
	// Verify checks the token at now; ErrAccessTokenExpired when only the
	// expiry fails.
	Verify(token string, now time.Time) (AccessClaims, error)
}

// RefreshTokenMAC tags refresh tokens (M1/P1 design 3.4): the first 16 bytes
// of HMAC-SHA256 under a key derived from the signing key.
type RefreshTokenMAC interface {
	Tag(message []byte) [16]byte
	// Verify reports whether tag is message's tag, in constant time.
	Verify(message []byte, tag [16]byte) bool
}
