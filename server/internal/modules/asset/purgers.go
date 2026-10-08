package asset

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	filesadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/files"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Purgers are the module's purgers (v0.1 design 13.1, item 6): the
// attachments' rows, each batch in a transaction of tx that deletes their
// files in store first (M7/P2 design 3.8). asset_blobs references the page
// module's nodes, so bootstrap lists these before that module's.
func Purgers(pool *pgxpool.Pool, tx shared.TxManager, store storage.Store, logger *slog.Logger) []jobs.Purger {
	purge := app.NewPurge(tx, postgresadapter.New(pool), filesadapter.New(store), logger)
	return []jobs.Purger{{Table: "asset_blobs", Purge: purge.Batch}}
}
