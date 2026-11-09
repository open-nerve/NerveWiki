package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// The freeze through the whole program (M11/P1 design 3.4, 7.1): serve
// registers the freeze guard first of the guards and of the vetoers, the
// prechecks and the transfer starts read the upgrade, and each path that
// writes a notebook's content reaches one of them.

// frozenNotebook is a notebook of acme's open to its members as editors,
// alice its admin, holding what each write of the freeze's paths writes:
// Page, "- [ ] b\n" at revision 2, its first version another; Renamed,
// renamed from Other by changeset; and Trashed, a trash item.
type frozenNotebook struct {
	guardedNotebook
	page, other, trashed, changeset string
}

// frozenNotebook is a new frozenNotebook named name, whose upgrade is not
// set yet.
func (tm acmeTeam) frozenNotebook(t *testing.T, name string) frozenNotebook {
	t.Helper()
	nb := tm.openNotebook(t, "alice", name)
	f := frozenNotebook{guardedNotebook: guardedNotebook{name: name, id: nb}}
	f.page = tm.createPageWith(t, "alice", nb, "", "Page", "- [ ] a\n")
	tm.send(t, contentWrite("alice", f.page, "- [ ] b\n", 1, ""), http.StatusOK)
	f.other = tm.createPage(t, "alice", nb, "", "Other")
	f.trashed = tm.createPage(t, "alice", nb, "", "Trashed")
	tm.send(t, nodeDeletion("alice", f.trashed), http.StatusNoContent)
	tm.send(t, nodeRename("alice", f.other, "Renamed"), http.StatusOK)
	f.changeset = tm.newest(t, nb).ID
	return f
}

// frozenWrite is a request of a path the freeze refuses, by its
// operation.
type frozenWrite struct {
	op string
	c  step
}

// writes are by's requests of the fourteen REST operations that write a
// notebook's content (M11 design 5), each one the freeze alone would
// refuse in f: the nine through a unit, the opening of an edit session,
// the upload and the three starts of a job.
func (f frozenNotebook) writes(t *testing.T, by string) []frozenWrite {
	t.Helper()
	return []frozenWrite{
		{"createPage", pageCreation(by, f.id, "New")},
		{"putPageContent", contentWrite(by, f.page, "- [ ] c\n", 2, "")},
		{"openEditSession", sessionOpening(by, f.page)},
		{"restorePageVersion", versionRestore(by, f.page, 1, 2)},
		{"toggleTask", taskToggle(by, f.page, 2, 3, true)},
		{"renameNode", nodeRename(by, f.other, "Again")},
		{"deleteNode", nodeDeletion(by, f.other)},
		{"moveNode", nodeMove(by, f.other, f.page)},
		{"restoreNode", nodeRestore(by, f.trashed)},
		{"revertChangeset", changesetRevert(by, f.changeset)},
		{"uploadAsset", assetUpload(t, by, f.id, "", "pic.png", pngHead)},
		{"startImport", importStep(t, by, f.id, "", flatImport(t, 1))},
		{"startTrashRestore", trashRestoreStart(by, f.id, f.trashed)},
		{"startRevert", revertStart(by, f.id, f.changeset, "")},
	}
}

// frozenTools are the five MCP writes, as an agent sends them in a
// frozenNotebook, each of the page named prefix that its write makes.
func frozenTools(prefix string) []struct {
	tool string
	args map[string]any
} {
	return []struct {
		tool string
		args map[string]any
	}{
		{"write", map[string]any{"path": prefix + ".md", "content": "x"}},
		{"edit", map[string]any{"path": prefix + ".md", "old_string": "x", "new_string": "y"}},
		{"move", map[string]any{"from": prefix + ".md", "to": prefix + " moved.md"}},
		{"delete", map[string]any{"path": prefix + " moved.md"}},
		{"batch", batched(opWrite(prefix+" batched.md", "z"))},
	}
}

