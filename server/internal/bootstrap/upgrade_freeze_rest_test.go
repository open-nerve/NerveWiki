package bootstrap

import (
	"net/http"
	"testing"
)

// What the freeze leaves, what it answers first, and whom it lets by,
// through the whole program (M11/P1 design 3.4, 7.1).

// The freeze leaves all but the content (M11 design 4.5): in a notebook
// being upgraded by alice, upgrading, reviewing or rolling back, bob's
// export starts and succeeds; bob reads a page; a write of the content it
// holds is answered as done, writing nothing; alice renames the notebook,
// closes it to the workspace, makes carol a member, unlocks the page bob
// had opened before the upgrade, and deletes the notebook.
func TestUpgradeFreezeLeavesTheRest(t *testing.T) {
	tm := newAcmeTeam(t, "member", "member")
	for _, state := range []string{"upgrading", "reviewing", "rolling_back"} {
		t.Run(state, func(t *testing.T) {
			nb := tm.openNotebook(t, "alice", "Kept "+state)
			p := tm.createPageWith(t, "alice", nb, "", "Page", "kept\n")
			tm.openSession(t, "bob", p)
			tm.upgrade(t, nb, "alice", state)
			g := guardedNotebook{name: state, id: nb}
			status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/exports", tm.tokens["bob"], `{}`)
			if status != http.StatusAccepted {
				t.Fatalf("%s: bob's export = %d %s, want 202", state, status, answer)
			}
			var export backgroundJob
			decodeAnswer(t, answer, &export)
			if j := tm.endedBackground(t, "bob", export.ID); j.State != "succeeded" {
				t.Errorf("%s: bob's export = %s, want it succeeded", state, j)
			}
			tm.guarded(t, g, "bob reads Page", request("bob", http.MethodGet, "/api/v0/pages/"+p+"/content", ""), cellOK())
			before := tm.guardRows(t, nb)
			tm.guarded(t, g, "bob writes Page as it is", contentWrite("bob", p, "kept\n", 1, ""), cellOK())
			if after := tm.guardRows(t, nb); after != before {
				t.Errorf("%s: a write of the content as it is left the rows %s, want %s", state, after, before)
			}
			tm.guarded(t, g, "alice renames the notebook", request("alice", http.MethodPatch, "/api/v0/notebooks/"+nb, `{"name":"Renamed `+state+`"}`),
				cellOK())
			tm.guarded(t, g, "alice closes it to the workspace", request("alice", http.MethodPatch, "/api/v0/notebooks/"+nb,
				`{"workspace_access":"none"}`), cellOK())
			tm.addNotebookMember(t, "alice", nb, "carol", "editor")
			tm.guarded(t, g, "alice unlocks Page", unlock("alice", p), cell{status: http.StatusNoContent})
			tm.guarded(t, g, "alice deletes the notebook", request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""),
				cell{status: http.StatusNoContent})
		})
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
	checkBackground(t, tm)
}

// The freeze comes first of the guards and of the vetoers (M11 design
// 4.5): with bob's edit session opened on Held before the upgrade, which
// its start would refuse, set by SQL, alice's deletion of Held, which the
// lock refuses too, is notebook.upgrading, and so is her opening of a
// session on it, which the lock would refuse too, bob's session not hers
// to take over; once the upgrade ended,
// the deletion meets the lock, page.locked naming bob's. The composition
// root registering the freeze after the lock fails the first.
func TestUpgradeFreezeAnswersFirst(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Notes")
	held := tm.createPage(t, "alice", nb, "", "Held")
	tm.openSession(t, "bob", held)
	tm.upgrade(t, nb, "alice", "upgrading")
	g := guardedNotebook{name: "the notebook being upgraded", id: nb}
	deletion := nodeDeletion("alice", held)
	tm.guarded(t, g, "alice deletes Held, which bob edits", deletion, cell{http.StatusConflict, "notebook.upgrading"})
	tm.guarded(t, g, "alice takes Held over", sessionTakeOver("alice", held), cell{http.StatusConflict, "notebook.upgrading"})
	tm.endUpgrade(t, nb)
	g.name = "the notebook once its upgrade ended"
	body := tm.guarded(t, g, "alice deletes Held, which bob edits", deletion, cell{http.StatusConflict, "page.locked"})
	if lockHolder(answer{body: body}) != "bob" {
		t.Errorf("the deletion once the upgrade ended = %s, want bob's lock", body)
	}
	checkPages(t, tm.pool)
}

