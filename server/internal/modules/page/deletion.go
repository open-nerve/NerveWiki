package page

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

// NotebookDeleted is notebooks being deleted, field by field as the
// notebook module's deletion tells it: bootstrap converts
// notebook.NotebookDeletion.
type NotebookDeleted = app.NotebookDeleted

// NotebookDeletion is the module's registrant of the notebook module's
// deletion (M4/P1 design 3.9): it deletes the notebooks' pages, what
// follows them and the notebooks' changesets at the deletion's time.
type NotebookDeletion = app.NotebookDeletion

// NewNotebookDeletion builds the registrant from the pool alone (v0.1
// design 13.1, item 21): the command line's composition reaches it through
// the workspace module's deletion. It runs in the deletion's transaction:
// the store finds it in the context.
func NewNotebookDeletion(pool *pgxpool.Pool) NotebookDeletion {
	return app.NotebookDeletion{Pages: postgresadapter.New(pool)}
}
