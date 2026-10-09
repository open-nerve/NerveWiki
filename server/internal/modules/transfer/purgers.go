package transfer

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	archiveadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/archive"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Purgers are the module's purgers (v0.1 design 13.1, item 6): the jobs'
// rows, each batch in a transaction of tx that deletes their archives in
// store first. transfer_jobs references the notebook module's notebooks,
// so bootstrap lists these before that module's.
func Purgers(pool *pgxpool.Pool, tx shared.TxManager, store storage.Store, logger *slog.Logger) []jobs.Purger {
	purge := app.NewPurge(tx, postgresadapter.New(pool), archiveadapter.New(store, logger), logger)
	return []jobs.Purger{{Table: "transfer_jobs", Purge: purge.Batch}}
}