// refusesAll sends each of writes, each answered 409 notebook.upgrading,
// leaving the notebook's rows and its jobs as they were.
func (tm acmeTeam) refusesAll(t *testing.T, f frozenNotebook, who string, writes []frozenWrite) {
	t.Helper()
	jobs := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", f.id)
	for _, w := range writes {
		tm.guarded(t, f.guardedNotebook, who+"'s "+w.op, w.c, cell{http.StatusConflict, "notebook.upgrading"})
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", f.id); n != jobs {
		t.Errorf("%s: %s's refused starts left %d jobs, want %d", f.name, who, n, jobs)
	}
}

// toolsRefused makes each of the five MCP writes as agent in f, of its
// pages, each notebook.upgrading before it writes anything.
func (tm acmeTeam) toolsRefused(t *testing.T, f frozenNotebook, who string, agent *mcpApp) {
	t.Helper()
	for _, c := range []struct {
		tool string
		args map[string]any
	}{
		{"write", map[string]any{"path": "New.md", "content": "x"}},
		{"edit", map[string]any{"path": "Page.md", "old_string": "b", "new_string": "c"}},
		{"move", map[string]any{"from": "Renamed.md", "to": "Moved.md"}},
		{"delete", map[string]any{"path": "Renamed.md"}},
		{"batch", batched(opWrite("New.md", "x"))},
	} {
		before := tm.guardRows(t, f.id)
		if member := agent.use(t, c.tool, c.args).toolError(t); member["code"] != "notebook.upgrading" {
			t.Errorf("%s: %s's %s = %v, want notebook.upgrading", f.name, who, c.tool, member)
		}
		if after := tm.guardRows(t, f.id); after != before {
			t.Errorf("%s: %s's %s, refused, left the rows %s, want %s", f.name, who, c.tool, after, before)
		}
	}
}

// toolsDone makes each of frozenTools as agent in f, each done.
func toolsDone(t *testing.T, f frozenNotebook, who string, agent *mcpApp) {
	t.Helper()
	for _, c := range frozenTools("Agent") {
		if r := agent.use(t, c.tool, c.args).result(t); r["isError"] == true {
			t.Errorf("%s: %s's %s = %v, want it done", f.name, who, c.tool, r)
		}
	}
}

// The freeze reaches every path that writes a notebook's content (M11/P1
// design 3.4, 3.24, 7.1). In a notebook being upgraded by alice, upgrading,
// reviewing or rolling back: bob's fourteen REST writes, the opening of an
// edit session and the starts of an import, a restore and a revert among
// them, are 409 notebook.upgrading, and so are alice's, the initiator's,
// from REST; bob's agent's five MCP writes are, and alice's while
// upgrading alone are let by. Each refusal leaves the rows and the jobs as
// they were. Then: the paths only a precheck answers, as another code
// would before the unit does; a job's batch after the upgrade began,
// which is refused; and, the freeze taken out of the guards and the
// vetoers, the writes the units and the openings refused succeed.
func TestUpgradeFreezeReachesEveryPath(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	for _, state := range []string{"upgrading", "reviewing", "rolling_back"} {
		t.Run(state, func(t *testing.T) {
			f := tm.frozenNotebook(t, state)
			tm.upgrade(t, f.id, "alice", state)
			tm.refusesAll(t, f, "bob", f.writes(t, "bob"))
			tm.refusesAll(t, f, "alice", f.writes(t, "alice"))
			tm.toolsRefused(t, f, "bob", tm.agent(t, "bob", f.id))
			alice := tm.agent(t, "alice", f.id)
			if state == "upgrading" {
				toolsDone(t, f, "alice", alice)
			} else {
				tm.toolsRefused(t, f, "alice", alice)
			}
		})
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)

	t.Run("only the prechecks answer", prechecksAnswerTheFreeze)
	t.Run("the jobs' batches", jobsBatchesMeetTheFreeze)
	t.Run("without the freeze", writesWithoutTheFreeze)
}

