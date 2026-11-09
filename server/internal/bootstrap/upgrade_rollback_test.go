package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// An upgrade's abandonment, its rollback and the rollback's end through
// the whole program (M11/P1 design 3.18–3.20, 7.1); the wiki module's
// half with fakes is wiki/app's TestAbandonUpgrade and
// TestEndUpgradeRollback, the transfer module's transfer/app's
// TestARollbackRunsPassByPass and TestARollbacksEnd.

// abandon is by's abandonment of nb's upgrade, or the continuation of its
// rollback, as the API answers it.
func (tm acmeTeam) abandon(t *testing.T, by, nb string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade/abandon", tm.tokens[by], "")
}

// endRollback is by's end of nb's upgrade's rollback, as the API answers
// it.
func (tm acmeTeam) endRollback(t *testing.T, by, nb string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade/end-rollback", tm.tokens[by], "")
}

// rollbackAnswer is an Upgrade answer of a rollback, as much as the tests
// check.
type rollbackAnswer struct {
	ID               string  `json:"id"`
	State            string  `json:"state"`
	Ending           *string `json:"ending"`
	RollbackJobID    *string `json:"rollback_job_id"`
	RollbackComplete *bool   `json:"rollback_complete"`
	RollbackLeft     *struct {
		Nodes   int `json:"nodes"`
		Pages   int `json:"pages"`
		Created int `json:"created"`
	} `json:"rollback_left"`
}

// left is the answer's rollback_left as "nodes pages created", "null" for
// none.
func (a rollbackAnswer) left() string {
	if a.RollbackLeft == nil {
		return "null"
	}
	l := a.RollbackLeft
	return fmt.Sprintf("%d %d %d", l.Nodes, l.Pages, l.Created)
}

// abandoned is by's abandonment of nb's upgrade, or the continuation of
// its rollback, which must answer 200, rolling back, abandoned, with a
// job; it answers the job's id.
func (tm acmeTeam) abandoned(t *testing.T, by, nb string) string {
	t.Helper()
	status, answer := tm.abandon(t, by, nb)
	var a rollbackAnswer
	if status == http.StatusOK {
		decodeAnswer(t, answer, &a)
	}
	if status != http.StatusOK || a.State != "rolling_back" || a.Ending == nil || *a.Ending != "abandoned" || a.RollbackJobID == nil {
		t.Fatalf("%s's abandonment of %s = %d %s, want it rolling back, abandoned, with a job", by, nb, status, answer)
	}
	return *a.RollbackJobID
}

// rollbackStops is the transfer module's Tree, the rollback of the
// notebook armed stopped at the plan or the batch armed, or once a plan
// found nothing left when done, its batches writing nothing when idle:
// the stop signals reached and waits for release. It stops after the
// run's retry read the job running (background.go), so the call goes on
// once released.
type rollbackStops struct {
	transfer.Tree
	mu             sync.Mutex
	nb             uuid.UUID
	plan, batch    int
	done, idle     bool
	plans, batches int
	reached        chan struct{}
	release        chan struct{}
}

// arm stops nb's rollback at its plan-th plan or its batch-th batch, 0
// for none, counting from now; idle makes its batches write nothing.
func (s *rollbackStops) arm(nb string, plan, batch int, idle bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nb, s.plan, s.batch, s.done, s.idle, s.plans, s.batches = uuid.MustParse(nb), plan, batch, false, idle, 0, 0
	s.reached, s.release = make(chan struct{}), make(chan struct{})
}

// armDone stops nb's rollback once a plan found nothing left, before its
// end.
func (s *rollbackStops) armDone(nb string) {
	s.arm(nb, 0, 0, false)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = true
}

// at stops a call of kind, "plan" or "batch", in nb where it is armed,
// and answers whether nb's batches are idle.
func (s *rollbackStops) at(nb uuid.UUID, kind string) bool {
	s.mu.Lock()
	if nb != s.nb {
		s.mu.Unlock()
		return false
	}
	n, at := &s.plans, s.plan
	if kind == "batch" {
		n, at = &s.batches, s.batch
	}
	*n++
	stop := at != 0 && *n == at
	reached, release, idle := s.reached, s.release, s.idle
	s.mu.Unlock()
	if stop {
		close(reached)
		<-release
	}
	return idle
}

