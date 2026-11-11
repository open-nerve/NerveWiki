package bootstrap

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// An upgrade's send-back and rejection through the whole program
// (M11/P3 design 3.7, 3.8, 7.1), and the review's interleavings (7.2);
// the wiki module's half with fakes is wiki/app's TestRejectUpgrade and
// TestSendBackUpgrade.

// rejectUpgrade is by's rejection of nb's upgrade, as the API answers it.
func (tm acmeTeam) rejectUpgrade(t *testing.T, by, nb string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade/reject", tm.tokens[by], "")
}

// sendBackUpgrade is by's send-back of nb's upgrade, as the API answers
// it.
func (tm acmeTeam) sendBackUpgrade(t *testing.T, by, nb string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade/send-back", tm.tokens[by], "")
}

// rejectStep and sendBackStep are by's rejection and send-back of nb's
// upgrade, as steps.
func rejectStep(by, nb string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+nb+"/upgrade/reject", "")
}

func sendBackStep(by, nb string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+nb+"/upgrade/send-back", "")
}

// rejected is by's rejection of nb's upgrade, which must answer 200,
// rolling back, rejected, with a job; it answers the job's id.
func (tm acmeTeam) rejected(t *testing.T, by, nb string) string {
	t.Helper()
	status, answer := tm.rejectUpgrade(t, by, nb)
	var a rollbackAnswer
	if status == http.StatusOK {
		decodeAnswer(t, answer, &a)
	}
	if status != http.StatusOK || a.State != "rolling_back" || a.Ending == nil || *a.Ending != "rejected" || a.RollbackJobID == nil {
		t.Fatalf("%s's rejection of %s = %d %s, want it rolling back, rejected, with a job", by, nb, status, answer)
	}
	return *a.RollbackJobID
}