// prechecksAnswerTheFreeze sends the paths where the freeze is the
// prechecks' alone, another code answering after them before any unit
// (M11/P1 design 3.24): an upload and an import with the store keeping no
// room, a background restore of an item past its period, a background
// revert of a changeset past MaxJobItems. Being upgraded, each is
// notebook.upgrading; once the upgrade ended, each is the other code.
func prechecksAnswerTheFreeze(t *testing.T) {
	retention := 24 * time.Hour
	tm := newAcmeTeamOn(t, "member", "", func(c *config.Config) {
		c.Storage.MinFreeBytes, c.Jobs.PurgeRetention = 1<<60, retention
	}, &wiring{page: func(d *page.Deps) { d.MaxJobItems = 1 }}, slog.New(slog.DiscardHandler))
	nb := tm.openNotebook(t, "alice", "Eng")
	expired := tm.createPage(t, "alice", nb, "", "Expired")
	tm.send(t, nodeDeletion("alice", expired), http.StatusNoContent)
	tm.trashedAgo(t, expired, retention-30*time.Minute)
	two := tm.agent(t, "alice", nb).batchDone(t, opWrite("A.md", "a"), opWrite("B.md", "b"))
	if two.ChangesetID == nil {
		t.Fatal("the batch of two wrote no changeset")
	}
	g := guardedNotebook{name: "the notebook being upgraded", id: nb}
	requests := []struct {
		what string
		c    step
		then cell
	}{
		{"an upload, the store full", assetUpload(t, "bob", nb, "", "pic.png", pngHead),
			cell{http.StatusInsufficientStorage, "storage_full"}},
		{"an import, the store full", importStep(t, "bob", nb, "", flatImport(t, 1)), cell{http.StatusInsufficientStorage, "storage_full"}},
		{"a restore of an item past its period", trashRestoreStart("bob", nb, expired), cell{http.StatusConflict, "page.restore_expired"}},
		{"a revert of two items, past MaxJobItems", revertStart("bob", nb, *two.ChangesetID, ""),
			cell{http.StatusUnprocessableEntity, "page.revert_too_large"}},
	}
	tm.upgrade(t, nb, "alice", "upgrading")
	for _, r := range requests {
		tm.guarded(t, g, r.what, r.c, cell{http.StatusConflict, "notebook.upgrading"})
	}
	tm.endUpgrade(t, nb)
	g.name = "the notebook once its upgrade ended"
	for _, r := range requests {
		tm.guarded(t, g, r.what, r.c, r.then)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", nb); n != 0 {
		t.Errorf("the refused starts left %d jobs", n)
	}
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// upgradeBetweenBatches is the transfer module's Tree, which, for the
// notebook armed, sets its upgrade as the job's second batch calls it,
// before it runs (M11/P1 design 7.1, review A-N6): as River runs it,
// after the job's retry saw it running.
type upgradeBetweenBatches struct {
	transfer.Tree
	mu      sync.Mutex
	armed   uuid.UUID
	calls   int
	upgrade func(ctx context.Context, nb uuid.UUID) error
}

// arm sets the notebook nb's upgrade at the second batch of its next job.
func (u *upgradeBetweenBatches) arm(nb string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.armed, u.calls = uuid.MustParse(nb), 0
}

// batch counts a batch of the notebook nb, and at the armed one's second
// sets its upgrade, disarmed.
func (u *upgradeBetweenBatches) batch(ctx context.Context, nb uuid.UUID) error {
	u.mu.Lock()
	if nb != u.armed {
		u.mu.Unlock()
		return nil
	}
	u.calls++
	second := u.calls == 2
	if second {
		u.armed = uuid.UUID{}
	}
	u.mu.Unlock()
	if !second {
		return nil
	}
	return u.upgrade(ctx, nb)
}

func (u *upgradeBetweenBatches) Import(ctx context.Context, spec transfer.ImportSpec,
	do func(ctx context.Context, unit transfer.ImportUnit) error,
) (uuid.UUID, error) {
	if err := u.batch(ctx, spec.NotebookID); err != nil {
		return uuid.UUID{}, err
	}
	return u.Tree.Import(ctx, spec, do)
}

func (u *upgradeBetweenBatches) RunBatch(ctx context.Context, spec transfer.JobSpec, p transfer.JobPlan, from, to int) (
	transfer.BatchDone, error,
) {
	if err := u.batch(ctx, spec.NotebookID); err != nil {
		return transfer.BatchDone{}, err
	}
	return u.Tree.RunBatch(ctx, spec, p, from, to)
}

// jobsBatchesMeetTheFreeze stops an import, a restore and a revert, each
// of three nodes in batches of one, between their first and second
// batches, and sets their notebook's upgrade then, as no start may: the
// second batch's unit is refused, the job failed refused,
// notebook.upgrading, the first batch's node written and nothing of the
// second (M11/P1 design 7.1, review A-N6). So is a restore's whose
// notebook starts rolling back then (review S1C-N1); an upgrade's own
// rollback, in batches of one, writes each of its batches as the
// notebook rolls back.
func jobsBatchesMeetTheFreeze(t *testing.T) {
	stop := &upgradeBetweenBatches{}
	tm := newAcmeTeamWiring(t, "member", "", wiring{batch: transfer.Batch{Nodes: 1}, transfer: func(d *transfer.Deps) {
		stop.Tree, d.Tree = d.Tree, stop
	}})
	alice := tm.userOf(t, "alice")
	state := "upgrading"
	stop.upgrade = func(ctx context.Context, nb uuid.UUID) error {
		_, err := upgradeIn(ctx, tm.pool, nb.String(), alice, state)
		return err
	}
	alive := func(nb string) int {
		return count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", nb)
	}
	check := func(what string, j backgroundJob, nb string, want int) {
		t.Helper()
		if j.State != "failed" || j.failure() != "refused" || j.Report.Refusal == nil || j.Report.Refusal.Code != "notebook.upgrading" ||
			j.Progress.Done != 1 {
			t.Errorf("%s = %s, want it failed refused notebook.upgrading after one batch", what, j)
		}
		if n := alive(nb); n != want {
			t.Errorf("%s: %d nodes alive, want %d, the first batch's alone written", what, n, want)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM upgrade_nodes u JOIN notebooks n ON n.upgrade_id = u.upgrade_id WHERE n.id = $1",
			nb); n == 0 {
			t.Errorf("%s: its notebook has no upgrade", what)
		}
	}

	imports := tm.openNotebook(t, "alice", "Imports")
	stop.arm(imports)
	started := tm.started(t, importStep(t, "alice", imports, "", flatImport(t, 3)))
	check("the import", tm.endedBackground(t, "alice", started.ID), imports, 1)

	restores := tm.openNotebook(t, "alice", "Restores")
	box := tm.createPage(t, "alice", restores, "", "Box")
	for _, name := range []string{"c1", "c2"} {
		tm.createPage(t, "alice", restores, box, name)
	}
	tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
	stop.arm(restores)
	check("the restore", tm.background(t, trashRestoreStart("alice", restores, box)), restores, 1)

	reverts := tm.openNotebook(t, "alice", "Reverts")
	if j := tm.imported(t, "alice", reverts, "", flatImport(t, 3)); j.State != "succeeded" {
		t.Fatalf("the import to revert = %+v", j)
	}
	imported := tm.newest(t, reverts).ID
	stop.arm(reverts)
	check("the revert", tm.background(t, revertStart("alice", reverts, imported, "")), reverts, 2)

	rolling := tm.openNotebook(t, "alice", "Rolling back")
	box = tm.createPage(t, "alice", rolling, "", "Box")
	for _, name := range []string{"c1", "c2"} {
		tm.createPage(t, "alice", rolling, box, name)
	}
	tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
	state = "rolling_back"
	stop.arm(rolling)
	check("the restore as the notebook rolls back", tm.background(t, trashRestoreStart("alice", rolling, box)), rolling, 1)

	own := tm.openNotebook(t, "alice", "Own")
	tm.createPage(t, "alice", own, "", "Guide")
	tm.upgraded(t, "alice", own)
	for _, path := range []string{"A.md", "B.md", "C.md"} {
		if r := tm.agent(t, "alice", own).use(t, "write", map[string]any{"path": path, "content": "x"}).result(t); r["isError"] == true {
			t.Fatalf("the agent's write of %s = %v", path, r)
		}
	}
	job := tm.abandoned(t, "alice", own)
	if j := tm.endedBackground(t, "alice", job); j.State != "succeeded" || j.Progress.Done != 5 {
		t.Errorf("the rollback in batches of one = %s, want it succeeded, its five new nodes deleted", j)
	}
	checkPages(t, tm.pool)
	checkBackground(t, tm)
}

// writesWithoutTheFreeze takes the freeze out of the page module's guards
// and vetoers as the composition root registered them: in a notebook
// being upgraded by alice, bob's writes through a unit, his opening of an
// edit session and his agent's five writes succeed (M11/P1 design 2.1
// item 7).
func writesWithoutTheFreeze(t *testing.T) {
	var guards, vetoers int
	tm := newAcmeTeamWiring(t, "member", "", wiring{page: func(d *page.Deps) {
		frozen := func(x any) bool { _, ok := x.(*page.FreezeGuard); return ok }
		guards, vetoers = len(d.Guards), len(d.EditSessionVetoers)
		d.Guards = slices.DeleteFunc(d.Guards, func(g page.WriteGuard) bool { return frozen(g) })
		d.EditSessionVetoers = slices.DeleteFunc(d.EditSessionVetoers, func(v page.EditSessionVetoer) bool { return frozen(v) })
		guards, vetoers = guards-len(d.Guards), vetoers-len(d.EditSessionVetoers)
	}})
	if guards != 1 || vetoers != 1 {
		t.Fatalf("took %d freeze guards out of the guards, %d out of the vetoers; want one each", guards, vetoers)
	}
	f := tm.frozenNotebook(t, "Unfrozen")
	tm.upgrade(t, f.id, "alice", "upgrading")
	for _, w := range []struct {
		what string
		c    step
		want int
	}{
		{"createPage", pageCreation("bob", f.id, "New"), http.StatusCreated},
		{"revertChangeset", changesetRevert("bob", f.changeset), http.StatusOK},
		{"putPageContent", contentWrite("bob", f.page, "- [ ] c\n", 2, ""), http.StatusOK},
		{"toggleTask", taskToggle("bob", f.page, 3, 3, true), http.StatusOK},
		{"restorePageVersion", versionRestore("bob", f.page, 1, 4), http.StatusOK},
		{"renameNode", nodeRename("bob", f.other, "Again"), http.StatusOK},
		{"moveNode", nodeMove("bob", f.other, f.page), http.StatusOK},
		{"restoreNode", nodeRestore("bob", f.trashed), http.StatusOK},
		{"deleteNode", nodeDeletion("bob", f.other), http.StatusNoContent},
	} {
		if status, answer := askTyped(t, tm.contract, w.c.method, tm.base+w.c.path, tm.tokens["bob"], w.c.contentType, w.c.body); status != w.want {
			t.Errorf("without the freeze, bob's %s = %d %s, want %d", w.what, status, answer, w.want)
		}
	}
	toolsDone(t, f, "bob", tm.agent(t, "bob", f.id))
	tm.send(t, sessionOpening("bob", f.page), http.StatusCreated)
	checkPages(t, tm.pool)
}
