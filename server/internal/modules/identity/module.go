// Package identity is the accounts module (M1 design): accounts, sessions
// and, from P3, personal access tokens. It brings registration, the
// caller's account, and the authentication every other operation goes
// through.
package identity

import (
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
	SigningKeyPEM  []byte
	AccessTokenTTL time.Duration
	SessionTTL     time.Duration
	Password       PasswordHashing
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

// Module is the wired identity module.
type Module struct {
	uc            httpadapter.UseCases
	authenticator *authn.Authenticator
}

// New wires the module. A signing key that cannot be parsed is an error
// that never quotes the key.
func New(d Deps) (*Module, error) {
	keys, err := signingKeys(d)
	if err != nil {
		return nil, err
	}
	store := postgresadapter.New(d.Pool)
	tokens := signing.NewAccessTokens(keys)
	return &Module{
		uc: httpadapter.UseCases{
			Register: app.NewRegister(app.RegisterDeps{
				Policy:   d.SignupPolicy,
				Rules:    domain.NewPasswordRules(),
				Hasher:   argon2adapter.New(argon2adapter.Params(d.Password), d.Logger),
				Tx:       d.Tx,
				Users:    store,
				Sessions: store,
				Issuance: app.Issuance{
					Tokens:     tokens,
					MAC:        signing.NewRefreshTokenMAC(keys),
					AccessTTL:  d.AccessTokenTTL,
					SessionTTL: d.SessionTTL,
				},
				Clock:  d.Clock,
				Logger: d.Logger,
			}),
			GetMe: app.NewGetMe(store),
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
	httpadapter.Register(router, api, m.uc)
}
