package bootstrap

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// The MCP writes over the whole program (M9/P2 design 3.13.4–3.14): each
// tool reaches its use case in both entries, a write never lands on
// another page than its version's, a move without its links leaves them,
// and the lines the writes log name no path and no content.

// agent is by's MCP client of tm's app, through a personal access token
// of by's own, its calls in the notebook nb.
func (tm acmeTeam) agent(t *testing.T, by, nb string) *mcpApp {
	t.Helper()
	return &mcpApp{base: tm.base, contract: tm.contract, notebook: nb, pat: createToken(t, tm.contract, tm.base, tm.tokens[by])}
}

// use is m's call of tool with args in its notebook, through the notebook
// entry.
func (m *mcpApp) use(t testing.TB, tool string, args map[string]any) mcpAnswer {
	t.Helper()
	return m.call(t, atNotebook(m.notebook), tool, args)
}

// answerOf is a's structured result as a T, failing on a tool's error.
func answerOf[T any](t *testing.T, a mcpAnswer) T {
	t.Helper()
	if r := a.result(t); r["isError"] == true {
		t.Fatalf("%s answered an error: %.500s", a.tool, a.body)
	}
	var v T
	structuredOf(t, a, &v)
	return v
}

// mcpWritten is write's answer, as these tests read it.
type mcpWritten struct {
	Path    string `json:"path"`
	NodeID  string `json:"node_id"`
	Version string `json:"version"`
	Changed bool   `json:"changed"`
	Created []struct {
		Path   string `json:"path"`
		NodeID string `json:"node_id"`
	} `json:"created"`
	CreatedCount int     `json:"created_count"`
	ChangesetID  *string `json:"changeset_id"`
}

// mcpEdited is edit's answer.
type mcpEdited struct {
	Version      string  `json:"version"`
	Replacements int     `json:"replacements"`
	Changed      bool    `json:"changed"`
	ChangesetID  *string `json:"changeset_id"`
}

// mcpMoved is move's answer.
type mcpMoved struct {
	Path        string  `json:"path"`
	NodeID      string  `json:"node_id"`
	Changed     bool    `json:"changed"`
	ChangesetID *string `json:"changeset_id"`
}

// mcpDeleted is delete's answer.
type mcpDeleted struct {
	NodeID      string `json:"node_id"`
	Pages       int    `json:"pages"`
	Attachments int    `json:"attachments"`
	ChangesetID string `json:"changeset_id"`
}

// mcpLock is a lock of a tool's error.
type mcpLock struct {
	PageID      string  `json:"page_id"`
	Path        *string `json:"path"`
	DisplayName string  `json:"display_name"`
	You         bool    `json:"you"`
}

// mcpRefusal is a tool's error, as these tests read it: its code, its
// text, the version there now, and the locks that refused it; lockPath
// whether its lock member has a path.
type mcpRefusal struct {
	Code           string    `json:"code"`
	Message        string    `json:"message"`
	CurrentVersion string    `json:"current_version"`
	Lock           *mcpLock  `json:"lock"`
	Locks          []mcpLock `json:"locks"`
	LocksCount     int       `json:"locks_count"`
	lockPath       bool
}

// refusalOf is a's tool error, failing on a result that is none.
func refusalOf(t *testing.T, a mcpAnswer) mcpRefusal {
	t.Helper()
	member := a.toolError(t)
	raw, err := json.Marshal(member)
	var r mcpRefusal
	if err == nil {
		err = json.Unmarshal(raw, &r)
	}
	if err != nil {
		t.Fatalf("the error of %.300s: %v", a.body, err)
	}
	if lock, ok := member["lock"].(map[string]any); ok {
		_, r.lockPath = lock["path"]
	}
	return r
}

// versionOf is the version token of the page id at revision.
func versionOf(id string, revision int) string { return id + ":" + strconv.Itoa(revision) }

// placeOf is the node id's parent ("" for the root) and name.
func (tm acmeTeam) placeOf(t *testing.T, id string) string {
	t.Helper()
	return queryStrings(t, tm.pool, "SELECT coalesce(parent_id::text, '') || '/' || name FROM nodes WHERE id = $1", id)[0]
}

