// Package bootstrap is nervewiki's only composition root: it builds every
// adapter from the configuration, wires them together and runs the commands.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/ratelimit"
	"github.com/open-nerve/NerveWiki/server/internal/platform/webui"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The platform and the shared kernel meet here by structure: neither imports
// the other.
var (
	_ shared.TxManager        = (*postgres.TxManager)(nil)
	_ httpserver.ProblemError = (*shared.Error)(nil)
)

// poolCloseTimeout bounds the wait for the pool's connections at shutdown:
// a handler that ignores its context may still hold one.
const poolCloseTimeout = 5 * time.Second

// databaseWait bounds how long startup waits for the database to answer. A
// host that drops packets would otherwise hold serve, or a migrate command,
// until the operating system gives up on the TCP connection, minutes later.
const databaseWait = 10 * time.Second

// app is a fully wired nervewiki server.
type app struct {
	cfg      config.Config
	logger   *slog.Logger
	pool     *pgxpool.Pool
	migrator *postgres.Migrator
	router   *httpserver.Router
	// poolCloseTimeout and databaseWait are the package's, but for tests.
	poolCloseTimeout time.Duration
	databaseWait     time.Duration
}

// newApp wires the server described by cfg around the given migrations and
// web frontend: the platform routes, each module's API behind the per-route
// middlewares, and the frontend on every other path. close releases it.
func newApp(ctx context.Context, cfg config.Config, logger *slog.Logger, migrationFiles, webFiles fs.FS) (*app, error) {
	signingKey, err := readSigningKey(cfg.Auth.JWT.PrivateKeyFile)
	if err != nil {
		return nil, err
	}
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}
	// The configuration log masks database.url as a whole; say where the pool
	// connects from pgx's own parse of it, which holds no password.
	target := pool.Config().ConnConfig
	logger.InfoContext(ctx, "database pool created",
		slog.String("host", target.Host), slog.Int("port", int(target.Port)),
		slog.String("database", target.Database), slog.String("user", target.User))
	migrator, err := postgres.NewMigrator(pool, migrationFiles)
	if err != nil {
		pool.Close()
		return nil, err
	}
	ident, err := identity.New(identity.Deps{
		Pool:           pool,
		Tx:             postgres.NewTxManager(pool, cfg.Database.CommitTimeout),
		Clock:          clock.System{},
		Logger:         logger,
		SignupPolicy:   signupSwitch(cfg.Auth.SignupEnabled),
		SigningKeyPEM:  signingKey,
		AccessTokenTTL: cfg.Auth.AccessTokenTTL,
		SessionTTL:     cfg.Auth.SessionTTL,
		Password:       passwordHashing(cfg.Auth.Password),
	})
	if err != nil {
		_ = migrator.Close()
		pool.Close()
		return nil, err
	}
	inst := instance.New(instance.Deps{SignupEnabled: cfg.Auth.SignupEnabled})
	// One limiter holds every bucket (M1/P2 design 3.2). It reads the
	// monotonic clock, which a jump of the wall clock does not move.
	limiter := ratelimit.New(time.Now)
	limits := cfg.RateLimit
	api, err := httpserver.NewAPI(httpserver.APIConfig{
		Logger:           logger,
		Authenticator:    ident.Authenticator(),
		PublicOperations: slices.Concat(ident.PublicOperations(), inst.PublicOperations()),
		MaxBodyBytes:     cfg.Server.MaxBodyBytes,
		RequestTimeout:   cfg.Server.RequestTimeout,
		TrustedProxies:   cfg.Server.TrustedProxies,
		IPv6PrefixLen:    limits.IPv6PrefixLen,
		Anonymous:        bucket(limiter, "anonymous", limits.Anonymous),
		Authenticated:    bucket(limiter, "authenticated", limits.Authenticated),
		AuthFailure:      bucket(limiter, "auth_failure", limits.AuthFailure),
	})
	if err != nil {
		_ = migrator.Close()
		pool.Close()
		return nil, err
	}
	router := httpserver.NewRouter(logger,
		httpserver.Check{Name: "database", Run: pool.Ping},
		httpserver.Check{Name: "migrations", Run: migrator.CheckUpToDate},
	)
	ident.Register(router, api)
	inst.Register(router, api)
	// "/" without a method is the least specific pattern: /api/ and the
	// probes keep their routes, and a wrong method on a page path gets the
	// frontend's 405 rather than a 404.
	router.Handle("/", webui.Handler(webFiles))
	return &app{
		cfg:              cfg,
		logger:           logger,
		pool:             pool,
		migrator:         migrator,
		router:           router,
		poolCloseTimeout: poolCloseTimeout,
		databaseWait:     databaseWait,
	}, nil
}

