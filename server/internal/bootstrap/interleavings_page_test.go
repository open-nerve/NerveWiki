package bootstrap

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The interleavings of the pages' lock protocol (M4/P1 design 3.13; M4/P2 design 3.8),
// harnessed as the workspace's (interleavings_workspace_test.go): a page
// write shares its workspace's row, then locks its notebook's; a tree's
// write locks it FOR NO KEY UPDATE, as the notebook's deletion does. The
// test holds the row both wait for, and they run in the order they came.
// Each ends by checking the pages' invariant and the notebooks'.

// checkPages fails t when the pages break an invariant (M4 design 4): a
// node not deleted under a deleted parent, or under an attachment (M7/P2
// design 3.3); two siblings not deleted with one title key, which the
// unique index refuses too, checked in case it changes; a page not deleted
// without exactly one content not deleted, or whose content's revision is
// not its latest version's; a page deeper than ten levels of pages (an
// attachment is no level), or on a chain that loops; a deleted page with a content, a
// version or an item not deleted, which would keep the purge from it, or
// a page not deleted with one deleted, which a restore left behind; a
// deleted node that is a trash item's root but for one whose parent was
// deleted at its time, or with its notebook, or the other way round, or a
// root whose counts are not its group's, the nodes deleted at its time
// from it down (M8/P2 design 3.3), or, once the purge has reached the
// group, less than what is left of it: the purge takes a page's content
// before any node, then the nodes leaves first, the root last, a batch a
// statement, so a page root without its content at its time is a group
// it may be midway through (M8/P2A fix check 1, N-1). Its limit: a purge
// that skips the root's content, which a lock holds, while it takes the
// group below leaves a valid state this reports; no test reaches it (M8/P2A
// fix check 2, N-c); an
// edit session of a page that is not one not deleted of its notebook
// (M4/P4 design 3.2); two sessions of a page alive at the database's time,
// which the edit lock refuses (M5/P1 design 3.8); a live LLM Wiki without
// its source root, a live page of its own, or its root's AGENTS page (M10
// design 4.4, 4.9: the template is written with the notebook, and neither
// page goes while it is one), or a live root node of an LLM Wiki whose
// title key is reserved for its virtual files; a source hash missing or wrong
// on a live eligible descendant, or left on a root, empty page, ordinary
// notebook, deleted node or outside the source area (M10/P2 design 3.7).
// Missing eligible page or attachment metadata is a violation too.
// The links' invariant and
// the search's (checkSearch, M8/P5 design 3.10) are checked with it.
func checkPages(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, v := range pageViolations(t, pool) {
		t.Error(v)
	}
	checkLinks(t, pool)
	checkSearch(t, pool)
}

// llmWikiGap is the rule of pageViolations that a live LLM Wiki has its
// source root and its root's AGENTS page.
const llmWikiGap = "live LLM Wikis without their source root or root AGENTS page"

