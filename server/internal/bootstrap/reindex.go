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
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Reindex is `nervewiki reindex` (M6/P3 design 3.6): it rebuilds the link
// index of the notebook id, or, when id is zero, of every notebook not
// deleted, one at a time and each in a transaction, on the command line's
// composition: a pool, the Markdown and its budget as serve's, and the
// linking module's rebuild; no HTTP server or jobs client. A line a
// notebook goes to out. A notebook whose siblings' title keys would clash
// is left as it was, the siblings listed on errOut, with the logs; the
// command then fails, after the others.
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
	clashed := 0
	for _, nb := range ids {
		r, err := admin.Rebuild(ctx, nb)
		switch {
		case errors.Is(err, linking.ErrNoNotebook) && id == uuid.Nil():
			continue // deleted since it was listed
		case errors.Is(err, linking.ErrNoNotebook):
			return fmt.Errorf("no notebook %s", nb)
		case err != nil:
			return fmt.Errorf("notebook %s: %w", nb, err)
		case len(r.Clashes) > 0:
			clashed++
			if err := writeLine(errOut, clashLine(nb, r.Clashes)); err != nil {
				return err
			}
			continue
		}
		if err := writeLine(out, fmt.Sprintf("notebook %s: %d pages, %d links, %d unresolved", nb, r.Pages, r.Links, r.Unresolved)); err != nil {
			return err
		}
	}
	if clashed > 0 {
		return fmt.Errorf("%d notebooks not reindexed: their titles listed above would share a key under one parent; rename them, then run reindex again", clashed)
	}
	return nil
}

// clashLine tells the siblings of the notebook nb whose title keys would
// clash, each group by its titles and ids.
func clashLine(nb uuid.UUID, clashes []linking.Clash) string {
	groups := make([]string, len(clashes))
	for i, c := range clashes {
		names := make([]string, len(c))
		for j, n := range c {
			names[j] = fmt.Sprintf("%q (%s)", n.Name, n.ID)
		}
		groups[i] = strings.Join(names, ", ")
	}
	return fmt.Sprintf("notebook %s: not reindexed: titles that would share a key: %s", nb, strings.Join(groups, "; "))
}

// reindexAdmin is the linking module's rebuild on pool: the Markdown and
// its budget are serve's (parsing), its links events go to the stream.
func reindexAdmin(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) (linking.Admin, error) {
	md, budget, err := parsing(cfg, logger)
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
