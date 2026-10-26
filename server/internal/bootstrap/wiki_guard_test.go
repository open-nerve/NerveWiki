package bootstrap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// The mode guard through the whole program (M10/P1 design 3.20, 3.24):
// serve registers it before the edit lock, and each REST path that can
// break an LLM Wiki's contract reaches it.

// guardedNotebook is a notebook of acme's open to its members as editors,
// alice its admin: an LLM Wiki, raw and AGENTS its template's, or a
// standard notebook whose root holds pages of the same names and
// contents.
type guardedNotebook struct {
	name, id, raw, agents string
	wiki                  bool
	template              string // the LLM Wiki's template's changeset
}

// llmWiki is an LLM Wiki of acme named name, open to acme's members as
// editors.
func (tm acmeTeam) llmWiki(t *testing.T, name string) guardedNotebook {
	t.Helper()
	n := tm.createWith(t, tm.tokens["alice"], `{"name":"`+name+`","mode":"llm_wiki","workspace_access":"editor"}`)
	root := tm.rootOf(t, n.ID)
	if len(root) != 2 || root[0].Name != "raw" || root[1].Name != "AGENTS" || n.SourceRootID == nil || *n.SourceRootID != root[0].ID {
		t.Fatalf("the LLM Wiki's root = %+v, want raw, its source root, then AGENTS", root)
	}
	return guardedNotebook{name: "an LLM Wiki", id: n.ID, raw: root[0].ID, agents: root[1].ID, wiki: true,
		template: tm.newest(t, n.ID).ID}
}

// standardTwin is a standard notebook of acme named name, open to acme's
// members as editors, whose root holds raw and AGENTS with the contents
// of wiki's.
func (tm acmeTeam) standardTwin(t *testing.T, name string, wiki guardedNotebook) guardedNotebook {
	t.Helper()
	n := tm.createWith(t, tm.tokens["alice"], `{"name":"`+name+`","workspace_access":"editor"}`)
	contents := map[string]string{}
	for _, r := range tm.rootOf(t, wiki.id) {
		contents[r.Name] = r.Content
	}
	return guardedNotebook{name: "a standard notebook", id: n.ID,
		raw:    tm.createPageWith(t, "alice", n.ID, "", "raw", contents["raw"]),
		agents: tm.createPageWith(t, "alice", n.ID, "", "AGENTS", contents["AGENTS"])}
}

// guardRows are the notebook id's nodes, changesets, their items and
// pages' versions, counted: what a refused write leaves as it was.
func (tm acmeTeam) guardRows(t *testing.T, id string) string {
	t.Helper()
	return fmt.Sprint(count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1", id),
		count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", id),
		count(t, tm.pool, "SELECT count(*) FROM changeset_items i JOIN changesets c ON c.id = i.changeset_id WHERE c.notebook_id = $1", id),
		count(t, tm.pool, "SELECT count(*) FROM page_revisions r JOIN nodes n ON n.id = r.node_id WHERE n.notebook_id = $1", id))
}

// guarded sends c and fails t unless it answers want: a refusal, any
// answer of 400 or more, leaves the notebook nb's rows as they were.
func (tm acmeTeam) guarded(t *testing.T, nb guardedNotebook, what string, c step, want cell) string {
	t.Helper()
	before := tm.guardRows(t, nb.id)
	status, body := askTyped(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.contentType, c.body)
	var p struct {
		Code string `json:"code"`
	}
	if status >= http.StatusBadRequest {
		_ = json.Unmarshal([]byte(body), &p)
	}
	if got := (cell{status, p.Code}); got != want {
		t.Errorf("%s: %s, in %s = %s %.300s; want %s", nb.name, what, nb.name, got, body, want)
	}
	if after := tm.guardRows(t, nb.id); status >= http.StatusBadRequest && after != before {
		t.Errorf("%s: %s, refused, left the rows %s, want them as they were, %s", nb.name, what, after, before)
	}
	return body
}

