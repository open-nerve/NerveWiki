package wiki

import (
	"context"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// NotebookDeletion follows the notebook module's deletions (M11/P1 design
// 3.15): the deleted notebooks' upgrades go with them at the deletion's
// time, in its transaction.
type NotebookDeletion interface {
	NotebooksDeleted(ctx context.Context, ids []uuid.UUID, by uuid.UUID, at time.Time) error
}

// NewNotebookDeletion builds the subscriber from the pool alone (v0.1
// design 13.1, item 21): the command line's composition reaches it through
// the workspace module's deletion, where no wiki.New is built.
func NewNotebookDeletion(pool *pgxpool.Pool) NotebookDeletion {
	return app.NotebookDeletion{Upgrades: postgresadapter.New(pool)}
}

// Purgers are the module's purgers (v0.1 design 13.1, item 6): its one
// table with deleted_at, the upgrades. They reference the notebook
// module's notebooks, ON DELETE RESTRICT, so bootstrap lists these before
// that module's.
func Purgers(pool *pgxpool.Pool) []jobs.Purger {
	store := postgresadapter.New(pool)
	return []jobs.Purger{{Table: "notebook_upgrades", Purge: store.PurgeUpgrades}}
}
