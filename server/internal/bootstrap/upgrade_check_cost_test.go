//go:build !race

package bootstrap

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

// The check's guard's rows beside its notebook's (decision 240, the P2
// design review B's seed): checkCostChangesets changesets its upgrade
// wrote and checkCostPlain of the notebook's before it; checkCostOthers
// notebooks of checkCostPages pages each, with another upgrade's baseline
// of them and checkCostChangesets changesets of it.
const (
	checkCostPages      = 50000
	checkCostChangesets = 1000
	checkCostPlain      = 100000
	checkCostOthers     = 9
)

// checkCostBound is the median of three checks the test below allows on
// each seed: three times the larger of the medians of two local rounds,
// rounded up to 100 ms, or 80% of the worker's 3 s, 2.4 s, the smaller
// (M11/P2 design 7.1, decisions 236, 262). The rounds' medians, every page
// kept then one not, were 487 and 487 ms, and 489 and 485 ms; three times
// 489 ms is 1.47 s. Of a check, the rules took 285-312 ms (missing_type and
// missing_summary 82-91 ms each), the sources 76-92 ms, UpgradePreserved
// 86-96 ms, UpgradeGateBaseline 13-19 ms, UpgradeChangesets 1-2 ms; the
// seed beside lint's fixture, 11.9 s.
const checkCostBound = 1500 * time.Millisecond

// An upgrade's check of 50,000 pages takes its time (M11/P2 design 7.1,
// decisions 236, 240, 262): lint's cost fixture of 50,000 pages, made a
// standard notebook and set upgrading by SQL, its upgrade's baseline of
// each node, 1,000 changesets of the upgrade and 100,000 of the notebook's
// before it; beside it nine notebooks of 50,000 pages with their upgrades'
// baselines and 1,000 changesets each; analyzed, as after a start. Under
// lint's guard's configuration, a 6 s request and a 3 s worker, the whole
// check (Module.CheckUpgrade, through lint's runs) on two seeds: every
// baseline page as the ledger accepted it, and one page's ledger another
// hash, gate 7 not passed. The median of three checks of each within
// checkCostBound. Only time: TestTheUpgradeGateReadsPlansAtAnyStatistics
// judges the gate reads' plans, its helpers in page/adapter/postgres's
// tests, which this package cannot import. -v prints each check's parts:
// the stages (sources, nodes, rules, paths), PreparationTimings, and each
// statement, UpgradeGateBaseline, UpgradePreserved and UpgradeChangesets
// among them, the rest of the wall the judgment's and the runs'.
func TestUpgradeCheckTakesItsTimeAtAnyStatistics(t *testing.T) {
	if testing.Short() {
		t.Skip("the check of 50,000 pages runs alone without race")
	}
	f := newWikiCostFixtureOf(t, "Fifty thousand pages", checkCostPages)
	ctx := context.Background()
	started := time.Now()
	f.tm.exec(t, "UPDATE notebooks SET mode = 'standard', source_root_id = NULL WHERE id = $1", f.book.id)
	upgrade, err := upgradeIn(ctx, f.tm.pool, f.book.id, f.tm.userOf(t, "alice"), "upgrading")
	if err != nil {
		t.Fatal(err)
	}
	f.tm.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, upgrade_id)
		SELECT gen_random_uuid(), n.id, 'edit', 'web', n.created_by_id, now(), now(), CASE WHEN g <= $2::integer THEN $3::uuid END
		FROM notebooks n, generate_series(1, $2::integer + $4::integer) g WHERE n.id = $1`,
		f.book.id, checkCostChangesets, upgrade, checkCostPlain)
	f.tm.exec(t, `INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at)
		SELECT gen_random_uuid(), b.workspace_id, 'Other ' || g, b.created_by_id, b.created_by_id, now(), now()
		FROM notebooks b, generate_series(1, $2::integer) g WHERE b.id = $1`, f.book.id, checkCostOthers)
	f.tm.exec(t, `INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id,
			created_at, updated_at)
		SELECT gen_random_uuid(), o.id, 'page', NULL, 'o' || k, 'o' || k, k, o.created_by_id, o.created_by_id, now(), now()
		FROM notebooks o, generate_series(1, $1::integer) k WHERE o.name LIKE 'Other %'`, checkCostPages)
	f.tm.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, 'x', 1, sha256('x'), 1, created_by_id, now() FROM nodes
		WHERE notebook_id IN (SELECT id FROM notebooks WHERE name LIKE 'Other %')`)
	// Each other notebook's upgrade's id is its id's hash, so that its
	// baseline and its changesets name the same upgrade.
	f.tm.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT md5(n.notebook_id::text)::uuid, n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision,
			c.content_hash, c.byte_size, 0, c.content_hash
		FROM nodes n JOIN page_contents c ON c.node_id = n.id WHERE n.notebook_id IN (SELECT id FROM notebooks WHERE name LIKE 'Other %')`)
	f.tm.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, upgrade_id)
		SELECT gen_random_uuid(), o.id, 'edit', 'web', o.created_by_id, now(), now(), md5(o.id::text)::uuid
		FROM notebooks o, generate_series(1, $1::integer) g WHERE o.name LIKE 'Other %'`, checkCostChangesets)
	f.tm.exec(t, "ANALYZE upgrade_nodes, nodes, page_contents, changesets")
	nodes := count(t, f.tm.pool, "SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1 AND baseline", upgrade)
	lintCostLog(t, "upgrade-check-seed", struct {
		ElapsedNS     int64
		BaselineNodes int
	}{int64(time.Since(started)), nodes})

	actor := f.actor(t, "alice")
	id := uuid.MustParse(f.book.id)
	medians := map[string]time.Duration{}
	for _, seed := range []string{"kept", "a page not kept"} {
		if seed == "a page not kept" {
			f.tm.exec(t, "UPDATE upgrade_nodes SET accepted_hash = sha256('another') WHERE upgrade_id = $1 AND node_id = $2", upgrade,
				f.prototype)
		}
		var took []time.Duration
		for sample := 1; sample <= 3; sample++ {
			checked, elapsed := f.checkSample(t, actor, id, seed, sample)
			took = append(took, elapsed)
			check := checked.Check
			p := check.Gates.Preserved
			switch {
			case check.Counts.Changesets != checkCostChangesets || p.Nodes != nodes || p.NodesKept != nodes:
				t.Fatalf("%s: the check counted %d changesets, %d of %d nodes kept; want %d, every one of %d", seed,
					check.Counts.Changesets, p.NodesKept, p.Nodes, checkCostChangesets, nodes)
			case seed == "kept" && !p.Passed:
				t.Fatalf("%s: gate 7 = %+v, want it passed", seed, p)
			case seed != "kept" && (p.Passed || !slices.Equal(p.NodeIDs, []uuid.UUID{uuid.MustParse(f.prototype)})):
				t.Fatalf("%s: gate 7 = %+v, want the prototype not kept", seed, p)
			}
		}
		slices.Sort(took)
		medians[seed] = took[1]
	}
	lintCostLog(t, "upgrade-check-medians", struct {
		KeptNS, NotKeptNS, BoundNS int64
	}{int64(medians["kept"]), int64(medians["a page not kept"]), int64(checkCostBound)})
	for seed, median := range medians {
		if median > checkCostBound {
			t.Errorf("%s: the median of three checks took %s, past %s", seed, median, checkCostBound)
		}
	}
}

