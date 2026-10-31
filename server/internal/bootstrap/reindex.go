package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Reindex is `nervewiki reindex` (M6/P3 design 3.6): it rebuilds the link
// index and title keys of one notebook, or every live notebook in id
// order. Each linking/rekey rebuild is a transaction holding the notebook
// and index locks. Failed or clashing rebuilds leave that stage unchanged.
// It uses the command's pool, Markdown and budget, without HTTP or jobs.
//
// After each rebuild, while ctx is usable, search rows with a missing or
// old tokenizer are queued even if linking failed. QueueSearch batches of
// at most 10000 nodes commit separately, after the linking locks end.
// A later batch's failure preserves earlier batches; queue failure does
// not undo a committed link index or its event. The server's search job
// builds the queued rows later (M8/P5 design 3.4).
//
// A notebook's result goes to out, failures and clashes to errOut. All-book
// mode continues after ordinary failures and finally returns an error.
// Single-book mode still attempts its queue after a linking failure and
// joins their errors (M8/P5A review B-N2). Missing notebooks, cancellation
// and output failures retain their early-return behavior.
func Reindex(ctx context.Context, cfg config.Config, errOut, out io.Writer, id uuid.UUID) error {
	c, err := openAdminCommand(ctx, cfg, errOut)
	if err != nil {
		return err
	}
	defer c.close()
	admin, err := reindexAdmin(c.pool, cfg, c.logger)
	if err != nil {
		return err
	}
	ids := []uuid.UUID{id}
	if id == uuid.Nil() {
		if ids, err = notebook.NewCatalog(c.pool).IDs(ctx); err != nil {
			return err
		}
	}
	search := page.NewSearchIndex(c.pool, cfg.Database.CommitTimeout)
	failed := 0
	for _, nb := range ids {
		r, err := admin.Rebuild(ctx, nb)
		switch {
		case errors.Is(err, linking.ErrNoNotebook) && id == uuid.Nil():
			continue // deleted since it was listed
		case errors.Is(err, linking.ErrNoNotebook):
			return fmt.Errorf("no notebook %s", nb)
		}
		var queued int
		var qerr error
		if ctx.Err() == nil {
			if queued, qerr = search.Requeue(ctx, nb); qerr != nil {
				qerr = fmt.Errorf("its nodes not queued for the search: %w", qerr)
			}
		}
		if (err != nil && id != uuid.Nil()) || ((err != nil || qerr != nil) && ctx.Err() != nil) {
			return fmt.Errorf("notebook %s: %w", nb, errors.Join(err, qerr))
		}
		searched := plural(queued, "node") + " queued for the search"
		if qerr != nil {
			searched = qerr.Error()
		}
		var line string
		switch {
		case err != nil:
			line = fmt.Sprintf("notebook %s: not reindexed: %v; %s", nb, err, searched)
		case len(r.Clashes) > 0:
			slugOf := func(ctx context.Context, nb uuid.UUID) (string, error) { return workspaceSlug(ctx, c.pool, nb) }
			if line, err = clashLine(ctx, c.logger, slugOf, nb, r.Clashes); err != nil {
				return err
			}
			line += "; " + searched
		default:
			line = fmt.Sprintf("notebook %s: %s, %s, %d unresolved; %s", nb, plural(r.Pages, "page"), plural(r.Links, "link"), r.Unresolved,
				searched)
			if qerr == nil {
				if err := writeLine(out, line); err != nil {
					return err
				}
				continue
			}
		}
		failed++
		if err := writeLine(errOut, line); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("%s not reindexed, as listed above: have the notebook's members rename one page of each group that would share a key, or see the logs, then run reindex again", plural(failed, "notebook"))
	}
	return nil
}

// plural is n things, "1 page", "2 pages".
func plural(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// clashLine tells the siblings of the notebook nb whose title keys would
// clash, each group by its pages' addresses in the web app, which the
// notebook's members open: the titles are the notebook's, which the
// server's administrator may not read, but its workspace's slug, of
// slugOf, is said (v0.1 design 13.1 rule 10; M6 Codex review, fix check
// B2-Q1). Of a workspace deleted meanwhile, or whose slug is not read, the
// error logged, by the pages' ids (fix check B3-M3): it fails only when ctx
// is done.
func clashLine(ctx context.Context, logger *slog.Logger, slugOf func(context.Context, uuid.UUID) (string, error),
	nb uuid.UUID, clashes []linking.Clash) (string, error) {
	slug, err := slugOf(ctx, nb)
	if err != nil && ctx.Err() != nil {
		return "", fmt.Errorf("notebook %s: %w", nb, err)
	}
	if err != nil {
		logger.LogAttrs(ctx, slog.LevelError, "the pages whose titles would share a key are told by id: their workspace's slug is not read",
			slog.String("notebook_id", nb.String()), slog.Any("error", err))
		slug = ""
	}
	path := func(id uuid.UUID) string { return id.String() }
	if slug != "" {
		path = func(id uuid.UUID) string { return fmt.Sprintf("/%s/notebooks/%s/pages/%s", slug, nb, id) }
	}
	groups := make([]string, len(clashes))
	for i, c := range clashes {
		pages := make([]string, len(c))
		for j, id := range c {
			pages[j] = path(id)
		}
		groups[i] = strings.Join(pages, ", ")
	}
	return fmt.Sprintf("notebook %s: not reindexed: the pages whose titles would share a key: %s", nb, strings.Join(groups, "; ")), nil
}

// workspaceSlug is the slug of the workspace of the notebook nb on pool; ""
// when the notebook or its workspace was deleted meanwhile.
func workspaceSlug(ctx context.Context, pool *pgxpool.Pool, nb uuid.UUID) (string, error) {
	ws, ok, err := notebook.NewNotebooks(pool).WorkspaceOf(ctx, nb)
	if err != nil || !ok {
		return "", err
	}
	slugs, err := workspace.NewWorkspaces(pool).Slugs(ctx, []uuid.UUID{ws})
	return slugs[ws], err
}

// reindexAdmin is the linking module's rebuild on pool: the Markdown and
// its budget are serve's (parsing) without its attachments' Assets, its
// links events go to the stream.
func reindexAdmin(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) (linking.Admin, error) {
	// It only parses: no reading view shows an attachment (M7/P3 design
	// 5.10).
	md, budget, err := parsing(cfg, logger, pool, nil)
	if err != nil {
		return linking.Admin{}, err
	}
	targets := linkTargets{page.NewLinkTargets(pool)}
	return linking.NewAdmin(linking.AdminDeps{
		Pool:      pool,
		Tx:        postgres.NewTxManager(pool, cfg.Database.CommitTimeout),
		Pages:     targets,
		Contents:  targets,
		Notebooks: notebook.NewNotebooks(pool),
		Publisher: linkEvents{events.NewPublisher()},
		Markdown:  md,
		Budget:    budget,
	}), nil
}
