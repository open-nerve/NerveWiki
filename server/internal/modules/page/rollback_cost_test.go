//go:build !race

package page_test

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The guard's notebook: rollbackFolders baseline folders, and
// rollbackScattered new pages, scattered below them one by one; and
// rollbackTrashed pages, 5 % of the new ones, deleted in a folder of
// their own before the upgrade started.
const (
	rollbackFolders   = 1000
	rollbackScattered = 100000
	rollbackTrashed   = 5000
)

// rollbackDeleteBound and rollbackDeleteAllocBound are what the pass that
// deletes first may take in the test below, the plan and all its units,
// and allocate: three times the larger of two local rounds, rounded up to
// 10 s and 100 MiB (M11/P1 design 7.1, decision 236). rollbackUnitBound is
// what each of its units may take, the slowest among them: 14 s, as a
// restore's and a revert's unit (decision 153), under the 15 s of a
// request, which the notebook's admin writes waiting for the unit's lock
// on the notebook's row spend (decision 243). The two rounds' passes, 40
// units of 2,500 pages, took 109.3 and 109.2 s and 737 MiB, their units
// 2.67 to 3.22 s (medians 2.72 and 2.71 s); the seed 2.4 and 2.5 s. On
// CI's EPYC 9V45 (run 42671683550) its units took 2.9 to 3.1 s, the pass
// 115.5 s. Units of MaxUnitNodes, 10,000 pages, took 10.6 to 11.1 s here:
// they would pass 14 s only on a machine like CI's EPYC 7763 (an
// estimate, 15 to 19 s at the S3 guard's ratio there, about 1.4 to 1.7,
// not measured); on one like its 9V45, near this one, they stay within
// it. So the bound is held by the unit test TestTheFirstDeletesBound
// (page/domain), not by this guard. Without the deleted folder, units of
// 2,500 took 3.0 s, then each more than the one before, 28.1 s the last,
// the pass 645 s.
const (
	rollbackDeleteBound      = 330 * time.Second
	rollbackDeleteAllocBound = 2300 << 20
	rollbackUnitBound        = 14 * time.Second
)

