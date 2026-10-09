package linking

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
)

// Statistics updates the statistics of the module's tables an import
// writes (M7/P6 design 3.6; M6 handoff, item 4): the transfer module runs
// no SQL on another module's tables. ANALYZE takes MAINTAIN on them,
// which deploy/runtime-grants.sql gives the serving role.
type Statistics struct {
	store *postgresadapter.Store
}

// NewStatistics returns the statistics of pool's tables.
func NewStatistics(pool *pgxpool.Pool) Statistics {
	return Statistics{store: postgresadapter.New(pool)}
}

// Analyze runs ANALYZE on the module's tables an import writes.
func (s Statistics) Analyze(ctx context.Context) error {
	return s.store.Analyze(ctx)
}