// Each write tool reaches its use case in both entries (M9/P2 design
// 3.13.6): the composition root that left one out fails here. write
// creates a page with its missing parents in one changeset of the agent's
// client, writes nothing for the content the page holds, and replaces it
// on read's version, bytes as they are; edit replaces once or every time,
// CRLF kept; move renames, moves and renames in one change, recases
// across parents as REST's two steps do, and renames an attachment; delete
// counts the children it refuses without recursive, and puts them in the
// trash with it. The invariants hold after each, and the search's queue
// holds what MCP wrote.
func TestTheMCPWritesReachTheUseCases(t *testing.T) {
	for _, entry := range []mcpserver.Entry{mcpserver.NotebookEntry, mcpserver.AccountEntry} {
		t.Run(entry.String(), func(t *testing.T) {
			tm := newAcmeTeam(t, "", "")
			nb, ops := tm.openNotebook(t, "alice", "Eng"), tm.openNotebook(t, "alice", "Ops")
			m := tm.agent(t, "alice", nb)
			m.handshakeAs(t, "whole-program")
			use := func(tool string, args map[string]any) mcpAnswer {
				t.Helper()
				if entry == mcpserver.AccountEntry {
					args["notebook"] = nb
					return m.call(t, mcpserver.Pattern, tool, args)
				}
				return m.use(t, tool, args)
			}
			checked := func() {
				t.Helper()
				checkPages(t, tm.pool)
				checkLinks(t, tm.pool)
			}
			changesets := func() int {
				return count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", nb)
			}

			// write: a page and its two missing parents, from the root down.
			plan := answerOf[mcpWritten](t, use("write", map[string]any{"path": "Projects/Q3/Plan.md", "content": "# Plan\n"}))
			var created []string
			for _, c := range plan.Created {
				created = append(created, c.Path)
			}
			if !slices.Equal(created, []string{"Projects.md", "Projects/Q3.md", "Projects/Q3/Plan.md"}) || plan.CreatedCount != 3 ||
				plan.Version != versionOf(plan.NodeID, 1) || !plan.Changed || plan.ChangesetID == nil {
				t.Fatalf("write of a new page under two missing parents = %+v", plan)
			}
			projects, q3 := plan.Created[0].NodeID, plan.Created[1].NodeID
			if got := queryStrings(t, tm.pool, `SELECT c.kind || ' ' || c.client || ' ' || u.email FROM changesets c
				JOIN users u ON u.id = c.created_by_id WHERE c.id = $1`, *plan.ChangesetID); got[0] != "edit mcp:whole-program alice@example.com" {
				t.Errorf("the write's changeset is %q, want an edit of mcp:whole-program by alice", got[0])
			}
			if items := count(t, tm.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id = $1", *plan.ChangesetID); items != 3 {
				t.Errorf("the write's changeset has %d items, want 3", items)
			}
			versions := queryStrings(t, tm.pool, `SELECT coalesce(base_revision::text, 'none') || ' ' || revision || ' ' || content
				FROM page_revisions WHERE changeset_id = $1 ORDER BY node_id`, *plan.ChangesetID)
			if !slices.Equal(versions, []string{"none 1 ", "none 1 ", "none 1 # Plan\n"}) {
				t.Errorf("the write's versions are %q, want the parents' empty and the page's, each the first", versions)
			}
			checked()
			before := changesets()
			if again := answerOf[mcpWritten](t, use("write", map[string]any{"path": "Projects/Q3/Plan.md", "content": "# Plan\n"})); again.Changed ||
				again.ChangesetID != nil || again.Version != plan.Version || changesets() != before {
				t.Errorf("the same write again = %+v, %d changesets; want nothing written", again, changesets())
			}
			if r := refusalOf(t, use("write", map[string]any{"path": "Projects/Q3/Plan.md", "content": "# Other\n"})); r.Code != "page.title_taken" {
				t.Errorf("a write of another content without a version = %+v, want page.title_taken", r)
			}

			// write over it, on read's version: its bytes as they are.
			meta, _, _ := readBack(t, use("read", map[string]any{"path": "Projects/Q3/Plan.md"}))
			const raw = "\xef\xbb\xbf# Plan\r\nline  \r\n\t\n"
			over := answerOf[mcpWritten](t, use("write", map[string]any{"path": "Projects/Q3/Plan.md", "content": raw, "base_version": meta.Version}))
			if over.Version != versionOf(plan.NodeID, 2) || !over.Changed || len(over.Created) != 0 {
				t.Errorf("the write on read's version = %+v, want version 2", over)
			}
			if r := refusalOf(t, use("write", map[string]any{"path": "Projects/Q3/Plan.md", "content": "x", "base_version": meta.Version})); r.Code !=
				"page.revision_mismatch" || r.CurrentVersion != versionOf(plan.NodeID, 2) {
				t.Errorf("a write on the old version = %+v, want page.revision_mismatch at version 2", r)
			}
			if _, text, _ := readBack(t, use("read", map[string]any{"path": "Projects/Q3/Plan.md"})); text != raw {
				t.Errorf("read gives %q, want the bytes written, %q", text, raw)
			}
			checked()

			// edit: once, refused where the string occurs three times, then
			// every time; a CRLF page edited with LF.
			use("write", map[string]any{"path": "Edits.md", "content": "alpha beta beta beta\n"})
			if e := answerOf[mcpEdited](t, use("edit", map[string]any{"path": "Edits.md", "old_string": "alpha", "new_string": "ALPHA"})); e.Replacements !=
				1 || !e.Changed || !strings.HasSuffix(e.Version, ":2") {
				t.Errorf("edit of a string found once = %+v, want one replacement at version 2", e)
			}
			if r := refusalOf(t, use("edit", map[string]any{"path": "Edits.md", "old_string": "beta", "new_string": "BETA"})); r.Code !=
				"validation_failed" || !strings.Contains(r.Message, "found 3 times") {
				t.Errorf("edit of a string found three times = %+v, want validation_failed: found 3 times", r)
			}
			all := answerOf[mcpEdited](t, use("edit", map[string]any{"path": "Edits.md", "old_string": "beta", "new_string": "BETA",
				"replace_all": true}))
			if meta, text, _ := readBack(t, use("read", map[string]any{"path": "Edits.md"})); all.Replacements != 3 || text != "ALPHA BETA BETA BETA\n" ||
				meta.Version != all.Version {
				t.Errorf("edit with replace_all = %+v, the page %q at %s; want three replacements, its version", all, text, meta.Version)
			}
			use("write", map[string]any{"path": "Lines.md", "content": "first\r\nsecond\r\nthird\r\n"})
			answerOf[mcpEdited](t, use("edit", map[string]any{"path": "Lines.md", "old_string": "first\nsecond", "new_string": "one\ntwo"}))
			if _, text, _ := readBack(t, use("read", map[string]any{"path": "Lines.md"})); text != "one\r\ntwo\r\nthird\r\n" {
				t.Errorf("edit of a CRLF page with LF wrote %q, want its CRLF kept", text)
			}
			// An old_string that starts with its one LF is matched as CRLF too,
			// not inside a CRLF: the page stays all CRLF, so the next edit
			// with LF finds its string (M9/P2 S1 review B-M1).
			answerOf[mcpEdited](t, use("edit", map[string]any{"path": "Lines.md", "old_string": "\nthird", "new_string": "\n3\nmore"}))
			answerOf[mcpEdited](t, use("edit", map[string]any{"path": "Lines.md", "old_string": "\ntwo", "new_string": " two"}))
			if _, text, _ := readBack(t, use("read", map[string]any{"path": "Lines.md"})); text != "one two\r\n3\r\nmore\r\n" {
				t.Errorf("edits of a CRLF page whose old_string starts with LF wrote %q, want it all CRLF", text)
			}
			checked()

			// move: a rename, a move and a rename in one change.
			renamed := answerOf[mcpMoved](t, use("move", map[string]any{"from": "Projects/Q3/Plan.md", "to": "Projects/Q3/Plan v2.md"}))
			if renamed.Path != "Projects/Q3/Plan v2.md" || renamed.NodeID != plan.NodeID || tm.placeOf(t, plan.NodeID) != q3+"/Plan v2" {
				t.Errorf("move as a rename = %+v, the node at %s", renamed, tm.placeOf(t, plan.NodeID))
			}
			both := answerOf[mcpMoved](t, use("move", map[string]any{"from": "Projects/Q3/Plan v2.md", "to": "Projects/Final.md"}))
			items := queryStrings(t, tm.pool, `SELECT before_parent_id::text || '/' || before_name || ' ' || after_parent_id::text || '/' || after_name
				FROM changeset_items WHERE changeset_id = $1`, *both.ChangesetID)
			if both.Path != "Projects/Final.md" || !slices.Equal(items, []string{q3 + "/Plan v2 " + projects + "/Final"}) {
				t.Errorf("move and rename = %+v, its changeset's items %q; want one, from Q3's Plan v2 to Projects' Final", both, items)
			}
			checked()

			// move across parents, only the case of the name: the links follow
			// as REST's move and rename leave them, in a notebook alike.
			recased := map[string]string{}
			for _, n := range []string{nb, ops} {
				a := tm.createPage(t, "alice", n, "", "a")
				tm.createPage(t, "alice", n, "", "b")
				recased[n] = tm.createPage(t, "alice", n, a, "x")
				recased[n+"/linker"] = tm.createPageWith(t, "alice", n, "", "Linker", "[[x]] [[x|别名]] [[x#h]]\n")
			}
			answerOf[mcpMoved](t, use("move", map[string]any{"from": "a/x.md", "to": "b/X.md"}))
			b := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = 'b'", ops)[0]
			tm.send(t, nodeMove("alice", recased[ops], b), http.StatusOK)
			tm.send(t, nodeRename("alice", recased[ops], "X"), http.StatusOK)
			if mcp, rest := tm.content(t, "alice", recased[nb+"/linker"]).Content, tm.content(t, "alice", recased[ops+"/linker"]).Content; mcp != rest {
				t.Errorf("the links after move's recase across parents are %q, REST's move and rename leave %q", mcp, rest)
			}
			tm.resolves(t, recased[nb+"/linker"], recased[nb], recased[nb], recased[nb])
			flow := tm.upload(t, "alice", nb, "", "flow.png", sevenByFive)
			if moved := answerOf[mcpMoved](t, use("move", map[string]any{"from": "flow.png", "to": "flow.jpg"})); moved.Path != "flow.jpg" ||
				tm.placeOf(t, flow.ID) != "/flow.jpg" {
				t.Errorf("move of an attachment to another extension = %+v, the node at %s", moved, tm.placeOf(t, flow.ID))
			}
			checked()

			// delete: refused with its children counted, then with them.
			if r := refusalOf(t, use("delete", map[string]any{"path": "Projects.md"})); r.Code != "validation_failed" ||
				!strings.Contains(r.Message, `"Projects.md" has 2 pages under it`) {
				t.Errorf("delete of a page with two children = %+v, want validation_failed counting them", r)
			}
			gone := answerOf[mcpDeleted](t, use("delete", map[string]any{"path": "Projects.md", "recursive": true}))
			if trash := queryStrings(t, tm.pool, "SELECT trash_root || ' ' || trash_pages || ' ' || trash_assets FROM nodes WHERE id = $1",
				projects); gone.NodeID != projects || gone.Pages != 3 || gone.Attachments != 0 || trash[0] != "true 3 0" {
				t.Errorf("delete with recursive = %+v, the trash root %q; want Projects and its two pages", gone, trash[0])
			}
			if items := tm.trash(t, "alice", nb); !slices.ContainsFunc(items, func(i trashItemFull) bool { return i.ID == projects && i.Pages == 3 }) {
				t.Errorf("the trash lists %+v, want Projects with its pages", items)
			}
			checked()
			drainSearch(t, tm.pool)
			checkSearch(t, tm.pool)
		})
	}
}

