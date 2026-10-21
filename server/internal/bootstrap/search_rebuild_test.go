package bootstrap

import (
	"fmt"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The search's index is its rebuild (M8/P5 design 3.10; v0.1 design 13.4,
// item 10): after a random sequence of a notebook's changes through the
// API and an agent's moves and batches through MCP, the revert's
// property's (revertWorld: pages and attachments created, renamed, moved,
// reordered, deleted, restored, written, their versions restored,
// changesets reverted, pages moved and renamed in one step, and batches,
// M9/P3 design 3.14.6) and an import among them, a run
// of the job at random points leaves each live node's row what
// SearchTokens makes of it from scratch, and no other (checkSearchBuilt);
// between the runs, each node is indexed as it is or queued
// (checkSearch). The seeds make 3 batches at least, together (M9/P3 review
// C-M2): a generator that stops making them, or batches refused, fail it.
// A third of the contents are cut (revertWorld.partials): the job's runs
// build a partial row of each cut at least, the seeds together (M9/P4
// design 6.1, review C2-M2).
func TestTheSearchIndexIsItsRebuild(t *testing.T) {
	const changes = 40
	seeds, batches := 0, 0
	partials := [3]map[string]bool{{}, {}, {}}
	for _, seed := range []uint64{1, 2, 3} {
		t.Run(fmt.Sprint("seed ", seed), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			w := newRevertWorld(t, tm, seed)
			w.batches, w.partials = true, true
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
					partialRows(t, tm.pool, seed, partials)
					runs++
				}
			}
			checkSearchBuilt(t, tm.pool)
			partialRows(t, tm.pool, seed, partials)
			checkPages(t, tm.pool)
			seeds, batches = seeds+1, batches+w.made["agent's batch"]
			t.Logf("changes made %v, skipped %v; %d runs of the job between them", w.made, w.skipped, runs)
		})
	}
	if seeds == 3 && batches < 3 {
		t.Errorf("the seeds made %d batches, want 3 at least", batches)
	}
	t.Logf("partial rows built of a long run, a data: string and the bound: %d, %d, %d", len(partials[0]), len(partials[1]),
		len(partials[2]))
	if seeds == 3 && slices.ContainsFunc(partials[:], func(rows map[string]bool) bool { return len(rows) == 0 }) {
		t.Errorf("the job built %d, %d and %d partial rows of a long run, a data: string and the bound; want one of each at least",
			len(partials[0]), len(partials[1]), len(partials[2]))
	}
}

// partialRows adds to rows the partial rows of pool by the cut their
// content holds (cutContent): a long run, a data: string, the bound; each
// a node of seed's.
func partialRows(t *testing.T, pool *pgxpool.Pool, seed uint64, rows [3]map[string]bool) {
	t.Helper()
	for i, cut := range []struct {
		holds string
		arg   any
	}{{"c.content LIKE '%' || $1 || '%'", cutLong}, {"c.content LIKE '%' || $1 || '%'", cutData}, {"c.byte_size >= $1", cutBounds}} {
		for _, id := range queryStrings(t, pool, `SELECT s.node_id::text FROM page_search s
			JOIN page_contents c ON c.node_id = s.node_id AND c.deleted_at IS NULL WHERE s.partial AND `+cut.holds, cut.arg) {
			rows[i][fmt.Sprint(seed, id)] = true
		}
	}
}
