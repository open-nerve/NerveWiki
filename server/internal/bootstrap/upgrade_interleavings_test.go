package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

// The start's interleavings (M11/P1 design 7.2: #1–4, 13, 14, 17) and the
// rollback's (#10–12, 16), each a determined order: the test holds the
// notebook's row both sides want, or stops one side by a hook of the
// wiring that does nothing unarmed. The rollbacks write batches of one
// node.

// stoppedClock is the page module's clock, which, armed, reads the time
// once and stops before answering it until let go.
type stoppedClock struct {
	page.Clock
	armed atomic.Bool
	read  chan time.Time
	free  chan struct{}
}

func (c *stoppedClock) Now() time.Time {
	now := c.Clock.Now()
	if c.armed.CompareAndSwap(true, false) {
		c.read <- now
		<-c.free
	}
	return now
}

// heldRules are lint's rules, which, armed, hold the lint slot of the
// next preparation until let go; failing, fail the next one (M11/P2
// design 7.1: a submission whose check answers an error).
type heldRules struct {
	wiki.Rules
	armed   atomic.Bool
	failing atomic.Bool
	held    chan struct{}
	free    chan struct{}
}

// errRulesFailed is the error the rules answer, failing.
var errRulesFailed = errors.New("the rules failed, as the test asked")

func (r *heldRules) PrepareRules(ctx context.Context, in wiki.ReadInput, q wiki.RuleQuery) (wiki.PreparedRules, error) {
	if r.armed.CompareAndSwap(true, false) {
		r.held <- struct{}{}
		<-r.free
	}
	if r.failing.CompareAndSwap(true, false) {
		return nil, errRulesFailed
	}
	return r.Rules.PrepareRules(ctx, in, q)
}

// upgradeStart is by's start of the notebook nb's upgrade.
func upgradeStart(by, nb string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+nb+"/upgrade", `{}`)
}

func wantAnswer(t *testing.T, what string, a answer, want cell) {
	t.Helper()
	if got := (cell{a.status, a.code}); got != want {
		t.Errorf("%s = %d %s %v, want %s", what, a.status, a.body, a.err, want)
	}
}

