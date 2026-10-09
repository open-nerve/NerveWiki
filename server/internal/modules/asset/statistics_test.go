package asset_test

import (
	"context"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// Statistics analyzes each of the module's tables an import writes.
func TestStatisticsAnalyzeTheTablesAnImportWrites(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := asset.NewStatistics(pool).Analyze(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"asset_blobs"} {
		var analyzed bool
		if err := pool.QueryRow(ctx, "SELECT last_analyze IS NOT NULL FROM pg_stat_user_tables WHERE relname = $1", table).Scan(&analyzed); err != nil || !analyzed {
			t.Errorf("%s analyzed: %t, %v; want it analyzed", table, analyzed, err)
		}
	}
}