// The mode guard reaches every REST path that writes a tree (M10/P1
// design 3.24): an LLM Wiki, alice its admin and bob a writer by its
// access, and a standard notebook alike, sent the same requests. In the
// LLM Wiki: the template's revert would delete AGENTS and raw; a writer's
// revert of alice's rename of raw puts the source root back, his; alice's
// does; a page at the root as index or index.md, not under Notes; AGENTS's
// rename, reorder and move; bob's rename and move of raw, alice's move of
// it under Notes; bob's reorder and rename of Notes, raw under it, but not
// its move under another page; an attachment at the root as LOG, its key
// log, but log.png; AGENTS's and raw's contents, a task, a version and an
// edit session; and last the deletion of AGENTS, raw and Notes, which
// holds raw, alice's too. Each refusal leaves the rows as they were. In
// the standard notebook every request succeeds, the guard letting it by:
// no template's revert there, nor alice's of her rename, which bob's
// reverted already. Each reserved key is used once, so that the standard
// notebook's root takes each. Without the guard registered, the LLM
// Wiki's refusals succeed.
func TestTheModeGuardReachesEveryPath(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Wiki")
	for _, nb := range []guardedNotebook{wiki, tm.standardTwin(t, "Plain", wiki)} {
		t.Run(nb.name, func(t *testing.T) { tm.guardPaths(t, nb) })
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// guardPaths sends TestTheModeGuardReachesEveryPath's requests in nb.
func (tm acmeTeam) guardPaths(t *testing.T, nb guardedNotebook) {
	ok, created, gone := cellOK(), cellCreated(), cell{status: http.StatusNoContent}
	// refused is a refusal in the LLM Wiki, answer in the standard notebook.
	refused := func(code string, answer cell) cell {
		if !nb.wiki {
			return answer
		}
		status := http.StatusConflict
		if code == "page.source_admin_only" {
			status = http.StatusForbidden
		}
		return cell{status, code}
	}
	notes := tm.createPage(t, "alice", nb.id, "", "Notes")
	other := tm.createPage(t, "alice", nb.id, "", "Other")
	rename, moveUnder, deletion := nodeRename, nodeMove, nodeDeletion
	// moveFirst puts the node id first among its siblings at the root.
	moveFirst := func(by, id string) step {
		return request(by, http.MethodPost, "/api/v0/nodes/"+id+"/move", `{"parent_id":null,"after_id":null}`)
	}
	name := func(id string) string {
		return queryStrings(t, tm.pool, "SELECT name FROM nodes WHERE id = $1", id)[0]
	}

	// (1) The template's revert, which would delete its two pages.
	if nb.wiki {
		tm.guarded(t, nb, "alice reverts the template", changesetRevert("alice", nb.template), refused("page.protected", ok))
	}
	// (2) A revert puts back a rename of raw as a rename of the source
	// root: a writer's is refused, an admin's is not.
	tm.guarded(t, nb, "alice renames raw", rename("alice", nb.raw, "sources"), ok)
	renamed := tm.newest(t, nb.id).ID
	tm.guarded(t, nb, "bob reverts it", changesetRevert("bob", renamed), refused("page.source_admin_only", ok))
	if nb.wiki {
		tm.guarded(t, nb, "alice reverts it", changesetRevert("alice", renamed), ok)
	}
	if got := name(nb.raw); got != "raw" {
		t.Errorf("%s: raw after the revert of its rename is named %q, want raw", nb.name, got)
	}
	// (3) A page created at the root as a generated page's name.
	tm.guarded(t, nb, "bob creates index at the root", pageCreation("bob", nb.id, "index"), refused("page.virtual_file", created))
	tm.guarded(t, nb, "bob creates index.md at the root", pageCreation("bob", nb.id, "index.md"), refused("page.virtual_file", created))
	tm.guarded(t, nb, "bob creates index under Notes", childCreation("bob", nb.id, notes, "index"), created)
	// (4) Renames.
	tm.guarded(t, nb, "alice renames AGENTS", rename("alice", nb.agents, "Rules"), refused("page.protected", ok))
	tm.guarded(t, nb, "bob renames raw", rename("bob", nb.raw, "sources"), refused("page.source_admin_only", ok))
	tm.guarded(t, nb, "bob renames Other to log.md", rename("bob", other, "log.md"), refused("page.virtual_file", ok))
	// (5) AGENTS reordered, and moved.
	tm.guarded(t, nb, "alice puts AGENTS first", moveFirst("alice", nb.agents), refused("page.protected", ok))
	tm.guarded(t, nb, "alice moves AGENTS under Notes", moveUnder("alice", nb.agents, notes), refused("page.protected", ok))
	// (6) raw moved, and Notes once raw is under it: a reorder and a
	// rename of it leave the source area where it is, a move under
	// another page takes it along.
	tm.guarded(t, nb, "bob moves raw under Notes", moveUnder("bob", nb.raw, notes), refused("page.source_admin_only", ok))
	tm.guarded(t, nb, "alice moves raw under Notes", moveUnder("alice", nb.raw, notes), ok)
	tm.guarded(t, nb, "bob puts Notes first", moveFirst("bob", notes), ok)
	tm.guarded(t, nb, "bob moves Notes under Other", moveUnder("bob", notes, other), refused("page.source_admin_only", ok))
	tm.guarded(t, nb, "bob renames Notes", rename("bob", notes, "Papers"), ok)
	// (7) An attachment at the root as a generated page's name, its key
	// folded.
	tm.guarded(t, nb, "bob uploads LOG at the root", assetUpload(t, "bob", nb.id, "", "LOG", "x"), refused("page.virtual_file", created))
	tm.guarded(t, nb, "bob uploads log.png at the root", assetUpload(t, "bob", nb.id, "", "log.png", pngHead), created)
	// (8) The two pages' contents are written as any page's.
	for _, p := range []struct{ name, id string }{{"AGENTS", nb.agents}, {"raw", nb.raw}} {
		tm.guarded(t, nb, "alice writes "+p.name, contentWrite("alice", p.id, "- [ ] a\n", 1, ""), ok)
		tm.guarded(t, nb, "alice ticks "+p.name+"'s task", taskToggle("alice", p.id, 2, 3, true), ok)
		tm.guarded(t, nb, "alice restores "+p.name+"'s first version", versionRestore("alice", p.id, 1, 3), ok)
		tm.guarded(t, nb, "alice opens an edit session of "+p.name, sessionOpening("alice", p.id), created)
	}
	// (9) Last, the deletions, an admin's: AGENTS, raw, and Notes, raw's
	// parent.
	tm.guarded(t, nb, "alice deletes AGENTS", deletion("alice", nb.agents), refused("page.protected", gone))
	tm.guarded(t, nb, "alice deletes raw", deletion("alice", nb.raw), refused("page.protected", gone))
	tm.guarded(t, nb, "alice deletes Notes", deletion("alice", notes), refused("page.protected", gone))
	if alive := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id IN ($1, $2) AND deleted_at IS NULL", nb.agents, nb.raw); nb.wiki &&
		(alive != 2 || name(nb.agents) != "AGENTS" || name(nb.raw) != "raw") {
		t.Errorf("%s: AGENTS and raw at the end: %d alive, named %q and %q; want both, so named", nb.name, alive, name(nb.agents),
			name(nb.raw))
	}
}

// The mode guard comes before the edit lock (M10/P1 design 3.20): with
// bob's edit session holding AGENTS, alice's deletion of it is
// page.protected, not page.locked; once the notebook is downgraded, the
// same deletion meets the lock, page.locked naming bob's. The
// composition root registering the two the other way, or the mode guard
// not at all, fails the first.
func TestModeComesBeforeTheLock(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Wiki")
	tm.openSession(t, "bob", wiki.agents)
	deletion := nodeDeletion("alice", wiki.agents)
	tm.guarded(t, wiki, "alice deletes AGENTS that bob edits", deletion, cell{http.StatusConflict, "page.protected"})
	if status, answer := tm.downgrade(t, "alice", wiki.id); status != http.StatusOK {
		t.Fatalf("downgrade = %d %s, want 200", status, answer)
	}
	wiki.name = "the notebook downgraded"
	body := tm.guarded(t, wiki, "alice deletes AGENTS that bob edits", deletion, cell{http.StatusConflict, "page.locked"})
	if lockHolder(answer{body: body}) != "bob" {
		t.Errorf("the deletion once downgraded = %s, want bob's lock", body)
	}
	checkPages(t, tm.pool)
}

// The paths with unlocked checks answer the same refusals as individual
// guarded writes; a later refused undo step wins over an earlier edit
// lock because the whole contract is checked before execution.
func TestPrechecksAnswerTheGuardsCodes(t *testing.T) {
	tm := newAcmeTeam(t, "member", "member")
	wiki := tm.llmWiki(t, "Wiki")
	alice, bob := tm.agent(t, "alice", wiki.id), tm.agent(t, "bob", wiki.id)
	for _, row := range []struct {
		op   map[string]any
		code string
	}{
		{opMove("AGENTS.md", "Rules.md"), "page.protected"},
		{opMove("raw.md", "sources.md"), "page.source_admin_only"},
		{opWrite("index.md", "x"), "page.virtual_file"},
	} {
		before := tm.guardRows(t, wiki.id)
		tool, _ := row.op["op"].(string)
		args := map[string]any{}
		for k, v := range row.op {
			if k != "op" {
				args[k] = v
			}
		}
		direct := refusalOf(t, bob.use(t, tool, args))
		batch := refusalOf(t, bob.batch(t, row.op))
		if direct.Code != row.code || batch.Code != direct.Code || refusedAt(batch) != 0 {
			t.Errorf("%s: guard = %+v, batch = %+v; want %s at op 0", tool, direct, batch, row.code)
		}
		if after := tm.guardRows(t, wiki.id); after != before {
			t.Errorf("refused batch changed rows: %s -> %s", before, after)
		}
	}
	tm.guarded(t, wiki, "reserved upload", assetUpload(t, "bob", wiki.id, "", "index", "x"), cell{http.StatusConflict, "page.virtual_file"})
	tm.openSession(t, "carol", wiki.agents)
	for _, call := range []step{changesetRevert("alice", wiki.template), revertStart("alice", wiki.id, wiki.template, "")} {
		before := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", wiki.id)
		tm.guarded(t, wiki, "template before its lock", call, cell{http.StatusConflict, "page.protected"})
		if after := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", wiki.id); after != before {
			t.Errorf("refused start created a job: %d -> %d", before, after)
		}
	}
	renamed := alice.batchDone(t, opMove("raw.md", "sources.md"))
	if renamed.ChangesetID == nil {
		t.Fatal("batch made no changeset")
	}
	for _, call := range []step{changesetRevert("bob", *renamed.ChangesetID), revertStart("bob", wiki.id, *renamed.ChangesetID, "")} {
		tm.guarded(t, wiki, "source put back", call, cell{http.StatusForbidden, "page.source_admin_only"})
	}
	if jobs := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", wiki.id); jobs != 0 {
		t.Errorf("refused starts left %d jobs", jobs)
	}
	tm.guarded(t, wiki, "admin puts source back", changesetRevert("alice", *renamed.ChangesetID), cellOK())
	mixed := alice.batchDone(t, opMove("raw.md", "sources.md"), opWrite("raw.md", "new raw"))
	if mixed.ChangesetID == nil {
		t.Fatal("batch made no changeset")
	}
	newRaw := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id=$1 AND parent_id IS NULL AND name='raw' AND deleted_at IS NULL", wiki.id)[0]
	tm.openSession(t, "carol", newRaw)
	tm.guarded(t, wiki, "contract before an earlier deletion's lock", changesetRevert("bob", *mixed.ChangesetID), cell{http.StatusForbidden, "page.source_admin_only"})
	if got := queryStrings(t, tm.pool, "SELECT name FROM nodes WHERE id=$1", wiki.raw)[0]; got != "sources" {
		t.Errorf("source = %q, want sources", got)
	}
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// An undo rejected by the contract never waits for parse capacity. The
// same undo by an admin reaches that budget and answers server_busy.
func TestTheServersLimitsComeAfterThePrechecks(t *testing.T) {
	tm := newAcmeTeamWith(t, "member", "", func(c *config.Config) {
		c.Page.ParseBudgetBytes, c.Page.ParseMaxWait = 64<<10, 300*time.Millisecond
	})
	wiki := tm.llmWiki(t, "Wiki")
	notes := tm.createPageWith(t, "alice", wiki.id, "", "Notes", strings.Repeat("n", 1<<10))
	alice := tm.agent(t, "alice", wiki.id)
	changed := alice.batchDone(t, opEdit("Notes.md", strings.Repeat("n", 1<<10), "x"), opMove("raw.md", "sources.md"))
	if changed.ChangesetID == nil {
		t.Fatal("batch made no changeset")
	}
	plain := tm.openNotebook(t, "alice", "Plain")
	held := tm.createPage(t, "alice", plain, "", "Held")
	whileAWriteWaitsForItsLock(t, tm, held, strings.Repeat("a", 640<<10), 1, func() {
		tm.guarded(t, wiki, "writer refused before parsing", changesetRevert("bob", *changed.ChangesetID), cell{http.StatusForbidden, "page.source_admin_only"})
		tm.guarded(t, wiki, "admin reaches the occupied budget", changesetRevert("alice", *changed.ChangesetID), cell{http.StatusServiceUnavailable, "server_busy"})
	})
	if got := queryStrings(t, tm.pool, "SELECT content FROM page_contents WHERE node_id=$1", notes)[0]; got != "x" {
		t.Errorf("Notes = %q, want x", got)
	}
	checkPages(t, tm.pool)
}
