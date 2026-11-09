package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

// An upgrade's start and read through the whole program (M11/P1 design
// 3.14, 7.1); the wiki module's half with fakes is wiki/app's
// TestUpgradeStart.

// getUpgrade is by's read of nb's upgrade, as the API answers it.
func (tm acmeTeam) getUpgrade(t *testing.T, by, nb string) (int, string) {
	t.Helper()
	return ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb+"/upgrade", tm.tokens[by], "")
}

// wantCode fails t unless status and answer are want's.
func wantCode(t *testing.T, what string, status int, answer string, want cell) {
	t.Helper()
	if got := (cell{status: status, code: problemCodeOf(answer)}); got != want {
		t.Errorf("%s = %d %s, want %s", what, status, answer, want)
	}
}

// problemCodeOf is answer's problem code, "" for none.
func problemCodeOf(answer string) string {
	_, rest, ok := strings.Cut(answer, `"code":"`)
	if !ok {
		return ""
	}
	code, _, _ := strings.Cut(rest, `"`)
	return code
}

// upgradeLines are the log's lines of msg after the first before.
func upgradeLines(t *testing.T, logs *syncBuffer, before int, msg string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range (&mcpApp{logs: logs}).logLines(t)[before:] {
		if l["msg"] == msg {
			out = append(out, l)
		}
	}
	return out
}