func (s *rollbackStops) PlanRollback(ctx context.Context, spec transfer.JobSpec) (transfer.JobPlan, transfer.RollbackPass, error) {
	s.at(spec.NotebookID, "plan")
	p, pass, err := s.Tree.PlanRollback(ctx, spec)
	s.mu.Lock()
	stop := err == nil && pass.Done && s.done && spec.NotebookID == s.nb
	reached, release := s.reached, s.release
	s.mu.Unlock()
	if stop {
		close(reached)
		<-release
	}
	return p, pass, err
}

func (s *rollbackStops) RunBatch(ctx context.Context, spec transfer.JobSpec, p transfer.JobPlan, from, to int) (transfer.BatchDone, error) {
	if spec.UpgradeID != (uuid.UUID{}) && s.at(spec.NotebookID, "batch") {
		return transfer.BatchDone{}, nil
	}
	return s.Tree.RunBatch(ctx, spec, p, from, to)
}

// counts are the plans and the batches of the notebook armed since.
func (s *rollbackStops) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plans, s.batches
}

// stopped waits until the armed call stops.
func (s *rollbackStops) stopped(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	reached := s.reached
	s.mu.Unlock()
	select {
	case <-reached:
	case <-time.After(interleavingWait):
		t.Fatal("the rollback did not reach its stop")
	}
}

// goOn releases the stopped call.
func (s *rollbackStops) goOn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.release)
}

// rollbackTeam is acme on an app whose rollbacks the test stops, which
// logs to its buffer; change alters its configuration when not nil.
func rollbackTeam(t *testing.T, change func(*config.Config)) (acmeTeam, *rollbackStops, *syncBuffer) {
	t.Helper()
	stops, logs := &rollbackStops{}, &syncBuffer{}
	w := wiring{transfer: func(d *transfer.Deps) { stops.Tree, d.Tree = d.Tree, stops }}
	return newAcmeTeamOn(t, "member", "", change, &w, slog.New(slog.NewJSONHandler(logs, nil))), stops, logs
}

// rollbackNotebook is a notebook alice upgrades, its agent's writes made:
// Guide moved under Made, which it wrote with Inner; Folder/Kept renamed
// Kept 2, Guide's link to it rewritten by the system, its link to Kept 2,
// wanted before, now Kept's too; Notes' frontmatter changed; Fresh,
// Folder/Child and Box with One and Two written; then the agent's more.
// Kept 2 named back Kept must leave Guide's links as they are (a
// rollback's moves rewrite none). It answers the notebook, the upgrade's
// id and the notebook's state before the start.
func (tm acmeTeam) rollbackNotebook(t *testing.T, name string, more ...agentWrite) (string, string, string) {
	t.Helper()
	nb := tm.openNotebook(t, "alice", name)
	tm.createPageWith(t, "alice", nb, "", "Guide", "guide [[Notes]] [[Kept]] [[Kept 2]]\n")
	folder := tm.createPage(t, "alice", nb, "", "Folder")
	tm.createPageWith(t, "alice", nb, folder, "Kept", "kept\n")
	tm.createPageWith(t, "alice", nb, "", "Notes", "---\ntags: [a]\n---\nnotes\n")
	before := tm.notebookState(t, nb)
	u := tm.upgraded(t, "alice", nb)
	agent := tm.agent(t, "alice", nb)
	for _, w := range append([]agentWrite{
		{"write", map[string]any{"path": "Made/Inner.md", "content": "inner\n"}},
		{"move", map[string]any{"from": "Guide.md", "to": "Made/Guide.md"}},
		{"move", map[string]any{"from": "Folder/Kept.md", "to": "Folder/Kept 2.md"}},
		{"edit", map[string]any{"path": "Notes.md", "old_string": "tags: [a]", "new_string": "tags: [b]"}},
		{"write", map[string]any{"path": "Fresh.md", "content": "fresh\n"}},
		{"write", map[string]any{"path": "Folder/Child.md", "content": "child\n"}},
		{"write", map[string]any{"path": "Box/One.md", "content": "one\n"}},
		{"write", map[string]any{"path": "Box/Two.md", "content": "two\n"}},
	}, more...) {
		if r := agent.use(t, w.tool, w.args).result(t); r["isError"] == true {
			t.Fatalf("the agent's %s %v = %v", w.tool, w.args, r)
		}
	}
	return nb, u.ID, before
}

// agentWrite is a call of an agent's tool, which must not answer an error.
type agentWrite struct {
	tool string
	args map[string]any
}

