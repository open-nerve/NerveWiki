package bootstrap

import (
	"maps"
	"net/http"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The LLM Wiki variant of the matrix (M10 design 4.17; M10/P1 design
// 3.14). wlab is lab's twin: each of lab's columns is its member with the
// same role, and lab's notebooks are there by their names, with their
// access, members, deletion and owner, each an LLM Wiki whose root holds
// raw, its source root, and AGENTS, the English template's pages, written
// as the seeded pages are. A row of the variant (matrixRow.wiki) targets
// them: its notebook columns' workspace is wlab (workspaceOfIn). No other
// row, nor prepareMatrix's other users, reads wlab, but MCP's notebooks,
// which lists every workspace of a column (accountNotebooksOf).

// variantWorkspace is the variant's workspace.
const variantWorkspace = "wlab"

// wikiSeeded are the variant's ids, fixed before prepareMatrix runs.
type wikiSeeded struct {
	t           testing.TB
	workspace   uuid.UUID
	memberships map[caller]uuid.UUID
	notebooks   map[string]uuid.UUID // by name
	raws        map[string]uuid.UUID // by notebook
	agentsPages map[string]uuid.UUID // by notebook
	members     map[string]uuid.UUID // by notebook/caller
}

func newWikiSeeded() wikiSeeded {
	w := wikiSeeded{workspace: uuid.NewV7(), memberships: map[caller]uuid.UUID{}, notebooks: map[string]uuid.UUID{},
		raws: map[string]uuid.UUID{}, agentsPages: map[string]uuid.UUID{}, members: map[string]uuid.UUID{}}
	for _, m := range wikiMemberships() {
		w.memberships[m.c] = uuid.NewV7()
	}
	for _, n := range wikiNotebooks() {
		w.notebooks[n.name], w.raws[n.name], w.agentsPages[n.name] = uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	}
	for _, m := range wikiNotebookMembers() {
		w.members[m.notebook+"/"+string(m.c)] = uuid.NewV7()
	}
	return w
}

// wikiMemberships are lab's memberships, the variant's.
func wikiMemberships() []matrixMembership {
	return slices.DeleteFunc(matrixMemberships(), func(m matrixMembership) bool { return m.slug != "lab" })
}

// wikiNotebooks are lab's notebooks, the variant's by their names.
func wikiNotebooks() []matrixNotebook {
	return slices.DeleteFunc(matrixNotebooks(), func(n matrixNotebook) bool { return n.slug != "lab" })
}

// wikiNotebookMembers are the memberships of lab's notebooks, the
// variant's.
func wikiNotebookMembers() []matrixNotebookMember {
	return slices.DeleteFunc(matrixNotebookMembers(), func(m matrixNotebookMember) bool {
		return !slices.ContainsFunc(wikiNotebooks(), func(n matrixNotebook) bool { return n.name == m.notebook })
	})
}

// notebook is the id of the variant's notebook name.
func (w wikiSeeded) notebook(name string) uuid.UUID { return w.of(w.notebooks, name, "notebook") }

// raw is the id of the variant's notebook name's source root.
func (w wikiSeeded) raw(name string) uuid.UUID { return w.of(w.raws, name, "raw") }

// agents is the id of the variant's notebook name's AGENTS page.
func (w wikiSeeded) agents(name string) uuid.UUID { return w.of(w.agentsPages, name, "AGENTS") }

func (w wikiSeeded) of(ids map[string]uuid.UUID, name, what string) uuid.UUID {
	id, ok := ids[name]
	if !ok {
		w.t.Helper()
		w.t.Fatalf("no %s of the variant's %s is seeded", what, name)
	}
	return id
}

// seededNotebooks are the ids of every seeded notebook, the variant's
// after the others: what the tests that read each one by one read.
func seededNotebooks(s seeded) []uuid.UUID {
	var ids []uuid.UUID
	for _, n := range matrixNotebooks() {
		ids = append(ids, s.notebook(n.name))
	}
	for _, n := range wikiNotebooks() {
		ids = append(ids, s.wiki.notebook(n.name))
	}
	return ids
}

// seededWorkspaces are the ids of every seeded workspace, by slug, the
// variant's among them.
func seededWorkspaces(s seeded) map[string]uuid.UUID {
	all := maps.Clone(s.workspaces)
	all[variantWorkspace] = s.wiki.workspace
	return all
}

// has reports whether id is one of the variant's rows.
func (w wikiSeeded) has(id uuid.UUID) bool {
	for _, ids := range []map[string]uuid.UUID{w.notebooks, w.raws, w.agentsPages, w.members} {
		for _, seededID := range ids {
			if seededID == id {
				return true
			}
		}
	}
	for _, seededID := range w.memberships {
		if seededID == id {
			return true
		}
	}
	return id == w.workspace
}

// seedWikiVariant writes the variant through exec at now, before the
// search's queue is filled; deleteNotebook deletes a notebook with what it
// holds, as the matrix deletes gone-nb.
func seedWikiVariant(t testing.TB, exec func(sql string, args ...any), w wikiSeeded, now time.Time, deleteNotebook func(id uuid.UUID)) {
	w.t = t
	account := "(SELECT id FROM users WHERE email = $2)"
	exec("INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) "+
		"VALUES ($1, $3, $3, "+account+", "+account+", $4, $4)", w.workspace, emailOf(callerOutsideAdmin), variantWorkspace, now)
	for _, m := range wikiMemberships() {
		exec("INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) "+
			"VALUES ($1, $3, "+account+", $4, "+account+", "+account+", $5, $5)",
			w.memberships[m.c], emailOf(m.c), w.workspace, string(m.role), now)
	}
	pages := wiki.TemplatePages(wiki.ModeLLMWiki, wiki.LocaleEn)
	for _, n := range wikiNotebooks() {
		id := w.notebooks[n.name]
		exec("INSERT INTO notebooks (id, workspace_id, name, workspace_access, mode, source_root_id, created_by_id, updated_by_id, "+
			"created_at, updated_at) SELECT $1, w.id, $3, $4, 'llm_wiki', $5, w.created_by_id, w.created_by_id, $6, $6 "+
			"FROM workspaces w WHERE w.id = $2", id, w.workspace, n.name, string(n.access), w.raw(n.name), now)
		for i, p := range pages {
			node := w.agents(n.name)
			if p.SourceRoot {
				node = w.raw(n.name)
			}
			exec("INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at) "+
				"SELECT $1, n.id, 'page', $3, $4, $5, n.created_by_id, n.created_by_id, $6, $6 FROM notebooks n WHERE n.id = $2",
				node, id, p.Title, shared.TitleKey(p.Title), i, now)
			exec("INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at) "+
				"SELECT id, $3, 1, sha256(convert_to($3, 'UTF8')), octet_length($3), created_by_id, $2 FROM nodes WHERE id = $1",
				node, now, p.Content)
			exec(seededPageHistory, node)
		}
	}
	for _, m := range wikiNotebookMembers() {
		var ended *time.Time
		if m.ended {
			ended = &now
		}
		exec("INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at, ended_at) "+
			"VALUES ($6, $1, "+account+", $3, "+account+", "+account+", $4, $4, $5)",
			w.notebooks[m.notebook], emailOf(m.c), string(m.role), now, ended, w.members[m.notebook+"/"+string(m.c)])
	}
	for _, n := range wikiNotebooks() {
		if n.deleted {
			deleteNotebook(w.notebooks[n.name])
		}
	}
	orphan, formerOwner := matrixOwnerless()
	exec("UPDATE notebooks SET ownerless_since = $3, former_owner_id = "+account+" WHERE id = $1",
		w.notebooks[orphan], emailOf(formerOwner), now)
}

// wikiMatrixRows are the rows of the variant: the downgrade of an LLM
// Wiki, by its admins, the answer and the row standard without a source
// root.
func wikiMatrixRows() []matrixRow {
	standard := func(t *testing.T, answer string) {
		t.Helper()
		var n struct {
			Mode         string  `json:"mode"`
			SourceRootID *string `json:"source_root_id"`
		}
		decodeAnswer(t, answer, &n)
		if n.Mode != "standard" || n.SourceRootID != nil {
			t.Errorf("the notebook %s, want it standard without a source root", answer)
		}
	}
	return []matrixRow{
		{
			op:      "downgradeNotebook",
			variant: "LLM Wiki",
			wiki:    true,
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/notebooks/" + s.wiki.notebook(notebookOf(c)).String() + "/downgrade", ""
			},
			cells: adminsOnly(cellOK(), cell{http.StatusNotFound, "notebook.not_found"}),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				standard(t, answer)
			},
			after: func(t *testing.T, c caller, s seeded, get func(path string) (int, string)) {
				t.Helper()
				status, answer := get("/api/v0/notebooks/" + s.wiki.notebook(notebookOf(c)).String())
				if status != http.StatusOK {
					t.Fatalf("GET the downgraded notebook = %d %s, want 200", status, answer)
				}
				standard(t, answer)
			},
		},
	}
}