// The start's codes in M11 design 4.4's order, each refusal leaving the
// notebook as it was; the template's pages the notebook lacks written at
// its root's end, as the upgrade's own unit, the baseline the notebook
// before them, the seeded raw the one protected row; a raw page at the
// root the source root, the wanted links of its pages counted in a second
// pass; the baseline's statistics refreshed after the commit; one log
// line; its own deadline, inside a statement or between two, server_busy,
// all rolled back, a warn line and no error. A full lint slot's 503 is
// TestUpgradeInterleavings' #17.
func TestUpgradeStart(t *testing.T) {
	logs := &syncBuffer{}
	tm := newAcmeTeamLogging(t, "member", "", nil, transfer.Batch{}, slog.New(slog.NewJSONHandler(logs, nil)))
	unchanged := func(nb string) {
		t.Helper()
		if n := count(t, tm.pool, `SELECT (SELECT count(*) FROM notebooks WHERE id = $1 AND upgrade_id IS NOT NULL)
			+ (SELECT count(*) FROM notebook_upgrades WHERE notebook_id = $1) + (SELECT count(*) FROM upgrade_nodes WHERE notebook_id = $1)`,
			nb); n != 0 {
			t.Errorf("a refused start left %d rows of an upgrade", n)
		}
	}
	t.Run("the codes in their order", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Refusals")
		page := tm.createPage(t, "alice", nb, "", "Page")
		status, answer := tm.startUpgrade(t, "alice", nb, `{"template_locale":"fr"}`)
		wantCode(t, "a language there is no template of", status, answer, cell{http.StatusUnprocessableEntity, "validation_failed"})
		status, answer = tm.startUpgrade(t, "alice", uuid.NewV7().String(), `{}`)
		wantCode(t, "no notebook", status, answer, cell{http.StatusNotFound, "notebook.not_found"})
		status, answer = tm.startUpgrade(t, "bob", nb, `{}`)
		wantCode(t, "an editor's", status, answer, cellForbidden())
		wiki := tm.createWith(t, tm.tokens["alice"], `{"name":"Field notes","mode":"llm_wiki"}`)
		status, answer = tm.startUpgrade(t, "alice", wiki.ID, `{}`)
		wantCode(t, "an LLM Wiki", status, answer, cell{http.StatusConflict, "notebook.already_llm_wiki"})

		release := tm.holdImportWorker(t)
		j := tm.started(t, importStep(t, "alice", nb, "", vaultZip(t, vaultFile{"a.md", "a"})))
		status, answer = tm.startUpgrade(t, "alice", nb, `{}`)
		wantCode(t, "an import queued", status, answer, cell{http.StatusConflict, "transfer.busy"})
		release()
		if ended := tm.endedJob(t, "alice", j.ID); ended.State != "succeeded" {
			t.Fatalf("the import = %+v, want succeeded", ended)
		}

		session := tm.openSession(t, "bob", page)
		status, answer = tm.startUpgrade(t, "alice", nb, `{}`)
		wantCode(t, "an edit session", status, answer, cell{http.StatusConflict, "wiki.upgrade_pages_locked"})
		if !strings.Contains(answer, `"locks":[{`) || !strings.Contains(answer, page) || !strings.Contains(answer, `"display_name":"bob"`) {
			t.Errorf("the locks named: %s", answer)
		}
		tm.exec(t, "DELETE FROM edit_sessions WHERE id = $1", session)

		tm.exec(t, "UPDATE nodes SET name = 'raw', name_key = 'raw' WHERE id = $1", page)
		asset := tm.upload(t, "alice", nb, "", "pic.png", pngHead).ID
		tm.exec(t, "UPDATE nodes SET name = 'AGENTS', name_key = 'agents' WHERE id = $1", asset)
		status, answer = tm.startUpgrade(t, "alice", nb, `{}`)
		wantCode(t, "an attachment AGENTS", status, answer, cell{http.StatusConflict, "wiki.upgrade_title_taken"})
		tm.exec(t, "UPDATE nodes SET name = 'pic.png', name_key = 'pic.png' WHERE id = $1", asset)
		tm.exec(t, "UPDATE nodes SET name = 'Page', name_key = 'page' WHERE id = $1", page)

		tm.exec(t, "UPDATE indexed_pages SET extractor = extractor - 1 WHERE node_id = $1", page)
		status, answer = tm.startUpgrade(t, "alice", nb, `{}`)
		wantCode(t, "a stale index", status, answer, cell{http.StatusConflict, "wiki.upgrade_stale_index"})
		tm.exec(t, "UPDATE indexed_pages SET extractor = extractor + 1 WHERE node_id = $1", page)
		unchanged(nb)
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name IN ('raw', 'AGENTS')", nb); n != 0 {
			t.Errorf("a refused start left %d pages of the template", n)
		}

		tm.upgraded(t, "alice", nb)
		status, answer = tm.startUpgrade(t, "alice", nb, `{}`)
		wantCode(t, "an upgrade under way", status, answer, cell{http.StatusConflict, "notebook.upgrading"})
		checkPages(t, tm.pool)
	})
	t.Run("it writes what the notebook lacks", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Lacking")
		tm.createPageWith(t, "alice", nb, "", "Page", "[[Nowhere]] and [[Nowhere]], [[Elsewhere]]\n")
		tm.createPage(t, "alice", nb, "", "Other")
		analyzed := queryStrings(t, tm.pool, "SELECT coalesce(last_analyze::text, '') FROM pg_stat_user_tables WHERE relname = 'upgrade_nodes'")[0]
		before := len((&mcpApp{logs: logs}).logLines(t))
		status, answer := tm.startUpgrade(t, "alice", nb, `{"template_locale":"zh-CN"}`)
		if status != http.StatusOK {
			t.Fatalf("start = %d %s", status, answer)
		}
		var u upgradeAnswer
		decodeAnswer(t, answer, &u)
		root := tm.rootOf(t, nb)
		if len(root) != 4 || root[2].Name != "raw" || root[3].Name != "AGENTS" || root[2].ID != u.SourceRootID ||
			len(u.SeededPageIDs) != 2 || u.SeededPageIDs[0] != root[2].ID || u.SeededPageIDs[1] != root[3].ID {
			t.Fatalf("the root %+v after the start %+v, want raw and AGENTS written last", root, u)
		}
		for i, p := range wiki.TemplatePages(wiki.ModeLLMWiki, wiki.LocaleZhCN) {
			if root[2+i].Content != p.Content {
				t.Errorf("%s holds %q, want the zh-CN template's", root[2+i].Name, root[2+i].Content)
			}
		}
		alice := tm.userOf(t, "alice")
		if u.State != "upgrading" || u.StartedBy.UserID != alice || u.StartedBy.DisplayName != "alice" ||
			u.Baseline.Nodes != 2 || u.Baseline.Pages != 2 || u.Baseline.Wanted != 2 {
			t.Errorf("the upgrade %+v, want alice's, upgrading, of two nodes, two pages and two wanted", u)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1 AND baseline", u.ID); n != 2 {
			t.Errorf("%d baseline rows, want the two pages before the start", n)
		}
		if got := queryStrings(t, tm.pool, "SELECT node_id::text FROM upgrade_nodes WHERE upgrade_id = $1 AND NOT baseline", u.ID); len(got) != 1 ||
			got[0] != u.SourceRootID {
			t.Errorf("protected rows %v, want the seeded raw's alone", got)
		}
		if got := queryStrings(t, tm.pool, `SELECT n.name || ':' || u.wanted FROM upgrade_nodes u JOIN nodes n ON n.id = u.node_id
			WHERE u.upgrade_id = $1 AND u.baseline ORDER BY n.name`, u.ID); strings.Join(got, ",") != "Other:0,Page:2" {
			t.Errorf("wanted by page %v, want Page's two distinct targets", got)
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM changesets WHERE notebook_id = $1 AND upgrade_id = $2 AND message = 'LLM Wiki 模板'`,
			nb, u.ID); n != 1 {
			t.Errorf("%d changesets of the seed of the upgrade, want one", n)
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM notebooks n JOIN notebook_upgrades w ON w.id = n.upgrade_id
			WHERE n.id = $1 AND n.upgrade_state = 'upgrading' AND w.locale = 'zh-CN' AND w.seeded_root AND w.seeded_agents_id = $2`,
			nb, u.SeededPageIDs[1]); n != 1 {
			t.Errorf("the notebook's columns and its upgrade's row do not agree")
		}
		if status, read := tm.getUpgrade(t, "alice", nb); status != http.StatusOK || read != answer {
			t.Errorf("GET the upgrade = %d %s, want the start's answer %s", status, read, answer)
		}
		if after := queryStrings(t, tm.pool, "SELECT coalesce(last_analyze::text, '') FROM pg_stat_user_tables WHERE relname = 'upgrade_nodes'")[0]; after == analyzed {
			t.Errorf("upgrade_nodes not analyzed after the start: %q", after)
		}
		lines := upgradeLines(t, logs, before, "upgrade started")
		if len(lines) != 1 || lines[0]["notebook_id"] != nb || lines[0]["upgrade_id"] != u.ID || lines[0]["user_id"] != alice ||
			lines[0]["baseline_nodes"] != 2.0 || lines[0]["baseline_pages"] != 2.0 || lines[0]["seeded_root"] != true ||
			lines[0]["seeded_agents"] != true || lines[0]["wanted"] != 2.0 || lines[0]["elapsed_ms"] == nil || strings.Contains(logs.String(), "Lacking") {
			t.Errorf("logged %v, want one line of the start by its ids", lines)
		}
		if l := upgradeLines(t, logs, before, "upgrade baseline not analyzed"); len(l) != 0 {
			t.Errorf("logged %v", l)
		}
		checkPages(t, tm.pool)
	})
	t.Run("a raw page is the source root", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Sourced")
		raw := tm.createPage(t, "alice", nb, "", "raw")
		tm.createPageWith(t, "alice", nb, raw, "Source", "[[Unwritten]]\n")
		tm.createPageWith(t, "alice", nb, "", "Notes", "[[Missing]]\n")
		u := tm.upgraded(t, "alice", nb)
		if u.SourceRootID != raw || len(u.SeededPageIDs) != 1 || u.Baseline.Nodes != 3 || u.Baseline.Wanted != 1 {
			t.Errorf("the upgrade %+v, want raw its source root, AGENTS alone written, one wanted outside raw", u)
		}
		if got := queryStrings(t, tm.pool, `SELECT n.name || ':' || u.wanted FROM upgrade_nodes u JOIN nodes n ON n.id = u.node_id
			WHERE u.upgrade_id = $1 AND u.baseline ORDER BY n.name`, u.ID); strings.Join(got, ",") != "Notes:1,Source:1,raw:0" {
			t.Errorf("wanted by page %v, want raw's page Source counted too (the second pass)", got)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1 AND NOT baseline", u.ID); n != 0 {
			t.Errorf("%d protected rows, want none: raw was the notebook's", n)
		}
		checkPages(t, tm.pool)
	})
	// Its own deadline, inside a statement and between two: 503 at once,
	// Retry-After: 1, one warn line and no error, nothing written.
	t.Run("its own deadline", func(t *testing.T) {
		logs := &syncBuffer{}
		var slow *slowCapture
		tm := newAcmeTeamOn(t, "", "", nil, &wiring{wiki: func(d *wiki.Deps) {
			slow = &slowCapture{UpgradePages: d.Upgrades}
			d.Upgrades, d.StartTimeout = slow, 300*time.Millisecond
		}}, slog.New(slog.NewJSONHandler(logs, nil)))
		nb := tm.openNotebook(t, "alice", "Slow")
		tm.createPage(t, "alice", nb, "", "Page")
		timedOut := func(t *testing.T, what string, before int, res *http.Response, body []byte) {
			t.Helper()
			wantCode(t, what, res.StatusCode, string(body), cell{http.StatusServiceUnavailable, "server_busy"})
			if res.Header.Get("Retry-After") != "1" {
				t.Errorf("%s: Retry-After %q, want 1", what, res.Header.Get("Retry-After"))
			}
			unchanged := count(t, tm.pool, `SELECT (SELECT count(*) FROM notebooks WHERE id = $1 AND upgrade_id IS NOT NULL)
				+ (SELECT count(*) FROM notebook_upgrades) + (SELECT count(*) FROM upgrade_nodes)
				+ (SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name IN ('raw', 'AGENTS'))`, nb)
			if unchanged != 0 {
				t.Errorf("%s left %d rows", what, unchanged)
			}
			lines := upgradeLines(t, logs, before, "upgrade start timed out")
			if len(lines) != 1 || lines[0]["level"] != "WARN" || lines[0]["notebook_id"] != nb || lines[0]["timeout_ms"] != 300.0 {
				t.Errorf("%s logged %v, want one warn line of the timeout", what, lines)
			}
			for _, l := range (&mcpApp{logs: logs}).logLines(t)[before:] {
				if l["level"] == "ERROR" {
					t.Errorf("an error line: %v", l)
				}
			}
			checkPages(t, tm.pool)
		}
		start := func(t *testing.T) (*http.Response, []byte) {
			t.Helper()
			req := newRequest(t, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/upgrade", tm.tokens["alice"], []byte(`{}`))
			res, body := sendRequest(t, req)
			tm.contract.CheckResponse(t, req, res)
			return res, body
		}
		// Waiting for the notebook's row a write of the test holds: pgx
		// closes the connection, the rollback fails, and only the start's
		// ctx says it was its deadline (decision 227).
		t.Run("inside a statement", func(t *testing.T) {
			before := len((&mcpApp{logs: logs}).logLines(t))
			holder := tm.holdNotebook(t, nb)
			res, body := start(t)
			if err := holder.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
			timedOut(t, "a start past its deadline inside a statement", before, res, body)
		})
		// Capture waiting in Go for its transaction's end: the error the
		// start's fn answers wraps the deadline.
		t.Run("between statements", func(t *testing.T) {
			before := len((&mcpApp{logs: logs}).logLines(t))
			slow.armed.Store(true)
			defer slow.armed.Store(false)
			res, body := start(t)
			if !slow.waited.Load() {
				t.Error("the start did not wait in Capture")
			}
			timedOut(t, "a start past its deadline between statements", before, res, body)
		})
	})
}

