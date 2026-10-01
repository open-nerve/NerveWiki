package workspace

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// Purgers are the module's purgers (M2/P4 design 3.4), one per table with
// deleted_at, leaf to root: the invitations and the members before their
// workspaces.
func Purgers(pool *pgxpool.Pool) []jobs.Purger {
	store := postgresadapter.New(pool)
	return []jobs.Purger{
		{Table: "workspace_invitations", Purge: store.PurgeInvitations},
		{Table: "workspace_members", Purge: store.PurgeMembers},
		{Table: "workspaces", Purge: store.PurgeWorkspaces},
	}
}
