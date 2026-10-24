package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/access"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/ratelimit"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/internal/platform/webui"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The platform and the shared kernel meet here by structure: neither imports
// the other. So do mcpserver, the page module's MCP entry and identity's
// client names.
var (
	_ shared.TxManager        = (*postgres.TxManager)(nil)
	_ shared.Snapshots        = (*postgres.TxManager)(nil)
	_ httpserver.ProblemError = (*shared.Error)(nil)
	_ mcpserver.Notebooks     = page.MCPEntry{}
	_ mcpserver.ClientNames   = mcpNames{}
)

// newApp wires the server described by cfg around the given migrations and
// web frontend: the platform routes, each module's API behind the per-route
// middlewares, MCP on /mcp, and the frontend on every other path. close
// releases it. The writing jobs' batches are the import's (M8/P4 design
// 3.6), and the MCP tools the modules'.
func newApp(ctx context.Context, cfg config.Config, logger *slog.Logger, migrationFiles, webFiles fs.FS) (*app, error) {
	return wireApp(ctx, cfg, logger, migrationFiles, webFiles, wiring{})
}

// wiring is what a test may add to the app: a batch of its own for the
// transfer jobs, and MCP tools beside the modules'. The zero wiring is
// serve's.
type wiring struct {
	// batch bounds the writing jobs' batches, zero the import's: the
	// interleavings make them small, so that a few nodes span batches
	// (M8/P4 design 3.10).
	batch transfer.Batch
	// mcpTools are registered after the modules': the whole-program tests
	// reach the MCP platform through them before any module has a tool
	// (M9/P1 design 3.4.9).
	mcpTools []mcpserver.Tool
	// largeBodyWait, when set, is how long a large body waits for its
	// slot: the large bodies' whole-program tests wait less than
	// largeBodyWait (M9/P2 design 3.2.5).
	largeBodyWait time.Duration
	// page, when set, changes the page module's dependencies as the
	// composition root gives them: grep's whole-program tests bound its
	// walk small and hold its slot (M9/P4 design 6.1, 3.12).
	page func(*page.Deps)
}

// wireApp is newApp with a test's wiring.
func wireApp(ctx context.Context, cfg config.Config, logger *slog.Logger, migrationFiles, webFiles fs.FS, w wiring,
) (_ *app, err error) {
	signingKey, err := readSigningKey(cfg.Auth.JWT.PrivateKeyFile)
	if err != nil {
		return nil, err
	}
	keys, err := identity.LoadSigningKeys(signingKey, logger)
	if err != nil {
		return nil, err
	}
	// serve refuses to start on a storage directory it cannot write, naming
	// the directory and the uid (M7/P1 design 3.5). The command line does
	// not open it.
	store, err := storage.OpenLocal(cfg.Storage.Dir, cfg.Storage.MinFreeBytes)
	if err != nil {
		return nil, err
	}
	free, err := store.Free(ctx)
	if err != nil {
		return nil, err
	}
	logger.InfoContext(ctx, "storage opened", slog.String("dir", store.Dir()), slog.Int64("free_bytes", free))
	// Out of quota or of inodes, a disk may be full with free bytes left.
	if full := store.FullAtOpen(); full != nil || free < cfg.Storage.MinFreeBytes {
		attrs := []any{slog.String("dir", store.Dir()), slog.Int64("free_bytes", free), slog.Int64("min_free_bytes", cfg.Storage.MinFreeBytes)}
		if full != nil {
			attrs = append(attrs, slog.Any("error", full))
		}
		logger.WarnContext(ctx, "storage is full: writes are refused", attrs...)
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
	// A reading view shows the attachments its links lead to, at their
	// contents' addresses (M7/P3 design 5.10): the Markdown comes before
	// the modules, so Embeds needs the pool and the content key alone.
	contentKey := keys.Derive(asset.ContentKeyInfo)
	embeds := assetEmbeds{asset.NewEmbeds(pool, contentKey, clock.System{})}
	md, budget, err := parsing(cfg, logger, pool, embeds.assets)
	if err != nil {
		return nil, err
	}
	pd := pageDeps(cfg, pool, logger, authorizer, md, budget)
	if w.page != nil {
		w.page(&pd)
	}
	pg := page.New(pd)
	as := asset.New(assetDeps(cfg, pool, logger, authorizer, store, pg, contentKey, limiter))
	ln := linking.New(linkingDeps(pool, authorizer, embeds))
	// The LLM Wiki's template is parsed once, here: a template the page
	// module refuses keeps serve from starting (M10/P1 design 3.8).
	template, err := wiki.NewTemplate(wikiSeeds{pg.TreeWrites()})
	if err != nil {
		return nil, err
	}
	// A job is enqueued in the transaction that writes its row, by a
	// client that only inserts: the runner's works the queues.
	inserter, err := jobs.NewInserter(pool, logger)
	if err != nil {
		return nil, err
	}
	trDeps := transferDeps(cfg, pool, logger, authorizer, store, inserter, pg.TreeWrites(), keys.Derive(transfer.DownloadKeyInfo))
	trDeps.Batch = w.batch
	tr := transfer.New(trDeps)
	// The notebook module comes last: its creation's seeder writes the
	// template through the page module's units (M10 design 4.1).
	nb := notebook.New(notebookDeps(cfg, pool, logger, authorizer, []notebook.Seeder{wikiTemplate{template}}))
	ev, listener := eventsModule(cfg, pool, logger)
	runner, err := jobs.New(pool, jobsConfig(cfg, logger),
		slices.Concat(ident.Jobs(), pg.Jobs(), as.Jobs(), tr.Jobs(), []jobs.Job{purgeJob(cfg, pool, store, logger)}))
	if err != nil {
		return nil, err
	}
	// /mcp's body is a call's, which can hold a page's content (M9 design
	// 4.4); past 64 KiB only a tool's call, a large body (M9/P2 design 3.2).
	bodies := maps.Clone(pg.BodyLimits())
	bodies[mcpserver.Pattern] = mcpBodyLimit
	apiCfg := apiConfig(cfg, logger, limiter, ident.Authenticator(),
		slices.Concat(ident.PublicOperations(), inst.PublicOperations(), ws.PublicOperations(), as.PublicOperations(), tr.PublicOperations(),
			mcpPublicOperations()),
		ident.RequestTimeouts(), bodies)
	apiCfg.LargeBodyAbove = map[string]int64{mcpserver.Pattern: mcpserver.MaxOtherBytes}
	if w.largeBodyWait > 0 {
		apiCfg.LargeBodyWait = w.largeBodyWait
	}
	apiCfg.NoTokenDetails = mcpNoTokenDetails()
	api, err := httpserver.NewAPI(apiCfg)
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
	ln.Register(router, api)
	as.Register(router, api)
	tr.Register(router, api)
	ev.Register(router, api)
	tools := mcpTools(pg, as, ln, nb)
	mountMCP(router, api, mcpServer(cfg, logger, api, pg, mcpNames{identity.NewClientNames(pool)}, slices.Concat(tools, w.mcpTools)))
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
		mcpTools:         tools,
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
