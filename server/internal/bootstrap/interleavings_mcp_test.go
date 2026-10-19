package bootstrap

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// The interleavings of the MCP writes (M9/P2 design 3.13.4), harnessed as
// REST's (interleavings_workspace_test.go): the test holds a row both
// steps need, sends each once the one before waits for it, and lets them
// run in the order they came. An agent's call is a step's command, its
// request sent over HTTP to /mcp in process. Each ends by checking the
// pages' invariant, the links' and the search's.

// mcpStep is m's call of tool with args in its notebook as an
// interleaving's step of by's: a command, its request built here and sent
// on the step's goroutine, its answer in got once it ran.
func (m *mcpApp) mcpStep(t *testing.T, by, tool string, args map[string]any, got *mcpAnswer) step {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	req := newRequest(t, http.MethodPost, m.base+atNotebook(m.notebook), m.pat, body)
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", mcpVersion)
	return step{by: by + "'s " + tool, command: func() error {
		res, err := client().Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = res.Body.Close() }()
		b, err := io.ReadAll(res.Body)
		*got = mcpAnswer{status: res.StatusCode, header: res.Header, body: b, tool: tool}
		return err
	}}
}

// ran fails t unless the command of a step ran without an error.
func ran(t *testing.T, a answer) {
	t.Helper()
	if a.err != nil {
		t.Fatalf("the agent's call failed: %v", a.err)
	}
}

// checkInterleaved checks the pages', the links' and the search's
// invariants once an interleaving ended.
func checkInterleaved(t *testing.T, tm acmeTeam) {
	t.Helper()
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkSearch(t, tm.pool)
}

// bothOrders runs test in each order, by the name of what goes first.
func bothOrders(t *testing.T, first, second string, test func(t *testing.T, firstFirst bool)) {
	t.Helper()
	for _, firstFirst := range []bool{true, false} {
		name := map[bool]string{true: first + " first", false: second + " first"}[firstFirst]
		t.Run(name, func(t *testing.T) { test(t, firstFirst) })
	}
}

// alice's write of Notes on its version meets bob's opening at the page's
// gate, its content row. The opening first: the write is page.locked by
// bob, Notes as it was, his session alive. The write first: it writes,
// and the opening follows (interleaving 51's).
func TestAnMCPWriteMeetsAnOpening(t *testing.T) {
	bothOrders(t, "the write", "the opening", func(t *testing.T, writeFirst bool) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		notes := tm.createPageWith(t, "alice", nb, "", "Notes", "Draft\n")
		var got mcpAnswer
		write := tm.agent(t, "alice", nb).mcpStep(t, "alice", "write",
			map[string]any{"path": "Notes.md", "content": "Mine\n", "base_version": versionOf(notes, 1)}, &got)
		if writeFirst {
			w, opened := tm.interleaveOn(t, contentRow(notes), write, sessionOpening("bob", notes))
			ran(t, w)
			if written := answerOf[mcpWritten](t, got); written.Version != versionOf(notes, 2) || !opened.is(http.StatusCreated, "") {
				t.Errorf("the write = %+v, then the opening = %d %s; want version 2, then 201", written, opened.status, opened.body)
			}
		} else {
			opened, w := tm.interleaveOn(t, contentRow(notes), sessionOpening("bob", notes), write)
			ran(t, w)
			r := refusalOf(t, got)
			if !opened.is(http.StatusCreated, "") || r.Code != "page.locked" || r.Lock == nil || r.Lock.DisplayName != "bob" ||
				tm.content(t, "alice", notes).Revision != 1 || tm.sessionsOf(t, notes) != 1 {
				t.Errorf("the opening = %d %s, then the write = %+v; want 201, then page.locked by bob, Notes at 1, his session",
					opened.status, opened.body, r)
			}
		}
		checkInterleaved(t, tm)
	})
}

// alice's edit reads Notes and passes its precheck, no session open, then
// waits at the page's gate behind bob's opening: the opening takes it
// first, and the edit's unit is page.locked by bob, the lock taken between
// its read and its write, with Notes' path; Notes as it was, his session
// alive.
func TestAnMCPEditMeetsAnOpeningAfterItsRead(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	notes := tm.createPageWith(t, "alice", nb, "", "Notes", "Draft\n")
	var got mcpAnswer
	edit := tm.agent(t, "alice", nb).mcpStep(t, "alice", "edit", map[string]any{"path": "Notes.md", "old_string": "Draft", "new_string": "Mine"}, &got)
	opened, e := tm.interleaveOn(t, contentRow(notes), sessionOpening("bob", notes), edit)
	ran(t, e)
	r := refusalOf(t, got)
	if !opened.is(http.StatusCreated, "") || r.Code != "page.locked" || r.Lock == nil || lockOf(*r.Lock) != notes+" Notes.md bob false" ||
		tm.content(t, "alice", notes).Revision != 1 || tm.sessionsOf(t, notes) != 1 {
		t.Errorf("the opening = %d %s, then the edit = %+v; want 201, then page.locked by bob at Notes.md, Notes at 1, his session",
			opened.status, opened.body, r)
	}
	checkInterleaved(t, tm)
}

