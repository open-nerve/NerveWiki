package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/access"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
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

// newApp wires the server described by cfg around the given migrations and
// web frontend: the platform routes, each module's API behind the per-route
// middlewares, and the frontend on every other path. close releases it.
func newApp(ctx context.Context, cfg config.Config, logger *slog.Logger, migrationFiles, webFiles fs.FS) (_ *app, err error) {
	signingKey, err := readSigningKey(cfg.Auth.JWT.PrivateKeyFile)
	if err != nil {
		return nil, err
	}
	keys, err := identity.LoadSigningKeys(signingKey, logger)
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
	// From here on, whatever fails releases both.
	defer func() {
		if err != nil {
			_ = migrator.Close()
			pool.Close()
		}
	}()
	// One limiter holds every bucket (M1/P2 design 3.2). It reads the
	// monotonic clock, which a jump of the wall clock does not move.
	limiter := ratelimit.New(time.Now)
	// Sign-up opens to an invitation of the workspace module, whose MAC key
	// derives from identity's signing key: both come before either module.
	invitationKey := keys.Derive(workspace.InvitationKeyInfo)
	signup := signupPolicy{open: cfg.Auth.SignupEnabled, invitations: workspace.NewInvitationCheck(pool, invitationKey)}
	ident, err := identity.New(identityDeps(cfg, pool, logger, limiter, keys, signup))
	if err != nil {
		return nil, err
	}
	inst := instance.New(instanceDeps(cfg))
	// The access module decides on the facts the workspace and notebook
	// modules keep; their use cases call its decisions.
	authorizer := access.New(access.Deps{
		Memberships: workspace.NewMemberships(pool),
		Notebooks:   notebookFacts{notebook.NewFacts(pool)},
	})
	ws := workspace.New(workspaceDeps(cfg, pool, logger, authorizer, invitationKey))
	nb := notebook.New(notebookDeps(cfg, pool, logger, authorizer))
	// One parse and rendering of Markdown with the registered extensions:
	// the page module's reading view and, from M6, the links (M4 design 8).
	md, err := markdown.New(markdownExtensions())
	if err != nil {
		return nil, err
	}
	pg := page.New(pageDeps(cfg, pool, logger, authorizer, md))
	ev, listener := eventsModule(cfg, pool, logger)
	runner, err := jobs.New(pool, jobs.Config{ShutdownTimeout: cfg.Jobs.ShutdownTimeout, Logger: logger},
		slices.Concat(ident.Jobs(), pg.Jobs(), []jobs.Job{purgeJob(cfg, pool, logger)}))
	if err != nil {
		return nil, err
	}
	api, err := httpserver.NewAPI(apiConfig(cfg, logger, limiter, ident.Authenticator(),
		slices.Concat(ident.PublicOperations(), inst.PublicOperations(), ws.PublicOperations()), ident.RequestTimeouts(),
		pg.BodyLimits()))
	if err != nil {
		return nil, err
	}
	router := httpserver.NewRouter(logger,
		httpserver.Check{Name: "database", Run: pool.Ping},
		httpserver.Check{Name: "migrations", Run: migrator.CheckUpToDate},
	)
	ident.Register(router, api)
	inst.Register(router, api)
	ws.Register(router, api)
	nb.Register(router, api)
	pg.Register(router, api)
	ev.Register(router, api)
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
		jobs:             runner,
		listener:         listener,
		poolCloseTimeout: poolCloseTimeout,
		databaseWait:     databaseWait,
		migrationPoll:    migrationPoll,
	}, nil
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