// rolledBack checks that nb's upgrade up ended rolled back, complete as
// complete says, its rollback's job job: the notebook's upgrade cleared,
// its baseline deleted, the wiki row's state.
func (tm acmeTeam) rolledBack(t *testing.T, nb, up, job string, complete bool) {
	t.Helper()
	if got := queryStrings(t, tm.pool, `SELECT concat_ws(' ', u.state, u.ending, u.rollback_job_id, u.rollback_complete::text,
			(n.upgrade_id IS NULL)::text, (SELECT count(*) FROM upgrade_nodes b WHERE b.upgrade_id = u.id))
		FROM notebook_upgrades u JOIN notebooks n ON n.id = u.notebook_id WHERE u.id = $1`, up); !slices.Equal(got,
		[]string{fmt.Sprintf("rolled_back abandoned %s %t true 0", job, complete)}) {
		t.Errorf("the upgrade %s = %q, want rolled back, abandoned, its job %s, complete %v, the notebook's upgrade cleared, no "+
			"baseline", up, got, job, complete)
	}
}

// rollbackCase is the team, the stops and the log TestUpgradeRollback's
// cases share, each in a notebook of its own.
type rollbackCase struct {
	tm    acmeTeam
	stops *rollbackStops
	logs  *syncBuffer
}

// mark is how many lines the log holds: a case reads its own after it.
func (r rollbackCase) mark(t *testing.T) int {
	t.Helper()
	return len((&mcpApp{logs: r.logs}).logLines(t))
}

// rollbackOf reads nb's upgrade as alice, which must answer 200.
func (tm acmeTeam) rollbackOf(t *testing.T, nb string) rollbackAnswer {
	t.Helper()
	status, answer := tm.getUpgrade(t, "alice", nb)
	if status != http.StatusOK {
		t.Fatalf("the upgrade of %s = %d %s", nb, status, answer)
	}
	var a rollbackAnswer
	decodeAnswer(t, answer, &a)
	return a
}

// frozen checks that bob's page creation in nb is notebook.upgrading.
func (tm acmeTeam) frozen(t *testing.T, nb, what string) {
	t.Helper()
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/pages", tm.tokens["bob"],
		`{"parent_id":null,"title":"Frozen"}`)
	wantCode(t, what+": bob's page", status, answer, cell{http.StatusConflict, "notebook.upgrading"})
}

// The rollback through the whole program (M11/P1 design 7.1), each case
// in a notebook of its own on one app, but the timeout's.
func TestUpgradeRollback(t *testing.T) {
	tm, stops, logs := rollbackTeam(t, nil)
	r := rollbackCase{tm: tm, stops: stops, logs: logs}
	t.Run("abandoned and done", r.abandonedAndDone)
	t.Run("busy while it runs", r.busyWhileItRuns)
	t.Run("stopped and continued", r.stoppedAndContinued)
	t.Run("stopped and ended short", r.stoppedAndEndedShort)
	t.Run("ended with nothing or a rename left", r.endedEarly)
	t.Run("a pass that leaves as much", r.aPassThatLeavesAsMuch)
	t.Run("its starter demoted", r.itsStarterDemoted)
	t.Run("the upgrade ended between passes", r.endedBetweenPasses)
	t.Run("queued and dropped", r.queuedAndDropped)
	t.Run("continued after a timeout", rollbackContinuedAfterATimeout)
	checkPages(t, tm.pool)
	checkBackground(t, tm)
}

