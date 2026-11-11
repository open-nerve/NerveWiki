package bootstrap

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// An upgrade's approval through the whole program (M11/P3 design 3.6,
// 7.1); the wiki module's half with fakes is wiki/app's TestApproveUpgrade.

// approveUpgrade is by's approval of nb's upgrade, as the API answers it.
func (tm acmeTeam) approveUpgrade(t *testing.T, by, nb string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade/approve", tm.tokens[by], "")
}

// approveStep is by's approval of nb's upgrade, as a step.
func approveStep(by, nb string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+nb+"/upgrade/approve", "")
}

// withReviewers is tm with dana a member of acme and erin an account of
// its, a member too: the third and the fourth of a notebook's admins.
func (tm acmeTeam) withReviewers(t *testing.T) acmeTeam {
	t.Helper()
	tm.join(t, "dana", "member")
	tm.tokens["erin"] = registerAccount(t, tm.contract, tm.base, "erin@example.com").AccessToken
	tm.join(t, "erin", "member")
	return tm
}

// reviewedNotebook is a notebook whose upgrade is in review with a report
// that passed: its id, raw (the proposed source root), the upgrade's and
// its submission's ids, its seeded AGENTS, and the sources the approval
// hashes, two pages, a page with a body over an empty one, and two
// attachments, one of no bytes, by id.
type reviewedNotebook struct {
	id, raw, upgrade, submission, agents string
	sources                              []string
	note, paper                          string
}

// reviewed is a notebook of alice's, titled title, dana and erin its
// other admins: at its root a note with clean frontmatter and raw; in raw
// two papers, an empty page, a folder page with a body and an empty page
// in it, an attachment and one of no bytes. alice starts its upgrade
// (raw its source root, AGENTS written), erin submits it, its check
// passing: three admins of their own, so that the approval's dana is
// neither (M11/P3 design 7.1, review P3A-N5).
func (tm acmeTeam) reviewed(t *testing.T, title string) reviewedNotebook {
	t.Helper()
	r := reviewedNotebook{id: tm.openNotebook(t, "alice", title)}
	tm.addNotebookMember(t, "alice", r.id, "dana", "admin")
	tm.addNotebookMember(t, "alice", r.id, "erin", "admin")
	r.note = tm.createPageWith(t, "alice", r.id, "", "Note", cleanNote)
	r.raw = tm.createPage(t, "alice", r.id, "", "raw")
	r.paper = tm.createPageWith(t, "alice", r.id, r.raw, "Paper one", "The first paper.\n")
	two := tm.createPageWith(t, "alice", r.id, r.raw, "Paper two", "The second paper.\n")
	tm.createPage(t, "alice", r.id, r.raw, "Empty")
	folder := tm.createPageWith(t, "alice", r.id, r.raw, "Folder", "A folder's own words.\n")
	tm.createPage(t, "alice", r.id, folder, "Inside")
	chart := tm.upload(t, "alice", r.id, r.raw, "chart.png", "chart bytes").ID
	zero := tm.upload(t, "alice", r.id, r.raw, "empty.bin", "").ID
	r.sources = []string{r.paper, two, folder, chart, zero}
	slices.Sort(r.sources)
	u := tm.upgraded(t, "alice", r.id)
	if u.SourceRootID != r.raw || len(u.SeededPageIDs) != 1 {
		t.Fatalf("the start of %s = %+v, want raw its source root, AGENTS written", title, u)
	}
	r.upgrade, r.agents = u.ID, u.SeededPageIDs[0]
	tm.submitted(t, "erin", r.id)
	r.submission = tm.submissionID(t, r.id)
	return r
}