// A write never lands on another page than the one its version names
// (M9 design 4.10; M9/P2 design 3.13.4): N is read at Inbox/todo.md,
// renamed, and M created there with a content in one call, its version
// the token's revision; the write on N's version is page.revision_mismatch
// of another page, at M's version, and both pages keep their contents.
func TestAWriteNeverLandsOnAnotherPageAtItsPath(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	inbox := tm.createPage(t, "alice", nb, "", "Inbox")
	n := tm.createPageWith(t, "alice", nb, inbox, "todo", "N's")
	m := tm.agent(t, "alice", nb)
	meta, _, _ := readBack(t, m.use(t, "read", map[string]any{"path": "Inbox/todo.md"}))
	if meta.Version != versionOf(n, 1) {
		t.Fatalf("read gives the version %s, want %s", meta.Version, versionOf(n, 1))
	}
	tm.send(t, nodeRename("alice", n, "done"), http.StatusOK)
	other := tm.createPageWith(t, "alice", nb, inbox, "todo", "M's")
	r := refusalOf(t, m.use(t, "write", map[string]any{"path": "Inbox/todo.md", "content": "mine", "base_version": meta.Version}))
	if r.Code != "page.revision_mismatch" || !strings.Contains(r.Message, "is another page now than the one base_version was read from") ||
		r.CurrentVersion != versionOf(other, 1) {
		t.Errorf("the write on N's version at M's path = %+v, want page.revision_mismatch of another page at M's version", r)
	}
	for id, want := range map[string]string{n: "N's", other: "M's"} {
		if c := tm.content(t, "alice", id); c.Content != want || c.Revision != 1 {
			t.Errorf("the page %s holds %q at %d, want %q at 1", id, c.Content, c.Revision, want)
		}
	}
	checkPages(t, tm.pool)
}

