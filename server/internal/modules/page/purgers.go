// Package page is the module of a notebook's pages: the tree of nodes,
// their content, the changesets and versions of their writes (v0.1 design
// 3.5, 3.6, 3.8; M4 design). Its root is what bootstrap sees.
package page

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// Purgers are the module's purgers (v0.1 design 13.1, item 6), one per
// table with deleted_at, leaf to root (M4/P1 design 3.10): what follows a
// node, the nodes leaves first, then the changesets. The nodes and the
// changesets reference the notebook module's notebooks, so bootstrap lists
// these before that module's.
func Purgers(pool *pgxpool.Pool) []jobs.Purger {
	store := postgresadapter.New(pool)
	return []jobs.Purger{
		{Table: "changeset_items", Purge: store.PurgeChangesetItems},
		{Table: "page_revisions", Purge: store.PurgePageRevisions},
		{Table: "page_contents", Purge: store.PurgePageContents},
		{Table: "nodes", Purge: store.PurgeNodes},
		{Table: "changesets", Purge: store.PurgeChangesets},
	}
}
