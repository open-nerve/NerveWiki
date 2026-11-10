package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// An upgrade's submission through the whole program (M11/P2 design 3.12,
// 7.1); the wiki module's half with fakes is wiki/app's
// TestSubmitUpgradeAnswersInOrder and its neighbours.

// submitUpgrade is by's submission of nb's upgrade, as the API answers it.
func (tm acmeTeam) submitUpgrade(t *testing.T, by, nb string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade/submit", tm.tokens[by], "")
}

// submitStep is by's submission of nb's upgrade, as a step.
func submitStep(by, nb string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+nb+"/upgrade/submit", "")
}

// submittedAnswer is an Upgrade answer with its submission and report, as
// much as the tests check.
type submittedAnswer struct {
	State       string          `json:"state"`
	SubmittedAt *time.Time      `json:"submitted_at"`
	SubmittedBy *struct{}       `json:"submitted_by"`
	Report      json.RawMessage `json:"report"`
}

// submitted is by's submission of nb's upgrade, which must answer 200,
// reviewing, with a report that passed.
func (tm acmeTeam) submitted(t *testing.T, by, nb string) submittedAnswer {
	t.Helper()
	status, answer := tm.submitUpgrade(t, by, nb)
	if status != http.StatusOK {
		t.Fatalf("%s's submission of %s = %d %s, want 200", by, nb, status, answer)
	}
	var a submittedAnswer
	decodeAnswer(t, answer, &a)
	var report struct {
		Passed bool              `json:"passed"`
		Rules  []json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(a.Report, &report); err != nil || a.State != "reviewing" || a.SubmittedAt == nil ||
		a.SubmittedBy == nil || !report.Passed || len(report.Rules) != 8 {
		t.Fatalf("submitted %s, want it reviewing with the report of a check passed", answer)
	}
	return a
}

// submission is the notebook nb's upgrade's row as a submission leaves
// it: its state and the notebook's, its submission's id ("" for none),
// whether it has its three columns and a report.
func (tm acmeTeam) submission(t *testing.T, nb string) string {
	t.Helper()
	rows := queryStrings(t, tm.pool, `SELECT u.state || ' ' || coalesce(n.upgrade_state, '-') || ' ' ||
			(u.submission_id IS NOT NULL AND u.submitted_by_id IS NOT NULL AND u.submitted_at IS NOT NULL)::text || ' ' ||
			(u.submission_id IS NULL AND u.submitted_by_id IS NULL AND u.submitted_at IS NULL)::text || ' ' || (u.report IS NOT NULL)::text
		FROM notebook_upgrades u JOIN notebooks n ON n.id = u.notebook_id
		WHERE u.notebook_id = $1 AND u.state IN ('upgrading', 'reviewing', 'rolling_back')`, nb)
	if len(rows) != 1 {
		t.Fatalf("the upgrades of %s under way: %v", nb, rows)
	}
	return rows[0]
}

// submissionID is the id of the submission nb's upgrade holds.
func (tm acmeTeam) submissionID(t *testing.T, nb string) string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT coalesce(submission_id::text, '') FROM notebook_upgrades
		WHERE notebook_id = $1 AND state IN ('upgrading', 'reviewing', 'rolling_back')`, nb)[0]
}

// The rows a submission leaves: upgrading again, no submission; reviewing
// with its report.
const (
	backToUpgrading  = "upgrading upgrading false true false"
	reviewedReported = "reviewing reviewing true false true"
)

// cleanNote is a page's content that passes the frontmatter gate.
const cleanNote = "---\ntype: note\nsummary: A note.\n---\nA note.\n"

// modesOf runs act once the frames of the notebooks so far have passed,
// on a stream of alice's opened then, and answers how many mode frames of
// nb it brought, eng's marker written after it.
func (tm acmeTeam) modesOf(t *testing.T, nb, eng, marker string, act func()) int {
	t.Helper()
	tm.settle(t, eng, marker)
	s := openStream(t, tm.base, "alice", tm.tokens["alice"])
	defer s.close()
	act()
	return tm.modeFrames(t, nb, eng, marker, s)[0]
}

// submitLines are the log's "upgrade submitted" lines of nb after before.
func submitLines(t *testing.T, logs *syncBuffer, before int, nb string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range upgradeLines(t, logs, before, "upgrade submitted") {
		if l["notebook_id"] == nb {
			out = append(out, l)
		}
	}
	return out
}

// The submission's codes in M11/P2 design 3.12's order, each refusal
// leaving the upgrade as it was: no notebook, one carol does not see, an
// editor's; no upgrade, one rolling back, one with a report; one whose
// submission is still checked. A stale index and a gate not passed set it
// upgrading again, its submission cleared, two mode frames, the first
// transaction's and the second's. One that passes stays reviewing with
// its report, which the upgrade's read answers too, two mode frames and
// an info line. On the separate app (7.2): a client gone while its check
// runs still has its report kept, stored; a check that answers an error
// sets the upgrade upgrading again, failed, and it is submitted again.
// Last, the deadline (7.1): REST's request deadline first.
func TestUpgradeSubmit(t *testing.T) {
	r := upgradeRunsOn(t, nil, 1, 2*time.Second, 6*time.Second)
	tm := r.tm
	tm.join(t, "dana", "member")
	eng := tm.openNotebook(t, "alice", "Eng")
	marker := tm.createPage(t, "alice", eng, "", "Marker")
	notebook := func(t *testing.T, name, content string) string {
		t.Helper()
		nb := tm.openNotebook(t, "alice", name)
		tm.createPageWith(t, "alice", nb, "", "Note", content)
		return nb
	}
	t.Run("the codes in their order", func(t *testing.T) {
		nb := notebook(t, "Codes", "A note.\n")
		plain := notebook(t, "Plain", cleanNote)
		tm.upgraded(t, "alice", nb)
		for _, tc := range []struct {
			what, by, nb string
			want         cell
		}{
			{"no notebook", "alice", "0199a2b4-0000-7000-8000-000000000000", cell{http.StatusNotFound, "notebook.not_found"}},
			{"a notebook carol does not see", "carol", nb, cell{http.StatusNotFound, "notebook.not_found"}},
			{"an editor's", "bob", nb, cellForbidden()},
			{"no upgrade", "alice", plain, cell{http.StatusConflict, "wiki.upgrade_state"}},
		} {
			status, answer := tm.submitUpgrade(t, tc.by, tc.nb)
			wantCode(t, tc.what, status, answer, tc.want)
		}
		if got := tm.submission(t, nb); got != backToUpgrading {
			t.Errorf("the refusals left %s", got)
		}

		rolling := notebook(t, "Rolling", cleanNote)
		tm.upgraded(t, "alice", rolling)
		job := tm.abandoned(t, "alice", rolling)
		status, answer := tm.submitUpgrade(t, "alice", rolling)
		wantCode(t, "an upgrade rolling back", status, answer, cell{http.StatusConflict, "wiki.upgrade_state"})
		tm.endedBackground(t, "alice", job)

		checking := notebook(t, "Checking", cleanNote)
		up := tm.upgraded(t, "alice", checking)
		tm.reviewing(t, checking, up.ID)
		status, answer = tm.submitUpgrade(t, "alice", checking)
		wantCode(t, "a submission still checked", status, answer, cell{http.StatusConflict, "wiki.upgrade_state"})
		if !strings.Contains(answer, "The previous submission is still being checked: check again in a moment.") {
			t.Errorf("a submission still checked = %s, want its detail", answer)
		}

		stale := notebook(t, "Stale", cleanNote)
		tm.upgraded(t, "alice", stale)
		tm.exec(t, "UPDATE indexed_pages SET extractor = extractor - 1 WHERE node_id IN (SELECT id FROM nodes WHERE notebook_id = $1 "+
			"AND name = 'Note')", stale)
		if modes := tm.modesOf(t, stale, eng, marker, func() {
			status, answer := tm.submitUpgrade(t, "alice", stale)
			wantCode(t, "a stale index", status, answer, cell{http.StatusConflict, "wiki.upgrade_stale_index"})
		}); modes != 2 {
			t.Errorf("a stale index brought %d mode frames, want the two transactions'", modes)
		}
		if got := tm.submission(t, stale); got != backToUpgrading {
			t.Errorf("a stale index left %s", got)
		}
		tm.exec(t, "UPDATE indexed_pages SET extractor = extractor + 1 WHERE node_id IN (SELECT id FROM nodes WHERE notebook_id = $1 "+
			"AND name = 'Note')", stale)

		if modes := tm.modesOf(t, nb, eng, marker, func() {
			status, answer := tm.submitUpgrade(t, "alice", nb)
			wantCode(t, "a gate not passed", status, answer, cell{http.StatusConflict, "wiki.upgrade_not_ready"})
			if !strings.Contains(answer, "These gates did not pass: frontmatter.") {
				t.Errorf("a gate not passed = %s, want it named", answer)
			}
		}); modes != 2 {
			t.Errorf("a gate not passed brought %d mode frames, want the two transactions'", modes)
		}
		if got := tm.submission(t, nb); got != backToUpgrading {
			t.Errorf("a gate not passed left %s", got)
		}

		// AGENTS alone: the start's AGENTS written again with no body by
		// the initiator's agent, every other gate passed.
		agentless := notebook(t, "Agentless", cleanNote)
		tm.upgraded(t, "alice", agentless)
		version := queryStrings(t, tm.pool, `SELECT n.id::text || ':' || c.revision FROM nodes n JOIN page_contents c ON c.node_id = n.id
			WHERE n.notebook_id = $1 AND n.parent_id IS NULL AND n.name = 'AGENTS'`, agentless)[0]
		if r := tm.agent(t, "alice", agentless).use(t, "write", map[string]any{"path": "AGENTS.md", "content": "---\nsummary: none\n---\n",
			"base_version": version}).result(t); r["isError"] == true {
			t.Fatalf("AGENTS without a body = %v", r)
		}
		status, answer = tm.submitUpgrade(t, "alice", agentless)
		wantCode(t, "AGENTS without a body", status, answer, cell{http.StatusConflict, "wiki.upgrade_not_ready"})
		if !strings.Contains(answer, "These gates did not pass: agents.") {
			t.Errorf("AGENTS without a body = %s, want agents alone named", answer)
		}
		if got := tm.submission(t, agentless); got != backToUpgrading {
			t.Errorf("AGENTS without a body left %s", got)
		}
		checkPages(t, tm.pool)
	})

	t.Run("passed", func(t *testing.T) {
		nb := notebook(t, "Passed", cleanNote)
		up := tm.upgraded(t, "alice", nb)
		before := len((&mcpApp{logs: r.logs}).logLines(t))
		var a submittedAnswer
		if modes := tm.modesOf(t, nb, eng, marker, func() { a = tm.submitted(t, "alice", nb) }); modes != 2 {
			t.Errorf("a submission passed brought %d mode frames, want the first transaction's and the report's", modes)
		}
		if got := tm.submission(t, nb); got != reviewedReported {
			t.Errorf("the submission left %s", got)
		}
		status, answer := tm.getUpgrade(t, "alice", nb)
		var read submittedAnswer
		decodeAnswer(t, answer, &read)
		if status != http.StatusOK || read.State != "reviewing" || string(read.Report) != string(a.Report) {
			t.Errorf("the upgrade read = %d %s, want the report the submission answered", status, answer)
		}
		lines := submitLines(t, r.logs, before, nb)
		if len(lines) != 1 || lines[0]["level"] != "INFO" || lines[0]["outcome"] != "stored" || lines[0]["upgrade_id"] != up.ID ||
			lines[0]["submission_id"] != tm.submissionID(t, nb) || lines[0]["passed"] != true {
			t.Errorf("the submission's lines = %v, want one stored", lines)
		}
		status, answer = tm.submitUpgrade(t, "alice", nb)
		wantCode(t, "an upgrade with a report", status, answer, cell{http.StatusConflict, "wiki.upgrade_state"})
		if strings.Contains(answer, "still being checked") {
			t.Errorf("an upgrade with a report = %s, want no submission still checked", answer)
		}
		checkPages(t, tm.pool)
	})

	t.Run("the client gone", func(t *testing.T) {
		s := submitApp(t)
		nb := s.tm.openNotebook(t, "alice", "Gone")
		s.tm.createPageWith(t, "alice", nb, "", "Note", cleanNote)
		s.tm.upgraded(t, "alice", nb)
		s.rules.armed.Store(true)
		ctx, cancel := context.WithCancel(context.Background())
		req := newRequest(t, http.MethodPost, s.tm.base+"/api/v0/notebooks/"+nb+"/upgrade/submit", s.tm.tokens["alice"], nil).WithContext(ctx)
		sent := make(chan error, 1)
		go func() {
			res, err := client().Do(req)
			if err == nil {
				_ = res.Body.Close()
			}
			sent <- err
		}()
		select {
		case <-s.rules.held:
		case <-time.After(interleavingWait):
			t.Fatal("the submission's check took no slot")
		}
		cancel()
		if err := <-sent; err == nil {
			t.Fatal("the cancelled submission answered")
		}
		s.rules.free <- struct{}{}
		for deadline := time.Now().Add(interleavingWait); len(submitLines(t, s.logs, 0, nb)) == 0; time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("the submission of a client gone wrote no line")
			}
		}
		if lines := submitLines(t, s.logs, 0, nb); len(lines) != 1 || lines[0]["outcome"] != "stored" {
			t.Errorf("the submission's lines = %v, want one stored", lines)
		}
		if got := s.tm.submission(t, nb); got != reviewedReported {
			t.Errorf("the submission of a client gone left %s", got)
		}
	})

	t.Run("the check failed", func(t *testing.T) {
		s := submitApp(t)
		nb := s.tm.openNotebook(t, "alice", "Failed")
		s.tm.createPageWith(t, "alice", nb, "", "Note", cleanNote)
		s.tm.upgraded(t, "alice", nb)
		s.rules.failing.Store(true)
		status, answer := s.tm.submitUpgrade(t, "alice", nb)
		if status != http.StatusInternalServerError {
			t.Errorf("a submission whose check failed = %d %s, want 500", status, answer)
		}
		if lines := submitLines(t, s.logs, 0, nb); len(lines) != 1 || lines[0]["outcome"] != "failed" {
			t.Errorf("the submission's lines = %v, want one failed", lines)
		}
		if got := s.tm.submission(t, nb); got != backToUpgrading {
			t.Errorf("a check failed left %s", got)
		}
		s.tm.submitted(t, "alice", nb)
	})

	// The deadline (7.1): alice's submission's check held, dana's
	// submission is "still being checked"; the test holds the notebook's
	// row, lets the check go: alice's second transaction waits on the row
	// to REST's request deadline, earlier than its submission's moment
	// plus 6 s, and answers server_busy, a warn line, nothing written.
	// The row let go, past that moment dana takes the submission over.
	t.Run("the deadline", func(t *testing.T) {
		nb := notebook(t, "Deadline", cleanNote)
		tm.addNotebookMember(t, "alice", nb, "dana", "admin")
		tm.upgraded(t, "alice", nb)
		before := len((&mcpApp{logs: r.logs}).logLines(t))
		r.rules.armed.Store(true)
		req := newRequest(t, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade/submit", tm.tokens["alice"], nil)
		type sent struct {
			res  *http.Response
			body []byte
			at   time.Time
		}
		answered := make(chan sent, 1)
		go func() {
			res, body := sendRequest(t, req)
			answered <- sent{res, body, time.Now()}
		}()
		select {
		case <-r.rules.held:
		case <-time.After(interleavingWait):
			t.Fatal("the submission's check took no slot")
		}
		first := tm.submissionID(t, nb)
		status, answer := tm.submitUpgrade(t, "dana", nb)
		wantCode(t, "dana's submission while alice's is checked", status, answer, cell{http.StatusConflict, "wiki.upgrade_state"})
		if !strings.Contains(answer, "still being checked") {
			t.Errorf("dana's submission = %s, want the detail of a submission still checked", answer)
		}
		ctx := context.Background()
		holder, err := tm.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = holder.Rollback(ctx) }()
		if _, err := holder.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR NO KEY UPDATE", nb); err != nil {
			t.Fatal(err)
		}
		r.rules.free <- struct{}{}
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
		var at time.Time
		if err := tm.pool.QueryRow(ctx, "SELECT submitted_at FROM notebook_upgrades WHERE notebook_id = $1 AND state = 'reviewing'",
			nb).Scan(&at); err != nil {
			t.Fatal(err)
		}
		var got sent
		select {
		case got = <-answered:
		case <-time.After(interleavingWait):
			t.Fatal("alice's submission did not answer")
		}
		tm.contract.CheckResponse(t, req, got.res)
		if got.res.StatusCode != http.StatusServiceUnavailable || problemCodeOf(string(got.body)) != "server_busy" ||
			got.res.Header.Get("Retry-After") != "1" || !got.at.Before(at.Add(6*time.Second)) {
			t.Errorf("alice's submission = %d %s %v at %s, want server_busy before %s", got.res.StatusCode, got.body, got.res.Header,
				got.at, at.Add(6*time.Second))
		}
		warn := upgradeLines(t, r.logs, before, "upgrade submission timed out")
		if len(warn) != 1 || warn[0]["submission_id"] != first || len(submitLines(t, r.logs, before, nb)) != 0 {
			t.Errorf("logged %v, want one warn line of alice's submission and no other", warn)
		}
		if got, id := tm.submission(t, nb), tm.submissionID(t, nb); got != "reviewing reviewing true false false" || id != first {
			t.Errorf("past its deadline alice's submission left %s of %s, want reviewing without a report, %s", got, id, first)
		}
		if err := holder.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Until(at.Add(6*time.Second + 10*time.Millisecond)))
		tm.submitted(t, "dana", nb)
		if id := tm.submissionID(t, nb); id == first || id == "" {
			t.Errorf("dana's takeover kept the submission %s, want one of hers", id)
		}
		checkPages(t, tm.pool)
	})
}

// The submission's interleavings (M11/P2 design 7.2): #5 on the standard
// app, the others on the separate one, each in a notebook of its own; at
// the end the pages', links' and search's rules hold.
func TestUpgradeSubmitInterleavings(t *testing.T) {
	// #5: a batch of the initiator's agent first holds the notebook's
	// row, the submission's first transaction waits and its report sees
	// what the batch wrote: the one page that lacked its type has it, and
	// the check passes. The submission first, the batch waits and is
	// notebook.upgrading, writing nothing.
	t.Run("5 the submission and a running batch", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Five")
		tm.createPageWith(t, "alice", nb, "", "Note", "---\nsummary: A note.\n---\nA note.\n")
		tm.upgraded(t, "alice", nb)
		agent := tm.agent(t, "alice", nb)
		var wrote mcpAnswer
		batch := step{by: "alice", command: func() error {
			wrote = agent.use(t, "batch", batched(opEdit("Note.md", "summary: A note.", "type: note\nsummary: A note.")))
			return nil
		}}
		first, second := tm.interleaveOn(t, notebookRow(nb), batch, submitStep("alice", nb))
		if r := wrote.result(t); r["isError"] == true || first.err != nil {
			t.Fatalf("the batch first = %v, %v", r, first.err)
		}
		wantAnswer(t, "the submission after it", second, cellOK())
		var report struct {
			Report struct {
				Rules []struct {
					Rule  string `json:"rule"`
					Count int    `json:"count"`
				} `json:"rules"`
			} `json:"report"`
		}
		decodeAnswer(t, second.body, &report)
		for _, rule := range report.Report.Rules {
			if rule.Rule == "missing_type" && rule.Count != 0 {
				t.Errorf("the report counts %d pages without a type, want none: the batch wrote its type", rule.Count)
			}
		}

		other := tm.openNotebook(t, "alice", "Five again")
		tm.createPageWith(t, "alice", other, "", "Note", "---\nsummary: A note.\n---\nA note.\n")
		tm.upgraded(t, "alice", other)
		agent = tm.agent(t, "alice", other)
		batch = step{by: "alice", command: func() error {
			wrote = agent.use(t, "batch", batched(opEdit("Note.md", "summary: A note.", "type: note\nsummary: A note.")))
			return nil
		}}
		first, _ = tm.interleaveOn(t, notebookRow(other), submitStep("alice", other), batch)
		wantAnswer(t, "the submission first", first, cell{http.StatusConflict, "wiki.upgrade_not_ready"})
		if member := wrote.toolError(t); member["code"] != "notebook.upgrading" {
			t.Errorf("the batch after it = %v, want notebook.upgrading", member)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM page_contents c JOIN nodes n ON n.id = c.node_id WHERE n.notebook_id = $1 "+
			"AND c.content LIKE '%type: note%'", other); n != 0 {
			t.Errorf("the refused batch wrote %d pages", n)
		}
		checkPages(t, tm.pool)
		checkLinks(t, tm.pool)
		checkSearch(t, tm.pool)
	})

	s := submitApp(t)
	tm := s.tm
	tm.join(t, "dana", "member")
	// dana is each notebook's other admin: her membership's id, by the
	// notebook's.
	danas := map[string]string{}
	// gated is a notebook of alice's, its one page Note holding content,
	// dana its other admin, whose upgrade alice started.
	gated := func(t *testing.T, name, content string) string {
		t.Helper()
		nb := tm.openNotebook(t, "alice", name)
		tm.createPageWith(t, "alice", nb, "", "Note", content)
		danas[nb] = tm.addNotebookMember(t, "alice", nb, "dana", "admin")
		tm.upgraded(t, "alice", nb)
		return nb
	}
	// sent sends c aside and answers what it got.
	sent := func(t *testing.T, c step) func() answer {
		t.Helper()
		send := tm.sender(t, c)
		done := make(chan answer, 1)
		go func() { done <- send() }()
		return func() answer {
			t.Helper()
			select {
			case a := <-done:
				if a.err == nil {
					tm.contract.CheckResponse(t, a.req, a.res)
				}
				return a
			case <-time.After(interleavingWait):
				t.Fatalf("%s did not answer", c.name())
				return answer{}
			}
		}
	}

	// #6: a check's snapshot taken, held; the agent writes the last
	// summary; the submission's flight is made, its own, and its report,
	// from a snapshot after the write, passes. Let go, the check answers
	// its own result, the snapshot before: not passed.
	t.Run("6 a check held, a write, the submission", func(t *testing.T) {
		nb := gated(t, "Six", "---\ntype: note\n---\nA note.\n")
		letGo := s.held(t, checkStep("alice", nb, ""))
		if r := tm.agent(t, "alice", nb).use(t, "write", map[string]any{"path": "Note.md", "content": cleanNote,
			"base_version": queryStrings(t, tm.pool, "SELECT n.id::text || ':' || c.revision::text FROM nodes n JOIN page_contents c "+
				"ON c.node_id = n.id WHERE n.notebook_id = $1 AND n.name = 'Note'", nb)[0]}).result(t); r["isError"] == true {
			t.Fatalf("the agent's write of the summary = %v", r)
		}
		s.drain()
		submitted := sent(t, submitStep("alice", nb))
		s.event(t, wiki.RunCreated, nb, 0)
		wantAnswer(t, "the submission", submitted(), cellOK())
		checked := letGo()
		wantAnswer(t, "the check held", checked, cellOK())
		if !strings.Contains(checked.body, `"passed":false`) {
			t.Errorf("the check held = %s, want its snapshot's result, not passed", checked.body)
		}
		if got := tm.submission(t, nb); got != reviewedReported {
			t.Errorf("the submission left %s", got)
		}
	})

	// #7: alice's submission's check held, dana abandons the upgrade; let
	// go, the second transaction reads it rolling back: nothing written,
	// wiki.upgrade_state, superseded; the rollback runs to its end.
	t.Run("7 the abandonment between the two transactions", func(t *testing.T) {
		nb := gated(t, "Seven", cleanNote)
		before := len((&mcpApp{logs: s.logs}).logLines(t))
		letGo := s.held(t, submitStep("alice", nb))
		job := tm.abandoned(t, "dana", nb)
		wantAnswer(t, "the submission, abandoned meanwhile", letGo(), cell{http.StatusConflict, "wiki.upgrade_state"})
		if lines := submitLines(t, s.logs, before, nb); len(lines) != 1 || lines[0]["outcome"] != "superseded" {
			t.Errorf("the submission's lines = %v, want one superseded", lines)
		}
		// The rollback runs on: it may have ended by now.
		left := queryStrings(t, tm.pool, `SELECT (state IN ('rolling_back', 'rolled_back'))::text || ' ' ||
				(submission_id IS NOT NULL AND submitted_by_id IS NOT NULL AND submitted_at IS NOT NULL)::text || ' ' ||
				(report IS NOT NULL)::text
			FROM notebook_upgrades WHERE notebook_id = $1`, nb)
		if !slices.Equal(left, []string{"true true false"}) {
			t.Errorf("the abandonment left %v, want it rolling back or rolled back, its submission kept, no report", left)
		}
		if j := tm.endedBackground(t, "alice", job); j.State != "succeeded" {
			t.Errorf("the rollback = %+v, want it done", j)
		}
	})

	// #15: alice's submission's check held; dana's submission reads it
	// reviewing without a report, within its time: wiki.upgrade_state,
	// "still being checked", no flight. Let go, alice's keeps its report.
	t.Run("15 a second submission while the first is checked", func(t *testing.T) {
		nb := gated(t, "Fifteen", cleanNote)
		letGo := s.held(t, submitStep("alice", nb))
		s.drain()
		status, answer := tm.submitUpgrade(t, "dana", nb)
		wantCode(t, "dana's submission", status, answer, cell{http.StatusConflict, "wiki.upgrade_state"})
		if !strings.Contains(answer, "still being checked") {
			t.Errorf("dana's submission = %s, want the detail of a submission still checked", answer)
		}
		select {
		case e := <-s.events:
			t.Errorf("dana's refused submission made a run: %+v", e)
		default:
		}
		wantAnswer(t, "alice's submission", letGo(), cellOK())
		if got := tm.submission(t, nb); got != reviewedReported {
			t.Errorf("the submission left %s", got)
		}
	})

	// The submitter demoted between the two transactions: dana's check
	// held, alice makes her the notebook's editor; let go, the report is
	// kept, the decision before the COMMIT forbidden, an info line
	// stored.
	t.Run("the submitter demoted", func(t *testing.T) {
		nb := gated(t, "Demoted", cleanNote)
		before := len((&mcpApp{logs: s.logs}).logLines(t))
		letGo := s.held(t, submitStep("dana", nb))
		tm.send(t, request("alice", http.MethodPatch, "/api/v0/notebook-members/"+danas[nb], `{"role":"editor"}`), http.StatusOK)
		wantAnswer(t, "the demoted submitter's submission", letGo(), cellForbidden())
		if lines := submitLines(t, s.logs, before, nb); len(lines) != 1 || lines[0]["outcome"] != "stored" {
			t.Errorf("the submission's lines = %v, want one stored", lines)
		}
		if got := tm.submission(t, nb); got != reviewedReported {
			t.Errorf("the submission left %s", got)
		}
	})

	// A check across a submission (P2C-I2): a check held, its snapshot
	// before the submission, whose flight is made; let go, the check
	// answers its upgrading snapshot's result. The submission keeps its
	// report; a check then computes the upgrade reviewing, passed.
	t.Run("a check across a submission", func(t *testing.T) {
		nb := gated(t, "Across", cleanNote)
		letGo := s.held(t, checkStep("alice", nb, ""))
		s.drain()
		submitted := sent(t, submitStep("alice", nb))
		s.event(t, wiki.RunCreated, nb, 0)
		if got := tm.submission(t, nb); !strings.HasPrefix(got, "reviewing reviewing true false") {
			t.Errorf("the submission's first transaction left %s", got)
		}
		wantAnswer(t, "the check held", letGo(), cellOK())
		wantAnswer(t, "the submission", submitted(), cellOK())
		if a := tm.checked(t, "alice", nb); !a.Passed {
			t.Errorf("the check reviewing failed %v", a.failed())
		}
	})
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkSearch(t, tm.pool)
}
