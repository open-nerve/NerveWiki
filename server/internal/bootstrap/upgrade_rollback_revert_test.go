package bootstrap

import (
	"net/http"
	"testing"
)

// A rollback's changeset reverted afterwards puts the notebook back as the
// agent left it (M11 closeout, review A-M1; decision 302, which M12/P3/S2
// fixed: M12 design 4.6.1, decision 304). The rollback writes its passes
// in one changeset: the first deletes, in a unit of its own, the new
// subtrees that hold no baseline node; a later one deletes, at another
// time, a new node that holds the baseline's pages moved under it. A new
// page deleted in the first pass under a new folder deleted later is a
// trash item's root itself: the revert takes it for a top of its own, and
// restores the folder's item, then the page's under it.
//
// flat: the new folder Made holds only Guide, moved under it; the first
// pass deletes the new subtrees that hold no baseline node, Fresh and the
// raw and AGENTS the start wrote, none of them under Made. Reverted, in
// the request, the notebook is as the agent left it.
//
// nested: Made holds Guide, moved under it, and Inner, new, which the
// first pass deletes, Made a later one. Reverted in the request, and in
// the background in a notebook of its own, the notebook is as the agent
// left it, one revert changeset each.
func TestRevertingARollbackPutsBackWhatTheAgentLeft(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	// rolledBack is the notebook name, upgraded by alice, its agent's writes
	// made, then abandoned and rolled back: the notebook, its state before
	// the start, as the agent left it, and the rollback's changeset.
	rolledBack := func(t *testing.T, name string, writes ...agentWrite) (nb, before, tidied, changeset string) {
		t.Helper()
		nb = tm.openNotebook(t, "alice", name)
		tm.createPageWith(t, "alice", nb, "", "Guide", "guide [[Notes]]\n")
		tm.createPageWith(t, "alice", nb, "", "Notes", "notes\n")
		before = tm.notebookState(t, nb)
		tm.upgraded(t, "alice", nb)
		agent := tm.agent(t, "alice", nb)
		for _, w := range writes {
			if r := agent.use(t, w.tool, w.args).result(t); r["isError"] == true {
				t.Fatalf("the agent's %s %v = %v", w.tool, w.args, r)
			}
		}
		tidied = tm.notebookState(t, nb)
		if tidied == before {
			t.Fatalf("the agent's writes left %s as it was", name)
		}
		j := tm.endedBackground(t, "alice", tm.abandoned(t, "alice", nb))
		if j.State != "succeeded" || j.Kind != "rollback" || j.ChangesetID == nil {
			t.Fatalf("the rollback of %s = %s, want it succeeded, with its changeset", name, j)
		}
		if now := tm.notebookState(t, nb); now != before {
			t.Fatalf("%s rolled back:\n%s\nwant as before the start:\n%s", name, now, before)
		}
		return nb, before, tidied, *j.ChangesetID
	}
	reverts := func(t *testing.T, nb string) int {
		t.Helper()
		return count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1 AND kind = 'revert'", nb)
	}

	t.Run("flat", func(t *testing.T) {
		nb, _, tidied, changeset := rolledBack(t, "Flat",
			agentWrite{"write", map[string]any{"path": "Made.md", "content": "made\n"}},
			agentWrite{"move", map[string]any{"from": "Guide.md", "to": "Made/Guide.md"}},
			agentWrite{"write", map[string]any{"path": "Fresh.md", "content": "fresh\n"}},
			agentWrite{"edit", map[string]any{"path": "Notes.md", "old_string": "notes", "new_string": "---\ntype: n\n---\nnotes"}})
		c := changesetRevert("alice", changeset)
		status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], "")
		var r revertAnswer
		decodeAnswer(t, body, &r)
		if status != http.StatusOK || !r.Changed || r.ChangesetID == nil {
			t.Fatalf("the revert of the rollback's changeset = %d %s, want 200, changed", status, body)
		}
		if now := tm.notebookState(t, nb); now != tidied {
			t.Errorf("reverted, the notebook:\n%s\nwant as the agent left it:\n%s", now, tidied)
		}
		if n := reverts(t, nb); n != 1 {
			t.Errorf("%d revert changesets, want the one", n)
		}
	})

	nested := []agentWrite{
		{"write", map[string]any{"path": "Made/Inner.md", "content": "inner\n"}},
		{"move", map[string]any{"from": "Guide.md", "to": "Made/Guide.md"}},
	}
	t.Run("nested, request", func(t *testing.T) {
		nb, _, tidied, changeset := rolledBack(t, "Nested", nested...)
		c := changesetRevert("alice", changeset)
		status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], "")
		var r revertAnswer
		decodeAnswer(t, body, &r)
		if status != http.StatusOK || !r.Changed || r.ChangesetID == nil {
			t.Fatalf("the revert of the rollback's changeset = %d %s, want 200, changed", status, body)
		}
		if now := tm.notebookState(t, nb); now != tidied {
			t.Errorf("reverted, the notebook:\n%s\nwant as the agent left it:\n%s", now, tidied)
		}
		if n := reverts(t, nb); n != 1 {
			t.Errorf("%d revert changesets, want the one", n)
		}
	})

	t.Run("nested, background", func(t *testing.T) {
		nb, _, tidied, changeset := rolledBack(t, "Nested Background", nested...)
		j := tm.endedBackground(t, "alice", tm.startedJob(t, revertStart("alice", nb, changeset, "")).ID)
		if j.State != "succeeded" || j.Kind != "revert" || j.counts()["restored"] != 4 || j.counts()["put_back"] != 1 {
			t.Fatalf("the background revert of the rollback's changeset = %s, want it succeeded, 4 nodes restored, Guide put back", j)
		}
		if now := tm.notebookState(t, nb); now != tidied {
			t.Errorf("reverted, the notebook:\n%s\nwant as the agent left it:\n%s", now, tidied)
		}
		if n := reverts(t, nb); n != 1 {
			t.Errorf("%d revert changesets, want the one", n)
		}
	})
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkBackground(t, tm)
}
