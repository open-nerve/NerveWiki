package page

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

// NotebookActivity is the pages' part in a notebook's activity (M4/P4
// design 3.10): the bytes of its pages not deleted, and the last write to
// its changesets not deleted.
type NotebookActivity = app.NotebookActivity

// Activities reads the pages' part in notebooks' activity: bootstrap
// converts it into the notebook module's activity source.
type Activities = app.Activities

// NewNotebookActivity builds the pages' activity from the pool alone. It
// reads in its caller's transaction, which the store finds in the context,
// unlocked; a notebook without a changeset is not in its answer.
func NewNotebookActivity(pool *pgxpool.Pool) Activities {
	return postgresadapter.New(pool)
}