// abandonedAndDone abandons an upgrade whose agent moved, renamed,
// rewrote and wrote: the job deletes the new nodes that hold no baseline
// node first, in one pass, many subtrees in one unit, then puts the rest
// back in one pass from the baseline, Notes' order among them (the agent
// moved it away and back, its parent and name as they were); the notebook
// is as before the start,
// the new pages in the trash, in one rollback changeset of the upgrade; a
// "rollback pass" line for each pass; the upgrade ends rolled back,
// complete, its info lines logged; the freeze is lifted. That the
// rollback's end ends the upgrade is the composition root's: without
// RollbackEnd, the job would end the notebook still rolling back.
func (r rollbackCase) abandonedAndDone(t *testing.T) {
	tm, mark := r.tm, r.mark(t)
	nb, up, before := tm.rollbackNotebook(t, "Done", agentWrite{"move", map[string]any{"from": "Notes.md", "to": "Folder/Notes.md"}},
		agentWrite{"move", map[string]any{"from": "Folder/Notes.md", "to": "Notes.md"}})
	if got := queryStrings(t, tm.pool, `SELECT concat_ws(' ', n.parent_id IS NOT DISTINCT FROM b.parent_id, n.name = b.name,
			n.sort_order = b.sort_order) FROM nodes n JOIN upgrade_nodes b ON b.node_id = n.id WHERE n.id =
			(SELECT id FROM nodes WHERE notebook_id = $1 AND name = 'Notes')`, nb); !slices.Equal(got, []string{"t t f"}) {
		t.Fatalf("Notes against its baseline (parent, name, order) = %q, want only its order changed", got)
	}
	r.stops.arm(nb, 0, 0, false)
	job := tm.abandoned(t, "alice", nb)
	j := tm.endedBackground(t, "alice", job)
	if j.State != "succeeded" || j.Kind != "rollback" || j.Name != "upgrade" || j.ChangesetID == nil {
		t.Fatalf("the rollback = %s, want it succeeded", j)
	}
	if now := tm.notebookState(t, nb); now != before {
		t.Errorf("the notebook after the rollback:\n%s\nwant as before the start:\n%s", now, before)
	}
	if got := queryStrings(t, tm.pool, `SELECT name FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL ORDER BY name`,
		nb); !slices.Equal(got, []string{"AGENTS", "Box", "Child", "Fresh", "Inner", "Made", "One", "Two", "raw"}) {
		t.Errorf("deleted %q, want the new nodes", got)
	}
	if got := queryStrings(t, tm.pool, `SELECT id::text FROM changesets WHERE notebook_id = $1 AND kind = 'rollback' AND upgrade_id = $2`,
		nb, up); !slices.Equal(got, []string{*j.ChangesetID}) {
		t.Errorf("the rollback changesets %q, want the job's %s, of the upgrade", got, *j.ChangesetID)
	}
	passes := upgradeLines(t, r.logs, mark, "rollback pass")
	if len(passes) != 2 || passes[0]["deleted_first"] != float64(8) || passes[0]["steps"] != float64(1) ||
		passes[1]["deleted_first"] != float64(0) || passes[1]["items"].(float64) == 0 || passes[0]["job_id"] != job ||
		passes[1]["upgrade_id"] != up {
		t.Errorf("rollback passes %v, want one deleting the 8 new nodes first in one step, then one from the baseline", passes)
	}
	// The first pass's eight nodes, of six subtrees, in one unit.
	if _, batches := r.stops.counts(); batches != 2 {
		t.Errorf("%d batches, want one a pass", batches)
	}
	if got := queryStrings(t, tm.pool, `SELECT count(*) || ' ' || count(DISTINCT i.created_at) FROM changeset_items i
		JOIN nodes n ON n.id = i.node_id WHERE i.changeset_id = $1 AND i.after_name IS NULL AND n.name <> 'Made'`,
		*j.ChangesetID); !slices.Equal(got, []string{"8 1"}) {
		t.Errorf("the deleted first %q, want 8 items of one unit", got)
	}
	tm.rolledBack(t, nb, up, job, true)
	for _, msg := range []string{"upgrade abandoned", "upgrade rolled back", "rollback ended"} {
		if lines := upgradeLines(t, r.logs, mark, msg); len(lines) != 1 || lines[0]["level"] != "INFO" {
			t.Errorf("logged %q %v, want one info line", msg, lines)
		}
	}
	tm.createPage(t, "bob", nb, "", "After")
	status, answer := tm.getUpgrade(t, "alice", nb)
	wantCode(t, "the upgrade read after its end", status, answer, cell{http.StatusNotFound, "wiki.upgrade_not_found"})
}

// busyWhileItRuns stops the rollback at its first batch: continuing it
// and ending it are transfer.busy; its read tells nothing left, its job
// running; the freeze holds. Let go, it is done.
func (r rollbackCase) busyWhileItRuns(t *testing.T) {
	tm := r.tm
	nb, up, before := tm.rollbackNotebook(t, "Busy")
	r.stops.arm(nb, 0, 1, false)
	job := tm.abandoned(t, "alice", nb)
	r.stops.stopped(t)
	status, answer := tm.abandon(t, "alice", nb)
	wantCode(t, "a continuation while it runs", status, answer, cell{http.StatusConflict, "transfer.busy"})
	status, answer = tm.endRollback(t, "alice", nb)
	wantCode(t, "an end while it runs", status, answer, cell{http.StatusConflict, "transfer.busy"})
	if a := tm.rollbackOf(t, nb); a.State != "rolling_back" || a.left() != "null" || *a.RollbackJobID != job {
		t.Errorf("the upgrade while it runs = %+v, want rolling back, nothing told left", a)
	}
	tm.frozen(t, nb, "while it runs")
	r.stops.goOn()
	if j := tm.endedBackground(t, "alice", job); j.State != "succeeded" {
		t.Fatalf("the rollback = %s, want it succeeded", j)
	}
	tm.rolledBack(t, nb, up, job, true)
	if now := tm.notebookState(t, nb); now != before {
		t.Errorf("the notebook after the rollback:\n%s\nwant as before the start:\n%s", now, before)
	}
}