// pageViolations are checkPages' rules that pool breaks, each a sentence
// with its count, by the rule (M10/P1 design 3.14): a test seeds a rule's
// counterexample and finds its sentence.
func pageViolations(t testing.TB, pool *pgxpool.Pool) []string {
	t.Helper()
	rules := map[string]string{
		sourceHashGap: sourceHashViolationSQL,
		"pages under a deleted parent": `SELECT count(*) FROM nodes c JOIN nodes p ON p.id = c.parent_id
			WHERE c.deleted_at IS NULL AND p.deleted_at IS NOT NULL`,
		"pages under an attachment": `SELECT count(*) FROM nodes c JOIN nodes p ON p.id = c.parent_id
			WHERE c.deleted_at IS NULL AND p.kind = 'asset'`,
		"pages with a sibling's title key": `SELECT count(*) FROM (SELECT 1 FROM nodes WHERE deleted_at IS NULL
			GROUP BY notebook_id, parent_id, name_key HAVING count(*) > 1) twins`,
		"pages without exactly one content": `SELECT count(*) FROM nodes n WHERE n.deleted_at IS NULL AND n.kind = 'page'
			AND (SELECT count(*) FROM page_contents c WHERE c.node_id = n.id AND c.deleted_at IS NULL) <> 1`,
		"pages whose content is not its latest version": `SELECT count(*) FROM page_contents c WHERE c.deleted_at IS NULL
			AND c.revision IS DISTINCT FROM (SELECT max(r.revision) FROM page_revisions r WHERE r.node_id = c.node_id)`,
		// A version goes on from the one before it: FindVersion reads N in the
		// first at or above it (M8/P1 review A).
		"pages whose versions do not chain": `SELECT count(DISTINCT node_id) FROM (SELECT r.node_id, r.base_revision,
				lag(r.revision) OVER (PARTITION BY r.node_id ORDER BY r.revision) AS prior
			FROM page_revisions r JOIN nodes n ON n.id = r.node_id WHERE n.deleted_at IS NULL) v
			WHERE base_revision IS DISTINCT FROM prior`,
		"pages deleted, with what follows it not deleted": `SELECT count(*) FROM nodes n WHERE n.deleted_at IS NOT NULL AND (
			EXISTS (SELECT 1 FROM page_contents c WHERE c.node_id = n.id AND c.deleted_at IS NULL)
			OR EXISTS (SELECT 1 FROM page_revisions r WHERE r.node_id = n.id AND r.deleted_at IS NULL)
			OR EXISTS (SELECT 1 FROM changeset_items i WHERE i.node_id = n.id AND i.deleted_at IS NULL))`,
		"pages not deleted, with what follows it deleted": `SELECT count(*) FROM nodes n WHERE n.deleted_at IS NULL AND (
			EXISTS (SELECT 1 FROM page_contents c WHERE c.node_id = n.id AND c.deleted_at IS NOT NULL)
			OR EXISTS (SELECT 1 FROM page_revisions r WHERE r.node_id = n.id AND r.deleted_at IS NOT NULL)
			OR EXISTS (SELECT 1 FROM changeset_items i WHERE i.node_id = n.id AND i.deleted_at IS NOT NULL))`,
		"pages deleted, a trash item's root or not as its parent and notebook make it": `SELECT count(*) FROM nodes n
			JOIN notebooks b ON b.id = n.notebook_id LEFT JOIN nodes p ON p.id = n.parent_id
			WHERE n.deleted_at IS NOT NULL AND n.trash_root <> ((p.id IS NULL OR p.deleted_at IS DISTINCT FROM n.deleted_at)
				AND b.deleted_at IS DISTINCT FROM n.deleted_at)`,
		"pages a trash item's root, its pages, attachments or height not its group's, or less than what the purge left": `WITH RECURSIVE g AS (
				SELECT r.id AS root, r.id, r.kind, 1 AS level, r.deleted_at FROM nodes r WHERE r.trash_root
				UNION ALL
				SELECT g.root, n.id, n.kind, g.level + 1, g.deleted_at FROM g JOIN nodes n ON n.parent_id = g.id
					AND n.deleted_at = g.deleted_at
			) SELECT count(*) FROM nodes r JOIN (SELECT root, count(*) FILTER (WHERE kind = 'page') AS pages,
					count(*) FILTER (WHERE kind = 'asset') AS assets,
					coalesce(max(level) FILTER (WHERE kind = 'page'), 0) AS height
				FROM g GROUP BY root) c ON c.root = r.id
			WHERE CASE WHEN r.kind = 'page' AND NOT EXISTS (SELECT 1 FROM page_contents x WHERE x.node_id = r.id AND x.deleted_at = r.deleted_at)
				THEN NOT coalesce(r.trash_pages >= c.pages AND r.trash_assets >= c.assets AND r.trash_height >= c.height, false)
				ELSE (r.trash_pages, r.trash_assets, r.trash_height) IS DISTINCT FROM (c.pages::integer, c.assets::integer, c.height) END`,
		"pages deeper than ten levels of pages, or in a loop": `WITH RECURSIVE up AS (
				SELECT id AS start, parent_id, 1 AS depth FROM nodes WHERE deleted_at IS NULL AND kind = 'page'
				UNION ALL
				SELECT u.start, p.parent_id, u.depth + 1 FROM up u JOIN nodes p ON p.id = u.parent_id WHERE u.depth < 11
			) SELECT count(DISTINCT start) FROM up WHERE parent_id IS NOT NULL AND depth >= 10`,
		"pages with an edit session, deleted or in another notebook": `SELECT count(*) FROM edit_sessions s
			WHERE NOT EXISTS (SELECT 1 FROM nodes n WHERE n.id = s.node_id AND n.notebook_id = s.notebook_id
				AND n.kind = 'page' AND n.deleted_at IS NULL)`,
		"pages with two alive edit sessions": `SELECT count(*) FROM (SELECT 1 FROM edit_sessions
			WHERE ended_reason IS NULL AND expires_at > now() GROUP BY node_id HAVING count(*) > 1) locked`,
		// A revert's changeset reverts one of its notebook, or one the purge
		// took; a page's deletion holds the revision it deleted, which the
		// revert of it compares (M8/P3 design 3.12).
		"pages reverted by a changeset of another notebook": `SELECT count(*) FROM changesets r
			JOIN changesets c ON c.id = r.reverts_id WHERE r.notebook_id <> c.notebook_id`,
		"pages deleted by an item without its revision": `SELECT count(*) FROM changeset_items i JOIN nodes n ON n.id = i.node_id
			WHERE i.after_name IS NULL AND n.kind = 'page' AND i.revision IS NULL`,
		// name_key is shared.TitleKey's: AGENTS's is agents.
		llmWikiGap: `SELECT count(*) FROM notebooks n
			WHERE n.mode = 'llm_wiki' AND n.deleted_at IS NULL AND (
				NOT EXISTS (SELECT 1 FROM nodes r WHERE r.id = n.source_root_id AND r.notebook_id = n.id
					AND r.kind = 'page' AND r.deleted_at IS NULL)
				OR NOT EXISTS (SELECT 1 FROM nodes a WHERE a.notebook_id = n.id AND a.parent_id IS NULL
					AND a.name_key = 'agents' AND a.kind = 'page' AND a.deleted_at IS NULL))`,
		"live LLM Wikis' root nodes with a reserved title key": `SELECT count(*) FROM nodes x JOIN notebooks n ON n.id = x.notebook_id
			WHERE n.mode = 'llm_wiki' AND n.deleted_at IS NULL AND x.deleted_at IS NULL AND x.parent_id IS NULL
				AND x.name_key IN ('index', 'log', 'index.md', 'log.md')`,
	}
	var found []string
	for _, what := range slices.Sorted(maps.Keys(rules)) {
		if n := count(t, pool, rules[what]); n != 0 {
			found = append(found, fmt.Sprintf("%d %s, want none", n, what))
		}
	}
	return found
}

