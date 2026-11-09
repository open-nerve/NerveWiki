//go:build !race

package page_test

import (
	"context"
	"encoding/json"
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
// rollbackScattered new pages, scattered below them one by one.
const (
	rollbackFolders   = 1000
	rollbackScattered = 100000
)

// rollbackDeleteBound and rollbackDeleteAllocBound are what the pass that
// deletes first may take in the test below, the plan and all its units,
// and allocate; rollbackUnitBound the median of its units: three times the
// larger of two local rounds, rounded up to 10 s, 1 s and 100 MiB (M11/P1
// design 7.1, decision 236). The rounds' passes took 180.0 and 182.3 s,
// 746 MiB each; their units' medians 12.0 and 13.0 s, the plan 174 and
// 179 ms, the seed 2.5 and 2.6 s. A unit deletes 10,000 subtrees one by
// one, about 1.2 ms each; the second to fourth units of each round took
// 27 to 38 s, the others 12 to 16 s.
const (
	rollbackDeleteBound      = 550 * time.Second
	rollbackDeleteAllocBound = 2300 << 20
	rollbackUnitBound        = 39 * time.Second
)

// The cost guard of a rollback that deletes first (M11/P1 design 7.1,
// decision 231): rollbackScattered new pages, each a free subtree of its
// own below one of rollbackFolders baseline folders, the notebook rolling
// back. The module's PlanRollback packs them into a pass of
// MaxUnitNodes's units, ten, each of 10,000 pages (one each would be
// 100,000 units); RunBatch runs each in a unit, the job's one changeset a
// rollback's of the upgrade; then the next PlanRollback is Done, the
// baseline alone alive, every page in the trash an item of its own. The
// pass's time and bytes allocated, and its units' median, are within
// their bounds; -v prints them and the seed's time.
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
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, 'x', 1, sha256('x'), 1, created_by_id, now() FROM nodes WHERE notebook_id = $1 AND id <> $2`, f.eng, f.notes)
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $1, n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision, c.content_hash, c.byte_size, 0,
			c.content_hash
		FROM nodes n JOIN page_contents c ON c.node_id = n.id WHERE n.notebook_id = $2`, upgrade, f.eng)
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
	f.exec(t, "ANALYZE nodes, page_contents, upgrade_nodes")
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
	if len(steps) != rollbackScattered/domain.MaxUnitNodes {
		t.Fatalf("the pass is of %d units, want %d of %d pages each", len(steps), rollbackScattered/domain.MaxUnitNodes,
			domain.MaxUnitNodes)
	}
	var units []time.Duration
	deleted := 0
	for i, step := range steps {
		began := time.Now()
		done, err := writes.RunBatch(ctx, spec, plan, i, i+1)
		units = append(units, time.Since(began))
		if err != nil || step.Nodes != domain.MaxUnitNodes || done.Deleted != step.Nodes {
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
	if n := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND trash_root", f.eng); n != rollbackScattered {
		t.Errorf("%d trash items, want %d", n, rollbackScattered)
	}
	if n := f.count(t, `SELECT count(*) FROM changesets WHERE notebook_id = $1 AND id = $2 AND kind = 'rollback' AND upgrade_id = $3`,
		f.eng, spec.Changeset, upgrade); n != 1 || f.count(t, "SELECT count(*) FROM changesets WHERE notebook_id = $1", f.eng) != 1 {
		t.Errorf("the job's changeset, a rollback's of the upgrade: %d, want it alone", n)
	}
	sorted := slices.Clone(units)
	slices.Sort(sorted)
	median := sorted[len(sorted)/2]
	ms := make([]int64, len(units))
	for i, d := range units {
		ms[i] = d.Milliseconds()
	}
	b, err := json.Marshal(map[string]any{"seed_ms": seeded.Milliseconds(), "plan_ms": planned.Milliseconds(), "pass_ms": took.Milliseconds(),
		"units_ms": ms, "unit_median_ms": median.Milliseconds(), "alloc_mb": alloc >> 20, "deleted": deleted})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("rollback delete first cost %s", b)
	if took > rollbackDeleteBound || alloc > rollbackDeleteAllocBound || median > rollbackUnitBound {
		t.Errorf("the pass took %s and %d MiB, its units' median %s, past %s, %d MiB or %s", took, alloc>>20, median, rollbackDeleteBound,
			rollbackDeleteAllocBound>>20, rollbackUnitBound)
	}
}