// alice's edit reads Notes at r and waits at its gate behind a REST write,
// which writes r + 1: the edit's write on r is refused in its unit, and it
// reads again and edits once more, at r + 2, the REST write's content
// edited, its line saying it was redone. With base_version the same meeting
// is page.revision_mismatch at the version the REST write left, and the
// edit writes nothing.
func TestAnMCPEditRedoesAfterAConcurrentWrite(t *testing.T) {
	logs := &syncBuffer{}
	tm := newAcmeTeamLogging(t, "member", "", nil, transfer.Batch{},
		slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	nb := tm.openNotebook(t, "alice", "Eng")
	notes := tm.createPageWith(t, "alice", nb, "", "Notes", "alpha\n")
	alice := tm.agent(t, "alice", nb)

	var got mcpAnswer
	written, e := tm.interleaveOn(t, contentRow(notes), contentWrite("bob", notes, "alpha beta\n", 1, ""),
		alice.mcpStep(t, "alice", "edit", map[string]any{"path": "Notes.md", "old_string": "alpha", "new_string": "ALPHA"}, &got))
	ran(t, e)
	edited := answerOf[mcpEdited](t, got)
	line := (&mcpApp{logs: logs}).logged(t, "page edited")
	if !written.is(http.StatusOK, "") || edited.Version != versionOf(notes, 3) || tm.content(t, "alice", notes).Content != "ALPHA beta\n" ||
		line == nil || line["redone"] != true {
		t.Errorf("the REST write = %d %s, then the edit = %+v, Notes %q, its line %v; want 200, then version 3 of the REST write edited, redone",
			written.status, written.body, edited, tm.content(t, "alice", notes).Content, line)
	}

	written, e = tm.interleaveOn(t, contentRow(notes), contentWrite("bob", notes, "ALPHA beta gamma\n", 3, ""),
		alice.mcpStep(t, "alice", "edit", map[string]any{"path": "Notes.md", "old_string": "beta", "new_string": "BETA",
			"base_version": versionOf(notes, 3)}, &got))
	ran(t, e)
	r := refusalOf(t, got)
	if !written.is(http.StatusOK, "") || r.Code != "page.revision_mismatch" || r.CurrentVersion != versionOf(notes, 4) ||
		tm.content(t, "alice", notes).Content != "ALPHA beta gamma\n" {
		t.Errorf("the REST write = %d %s, then the edit on version 3 = %+v; want 200, then page.revision_mismatch at version 4",
			written.status, written.body, r)
	}
	checkInterleaved(t, tm)
}

// bob's agent deletes Notes while bob opens its session, both waiting for
// the notebook's row. The opening first: the deletion is page.locked by
// him, told as his, his session alive (REST's deletion would end it). The
// deletion first: Notes is in the trash, and the opening is 404
// page.not_found (interleaving 50's).
func TestAnMCPDeleteMeetsItsCallersOpening(t *testing.T) {
	bothOrders(t, "the deletion", "the opening", func(t *testing.T, deletionFirst bool) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		notes := tm.createPage(t, "alice", nb, "", "Notes")
		var got mcpAnswer
		deletion := tm.agent(t, "bob", nb).mcpStep(t, "bob", "delete", map[string]any{"path": "Notes.md"}, &got)
		if deletionFirst {
			d, opened := tm.interleaveOn(t, notebookRow(nb), deletion, sessionOpening("bob", notes))
			ran(t, d)
			if gone := answerOf[mcpDeleted](t, got); gone.NodeID != notes || !opened.is(http.StatusNotFound, "page.not_found") ||
				tm.sessionsOf(t, notes) != 0 {
				t.Errorf("the deletion = %+v, then the opening = %d %s; want Notes in the trash, then 404 page.not_found", gone,
					opened.status, opened.body)
			}
		} else {
			opened, d := tm.interleaveOn(t, notebookRow(nb), sessionOpening("bob", notes), deletion)
			ran(t, d)
			r := refusalOf(t, got)
			if !opened.is(http.StatusCreated, "") || r.Code != "page.locked" || len(r.Locks) != 1 || !r.Locks[0].You ||
				tm.sessionsOf(t, notes) != 1 {
				t.Errorf("the opening = %d %s, then the deletion = %+v; want 201, then page.locked by bob, his own, his session alive",
					opened.status, opened.body, r)
			}
		}
		checkInterleaved(t, tm)
	})
}