// A version of a page in another notebook writes nothing (M9/P2 design
// 3.13.4): A and B each hold Doc.md, each made with its content in one
// call; a write of A's Doc.md on B's Doc's version is page.revision_mismatch
// of another page at A's version, and neither notebook has a changeset
// more.
func TestAVersionOfAnotherNotebookWritesNothing(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	a, b := tm.openNotebook(t, "alice", "A"), tm.openNotebook(t, "alice", "B")
	inA, inB := tm.createPageWith(t, "alice", a, "", "Doc", "A's"), tm.createPageWith(t, "alice", b, "", "Doc", "B's")
	changesets := func() int { return count(t, tm.pool, "SELECT count(*) FROM changesets") }
	before := changesets()
	r := refusalOf(t, tm.agent(t, "alice", a).use(t, "write", map[string]any{"path": "Doc.md", "content": "x", "base_version": versionOf(inB, 1)}))
	if r.Code != "page.revision_mismatch" || !strings.Contains(r.Message, "is another page now") || r.CurrentVersion != versionOf(inA, 1) {
		t.Errorf("the write of A's Doc.md on B's version = %+v, want page.revision_mismatch of another page at A's version", r)
	}
	for id, want := range map[string]string{inA: "A's", inB: "B's"} {
		if c := tm.content(t, "alice", id); c.Content != want || c.Revision != 1 {
			t.Errorf("the page %s holds %q at %d, want %q at 1", id, c.Content, c.Revision, want)
		}
	}
	if changesets() != before {
		t.Errorf("%d changesets, want %d", changesets(), before)
	}
}