// stopAndCancel abandons nb's upgrade, its rollback stopped at the
// plan-th plan or the batch-th batch, cancels it there, and lets it go:
// the job ends cancelled. It answers the job's id.
func (r rollbackCase) stopAndCancel(t *testing.T, nb string, plan, batch int) string {
	t.Helper()
	r.stops.arm(nb, plan, batch, false)
	job := r.tm.abandoned(t, "alice", nb)
	r.stops.stopped(t)
	r.tm.cancelSeen(t, "alice", job)
	r.stops.goOn()
	if j := r.tm.endedBackground(t, "alice", job); j.State != "cancelled" {
		t.Fatalf("the rollback = %s, want it cancelled", j)
	}
	return job
}

// stoppedAndContinued cancels the rollback at its first batch: the
// notebook stays rolling back, frozen, its read telling what is left; an
// admin continues it with a new job, which is done.
func (r rollbackCase) stoppedAndContinued(t *testing.T) {
	tm, mark := r.tm, r.mark(t)
	nb, up, before := tm.rollbackNotebook(t, "Continued")
	first := r.stopAndCancel(t, nb, 0, 1)
	if a := tm.rollbackOf(t, nb); a.State != "rolling_back" || a.left() == "null" || a.left() == "0 0 0" || *a.RollbackJobID != first {
		t.Errorf("the upgrade stopped = %+v, want rolling back, what is left told", a)
	}
	tm.frozen(t, nb, "stopped")
	r.stops.arm(nb, 0, 0, false)
	second := tm.abandoned(t, "alice", nb)
	if second == first {
		t.Fatalf("the continuation's job is the stopped one's, %s", first)
	}
	if j := tm.endedBackground(t, "alice", second); j.State != "succeeded" {
		t.Fatalf("the continued rollback = %s, want it succeeded", j)
	}
	tm.rolledBack(t, nb, up, second, true)
	if now := tm.notebookState(t, nb); now != before {
		t.Errorf("the notebook after the rollback:\n%s\nwant as before the start:\n%s", now, before)
	}
	if lines := upgradeLines(t, r.logs, mark, "upgrade rollback continued"); len(lines) != 1 || lines[0]["job_id"] != second {
		t.Errorf("logged %v, want one continuation of the job %s", lines, second)
	}
}

// stoppedAndEndedShort cancels the rollback after it deleted first: its
// end keeps the notebook as it is, rolled back, not complete, with what
// was left: Guide and Kept 2 not in place, Notes and Guide not written
// back, Made
// still there; the freeze is lifted, and a line logged.
func (r rollbackCase) stoppedAndEndedShort(t *testing.T) {
	tm, mark := r.tm, r.mark(t)
	nb, up, _ := tm.rollbackNotebook(t, "Short")
	job := r.stopAndCancel(t, nb, 2, 0)
	status, answer := tm.endRollback(t, "alice", nb)
	var a rollbackAnswer
	decodeAnswer(t, answer, &a)
	if status != http.StatusOK || a.State != "rolled_back" || a.RollbackComplete == nil || *a.RollbackComplete || a.left() != "2 2 1" {
		t.Errorf("the end = %d %s, want rolled back, not complete, 2 nodes, 2 pages and 1 node new left", status, answer)
	}
	tm.rolledBack(t, nb, up, job, false)
	if got := queryStrings(t, tm.pool, `SELECT concat_ws(' ', rollback_left_nodes, rollback_left_pages, rollback_left_created)
		FROM notebook_upgrades WHERE id = $1`, up); !slices.Equal(got, []string{"2 2 1"}) {
		t.Errorf("the row's left %q, want 2 2 1", got)
	}
	if got := queryStrings(t, tm.pool, `SELECT name FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL AND name IN ('Made', 'Kept 2')
		ORDER BY name`, nb); !slices.Equal(got, []string{"Kept 2", "Made"}) {
		t.Errorf("alive %q, want Kept 2 and Made as the end found them", got)
	}
	lines := upgradeLines(t, r.logs, mark, "upgrade rollback ended")
	if len(lines) != 1 || lines[0]["nodes"] != float64(2) || lines[0]["pages"] != float64(2) || lines[0]["created"] != float64(1) ||
		lines[0]["complete"] != false || lines[0]["upgrade_id"] != up {
		t.Errorf("logged %v, want one end of 2, 2 and 1, not complete", lines)
	}
	tm.createPage(t, "bob", nb, "", "After")
}

