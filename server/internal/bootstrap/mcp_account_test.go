package bootstrap

import (
	"net/http"
	"reflect"
	"testing"

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
		"notebook": empty, "instructions": builtIn, "instructions_from": nil, "truncated": false,
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
		return map[string]any{"id": id, "name": name, "role": "admin", "workspace": map[string]any{"slug": slug, "name": workspace}}
	}
	res := r.call(t, mcpserver.Pattern, "notebooks", map[string]any{}).result(t)
	want := map[string]any{"truncated": false, "notebooks": []any{
		item(bravo, "Bravo", "acme", "acme"), item(r.notebook, "Notes", "acme", "acme"), item(alpha, "Alpha", "zeta", "Zeta Labs"),
	}}
	if got := res["structuredContent"]; res["isError"] == true || !reflect.DeepEqual(got, want) {
		t.Errorf("notebooks = %v, want %v", res, want)
	}
}