func TestUpgradeInterleavings(t *testing.T) {
	clock := &stoppedClock{read: make(chan time.Time, 1), free: make(chan struct{})}
	rules := &heldRules{held: make(chan struct{}, 1), free: make(chan struct{})}
	stops, end, logs := &rollbackStops{}, &stoppedEnd{reached: make(chan struct{}, 1), free: make(chan struct{})}, &syncBuffer{}
	gate := &writingGate{}
	tm := newAcmeTeamOn(t, "member", "", nil, &wiring{
		page: func(d *page.Deps) { clock.Clock, d.Clock = d.Clock, clock },
		wiki: func(d *wiki.Deps) {
			rules.Rules, d.Rules = d.Rules, rules
			gate.WritingJobs, d.Writing = d.Writing, gate
		},
		transfer: func(d *transfer.Deps) {
			stops.Tree, d.Tree = d.Tree, stops
			end.RollbackEnd, d.RollbackEnd = d.RollbackEnd, end
		},
		batch: transfer.Batch{Nodes: 1},
	}, slog.New(slog.NewJSONHandler(logs, nil)))
	rollbacks := rollbackInterleavings{tm: tm, stops: stops, end: end, gate: gate, logs: logs}
	created := cell{status: http.StatusCreated}
	upgrading := cell{http.StatusConflict, "notebook.upgrading"}

	// #1: a unit first, the start waits for the notebook's row and its
	// baseline holds the page the unit wrote; the start first, another's
	// write waits for the row and is notebook.upgrading.
	t.Run("1 the start and a running unit", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "One")
		tm.createPage(t, "alice", nb, "", "Page")
		unit, start := tm.interleaveOn(t, notebookRow(nb), pageCreation("bob", nb, "Written"), upgradeStart("alice", nb))
		wantAnswer(t, "the unit first", unit, created)
		wantAnswer(t, "the start after it", start, cellOK())
		if n := count(t, tm.pool, `SELECT count(*) FROM upgrade_nodes u JOIN nodes n ON n.id = u.node_id
			WHERE n.notebook_id = $1 AND n.name = 'Written' AND u.baseline`, nb); n != 1 {
			t.Errorf("the baseline holds %d rows of the page written before, want one", n)
		}
		other := tm.openNotebook(t, "alice", "One again")
		tm.createPage(t, "alice", other, "", "Page")
		start, unit = tm.interleaveOn(t, notebookRow(other), upgradeStart("alice", other), pageCreation("bob", other, "Written"))
		wantAnswer(t, "the start first", start, cellOK())
		wantAnswer(t, "the unit after it", unit, upgrading)
		checkPages(t, tm.pool)
	})

	// #2: an opening of an edit session first, the start after it is
	// wiki.upgrade_pages_locked; the start first, the opening is vetoed.
	t.Run("2 the start and an opening", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Two")
		p := tm.createPage(t, "alice", nb, "", "Page")
		open, start := tm.interleaveOn(t, notebookRow(nb), sessionOpening("bob", p), upgradeStart("alice", nb))
		wantAnswer(t, "the opening first", open, created)
		wantAnswer(t, "the start after it", start, cell{http.StatusConflict, "wiki.upgrade_pages_locked"})
		other := tm.openNotebook(t, "alice", "Two again")
		q := tm.createPage(t, "alice", other, "", "Page")
		start, open = tm.interleaveOn(t, notebookRow(other), upgradeStart("alice", other), sessionOpening("bob", q))
		wantAnswer(t, "the start first", start, cellOK())
		wantAnswer(t, "the opening after it", open, upgrading)
		checkPages(t, tm.pool)
	})

	// #3: a heartbeat read the clock while its session was alive and
	// stopped; the session expired before the start, which deletes it;
	// let go, the heartbeat finds no session to keep alive.
	t.Run("3 the start and a heartbeat past its session", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Three")
		p := tm.createPage(t, "alice", nb, "", "Page")
		session := tm.openSession(t, "bob", p)
		clock.armed.Store(true)
		heartbeat := tm.sender(t, request("bob", http.MethodPost, "/api/v0/edit-sessions/"+session+"/heartbeat", ""))
		beat := make(chan answer, 1)
		go func() { beat <- heartbeat() }()
		var read time.Time
		select {
		case read = <-clock.read:
		case <-time.After(interleavingWait):
			t.Fatal("the heartbeat did not read the clock")
		}
		tm.exec(t, "UPDATE edit_sessions SET expires_at = $2 WHERE id = $1", session, read.Add(time.Millisecond))
		time.Sleep(10 * time.Millisecond)
		status, started := tm.startUpgrade(t, "alice", nb, `{}`)
		wantCode(t, "the start once the session expired", status, started, cellOK())
		clock.free <- struct{}{}
		var a answer
		select {
		case a = <-beat:
		case <-time.After(interleavingWait):
			t.Fatal("the heartbeat did not answer")
		}
		if a.res != nil {
			tm.contract.CheckResponse(t, a.req, a.res)
		}
		wantAnswer(t, "the heartbeat let go", a, cell{http.StatusNotFound, "page.edit_session_not_found"})
		if n := count(t, tm.pool, "SELECT count(*) FROM edit_sessions WHERE notebook_id = $1", nb); n != 0 {
			t.Errorf("%d edit sessions of the notebook being upgraded, want none", n)
		}
		checkPages(t, tm.pool)
	})

	// #4: an import's start was checked and its file is uploading; the
	// upgrade starts; the upload done, the job is refused and its file
	// deleted.
	t.Run("4 the start and an uploading import", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Four")
		tm.createPage(t, "alice", nb, "", "Page")
		zip := vaultZip(t, vaultFile{"a.md", "a"})
		pr, pw := io.Pipe()
		mw := multipart.NewWriter(pw)
		parts := make(chan io.Writer, 1)
		go func() {
			w, err := mw.CreateFormFile("file", "vault.zip")
			if err == nil {
				_, err = w.Write(zip[:len(zip)/2])
			}
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			parts <- w
		}()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/imports", pr)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tm.tokens["alice"])
		req.Header.Set("Content-Type", mw.FormDataContentType())
		done := make(chan answer, 1)
		go func() {
			res, err := client().Do(req)
			a := answer{req: req, res: res, err: err}
			if err == nil {
				body, _ := io.ReadAll(res.Body)
				_ = res.Body.Close()
				a.status, a.body, a.code = res.StatusCode, string(body), problemCodeOf(string(body))
				a.res.Body = io.NopCloser(bytes.NewReader(body))
			}
			done <- a
		}()
		t.Cleanup(func() { _ = pw.CloseWithError(errors.New("the test is over")) })
		tmp := filepath.Join(tm.storage, "imports", ".tmp")
		storing := func() bool { files, _ := os.ReadDir(tmp); return len(files) > 0 }
		for deadline := time.Now().Add(interleavingWait); !storing(); {
			if time.Now().After(deadline) {
				t.Fatal("the import's file was never stored")
			}
			time.Sleep(10 * time.Millisecond)
		}
		status, started := tm.startUpgrade(t, "alice", nb, `{}`)
		wantCode(t, "the start while the file uploads", status, started, cellOK())
		w := <-parts
		if _, err := w.Write(zip[len(zip)/2:]); err != nil {
			t.Fatal(err)
		}
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}
		_ = pw.Close()
		var a answer
		select {
		case a = <-done:
		case <-time.After(interleavingWait):
			t.Fatal("the import did not answer")
		}
		if a.res != nil {
			tm.contract.CheckResponse(t, a.req, a.res)
		}
		wantAnswer(t, "the import's start once uploaded", a, upgrading)
		if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", nb); n != 0 {
			t.Errorf("%d jobs of the notebook being upgraded, want none", n)
		}
		for deadline := time.Now().Add(interleavingWait); storing(); {
			if time.Now().After(deadline) {
				t.Fatal("the refused import's file stayed")
			}
			time.Sleep(10 * time.Millisecond)
		}
		checkPages(t, tm.pool)
	})

	// #13: the initiator made an editor first; his agent's write after it
	// is notebook.upgrading, writing nothing.
	t.Run("13 the initiator demoted and his write", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Thirteen")
		tm.createPage(t, "alice", nb, "", "Page")
		bob := tm.addNotebookMember(t, "alice", nb, "bob", "admin")
		tm.upgraded(t, "bob", nb)
		agent := tm.agent(t, "bob", nb)
		var wrote mcpAnswer
		write := step{by: "bob", command: func() error {
			wrote = agent.use(t, "write", map[string]any{"path": "New.md", "content": "x"})
			return nil
		}}
		demote := request("alice", http.MethodPatch, "/api/v0/notebook-members/"+bob, `{"role":"editor"}`)
		first, _ := tm.interleaveOn(t, notebookRow(nb), demote, write)
		wantAnswer(t, "the demotion first", first, cellOK())
		if member := wrote.toolError(t); member["code"] != "notebook.upgrading" {
			t.Errorf("the demoted initiator's agent's write = %v, want notebook.upgrading", member)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name = 'New'", nb); n != 0 {
			t.Errorf("the refused write wrote %d pages", n)
		}
		checkPages(t, tm.pool)
	})

	// #14: two starts at once: the second waits for the notebook's row
	// and is notebook.upgrading.
	t.Run("14 two starts", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Fourteen")
		tm.createPage(t, "alice", nb, "", "Page")
		first, second := tm.interleaveOn(t, notebookRow(nb), upgradeStart("alice", nb), upgradeStart("alice", nb))
		wantAnswer(t, "the first start", first, cellOK())
		wantAnswer(t, "the second", second, upgrading)
		if n := count(t, tm.pool, "SELECT count(*) FROM notebook_upgrades WHERE notebook_id = $1", nb); n != 1 {
			t.Errorf("%d upgrades of the notebook, want one", n)
		}
		checkPages(t, tm.pool)
	})

	// #17: lint's only slot held by a read, the start is server_busy at
	// once, Retry-After: 1, far within the slot's wait. The start is sent
	// aside: one that waited for the slot fails here in 2 s and gets it
	// once the read gives it back, rather than hang the shard.
	t.Run("17 the start and the lint slots taken", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Seventeen")
		tm.createPage(t, "alice", nb, "", "Page")
		rules.armed.Store(true)
		lint := tm.sender(t, request("alice", http.MethodGet, "/api/v0/notebooks/"+nb+"/lint", ""))
		linted := make(chan answer, 1)
		go func() { linted <- lint() }()
		select {
		case <-rules.held:
		case <-time.After(interleavingWait):
			t.Fatal("the lint read took no slot")
		}
		req := newRequest(t, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade", tm.tokens["alice"], []byte(`{}`))
		type sent struct {
			res     *http.Response
			body    []byte
			err     error
			elapsed time.Duration
		}
		started := make(chan sent, 1)
		go func() {
			at := time.Now()
			res, err := client().Do(req)
			var body []byte
			if err == nil {
				body, err = io.ReadAll(res.Body)
				_ = res.Body.Close()
				res.Body = io.NopCloser(bytes.NewReader(body))
			}
			started <- sent{res, body, err, time.Since(at)}
		}()
		select {
		case s := <-started:
			rules.free <- struct{}{}
			if s.err != nil {
				t.Fatal(s.err)
			}
			tm.contract.CheckResponse(t, req, s.res)
			if s.res.StatusCode != http.StatusServiceUnavailable || problemCodeOf(string(s.body)) != "server_busy" ||
				s.res.Header.Get("Retry-After") != "1" || s.elapsed > time.Second {
				t.Errorf("the start with the slot taken = %d %s %v after %v, want 503 server_busy at once", s.res.StatusCode, s.body, s.res.Header, s.elapsed)
			}
		case <-time.After(2 * time.Second):
			t.Error("the start with the slot taken did not answer in 2 s: it waited for the slot")
			rules.free <- struct{}{}
			select {
			case <-started:
			case <-time.After(interleavingWait):
				t.Error("the start did not answer once the slot was given back")
			}
		}
		select {
		case a := <-linted:
			wantAnswer(t, "the lint read let go", a, cellOK())
		case <-time.After(interleavingWait):
			t.Fatal("the lint read did not answer")
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM notebook_upgrades WHERE notebook_id = $1", nb); n != 0 {
			t.Errorf("the refused start left %d upgrades", n)
		}
		checkPages(t, tm.pool)
	})

	t.Run("10 the abandonment and the initiator's batch", rollbacks.abandonAndTheBatch)
	t.Run("11 a rollback's batch and its cancel", rollbacks.theBatchAndTheCancel)
	t.Run("12 a rollback's end and the notebook's deletion", rollbacks.theEndAndTheDeletion)
	t.Run("16 the rescue, the running worker and the end", rollbacks.theRescueAndTheEnd)
	t.Run("the read of an upgrade across its rollback's end", rollbacks.readAcrossTheEnd)
}
