package bootstrap

import (
	"net/http"
	"testing"
)

// Decision 181 through the whole program (M11/P3 design 3.10, 7.1): a
// notebook whose index and log at its root were deleted, each its own
// changeset, and whose page index.md at its root its upgrade's agent
// renamed, approved. A revert that would bring a node back to the root
// under a reserved title is page.virtual_file, nothing written: the
// deletion's revert, in the request and in the background, and the
// rename's. A restore from the trash numbers the title instead, in the
// request and in the background. At the end no node at the root holds a
// reserved key.
func TestUpgradeTrashReservedTitles(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	tm.join(t, "dana", "member")
	nb := tm.openNotebook(t, "alice", "Reserved")
	tm.addNotebookMember(t, "alice", nb, "dana", "admin")
	// latest is the notebook's last changeset.
	latest := func(t *testing.T) string {
		t.Helper()
		return queryStrings(t, tm.pool, "SELECT id::text FROM changesets WHERE notebook_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1",
			nb)[0]
	}
	index := tm.createPage(t, "alice", nb, "", "index")
	tm.send(t, nodeDeletion("alice", index), http.StatusNoContent)
	indexDeleted := latest(t)
	log := tm.createPage(t, "alice", nb, "", "log")
	tm.send(t, nodeDeletion("alice", log), http.StatusNoContent)
	tm.createPageWith(t, "alice", nb, "", "Note", cleanNote)
	named := tm.createPageWith(t, "alice", nb, "", "index.md", cleanNote)

	tm.upgraded(t, "alice", nb)
	if r := tm.agent(t, "alice", nb).use(t, "move", map[string]any{"from": "index.md.md", "to": "Index notes.md"}).result(t); r["isError"] == true {
		t.Fatalf("the agent's rename of index.md = %v", r)
	}
	renamed := latest(t)
	tm.submitted(t, "alice", nb)
	if status, answer := tm.approveUpgrade(t, "dana", nb); status != http.StatusOK {
		t.Fatalf("the approval = %d %s", status, answer)
	}

	changesets := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", nb)
	jobs := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", nb)
	unchanged := func(t *testing.T, what string) {
		t.Helper()
		if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", nb); n != changesets {
			t.Errorf("%s wrote %d changesets", what, n-changesets)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", nb); n != jobs {
			t.Errorf("%s queued %d jobs", what, n-jobs)
		}
	}
	// nameOf is the node id's name, its parent's id ("-" for the root)
	// and whether it is in the trash.
	nameOf := func(t *testing.T, id string) string {
		t.Helper()
		return queryStrings(t, tm.pool, `SELECT name || ' ' || coalesce(parent_id::text, '-') || ' ' || (deleted_at IS NOT NULL)::text
			FROM nodes WHERE id = $1`, id)[0]
	}

	c := changesetRevert("alice", indexDeleted)
	status, answer := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], "")
	wantCode(t, "the revert of index's deletion", status, answer, cell{http.StatusConflict, "page.virtual_file"})
	unchanged(t, "the revert of index's deletion")
	if got := nameOf(t, index); got != "index - true" {
		t.Errorf("the refused revert left index %s, want it in the trash", got)
	}
	c = revertStart("alice", nb, indexDeleted, "")
	status, answer = ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], c.body)
	wantCode(t, "the background revert of index's deletion", status, answer, cell{http.StatusConflict, "page.virtual_file"})
	unchanged(t, "the background revert of index's deletion")

	c = changesetRevert("alice", renamed)
	status, answer = ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], "")
	wantCode(t, "the revert of the agent's rename", status, answer, cell{http.StatusConflict, "page.virtual_file"})
	unchanged(t, "the revert of the agent's rename")
	if got := nameOf(t, named); got != "Index notes - false" {
		t.Errorf("the refused revert left index.md %s, want it Index notes", got)
	}

	if r := tm.restored(t, "alice", index); r.Name != "index 2" || r.Placed.ParentID != nil {
		t.Errorf("index restored = %+v, want index 2 at the root", r)
	}
	j := tm.startedJob(t, trashRestoreStart("alice", nb, log))
	if j = tm.endedBackground(t, "alice", j.ID); j.State != "succeeded" {
		t.Errorf("the background restore of log = %s, want it done", j)
	}
	if got := nameOf(t, log); got != "log 2 - false" {
		t.Errorf("log restored in the background is %s, want log 2 at the root", got)
	}
	if got := queryStrings(t, tm.pool, `SELECT name FROM nodes WHERE notebook_id = $1 AND parent_id IS NULL AND deleted_at IS NULL
		AND name_key IN ('index', 'log', 'index.md', 'log.md')`, nb); len(got) != 0 {
		t.Errorf("reserved titles at the root: %v", got)
	}
	checkPages(t, tm.pool)
}