// hasLLMWikiGap reports whether violations hold llmWikiGap's sentence.
func hasLLMWikiGap(violations []string) bool {
	return slices.ContainsFunc(violations, func(v string) bool { return strings.HasSuffix(v, " "+llmWikiGap+", want none") })
}

// pageViolations finds each gap of a live LLM Wiki's template and no
// other (M10/P1 design 3.14): an LLM Wiki created through the API breaks
// no rule; each row changes it through SQL, looks for the rule's sentence
// and puts it back; a soft deletion breaks other rules too, so a row asks
// only whether the sentence is among them. Last, the LLM Wiki's deletion,
// pages and all, breaks none: the rule reads live notebooks.
func TestPageViolationsCatchEachLLMWikiGap(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	wiki := tm.createWith(t, tm.tokens["alice"], `{"name":"Field notes","mode":"llm_wiki"}`)
	other := tm.createWith(t, tm.tokens["alice"], `{"name":"Other notes","mode":"llm_wiki"}`)
	root := tm.rootOf(t, wiki.ID)
	if len(root) != 2 || root[0].ID != *wiki.SourceRootID || root[1].Name != "AGENTS" {
		t.Fatalf("the LLM Wiki's root = %+v, want raw, its source root, then AGENTS", root)
	}
	raw, agents := root[0].ID, root[1].ID
	if v := pageViolations(t, tm.pool); len(v) != 0 {
		t.Fatalf("an LLM Wiki as created breaks %q, want nothing", v)
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tm.pool.Exec(context.Background(), query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	for _, tt := range []struct {
		name         string
		change, undo func()
		want         bool
	}{
		{"raw deleted",
			func() { exec("UPDATE nodes SET deleted_at = now() WHERE id = $1", raw) },
			func() { exec("UPDATE nodes SET deleted_at = NULL WHERE id = $1", raw) }, true},
		{"AGENTS deleted",
			func() { exec("UPDATE nodes SET deleted_at = now() WHERE id = $1", agents) },
			func() { exec("UPDATE nodes SET deleted_at = NULL WHERE id = $1", agents) }, true},
		{"raw an attachment",
			func() { exec("UPDATE nodes SET kind = 'asset' WHERE id = $1", raw) },
			func() { exec("UPDATE nodes SET kind = 'page' WHERE id = $1", raw) }, true},
		{"AGENTS an attachment",
			func() { exec("UPDATE nodes SET kind = 'asset' WHERE id = $1", agents) },
			func() { exec("UPDATE nodes SET kind = 'page' WHERE id = $1", agents) }, true},
		{"AGENTS under raw",
			func() { exec("UPDATE nodes SET parent_id = $2 WHERE id = $1", agents, raw) },
			func() { exec("UPDATE nodes SET parent_id = NULL WHERE id = $1", agents) }, true},
		{"the source root another notebook's page",
			func() { exec("UPDATE notebooks SET source_root_id = $2 WHERE id = $1", wiki.ID, *other.SourceRootID) },
			func() { exec("UPDATE notebooks SET source_root_id = $2 WHERE id = $1", wiki.ID, raw) }, true},
		{"raw deleted in a standard notebook",
			func() {
				exec("UPDATE nodes SET deleted_at = now() WHERE id = $1", raw)
				exec("UPDATE notebooks SET mode = 'standard', source_root_id = NULL WHERE id = $1", wiki.ID)
			},
			func() {
				exec("UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2 WHERE id = $1", wiki.ID, raw)
				exec("UPDATE nodes SET deleted_at = NULL WHERE id = $1", raw)
			}, false},
	} {
		tt.change()
		if got := pageViolations(t, tm.pool); hasLLMWikiGap(got) != tt.want {
			t.Errorf("%s: the violations %q hold %q = %t, want %t", tt.name, got, llmWikiGap, !tt.want, tt.want)
		}
		tt.undo()
		if v := pageViolations(t, tm.pool); len(v) != 0 {
			t.Fatalf("%s put back breaks %q, want nothing", tt.name, v)
		}
	}
	standard := tm.createWith(t, tm.tokens["alice"], `{"name":"Standard notes"}`)
	rootPage := tm.createPage(t, "alice", wiki.ID, "", "Notes")
	nestedPage := tm.createPage(t, "alice", wiki.ID, rootPage, "Nested")
	standardPage := tm.createPage(t, "alice", standard.ID, "", "Notes")
	const reserved = "live LLM Wikis' root nodes with a reserved title key"
	for _, tt := range []struct {
		label, id, name string
		want            bool
	}{
		{"reserved at the LLM Wiki root", rootPage, "Notes", true},
		{"reserved under a page", nestedPage, "Nested", false},
		{"reserved at a standard root", standardPage, "Notes", false},
	} {
		exec("UPDATE nodes SET name = 'Index', name_key = 'index' WHERE id = $1", tt.id)
		got := pageViolations(t, tm.pool)
		found := slices.Contains(got, "1 "+reserved+", want none")
		if found != tt.want {
			t.Errorf("%s: violations %q hold %q = %t, want %t", tt.label, got, reserved, found, tt.want)
		}
		exec("UPDATE nodes SET name = $2, name_key = lower($2) WHERE id = $1", tt.id, tt.name)
		if v := pageViolations(t, tm.pool); len(v) != 0 {
			t.Fatalf("%s put back breaks %q, want nothing", tt.label, v)
		}
	}
	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/notebooks/"+wiki.ID, tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("DELETE the LLM Wiki = %d %s, want 204", status, answer)
	}
	if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", wiki.ID) != 0 {
		t.Fatal("the deleted LLM Wiki keeps live pages, want none")
	}
	if v := pageViolations(t, tm.pool); len(v) != 0 {
		t.Errorf("a deleted LLM Wiki breaks %q, want nothing", v)
	}
}

