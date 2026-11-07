package postgresadapter

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The existing migrated fixture, actual Store, Tx manager and native fixed-
// Scan observer are reused. Capacity is observed on the returned owned F;
// incoming size never becomes a bound on complete old native rows.
func TestPreparedSourceAssetCapacityHint(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	trace.sql = gen.InspectPageLinksForPreparation
	for _, tc := range []struct {
		name                    string
		old, incoming, capacity int
	}{
		{"empty_source", 0, 10000, 0},
		{"no_assets", 513, 10000, 0},
		{"late_asset", 513, 10000, 10000},
		{"dense", 10000, 10000, 10000},
		{"bounded_hint", 1, 10001, 10000},
		{"old_over_hint", 513, 1, -1},
		{"zero_incoming", 513, 0, -1},
		{"negative", 2, 513, 513},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _, _ := retentionCollectorSeed(t, pool, tc.old)
			if tc.name == "no_assets" || tc.name == "late_asset" {
				cutoff := tc.old
				if tc.name == "late_asset" {
					cutoff--
				}
				if _, err := pool.Exec(t.Context(), "UPDATE page_links SET resolved_asset=false WHERE source_id=$1 AND range_start<$2", p.ID, int32(2*cutoff)); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "negative" {
				if _, err := pool.Exec(t.Context(), "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", p.ID); err != nil {
					t.Fatal(err)
				}
			}
			facts := domain.Facts{FrontmatterValid: true, Links: make([]domain.Link, tc.incoming)}
			for i := range facts.Links {
				facts.Links[i] = domain.Link{Kind: "wikilink", Target: "Note", Start: 2 * i, End: 2*i + 1}
			}
			var want []uuid.UUID
			for i := range tc.old {
				if tc.name == "no_assets" || tc.name == "late_asset" && i != tc.old-1 {
					continue
				}
				id := uuid.UUID{15: 11}
				if tc.old > 1 && i == tc.old-1 {
					id = uuid.UUID{15: 12}
				}
				want = append(want, id)
			}
			rollback := errors.New("asset hint read rollback")
			calls := trace.count()
			err := postgres.NewTxManager(pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
				s := New(pool)
				if err := s.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				tx, _ := postgres.TxFrom(ctx)
				w := &preparedWatch{rows: &linksCollectorRows{stable: true, columns: 15}, pid: tx.Conn().PgConn().PID()}
				ctx = preparedContext{Context: ctx, watch: w}
				unit := &app.SourcePreparationUnit{NotebookID: p.NotebookID}
				p.Revision = 2
				plan := s.NewSource(ctx, unit, p, facts)
				if plan == nil || len(w.calls) != 0 {
					t.Fatal("HINT_FACTORY: valid incoming facts were rejected/read early")
				}
				assets, err := plan.ReadBefore(ctx, unit)
				if err != nil {
					return err
				}
				if assets == nil || !slices.Equal(assets, want) || cap(assets) < len(want) || tc.capacity >= 0 && cap(assets) != tc.capacity {
					t.Fatalf("ASSET_HINT: %s full F len/cap=%d/%d want %d/%d", tc.name, len(assets), cap(assets), len(want), tc.capacity)
				}
				if plan.(*preparedSource).read.canRetain != (tc.name != "negative") {
					t.Fatal("hint changed storage eligibility")
				}
				w.rows.check(t, max(1, tc.old), 1, max(1, tc.old), 1)
				if w.early || !slices.Equal(w.calls, []string{"InspectPageLinksForPreparation"}) || trace.count() != calls+1 {
					t.Fatal("HINT_NATIVE: original one-query/Tx path changed")
				}
				retentionCollectorCheckTrace(t, trace, w.pid, p, nil)
				if tx.Conn().PgConn().IsBusy() {
					t.Fatal("hint returned before native Close")
				}
				var one int
				if err := tx.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
					t.Fatalf("reuse actual transaction after read: %d %v", one, err)
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatalf("original manager/cause: %v", err)
			}
		})
	}
}