// run waits for the database, applies pending migrations when
// database.auto_migrate is on, checks the database, and serves HTTP on
// server.addr until ctx is done. HTTP stops first (see
// httpserver.Server.Serve); close then releases the migrator and the pool.
func (a *app) run(ctx context.Context) error {
	warnIfExposed(ctx, a.logger, a.cfg)
	if err := awaitDatabase(ctx, a.pool, a.databaseWait); err != nil {
		return err
	}
	if a.cfg.Database.AutoMigrate {
		applied, err := a.migrator.Up(ctx)
		// Log what was applied even when a later migration failed.
		for _, m := range applied {
			a.logger.InfoContext(ctx, "migration applied", slog.Int64("version", m.Version), slog.String("source", m.Source))
		}
		if err != nil {
			return err
		}
	}
	if err := postgres.CheckDatabase(ctx, a.pool); err != nil {
		return err
	}
	return httpserver.NewServer(a.cfg.Server, a.router, a.logger).ListenAndServe(ctx)
}

// readSigningKey returns the content of auth.jwt.private_key_file, or nil
// when it is not set. Errors name the key, never the path or the content.
func readSigningKey(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err
		}
		return nil, fmt.Errorf("auth.jwt.private_key_file: %w", err)
	}
	return data, nil
}

// passwordHashing is auth.password as identity takes it.
func passwordHashing(p config.PasswordConfig) identity.PasswordHashing {
	return identity.PasswordHashing{
		MemoryKiB:     p.Argon2MemoryKiB,
		Iterations:    p.Argon2Iterations,
		Parallelism:   p.Argon2Parallelism,
		MaxConcurrent: p.MaxConcurrentHashes,
		MaxWait:       p.MaxWait,
	}
}

// bucket is limiter's bucket of the configured rate, named after its key.
func bucket(limiter *ratelimit.Limiter, name string, c config.BucketConfig) *ratelimit.Bucket {
	return limiter.Bucket(name, ratelimit.Rate{PerMinute: c.PerMinute, Burst: c.Burst})
}

// signupSwitch is auth.signup_enabled as identity's SignupPolicy.
type signupSwitch bool

func (s signupSwitch) AllowSignup(context.Context) (bool, error) { return bool(s), nil }

// warnIfExposed warns once when a non-prod nervewiki listens beyond
// loopback (M1/P1 design 3.6): most likely a deployment without
// NWIKI_ENV=prod, with sign-up open and, without a key file, an ephemeral
// signing key.
func warnIfExposed(ctx context.Context, logger *slog.Logger, cfg config.Config) {
	if cfg.Env == config.EnvProd || loopback(cfg.Server.Addr) {
		return
	}
	logger.WarnContext(ctx, "not running as prod but listening beyond this machine; set NWIKI_ENV=prod to deploy",
		slog.String("env", cfg.Env), slog.String("addr", cfg.Server.Addr),
		slog.Bool("signup_enabled", cfg.Auth.SignupEnabled),
		slog.Bool("ephemeral_signing_key", cfg.Auth.JWT.PrivateKeyFile == ""))
}

func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

// awaitDatabase fails when the database does not answer within wait.
func awaitDatabase(ctx context.Context, pool *pgxpool.Pool, wait time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database unreachable (waited up to %s): %w", wait, err)
	}
	return nil
}

// close releases the database resources: the migrator, then the pool, whose
// wait for connections still in use is bounded. Call it after run has
// returned.
func (a *app) close() {
	if err := a.migrator.Close(); err != nil {
		a.logger.Warn("close migrator", slog.Any("error", err))
	}
	closed := make(chan struct{})
	go func() {
		a.pool.Close()
		close(closed)
	}()
	select {
	case <-closed:
		a.logger.Info("database pool closed")
	case <-time.After(a.poolCloseTimeout):
		a.logger.Warn("database pool not closed: connections still in use", slog.Duration("waited", a.poolCloseTimeout))
	}
}
