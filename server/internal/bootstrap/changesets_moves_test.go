package bootstrap

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
)

// The units mark their changesets' renames and moves, of all their items
// (M9/P3 design 3.20.2, 3.21.4): after a REST rename and a REST move, an
// agent's move and rename, a batch whose rename is its 1,002nd item by
// node id, past a thousand pages seeded smaller (their folder, then them)
// that it deletes, an import, and a background revert of a renaming
// batch, each changeset's renames and moves are the items' own: one of
// them alive before and after it under another name, or under another
// parent. Each is checked by what it does too. The background revert of
// a shape with a step aside comes in S3: S2's job refuses it.
func TestTheChangesetsRenamesCountEveryItem(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	agent := tm.agent(t, "alice", nb)
	ids := tm.seed(t, nb, restPage{"Renamed", "r"}, restPage{"Moved", "m"}, restPage{"Folder", ""}, restPage{"Agent", "a"},
		restPage{"Thousand", ""}, restPage{"Batched", "b"})
	want := map[string][2]bool{}
	newest := func(renames, moves bool) {
		t.Helper()
		want[tm.newest(t, nb).ID] = [2]bool{renames, moves}
	}

	tm.send(t, nodeRename("alice", ids["Renamed"], "Renamed 2"), http.StatusOK)
	newest(true, false)
	tm.send(t, nodeMove("alice", ids["Moved"], ids["Folder"]), http.StatusOK)
	newest(false, true)
	if a := agent.use(t, "move", map[string]any{"from": "Agent.md", "to": "Folder/Agent 2.md"}); a.result(t)["isError"] == true {
		t.Fatalf("the agent's move = %.300s", a.body)
	}
	newest(true, true)
	pages := make([]sqlPage, 1000)
	for i := range pages {
		pages[i] = sqlPage{ids["Thousand"], fmt.Sprint("p", i), "x"}
	}
	tm.seedSQL(t, nb, pages)
	newest(false, false)
	last := tm.createPage(t, "alice", nb, "", "Last") // after the thousand: a greater id
	b := agent.batchDone(t, opDeleteAll("Thousand.md"), opMove("Last.md", "Last 2.md"))
	if b.ChangesetID == nil || count(t, tm.pool, `SELECT count(*) FROM changeset_items WHERE changeset_id = $1 AND node_id::text < $2`,
		*b.ChangesetID, last) != 1001 {
		t.Fatalf("the batch = %+v, want a changeset whose rename comes after its 1,001 deletions", b)
	}
	newest(true, false)
	if j := tm.imported(t, "alice", nb, "", flatImport(t, 3)); j.State != "succeeded" {
		t.Fatalf("the import = %+v", j)
	}
	newest(false, false)
	renaming := agent.batchDone(t, opMove("Batched.md", "Batched 2.md"))
	newest(true, false)
	if j := tm.background(t, revertStart("alice", nb, *renaming.ChangesetID, "")); j.State != "succeeded" {
		t.Fatalf("the background revert = %s, want it succeeded", j)
	}
	newest(true, false)

	rows := queryStrings(t, tm.pool, `SELECT c.id || ' ' || c.renames || ' ' || c.moves || ' ' ||
			coalesce(bool_or(i.before_name <> i.after_name), false) || ' ' ||
			coalesce(bool_or(i.before_name IS NOT NULL AND i.after_name IS NOT NULL AND i.before_parent_id IS DISTINCT FROM i.after_parent_id), false)
		FROM changesets c LEFT JOIN changeset_items i ON i.changeset_id = c.id WHERE c.notebook_id = $1 GROUP BY c.id ORDER BY c.id`, nb)
	checked := 0
	for _, r := range rows {
		var id string
		var renames, moves, itemsRename, itemsMove bool
		if _, err := fmt.Sscan(r, &id, &renames, &moves, &itemsRename, &itemsMove); err != nil {
			t.Fatalf("%q: %v", r, err)
		}
		if renames != itemsRename || moves != itemsMove {
			t.Errorf("the changeset %s marks renames %v, moves %v; its items rename %v, move %v", id, renames, moves, itemsRename, itemsMove)
		}
		if w, ok := want[id]; ok {
			checked++
			if w != [2]bool{renames, moves} {
				t.Errorf("the changeset %s marks renames %v, moves %v; want %v", id, renames, moves, w)
			}
		}
	}
	if checked != len(want) || len(want) != 8 || !slices.Contains(rows, *b.ChangesetID+" true false true false") {
		t.Errorf("%d changesets checked by what each does, of %d; the batch's row missing from %d", checked, len(want), len(rows))
	}
}