// endedEarly cancels the rollback at its first plan, before it writes:
// a notebook whose root held raw and AGENTS, which the start wrote none
// of, ends complete with nothing done; one whose agent renamed a page
// ends not complete, the one node left.
func (r rollbackCase) endedEarly(t *testing.T) {
	tm := r.tm
	for _, tt := range []struct {
		name     string
		rename   bool
		complete bool
		left     string
	}{{"Nothing", false, true, "null"}, {"Renamed", true, false, "1 0 0"}} {
		nb := tm.openNotebook(t, "alice", tt.name)
		for _, title := range []string{"raw", "AGENTS", "Guide"} {
			tm.createPage(t, "alice", nb, "", title)
		}
		up := tm.upgraded(t, "alice", nb).ID
		if tt.rename {
			if res := tm.agent(t, "alice", nb).use(t, "move", map[string]any{"from": "Guide.md", "to": "Guide 2.md"}).result(t); res["isError"] == true {
				t.Fatalf("the rename = %v", res)
			}
		}
		job := r.stopAndCancel(t, nb, 1, 0)
		status, answer := tm.endRollback(t, "alice", nb)
		var a rollbackAnswer
		decodeAnswer(t, answer, &a)
		if status != http.StatusOK || a.State != "rolled_back" || a.RollbackComplete == nil || *a.RollbackComplete != tt.complete ||
			a.left() != tt.left {
			t.Errorf("%s: the end = %d %s, want rolled back, complete %v, left %s", tt.name, status, answer, tt.complete, tt.left)
		}
		tm.rolledBack(t, nb, up, job, tt.complete)
	}
}

// aPassThatLeavesAsMuch has the rollback's batches write nothing: its
// second pass finds as much left as the first, and the job ends failed,
// internal, its "rollback ended" line an error's; the notebook still
// rolling back, a continuation is done.
func (r rollbackCase) aPassThatLeavesAsMuch(t *testing.T) {
	tm, mark := r.tm, r.mark(t)
	nb, up, before := tm.rollbackNotebook(t, "Idle")
	r.stops.arm(nb, 0, 0, true)
	job := tm.abandoned(t, "alice", nb)
	if j := tm.endedBackground(t, "alice", job); j.State != "failed" || j.failure() != "internal" {
		t.Fatalf("the rollback that writes nothing = %s, want it failed, internal", j)
	}
	var ended []map[string]any
	for _, l := range upgradeLines(t, r.logs, mark, "rollback ended") {
		if l["job_id"] == job {
			ended = append(ended, l)
		}
	}
	if len(ended) != 1 || ended[0]["level"] != "ERROR" {
		t.Errorf("logged %v, want one error line rollback ended", ended)
	}
	if a := tm.rollbackOf(t, nb); a.State != "rolling_back" {
		t.Errorf("the upgrade = %+v, want it rolling back", a)
	}
	r.stops.arm(nb, 0, 0, false)
	second := tm.abandoned(t, "alice", nb)
	if j := tm.endedBackground(t, "alice", second); j.State != "succeeded" {
		t.Fatalf("the continued rollback = %s, want it succeeded", j)
	}
	tm.rolledBack(t, nb, up, second, true)
	if now := tm.notebookState(t, nb); now != before {
		t.Errorf("the notebook after the rollback:\n%s\nwant as before the start:\n%s", now, before)
	}
}

