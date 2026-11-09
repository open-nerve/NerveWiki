package bootstrap

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The rollback's interleavings (M11/P1 design 7.2: #10, 11, 12, 16; and
// the read of an upgrade across its rollback's end), in
// TestUpgradeInterleavings, on its app: the rollbacks' batches of one
// node, their plans and batches stopped by rollbackStops, their ends by
// stoppedEnd, the wiki module's reads of the writing jobs by writingGate,
// the log in a buffer.

// writingGate is the wiki module's WritingJobs, which, armed, stops its
// next call before it reads, until released.
type writingGate struct {
	wiki.WritingJobs
	mu               sync.Mutex
	armed            bool
	reached, release chan struct{}
}

// arm stops the next call.
func (g *writingGate) arm() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.armed, g.reached, g.release = true, make(chan struct{}), make(chan struct{})
}

func (g *writingGate) Writing(ctx context.Context, notebookID uuid.UUID) (bool, error) {
	g.mu.Lock()
	stop, reached, release := g.armed, g.reached, g.release
	g.armed = false
	g.mu.Unlock()
	if stop {
		close(reached)
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
	return g.WritingJobs.Writing(ctx, notebookID)
}

// stoppedEnd is the wiki module's end of the rollbacks, which, armed,
// stops in End, the notebook's row locked and the job's end written,
// until let go; it counts the calls of End.
type stoppedEnd struct {
	transfer.RollbackEnd
	armed   atomic.Bool
	reached chan struct{}
	free    chan struct{}
	ends    atomic.Int32
}

func (e *stoppedEnd) End(ctx context.Context, notebookID, upgradeID uuid.UUID) (bool, func(), error) {
	e.ends.Add(1)
	if e.armed.CompareAndSwap(true, false) {
		e.reached <- struct{}{}
		<-e.free
	}
	return e.RollbackEnd.End(ctx, notebookID, upgradeID)
}

// rollbackInterleavings are what the rollback's interleavings share.
type rollbackInterleavings struct {
	tm    acmeTeam
	stops *rollbackStops
	end   *stoppedEnd
	gate  *writingGate
	logs  *syncBuffer
}

// mark is how many lines the log holds.
func (r rollbackInterleavings) mark(t *testing.T) int {
	t.Helper()
	return len((&mcpApp{logs: r.logs}).logLines(t))
}

// awaitLine waits until the log holds msg of the job after mark.
func (r rollbackInterleavings) awaitLine(t *testing.T, mark int, msg, job string) {
	t.Helper()
	for deadline := time.Now().Add(interleavingWait); ; time.Sleep(20 * time.Millisecond) {
		for _, l := range upgradeLines(t, r.logs, mark, msg) {
			if l["job_id"] == job {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %q of the job %s logged", msg, job)
		}
	}
}

// abandonAndTheBatch is #10: the initiator's agent's batch holds the
// notebook's row first, the abandonment waits for it; the rollback's plan
// holds the batch's writes, and once done the notebook is as before the
// start.
func (r rollbackInterleavings) abandonAndTheBatch(t *testing.T) {
	tm := r.tm
	nb, _, before := tm.rollbackNotebook(t, "Ten")
	agent := tm.agent(t, "alice", nb)
	var wrote mcpAnswer
	batch := step{by: "alice", command: func() error {
		wrote = agent.use(t, "batch", batched(opWrite("Late.md", "late"), opMove("Notes.md", "Folder/Notes.md")))
		return nil
	}}
	r.stops.arm(nb, 0, 0, false)
	first, second := tm.interleaveOn(t, notebookRow(nb), batch, request("alice", http.MethodPost, "/api/v0/notebooks/"+nb+"/upgrade/abandon", ""))
	if first.err != nil || wrote.result(t)["isError"] == true {
		t.Fatalf("the batch first = %v %s", first.err, wrote.body)
	}
	wantAnswer(t, "the abandonment after it", second, cellOK())
	var a rollbackAnswer
	decodeAnswer(t, second.body, &a)
	if a.RollbackJobID == nil {
		t.Fatalf("the abandonment = %s, want its job", second.body)
	}
	if j := tm.endedBackground(t, "alice", *a.RollbackJobID); j.State != "succeeded" {
		t.Fatalf("the rollback = %s, want it succeeded", j)
	}
	if now := tm.notebookState(t, nb); now != before {
		t.Errorf("the notebook after the rollback:\n%s\nwant as before the start:\n%s", now, before)
	}
	checkPages(t, tm.pool)
}

// theBatchAndTheCancel is #11: the rollback stopped at its first batch,
// the unit of the new nodes deleted first, one step past the batch's one
// node; its cancel asked and read by its heartbeat as ever; let go, the
// first batch is written whole and the job stops before the next,
// cancelled, the notebook rolling back. Then it is continued and done, or, in
// another notebook, ended. The rollbacks' stops stand in for stopAtFirst,
// whose share of the notebook's row the abandonment would wait for.
func (r rollbackInterleavings) theBatchAndTheCancel(t *testing.T) {
	tm := r.tm
	for _, then := range []string{"continued", "ended"} {
		nb, up, before := tm.rollbackNotebook(t, "Eleven "+then)
		r.stops.arm(nb, 0, 1, false)
		job := tm.abandoned(t, "alice", nb)
		r.stops.stopped(t)
		tm.cancelSeen(t, "alice", job)
		r.stops.goOn()
		j := tm.endedBackground(t, "alice", job)
		if j.State != "cancelled" || j.counts()["deleted"] != 8 || j.Progress.Done != 8 {
			t.Fatalf("the rollback = %s, want it cancelled after its first batch, the unit of the eight new nodes deleted first", j)
		}
		if _, batches := r.stops.counts(); batches != 1 {
			t.Errorf("%d batches, want the first alone", batches)
		}
		if a := tm.rollbackOf(t, nb); a.State != "rolling_back" {
			t.Errorf("the upgrade = %+v, want it rolling back", a)
		}
		if then == "continued" {
			r.stops.arm(nb, 0, 0, false)
			second := tm.abandoned(t, "alice", nb)
			if j := tm.endedBackground(t, "alice", second); j.State != "succeeded" {
				t.Fatalf("the continuation = %s, want it succeeded", j)
			}
			tm.rolledBack(t, nb, up, second, true)
			if now := tm.notebookState(t, nb); now != before {
				t.Errorf("the notebook after the rollback:\n%s\nwant as before the start:\n%s", now, before)
			}
			continue
		}
		status, answer := tm.endRollback(t, "alice", nb)
		var a rollbackAnswer
		decodeAnswer(t, answer, &a)
		if status != http.StatusOK || a.State != "rolled_back" || a.left() != "2 2 1" {
			t.Errorf("the end = %d %s, want rolled back, what the first batch did not delete left", status, answer)
		}
		tm.rolledBack(t, nb, up, job, false)
	}
	checkPages(t, tm.pool)
}

// theEndAndTheDeletion is #12: the rollback's end's transaction stopped
// in End, the notebook's row locked, the notebook's deletion waits for
// it, and comes after the upgrade rolled back; the deletion first, the
// end's transaction waits for the row, then finds none to lock: it
// writes nothing, calls no End, logs "stopped". Neither deadlocks.
func (r rollbackInterleavings) theEndAndTheDeletion(t *testing.T) {
	tm := r.tm
	nb, up, _ := tm.rollbackNotebook(t, "Twelve")
	r.stops.arm(nb, 0, 0, false)
	r.end.armed.Store(true)
	job := tm.abandoned(t, "alice", nb)
	select {
	case <-r.end.reached:
	case <-time.After(jobWait):
		t.Fatal("the rollback did not reach its end")
	}
	deletion := tm.inBackground(t, request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""))
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	r.end.free <- struct{}{}
	wantAnswer(t, "the deletion after the end", deletion(), cell{status: http.StatusNoContent})
	if got := queryStrings(t, tm.pool, `SELECT u.state || ' ' || (u.deleted_at IS NOT NULL)::text || ' ' || j.state
		FROM notebook_upgrades u, transfer_jobs j WHERE u.id = $1 AND j.id = $2`, up, job); !slices.Equal(got,
		[]string{"rolled_back true succeeded"}) {
		t.Errorf("the upgrade and its job = %q, want rolled back, then deleted with the notebook; the job succeeded", got)
	}

	other, otherUp, _ := tm.rollbackNotebook(t, "Twelve again")
	r.stops.armDone(other)
	ends, mark := r.end.ends.Load(), r.mark(t)
	job = tm.abandoned(t, "alice", other)
	r.stops.stopped(t)
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	if _, err := holder.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR NO KEY UPDATE", other); err != nil {
		t.Fatal(err)
	}
	deletion = tm.inBackground(t, request("alice", http.MethodDelete, "/api/v0/notebooks/"+other, ""))
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	r.stops.goOn()
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2, interleavingWait)
	commit(t, holder)
	wantAnswer(t, "the deletion first", deletion(), cell{status: http.StatusNoContent})
	r.awaitLine(t, mark, "rollback stopped: the job was deleted, or no longer runs", job)
	if n := r.end.ends.Load(); n != ends {
		t.Errorf("End called %d times, want none", n-ends)
	}
	if got := queryStrings(t, tm.pool, `SELECT u.state || ' ' || (u.deleted_at IS NOT NULL)::text || ' ' || j.state || ' ' ||
			(j.deleted_at IS NOT NULL)::text
		FROM notebook_upgrades u, transfer_jobs j WHERE u.id = $1 AND j.id = $2`, otherUp, job); !slices.Equal(got,
		[]string{"rolling_back true running true"}) {
		t.Errorf("the upgrade and its job = %q, want both deleted with the notebook as they were", got)
	}
	awaitBackground(t, tm)
	checkPages(t, tm.pool)
}

