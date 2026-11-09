//go:build !race

package app_test

import (
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// rollbackPlanBound and rollbackPlanAllocBound are the median of five
// plans of a rollback of 100,000 items the test below allows, its time
// and its bytes allocated: three times the larger of the medians of two
// local rounds, rounded up to 100 ms and 100 MiB (M11/P1 design 7.1,
// decision 236). The rounds' medians were 644 and 639 ms, 527 and 527
// MiB; the seed took 136 and 154 ms.
const (
	rollbackPlanBound      = 2 * time.Second
	rollbackPlanAllocBound = 1600 << 20
)

// The rollback's plan's cost guard, its pure part (M11/P1 design 7.1):
// a baseline of 100 folders of 1,000 pages each, every page then moved to
// the next folder, renamed or reordered, a third each, 100,000 items; the
// rollback's PlanRollback over the reads in memory (RollbackState, the
// revert's plan of it, its steps), five times. Each answers one pass of
// 100,000 items, its plan of steps for every one; the median's time and
// bytes allocated are within their bounds. -v prints the medians and the
// seed's time. The synthesized reads' cost is the adapter's guard's
// (TestARollbacksReadsTakeTheirTimeAtAnyStatistics).
func TestARollbacksPlanOfAHundredThousandItemsTakesItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("the rollback's plan's cost guard runs alone, without race")
	}
	const folders, pages = 100, 1000
	started := time.Now()
	f := newFixture()
	f.grant(changeMode)
	reads := &rollbackReads{f: f}
	tops := make([]uuid.UUID, folders)
	for k := range folders {
		tops[k] = f.plant(uuid.NewV7(), domain.KindPage, domain.TreeState{Name: fmt.Sprintf("f%d", k), SortOrder: float64(k)}, "f").ID
	}
	var ids []uuid.UUID
	for k := range folders {
		for i := range pages {
			ids = append(ids, f.plant(uuid.NewV7(), domain.KindPage, domain.TreeState{ParentID: &tops[k], Name: fmt.Sprintf("p%d-%d", k, i),
				SortOrder: float64(i)}, "p").ID)
		}
	}
	for id := range f.store.nodes {
		reads.baseline = append(reads.baseline, f.baselineOf(id))
	}
	for j, id := range ids {
		n := f.store.nodes[id]
		switch j % 3 {
		case 0:
			n.ParentID = &tops[(j/pages+1)%folders]
		case 1:
			n.Name = "r" + n.Name
			n.NameKey = shared.TitleKey(n.Name)
		case 2:
			n.SortOrder += 0.5
		}
		f.store.nodes[id] = n
	}
	up := uuid.NewV7()
	f.rollingBack(up, domain.UpgradeRollingBack)
	w, spec := f.rollbacks(&checkedStore{fakeStore: f.store}, reads, 0, 0), f.jobSpec()
	spec.UpgradeID = up
	seeded := time.Since(started)

	var took []time.Duration
	var allocs []uint64
	for range 5 {
		f.rec.calls = nil
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		began := time.Now()
		plan, pass, err := w.PlanRollback(f.asJob(), spec)
		took = append(took, time.Since(began))
		runtime.ReadMemStats(&after)
		allocs = append(allocs, after.TotalAlloc-before.TotalAlloc)
		if err != nil || pass.Deletes || pass.Done || pass.Items != folders*pages || len(plan.Steps()) == 0 {
			t.Fatalf("PlanRollback = %d steps, %+v, %v; want one pass of %d items", len(plan.Steps()), pass, err, folders*pages)
		}
	}
	slices.Sort(took)
	slices.Sort(allocs)
	b, err := json.Marshal(map[string]any{"seed_ms": seeded.Milliseconds(), "median_ms": took[2].Milliseconds(),
		"took_ms": durationsMs(took), "median_alloc_mb": allocs[2] >> 20})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("rollback plan cost %s", b)
	if took[2] > rollbackPlanBound || allocs[2] > rollbackPlanAllocBound {
		t.Errorf("the median of five plans of %d items took %s and %d MiB, past %s or %d MiB", folders*pages, took[2], allocs[2]>>20,
			rollbackPlanBound, rollbackPlanAllocBound>>20)
	}
}

// durationsMs are ds in milliseconds.
func durationsMs(ds []time.Duration) []int64 {
	out := make([]int64, len(ds))
	for i, d := range ds {
		out[i] = d.Milliseconds()
	}
	return out
}
