package page

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

// These read ports preserve the write unit's transaction and notebook locks.
type (
	IngestNodeBefore = app.IngestNodeBefore
	IngestBefore     = app.IngestBefore
	IngestHead       = app.IngestHead
	IngestHeadBatch  = app.IngestHeadBatch
	IngestNow        = app.IngestNow
	IngestMetadata   = app.IngestMetadata
)

func NewIngestMetadata(pool *pgxpool.Pool) IngestMetadata { return postgresadapter.New(pool) }