// slowCapture is the page module's upgrade port whose Capture, once armed,
// waits for its transaction's end: the start's deadline runs out in its
// transaction, between two statements.
type slowCapture struct {
	wiki.UpgradePages
	armed, waited atomic.Bool
}

func (s *slowCapture) Capture(ctx context.Context, upgradeID, notebookID uuid.UUID) (wiki.UpgradeBaseline, error) {
	if !s.armed.Load() {
		return s.UpgradePages.Capture(ctx, upgradeID, notebookID)
	}
	s.waited.Store(true)
	<-ctx.Done()
	return wiki.UpgradeBaseline{}, ctx.Err()
}

// An upgrade's states as getUpgrade and the notebook's reads answer them
// (M11/P1 design 3.14): started, upgrading, its start's answer; set by SQL
// reviewing, then rolling back, each with its step's fields; once it
// ended, wiki.upgrade_not_found; an editor's read forbidden. The notebook
// read and listed carries the upgrade under way, and none once it ended.
func TestUpgradeStates(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Notes")
	tm.createPage(t, "alice", nb, "", "Page")
	status, answer := tm.getUpgrade(t, "alice", nb)
	wantCode(t, "no upgrade yet", status, answer, cell{http.StatusNotFound, "wiki.upgrade_not_found"})
	u := tm.upgraded(t, "alice", nb)
	carries := func(state string) {
		t.Helper()
		type upgradeOf struct {
			ID    string `json:"id"`
			State string `json:"state"`
		}
		var one struct {
			Upgrade *upgradeOf `json:"upgrade"`
		}
		var list struct {
			Data []struct {
				ID      string     `json:"id"`
				Upgrade *upgradeOf `json:"upgrade"`
			} `json:"data"`
		}
		_, read := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb, tm.tokens["bob"], "")
		decodeAnswer(t, read, &one)
		_, listed := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/workspaces/acme/notebooks", tm.tokens["bob"], "")
		decodeAnswer(t, listed, &list)
		got := []*upgradeOf{one.Upgrade}
		for _, n := range list.Data {
			if n.ID == nb {
				got = append(got, n.Upgrade)
			}
		}
		for i, g := range got {
			if state == "" && g != nil || state != "" && (g == nil || g.ID != u.ID || g.State != state) || len(got) != 2 {
				t.Errorf("read %d of the notebook carries %+v, want its upgrade %q", i, g, state)
			}
		}
	}
	carries("upgrading")
	status, answer = tm.getUpgrade(t, "bob", nb)
	wantCode(t, "an editor's read", status, answer, cellForbidden())
	bob := tm.userOf(t, "bob")
	tm.exec(t, `UPDATE notebooks SET upgrade_state = 'reviewing' WHERE id = $1`, nb)
	tm.exec(t, `UPDATE notebook_upgrades SET state = 'reviewing', submission_id = gen_random_uuid(), submitted_by_id = $2,
		submitted_at = now() WHERE id = $1`, u.ID, bob)
	status, answer = tm.getUpgrade(t, "alice", nb)
	if status != http.StatusOK || !strings.Contains(answer, `"state":"reviewing"`) ||
		!strings.Contains(answer, `"submitted_by":{"display_name":"bob","user_id":"`+bob+`"}`) || strings.Contains(answer, `"submitted_at":null`) {
		t.Errorf("reviewing = %d %s", status, answer)
	}
	carries("reviewing")
	job := uuid.NewV7().String()
	tm.exec(t, `UPDATE notebooks SET upgrade_state = 'rolling_back' WHERE id = $1`, nb)
	tm.exec(t, `UPDATE notebook_upgrades SET state = 'rolling_back', ending = 'rejected', rollback_job_id = $2 WHERE id = $1`, u.ID, job)
	status, answer = tm.getUpgrade(t, "alice", nb)
	if status != http.StatusOK || !strings.Contains(answer, `"state":"rolling_back"`) || !strings.Contains(answer, `"ending":"rejected"`) ||
		!strings.Contains(answer, `"rollback_job_id":"`+job+`"`) || !strings.Contains(answer, `"rollback_left":null`) {
		t.Errorf("rolling back = %d %s", status, answer)
	}
	carries("rolling_back")
	tm.endUpgrade(t, nb)
	status, answer = tm.getUpgrade(t, "alice", nb)
	wantCode(t, "an upgrade ended", status, answer, cell{http.StatusNotFound, "wiki.upgrade_not_found"})
	carries("")
	checkPages(t, tm.pool)
}