// itsStarterDemoted: bob, an admin, abandons; made an editor while his
// rollback runs, his cancel of it is forbidden, the job running on; it
// then fails forbidden, the notebook rolling back; alice, an admin,
// continues it, and it is done.
func (r rollbackCase) itsStarterDemoted(t *testing.T) {
	tm := r.tm
	nb := tm.openNotebook(t, "alice", "Demoted")
	tm.createPage(t, "alice", nb, "", "Guide")
	bob := tm.addNotebookMember(t, "alice", nb, "bob", "admin")
	up := tm.upgraded(t, "bob", nb).ID
	r.stops.arm(nb, 1, 0, false)
	job := tm.abandoned(t, "bob", nb)
	r.stops.stopped(t)
	tm.send(t, request("alice", http.MethodPatch, "/api/v0/notebook-members/"+bob, `{"role":"editor"}`), http.StatusOK)
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/transfer-jobs/"+job+"/cancel", tm.tokens["bob"], "")
	wantCode(t, "the demoted starter's cancel", status, answer, cellForbidden())
	if got := queryStrings(t, tm.pool, "SELECT state || ' ' || (cancel_requested_at IS NULL)::text FROM transfer_jobs WHERE id = $1",
		job); !slices.Equal(got, []string{"running true"}) {
		t.Errorf("the job after the refused cancel = %q, want running, no cancel asked", got)
	}
	r.stops.goOn()
	if j := tm.endedBackground(t, "alice", job); j.State != "failed" || j.failure() != "forbidden" {
		t.Fatalf("the demoted starter's rollback = %s, want it failed, forbidden", j)
	}
	r.stops.arm(nb, 0, 0, false)
	second := tm.abandoned(t, "alice", nb)
	if j := tm.endedBackground(t, "alice", second); j.State != "succeeded" {
		t.Fatalf("alice's continuation = %s, want it succeeded", j)
	}
	tm.rolledBack(t, nb, up, second, true)
}

// endedBetweenPasses ends the upgrade by SQL as the rollback is about to
// plan its second pass (decision 233): the job logs "stopped" and writes
// no end, its row running; no unit runs, Made and Guide under it as the
// first pass left them. The test then fails the row itself.
func (r rollbackCase) endedBetweenPasses(t *testing.T) {
	tm, mark := r.tm, r.mark(t)
	nb, _, _ := tm.rollbackNotebook(t, "Gone")
	r.stops.arm(nb, 2, 0, false)
	job := tm.abandoned(t, "alice", nb)
	r.stops.stopped(t)
	tm.endUpgrade(t, nb)
	r.stops.goOn()
	for deadline := time.Now().Add(jobWait); ; time.Sleep(20 * time.Millisecond) {
		lines := upgradeLines(t, r.logs, mark, "rollback stopped: the job was deleted, or no longer runs")
		if len(lines) == 1 && lines[0]["job_id"] == job {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("logged %v, want the job %s stopped", lines, job)
		}
	}
	awaitBackground(t, tm)
	if got := queryStrings(t, tm.pool, "SELECT state FROM transfer_jobs WHERE id = $1", job); !slices.Equal(got, []string{"running"}) {
		t.Errorf("the job = %q, want its row running, no end written", got)
	}
	if _, batches := r.stops.counts(); batches != 1 {
		t.Errorf("%d batches, want the first pass's alone", batches)
	}
	if got := queryStrings(t, tm.pool, `SELECT g.name || ' ' || m.name FROM nodes g JOIN nodes m ON m.id = g.parent_id
		WHERE g.notebook_id = $1 AND g.name = 'Guide' AND g.deleted_at IS NULL AND m.deleted_at IS NULL`, nb); !slices.Equal(got,
		[]string{"Guide Made"}) {
		t.Errorf("Guide's place %q, want under Made, as the first pass left it", got)
	}
	tm.exec(t, `UPDATE transfer_jobs SET state = 'failed', finished_at = now(), report = '{"failure": "interrupted"}' WHERE id = $1`, job)
}