// The cost guard of a rollback that deletes first (M11/P1 design 7.1,
// decisions 231 and 243): rollbackScattered new pages, each a free
// subtree of its own below one of rollbackFolders baseline folders, the
// notebook rolling back. The module's PlanRollback packs them into a pass
// of units of MaxDeleteFirstNodes pages each (one each would be 100,000
// units); RunBatch runs each in a unit, the job's one changeset a
// rollback's of the upgrade; then the next PlanRollback is Done, the
// baseline alone alive, every page in the trash an item of its own. The
// pass's time and bytes allocated, and each of its units, are within
// their bounds; -v prints them, their median and the seed's time.
//
// The guard measures the steady state, not when autovacuum wakes. Before
// the upgrade starts, alice deletes a folder of rollbackTrashed pages, so
// that the statistics of nodes and page_contents (and of changeset_items
// and page_search_queue) the seed's ANALYZE makes have seen deleted rows,
// as a notebook's do once anything was ever deleted in it; autovacuum is
// off on the tables the units write, so nothing analyzes them again during
// the pass. Statistics that have seen no deleted row make each unit slower
// than the one before as the pass deletes (M11/P1 S3 review B: 14 to 71 s
// a unit of 10,000, autovacuum off, the seed's ANALYZE alone); which
// statement's plan turns is not known, the M12 performance handover's
// (decision 243).
func TestARollbackDeletesAHundredThousandScatteredPagesFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("the rollback's cost guard runs alone, without race")
	}
	f := newFixture(t)
	upgrade := uuid.NewV7()
	started := time.Now()
	f.exec(t, `INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), $1, 'page', NULL, 'f' || k, 'f' || k, k, $2, $2, now(), now() FROM generate_series(1, $3::integer) k`,
		f.eng, f.alice, rollbackFolders)
	old := uuid.NewV7()
	f.exec(t, `INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		VALUES ($1, $2, 'page', NULL, 'Old', 'old', $4, $3, $3, now(), now())`, old, f.eng, f.alice, rollbackFolders+1)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), $1, 'page', $3, 'o' || k, 'o' || k, k, $2, $2, now(), now() FROM generate_series(1, $4::integer) k`,
		f.eng, f.alice, old, rollbackTrashed)
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, 'x', 1, sha256('x'), 1, created_by_id, now() FROM nodes WHERE notebook_id = $1 AND id <> $2`, f.eng, f.notes)
	if rec := f.serve(t, "session", http.MethodDelete, "/api/v0/nodes/"+old.String(), "", nil, nil, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("the deletion of Old = %d %s", rec.Code, rec.Body)
	}
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $1, n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision, c.content_hash, c.byte_size, 0,
			c.content_hash
		FROM nodes n JOIN page_contents c ON c.node_id = n.id WHERE n.notebook_id = $2 AND n.deleted_at IS NULL`, upgrade, f.eng)
	f.exec(t, `WITH made AS (
			INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
				updated_at)
			SELECT gen_random_uuid(), $1, 'page', d.id, 'n' || k, 'n' || k, k, $2, $2, now(), now()
			FROM generate_series(1, $3::integer) k
			JOIN nodes d ON d.notebook_id = $1 AND d.parent_id IS NULL AND d.name = 'f' || (k % $4::integer + 1)
			RETURNING id
		)
		INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, 'x', 1, sha256('x'), 1, $2, now() FROM made`, f.eng, f.alice, rollbackScattered, rollbackFolders)
	f.exec(t, `UPDATE notebooks SET upgrade_id = $2, upgrade_state = 'rolling_back', upgrade_by_id = $3, upgrade_at = now()
		WHERE id = $1`, f.eng, upgrade, f.alice)
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changeset_items", "changesets", "page_search_queue"} {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	f.exec(t, "ANALYZE nodes, page_contents, page_revisions, changeset_items, changesets, page_search_queue, upgrade_nodes")
	seeded := time.Since(started)

	writes := f.module(nil, nil, nil).TreeWrites()
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, JobID: uuid.NewV7()})
	spec := page.JobSpec{NotebookID: f.eng, Client: domain.ClientWeb, Changeset: uuid.NewV7(), UpgradeID: upgrade}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	began := time.Now()
	plan, pass, err := writes.PlanRollback(ctx, spec)
	planned := time.Since(began)
	if err != nil || !pass.Deletes || pass.Left != (page.RollbackLeft{Created: rollbackScattered}) {
		t.Fatalf("PlanRollback = %+v, %v; want a pass that deletes %d new pages", pass, err, rollbackScattered)
	}
	steps := plan.Steps()
	if len(steps) != rollbackScattered/domain.MaxDeleteFirstNodes {
		t.Fatalf("the pass is of %d units, want %d of %d pages each", len(steps), rollbackScattered/domain.MaxDeleteFirstNodes,
			domain.MaxDeleteFirstNodes)
	}
	var units []time.Duration
	deleted := 0
	for i, step := range steps {
		began := time.Now()
		done, err := writes.RunBatch(ctx, spec, plan, i, i+1)
		units = append(units, time.Since(began))
		if err != nil || step.Nodes != domain.MaxDeleteFirstNodes || done.Deleted != step.Nodes {
			t.Fatalf("unit %d of %d nodes deleted %+v, %v", i, step.Nodes, done, err)
		}
		deleted += done.Deleted
	}
	took := time.Since(began)
	runtime.ReadMemStats(&after)
	alloc := after.TotalAlloc - before.TotalAlloc
	if _, pass, err := writes.PlanRollback(ctx, spec); err != nil || !pass.Done {
		t.Errorf("the next PlanRollback = %+v, %v; want Done", pass, err)
	}
	if n := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", f.eng); n != rollbackFolders+1 {
		t.Errorf("%d nodes alive, want the baseline's %d", n, rollbackFolders+1)
	}
	if n := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND trash_root", f.eng); n != rollbackScattered+1 {
		t.Errorf("%d trash items, want %d and Old", n, rollbackScattered)
	}
	if n := f.count(t, `SELECT count(*) FROM changesets WHERE notebook_id = $1 AND id = $2 AND kind = 'rollback' AND upgrade_id = $3`,
		f.eng, spec.Changeset, upgrade); n != 1 || f.count(t, "SELECT count(*) FROM changesets WHERE notebook_id = $1", f.eng) != 2 {
		t.Errorf("the job's changeset, a rollback's of the upgrade: %d, want it alone but Old's deletion", n)
	}
	sorted := slices.Clone(units)
	slices.Sort(sorted)
	median, slowest := sorted[len(sorted)/2], sorted[len(sorted)-1]
	ms := make([]int64, len(units))
	for i, d := range units {
		ms[i] = d.Milliseconds()
	}
	b, err := json.Marshal(map[string]any{"seed_ms": seeded.Milliseconds(), "plan_ms": planned.Milliseconds(), "pass_ms": took.Milliseconds(),
		"units_ms": ms, "unit_median_ms": median.Milliseconds(), "unit_max_ms": slowest.Milliseconds(), "alloc_mb": alloc >> 20,
		"deleted": deleted})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("rollback delete first cost %s", b)
	if took > rollbackDeleteBound || alloc > rollbackDeleteAllocBound || slowest > rollbackUnitBound {
		t.Errorf("the pass took %s and %d MiB, its slowest unit %s, past %s, %d MiB or %s", took, alloc>>20, slowest, rollbackDeleteBound,
			rollbackDeleteAllocBound>>20, rollbackUnitBound)
	}
}