// theRescueAndTheEnd is #16 (decision 234): the rollback stopped in its
// second batch, the test's trigger fails its heartbeat's write, the warn
// logged; the rescue's InterruptJobs fails it, interrupted; an admin ends
// the rollback, the job no longer running; let go, the batch writes
// nothing, the upgrade having ended, the job's row stays failed,
// interrupted, and "stopped" is logged.
func (r rollbackInterleavings) theRescueAndTheEnd(t *testing.T) {
	tm := r.tm
	nb, up, _ := tm.rollbackNotebook(t, "Sixteen")
	r.stops.arm(nb, 0, 2, false)
	job := tm.abandoned(t, "alice", nb)
	r.stops.stopped(t)
	mark := r.mark(t)
	tm.exec(t, `CREATE FUNCTION fail_the_heartbeat() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'the test fails the heartbeat'; END $$`)
	tm.exec(t, `CREATE TRIGGER fail_the_heartbeat BEFORE UPDATE ON transfer_jobs FOR EACH ROW
		WHEN (OLD.id = '`+job+`' AND OLD.state = 'running' AND NEW.state = 'running') EXECUTE FUNCTION fail_the_heartbeat()`)
	r.awaitLine(t, mark, "rollback heartbeat not written", job)
	tm.exec(t, `UPDATE transfer_jobs SET state = 'failed', finished_at = now(),
		report = coalesce(report || jsonb_build_object('failure', 'interrupted'), '{"failure": "interrupted"}')
		WHERE state = 'running' AND id = $1`, job)
	alive := func() []string {
		return queryStrings(t, tm.pool, "SELECT name FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL ORDER BY name", nb)
	}
	status, answer := tm.endRollback(t, "alice", nb)
	if status != http.StatusOK {
		t.Fatalf("the end of the interrupted rollback = %d %s, want 200", status, answer)
	}
	tm.createPage(t, "bob", nb, "", "After")
	before := alive()
	mark = r.mark(t)
	r.stops.goOn()
	r.awaitLine(t, mark, "rollback stopped: the job was deleted, or no longer runs", job)
	if now := alive(); !slices.Equal(now, before) {
		t.Errorf("alive %q after the stopped batch, want %q", now, before)
	}
	if got := queryStrings(t, tm.pool, "SELECT state || ' ' || (report->>'failure') FROM transfer_jobs WHERE id = $1", job); !slices.Equal(
		got, []string{"failed interrupted"}) {
		t.Errorf("the job = %q, want failed, interrupted", got)
	}
	tm.exec(t, "DROP TRIGGER fail_the_heartbeat ON transfer_jobs")
	tm.exec(t, "DROP FUNCTION fail_the_heartbeat()")
	tm.rolledBack(t, nb, up, job, false)
	awaitBackground(t, tm)
	checkPages(t, tm.pool)
}

