// Package identity is the accounts module (M1 design): accounts, sessions
// and, from P3, personal access tokens. It brings registration, the
// caller's account, and the authentication every other operation goes
// through.
package identity

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	argon2adapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/argon2"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/authn"
	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/signing"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/ratelimit"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Deps are what bootstrap builds for the module.
type Deps struct {
	Pool   *pgxpool.Pool
	Tx     shared.TxManager
	Clock  app.Clock
	Logger *slog.Logger
	// SignupPolicy is auth.signup_enabled.
	SignupPolicy app.SignupPolicy
	// SigningKeyPEM is the content of auth.jwt.private_key_file; nil for
	// none, then the key is ephemeral (dev and test only).
	SigningKeyPEM   []byte
	AccessTokenTTL  time.Duration
	SessionTTL      time.Duration
	RefreshDeadline time.Duration // auth.refresh_deadline
	Password        PasswordHashing
	RateLimits      RateLimits
}

// PasswordHashing is auth.password: argon2id's parameters and the limits on
// concurrent hashes.
type PasswordHashing struct {
	MemoryKiB     uint32
	Iterations    uint32
	Parallelism   uint8
	MaxConcurrent int
	MaxWait       time.Duration
}

// RateLimits are the module's own buckets, on the limiter they belong to
// (M1/P2 design 3.2).
type RateLimits struct {
	Limiter      httpadapter.RateLimiter
	LoginIP      *ratelimit.Bucket
	LoginIPEmail *ratelimit.Bucket
	RegisterIP   *ratelimit.Bucket
}

// Module is the wired identity module.
type Module struct {
	uc            httpadapter.UseCases
	settings      httpadapter.Settings
	authenticator *authn.Authenticator
}

// New wires the module. A signing key that cannot be parsed is an error
// that never quotes the key.
func New(d Deps) (*Module, error) {
	keys, err := signingKeys(d)
	if err != nil {
		return nil, err
	}
	hasher := argon2adapter.New(argon2adapter.Params(d.Password), d.Logger)
	// Login verifies an unknown address against this, so that it takes as
	// long as a known one (M1/P2 design 3.4).
	dummy, err := hasher.Hash(context.Background(), rand.Text())
	if err != nil {
		return nil, fmt.Errorf("hash the dummy password: %w", err)
	}
	store := postgresadapter.New(d.Pool)
	tokens := signing.NewAccessTokens(keys)
	issuance := app.Issuance{
		Tokens:     tokens,
		MAC:        signing.NewRefreshTokenMAC(keys),
		AccessTTL:  d.AccessTokenTTL,
		SessionTTL: d.SessionTTL,
	}
	return &Module{
		uc: httpadapter.UseCases{
			Register: app.NewRegister(app.RegisterDeps{
				Policy: d.SignupPolicy, Rules: domain.NewPasswordRules(), Hasher: hasher, Tx: d.Tx,
				Users: store, Sessions: store, Issuance: issuance, Clock: d.Clock, Logger: d.Logger,
			}),
			Login: app.NewLogin(app.LoginDeps{
				Accounts: store, Locker: store, Passwords: store, Sessions: store, Verifier: hasher, Hasher: hasher, Tx: d.Tx,
				Issuance: issuance, Clock: d.Clock, Logger: d.Logger, DummyHash: dummy,
			}),
			Refresh: app.NewRefresh(app.RefreshDeps{Sessions: store, Tx: d.Tx, Issuance: issuance, Clock: d.Clock, Logger: d.Logger}),
			Logout:  app.NewLogout(store, d.Clock, d.Logger),
			GetMe:   app.NewGetMe(store),
		},
		settings: httpadapter.Settings{
			Limits: httpadapter.Limits{
				Limiter:      d.RateLimits.Limiter,
				LoginIP:      d.RateLimits.LoginIP,
				LoginIPEmail: d.RateLimits.LoginIPEmail,
				RegisterIP:   d.RateLimits.RegisterIP,
			},
			RefreshDeadline: d.RefreshDeadline,
			Logger:          d.Logger,
		},
		authenticator: authn.New(app.NewAuthenticate(app.AuthenticateDeps{
			AccessTokens: tokens, Sessions: store, Clock: d.Clock,
		})),
	}, nil
}

func signingKeys(d Deps) (*signing.Keys, error) {
	if d.SigningKeyPEM == nil {
		d.Logger.Warn("auth.jwt.private_key_file is not set: signing with an ephemeral key; " +
			"access tokens stop verifying at restart (dev and test only)")
		return signing.EphemeralKeys(), nil
	}
	keys, err := signing.ParseKeys(d.SigningKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("auth.jwt.private_key_file: %w", err)
	}
	return keys, nil
}

// PublicOperations are the module's routes that need no token.
func (m *Module) PublicOperations() []string {
	return httpadapter.PublicOperations()
}

// Authenticator checks the bearer token of every non-public operation.
func (m *Module) Authenticator() httpserver.Authenticator {
	return m.authenticator
}

// Register mounts the module's API on router behind api's middlewares.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc, m.settings)
}
