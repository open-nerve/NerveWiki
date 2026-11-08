package postgresadapter_test

import (
	"context"
	"maps"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// An upgrade's rows the deletion guardrail reads (M11/P1 design 3.4):
// those of the ids asked that have a row of the upgrade, its baseline's
// and its protected source root's, in the unit's transaction; another
// upgrade's rows and nodes with none are not.
func TestUpgradeProtected(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	upgrade, other := uuid.NewV7(), uuid.NewV7()
	page, asset, root, others, fresh := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash) VALUES
		($1, $3, $7, true, 'page', NULL, 'Guide', 0, 1, sha256('x'), 1, 0, sha256('x')),
		($1, $4, $7, true, 'asset', $3, 'a.png', 1, NULL, NULL, NULL, NULL, NULL),
		($1, $5, $7, false, 'page', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL),
		($2, $6, $8, true, 'asset', NULL, 'b.png', 0, NULL, NULL, NULL, NULL, NULL)`,
		upgrade, other, page, asset, root, others, f.eng, f.ops)
	want := map[uuid.UUID]bool{page: true, asset: true, root: true}
	if err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(ctx, func(ctx context.Context) error {
		got, err := f.s.Protected(ctx, upgrade, []uuid.UUID{page, asset, root, others, fresh})
		if err != nil {
			return err
		}
		if !maps.Equal(got, want) {
			t.Errorf("Protected = %v, want the upgrade's three rows %v", got, want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := f.s.Protected(ctx, upgrade, []uuid.UUID{fresh}); err != nil || len(got) != 0 {
		t.Errorf("Protected of a node with no row = %v, %v; want none", got, err)
	}
}

// A changeset is written with the upgrade it is of, NULL for none (M11/P1
// design 3.4).
func TestCreateChangesetWritesItsUpgrade(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	upgrade := uuid.NewV7()
	of, none := uuid.NewV7(), uuid.NewV7()
	for id, u := range map[uuid.UUID]uuid.UUID{of: upgrade, none: {}} {
		if err := f.s.CreateChangeset(ctx, app.Changeset{ID: id, NotebookID: f.eng, Kind: domain.ChangesetEdit,
			Client: domain.MCPClient("claude-code"), By: f.alice, At: now(), UpgradeID: u}); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND upgrade_id = $2", of, upgrade); n != 1 {
		t.Errorf("the upgrade's changeset has upgrade_id %d times, want once", n)
	}
	if n := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND upgrade_id IS NULL", none); n != 1 {
		t.Errorf("a changeset of no upgrade has upgrade_id NULL %d times, want once", n)
	}
}

// A notebook's deletion deletes its upgrades' rows, baseline and
// protected, by the notebook (M11/P1 design 3.12); another notebook's
// stay.
func TestTheNotebooksDeletionDeletesTheirUpgradesRows(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash) VALUES
		($1, gen_random_uuid(), $3, true, 'page', NULL, 'Guide', 0, 1, sha256('x'), 1, 0, sha256('x')),
		($1, gen_random_uuid(), $3, false, 'page', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL),
		($2, gen_random_uuid(), $4, true, 'asset', NULL, 'b.png', 0, NULL, NULL, NULL, NULL, NULL)`,
		uuid.NewV7(), uuid.NewV7(), f.eng, f.ops)
	if err := f.s.DeleteNotebooksUpgradeNodes(ctx, []uuid.UUID{f.eng}); err != nil {
		t.Fatal(err)
	}
	var eng, ops int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE notebook_id = $1), count(*) FILTER (WHERE notebook_id = $2)
		FROM upgrade_nodes`, f.eng, f.ops).Scan(&eng, &ops); err != nil {
		t.Fatal(err)
	}
	if eng != 0 || ops != 1 {
		t.Errorf("after eng's deletion, eng has %d rows, ops %d; want none, ops' one", eng, ops)
	}
}
