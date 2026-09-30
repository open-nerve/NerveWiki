// Package bootstrap is nervewiki's only composition root: it builds every
// adapter from the configuration, wires them together and runs the commands.
package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/instance"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
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
	api, err := httpserver.NewAPI(httpserver.APIConfig{
		Logger:         logger,
		MaxBodyBytes:   cfg.Server.MaxBodyBytes,
		RequestTimeout: cfg.Server.RequestTimeout,
	})
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
	router := httpserver.NewRouter(logger,
		httpserver.Check{Name: "database", Run: pool.Ping},
		httpserver.Check{Name: "migrations", Run: migrator.CheckUpToDate},
	)
	instance.New().Register(router, api)
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