// ended is the notebook nb's last upgrade's row once its rollback ended:
// its state, its ending, whether complete, whether its submission's three
// columns and its report are kept; and the notebook's upgrade state.
func (tm acmeTeam) ended(t *testing.T, nb string) string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT u.state || ' ' || coalesce(u.ending, '-') || ' ' || coalesce(u.rollback_complete::text, '-') || ' ' ||
			(u.submission_id IS NOT NULL AND u.submitted_by_id IS NOT NULL AND u.submitted_at IS NOT NULL)::text || ' ' ||
			(u.report IS NOT NULL)::text || ' ' || coalesce(n.upgrade_state, '-')
		FROM notebook_upgrades u JOIN notebooks n ON n.id = u.notebook_id WHERE u.notebook_id = $1 ORDER BY u.created_at DESC LIMIT 1`, nb)[0]
}

// The send-back's and the rejection's codes in M11/P3 design 3.7 and
// 3.8's orders, each refusal leaving the upgrade as it was; then each
// done, through the whole program: sent back, the upgrade upgrading, its
// submission and report gone, one mode frame, the initiator's agent able
// to write a page's frontmatter, a submission then keeping a report of its
// own; rejected, with
// its report and without one, rolling back with its job, the rollback
// done putting the notebook back (AGENTS the start wrote gone, the note as
// it was), two mode frames in all, the submission kept. One info line of
// each, whole.
func TestUpgradeReview(t *testing.T) {
	logs := &syncBuffer{}
	tm := newAcmeTeamOn(t, "member", "", nil, nil, slog.New(slog.NewJSONHandler(logs, nil))).withReviewers(t)
	eng := tm.openNotebook(t, "alice", "Eng")
	marker := tm.createPage(t, "alice", eng, "", "Marker")
	acme := queryStrings(t, tm.pool, "SELECT id::text FROM workspaces WHERE slug = 'acme'")[0]
	// notebook is a notebook of alice's with a clean note, dana its other
	// admin.
	notebook := func(t *testing.T, name string) string {
		t.Helper()
		nb := tm.openNotebook(t, "alice", name)
		tm.createPageWith(t, "alice", nb, "", "Note", cleanNote)
		tm.addNotebookMember(t, "alice", nb, "dana", "admin")
		return nb
	}
	codes := func(t *testing.T, of string, act func(t *testing.T, by, nb string) (int, string)) string {
		t.Helper()
		upgrading, rolling, plain := notebook(t, "Upgrading "+of), notebook(t, "Rolling "+of), notebook(t, "Plain "+of)
		tm.upgraded(t, "alice", upgrading)
		tm.upgraded(t, "alice", rolling)
		job := tm.abandoned(t, "alice", rolling)
		for _, tc := range []struct {
			what, by, nb string
			want         cell
		}{
			{"no notebook", "dana", "0199a2b4-0000-7000-8000-000000000000", cell{http.StatusNotFound, "notebook.not_found"}},
			{"a notebook carol does not see", "carol", upgrading, cell{http.StatusNotFound, "notebook.not_found"}},
			{"an editor's", "bob", upgrading, cellForbidden()},
			{"no upgrade", "dana", plain, cell{http.StatusConflict, "wiki.upgrade_state"}},
			{"an upgrade upgrading", "dana", upgrading, cell{http.StatusConflict, "wiki.upgrade_state"}},
			{"an upgrade rolling back", "dana", rolling, cell{http.StatusConflict, "wiki.upgrade_state"}},
		} {
			status, answer := act(t, tc.by, tc.nb)
			wantCode(t, tc.what, status, answer, tc.want)
		}
		if got := tm.submission(t, upgrading); got != backToUpgrading {
			t.Errorf("the refusals left %s", got)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", upgrading); n != 0 {
			t.Errorf("the refusals started %d jobs", n)
		}
		tm.endedBackground(t, "alice", job)
		return upgrading
	}

	t.Run("the send-back's codes", func(t *testing.T) {
		upgrading := codes(t, "send-back", tm.sendBackUpgrade)
		up := queryStrings(t, tm.pool, "SELECT upgrade_id::text FROM notebooks WHERE id = $1", upgrading)[0]
		tm.reviewing(t, upgrading, up)
		status, answer := tm.sendBackUpgrade(t, "dana", upgrading)
		wantCode(t, "a submission still checked", status, answer, cell{http.StatusConflict, "wiki.upgrade_state"})
		if !strings.Contains(answer, "The previous submission is still being checked: check again in a moment.") {
			t.Errorf("a submission still checked = %s, want its detail", answer)
		}
		if got := tm.submission(t, upgrading); got != "reviewing reviewing true false false" {
			t.Errorf("a submission still checked left %s", got)
		}
		tm.exec(t, "UPDATE notebook_upgrades SET submitted_at = submitted_at - interval '1 hour' WHERE id = $1", up)
		if status, answer := tm.sendBackUpgrade(t, "dana", upgrading); status != http.StatusOK {
			t.Errorf("a submission taken over = %d %s, want 200", status, answer)
		}
		if got := tm.submission(t, upgrading); got != backToUpgrading {
			t.Errorf("a submission taken over left %s", got)
		}
		checkPages(t, tm.pool)
	})

	t.Run("sent back", func(t *testing.T) {
		r := tm.reviewed(t, "Sent back")
		lines := len((&mcpApp{logs: logs}).logLines(t))
		var status int
		var answer string
		if modes := tm.modesOf(t, r.id, eng, marker, func() { status, answer = tm.sendBackUpgrade(t, "dana", r.id) }); status != http.StatusOK ||
			modes != 1 {
			t.Fatalf("the send-back = %d %s, %d mode frames; want 200, one", status, answer, modes)
		}
		var a map[string]any
		decodeAnswer(t, answer, &a)
		if a["state"] != "upgrading" || a["id"] != r.upgrade || a["submitted_at"] != nil || a["submitted_by"] != nil || a["report"] != nil ||
			a["ending"] != nil || a["rollback_job_id"] != nil {
			t.Errorf("sent back %s, want it upgrading, no submission, no report", answer)
		}
		if got := tm.submission(t, r.id); got != backToUpgrading {
			t.Errorf("the send-back left %s", got)
		}
		info := upgradeLines(t, logs, lines, "upgrade sent back")
		if len(info) != 1 || !slices.Equal(lineKeysOf(info[0]), []string{"level", "msg", "notebook_id", "reported", "submission_id", "time",
			"upgrade_id", "user_id", "workspace_id"}) {
			t.Fatalf("logged %v, want one info line", info)
		}
		if l := info[0]; l["level"] != "INFO" || l["workspace_id"] != acme || l["notebook_id"] != r.id || l["upgrade_id"] != r.upgrade ||
			l["submission_id"] != r.submission || l["user_id"] != tm.userOf(t, "dana") || l["reported"] != true {
			t.Errorf("the info line %v", l)
		}
		version := queryStrings(t, tm.pool, "SELECT n.id::text || ':' || c.revision FROM nodes n JOIN page_contents c ON c.node_id = n.id "+
			"WHERE n.id = $1", r.note)[0]
		if w := tm.agent(t, "alice", r.id).use(t, "write", map[string]any{"path": "Note.md", "content": "---\ntype: note\nsummary: A note sent back.\n---\nA note.\n",
			"base_version": version}).result(t); w["isError"] == true {
			t.Fatalf("the initiator's agent's write after the send-back = %v", w)
		}
		tm.submitted(t, "erin", r.id)
		if again := tm.submissionID(t, r.id); again == r.submission || again == "" {
			t.Errorf("submitted again as %s, want a submission of its own", again)
		}
		if got := tm.submission(t, r.id); got != reviewedReported {
			t.Errorf("submitted again: %s", got)
		}
		checkPages(t, tm.pool)
	})

	t.Run("the rejection's codes", func(t *testing.T) {
		codes(t, "rejection", tm.rejectUpgrade)
		checkPages(t, tm.pool)
	})

	t.Run("rejected", func(t *testing.T) {
		r := tm.reviewed(t, "Rejected")
		lines := len((&mcpApp{logs: logs}).logLines(t))
		var job string
		if modes := tm.modesOf(t, r.id, eng, marker, func() {
			job = tm.rejected(t, "dana", r.id)
			if j := tm.endedBackground(t, "dana", job); j.State != "succeeded" {
				t.Errorf("the rollback = %+v, want it done", j)
			}
		}); modes != 2 {
			t.Errorf("the rejection and its rollback brought %d mode frames, want two", modes)
		}
		if got := tm.ended(t, r.id); got != "rolled_back rejected true true true -" {
			t.Errorf("the rejection left %s, want it rolled back, rejected, complete, its submission and report kept", got)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", r.agents); n != 0 {
			t.Errorf("AGENTS the start wrote is still alive after the rollback")
		}
		if c := tm.content(t, "alice", r.note).Content; c != cleanNote {
			t.Errorf("the note after the rollback = %q, want it as the upgrade started", c)
		}
		info := upgradeLines(t, logs, lines, "upgrade rejected")
		if len(info) != 1 || !slices.Equal(lineKeysOf(info[0]), []string{"job_id", "level", "msg", "notebook_id", "submission_id", "time",
			"upgrade_id", "user_id", "workspace_id"}) {
			t.Fatalf("logged %v, want one info line", info)
		}
		if l := info[0]; l["level"] != "INFO" || l["workspace_id"] != acme || l["notebook_id"] != r.id || l["upgrade_id"] != r.upgrade ||
			l["submission_id"] != r.submission || l["user_id"] != tm.userOf(t, "dana") || l["job_id"] != job {
			t.Errorf("the info line %v", l)
		}
		checkPages(t, tm.pool)

		// In review without a report, its submission still checked.
		nb := notebook(t, "Rejected unchecked")
		up := tm.upgraded(t, "alice", nb)
		tm.reviewing(t, nb, up.ID)
		job = tm.rejected(t, "dana", nb)
		if j := tm.endedBackground(t, "dana", job); j.State != "succeeded" {
			t.Errorf("the rollback = %+v, want it done", j)
		}
		if got := tm.ended(t, nb); got != "rolled_back rejected true true false -" {
			t.Errorf("the rejection left %s", got)
		}
		checkPages(t, tm.pool)
	})
}

// The review's interleavings (M11/P3 design 7.2), on the standard app,
// each order on a notebook in review of its own, its notebook's row held
// (interleaveOn): the approval and the rejection, the approval and the
// send-back, the approval and the notebook's deletion, the approval and a
// write of a source page waiting for the row. The first answers as alone;
// the second as after it; at the end the pages', links' and search's
// rules hold.
func TestUpgradeReviewInterleavings(t *testing.T) {
	tm := newAcmeTeam(t, "member", "").withReviewers(t)
	for _, order := range []string{"approval first", "rejection first"} {
		t.Run("9 the approval and the rejection, "+order, func(t *testing.T) {
			r := tm.reviewed(t, "Nine "+order)
			first, second := approveStep("dana", r.id), rejectStep("erin", r.id)
			if order == "rejection first" {
				first, second = second, first
			}
			a, b := tm.interleaveOn(t, notebookRow(r.id), first, second)
			wantAnswer(t, "the first", a, cellOK())
			wantAnswer(t, "the second", b, cell{http.StatusConflict, "wiki.upgrade_state"})
			if order == "approval first" {
				if mode := tm.notebookMode(t, r.id); mode != "llm_wiki "+r.raw+" false" {
					t.Errorf("the approval first left %s", mode)
				}
				if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", r.id); n != 0 {
					t.Errorf("the rejection after it started %d jobs", n)
				}
				return
			}
			var job rollbackAnswer
			decodeAnswer(t, a.body, &job)
			if j := tm.endedBackground(t, "erin", *job.RollbackJobID); j.State != "succeeded" {
				t.Errorf("the rollback = %+v, want it done", j)
			}
			if got, mode := tm.ended(t, r.id), tm.notebookMode(t, r.id); got != "rolled_back rejected true true true -" ||
				mode != "standard - false" {
				t.Errorf("the rejection first left %s, %s", got, mode)
			}
		})
	}
	for _, order := range []string{"approval first", "send-back first"} {
		t.Run("the approval and the send-back, "+order, func(t *testing.T) {
			r := tm.reviewed(t, "Back "+order)
			first, second := approveStep("dana", r.id), sendBackStep("erin", r.id)
			if order == "send-back first" {
				first, second = second, first
			}
			a, b := tm.interleaveOn(t, notebookRow(r.id), first, second)
			wantAnswer(t, "the first", a, cellOK())
			wantAnswer(t, "the second", b, cell{http.StatusConflict, "wiki.upgrade_state"})
			want := "llm_wiki " + r.raw + " false"
			if order == "send-back first" {
				want = "standard - true"
				if got := tm.submission(t, r.id); got != backToUpgrading {
					t.Errorf("the send-back first left %s", got)
				}
			}
			if mode := tm.notebookMode(t, r.id); mode != want {
				t.Errorf("%s left %s, want %s", order, mode, want)
			}
		})
	}
	for _, order := range []string{"approval first", "deletion first"} {
		t.Run("9 the approval and the notebook's deletion, "+order, func(t *testing.T) {
			r := tm.reviewed(t, "Deleted "+order)
			first, second := approveStep("dana", r.id), request("alice", http.MethodDelete, "/api/v0/notebooks/"+r.id, "")
			if order == "deletion first" {
				first, second = second, first
			}
			a, b := tm.interleaveOn(t, notebookRow(r.id), first, second)
			approval, deletion := a, b
			if order == "deletion first" {
				approval, deletion = b, a
			}
			wantAnswer(t, "the deletion", deletion, cell{status: http.StatusNoContent})
			if order == "approval first" {
				wantAnswer(t, "the approval", approval, cellOK())
			} else {
				wantAnswer(t, "the approval", approval, cell{http.StatusNotFound, "notebook.not_found"})
			}
			if got := queryStrings(t, tm.pool, `SELECT (n.deleted_at IS NOT NULL)::text || ' ' || (u.deleted_at IS NOT NULL)::text
				FROM notebooks n JOIN notebook_upgrades u ON u.notebook_id = n.id WHERE n.id = $1`, r.id); !slices.Equal(got,
				[]string{"true true"}) {
				t.Errorf("%s left %v, want the notebook and its upgrade deleted", order, got)
			}
		})
	}
	for _, order := range []string{"approval first", "write first"} {
		t.Run("the approval and a write waiting for the notebook, "+order, func(t *testing.T) {
			r := tm.reviewed(t, "Written "+order)
			tm.addNotebookMember(t, "alice", r.id, "bob", "editor")
			rev := tm.content(t, "bob", r.paper).Revision
			write := contentWrite("bob", r.paper, "Bob's paper.\n", rev, "")
			first, second := approveStep("dana", r.id), write
			if order == "write first" {
				first, second = second, first
			}
			a, b := tm.interleaveOn(t, notebookRow(r.id), first, second)
			approval, written := a, b
			if order == "write first" {
				approval, written = b, a
			}
			wantAnswer(t, "the approval", approval, cellOK())
			if order == "approval first" {
				wantAnswer(t, "the write after it", written, cell{http.StatusConflict, "page.source_immutable"})
			} else {
				wantAnswer(t, "the write before it", written, cell{http.StatusConflict, "notebook.upgrading"})
			}
			if c := tm.content(t, "bob", r.paper); c.Content != "The first paper.\n" || c.Revision != rev {
				t.Errorf("the source after %s = %+v, want it as it was", order, c)
			}
		})
	}
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkSearch(t, tm.pool)
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
}
