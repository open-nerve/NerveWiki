//go:build !race

package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The deletion guardrail's read holds its time at a unit's bound, at any
// statistics (M11/P1 design 7.1's cost guard): the delete of a subtree of
// MaxUnitNodes nodes, half of them of the upgrade's baseline of 100,000
// rows, half created since, beside another upgrade's 100,000 rows, read on
// one connection, which caches the statement as a server's does: without
// statistics; analyzed; with statistics of the other upgrade's rows alone,
// this one's written after them, as when a start's ANALYZE failed; and
// planned for any arguments, as a cached statement may be from its sixth
// run. Each read answers the subtree's half of the baseline, the median
// of five within the unit's 2 seconds; -v prints the medians.
func TestUpgradeProtectedReadsInTimeAtTheUnitsBound(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.exec(t, "ALTER TABLE upgrade_nodes SET (autovacuum_enabled = false)")
	upgrade, other := uuid.NewV7(), uuid.NewV7()
	started := time.Now()
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT u.id, gen_random_uuid(), u.notebook_id, true, 'page', NULL, 'p' || g, g, 1, sha256(g::text::bytea), 1, 0,
			sha256(g::text::bytea)
		FROM (VALUES ($1::uuid, $2::uuid), ($3::uuid, $4::uuid)) u(id, notebook_id), generate_series(1, 100000) g`,
		upgrade, f.eng, other, f.ops)
	var ids []uuid.UUID
	rows, err := f.pool.Query(ctx, "SELECT node_id FROM upgrade_nodes WHERE upgrade_id = $1 ORDER BY node_id LIMIT $2", upgrade,
		domain.MaxUnitNodes/2)
	if err == nil {
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				break
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err == nil {
			err = rows.Err()
		}
	}
	if err != nil || len(ids) != domain.MaxUnitNodes/2 {
		t.Fatalf("the baseline's ids: %d, %v", len(ids), err)
	}
	for range domain.MaxUnitNodes / 2 {
		ids = append(ids, uuid.NewV7())
	}
	t.Logf("seeded in %s", time.Since(started))

	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := postgresadapter.New(pool)
	read := func(stats string) {
		t.Helper()
		var took []time.Duration
		for range 5 {
			began := time.Now()
			got, err := s.Protected(ctx, upgrade, ids)
			took = append(took, time.Since(began))
			if err != nil || len(got) != domain.MaxUnitNodes/2 {
				t.Fatalf("Protected(%s) = %d ids, %v; want %d", stats, len(got), err, domain.MaxUnitNodes/2)
			}
		}
		slices.Sort(took)
		t.Logf("%s: the median of five reads of %d ids, %s", stats, len(ids), took[2])
		if took[2] > 2*time.Second {
			t.Errorf("%s: the median of five reads of %d ids took %s, past the unit's 2 s", stats, len(ids), took[2])
		}
	}
	read("without statistics")
	f.exec(t, "ANALYZE upgrade_nodes")
	read("analyzed")
	f.exec(t, "CREATE TABLE kept AS SELECT * FROM upgrade_nodes WHERE upgrade_id = $1", upgrade)
	f.exec(t, "DELETE FROM upgrade_nodes WHERE upgrade_id = $1", upgrade)
	f.exec(t, "ANALYZE upgrade_nodes")
	f.exec(t, "INSERT INTO upgrade_nodes SELECT * FROM kept")
	read("with statistics of the other upgrade's rows alone")
	if _, err := pool.Exec(ctx, "SET plan_cache_mode = force_generic_plan"); err != nil {
		t.Fatal(err)
	}
	read("planned for any arguments")
}