// checkSample is one check of the notebook id by actor under the guard's
// request, which must answer its check, and its wall time; -v prints its
// parts. It waits for the run to finish, so that the next one is a run of
// its own.
func (f *wikiCostFixture) checkSample(t *testing.T, actor context.Context, id uuid.UUID, seed string, sample int,
) (wiki.UpgradeChecked, time.Duration) {
	t.Helper()
	f.trace.reset()
	ctx, cancel := context.WithTimeout(actor, wikiCostRequest)
	defer cancel()
	start := time.Now()
	checked, err := f.module.CheckUpgrade(ctx, id, wiki.LintQuery{})
	elapsed := time.Since(start)
	queries, stages, timings, _ := f.trace.stop()
	var sqlNS int64
	byName := map[string]int64{}
	for _, q := range queries {
		sqlNS += q.ElapsedNS
		byName[q.Name] += q.ElapsedNS
	}
	lintCostLog(t, "upgrade-check", struct {
		Seed                    string
		Sample                  int
		ElapsedNS, SQLNS        int64
		OtherWallNS             int64
		Failed                  bool
		StatementNS             map[string]int64
		Stages                  []wikiCostStage
		Timings                 *wiki.PreparationTimings
		Statements, ResultRules int
	}{seed, sample, int64(elapsed), sqlNS, int64(elapsed) - sqlNS, err != nil, byName, stages, timings, len(queries),
		resultRules(checked)})
	if err != nil || checked.Check == nil {
		t.Fatalf("%s, sample %d: the check = %+v, %v", seed, sample, checked, err)
	}
	var created wiki.RunEvent
	for created.Kind != wiki.RunCreated {
		select {
		case event := <-f.events:
			created = event
		default:
			t.Fatalf("%s, sample %d: the check made no run", seed, sample)
		}
	}
	if created.NotebookID != id {
		t.Fatalf("%s, sample %d: the run of another notebook, %s", seed, sample, created.NotebookID)
	}
	f.event(t, wiki.RunFinished, created.FlightID)
	return checked, elapsed
}

// resultRules is how many rules a check answered, none for none.
func resultRules(c wiki.UpgradeChecked) int {
	if c.Check == nil {
		return 0
	}
	return len(c.Check.Rules)
}
