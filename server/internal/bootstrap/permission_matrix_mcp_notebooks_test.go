package bootstrap

import (
	"encoding/json"
	"slices"
	"testing"
	"uuid"

	pagemcp "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/mcp"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// structuredOf decodes a success's structured content into v.
func structuredOf(t *testing.T, a mcpAnswer, v any) {
	t.Helper()
	raw, err := json.Marshal(a.result(t)["structuredContent"])
	if err == nil {
		err = json.Unmarshal(raw, v)
	}
	if err != nil {
		t.Fatalf("the structured content of %.300s: %v", a.body, err)
	}
}

// accountNotebook is an item of notebooks' answer.
type accountNotebook struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Workspace struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"workspace"`
}

// accountNotebooksOf are the notebooks a notebook column sees in every
// workspace it is in, by workspace, then by name: lab's as REST's list of
// lab answers them (listedNotebooks), other-nb of other, whose admin is
// the column outside lab (M9/P1 design 3.17.2), and wlab's, the LLM Wiki
// variant's, lab's twin, as lab's (M10/P1 design 3.14). Each workspace's
// name is its slug.
func accountNotebooksOf(c caller, s seeded) []accountNotebook {
	item := func(slug string, id uuid.UUID, name, role string) accountNotebook {
		n := accountNotebook{ID: id.String(), Name: name, Role: role}
		n.Workspace.Slug, n.Workspace.Name = slug, slug
		return n
	}
	var out []accountNotebook
	for _, n := range listedNotebooks(c) {
		out = append(out, item("lab", s.notebook(n.Name), n.Name, n.Role))
	}
	if c == callerOutsideWorkspace {
		out = append(out, item("other", s.notebook("other-nb"), "other-nb", "admin"))
	}
	for _, n := range listedNotebooks(c) {
		out = append(out, item(variantWorkspace, s.wiki.notebook(n.Name), n.Name, n.Role))
	}
	return out
}

// notebookMCPRows are notebook's row: notebooks, of no notebook, which
// every notebook column calls with success, its answer the notebooks the
// column sees across its workspaces, never a deleted one, nor one of an
// ended membership, nor one it does not see (M9/P1 design 3.17.2, 3.17.4).
func notebookMCPRows() []mcpRow {
	return []mcpRow{
		{
			tool: "notebooks", entries: mcpserver.AccountEntry, want: func(caller) mcpCell { return mcpCell{} },
			check: func(t *testing.T, c caller, s seeded, a mcpAnswer) {
				t.Helper()
				var got struct {
					Notebooks []accountNotebook `json:"notebooks"`
					Truncated bool              `json:"truncated"`
				}
				structuredOf(t, a, &got)
				if want := accountNotebooksOf(c, s); !slices.Equal(got.Notebooks, want) || got.Truncated {
					t.Errorf("listed %+v, truncated %v; want %+v", got.Notebooks, got.Truncated, want)
				}
			},
		},
	}
}

// openNotebookMCPRow is page's open_notebook, of the account entry alone:
// a column that cannot see its notebook is the tool's notebook.not_found,
// the others' answers that notebook's overview, its seeded pages and
// attachments counted, its root's nodes in the tree's order, its last
// change set (each notebook has its changesets), and the built-in
// instructions, the matrix seeding no AGENTS page (M9/P1 design 3.17.3,
// 3.17.4).
func openNotebookMCPRow() mcpRow {
	return mcpRow{
		tool: "open_notebook", entries: mcpserver.AccountEntry,
		want: func(c caller) mcpCell {
			if _, seen := roleIn(c); seen {
				return mcpCell{}
			}
			return mcpCell{code: "notebook.not_found"}
		},
		check: func(t *testing.T, c caller, s seeded, a mcpAnswer) {
			t.Helper()
			var got struct {
				Notebook         string  `json:"notebook"`
				Instructions     string  `json:"instructions"`
				InstructionsFrom *string `json:"instructions_from"`
				Overview         struct {
					Pages         int     `json:"pages"`
					Attachments   int     `json:"attachments"`
					LastChangedAt *string `json:"last_changed_at"`
					Root          []struct {
						Path string `json:"path"`
					} `json:"root"`
				} `json:"overview"`
				Truncated bool `json:"truncated"`
			}
			structuredOf(t, a, &got)
			nb := notebookOf(c)
			count := func(of []matrixPage) int {
				return len(slices.DeleteFunc(slices.Clone(of), func(p matrixPage) bool { return p.notebook != nb }))
			}
			var root []string
			for _, n := range got.Overview.Root {
				root = append(root, n.Path)
			}
			o := got.Overview
			if got.Notebook != s.notebook(nb).String() || got.Instructions != pagemcp.NotebookInstructions || got.InstructionsFrom != nil ||
				o.Pages != count(matrixPages()) || o.Attachments != count(matrixAssets()) || o.LastChangedAt == nil ||
				!slices.Equal(root, rootPathsOf(nb)) || got.Truncated {
				t.Errorf("opened %.600s; want %s of %d pages, %d attachments, the root %v, the built-in instructions", a.body, nb,
					count(matrixPages()), count(matrixAssets()), rootPathsOf(nb))
			}
		},
	}
}
