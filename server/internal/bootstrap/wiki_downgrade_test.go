package bootstrap

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

// The downgrade through the whole program (M10/P1 design 3.16).

// downgrade is by's downgrade of the notebook id, as the API answers it.
func (tm acmeTeam) downgrade(t *testing.T, by, id string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+id+"/downgrade", tm.tokens[by], "")
}

// The downgrade's codes in their order: a standard notebook is none to
// downgrade; an LLM Wiki with an import queued, the imports' one worker
// held, waits for it (transfer.busy), its mode kept; once the import has
// ended, it is made standard, the answer what a read answers after, and
// logged once by its ids, without its name.
func TestTheDowngradeAnswersInItsOrder(t *testing.T) {
	logs := &syncBuffer{}
	tm := newAcmeTeamLogging(t, "", "", nil, transfer.Batch{}, slog.New(slog.NewJSONHandler(logs, nil)))
	plain := tm.createWith(t, tm.tokens["alice"], `{"name":"Plain"}`)
	wiki := tm.createWith(t, tm.tokens["alice"], `{"name":"Field notes","mode":"llm_wiki"}`)

	if status, answer := tm.downgrade(t, "alice", plain.ID); status != http.StatusConflict ||
		!strings.Contains(answer, `"code":"notebook.not_llm_wiki"`) {
		t.Errorf("downgrade a standard notebook = %d %s, want 409 notebook.not_llm_wiki", status, answer)
	}

	release := tm.holdImportWorker(t)
	j := tm.started(t, importStep(t, "alice", wiki.ID, "", vaultZip(t, vaultFile{"a.md", "a"})))
	if status, answer := tm.downgrade(t, "alice", wiki.ID); status != http.StatusConflict ||
		!strings.Contains(answer, `"code":"transfer.busy"`) {
		t.Errorf("downgrade with an import queued = %d %s, want 409 transfer.busy", status, answer)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE id = $1 AND mode = 'llm_wiki' AND source_root_id IS NOT NULL",
		wiki.ID); n != 1 {
		t.Errorf("the LLM Wiki after a refused downgrade: %d rows of it, want one still an LLM Wiki", n)
	}
	release()
	if ended := tm.endedJob(t, "alice", j.ID); ended.State != "succeeded" {
		t.Fatalf("the import = %+v, want succeeded", ended)
	}

	before := len((&mcpApp{logs: logs}).logLines(t))
	status, answer := tm.downgrade(t, "alice", wiki.ID)
	if status != http.StatusOK {
		t.Fatalf("downgrade once the import ended = %d %s, want 200", status, answer)
	}
	var got createdNotebook
	if err := json.Unmarshal([]byte(answer), &got); err != nil || got.Mode != "standard" || got.SourceRootID != nil {
		t.Errorf("the downgrade answered %s, want the notebook standard without a source root", answer)
	}
	if status, read := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+wiki.ID, tm.tokens["alice"], ""); status != http.StatusOK ||
		read != answer {
		t.Errorf("GET after the downgrade = %d %s, want the downgrade's answer %s", status, read, answer)
	}
	var lines []map[string]any
	for _, e := range (&mcpApp{logs: logs}).logLines(t)[before:] {
		if e["msg"] == "notebook downgraded" {
			lines = append(lines, e)
		}
	}
	acme := queryStrings(t, tm.pool, "SELECT id::text FROM workspaces WHERE slug = 'acme'")
	if len(lines) != 1 || lines[0]["notebook_id"] != wiki.ID || lines[0]["user_id"] != tm.userID(t, "alice").String() ||
		len(acme) != 1 || lines[0]["workspace_id"] != acme[0] || strings.Contains(logs.String(), "Field notes") {
		t.Errorf("logged %v, want one line of the downgrade by its ids, without the notebook's name", lines)
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
}

// The mode event's last hop (M10/P1 design 3.11): a downgrade reaches, as
// one mode frame of the notebook's ids, each stream that sees the
// notebook, its admin's and a reader's, and no other, a member's who does
// not see it; a write of a marker page every stream sees comes next in
// each. The composition root registers the streams as a mode subscriber:
// without it, no mode frame comes.
func TestTheModeEventReachesTheStreams(t *testing.T) {
	tm := newAcmeTeam(t, "member", "member")
	wiki := tm.createWith(t, tm.tokens["alice"], `{"name":"Field notes","mode":"llm_wiki"}`)
	tm.addNotebookMember(t, "alice", wiki.ID, "bob", "reader")
	eng := tm.openNotebook(t, "alice", "Eng")
	marker := tm.createPage(t, "alice", eng, "", "Marker")
	tm.settle(t, eng, marker)
	alice := openStream(t, tm.base, "alice", tm.tokens["alice"])
	bob := openStream(t, tm.base, "bob", tm.tokens["bob"])
	carol := openStream(t, tm.base, "carol", tm.tokens["carol"])

	if status, answer := tm.downgrade(t, "alice", wiki.ID); status != http.StatusOK {
		t.Fatalf("downgrade = %d %s, want 200", status, answer)
	}

	acme := queryStrings(t, tm.pool, "SELECT id::text FROM workspaces WHERE slug = 'acme'")
	for _, s := range []*eventStream{alice, bob} {
		f := s.expect(t, "mode")
		apitest.Load(t).CheckSchema(t, "EventMode", []byte(f.data))
		if len(acme) != 1 || f.field(t, "workspace_id") != acme[0] || f.field(t, "notebook_id") != wiki.ID {
			t.Errorf("%s's stream: mode %s, want acme's %s", s.name, f.data, wiki.ID)
		}
	}
	tm.quiet(t, eng, marker, alice, bob, carol)
}

