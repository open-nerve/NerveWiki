package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

// The LLM Wiki's creation through the wired app (M10/P1 design 3.16): the
// notebook module's creation, the wiki module's seeder and the page
// module's template unit, in one transaction.

// createdNotebook is the Notebook answer, as much as these tests read.
type createdNotebook struct {
	ID           string  `json:"id"`
	Mode         string  `json:"mode"`
	SourceRootID *string `json:"source_root_id"`
	CreatedAt    string  `json:"created_at"`
}

// createWith creates the notebook body tells in acme with token, and
// answers it; t fails unless it is 201.
func (tm acmeTeam) createWith(t *testing.T, token, body string) createdNotebook {
	t.Helper()
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/notebooks", token, body)
	var n createdNotebook
	if status != http.StatusCreated || json.Unmarshal([]byte(answer), &n) != nil {
		t.Fatalf("POST %s = %d %s, want 201", body, status, answer)
	}
	return n
}

// rootNode is a node at a notebook's root, as these tests read it.
type rootNode struct {
	ID, Name, Kind, Content string
}

// rootOf is the notebook id's live nodes at its root, in order, with
// their contents.
func (tm acmeTeam) rootOf(t *testing.T, id string) []rootNode {
	t.Helper()
	rows, err := tm.pool.Query(context.Background(), `SELECT n.id::text, n.name, n.kind, coalesce(c.content, '')
		FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id
		WHERE n.notebook_id = $1 AND n.parent_id IS NULL AND n.deleted_at IS NULL ORDER BY n.sort_order`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var nodes []rootNode
	for rows.Next() {
		var n rootNode
		if err := rows.Scan(&n.ID, &n.Name, &n.Kind, &n.Content); err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return nodes
}

// An LLM Wiki is created with its template in the language asked, en
// when none is, through a personal access token and a sign-in session
// alike: its root holds raw, its source root, then AGENTS, each the
// contract's page byte for byte, written in one changeset of the edit kind
// by its creator from the credential's client, of its language's message,
// at the notebook's own time, queued exact for the search, linking and
// tagging nothing. A standard notebook writes nothing.
func TestLLMWikiCreationWritesItsTemplate(t *testing.T) {
	tm := newAcmeTeam(t, "guest", "")
	alice := tm.userID(t, "alice")
	pat := createToken(t, tm.contract, tm.base, tm.tokens["alice"])
	for _, c := range []struct {
		name, token, body string
		locale            wiki.Locale
		client, message   string
	}{
		{"zh-CN, by a personal access token", pat, `{"name":"Field notes","mode":"llm_wiki","template_locale":"zh-CN"}`,
			wiki.LocaleZhCN, "api", "LLM Wiki 模板"},
		{"en, by a sign-in session", tm.tokens["alice"], `{"name":"Lab notes","mode":"llm_wiki","template_locale":"en"}`,
			wiki.LocaleEn, "web", "LLM Wiki template"},
		{"no language, by a sign-in session", tm.tokens["alice"], `{"name":"Reading","mode":"llm_wiki"}`,
			wiki.LocaleEn, "web", "LLM Wiki template"},
	} {
		t.Run(c.name, func(t *testing.T) {
			n := tm.createWith(t, c.token, c.body)
			if n.Mode != "llm_wiki" || n.SourceRootID == nil {
				t.Fatalf("created %+v, want an LLM Wiki with its source root", n)
			}
			root, pages := tm.rootOf(t, n.ID), wiki.TemplatePages(wiki.ModeLLMWiki, c.locale)
			if len(root) != len(pages) || len(pages) != 2 || root[0].ID != *n.SourceRootID {
				t.Fatalf("the root holds %+v, want the %d pages of the template, raw first with the source root %s", root, len(pages),
					*n.SourceRootID)
			}
			for i, p := range pages {
				if root[i].Name != p.Title || root[i].Kind != "page" || root[i].Content != p.Content {
					t.Errorf("root node %d is %s %q of %d bytes, want the page %q of the template, byte for byte", i, root[i].Kind,
						root[i].Name, len(root[i].Content), p.Title)
				}
			}
			ids := []string{root[0].ID, root[1].ID}
			var changesets, items, versions, atItsTime, links, tags, indexed, exact int
			var kind, client, message string
			var byAlice bool
			// No changeset, or one without a message, reads as "" (and
			// false), so that the comparison below tells what is missing.
			if err := tm.pool.QueryRow(context.Background(), `SELECT count(*), coalesce(min(c.kind), ''), coalesce(min(c.client), ''),
					coalesce(min(c.message), ''), coalesce(bool_and(c.created_by_id = $2), false),
					count(*) FILTER (WHERE c.created_at = b.created_at)
				FROM changesets c JOIN notebooks b ON b.id = c.notebook_id WHERE c.notebook_id = $1`, n.ID, alice).
				Scan(&changesets, &kind, &client, &message, &byAlice, &atItsTime); err != nil {
				t.Fatal(err)
			}
			if changesets != 1 || kind != "edit" || client != c.client || message != c.message || !byAlice || atItsTime != 1 {
				t.Errorf("%d changesets, of kind %s from %s, message %q, by alice %v, %d at the notebook's time; want one edit from %s, %q, "+
					"by alice, at its time", changesets, kind, client, message, byAlice, atItsTime, c.client, c.message)
			}
			if err := tm.pool.QueryRow(context.Background(), `SELECT
					(SELECT count(*) FROM changeset_items i JOIN changesets c ON c.id = i.changeset_id
						WHERE c.notebook_id = $1 AND i.node_id::text = ANY($2)),
					(SELECT count(*) FROM page_revisions r JOIN changesets c ON c.id = r.changeset_id JOIN notebooks b ON b.id = c.notebook_id
						WHERE c.notebook_id = $1 AND r.node_id::text = ANY($2) AND r.revision = 1 AND r.created_at = b.created_at),
					(SELECT count(*) FROM page_links WHERE source_id::text = ANY($2)),
					(SELECT count(*) FROM page_tags WHERE source_id::text = ANY($2)),
					(SELECT count(*) FROM indexed_pages WHERE node_id::text = ANY($2)),
					(SELECT count(*) FROM page_search_queue WHERE node_id::text = ANY($2) AND NOT bulk)`, n.ID, ids).
				Scan(&items, &versions, &links, &tags, &indexed, &exact); err != nil {
				t.Fatal(err)
			}
			if items != 2 || versions != 2 || links != 0 || tags != 0 || indexed != 2 || exact != 2 {
				t.Errorf("%d items, %d versions at the notebook's time, %d links, %d tags of %d pages indexed, %d queued exact; "+
					"want 2 items and versions, no link or tag of the 2 indexed, both queued exact", items, versions, links, tags, indexed, exact)
			}
			if at := count(t, tm.pool, `SELECT count(*) FROM nodes n JOIN notebooks b ON b.id = n.notebook_id
				WHERE n.notebook_id = $1 AND n.created_at = b.created_at AND n.created_by_id = $2`, n.ID, alice); at != 2 {
				t.Errorf("%d of the template's nodes by alice at the notebook's time, want 2", at)
			}
		})
	}
	t.Run("standard", func(t *testing.T) {
		n := tm.createWith(t, tm.tokens["alice"], `{"name":"Plain"}`)
		if n.Mode != "standard" || n.SourceRootID != nil || len(tm.rootOf(t, n.ID)) != 0 ||
			count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", n.ID) != 0 {
			t.Errorf("created %+v; want a standard notebook without a source root, a node or a changeset", n)
		}
	})
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)

	// The values are checked, after the decision, and nothing is written
	// when one is refused; the mode is only ever changed by a downgrade.
	before := count(t, tm.pool, "SELECT count(*) FROM notebooks")
	for _, c := range []struct {
		name, by, body string
		status         int
		code, field    string
	}{
		{"a language there is no template of", "alice", `{"name":"X","mode":"llm_wiki","template_locale":"fr"}`,
			http.StatusUnprocessableEntity, "validation_failed", "template_locale invalid_format"},
		{"a mode spelled in upper case", "alice", `{"name":"X","mode":"LLM_WIKI"}`,
			http.StatusUnprocessableEntity, "validation_failed", "mode invalid_format"},
		{"a guest's bad mode", "bob", `{"name":"X","mode":"LLM_WIKI"}`, http.StatusForbidden, "forbidden", ""},
	} {
		status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/notebooks", tm.tokens[c.by], c.body)
		if status != c.status || !strings.Contains(answer, `"code":"`+c.code+`"`) || (c.field != "" && fieldsOf(t, answer) != c.field) {
			t.Errorf("%s: POST = %d %s; want %d %s %s", c.name, status, answer, c.status, c.code, c.field)
		}
	}
	n := tm.createWith(t, tm.tokens["alice"], `{"name":"Kept"}`)
	if status, answer := ask(t, tm.contract, http.MethodPatch, tm.base+"/api/v0/notebooks/"+n.ID, tm.tokens["alice"],
		`{"mode":"llm_wiki"}`); status != http.StatusBadRequest || !strings.Contains(answer, `"code":"not_allowed"`) {
		t.Errorf("PATCH a mode = %d %s, want 400 with the field not_allowed", status, answer)
	}
	if after := count(t, tm.pool, "SELECT count(*) FROM notebooks"); after != before+1 ||
		count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE id = $1 AND mode = 'standard'", n.ID) != 1 {
		t.Errorf("%d notebooks, then %d; want only the one created after, standard still", before, after)
	}
}

