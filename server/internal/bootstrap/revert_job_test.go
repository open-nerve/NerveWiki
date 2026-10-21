package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The background revert of a batch's changeset, in segments (M9/P3 design
// 3.26): each changeset made through MCP's batch, reverted by a job whose
// batches the test sizes, its batches held on the rows they write.

// asideNames are the names of the notebook nb's nodes alive that are
// named aside.
func (tm acmeTeam) asideNames(t *testing.T, nb string) []string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT name FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL AND name LIKE '% ~%'
		ORDER BY name`, nb)
}

// heldNodes are rows of nodes the test holds FOR UPDATE, each in a
// transaction of its own on a connection of its own, out of the pool's
// few, by the holder's backend: a batch that writes one waits for it,
// holding the notebook, the batches before it committed.
type heldNodes struct {
	tm acmeTeam
	tx map[int32]pgx.Tx
	of map[int32]string
}

func (tm acmeTeam) holdNodes(t *testing.T, ids ...string) heldNodes {
	t.Helper()
	ctx := context.Background()
	h := heldNodes{tm: tm, tx: map[int32]pgx.Tx{}, of: map[int32]string{}}
	for _, id := range ids {
		conn, err := pgx.Connect(ctx, tm.url)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close(ctx) })
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(ctx) })
		var pid int32
		if err := tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM nodes WHERE id = $1 FOR UPDATE", id).Scan(&pid); err != nil {
			t.Fatal(err)
		}
		h.tx[pid], h.of[pid] = tx, id
	}
	return h
}

// waited waits until a batch waits for a row h holds, and answers that
// row's node.
func (h heldNodes) waited(t *testing.T) string {
	t.Helper()
	pids := make([]int32, 0, len(h.tx))
	for pid := range h.tx {
		pids = append(pids, pid)
	}
	for deadline := time.Now().Add(interleavingWait); ; time.Sleep(20 * time.Millisecond) {
		var pid int32
		err := h.tm.pool.QueryRow(context.Background(), `SELECT b FROM pg_stat_activity a CROSS JOIN LATERAL unnest(pg_blocking_pids(a.pid)) b
			WHERE a.datname = current_database() AND b = ANY($1) LIMIT 1`, pids).Scan(&pid)
		switch {
		case err == nil:
			return h.of[pid]
		case !errors.Is(err, pgx.ErrNoRows):
			t.Fatal(err)
		case time.Now().After(deadline):
			t.Fatalf("no batch waits for the rows held, %d of them", len(pids))
		}
	}
}

// letGo ends the holder of the node id's row.
func (h heldNodes) letGo(t *testing.T, id string) {
	t.Helper()
	for pid, of := range h.of {
		if of == id {
			commit(t, h.tx[pid])
			delete(h.tx, pid)
			delete(h.of, pid)
			return
		}
	}
	t.Fatalf("the node %s is not held", id)
}

// inTurn lets each row go once a batch waits for it, see run first each
// time: at the start of each batch, and at each of its writes, the batches
// before it committed.
func (h heldNodes) inTurn(t *testing.T, see func()) {
	t.Helper()
	for len(h.tx) > 0 {
		id := h.waited(t)
		see()
		h.letGo(t, id)
	}
}

// cyclePages are the pages a name cycle's batch changes: O, renamed by
// itself; A, B and C, whose names it turns around; Plan, which it renames
// Old, then makes a page Plan; E, which it edits.
func cyclePages() []restPage {
	return []restPage{{"O", "o"}, {"A", "a"}, {"B", "b"}, {"C", "c"}, {"Plan", "plan"}, {"E", "e"}}
}

// cycleBatch is alice's batch through MCP in the notebook nb seeded with
// cyclePages: O renamed O2; A, B and C's names turned around through T;
// Plan renamed Old, then a page Plan made; and E edited, when edit. Its
// revert puts O back and deletes the new Plan, writes E back, puts Old
// back, then the cycle, a step aside and three places, one segment
// (M9/P3 design 3.26). It answers the changeset and the page it made.
func (tm acmeTeam) cycleBatch(t *testing.T, nb string, edit bool) (string, string) {
	t.Helper()
	ops := []map[string]any{opMove("O.md", "O2.md"), opMove("A.md", "T.md"), opMove("C.md", "A.md"), opMove("B.md", "C.md"),
		opMove("T.md", "B.md"), opMove("Plan.md", "Old.md"), opWrite("Plan.md", "new plan")}
	if edit {
		ops = append(ops, opEdit("E.md", "e", "e by the batch"))
	}
	b := tm.agent(t, "alice", nb).batchDone(t, ops...)
	if b.ChangesetID == nil {
		t.Fatalf("the batch = %+v, want a changeset", b)
	}
	made := queryStrings(t, tm.pool, "SELECT node_id::text FROM changeset_items WHERE changeset_id = $1 AND before_name IS NULL",
		*b.ChangesetID)
	if len(made) != 1 {
		t.Fatalf("the batch made %v, want the new Plan", made)
	}
	return *b.ChangesetID, made[0]
}

// A background revert never cuts a batch within a segment (M9/P3 design
// 3.22.2, 3.26): of a batch's changeset, a rename, then three pages' names
// turned around and a rename, then a page of the old name, in batches of
// two nodes, the cycle's step aside and its places are one batch, though
// four nodes. Each node the job writes is held, in a transaction of its
// own, and let go as a batch waits for it: at the start of each batch, the
// batches before it committed, no node of the notebook is named aside
// (review C-M8, B-N5). The job succeeds; the notebook is as before the
// batch, one changeset more, the job's revert.
func TestTheBackgroundRevertOfANameCycleKeepsItsSegments(t *testing.T) {
	tm := newAcmeTeamWired(t, "member", "", nil, transfer.Batch{Nodes: 2})
	nb := tm.openNotebook(t, "alice", "Eng")
	ids := tm.seed(t, nb, cyclePages()...)
	before := tm.shapeBefore(t, nb)
	c, made := tm.cycleBatch(t, nb, false)
	sets := tm.changesetsIn(t, nb)
	h := tm.holdNodes(t, ids["O"], made, ids["Plan"], ids["A"], ids["B"], ids["C"])
	j := tm.startedJob(t, revertStart("alice", nb, c, ""))
	seen := 0
	h.inTurn(t, func() {
		seen++
		if aside := tm.asideNames(t, nb); len(aside) != 0 {
			t.Errorf("between two batches, at the job's write %d, nodes named aside: %v", seen, aside)
		}
	})
	got := tm.endedBackground(t, "alice", j.ID)
	if got.State != "succeeded" || got.Progress.Done != 7 || got.counts()["put_back"] != 5 || got.counts()["deleted"] != 1 {
		t.Fatalf("the revert = %s, want it succeeded, five nodes put back, the new Plan deleted", got)
	}
	if seen != 6 {
		t.Errorf("the job waited for %d of its writes, want each of the six", seen)
	}
	tm.undoneAs(t, nb, before, c, *got.ChangesetID, sets)
}

// A web's write between two batches of a background revert stops it at
// the next batch that comes to its node, or to a step whose premise it
// breaks (M9/P3 design 3.24, 3.26; 4.15's
// interleaving of a batch's changeset with a name cycle and the writes
// between its batches): the job holds O, its first batch's first write;
// the web's writes wait for the notebook its batch holds, and land after
// it, before the next (PostgreSQL hands the row to its waiters in the
// order they came). The pages', the links' and the search's invariants
// hold after.
func TestTheBackgroundRevertStopsAtAWriteBetweenItsBatches(t *testing.T) {
	// In batches of two, [put O back, delete the new Plan], [write E back,
	// put Old back], then the cycle: the web writes E and renames Old as the
	// first waits. The second's L finds both changed, each its own batch's
	// node (review N-N1), named as in S* (review B-I1); E keeps the web's
	// content, Old the web's name, the first batch's work stays, no node is
	// aside (review B-M3, C-M8).
	t.Run("changed between two batches", func(t *testing.T) {
		tm := newAcmeTeamWired(t, "member", "", nil, transfer.Batch{Nodes: 2})
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := tm.seed(t, nb, cyclePages()...)
		c, made := tm.cycleBatch(t, nb, true)
		h := tm.holdNodes(t, ids["O"])
		j := tm.startedJob(t, revertStart("alice", nb, c, ""))
		h.waited(t)
		e := tm.content(t, "alice", ids["E"])
		wrote := tm.inBackground(t, contentWrite("alice", ids["E"], "e by the web", e.Revision, ""))
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
		renamed := tm.inBackground(t, nodeRename("alice", ids["Plan"], "Older"))
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2, interleavingWait)
		h.letGo(t, ids["O"])
		if a, b := wrote(), renamed(); a.status != http.StatusOK || b.status != http.StatusOK {
			t.Fatalf("the web's write = %d %s, its rename = %d %s; want both done", a.status, a.body, b.status, b.body)
		}
		got := tm.endedBackground(t, "alice", j.ID)
		reasons, names := map[string]string{}, map[string]string{}
		for _, x := range got.Conflicts {
			reasons[x.NodeID], names[x.NodeID] = x.Reason, x.Name
		}
		if got.State != "failed" || got.failure() != "revert_conflict" || got.ConflictCount != 2 || reasons[ids["E"]] != "changed" ||
			reasons[ids["Plan"]] != "changed" || got.counts()["put_back"] != 1 || got.counts()["deleted"] != 1 {
			t.Fatalf("the revert = %s; want it stopped at its second batch, E and Old changed, its first batch's two writes kept", got)
		}
		// Each named as in S*, as the job's start would (review B-I1): Old,
		// renamed Older by the web, is Plan.
		if names[ids["E"]] != "E" || names[ids["Plan"]] != "Plan" {
			t.Errorf("the conflicts named %q and %q, want E and Plan", names[ids["E"]], names[ids["Plan"]])
		}
		if content := tm.content(t, "alice", ids["E"]).Content; content != "e by the web" || tm.placeOf(t, ids["Plan"]) != "/Older" ||
			tm.placeOf(t, ids["O"]) != "/O" || !tm.trashedNow(t, made) {
			t.Errorf("E holds %q, Old at %s, O at %s; want the web's content and name, O put back, the new Plan in the trash", content,
				tm.placeOf(t, ids["Plan"]), tm.placeOf(t, ids["O"]))
		}
		if aside := tm.asideNames(t, nb); len(aside) != 0 {
			t.Errorf("nodes named aside after the job: %v", aside)
		}
		checkPages(t, tm.pool)
		checkLinks(t, tm.pool)
		checkSearch(t, tm.pool)
	})
	// A step a batch: put O back, delete the new Plan, write E back. The web
	// writes E to other bytes as the first waits, then back to the batch's
	// as the second waits, each landing after the batch it waited for: E is
	// two revisions on, at the bytes the batch wrote, which L lets go; the
	// write back is on the revision just checked, and the job ends, E as
	// before the batch. On the job's first Current, the store would refuse
	// it (review N-M3 = NC-M1: a write of the same bytes once makes no
	// revision).
	t.Run("written away and back between batches", func(t *testing.T) {
		tm := newAcmeTeamWired(t, "member", "", nil, transfer.Batch{Nodes: 1})
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := tm.seed(t, nb, cyclePages()...)
		before := tm.shapeBefore(t, nb)
		c, made := tm.cycleBatch(t, nb, true)
		sets := tm.changesetsIn(t, nb)
		h := tm.holdNodes(t, ids["O"], made)
		j := tm.startedJob(t, revertStart("alice", nb, c, ""))
		if at := h.waited(t); at != ids["O"] {
			t.Fatalf("the first batch waits for %s, want O", at)
		}
		e := tm.content(t, "alice", ids["E"])
		away := tm.inBackground(t, contentWrite("alice", ids["E"], "e by the web", e.Revision, ""))
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
		h.letGo(t, ids["O"])
		if a := away(); a.status != http.StatusOK {
			t.Fatalf("the web's write = %d %s", a.status, a.body)
		}
		if at := h.waited(t); at != made {
			t.Fatalf("the second batch waits for %s, want the new Plan", at)
		}
		back := tm.inBackground(t, contentWrite("alice", ids["E"], "e by the batch", e.Revision+1, ""))
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
		h.letGo(t, made)
		if a := back(); a.status != http.StatusOK {
			t.Fatalf("the web's write back = %d %s", a.status, a.body)
		}
		got := tm.endedBackground(t, "alice", j.ID)
		if got.State != "succeeded" {
			t.Fatalf("the revert = %s, want it succeeded", got)
		}
		if now := tm.content(t, "alice", ids["E"]); now.Content != "e" || now.Revision != e.Revision+3 {
			t.Errorf("E holds %q at %d, want its content before the batch at %d", now.Content, now.Revision, e.Revision+3)
		}
		// The web's two writes are changesets of their own, after the job's.
		if n := tm.changesetsIn(t, nb); n != sets+3 {
			t.Errorf("%d changesets, want %d: the job's and the web's two", n, sets+3)
		}
		tm.asItWas(t, nb, before)
		checkPages(t, tm.pool)
		checkLinks(t, tm.pool)
		checkSearch(t, tm.pool)
	})
	// A step a batch, the first holding O: the web's write lands after it,
	// before the batch it meets, whose premise alone sees it, L reading
	// only the batch's own nodes (review C-I1). Each stops the job there,
	// its one conflict the step's, named as in S*, the first batch's work
	// kept, nothing aside.
	for _, tt := range []struct {
		name  string
		seed  []restPage
		ops   []map[string]any
		write func(tm acmeTeam, nb string, ids map[string]string) step
		node  string // the step's node, its S* name the conflict's
		why   string
		taker bool // the web's write makes the holder of a title_taken
	}{
		{"a place's parent deleted between batches", []restPage{{"O", "o"}, {"P", "p"}, {"P/x", "x"}},
			[]map[string]any{opMove("O.md", "O2.md"), opMove("P/x.md", "x.md")},
			func(tm acmeTeam, nb string, ids map[string]string) step { return nodeDeletion("alice", ids["P"]) }, "P/x", "parent_gone", false},
		{"a place's name taken between batches", []restPage{{"O", "o"}, {"Plan", "p"}},
			[]map[string]any{opMove("O.md", "O2.md"), opMove("Plan.md", "Old.md")},
			func(tm acmeTeam, nb string, ids map[string]string) step {
				return request("alice", http.MethodPost, "/api/v0/notebooks/"+nb+"/pages", `{"parent_id":null,"title":"Plan"}`)
			}, "Plan", "title_taken", true},
		{"a restore's parent deleted between batches", []restPage{{"O", "o"}, {"P", "p"}, {"P/z", "z"}},
			[]map[string]any{opMove("O.md", "O2.md"), opDelete("P/z.md")},
			func(tm acmeTeam, nb string, ids map[string]string) step { return nodeDeletion("alice", ids["P"]) }, "P/z", "parent_gone", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newAcmeTeamWired(t, "member", "", nil, transfer.Batch{Nodes: 1})
			nb := tm.openNotebook(t, "alice", "Eng")
			ids := tm.seed(t, nb, tt.seed...)
			b := tm.agent(t, "alice", nb).batchDone(t, tt.ops...)
			h := tm.holdNodes(t, ids["O"])
			j := tm.startedJob(t, revertStart("alice", nb, *b.ChangesetID, ""))
			if at := h.waited(t); at != ids["O"] {
				t.Fatalf("the first batch waits for %s, want O", at)
			}
			wrote := tm.inBackground(t, tt.write(tm, nb, ids))
			pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
			h.letGo(t, ids["O"])
			a := wrote()
			if a.status != http.StatusNoContent && a.status != http.StatusCreated {
				t.Fatalf("the web's write = %d %s", a.status, a.body)
			}
			got := tm.endedBackground(t, "alice", j.ID)
			name := tt.node[strings.LastIndex(tt.node, "/")+1:]
			if got.State != "failed" || got.failure() != "revert_conflict" || len(got.Conflicts) != 1 || got.Conflicts[0].NodeID != ids[tt.node] ||
				got.Conflicts[0].Reason != tt.why || got.Conflicts[0].Name != name || got.counts()["put_back"] != 1 {
				t.Fatalf("the revert = %s; want it stopped at its second batch, %s %s, O put back", got, name, tt.why)
			}
			if holder := got.Conflicts[0].HolderID; tt.taker != (holder != nil) || tt.taker && *holder != idOf(t, a) {
				t.Errorf("the conflict's holder %v, want the web's new page %v", holder, tt.taker)
			}
			if tm.placeOf(t, ids["O"]) != "/O" {
				t.Errorf("O at %s, want it put back", tm.placeOf(t, ids["O"]))
			}
			if aside := tm.asideNames(t, nb); len(aside) != 0 {
				t.Errorf("nodes named aside after the job: %v", aside)
			}
			checkPages(t, tm.pool)
		})
	}
}

// A segment past what a batch writes stops the job, revert_too_large
// (M9/P3 design 3.23, 3.26): G holds 101 pages of 210 links each, then K
// with x and y; a batch renames another page O, moves x and y out, deletes
// G, makes a new G and moves y into it. The revert's segment runs from y's
// step aside to its place, and its restore of G goes as far as K, last in
// G's pre-order: 103 members, 21,210 links, past a unit's 20,000. The job
// stops there; O's rename, the batch before, is undone and stays (review
// C-M8). The report's text is the web's, which vitest checks
// (transfer-report.test.ts).
func TestTheBackgroundRevertOfASegmentPastABatchStops(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	o := tm.createPageWith(t, "alice", nb, "", "O", "o")
	g := tm.createPageWith(t, "alice", nb, "", "G", "g")
	var links strings.Builder
	for i := range 210 {
		fmt.Fprintf(&links, "[[T %d]] ", i)
	}
	for i := range 101 {
		tm.createPageWith(t, "alice", nb, g, fmt.Sprintf("P %d", i), links.String())
	}
	k := tm.createPageWith(t, "alice", nb, g, "K", "k")
	x := tm.createPageWith(t, "alice", nb, k, "x", "x")
	y := tm.createPageWith(t, "alice", nb, k, "y", "y")
	b := tm.agent(t, "alice", nb).batchDone(t, opMove("O.md", "O2.md"), opMove("G/K/x.md", "x.md"), opMove("G/K/y.md", "y.md"),
		opDeleteAll("G.md"), opWrite("G.md", "new g"), opMove("y.md", "G/y.md"))
	if b.ChangesetID == nil {
		t.Fatalf("the batch = %+v, want a changeset", b)
	}
	got := tm.background(t, revertStart("alice", nb, *b.ChangesetID, ""))
	if got.State != "failed" || got.failure() != "revert_too_large" || got.Progress.Done != 1 || got.counts()["put_back"] != 1 ||
		got.counts()["restored"] != 0 {
		t.Fatalf("the revert = %s, want it stopped at its second batch, revert_too_large, O put back", got)
	}
	if tm.placeOf(t, o) != "/O" || !tm.trashedNow(t, g, k) || tm.placeOf(t, x) != "/x" || !strings.HasSuffix(tm.placeOf(t, y), "/y") {
		t.Errorf("O at %s, G in the trash %v, x at %s, y at %s; want O put back, the rest as the batch left it", tm.placeOf(t, o),
			tm.trashedNow(t, g, k), tm.placeOf(t, x), tm.placeOf(t, y))
	}
	if aside := tm.asideNames(t, nb); len(aside) != 0 {
		t.Errorf("nodes named aside after the job: %v", aside)
	}
	checkBackground(t, tm)
}

// A group larger than a batch is restored whole by its batches (M9/P3
// design 3.24, 3.26; review B-I1's fourth, B-I2): in batches of ten nodes,
// a folder F of 15 pages, more than a batch, as production's 100 are of a
// larger one. Each batch reads its group whole when it holds the group's
// first step, or its later members as they are in the trash; checks L,
// then its steps in their order, from the deletions' subtrees at their
// moments; and restores its members of the group at once. Each job
// succeeds: the notebook is as before the batch, the tree, the contents
// and those in the trash, no member left in the trash.
func TestTheBackgroundRevertOfALargeFolderHoldsItsGroup(t *testing.T) {
	tm := newAcmeTeamWired(t, "member", "", nil, transfer.Batch{Nodes: 10})
	agent := tm.agent(t, "alice", "")
	notebooks := 0
	folder := func(names ...string) []restPage {
		pages := []restPage{{"F", "f"}}
		for _, n := range names {
			pages = append(pages, restPage{"F/" + n, n})
		}
		return pages
	}
	numbered := func(n int) []string {
		var out []string
		for i := 1; i <= n; i++ {
			out = append(out, fmt.Sprintf("p%02d", i))
		}
		return out
	}
	each := func(name string, pages []restPage, test func(t *testing.T, nb string, ids map[string]string, m *mcpApp) string) {
		t.Run(name, func(t *testing.T) {
			notebooks++
			nb := tm.openNotebook(t, "alice", fmt.Sprint("Eng ", notebooks))
			ids := tm.seed(t, nb, pages...)
			before := tm.shapeBefore(t, nb)
			c := test(t, nb, ids, agent.in(nb))
			sets := tm.changesetsIn(t, nb)
			got := tm.background(t, revertStart("alice", nb, c, ""))
			if got.State != "succeeded" {
				t.Fatalf("the revert = %s, want it succeeded", got)
			}
			tm.undoneAs(t, nb, before, c, *got.ChangesetID, sets)
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL AND id::text = ANY($2)",
				nb, slices.Collect(maps.Values(ids))); n != 0 {
				t.Errorf("%d of the notebook's pages left in the trash, want none", n)
			}
		})
	}
	batch := func(t *testing.T, m *mcpApp, ops ...map[string]any) string {
		t.Helper()
		b := m.batchDone(t, ops...)
		if b.ChangesetID == nil {
			t.Fatalf("the batch = %+v, want a changeset", b)
		}
		return *b.ChangesetID
	}

	// (1) 4.11's "move out of a folder, then delete it": q's restore, a
	// later member's, and x's place in one batch, the second. A batch that
	// plans S* again finds x's parent gone.
	each("a page moved out of a late page of F, then F deleted", folder(append(numbered(13), "q", "q/x")...),
		func(t *testing.T, nb string, ids map[string]string, m *mcpApp) string {
			return batch(t, m, opMove("F/q/x.md", "x.md"), opDeleteAll("F.md"))
		})
	// (2) z's restore and its write back in the second batch: one that checks
	// it as a page alive finds it changed.
	each("a late page of F edited, then F deleted", folder(append(numbered(14), "z")...),
		func(t *testing.T, nb string, ids map[string]string, m *mcpApp) string {
			return batch(t, m, opEdit("F/z.md", "z", "z by the batch"), opDeleteAll("F.md"))
		})
	// (3) Twelve places back, then D's deletion, in two batches; someone edits
	// a page moved meanwhile. The deletion is of D alone, its subtree at its
	// moment: as the plan read the tree, it would take the pages put back,
	// changed, and the job's start would refuse it, page.locked.
	each("a folder D made, twelve pages moved into it", folder(numbered(15)...),
		func(t *testing.T, nb string, ids map[string]string, m *mcpApp) string {
			ops := []map[string]any{opWrite("D.md", "d")}
			for _, p := range numbered(12) {
				ops = append(ops, opMove("F/"+p+".md", "D/"+p+".md"))
			}
			c := batch(t, m, ops...)
			tm.openSession(t, "bob", ids["F/p01"])
			return c
		})
	// (4) F's write back between F's restore and its pages': the first batch
	// restores F and eight of them at once.
	each("F edited, then deleted", folder(numbered(15)...), func(t *testing.T, nb string, ids map[string]string, m *mcpApp) string {
		return batch(t, m, opEdit("F.md", "f", "f by the batch"), opDeleteAll("F.md"))
	})
}