// notebookMode is the notebook nb's mode, its source root and whether it
// has an upgrade: "standard - true" or "llm_wiki <root> false".
func (tm acmeTeam) notebookMode(t *testing.T, nb string) string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT mode || ' ' || coalesce(source_root_id::text, '-') || ' ' || (upgrade_id IS NOT NULL)::text
		FROM notebooks WHERE id = $1`, nb)[0]
}

// lineKeysOf are line's keys, sorted.
func lineKeysOf(line map[string]any) []string { return slices.Sorted(maps.Keys(line)) }

// The approval's codes in M11/P3 design 3.6's order, each refusal leaving
// the upgrade and the notebook as they were: no notebook, one carol does
// not see, an editor's; no upgrade, one upgrading, one reviewing without a
// report just submitted (not "still being checked": an approval does not
// wait), then an hour after. Approved by dana, neither who started it nor
// who submitted it: the answer the upgrade as read before, approved; the
// notebook an LLM Wiki whose source root is raw, its upgrade ended, the
// baseline gone, the row approved by dana; the sources hashed as the
// invariant wants, five; the upgrade read no more; an LLM Wiki's contract
// then (AGENTS protected, a source immutable); alice's other tab told by
// one mode frame; one info line. Gates 1 to 3 checked again under the
// lock: AGENTS's body cleared by SQL, the defect it stands for,
// wiki.upgrade_not_ready, nothing written, one error line. On apps of
// their own: a rebuild that writes nothing (the invariant then names
// exactly this notebook's sources), and the approval's own deadline,
// inside a statement before the decision, inside one after the writes,
// and between two.
func TestUpgradeApproval(t *testing.T) {
	logs := &syncBuffer{}
	tm := newAcmeTeamOn(t, "member", "", nil, nil, slog.New(slog.NewJSONHandler(logs, nil))).withReviewers(t)
	eng := tm.openNotebook(t, "alice", "Eng")
	marker := tm.createPage(t, "alice", eng, "", "Marker")

	t.Run("the codes in their order", func(t *testing.T) {
		upgrading := tm.openNotebook(t, "alice", "Upgrading")
		tm.createPageWith(t, "alice", upgrading, "", "Note", cleanNote)
		tm.addNotebookMember(t, "alice", upgrading, "dana", "admin")
		up := tm.upgraded(t, "alice", upgrading)
		plain := tm.openNotebook(t, "alice", "Plain")
		tm.addNotebookMember(t, "alice", plain, "dana", "admin")
		for _, tc := range []struct {
			what, by, nb string
			want         cell
		}{
			{"no notebook", "dana", "0199a2b4-0000-7000-8000-000000000000", cell{http.StatusNotFound, "notebook.not_found"}},
			{"a notebook carol does not see", "carol", upgrading, cell{http.StatusNotFound, "notebook.not_found"}},
			{"an editor's", "bob", upgrading, cellForbidden()},
			{"no upgrade", "dana", plain, cell{http.StatusConflict, "wiki.upgrade_state"}},
			{"an upgrade upgrading", "dana", upgrading, cell{http.StatusConflict, "wiki.upgrade_state"}},
		} {
			status, answer := tm.approveUpgrade(t, tc.by, tc.nb)
			wantCode(t, tc.what, status, answer, tc.want)
		}
		tm.reviewing(t, upgrading, up.ID)
		for _, what := range []string{"reviewing without a report, just submitted", "reviewing without a report, an hour after"} {
			status, answer := tm.approveUpgrade(t, "dana", upgrading)
			wantCode(t, what, status, answer, cell{http.StatusConflict, "wiki.upgrade_state"})
			if strings.Contains(answer, "still being checked") {
				t.Errorf("%s = %s, want no submission still checked", what, answer)
			}
			if got, mode := tm.submission(t, upgrading), tm.notebookMode(t, upgrading); got != "reviewing reviewing true false false" ||
				mode != "standard - true" {
				t.Errorf("%s left %s, %s", what, got, mode)
			}
			tm.exec(t, "UPDATE notebook_upgrades SET submitted_at = submitted_at - interval '1 hour' WHERE id = $1", up.ID)
		}
		if mode := tm.notebookMode(t, plain); mode != "standard - false" {
			t.Errorf("no upgrade left %s", mode)
		}
		checkPages(t, tm.pool)
	})

	t.Run("approved, another tab told", func(t *testing.T) {
		r := tm.reviewed(t, "Approved")
		status, before := tm.getUpgrade(t, "dana", r.id)
		if status != http.StatusOK {
			t.Fatalf("the upgrade in review = %d %s", status, before)
		}
		var answer string
		lines := len((&mcpApp{logs: logs}).logLines(t))
		if modes := tm.modesOf(t, r.id, eng, marker, func() {
			status, answer = tm.approveUpgrade(t, "dana", r.id)
		}); status != http.StatusOK || modes != 1 {
			t.Fatalf("the approval = %d %s, %d mode frames to alice's other tab; want 200, one", status, answer, modes)
		}
		var got, want map[string]any
		decodeAnswer(t, answer, &got)
		decodeAnswer(t, before, &want)
		want["state"] = "approved"
		if !maps.EqualFunc(got, want, func(a, b any) bool { x, _ := json.Marshal(a); y, _ := json.Marshal(b); return string(x) == string(y) }) {
			t.Errorf("approved %s\nwant the upgrade read before, approved: %v", answer, want)
		}
		alice, erin, dana := tm.userOf(t, "alice"), tm.userOf(t, "erin"), tm.userOf(t, "dana")
		type userRef struct {
			UserID string `json:"user_id"`
		}
		var a struct {
			State        string  `json:"state"`
			SourceRootID string  `json:"source_root_id"`
			StartedBy    userRef `json:"started_by"`
			SubmittedBy  userRef `json:"submitted_by"`
			Report       struct {
				Passed bool `json:"passed"`
			} `json:"report"`
			Ending        *string `json:"ending"`
			RollbackJobID *string `json:"rollback_job_id"`
		}
		decodeAnswer(t, answer, &a)
		if a.State != "approved" || a.StartedBy.UserID != alice || a.SubmittedBy.UserID != erin || !a.Report.Passed || a.Ending != nil ||
			a.RollbackJobID != nil || a.SourceRootID != r.raw {
			t.Errorf("approved %s, want started by alice, submitted by erin, its report passed, no rollback", answer)
		}
		if mode := tm.notebookMode(t, r.id); mode != "llm_wiki "+r.raw+" false" {
			t.Errorf("the notebook approved: %s, want an LLM Wiki, raw its source root, no upgrade", mode)
		}
		if got := queryStrings(t, tm.pool, `SELECT u.state || ' ' || u.updated_by_id || ' ' || (u.updated_at = n.updated_at)::text || ' ' ||
				n.updated_by_id || ' ' || (u.report IS NOT NULL AND u.submission_id::text = $2)::text
			FROM notebook_upgrades u JOIN notebooks n ON n.id = u.notebook_id WHERE u.id = $1`, r.upgrade, r.submission); !slices.Equal(got,
			[]string{"approved " + dana + " true " + dana + " true"}) {
			t.Errorf("the rows approved: %v, want approved by dana at the notebook's moment, its submission and report kept", got)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1", r.upgrade); n != 0 {
			t.Errorf("%d baseline rows left, want none", n)
		}
		if got := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND source_hash IS NOT NULL ORDER BY id",
			r.id); !slices.Equal(got, r.sources) {
			t.Errorf("hashed %v, want the five sources %v", got, r.sources)
		}
		assertSourceHashesMatchRebuild(t, context.Background(), tm.pool)
		checkPages(t, tm.pool)
		status, read := tm.getUpgrade(t, "dana", r.id)
		wantCode(t, "the upgrade approved", status, read, cell{http.StatusNotFound, "wiki.upgrade_not_found"})
		_, read = ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+r.id, tm.tokens["dana"], "")
		if !strings.Contains(read, `"mode":"llm_wiki"`) || !strings.Contains(read, `"upgrade":null`) {
			t.Errorf("the notebook read %s, want an LLM Wiki without an upgrade", read)
		}
		status, deleted := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/nodes/"+r.agents, tm.tokens["dana"], "")
		wantCode(t, "AGENTS deleted", status, deleted, cell{http.StatusConflict, "page.protected"})
		rev := tm.content(t, "alice", r.paper).Revision
		c := contentWrite("alice", r.paper, "Another paper.\n", rev, "")
		status, written := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], c.body)
		wantCode(t, "a source written", status, written, cell{http.StatusConflict, "page.source_immutable"})
		info := upgradeLines(t, logs, lines, "upgrade approved")
		if len(info) != 1 || !slices.Equal(lineKeysOf(info[0]), []string{"elapsed_ms", "level", "msg", "notebook_id", "sources_hashed",
			"submission_id", "time", "upgrade_id", "user_id", "workspace_id"}) {
			t.Fatalf("logged %v, want one info line", info)
		}
		acme := queryStrings(t, tm.pool, "SELECT id::text FROM workspaces WHERE slug = 'acme'")[0]
		if l := info[0]; l["level"] != "INFO" || l["workspace_id"] != acme || l["notebook_id"] != r.id || l["upgrade_id"] != r.upgrade ||
			l["submission_id"] != r.submission || l["user_id"] != dana || l["sources_hashed"] != 5.0 {
			t.Errorf("the info line %v", l)
		}
	})

	t.Run("the gates checked again under the lock", func(t *testing.T) {
		r := tm.reviewed(t, "Gates")
		lines := len((&mcpApp{logs: logs}).logLines(t))
		kept := queryStrings(t, tm.pool, "SELECT content FROM page_contents WHERE node_id = $1", r.agents)[0]
		tm.exec(t, "UPDATE page_contents SET content = '', byte_size = 0, content_hash = sha256(''::bytea) WHERE node_id = $1", r.agents)
		status, answer := tm.approveUpgrade(t, "dana", r.id)
		wantCode(t, "AGENTS without a body", status, answer, cell{http.StatusConflict, "wiki.upgrade_not_ready"})
		if !strings.Contains(answer, "These gates did not pass: agents. The upgrade is still in review: send it back, or reject it.") {
			t.Errorf("AGENTS without a body = %s, want agents named, still in review", answer)
		}
		if got, mode := tm.submission(t, r.id), tm.notebookMode(t, r.id); got != reviewedReported || mode != "standard - true" {
			t.Errorf("the gates not passed left %s, %s", got, mode)
		}
		if l := upgradeLines(t, logs, lines, "upgrade approved"); len(l) != 0 {
			t.Errorf("logged %v", l)
		}
		failed := upgradeLines(t, logs, lines, "upgrade gates failed at approval")
		if len(failed) != 1 || !slices.Equal(lineKeysOf(failed[0]), []string{"failed_gates", "level", "msg", "notebook_id", "time", "upgrade_id"}) {
			t.Fatalf("logged %v, want one error line", failed)
		}
		if l := failed[0]; l["level"] != "ERROR" || l["notebook_id"] != r.id || l["upgrade_id"] != r.upgrade ||
			!slices.Equal(stringsOf(l["failed_gates"]), []string{"agents"}) {
			t.Errorf("the error line %v", l)
		}
		tm.exec(t, `UPDATE page_contents SET content = $2, byte_size = octet_length(convert_to($2, 'UTF8')),
			content_hash = sha256(convert_to($2, 'UTF8')) WHERE node_id = $1`, r.agents, kept)
		checkPages(t, tm.pool)
	})

	t.Run("a rebuild that writes nothing", func(t *testing.T) {
		tm := newAcmeTeamOn(t, "member", "", nil, &wiring{wiki: func(d *wiki.Deps) { d.SourceHashes = noRebuild{} }},
			slog.New(slog.DiscardHandler)).withReviewers(t)
		r := tm.reviewed(t, "Unhashed")
		if status, answer := tm.approveUpgrade(t, "dana", r.id); status != http.StatusOK {
			t.Fatalf("the approval = %d %s", status, answer)
		}
		var named []string
		for _, m := range sourceHashRebuildMismatches(t, context.Background(), tm.pool) {
			named = append(named, strings.Fields(m)[1])
		}
		slices.Sort(named)
		if !slices.Equal(named, r.sources) {
			t.Errorf("the invariant named %v, want this notebook's five sources %v", named, r.sources)
		}
	})

	// Its own deadline, 2 s (the request's 4 s), in each place: 503 at
	// once, Retry-After: 1, one warn line and no error, nothing written.
	t.Run("its own deadline", func(t *testing.T) {
		logs := &syncBuffer{}
		held := &heldRebuild{}
		tm := newAcmeTeamOn(t, "member", "", nil, &wiring{wiki: func(d *wiki.Deps) {
			held.SourceHashes = d.SourceHashes
			d.SourceHashes, d.ApproveTimeout = held, 2*time.Second
		}}, slog.New(slog.NewJSONHandler(logs, nil))).withReviewers(t)
		timedOut := func(t *testing.T, what string, r reviewedNotebook, before int, res *http.Response, body []byte) {
			t.Helper()
			wantCode(t, what, res.StatusCode, string(body), cell{http.StatusServiceUnavailable, "server_busy"})
			if res.Header.Get("Retry-After") != "1" {
				t.Errorf("%s: Retry-After %q, want 1", what, res.Header.Get("Retry-After"))
			}
			if got, mode := tm.submission(t, r.id), tm.notebookMode(t, r.id); got != reviewedReported || mode != "standard - true" {
				t.Errorf("%s left %s, %s", what, got, mode)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND source_hash IS NOT NULL", r.id); n != 0 {
				t.Errorf("%s left %d source hashes", what, n)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1", r.upgrade); n == 0 {
				t.Errorf("%s deleted the baseline", what)
			}
			lines := upgradeLines(t, logs, before, "upgrade approval timed out")
			if len(lines) != 1 || lines[0]["level"] != "WARN" || lines[0]["notebook_id"] != r.id || lines[0]["timeout_ms"] != 2000.0 {
				t.Errorf("%s logged %v, want one warn line of the timeout", what, lines)
			}
			for _, l := range (&mcpApp{logs: logs}).logLines(t)[before:] {
				if l["level"] == "ERROR" || l["msg"] == "upgrade approved" {
					t.Errorf("%s logged %v", what, l)
				}
			}
			checkPages(t, tm.pool)
		}
		approve := func(t *testing.T, r reviewedNotebook) func() (*http.Response, []byte) {
			t.Helper()
			req := newRequest(t, http.MethodPost, tm.base+"/api/v0/notebooks/"+r.id+"/upgrade/approve", tm.tokens["dana"], nil)
			done := make(chan struct{})
			var res *http.Response
			var body []byte
			go func() {
				defer close(done)
				res, body = sendRequest(t, req)
			}()
			return func() (*http.Response, []byte) {
				t.Helper()
				select {
				case <-done:
				case <-time.After(interleavingWait):
					t.Fatal("the approval did not answer")
				}
				tm.contract.CheckResponse(t, req, res)
				return res, body
			}
		}
		// Waiting for the notebook's row the test holds, before the
		// decision.
		t.Run("inside a statement, before the decision", func(t *testing.T) {
			r := tm.reviewed(t, "Waiting")
			before := len((&mcpApp{logs: logs}).logLines(t))
			holder := tm.holdNotebook(t, r.id)
			res, body := approve(t, r)()
			if err := holder.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
			timedOut(t, "an approval past its deadline waiting for the notebook", r, before, res, body)
		})
		// Waiting for a source's row the test holds, after the notebook's
		// write and the hashes cleared: SetSourceHashes's.
		t.Run("inside a statement, after the writes", func(t *testing.T) {
			r := tm.reviewed(t, "Written")
			before := len((&mcpApp{logs: logs}).logLines(t))
			ctx := context.Background()
			holder, err := tm.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = holder.Rollback(ctx) }()
			if _, err := holder.Exec(ctx, "SELECT 1 FROM nodes WHERE id = $1 FOR UPDATE", r.paper); err != nil {
				t.Fatal(err)
			}
			held.entered.Store(false)
			answered := approve(t, r)
			pgtest.WaitForLockWaitsOn(t, tm.pool, "nodes", 1, interleavingWait)
			res, body := answered()
			if err := holder.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if !held.entered.Load() {
				t.Error("the approval timed out before its rebuild")
			}
			timedOut(t, "an approval past its deadline waiting for a source", r, before, res, body)
		})
		// The rebuild waiting in Go for its transaction's end.
		t.Run("between statements", func(t *testing.T) {
			r := tm.reviewed(t, "Between")
			before := len((&mcpApp{logs: logs}).logLines(t))
			held.armed.Store(true)
			defer held.armed.Store(false)
			res, body := approve(t, r)()
			timedOut(t, "an approval past its deadline between statements", r, before, res, body)
		})
	})
}

// stringsOf is v, a JSON array of strings as decoded, as a slice.
func stringsOf(v any) []string {
	list, _ := v.([]any)
	out := make([]string, len(list))
	for i, x := range list {
		out[i], _ = x.(string)
	}
	return out
}

// noRebuild is a rebuild of the source hashes that writes none: the
// invariant's counterexample (00-M11 design 9).
type noRebuild struct{}

func (noRebuild) Rebuild(context.Context, uuid.UUID, uuid.UUID) (int, error) { return 0, nil }

// heldRebuild is the page module's rebuild, which records that the
// approval entered it, and once armed waits there for its transaction's
// end: the approval's deadline runs out between two statements.
type heldRebuild struct {
	wiki.SourceHashes
	armed, entered atomic.Bool
}

func (h *heldRebuild) Rebuild(ctx context.Context, notebookID, rootID uuid.UUID) (int, error) {
	h.entered.Store(true)
	if !h.armed.Load() {
		return h.SourceHashes.Rebuild(ctx, notebookID, rootID)
	}
	<-ctx.Done()
	return 0, ctx.Err()
}