// alice's agent writes a new Plan.md while bob creates Plan, both waiting
// for the notebook's row: the later is refused, the agent's saying Plan is
// a page already, REST's 409 page.title_taken, and one Plan is alive.
func TestAnMCPCreateMeetsACreationOfItsName(t *testing.T) {
	bothOrders(t, "the agent's", "REST's", func(t *testing.T, agentFirst bool) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		var got mcpAnswer
		write := tm.agent(t, "alice", nb).mcpStep(t, "alice", "write", map[string]any{"path": "Plan.md", "content": "agent's\n"}, &got)
		if agentFirst {
			w, created := tm.interleaveOn(t, notebookRow(nb), write, pageCreation("bob", nb, "Plan"))
			ran(t, w)
			if written := answerOf[mcpWritten](t, got); !written.Changed || !created.is(http.StatusConflict, "page.title_taken") {
				t.Errorf("the agent's write = %+v, then REST's creation = %d %s; want it created, then 409 page.title_taken", written,
					created.status, created.body)
			}
		} else {
			created, w := tm.interleaveOn(t, notebookRow(nb), pageCreation("bob", nb, "Plan"), write)
			ran(t, w)
			if r := refusalOf(t, got); !created.is(http.StatusCreated, "") || r.Code != "page.title_taken" ||
				!strings.Contains(r.Message, `"Plan.md" is a page already`) {
				t.Errorf("REST's creation = %d %s, then the agent's write = %+v; want 201, then page.title_taken: a page already",
					created.status, created.body, r)
			}
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name = 'Plan' AND deleted_at IS NULL", nb); n != 1 {
			t.Errorf("%d pages named Plan, want one", n)
		}
		checkInterleaved(t, tm)
	})
}

// alice's agent deletes Notes without recursive, which has no children
// when it looks, while bob creates a page under it, both waiting for the
// notebook's row. The creation first: the deletion's unit counts the new
// child, 422 of recursive, and deletes nothing; the deletion first: Notes
// is in the trash, and the creation under it is 422 validation_failed.
// Counted outside the unit, the first order would put bob's page in the
// trash with Notes (M9/P2 design 2.1, item 13).
func TestAnMCPDeleteSeesAChildAddedBeforeItsUnit(t *testing.T) {
	bothOrders(t, "the deletion", "the creation", func(t *testing.T, deletionFirst bool) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		notes := tm.createPage(t, "alice", nb, "", "Notes")
		var got mcpAnswer
		deletion := tm.agent(t, "alice", nb).mcpStep(t, "alice", "delete", map[string]any{"path": "Notes.md"}, &got)
		creation := childCreation("bob", nb, notes, "Late")
		alive := func() int {
			return count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", nb)
		}
		if deletionFirst {
			d, created := tm.interleaveOn(t, notebookRow(nb), deletion, creation)
			ran(t, d)
			if gone := answerOf[mcpDeleted](t, got); gone.Pages != 1 || !created.is(http.StatusUnprocessableEntity, "validation_failed") ||
				alive() != 0 {
				t.Errorf("the deletion = %+v, then the creation = %d %s, %d nodes alive; want Notes alone in the trash, then 422", gone,
					created.status, created.body, alive())
			}
		} else {
			created, d := tm.interleaveOn(t, notebookRow(nb), creation, deletion)
			ran(t, d)
			if r := refusalOf(t, got); !created.is(http.StatusCreated, "") || r.Code != "validation_failed" ||
				!strings.Contains(r.Message, `"Notes.md" has 1 page under it`) || alive() != 2 {
				t.Errorf("the creation = %d %s, then the deletion = %+v, %d nodes alive; want 201, then 422 counting the page, nothing deleted",
					created.status, created.body, r, alive())
			}
		}
		checkInterleaved(t, tm)
	})
}

// alice's agent writes Notes.md on Notes' version while bob renames it
// Old, both waiting for the notebook's row. The rename first: the write,
// finding the path again under the lock, finds nothing, page.not_found,
// and Old keeps its content; the write first: it writes, then the rename.
func TestAnMCPWriteFindsItsPathUnderTheLock(t *testing.T) {
	bothOrders(t, "the write", "the rename", func(t *testing.T, writeFirst bool) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		notes := tm.createPageWith(t, "alice", nb, "", "Notes", "Draft\n")
		var got mcpAnswer
		write := tm.agent(t, "alice", nb).mcpStep(t, "alice", "write",
			map[string]any{"path": "Notes.md", "content": "Mine\n", "base_version": versionOf(notes, 1)}, &got)
		rename := nodeRename("bob", notes, "Old")
		if writeFirst {
			w, renamed := tm.interleaveOn(t, notebookRow(nb), write, rename)
			ran(t, w)
			if written := answerOf[mcpWritten](t, got); written.Version != versionOf(notes, 2) || !renamed.is(http.StatusOK, "") {
				t.Errorf("the write = %+v, then the rename = %d %s; want version 2, then 200", written, renamed.status, renamed.body)
			}
		} else {
			renamed, w := tm.interleaveOn(t, notebookRow(nb), rename, write)
			ran(t, w)
			if r := refusalOf(t, got); !renamed.is(http.StatusOK, "") || r.Code != "page.not_found" ||
				tm.content(t, "alice", notes).Content != "Draft\n" {
				t.Errorf("the rename = %d %s, then the write = %+v; want 200, then page.not_found, Old as it was", renamed.status,
					renamed.body, r)
			}
		}
		checkInterleaved(t, tm)
	})
}
