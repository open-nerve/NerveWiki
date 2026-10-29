package bootstrap

import (
	"context"
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
	t            testing.TB
	workspace    uuid.UUID
	memberships  map[caller]uuid.UUID
	notebooks    map[string]uuid.UUID // by name
	raws         map[string]uuid.UUID // by notebook
	agentsPages  map[string]uuid.UUID // by notebook
	sourcePages  map[string]uuid.UUID
	sourceAssets map[string]uuid.UUID
	sourceTrash  map[string]uuid.UUID
	outsidePages map[string]uuid.UUID
	members      map[string]uuid.UUID // by notebook/caller
}

func newWikiSeeded() wikiSeeded {
	w := wikiSeeded{workspace: uuid.NewV7(), memberships: map[caller]uuid.UUID{}, notebooks: map[string]uuid.UUID{},
		raws: map[string]uuid.UUID{}, agentsPages: map[string]uuid.UUID{}, members: map[string]uuid.UUID{},
		sourcePages: map[string]uuid.UUID{}, sourceAssets: map[string]uuid.UUID{}, sourceTrash: map[string]uuid.UUID{}, outsidePages: map[string]uuid.UUID{}}
	for _, m := range wikiMemberships() {
		w.memberships[m.c] = uuid.NewV7()
	}
	for _, n := range wikiNotebooks() {
		w.notebooks[n.name], w.raws[n.name], w.agentsPages[n.name] = uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		w.sourcePages[n.name], w.sourceAssets[n.name], w.sourceTrash[n.name], w.outsidePages[n.name] = uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
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
	for _, ids := range []map[string]uuid.UUID{w.notebooks, w.raws, w.agentsPages, w.members, w.sourcePages, w.sourceAssets, w.sourceTrash, w.outsidePages} {
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

// seedWikiVariant creates the variant two hours before the caller's now.
// Its independent source trash is deleted an hour later; deleteNotebook
// still uses the caller's now, so gone-nb keeps that earlier trash group.
// All of this precedes filling the search's queue.
func seedWikiVariant(t testing.TB, exec func(sql string, args ...any), w wikiSeeded, now time.Time, deleteNotebook func(id uuid.UUID)) {
	now = now.Add(-2 * time.Hour)
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
		seedWikiSources(exec, w, n.name, now)
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
// root; and its contract, which the mode guard keeps (M10/P1 design 3.22):
// the source root's rename and move to another page, its admins' alone;
// AGENTS's and the source root's deletion, which no one may make; a page
// and an attachment at the root under a generated page's name.
func wikiMatrixRows() []matrixRow {
	pageNotFound := cell{http.StatusNotFound, "page.not_found"}
	notebookNotFound := cell{http.StatusNotFound, "notebook.not_found"}
	node := func(id uuid.UUID) string { return "/api/v0/nodes/" + id.String() }
	notebook := func(c caller, s seeded) string { return "/api/v0/notebooks/" + s.wiki.notebook(notebookOf(c)).String() }
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
	rows := []matrixRow{
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
		{
			op:      "renameNode",
			variant: "LLM Wiki source root",
			wiki:    true,
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPatch, node(s.wiki.raw(notebookOf(c))), `{"name":"sources"}`
			},
			cells: adminsOnlyOr(cellOK(), cell{http.StatusForbidden, "page.source_admin_only"}, pageNotFound),
			check: func(t *testing.T, _ caller, _ seeded, answer string) {
				t.Helper()
				var n treeNodeAnswer
				decodeAnswer(t, answer, &n)
				if n.Name != "sources" || n.ParentID != nil {
					t.Errorf("renamed %+v, want sources at the root", n)
				}
			},
		},
		{
			op:      "moveNode",
			variant: "LLM Wiki source root",
			wiki:    true,
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, node(s.wiki.raw(notebookOf(c))) + "/move",
					`{"parent_id":"` + s.wiki.agents(notebookOf(c)).String() + `"}`
			},
			cells: adminsOnlyOr(cellOK(), cell{http.StatusForbidden, "page.source_admin_only"}, pageNotFound),
			check: func(t *testing.T, c caller, s seeded, answer string) {
				t.Helper()
				var n treeNodeAnswer
				decodeAnswer(t, answer, &n)
				if want := s.wiki.agents(notebookOf(c)).String(); n.Name != "raw" || n.ParentID == nil || *n.ParentID != want {
					t.Errorf("moved %+v, want raw under AGENTS, %s", n, want)
				}
			},
		},
		{
			op:      "deleteNode",
			variant: "LLM Wiki AGENTS",
			wiki:    true,
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, node(s.wiki.agents(notebookOf(c))), ""
			},
			cells: editorsOnly(cell{http.StatusConflict, "page.protected"}, pageNotFound),
		},
		{
			op:      "deleteNode",
			variant: "LLM Wiki source root",
			wiki:    true,
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodDelete, node(s.wiki.raw(notebookOf(c))), ""
			},
			cells: editorsOnly(cell{http.StatusConflict, "page.protected"}, pageNotFound),
		},
		{
			op:      "createPage",
			variant: "LLM Wiki reserved title",
			wiki:    true,
			columns: notebookColumns(),
			write:   true,
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, notebook(c, s) + "/pages", `{"parent_id":null,"title":"index"}`
			},
			cells: editorsOnly(cell{http.StatusConflict, "page.virtual_file"}, notebookNotFound),
		},
		{
			op:          "uploadAsset",
			variant:     "LLM Wiki reserved title",
			wiki:        true,
			columns:     notebookColumns(),
			write:       true,
			contentType: "multipart/form-data; boundary=matrix",
			request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, notebook(c, s) + "/assets", "--matrix\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\nlog" +
					"\r\n--matrix\r\nContent-Disposition: form-data; name=\"file\"; filename=\"pic.png\"\r\n" +
					"Content-Type: application/octet-stream\r\n\r\n" + pngHead + "\r\n--matrix--\r\n"
			},
			cells: editorsOnly(cell{http.StatusConflict, "page.virtual_file"}, notebookNotFound),
		},
	}
	return append(rows, sourceMatrixRows()...)
}

