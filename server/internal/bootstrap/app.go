// Package bootstrap is nervewiki's only composition root: it builds every
// adapter from the configuration, wires them together and runs the commands.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// poolCloseTimeout bounds the wait for the pool's connections at shutdown:
// a handler that ignores its context may still hold one.
const poolCloseTimeout = 5 * time.Second

// databaseWait bounds how long startup waits for the database to answer. A
// host that drops packets would otherwise hold serve, or a migrate command,
// until the operating system gives up on the TCP connection, minutes later.
const databaseWait = 10 * time.Second

// migrationPoll is how often serve looks, while migrations are pending,
// whether they have been applied, to start the background jobs.
const migrationPoll = 2 * time.Second

// app is a fully wired nervewiki server.
type app struct {
	cfg      config.Config
	logger   *slog.Logger
	pool     *pgxpool.Pool
	migrator *postgres.Migrator
	router   *httpserver.Router
	jobs     *jobs.Runner
	// poolCloseTimeout, databaseWait and migrationPoll are the package's, but
	// for tests.
	poolCloseTimeout time.Duration
	databaseWait     time.Duration
	migrationPoll    time.Duration
}

// run waits for the database, applies pending migrations when
// database.auto_migrate is on, checks the database, and serves HTTP on
// server.addr until ctx is done, with the background jobs once no migration
// is pending (M1/P4 design 3.4). HTTP stops first (see
// httpserver.Server.Serve), its requests done, then the jobs; close then
// releases the migrator and the pool.
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
	// A job that fails to start stops the server, with its error.
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	started := make(chan error, 1)
	go func() {
		err := a.startJobs(ctx)
		if err != nil {
			cancel(err)
		}
		started <- err
	}()
	serveErr := httpserver.NewServer(a.cfg.Server, a.router, a.logger).ListenAndServe(ctx)
	cancel(nil) // HTTP may have stopped on its own: stop waiting to start the jobs
	startErr := <-started
	return errors.Join(serveErr, startErr, a.jobs.Stop(context.WithoutCancel(ctx)))
}

// startJobs starts the background jobs once the migration check passes:
// with database.auto_migrate off, serve starts on a database behind the
// migrations and waits, not ready, for the operator to apply them (M0), and
// River needs its tables. It looks every migrationPoll, warns once with the
// reason the check fails (pending migrations, or a role that cannot read
// goose's record yet), and returns nil when ctx ends first, the jobs never
// started.
func (a *app) startJobs(ctx context.Context) error {
	for waiting := false; ; waiting = true {
		err := a.migrator.CheckUpToDate(ctx)
		if err == nil {
			if err := a.jobs.Start(ctx); err != nil && ctx.Err() == nil {
				return err
			}
			return nil
		}
		if ctx.Err() != nil {
			return nil
		}
		if !waiting {
			a.logger.WarnContext(ctx, "background jobs wait for the migration check to pass", slog.Any("reason", err))
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(a.migrationPoll):
		}
	}
}

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
