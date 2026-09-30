// Package identity is the accounts module (M1 design): accounts, sessions
// and personal access tokens. It brings registration, sign-in, refresh and
// sign-out, the caller's account and tokens, the authentication every other
// operation goes through, and the periodic cleanup of expired sessions.
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
	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/signing"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
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
	// SessionCleanupInterval is auth.session_cleanup_interval: how often the
	// expired sessions are deleted.
	SessionCleanupInterval time.Duration
	Password               PasswordHashing
	RateLimits             RateLimits
	// The registrants of the deactivation's extension point (M1 design 8),
	// built from the pool alone; M2 brings the first.
	DeactivationVetoers     []DeactivationVetoer
	DeactivationSubscribers []DeactivationSubscriber
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
	PasswordUser *ratelimit.Bucket
}

// Module is the wired identity module.
type Module struct {
	uc              httpadapter.UseCases
	settings        httpadapter.Settings
	refreshDeadline time.Duration
	authenticator   *authn.Authenticator
	jobs            []jobs.Job
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
	rules := domain.NewPasswordRules()
	lock := app.CredentialLock{Locker: store, Sessions: store, APITokens: store}
	password := app.CurrentPassword{Accounts: store, Verifier: hasher, Lock: lock, Tx: d.Tx}
	return &Module{
		uc: httpadapter.UseCases{
			Register: app.NewRegister(app.RegisterDeps{
				Policy: d.SignupPolicy, Rules: rules, Hasher: hasher, Tx: d.Tx,
				Users: store, Sessions: store, Issuance: issuance, Clock: d.Clock, Logger: d.Logger,
			}),
			Login: app.NewLogin(app.LoginDeps{
				Accounts: store, Locker: store, Passwords: store, Sessions: store, Verifier: hasher, Hasher: hasher, Tx: d.Tx,
				Issuance: issuance, Clock: d.Clock, Logger: d.Logger, DummyHash: dummy,
			}),
			Refresh:              app.NewRefresh(app.RefreshDeps{Sessions: store, Tx: d.Tx, Issuance: issuance, Clock: d.Clock, Logger: d.Logger}),
			Logout:               app.NewLogout(store, d.Clock, d.Logger),
			GetMe:                app.NewGetMe(store),
			UpdateMe:             app.NewUpdateMe(store, store, d.Clock),
			RecordOnboardingStep: app.NewRecordOnboardingStep(store, d.Clock),
			ChangePassword: app.NewChangePassword(app.ChangePasswordDeps{
				Password: password, Rules: rules, Hasher: hasher, Passwords: store, Sessions: store, Clock: d.Clock, Logger: d.Logger,
			}),
			Deactivate: app.NewDeactivate(app.DeactivateDeps{
				Lock: lock, Users: store, Sessions: store, Vetoers: d.DeactivationVetoers,
				Subscribers: d.DeactivationSubscribers, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
			}),
			ListAPITokens: app.NewListAPITokens(store),
			CreateAPIToken: app.NewCreateAPIToken(app.CreateAPITokenDeps{
				Password: password, Tokens: store, Clock: d.Clock, Logger: d.Logger,
			}),
			RevokeAPIToken: app.NewRevokeAPIToken(store, d.Clock, d.Logger),
		},
		settings: httpadapter.Settings{
			Limits: httpadapter.Limits{
				Limiter:      d.RateLimits.Limiter,
				LoginIP:      d.RateLimits.LoginIP,
				LoginIPEmail: d.RateLimits.LoginIPEmail,
				RegisterIP:   d.RateLimits.RegisterIP,
				PasswordUser: d.RateLimits.PasswordUser,
			},
			Logger: d.Logger,
		},
		refreshDeadline: d.RefreshDeadline,
		authenticator: authn.New(app.NewAuthenticate(app.AuthenticateDeps{
			AccessTokens: tokens, Sessions: store, APITokens: store, Touch: store, Clock: d.Clock, Logger: d.Logger,
		})),
		jobs: []jobs.Job{
			riveradapter.CleanupJob(app.NewCleanupSessions(store, d.Clock, d.Logger), d.SessionCleanupInterval),
		},
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

// RequestTimeouts are the module's routes with a deadline of their own.
func (m *Module) RequestTimeouts() map[string]time.Duration {
	return httpadapter.RequestTimeouts(m.refreshDeadline)
}

// Authenticator checks the bearer token of every non-public operation.
func (m *Module) Authenticator() httpserver.Authenticator {
	return m.authenticator
}

// Jobs are the module's background jobs, for the server's jobs runner.
func (m *Module) Jobs() []jobs.Job {
	return m.jobs
}

// Register mounts the module's API on router behind api's middlewares.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc, m.settings)
}
