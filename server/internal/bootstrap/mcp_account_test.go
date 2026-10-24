package bootstrap

import (
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// The account entry's own tools through serve (M9/P1 design 3.17.4):
// notebooks and open_notebook, the notebooks they reach and their
// workspaces' slugs and names through the composition's AccountWorkspaces.

// A notebook created and never written in opens with nothing counted, no
// last change, an empty root and the built-in instructions, not
// internal_error: its changesets are none, which the matrix's notebooks
// all have (design review B-I2).
func TestOpenNotebookOfAnEmptyNotebook(t *testing.T) {
	r := startReadsApp(t, "")
	empty := r.createNotebook(t, r.access, "acme", "Empty")
	builtIn := r.instructionsAt(t, atNotebook(empty))
	want := map[string]any{
		"notebook": empty, "mode": "standard", "instructions": builtIn, "instructions_from": nil, "truncated": false,
		"overview": map[string]any{"pages": 0.0, "attachments": 0.0, "last_changed_at": nil, "root": []any{}},
	}
	if got := r.opened(t, empty); !reflect.DeepEqual(got, want) {
		t.Errorf("open_notebook of an empty notebook = %v, want %v", got, want)
	}
}

// notebooks lists every notebook the account sees, by workspace, then by
// name, each with its workspace's slug and name as the composition hands
// them over: a workspace named otherwise than its slug tells them apart.
// A deleted notebook and another account's are not listed.
func TestNotebooksListsTheAccountsWorkspaces(t *testing.T) {
	r := startReadsApp(t, "")
	if status, answer := ask(t, r.contract, http.MethodPost, r.base+"/api/v0/workspaces", r.access,
		`{"name":"Zeta Labs","slug":"zeta"}`); status != http.StatusCreated {
		t.Fatalf("create zeta = %d %s", status, answer)
	}
	alpha := r.createNotebook(t, r.access, "zeta", "Alpha")
	bravo := r.createNotebook(t, r.access, "acme", "Bravo")
	item := func(id, name, slug, workspace string) map[string]any {
		return map[string]any{"id": id, "name": name, "role": "admin", "mode": "standard",
			"workspace": map[string]any{"slug": slug, "name": workspace}}
	}
	res := r.call(t, mcpserver.Pattern, "notebooks", map[string]any{}).result(t)
	want := map[string]any{"truncated": false, "notebooks": []any{
		item(bravo, "Bravo", "acme", "acme"), item(r.notebook, "Notes", "acme", "acme"), item(alpha, "Alpha", "zeta", "Zeta Labs"),
	}}
	if got := res["structuredContent"]; res["isError"] == true || !reflect.DeepEqual(got, want) {
		t.Errorf("notebooks = %v, want %v", res, want)
	}
}

// MCP answers each notebook's mode (M10/P1 design 3.12, 3.16): notebooks
// lists it for each; open_notebook answers an LLM Wiki's, its
// instructions its AGENTS page's text, and once the LLM Wiki is
// downgraded, both answer standard at the next call.
func TestNotebooksAnswerTheirModeOverMCP(t *testing.T) {
	r := startReadsApp(t, "")
	status, created := ask(t, r.contract, http.MethodPost, r.base+"/api/v0/workspaces/acme/notebooks", r.access,
		`{"name":"Field notes","mode":"llm_wiki"}`)
	if status != http.StatusCreated {
		t.Fatalf("create an LLM Wiki = %d %s", status, created)
	}
	wiki := idOf(t, answer{body: created})
	listedModes := func(when string, want map[string]string) {
		t.Helper()
		listed := r.call(t, mcpserver.Pattern, "notebooks", map[string]any{})
		var got struct {
			Notebooks []struct{ ID, Mode string } `json:"notebooks"`
		}
		structuredOf(t, listed, &got)
		modes := map[string]string{}
		for _, n := range got.Notebooks {
			modes[n.ID] = n.Mode
		}
		if !maps.Equal(modes, want) {
			t.Errorf("notebooks %s = %s, want the modes %v", when, listed.body, want)
		}
	}
	listedModes("of the LLM Wiki", map[string]string{wiki: "llm_wiki", r.notebook: "standard"})
	agents := wikiPages(t)[1].Content
	if o := r.opened(t, wiki); o["mode"] != "llm_wiki" || o["instructions_from"] != "AGENTS.md" ||
		o["instructions"] != strings.TrimSpace(agents) {
		t.Errorf("open_notebook of the LLM Wiki = mode %v, instructions from %v, %d bytes of them; want llm_wiki, its AGENTS page's text",
			o["mode"], o["instructions_from"], len(fmt.Sprint(o["instructions"])))
	}
	if status, answer := ask(t, r.contract, http.MethodPost, r.base+"/api/v0/notebooks/"+wiki+"/downgrade", r.access, ""); status != http.StatusOK {
		t.Fatalf("downgrade = %d %s, want 200", status, answer)
	}
	if o := r.opened(t, wiki); o["mode"] != "standard" {
		t.Errorf("open_notebook after the downgrade = mode %v, want standard", o["mode"])
	}
	listedModes("after the downgrade", map[string]string{wiki: "standard", r.notebook: "standard"})
}

// wikiPages are the English template's pages, raw then AGENTS.
func wikiPages(t *testing.T) []wiki.TemplatePage {
	t.Helper()
	pages := wiki.TemplatePages(wiki.ModeLLMWiki, wiki.LocaleEn)
	if len(pages) != 2 || pages[1].Title != "AGENTS" {
		t.Fatalf("the English template = %+v, want raw then AGENTS", pages)
	}
	return pages
}
