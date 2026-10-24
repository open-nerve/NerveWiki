package bootstrap

import (
	"encoding/json"
	"log/slog"
	"net/http"
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