// adminsOnlyOr answers a notebook's admins answer, its other writers
// writer, refuses its readers, and does not show it to the rest: a write
// an LLM Wiki keeps to its admins (M10/P1 design 3.22).
func adminsOnlyOr(answer, writer, notFound cell) map[caller]cell {
	cells := adminsOnly(answer, notFound)
	for _, c := range notebookColumns() {
		if role, _ := roleIn(c); role == "editor" {
			cells[c] = writer
		}
	}
	return cells
}

// source is the variant's existing nonempty source page or attachment.
func (w wikiSeeded) source(notebook, kind string) uuid.UUID {
	if kind == "asset" {
		return w.of(w.sourceAssets, notebook, "source asset")
	}
	return w.of(w.sourcePages, notebook, "source page")
}

// seedWikiSources supplies real descendants for the permission rows, with
// page history and an asset blob just like the ordinary matrix seeds. A
// deleted source retains its original live parent for restore-in. Its
// deletion is after creation and before the caller deletes the notebook.
func seedWikiSources(exec func(string, ...any), w wikiSeeded, notebook string, now time.Time) {
	for i, p := range []struct {
		id            uuid.UUID
		parent        *uuid.UUID
		name, content string
		deleted       bool
	}{
		{w.sourcePages[notebook], new(w.raw(notebook)), "Source", "source original\n", false},
		{w.outsidePages[notebook], nil, "Outside", "outside original\n", false},
		{w.sourceTrash[notebook], new(w.raw(notebook)), "Trashed source", "trashed original\n", true},
	} {
		exec("INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at) "+
			"SELECT $1, n.id, $3, 'page', $4, $5, $6, n.created_by_id, n.created_by_id, $7, $7 FROM notebooks n WHERE n.id = $2",
			p.id, w.notebook(notebook), p.parent, p.name, shared.TitleKey(p.name), i+2, now)
		exec("INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at) "+
			"SELECT id, $2, 1, sha256(convert_to($2, 'UTF8')), octet_length($2), created_by_id, created_at FROM nodes WHERE id = $1", p.id, p.content)
		exec(seededPageHistory, p.id)
		if !p.deleted && p.parent != nil {
			exec("UPDATE nodes SET source_hash = sha256(convert_to($2, 'UTF8')) WHERE id = $1", p.id, p.content)
		}
		if p.deleted {
			trashedAt := now.Add(time.Hour)
			exec("UPDATE nodes SET deleted_at = $2, updated_at = $2, trash_root = true, trash_pages = 1, trash_assets = 0, trash_height = 1 WHERE id = $1", p.id, trashedAt)
			for _, table := range []string{"page_contents", "page_revisions", "changeset_items"} {
				exec("UPDATE "+table+" SET deleted_at = $2 WHERE node_id = $1", p.id, trashedAt)
			}
			exec("DELETE FROM indexed_pages WHERE node_id = $1", p.id)
		}
	}
	asset := w.sourceAssets[notebook]
	exec("INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at) "+
		"SELECT $1, n.id, $3, 'asset', 'source.txt', 'source.txt', 5, n.created_by_id, n.created_by_id, $4, $4 FROM notebooks n WHERE n.id = $2",
		asset, w.notebook(notebook), w.raw(notebook), now)
	exec("INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at) "+
		"SELECT gen_random_uuid(), id, notebook_id, 'text/plain', 3, sha256('abc'), created_by_id, $2 FROM nodes WHERE id = $1", asset, now)
	exec("UPDATE nodes SET source_hash = sha256('abc') WHERE id = $1", asset)
}