// The start's mode frame (M11/P1 design 3.11): a start reaches, as the
// pages frame of the template's pages and then one mode frame of the
// notebook's ids, each stream that sees the notebook,
// its admin's and a reader's, and no other, a member's who does not see
// it; a write of a marker page every stream sees comes next in each. The
// composition root hands the start the mode's subscribers: without them,
// no mode frame comes.
func TestUpgradeStatesReachTheStreams(t *testing.T) {
	tm := newAcmeTeam(t, "member", "member")
	nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Notes","workspace_access":"none"}`).ID
	tm.createPage(t, "alice", nb, "", "Page")
	tm.addNotebookMember(t, "alice", nb, "bob", "reader")
	eng := tm.openNotebook(t, "alice", "Eng")
	marker := tm.createPage(t, "alice", eng, "", "Marker")
	tm.settle(t, eng, marker)
	alice := openStream(t, tm.base, "alice", tm.tokens["alice"])
	bob := openStream(t, tm.base, "bob", tm.tokens["bob"])
	carol := openStream(t, tm.base, "carol", tm.tokens["carol"])

	u := tm.upgraded(t, "alice", nb)

	acme := queryStrings(t, tm.pool, "SELECT id::text FROM workspaces WHERE slug = 'acme'")
	for _, s := range []*eventStream{alice, bob} {
		// The seed's unit wrote raw and AGENTS before the mode's
		// subscribers were told, in the start's transaction.
		if p := s.expect(t, "pages"); !strings.Contains(p.data, u.SeededPageIDs[0]) || !strings.Contains(p.data, u.SeededPageIDs[1]) {
			t.Errorf("%s's stream: pages %s, want the template's two", s.name, p.data)
		}
		f := s.expect(t, "mode")
		apitest.Load(t).CheckSchema(t, "EventMode", []byte(f.data))
		if len(acme) != 1 || f.field(t, "workspace_id") != acme[0] || f.field(t, "notebook_id") != nb {
			t.Errorf("%s's stream: mode %s, want acme's %s", s.name, f.data, nb)
		}
	}
	tm.quiet(t, eng, marker, alice, bob, carol)
}

// A notebook's deletion takes its upgrade with it (M11/P1 design 3.15):
// its row soft-deleted at the deletion's time, its baseline's and
// protected rows deleted; the purge later takes the row. The composition
// root registers the wiki module's subscriber: without it, the row stays
// under way.
func TestUpgradeDeletion(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	nb := tm.openNotebook(t, "alice", "Notes")
	tm.createPage(t, "alice", nb, "", "Page")
	u := tm.upgraded(t, "alice", nb)
	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/notebooks/"+nb, tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("DELETE the notebook being upgraded = %d %s, want 204", status, answer)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM notebook_upgrades u JOIN notebooks n ON n.id = u.notebook_id
		WHERE u.id = $1 AND u.deleted_at = n.deleted_at AND u.updated_by_id = n.updated_by_id`, u.ID); n != 1 {
		t.Errorf("the upgrade's row of the deleted notebook: %d soft-deleted with it, want one", n)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM upgrade_nodes WHERE upgrade_id = $1", u.ID); n != 0 {
		t.Errorf("%d rows of the deleted notebook's upgrade, want none", n)
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
}
