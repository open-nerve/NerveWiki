package bootstrap

import (
	"net/http"
	"testing"
)

// The interleavings of a revert in rounds (M9 design 4.15; M9/P3 design
// 3.21.5), harnessed as the revert's (interleavings_history_test.go).

// A revert whose schedule steps a node aside meets a rename of a node it
// puts back: the changeset an agent's batch that swapped A and B through
// a third name. The rename first: it commits, and the revert, planned
// again under the notebook's lock, finds the node changed, 409
// page.revert_conflict, writing nothing. The revert first, holding the
// notebook's row through its rounds: the rename waits, then renames the
// node from the name the revert put back, and no name aside is left. The
// pages', links' and search's invariants hold.
func TestTheMultiRoundRevertMeetsAWrite(t *testing.T) {
	orders(t, "revert", "rename", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := tm.seed(t, nb, restPage{"A", "a"}, restPage{"B", "b"}, restPage{"Linker", "[[Q]]"})
		b := tm.agent(t, "alice", nb).batchDone(t, opMove("A.md", "T.md"), opMove("B.md", "A.md"), opMove("T.md", "B.md"))
		if b.ChangesetID == nil {
			t.Fatalf("the batch = %+v, want a changeset", b)
		}
		was := tm.shapeState(t, nb)
		steps := map[string]step{"revert": changesetRevert("bob", *b.ChangesetID), "rename": nodeRename("alice", ids["A"], "Q")}
		x, y := tm.interleaveOn(t, notebookRow(nb), steps[first], steps[second])
		got := map[string]answer{first: x, second: y}
		revert, rename := got["revert"], got["rename"]
		switch first {
		case "rename":
			conflicted(t, "the revert after the rename", revert, ids["A"], "changed")
			if rename.status != http.StatusOK || tm.reverts(t, nb) != 0 || tm.placeOf(t, ids["A"]) != "/Q" || tm.placeOf(t, ids["B"]) != "/A" {
				t.Errorf("the rename = %d %s, A at %s, B at %s; want 200, no revert, A renamed Q from B, B at A", rename.status,
					rename.body, tm.placeOf(t, ids["A"]), tm.placeOf(t, ids["B"]))
			}
		case "revert":
			if revert.status != http.StatusOK || rename.status != http.StatusOK || count(t, tm.pool, `SELECT count(*) FROM changeset_items i
				JOIN changesets c ON c.id = i.changeset_id WHERE i.node_id = $1 AND c.kind = 'edit' AND i.before_name = 'A'
				AND i.after_name = 'Q'`, ids["A"]) != 1 || tm.placeOf(t, ids["B"]) != "/B" {
				t.Errorf("the revert = %d %s, the rename = %d %s; want both 200, the rename from A, B back at B", revert.status,
					revert.body, rename.status, rename.body)
			}
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name LIKE '% ~%'", nb); n != 0 {
			t.Errorf("%d nodes named aside, want none", n)
		}
		if first == "rename" && tm.shapeState(t, nb) == was {
			t.Errorf("the rename first changed nothing")
		}
		checkPages(t, tm.pool)
		checkLinks(t, tm.pool)
		checkSearch(t, tm.pool)
		tm.checkHistory(t)
	})
}