// sourceMatrixRows extend the actual API operations. Each cell still runs
// on the matrix's independent database copy, with all notebook roles.
func sourceMatrixRows() []matrixRow {
	pageNotFound := cell{http.StatusNotFound, "page.not_found"}
	notebookNotFound := cell{http.StatusNotFound, "notebook.not_found"}
	manage := cell{http.StatusForbidden, "page.source_admin_only"}
	var rows []matrixRow
	for _, kind := range []string{"page", "asset"} {
		renamed := "Renamed source"
		if kind == "asset" {
			renamed += ".txt"
		}
		for _, action := range []string{"rename", "move", "reorder", "delete"} {
			op := map[string]string{"rename": "renameNode", "move": "moveNode", "reorder": "moveNode", "delete": "deleteNode"}[action]
			variant := "LLM Wiki source " + kind
			if action == "reorder" {
				variant += " reorder"
			}
			ok := cellOK()
			if action == "delete" {
				ok = cell{http.StatusNoContent, ""}
			}
			rows = append(rows, matrixRow{op: op, variant: variant, wiki: true, columns: notebookColumns(), write: true,
				cells: adminsOnlyOr(ok, manage, pageNotFound),
				request: func(c caller, s seeded) (string, string, string) {
					n := notebookOf(c)
					path := "/api/v0/nodes/" + s.wiki.source(n, kind).String()
					switch action {
					case "rename":
						return http.MethodPatch, path, `{"name":"` + renamed + `"}`
					case "move":
						return http.MethodPost, path + "/move", `{"parent_id":null}`
					case "reorder":
						after := "null"
						if kind == "page" {
							after = `"` + s.wiki.source(n, "asset").String() + `"`
						}
						return http.MethodPost, path + "/move", `{"parent_id":"` + s.wiki.raw(n).String() + `","after_id":` + after + `}`
					default:
						return http.MethodDelete, path, ""
					}
				},
				after: func(t *testing.T, c caller, s seeded, get func(string) (int, string)) {
					t.Helper()
					n := notebookOf(c)
					if action == "delete" {
						status, body := get("/api/v0/nodes/" + s.wiki.source(n, kind).String() + "/trash-item")
						if status != http.StatusOK {
							t.Fatalf("deleted source not in trash: %d %s", status, body)
						}
						return
					}
					status, body := get("/api/v0/notebooks/" + s.wiki.notebook(n).String() + "/nodes")
					if status != http.StatusOK {
						t.Fatalf("read source placement: %d %s", status, body)
					}
					type listedNode struct {
						ID, Name string
						ParentID *string `json:"parent_id"`
					}
					var out struct{ Data []listedNode }
					decodeAnswer(t, body, &out)
					children := slices.DeleteFunc(out.Data, func(x listedNode) bool {
						if action == "move" {
							return x.ParentID != nil
						}
						return x.ParentID == nil || *x.ParentID != s.wiki.raw(n).String()
					})
					index := slices.IndexFunc(children, func(x listedNode) bool { return x.ID == s.wiki.source(n, kind).String() })
					if index < 0 {
						t.Fatalf("source missing from requested parent: %s", body)
					}
					if action == "rename" && children[index].Name != renamed {
						t.Errorf("source name = %s", children[index].Name)
					}
					if action == "reorder" && (kind == "page" && index != len(children)-1 || kind == "asset" && index != 0) {
						t.Errorf("source reorder at %d of %d", index, len(children))
					}
				},
			})
		}
	}
	rows = append(rows,
		matrixRow{op: "moveNode", variant: "LLM Wiki source entry", wiki: true, columns: notebookColumns(), write: true,
			cells: editorsOnly(cellOK(), pageNotFound), request: func(c caller, s seeded) (string, string, string) {
				n := notebookOf(c)
				return http.MethodPost, "/api/v0/nodes/" + s.wiki.outsidePages[n].String() + "/move", `{"parent_id":"` + s.wiki.raw(n).String() + `"}`
			}, check: func(t *testing.T, c caller, s seeded, body string) {
				t.Helper()
				var node treeNodeAnswer
				decodeAnswer(t, body, &node)
				if node.Name != "Outside" || node.ParentID == nil || *node.ParentID != s.wiki.raw(notebookOf(c)).String() {
					t.Errorf("source entry = %s", body)
				}
			}},
		matrixRow{op: "createPage", variant: "LLM Wiki source creation", wiki: true, columns: notebookColumns(), write: true,
			cells: editorsOnly(cellCreated(), notebookNotFound), request: func(c caller, s seeded) (string, string, string) {
				n := notebookOf(c)
				return http.MethodPost, "/api/v0/notebooks/" + s.wiki.notebook(n).String() + "/pages", `{"parent_id":"` + s.wiki.raw(n).String() + `","title":"New source","content":"first"}`
			}, check: func(t *testing.T, _ caller, _ seeded, body string) {
				t.Helper()
				var page pageAnswer
				decodeAnswer(t, body, &page)
				if page.Name != "New source" || page.Revision != 1 || len(page.Ancestors) != 1 || page.Ancestors[0].Name != "raw" {
					t.Errorf("source creation = %s", body)
				}
			}},
		matrixRow{op: "uploadAsset", variant: "LLM Wiki source creation", wiki: true, columns: notebookColumns(), write: true, contentType: "multipart/form-data; boundary=matrix",
			cells: editorsOnly(cellCreated(), notebookNotFound), request: func(c caller, s seeded) (string, string, string) {
				n := notebookOf(c)
				return http.MethodPost, "/api/v0/notebooks/" + s.wiki.notebook(n).String() + "/assets", uploadForm(s.wiki.raw(n).String())
			}, check: func(t *testing.T, c caller, s seeded, body string) {
				t.Helper()
				var asset assetAnswer
				decodeAnswer(t, body, &asset)
				if asset.Name != "pic.png" || asset.ParentID != s.wiki.raw(notebookOf(c)).String() {
					t.Errorf("source upload = %s", body)
				}
			}},
		matrixRow{op: "restoreNode", variant: "LLM Wiki source return", wiki: true, columns: notebookColumns(), write: true,
			cells: editorsOnly(cellOK(), pageNotFound), request: func(c caller, s seeded) (string, string, string) {
				return http.MethodPost, "/api/v0/nodes/" + s.wiki.sourceTrash[notebookOf(c)].String() + "/restore", ""
			}, after: func(t *testing.T, c caller, s seeded, get func(string) (int, string)) {
				t.Helper()
				status, body := get("/api/v0/pages/" + s.wiki.sourceTrash[notebookOf(c)].String())
				var page pageAnswer
				decodeAnswer(t, body, &page)
				if status != http.StatusOK || page.Name != "Trashed source" || len(page.Ancestors) != 1 || page.Ancestors[0].Name != "raw" {
					t.Errorf("restored source = %d %s", status, body)
				}
			}},
	)
	for _, target := range []string{"source page", "source root", "AGENTS"} {
		cells := editorsOnly(cellOK(), pageNotFound)
		if target == "source page" {
			cells = editorsOnly(cell{http.StatusConflict, "page.source_immutable"}, pageNotFound)
		}
		rows = append(rows, matrixRow{op: "putPageContent", variant: "LLM Wiki " + target, wiki: true, columns: notebookColumns(), write: true, cells: cells,
			request: func(c caller, s seeded) (string, string, string) {
				n := notebookOf(c)
				id := s.wiki.source(n, "page")
				switch target {
				case "source root":
					id = s.wiki.raw(n)
				case "AGENTS":
					id = s.wiki.agents(n)
				}
				return http.MethodPut, "/api/v0/pages/" + id.String() + "/content", `{"content":"updated","base_revision":1}`
			}, check: func(t *testing.T, _ caller, _ seeded, body string) {
				t.Helper()
				var page pageAnswer
				decodeAnswer(t, body, &page)
				if page.Revision != 2 {
					t.Errorf("ordinary body write revision = %d, want 2", page.Revision)
				}
			}})
	}
	return rows
}