// readAcrossTheEnd is the read of an upgrade across its rollback's end
// (M11/P1 S3 review A-M2): getUpgrade reads the upgrade's row, then waits
// before it reads whether a job writes; the rollback's job, let go, ends
// the upgrade. Let go, getUpgrade answers as its snapshot had it: rolling
// back, the job running, nothing told left; not the upgrade rolling back
// with its baseline gone, every node alive told as created. Read again,
// there is none.
func (r rollbackInterleavings) readAcrossTheEnd(t *testing.T) {
	tm := r.tm
	nb, _, _ := tm.rollbackNotebook(t, "Read across the end")
	r.stops.arm(nb, 1, 0, false)
	job := tm.abandoned(t, "alice", nb)
	r.stops.stopped(t)
	r.gate.arm()
	type answered struct {
		status int
		answer string
	}
	read := make(chan answered, 1)
	go func() {
		status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb+"/upgrade", tm.tokens["alice"], "")
		read <- answered{status, answer}
	}()
	select {
	case <-r.gate.reached:
	case <-time.After(interleavingWait):
		t.Fatal("getUpgrade did not read whether a job writes")
	}
	r.stops.goOn()
	if j := tm.endedBackground(t, "alice", job); j.State != "succeeded" {
		t.Fatalf("the rollback = %s, want it succeeded", j)
	}
	close(r.gate.release)
	var got answered
	select {
	case got = <-read:
	case <-time.After(interleavingWait):
		t.Fatal("getUpgrade did not answer")
	}
	var a rollbackAnswer
	decodeAnswer(t, got.answer, &a)
	if got.status != http.StatusOK || a.State != "rolling_back" || a.RollbackJobID == nil || *a.RollbackJobID != job || a.left() != "null" {
		t.Errorf("getUpgrade across the end = %d %s; want the upgrade rolling back, its job %s running, nothing told left", got.status,
			got.answer, job)
	}
	status, answer := tm.getUpgrade(t, "alice", nb)
	wantCode(t, "the read after the end", status, answer, cell{http.StatusNotFound, "wiki.upgrade_not_found"})
}
