package page

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

// IngestMarkResult reports the appended count and persisted total.
type IngestMarkResult = app.IngestMarkResult

// IngestMarks stores the proven sources of an owned live edit changeset.
type IngestMarks = app.IngestMarks

// NewIngestMarks returns the page-owned store, using the caller's transaction.
func NewIngestMarks(pool *pgxpool.Pool) IngestMarks {
	return postgresadapter.New(pool)
}