// Operation coverage alone cannot notice removal of one source variant when
// the operation's ordinary row remains. This independent list keeps each real
// source-policy row in the permission runner, including the standard control.
func TestSourcePermissionMatrixKeepsEveryRequiredRow(t *testing.T) {
	want := map[string]bool{
		"renameNode, LLM Wiki source page": false, "moveNode, LLM Wiki source page": false,
		"moveNode, LLM Wiki source page reorder": false, "deleteNode, LLM Wiki source page": false,
		"renameNode, LLM Wiki source asset": false, "moveNode, LLM Wiki source asset": false,
		"moveNode, LLM Wiki source asset reorder": false, "deleteNode, LLM Wiki source asset": false,
		"moveNode, LLM Wiki source entry": false, "createPage, LLM Wiki source creation": false,
		"uploadAsset, LLM Wiki source creation": false, "restoreNode, LLM Wiki source return": false,
		"putPageContent, LLM Wiki source page": false, "putPageContent, LLM Wiki source root": false,
		"putPageContent, LLM Wiki AGENTS": false, "putPageContent": false,
	}
	for _, row := range matrixRows() {
		if _, required := want[row.name()]; required {
			want[row.name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("source permission coverage lost actual operation row %s", name)
		}
	}
}

// The variant's independent trash survives a later notebook deletion with
// its own time and group metadata, as the ordinary matrix's trash does.
func TestSourceMatrixSeedKeepsItsTrashGroups(t *testing.T) {
	d := prepareMatrix(t)
	w := d.seeded.in(t).wiki
	pool := connect(t, d.url)
	gone := 0
	for _, notebook := range wikiNotebooks() {
		if !notebook.deleted {
			continue
		}
		gone++
		var created, trashed, deleted time.Time
		var root bool
		var pages, assets, height int
		if err := pool.QueryRow(context.Background(), `SELECT n.created_at, n.deleted_at, b.deleted_at,
			n.trash_root, n.trash_pages, n.trash_assets, n.trash_height
			FROM nodes n JOIN notebooks b ON b.id = n.notebook_id
			WHERE n.id = $1 AND n.notebook_id = $2 AND n.parent_id = $3`,
			w.sourceTrash[notebook.name], w.notebook(notebook.name), w.raw(notebook.name)).Scan(
			&created, &trashed, &deleted, &root, &pages, &assets, &height); err != nil {
			t.Fatal(err)
		}
		if !created.Before(trashed) || !trashed.Before(deleted) {
			t.Errorf("%s source times: created=%s trash=%s notebook deletion=%s, want created < trash < notebook deletion",
				notebook.name, created, trashed, deleted)
		}
		if !root || pages != 1 || assets != 0 || height != 1 {
			t.Errorf("%s old source trash: root=%t pages=%d assets=%d height=%d, want true/1/0/1",
				notebook.name, root, pages, assets, height)
		}
	}
	if gone == 0 {
		t.Fatal("the matrix has no deleted LLM Wiki to check")
	}
	checkPages(t, pool)
}