// openNotebook creates a notebook named name in acme as by, open to acme's
// members to edit, and returns its id.
func (tm acmeTeam) openNotebook(t *testing.T, by, name string) string {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/notebooks", tm.tokens[by],
		`{"name":"`+name+`","workspace_access":"editor"}`)
	if status != http.StatusCreated {
		t.Fatalf("create %s as %s = %d %s", name, by, status, body)
	}
	return idOf(t, answer{body: body})
}

// createPage creates a page titled title in the notebook id as by, under
// the page parent, or at the root when it is "", checks the invariant, and
// returns its id.
func (tm acmeTeam) createPage(t *testing.T, by, notebookID, parent, title string) string {
	t.Helper()
	parentID := "null"
	if parent != "" {
		parentID = `"` + parent + `"`
	}
	status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+notebookID+"/pages", tm.tokens[by],
		`{"parent_id":`+parentID+`,"title":"`+title+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("create %s as %s = %d %s", title, by, status, body)
	}
	checkPages(t, tm.pool)
	return idOf(t, answer{body: body})
}

// pageCreation is by's creation of a page titled title at the root of the
// notebook id.
func pageCreation(by, notebookID, title string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+notebookID+"/pages", `{"parent_id":null,"title":"`+title+`"}`)
}

// deletedWithNotebook counts the pages of the notebook id deleted at its
// time, with their contents.
func (tm acmeTeam) deletedWithNotebook(t *testing.T, notebookID string) (pages, contents int) {
	t.Helper()
	pages = count(t, tm.pool, `SELECT count(*) FROM nodes x JOIN notebooks n ON n.id = x.notebook_id
		WHERE n.id = $1 AND x.deleted_at = n.deleted_at`, notebookID)
	contents = count(t, tm.pool, `SELECT count(*) FROM page_contents c JOIN nodes x ON x.id = c.node_id JOIN notebooks n ON n.id = x.notebook_id
		WHERE n.id = $1 AND c.deleted_at = n.deleted_at`, notebookID)
	return pages, contents
}

