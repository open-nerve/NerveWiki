package postgresadapter_test

import (
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Copy the fixture's full pool setup, including AfterConnect, and only
// select one connection, a requested mode and this test's optional tracer.
func trashHistoryOneConnection(t *testing.T, f fixture, mode string, tr pgx.QueryTracer) (*postgresadapter.Store, *pgxpool.Pool) {
	t.Helper()
	cfg := f.pool.Config()
	cfg.MaxConns, cfg.ConnConfig.Tracer = 1, tr
	cfg.ConnConfig.RuntimeParams["plan_cache_mode"] = mode
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgresadapter.New(pool), pool
}

// Independent literal results, through the real Store port: limits apply
// separately to revisions and items; IDs behave as set membership. This
// is adapter history metadata, not a complete source/files/blob fixture.
func TestTrashHistoryKeepsItsTimeMembershipAndPerTableLimit(t *testing.T) {
	f := newFixture(t)
	p := f.page(t, f.eng, nil, "P", 0)
	a := f.asset(t, f.eng, &p.ID, "a.png", 0)
	other := f.page(t, f.eng, nil, "Unselected", 1)
	f.versions(t, p.ID, 3)
	f.versions(t, other.ID, 2)
	var earlierItem uuid.UUID
	for run := range 2 {
		cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.ChangesetEdit, Client: domain.ClientWeb, By: f.alice, At: now()}
		if err := f.s.CreateChangeset(t.Context(), cs); err != nil {
			t.Fatal(err)
		}
		var items []app.Item
		for _, n := range []domain.Node{p, a, other} {
			state := n.State()
			id := uuid.NewV7()
			if run == 0 && n.ID == a.ID {
				earlierItem = id
			}
			items = append(items, app.Item{ID: id, ChangesetID: cs.ID, Change: domain.Change{NodeID: n.ID, After: &state}, At: now()})
		}
		if err := f.s.RecordItems(t.Context(), items); err != nil {
			t.Fatal(err)
		}
	}
	at := now().Add(time.Minute)
	f.trashed(t, at, 1, 1, 1, p.ID, a.ID)
	f.trashed(t, at, 1, 0, 1, other.ID)
	earlier := at.Add(-time.Second)
	f.exec(t, "UPDATE page_revisions SET deleted_at=$2 WHERE node_id=$1 AND revision=1", p.ID, earlier)
	f.exec(t, "UPDATE changeset_items SET deleted_at=$2 WHERE id=$1", earlierItem, earlier)
	unknown := uuid.NewV7()
	for _, mode := range []string{"force_generic_plan", "force_custom_plan"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := trashHistoryOneConnection(t, f, mode, nil)
			for _, tc := range []struct {
				name  string
				ids   []uuid.UUID
				at    time.Time
				limit int
				want  int
			}{
				{"limit0", []uuid.UUID{p.ID, a.ID}, at, 0, 0},
				{"limit1", []uuid.UUID{p.ID, a.ID}, at, 1, 2},
				{"limit2", []uuid.UUID{p.ID, a.ID}, at, 2, 4},
				{"limit3", []uuid.UUID{p.ID, a.ID}, at, 3, 5},
				{"limit100", []uuid.UUID{p.ID, a.ID}, at, 100, 5},
				{"reversed", []uuid.UUID{a.ID, p.ID}, at, 100, 5},
				{"duplicates", []uuid.UUID{a.ID, p.ID, a.ID, p.ID}, at, 100, 5},
				{"nil", nil, at, 100, 0},
				{"empty", []uuid.UUID{}, at, 100, 0},
				{"unknown", []uuid.UUID{unknown}, at, 100, 0},
				{"mixed", []uuid.UUID{unknown, a.ID, p.ID, unknown}, at, 100, 5},
				{"pageOnly", []uuid.UUID{p.ID}, at, 100, 4},
				{"assetOnly", []uuid.UUID{a.ID}, at, 100, 1},
				{"earlierTime", []uuid.UUID{p.ID, a.ID}, earlier, 100, 2},
				{"absentTime", []uuid.UUID{p.ID, a.ID}, at.Add(time.Second), 100, 0},
				{"otherNodeAlone", []uuid.UUID{other.ID}, at, 100, 4},
			} {
				t.Run(tc.name, func(t *testing.T) {
					got, err := s.TrashHistory(t.Context(), tc.ids, tc.at, tc.limit)
					if err != nil || got != tc.want {
						t.Fatalf("real Store TrashHistory=%d err=%v want%d", got, err, tc.want)
					}
				})
			}
		})
	}
}
