package bootstrap

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// A rollback's changeset reverted afterwards, as it stands in v0.1 (M11
// closeout, review A-M1; decision 302). The rollback writes its passes in
// one changeset: the first deletes, in a unit of its own, the new subtrees
// that hold no baseline node; a later one deletes, at another time, a new
// node that holds the baseline's pages moved under it. M8's revert plan
// finds the tops of a changeset's entries by their parents: a new page
// deleted in the first pass under a new folder deleted later has that
// folder for its top, which the trash's group of the folder does not hold.
//
// flat: the new folder Made holds only Guide, moved under it; the first
// pass deletes Fresh, at the root, alone. Reverted, in the request, the
// notebook is as the agent left it.
//
// nested: Made holds Guide, moved under it, and Inner, new. The revert, in
// the request and in the background, answers 409 page.revert_conflict of
// Made and Inner, changed, and writes nothing: the notebook is as before
// the upgrade's start. This cell pins a known gap: once it is fixed (M12's
// handoff), the revert answers 200 here too, and this cell turns over.
func TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages(t *testing.T) {
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

	t.Run("nested", func(t *testing.T) {
		nb, before, _, changeset := rolledBack(t, "Nested",
			agentWrite{"write", map[string]any{"path": "Made/Inner.md", "content": "inner\n"}},
			agentWrite{"move", map[string]any{"from": "Guide.md", "to": "Made/Guide.md"}})
		var want []revertConflict
		for _, name := range []string{"Inner", "Made"} {
			ids := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = $2 AND deleted_at IS NOT NULL", nb, name)
			if len(ids) != 1 {
				t.Fatalf("%s in the trash = %q, want one", name, ids)
			}
			want = append(want, revertConflict{NodeID: ids[0], Name: name, Reason: "changed"})
		}
		for _, s := range []step{changesetRevert("alice", changeset), revertStart("alice", nb, changeset, "")} {
			status, body := ask(t, tm.contract, s.method, tm.base+s.path, tm.tokens["alice"], s.body)
			conflicts, n := conflictsOf(t, answer{body: body})
			slices.SortFunc(conflicts, func(a, b revertConflict) int { return strings.Compare(a.Name, b.Name) })
			if status != http.StatusConflict || problemCode(t, body) != "page.revert_conflict" || n != 2 ||
				!reflect.DeepEqual(conflicts, want) {
				t.Errorf("%s %s = %d %s, want 409 page.revert_conflict of %+v", s.method, s.path, status, body, want)
			}
			if now := tm.notebookState(t, nb); now != before {
				t.Errorf("after %s %s, the notebook:\n%s\nwant as before the start:\n%s", s.method, s.path, now, before)
			}
		}
		if n := reverts(t, nb); n != 0 {
			t.Errorf("%d revert changesets, want none", n)
		}
	})
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkBackground(t, tm)
}
