package notebook

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// Purgers are the module's purgers (v0.1 design 13.1, item 6), one per
// table with deleted_at, leaf to root: the members before their notebooks.
// The notebooks reference the workspace module's workspaces, so bootstrap
// lists these before that module's.
func Purgers(pool *pgxpool.Pool) []jobs.Purger {
	store := postgresadapter.New(pool)
	return []jobs.Purger{
		{Table: "notebook_members", Purge: store.PurgeMembers},
		{Table: "notebooks", Purge: store.PurgeNotebooks},
	}
}
