package notebook

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
)

// WorkspaceDeleted is a workspace being deleted, field by field as the
// workspace module's deletion tells it: bootstrap converts
// workspace.WorkspaceDeletion.
type WorkspaceDeleted = app.WorkspaceDeleted

// WorkspaceDeletion is the module's registrant of the workspace module's
// deletion (M3/P1 design 3.8): it deletes the workspace's notebooks and
// their members, and tells the notebook deletion's subscribers.
type WorkspaceDeletion = app.WorkspaceDeletion

// NewWorkspaceDeletion builds the registrant from the pool alone (v0.1
// design 13.1, item 21), with the notebook deletion's subscribers it
// calls. It runs in the deletion's transaction: the store finds it in the
// context.
func NewWorkspaceDeletion(pool *pgxpool.Pool, subscribers []NotebookDeletionSubscriber) WorkspaceDeletion {
	return app.WorkspaceDeletion{Notebooks: postgresadapter.New(pool), Subscribers: subscribers}
}