// fieldsOf is a 422's fields, as "field code" joined by commas.
func fieldsOf(t *testing.T, answer string) string {
	t.Helper()
	var p struct {
		Errors []struct{ Field, Code string } `json:"errors"`
	}
	if err := json.Unmarshal([]byte(answer), &p); err != nil {
		t.Fatal(err)
	}
	var fields []string
	for _, e := range p.Errors {
		fields = append(fields, e.Field+" "+e.Code)
	}
	return strings.Join(fields, ",")
}

// The template's unit is the creation's: a trigger of the test's own
// refuses AGENTS's node, the second the unit writes, and the whole
// creation rolls back, raw's node and the notebook with it; the error is
// logged with the trigger's words. The sequence the trigger counts with
// keeps its two calls through the rollback. A standard notebook is
// created as before on the same database.
func TestTheTemplateRollsBackWithItsNotebook(t *testing.T) {
	logs := &syncBuffer{}
	tm := newAcmeTeamLogging(t, "", "", nil, transfer.Batch{}, slog.New(slog.NewJSONHandler(logs, nil)))
	ctx := context.Background()
	for _, stmt := range []string{
		"CREATE SEQUENCE seen_nodes",
		`CREATE FUNCTION refuse_agents() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN
				PERFORM nextval('seen_nodes');
				IF NEW.name = 'AGENTS' THEN
					RAISE EXCEPTION 'the test refused AGENTS';
				END IF;
				RETURN NEW;
			END $$`,
		"CREATE TRIGGER refuse_agents BEFORE INSERT ON nodes FOR EACH ROW EXECUTE FUNCTION refuse_agents()",
	} {
		if _, err := tm.pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/notebooks", tm.tokens["alice"],
		`{"name":"Field notes","mode":"llm_wiki","template_locale":"en"}`)

	if status != http.StatusInternalServerError || !strings.Contains(answer, `"code":"internal_error"`) {
		t.Errorf("POST an LLM Wiki whose AGENTS is refused = %d %s, want 500 internal_error", status, answer)
	}
	lines := (&mcpApp{logs: logs}).loggedAt(t, "ERROR")
	if !slices.ContainsFunc(lines, func(e map[string]any) bool {
		return e["msg"] == "API handler failed" && strings.Contains(fmt.Sprint(e["error"]), "the test refused AGENTS")
	}) {
		t.Errorf("the app logged at ERROR %v, want the creation's failure with the trigger's words", lines)
	}
	var last int64
	var called bool
	if err := tm.pool.QueryRow(ctx, "SELECT last_value, is_called FROM seen_nodes").Scan(&last, &called); err != nil {
		t.Fatal(err)
	}
	if last != 2 || !called {
		t.Errorf("the trigger counted %d nodes (called %v), want 2: raw written, AGENTS refused", last, called)
	}
	for _, table := range []string{"notebooks", "notebook_members", "nodes", "page_contents", "page_revisions", "changesets",
		"page_search_queue"} {
		if n := count(t, tm.pool, "SELECT count(*) FROM "+table); n != 0 {
			t.Errorf("%d rows of %s, want none: the creation rolled back whole", n, table)
		}
	}
	if n := tm.createWith(t, tm.tokens["alice"], `{"name":"Plain"}`); n.Mode != "standard" {
		t.Errorf("a standard notebook after = %+v, want it created", n)
	}
}