// A move rewrites the links to what moves unless update_links is false
// (M6/P4's hand-over, item 2; M9/P2 design 3.13.5). With B in alice's
// edit session and C and E in bob's: the move of A, whose links B and C
// hold, is linking.pages_locked by both, in their pages' ids' order, hers
// told as hers, and writes nothing; once B and C are free, it moves A and
// their links follow; the move of D without its links passes E's session,
// leaves E's bytes, and E's index, parsed again, resolves nowhere.
func TestAMoveWithoutItsLinksLeavesThemAndTheIndex(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	tm.createPage(t, "alice", nb, "", "X")
	a := tm.createPage(t, "alice", nb, "", "A")
	c := tm.createPageWith(t, "alice", nb, "", "C", "[[A]]\n")
	b := tm.createPageWith(t, "alice", nb, "", "B", "[[A]] [[A|别名]]\n")
	tm.createPage(t, "alice", nb, "", "D")
	e := tm.createPageWith(t, "alice", nb, "", "E", "[[D]] [[D|别名]]\n")
	hers, his := tm.openSession(t, "alice", b), tm.openSession(t, "bob", c)
	tm.openSession(t, "bob", e)
	m := tm.agent(t, "alice", nb)

	r := refusalOf(t, m.use(t, "move", map[string]any{"from": "A.md", "to": "X/A2.md"}))
	pathOf := func(l mcpLock) string {
		if l.Path == nil {
			return "null"
		}
		return *l.Path
	}
	var locks []string
	for _, l := range r.Locks {
		locks = append(locks, l.PageID+" "+pathOf(l)+" "+l.DisplayName+" "+strconv.FormatBool(l.You))
	}
	if want := []string{c + " C.md bob false", b + " B.md alice true"}; r.Code != "linking.pages_locked" || r.LocksCount != 2 ||
		!slices.Equal(locks, want) || !strings.Contains(r.Message, "2 pages being edited in the web editor, by bob and by you, in a browser") {
		t.Errorf("the move of A with B and C being edited = %+v, locks %q; want linking.pages_locked by %q", r, locks, want)
	}
	if tm.placeOf(t, a) != "/A" {
		t.Errorf("the move refused left A at %s", tm.placeOf(t, a))
	}

	tm.send(t, request("alice", http.MethodDelete, "/api/v0/edit-sessions/"+hers, ""), http.StatusNoContent)
	tm.send(t, request("bob", http.MethodDelete, "/api/v0/edit-sessions/"+his, ""), http.StatusNoContent)
	if moved := answerOf[mcpMoved](t, m.use(t, "move", map[string]any{"from": "A.md", "to": "X/A2.md"})); moved.Path != "X/A2.md" {
		t.Errorf("the move of A once B and C are free = %+v", moved)
	}
	for id, want := range map[string]string{b: "[[A2]] [[A2|别名]]\n", c: "[[A2]]\n"} {
		if got := tm.content(t, "alice", id).Content; got != want {
			t.Errorf("the links that followed A read %q, want %q", got, want)
		}
	}
	tm.resolves(t, b, a, a)
	tm.resolves(t, c, a)
	checkLinks(t, tm.pool)

	was := tm.content(t, "bob", e)
	if moved := answerOf[mcpMoved](t, m.use(t, "move", map[string]any{"from": "D.md", "to": "X/D2.md", "update_links": false})); moved.Path != "X/D2.md" {
		t.Errorf("the move of D without its links = %+v", moved)
	}
	if now := tm.content(t, "bob", e); now != was {
		t.Errorf("E after the move without its links = %+v, want as it was, %+v", now, was)
	}
	tm.resolves(t, e, "", "")
	checkLinks(t, tm.pool)
	checkPages(t, tm.pool)
}