// Interleaving 30: deleting a notebook and creating or renaming a page of
// it. The deletion first: the write finds the notebook deleted once it has
// its row: 404, nothing written. The write first: the deletion takes its
// page with the notebook, at the notebook's time.
func TestDeletingANotebookAndWritingAPage(t *testing.T) {
	for _, tt := range []struct {
		name     string
		write    func(nb, page string) step
		notFound string
		written  int // the pages the write leaves when first
	}{
		{"creating", func(nb, _ string) step { return pageCreation("bob", nb, "Late") }, "notebook.not_found", 2},
		{"renaming", func(_, page string) step {
			return request("bob", http.MethodPatch, "/api/v0/nodes/"+page, `{"name":"Renamed"}`)
		}, "page.not_found", 1},
	} {
		t.Run(tt.name+", the deletion first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			page := tm.createPage(t, "alice", nb, "", "Notes")
			deleted, written := tm.interleaveOn(t, notebookRow(nb), request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""),
				tt.write(nb, page))
			if !deleted.is(http.StatusNoContent, "") || !written.is(http.StatusNotFound, tt.notFound) {
				t.Errorf("DELETE the notebook = %d, then %s = %d %s; want 204, then 404 %s", deleted.status, tt.name, written.status,
					written.code, tt.notFound)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE name <> 'Notes'"); n != 0 {
				t.Errorf("%d pages written after the deletion, want none", n)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
		t.Run(tt.name+", the write first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			page := tm.createPage(t, "alice", nb, "", "Notes")
			written, deleted := tm.interleaveOn(t, notebookRow(nb), tt.write(nb, page),
				request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""))
			if written.status/100 != 2 || !deleted.is(http.StatusNoContent, "") {
				t.Errorf("%s = %d %s, then DELETE the notebook = %d; want 2xx, then 204", tt.name, written.status, written.code, deleted.status)
			}
			if pages, contents := tm.deletedWithNotebook(t, nb); pages != tt.written || contents != tt.written {
				t.Errorf("%d pages and %d contents deleted with the notebook, want %d of each", pages, contents, tt.written)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 31: deleting a workspace and creating a page in it. The
// deletion first: the creation finds the workspace deleted once it shares
// its row: 404, no page. The creation first: the deletion takes the page
// with its notebook, at the workspace's time.
func TestDeletingAWorkspaceAndCreatingAPage(t *testing.T) {
	deletion := request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "")
	t.Run("the deletion first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		deleted, created := tm.interleave(t, deletion, pageCreation("bob", nb, "Late"))
		if !deleted.is(http.StatusNoContent, "") || !created.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("DELETE acme = %d, then POST a page = %d %s; want 204, then 404 notebook.not_found", deleted.status, created.status, created.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes"); n != 0 {
			t.Errorf("%d pages, want none", n)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the creation first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		created, deleted := tm.interleave(t, pageCreation("bob", nb, "Late"), deletion)
		if !created.is(http.StatusCreated, "") || !deleted.is(http.StatusNoContent, "") {
			t.Errorf("POST a page = %d %s, then DELETE acme = %d; want 201, then 204", created.status, created.code, deleted.status)
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM nodes x JOIN notebooks n ON n.id = x.notebook_id
			JOIN workspaces w ON w.id = n.workspace_id WHERE x.deleted_at = w.deleted_at`); n != 1 {
			t.Errorf("%d pages deleted with acme, want the new one", n)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// sharedNotebookRow is the row of notebook id as a page write that keeps
// the tree holds it, FOR SHARE: a step waits for it only if it locks the
// tree.
func sharedNotebookRow(id string) held {
	return held{"notebooks", "SELECT 1 FROM notebooks WHERE id = '" + id + "' FOR SHARE"}
}

// Interleaving 32: two pages of one title created at once under the same
// parent. The test holds the notebook's row FOR SHARE, so each creation
// waits for it only because it locks the tree; the second finds the
// first's title under that lock: 409, and the unique index is never
// reached.
func TestCreatingTwoPagesOfOneTitle(t *testing.T) {
	orders(t, "alice", "bob", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		a, b := tm.interleaveOn(t, sharedNotebookRow(nb), pageCreation(first, nb, "Same"), pageCreation(second, nb, "SAME"))
		if !a.is(http.StatusCreated, "") || !b.is(http.StatusConflict, "page.title_taken") {
			t.Errorf("%s created: %d %s; %s created: %d %s; want 201, then 409 page.title_taken", first, a.status, a.code, second, b.status, b.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 33: bob creates a page while alice removes him from acme.
// The removal first: the creation decides under acme's lock that he sees
// the notebook no more: 404, no page. The creation first: the page stays,
// and he sees it no more.
func TestCreatingAPageAndRemovingTheWriterFromTheWorkspace(t *testing.T) {
	t.Run("the removal first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		removed, created := tm.interleave(t, tm.removal("alice", "bob"), pageCreation("bob", nb, "Late"))
		if !removed.is(http.StatusNoContent, "") || !created.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("remove bob = %d, then his POST a page = %d %s; want 204, then 404 notebook.not_found", removed.status, created.status, created.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes"); n != 0 {
			t.Errorf("%d pages, want none", n)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the creation first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		created, removed := tm.interleave(t, pageCreation("bob", nb, "Late"), tm.removal("alice", "bob"))
		if !created.is(http.StatusCreated, "") || !removed.is(http.StatusNoContent, "") {
			t.Errorf("his POST a page = %d %s, then remove bob = %d; want 201, then 204", created.status, created.code, removed.status)
		}
		page := idOf(t, created)
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", page); n != 1 {
			t.Errorf("bob's page is gone, want it kept")
		}
		if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+page, tm.tokens["bob"], ""); status != http.StatusNotFound {
			t.Errorf("bob reads his page after his removal = %d %s, want 404", status, answer)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// The tree's writes as the interleavings send them: by's move of the node
// id under parent ("" for the root), last there; its rename; its deletion;
// a creation under parent.
func nodeMove(by, id, parent string) step {
	parentID := "null"
	if parent != "" {
		parentID = `"` + parent + `"`
	}
	return request(by, http.MethodPost, "/api/v0/nodes/"+id+"/move", `{"parent_id":`+parentID+`}`)
}

func nodeRename(by, id, name string) step {
	return request(by, http.MethodPatch, "/api/v0/nodes/"+id, `{"name":"`+name+`"}`)
}

func nodeDeletion(by, id string) step { return request(by, http.MethodDelete, "/api/v0/nodes/"+id, "") }

func childCreation(by, notebookID, parent, title string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+notebookID+"/pages", `{"parent_id":"`+parent+`","title":"`+title+`"}`)
}

// Interleaving 34: A moved under B while B is moved under A. Each alone is
// no cycle; the second finds the first's move under the notebook's lock:
// 409 page.cycle, and the tree has no loop.
func TestMovingTwoPagesUnderEachOther(t *testing.T) {
	orders(t, "A", "B", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := map[string]string{"A": tm.createPage(t, "alice", nb, "", "A"), "B": tm.createPage(t, "alice", nb, "", "B")}
		a, b := tm.interleaveOn(t, sharedNotebookRow(nb), nodeMove("alice", ids[first], ids[second]),
			nodeMove("bob", ids[second], ids[first]))
		if !a.is(http.StatusOK, "") || !b.is(http.StatusConflict, "page.cycle") {
			t.Errorf("%s under %s = %d %s, then %s under %s = %d %s; want 200, then 409 page.cycle", first, second, a.status, a.code,
				second, first, b.status, b.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 35: a move that takes a subtree deeper while a page is
// created at its bottom. Each alone stays within ten levels, both would
// not: the second finds the first's under the notebook's lock, 409
// page.too_deep.
func TestMovingASubtreeDeeperAndCreatingAtItsBottom(t *testing.T) {
	setup := func(t *testing.T) (tm acmeTeam, nb, eighth, top, bottom string) {
		tm = newAcmeTeam(t, "member", "")
		nb = tm.openNotebook(t, "alice", "Eng")
		for i := range domain.MaxDepth - 2 {
			eighth = tm.createPage(t, "alice", nb, eighth, "Level "+string(rune('A'+i)))
		}
		top = tm.createPage(t, "alice", nb, "", "Top")
		bottom = tm.createPage(t, "alice", nb, top, "Bottom")
		return tm, nb, eighth, top, bottom
	}
	t.Run("the move first", func(t *testing.T) {
		tm, nb, eighth, top, bottom := setup(t)
		moved, created := tm.interleaveOn(t, sharedNotebookRow(nb), nodeMove("alice", top, eighth), childCreation("bob", nb, bottom, "Deep"))
		if !moved.is(http.StatusOK, "") || !created.is(http.StatusConflict, "page.too_deep") {
			t.Errorf("the move = %d %s, then the creation = %d %s; want 200, then 409 page.too_deep", moved.status, moved.code,
				created.status, created.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the creation first", func(t *testing.T) {
		tm, nb, eighth, top, bottom := setup(t)
		created, moved := tm.interleaveOn(t, sharedNotebookRow(nb), childCreation("bob", nb, bottom, "Deep"), nodeMove("alice", top, eighth))
		if !created.is(http.StatusCreated, "") || !moved.is(http.StatusConflict, "page.too_deep") {
			t.Errorf("the creation = %d %s, then the move = %d %s; want 201, then 409 page.too_deep", created.status, created.code,
				moved.status, moved.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 36: deleting a subtree while a page is created in it or one
// of its pages renamed. The deletion first: the parent is gone, 422
// parent_id; the page is gone, 404. The write first: what it wrote goes
// with the subtree, at the deletion's time.
func TestDeletingASubtreeAndWritingInIt(t *testing.T) {
	for _, tt := range []struct {
		name    string
		write   func(nb, child string) step
		refused [2]string // status and code when the deletion is first
		deleted int       // the pages the deletion takes when the write is first
	}{
		{"creating", func(nb, child string) step { return childCreation("bob", nb, child, "Late") },
			[2]string{"422", "validation_failed"}, 3},
		{"renaming", func(_, child string) step { return nodeRename("bob", child, "Renamed") }, [2]string{"404", "page.not_found"}, 2},
	} {
		setup := func(t *testing.T) (tm acmeTeam, nb, top, child string) {
			tm = newAcmeTeam(t, "member", "")
			nb = tm.openNotebook(t, "alice", "Eng")
			top = tm.createPage(t, "alice", nb, "", "Top")
			return tm, nb, top, tm.createPage(t, "alice", nb, top, "Child")
		}
		t.Run(tt.name+", the deletion first", func(t *testing.T) {
			tm, nb, top, child := setup(t)
			deleted, written := tm.interleaveOn(t, sharedNotebookRow(nb), nodeDeletion("alice", top), tt.write(nb, child))
			if !deleted.is(http.StatusNoContent, "") || strconv.Itoa(written.status) != tt.refused[0] || written.code != tt.refused[1] {
				t.Errorf("the deletion = %d, then %s = %d %s; want 204, then %s %s", deleted.status, tt.name, written.status, written.code,
					tt.refused[0], tt.refused[1])
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE name NOT IN ('Top', 'Child')"); n != 0 {
				t.Errorf("%d pages written after the deletion, want none", n)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
		t.Run(tt.name+", the write first", func(t *testing.T) {
			tm, nb, top, child := setup(t)
			written, deleted := tm.interleaveOn(t, sharedNotebookRow(nb), tt.write(nb, child), nodeDeletion("alice", top))
			if written.status/100 != 2 || !deleted.is(http.StatusNoContent, "") {
				t.Errorf("%s = %d %s, then the deletion = %d; want 2xx, then 204", tt.name, written.status, written.code, deleted.status)
			}
			if n := count(t, tm.pool, `SELECT count(*) FROM nodes x JOIN nodes t ON t.id = $1
				WHERE x.deleted_at = t.deleted_at AND x.deleted_at IS NOT NULL`, top); n != tt.deleted {
				t.Errorf("%d pages deleted with Top at its time, want %d", n, tt.deleted)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 37: a page renamed to a title while another page of that
// title is moved under the same parent. Each alone is no clash; the
// second finds the first's under the notebook's lock: 409
// page.title_taken.
func TestRenamingAndMovingIntoOneTitle(t *testing.T) {
	orders(t, "rename", "move", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		parent := tm.createPage(t, "alice", nb, "", "Parent")
		keep := tm.createPage(t, "alice", nb, parent, "Keep")
		merge := tm.createPage(t, "alice", nb, "", "Merge")
		steps := map[string]step{"rename": nodeRename("alice", keep, "MERGE"), "move": nodeMove("bob", merge, parent)}
		a, b := tm.interleaveOn(t, sharedNotebookRow(nb), steps[first], steps[second])
		if a.status != http.StatusOK || !b.is(http.StatusConflict, "page.title_taken") {
			t.Errorf("the %s = %d %s, then the %s = %d %s; want 200, then 409 page.title_taken", first, a.status, a.code, second,
				b.status, b.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}