// queuedAndDropped holds the only worker of the imports' queue, which
// the rollbacks share, with a restore in another notebook: two rollbacks
// wait queued; River drops them (the test deletes their River jobs), and
// the rescue fails them, interrupted, its River job enqueued now rather
// than at its interval. One is continued and done; the other's end finds
// everything left.
func (r rollbackCase) queuedAndDropped(t *testing.T) {
	tm := r.tm
	other := tm.openNotebook(t, "alice", "Other")
	box := tm.createPage(t, "alice", other, "", "Box")
	tm.createPage(t, "alice", other, box, "In")
	tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
	type dropped struct{ nb, up, job, before string }
	var held []dropped
	restore := tm.stopAtFirst(t, other, trashRestoreStart("alice", other, box), func(backgroundJob) {
		for _, name := range []string{"Dropped and continued", "Dropped and ended"} {
			nb, up, before := tm.rollbackNotebook(t, name)
			held = append(held, dropped{nb: nb, up: up, job: tm.abandoned(t, "alice", nb), before: before})
		}
		for _, d := range held {
			if got := queryStrings(t, tm.pool, "SELECT state FROM transfer_jobs WHERE id = $1", d.job); !slices.Equal(got,
				[]string{"queued"}) {
				t.Fatalf("the rollback %s = %q, want it queued behind the restore", d.job, got)
			}
			tm.exec(t, `DELETE FROM river_job WHERE kind = 'transfer.rollback' AND args->>'job_id' = $1`, d.job)
		}
		tm.rescued(t)
		for _, d := range held {
			if got := queryStrings(t, tm.pool, "SELECT state || ' ' || (report->>'failure') FROM transfer_jobs WHERE id = $1",
				d.job); !slices.Equal(got, []string{"failed interrupted"}) {
				t.Fatalf("the dropped rollback %s = %q, want the rescue's failed, interrupted", d.job, got)
			}
		}
	})
	if j := tm.endedBackground(t, "alice", restore.ID); j.State != "succeeded" {
		t.Fatalf("the restore = %s", j)
	}
	continued, ended := held[0], held[1]
	job := tm.abandoned(t, "alice", continued.nb)
	if j := tm.endedBackground(t, "alice", job); j.State != "succeeded" {
		t.Fatalf("the continuation of a dropped rollback = %s, want it succeeded", j)
	}
	tm.rolledBack(t, continued.nb, continued.up, job, true)
	if now := tm.notebookState(t, continued.nb); now != continued.before {
		t.Errorf("the notebook after the rollback:\n%s\nwant as before the start:\n%s", now, continued.before)
	}
	status, answer := tm.endRollback(t, "alice", ended.nb)
	var a rollbackAnswer
	decodeAnswer(t, answer, &a)
	if status != http.StatusOK || a.State != "rolled_back" || a.left() != "2 2 9" {
		t.Errorf("the end of a dropped rollback = %d %s, want rolled back, everything left", status, answer)
	}
	tm.rolledBack(t, ended.nb, ended.up, ended.job, false)
}

// rescued runs the transfer module's rescue now, as River would every
// RescueInterval: its River job, enqueued by SQL, completed.
func (tm acmeTeam) rescued(t *testing.T) {
	t.Helper()
	const done = "SELECT count(*) FROM river_job WHERE kind = 'transfer.rescue_interrupted' AND state = 'completed'"
	before := count(t, tm.pool, done)
	tm.exec(t, `INSERT INTO river_job (kind, args, queue, max_attempts) VALUES ('transfer.rescue_interrupted', '{}', 'default', 1)`)
	for deadline := time.Now().Add(interleavingWait); count(t, tm.pool, done) == before; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the rescue did not complete: %q", queryStrings(t, tm.pool,
				"SELECT state || ' ' || coalesce(array_to_string(errors, ' '), '') FROM river_job WHERE kind = 'transfer.rescue_interrupted'"))
		}
	}
}

// rollbackContinuedAfterATimeout stops a rollback past transfer's
// job_timeout: the job fails, timeout, the notebook rolling back; its
// continuation, a new job, is done.
func rollbackContinuedAfterATimeout(t *testing.T) {
	const timeout = 3 * time.Second
	tm, stops, _ := rollbackTeam(t, func(c *config.Config) { c.Transfer.JobTimeout = timeout })
	nb, up, before := tm.rollbackNotebook(t, "Timed out")
	stops.arm(nb, 0, 1, false)
	job := tm.abandoned(t, "alice", nb)
	stops.stopped(t)
	var started time.Time
	if err := tm.pool.QueryRow(t.Context(), "SELECT started_at FROM transfer_jobs WHERE id = $1", job).Scan(&started); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Until(started.Add(timeout + 200*time.Millisecond)))
	stops.goOn()
	if j := tm.endedBackground(t, "alice", job); j.State != "failed" || j.failure() != "timeout" {
		t.Fatalf("the rollback past its timeout = %s, want it failed, timeout", j)
	}
	stops.arm(nb, 0, 0, false)
	second := tm.abandoned(t, "alice", nb)
	if j := tm.endedBackground(t, "alice", second); j.State != "succeeded" {
		t.Fatalf("the continuation = %s, want it succeeded", j)
	}
	tm.rolledBack(t, nb, up, second, true)
	if now := tm.notebookState(t, nb); now != before {
		t.Errorf("the notebook after the rollback:\n%s\nwant as before the start:\n%s", now, before)
	}
	checkPages(t, tm.pool)
	checkBackground(t, tm)
}