// A downgrade ends the contract (M10/P1 design 3.24): the notebook,
// standard, has a writer rename raw and move it under Notes, alice rename
// AGENTS and a writer create a page at its root named index, and an agent
// delete AGENTS.md. P2/S1 also checks that the same nonempty source, whose
// body both writer and admin could not change, becomes writable through
// REST and MCP. It is the test LW12's MCP exemption names (M10 design 9):
// P2/S2 adds duplicates let by, P4 list without the two generated pages,
// P5 the handshake without the system's part.
func TestTheDowngradeEndsTheContract(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Wiki")
	notes := tm.createPage(t, "alice", wiki.id, "", "Notes")
	source := tm.createPageWith(t, "bob", wiki.id, wiki.raw, "Source", "source original")
	for _, by := range []string{"bob", "alice"} {
		tm.sourceUnchanged(t, wiki, source, contentWrite(by, source, "changed before downgrade", 1, ""),
			cell{http.StatusConflict, "page.source_immutable"})
	}
	if status, answer := tm.downgrade(t, "alice", wiki.id); status != http.StatusOK {
		t.Fatalf("downgrade = %d %s, want 200", status, answer)
	}
	wiki.name = "the notebook downgraded"
	// Write before moving raw so both clients address the very same page
	// under its unchanged source parent; only the notebook mode changed.
	tm.send(t, contentWrite("bob", source, "REST after downgrade", 1, ""), http.StatusOK)
	if got := tm.content(t, "bob", source); got.Content != "REST after downgrade" || got.Revision != 2 {
		t.Fatalf("the former source after REST = %+v, want REST after downgrade at revision 2", got)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM page_contents c JOIN nodes n ON n.id = c.node_id
		JOIN notebooks b ON b.id = n.notebook_id WHERE c.node_id = $1 AND n.parent_id = $2
		AND c.content = 'REST after downgrade' AND c.revision = 2
		AND b.mode = 'standard' AND b.source_root_id IS NULL`, source, wiki.raw); n != 1 {
		t.Fatalf("the former source's stored REST write: %d rows, want its body at revision 2 under the same parent", n)
	}
	edited := answerOf[mcpEdited](t, tm.agent(t, "bob", wiki.id).use(t, "edit", map[string]any{
		"path": "raw/Source.md", "old_string": "REST", "new_string": "MCP", "base_version": source + ":2",
	}))
	if !edited.Changed || edited.Version != source+":3" || edited.Replacements != 1 || edited.ChangesetID == nil {
		t.Fatalf("the former source after MCP = %+v, want one actual edit of the same page at revision 3", edited)
	}
	if got := tm.content(t, "bob", source); got.Content != "MCP after downgrade" || got.Revision != 3 {
		t.Errorf("the former source after MCP = %+v, want MCP after downgrade at revision 3", got)
	}
	if got := queryStrings(t, tm.pool, "SELECT content || ':' || revision::text FROM page_contents WHERE node_id = $1", source); !slices.Equal(got, []string{"MCP after downgrade:3"}) {
		t.Errorf("stored current source = %q, want MCP after downgrade at revision 3", got)
	}
	if got := queryStrings(t, tm.pool, `SELECT revision::text || ':' || content || ':' || rewrite::text
		FROM page_revisions WHERE node_id = $1 ORDER BY revision`, source); !slices.Equal(got, []string{"1:source original:false", "2:REST after downgrade:false", "3:MCP after downgrade:false"}) {
		t.Errorf("stored source versions = %q, want the original and two ordinary writes", got)
	}
	tm.guarded(t, wiki, "bob renames raw", nodeRename("bob", wiki.raw, "sources"), cellOK())
	tm.guarded(t, wiki, "bob moves it under Notes", nodeMove("bob", wiki.raw, notes), cellOK())
	tm.guarded(t, wiki, "alice renames AGENTS", nodeRename("alice", wiki.agents, "Agents"), cellOK())
	tm.guarded(t, wiki, "bob creates index at the root", pageCreation("bob", wiki.id, "index"), cellCreated())
	if r := tm.agent(t, "alice", wiki.id).use(t, "delete", map[string]any{"path": "AGENTS.md"}).result(t); r["isError"] == true {
		t.Errorf("an agent's delete of AGENTS.md once downgraded = %v, want it deleted", r)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NOT NULL", wiki.agents); n != 1 {
		t.Errorf("AGENTS after the agent's delete: %d of it deleted, want it", n)
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
}