// The freeze asks the initiator's role each time (M11 design 4.5): bob,
// made an admin of alice's notebook, upgrades it; his agent writes; made
// an editor, its write is notebook.upgrading, writing nothing; an admin
// again, it writes.
func TestUpgradeFreezeFollowsTheInitiatorsRole(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Notes")
	tm.createPage(t, "alice", nb, "", "raw")
	bob := tm.addNotebookMember(t, "alice", nb, "bob", "admin")
	tm.upgrade(t, nb, "bob", "upgrading")
	agent := tm.agent(t, "bob", nb)
	write := func(name string) mcpAnswer {
		return agent.use(t, "write", map[string]any{"path": name + ".md", "content": "x"})
	}
	role := func(r string) {
		tm.send(t, request("alice", http.MethodPatch, "/api/v0/notebook-members/"+bob, `{"role":"`+r+`"}`), http.StatusOK)
	}
	if r := write("Admin").result(t); r["isError"] == true {
		t.Errorf("the admin's agent's write = %v, want it done", r)
	}
	role("editor")
	before := tm.guardRows(t, nb)
	if member := write("Editor").toolError(t); member["code"] != "notebook.upgrading" {
		t.Errorf("the editor's agent's write = %v, want notebook.upgrading", member)
	}
	if after := tm.guardRows(t, nb); after != before {
		t.Errorf("the editor's agent's write, refused, left the rows %s, want %s", after, before)
	}
	role("admin")
	if r := write("Admin again").result(t); r["isError"] == true {
		t.Errorf("the admin's agent's write, an admin again = %v, want it done", r)
	}
	checkPages(t, tm.pool)
}

// The freeze lets by no write for its kind (M11 design 4.5, decision
// 177): in a notebook being upgraded, upgrading or rolling back, bob's
// revert from the web of an old changeset of his, and alice's, the
// initiator's, are notebook.upgrading, as is the revert's start in the
// background; so is alice's revert from the web of a write her agent made
// for the upgrade.
func TestUpgradeFreezeLetsOnlyItsOwnWrites(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	for _, state := range []string{"upgrading", "rolling_back"} {
		t.Run(state, func(t *testing.T) {
			nb := tm.openNotebook(t, "alice", "Own "+state)
			p := tm.createPage(t, "bob", nb, "", "Page")
			tm.send(t, nodeRename("bob", p, "Renamed"), http.StatusOK)
			old := tm.newest(t, nb).ID
			tm.upgrade(t, nb, "alice", "upgrading")
			g := guardedNotebook{name: state, id: nb}
			if r := tm.agent(t, "alice", nb).use(t, "write", map[string]any{"path": "Agent.md", "content": "x"}).result(t); r["isError"] == true {
				t.Fatalf("alice's agent's write = %v, want it done", r)
			}
			own := tm.newest(t, nb).ID
			if state != "upgrading" {
				tm.exec(t, "UPDATE notebooks SET upgrade_state = $2 WHERE id = $1", nb, state)
				tm.exec(t, `UPDATE notebook_upgrades SET state = $2, ending = 'abandoned', rollback_job_id = gen_random_uuid()
					WHERE id = (SELECT upgrade_id FROM notebooks WHERE id = $1)`, nb, state)
			}
			refused := cell{http.StatusConflict, "notebook.upgrading"}
			tm.guarded(t, g, "bob reverts his rename", changesetRevert("bob", old), refused)
			tm.guarded(t, g, "alice reverts bob's rename", changesetRevert("alice", old), refused)
			tm.guarded(t, g, "bob starts a revert of his rename", revertStart("bob", nb, old, ""), refused)
			tm.guarded(t, g, "alice reverts her agent's write", changesetRevert("alice", own), refused)
		})
	}
	checkPages(t, tm.pool)
}