// The writes' lines name ids and counts, never a path, a title, a content
// or an old_string (M9/P2 design 3.14): each use case's line, of the
// agent's client, beside each call's, with every path and content of the
// marker letters, which no line holds.
func TestTheMCPWritesLogNoPathOrContent(t *testing.T) {
	m := startMCPApp(t, nil)
	m.handshakeAs(t, "whole-program")
	const mark = "qj"
	written := answerOf[mcpWritten](t, m.use(t, "write", map[string]any{"path": "Qjq/Qjqj.md", "content": "qjqj qjqj\n"}))
	answerOf[mcpWritten](t, m.use(t, "write", map[string]any{"path": "Qjq/Qjqj.md", "content": "qjqj\n", "base_version": written.Version}))
	answerOf[mcpEdited](t, m.use(t, "edit", map[string]any{"path": "Qjq/Qjqj.md", "old_string": "qjqj", "new_string": "jqjq"}))
	answerOf[mcpMoved](t, m.use(t, "move", map[string]any{"from": "Qjq/Qjqj.md", "to": "Qjq/Qjjq.md"}))
	answerOf[mcpWritten](t, m.use(t, "write", map[string]any{"path": "Jqj.md", "content": "qj"}))
	answerOf[mcpMoved](t, m.use(t, "move", map[string]any{"from": "Jqj.md", "to": "Qjq/Jqj.md", "update_links": false}))
	answerOf[mcpDeleted](t, m.use(t, "delete", map[string]any{"path": "Qjq.md", "recursive": true}))
	for _, tt := range []struct {
		msg  string
		want map[string]any
	}{
		{"page created", map[string]any{"pages": 2.0}},
		{"page content written", map[string]any{"revision": 2.0}},
		{"page edited", map[string]any{"revision": 3.0, "replacements": 1.0, "redone": false}},
		{"node renamed", map[string]any{"update_links": true}},
		{"node moved", map[string]any{"update_links": false}},
		{"node deleted", map[string]any{"nodes": 3.0, "pages": 3.0, "assets": 0.0}},
	} {
		line := m.logged(t, tt.msg)
		if line == nil || line["client"] != "mcp:whole-program" || line["node_id"] == nil || line["changeset_id"] == nil {
			t.Errorf("the line %q = %v, want the agent's client, the node and the changeset", tt.msg, line)
			continue
		}
		for k, v := range tt.want {
			if line[k] != v {
				t.Errorf("the line %q has %s %v, want %v", tt.msg, k, line[k], v)
			}
		}
	}
	for _, tool := range []string{"write", "edit", "move", "delete"} {
		if line := m.callLogged(t, tool); line == nil || line["outcome"] != "ok" {
			t.Errorf("the call line of %s = %v, want one, ok", tool, line)
		}
	}
	if logs := strings.ToLower(m.logs.String()); strings.Contains(logs, mark) {
		t.Errorf("a log line names a path or a content:\n%s", m.logs.String())
	}
}
