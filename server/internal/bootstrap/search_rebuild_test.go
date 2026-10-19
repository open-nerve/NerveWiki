package bootstrap

import (
	"fmt"
	"testing"
)

// The search's index is its rebuild (M8/P5 design 3.10; v0.1 design 13.4,
// item 10): after a random sequence of a notebook's changes through the
// API and an agent's moves through MCP, the revert's property's
// (revertWorld: pages and attachments created, renamed, moved, reordered,
// deleted, restored, written, their versions restored, changesets
// reverted, pages moved and renamed in one step) and an import among them, a run
// of the job at random points leaves each live node's row what
// SearchTokens makes of it from scratch, and no other (checkSearchBuilt);
// between the runs, each node is indexed as it is or queued
// (checkSearch).
func TestTheSearchIndexIsItsRebuild(t *testing.T) {
	const changes = 40
	for _, seed := range []uint64{1, 2, 3} {
		t.Run(fmt.Sprint("seed ", seed), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			w := newRevertWorld(t, tm, seed)
			runs := 0
			for i := range changes {
				w.change(t)
				if i == changes/2 {
					j := tm.started(t, importStep(t, "alice", w.nb, "", vaultZip(t, vaultFile{"Imported.md", "[[Alpha]] word 7"},
						vaultFile{"Imported/kept.png", pngHead})))
					if ended := tm.endedJob(t, "alice", j.ID); ended.State != "succeeded" {
						t.Fatalf("the import = %+v, want it succeeded", ended)
					}
				}
				checkSearch(t, tm.pool)
				if w.rng.IntN(6) == 0 {
					checkSearchBuilt(t, tm.pool)
					runs++
				}
			}
			checkSearchBuilt(t, tm.pool)
			checkPages(t, tm.pool)
			t.Logf("changes made %v, skipped %v; %d runs of the job between them", w.made, w.skipped, runs)
		})
	}
}
