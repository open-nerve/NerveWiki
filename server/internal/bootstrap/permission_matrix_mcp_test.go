package bootstrap

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The permission matrix over MCP (M9/P1 design 3.6): each tool of
// api/mcp/tools.json called through each entry that lists it, by each
// notebook column's personal access token, on the REST matrix's prepared
// data (prepareMatrix), its answer asserted cell by cell. Each module's
// rows come with its tools (S2 on).

// mcpRow is a tool's row: how each column's call answers, through each
// entry the tool is listed in.
type mcpRow struct {
	tool    string
	entries mcpserver.Entry
	write   bool                                    // the call writes: its cells run on a copy of the database (P2 on)
	args    func(c caller, s seeded) map[string]any // the call's arguments, the account entry's notebook added by the runner (takesNotebook)
	want    func(c caller) mcpCell                  // the cell
	// check, when set, runs on each answer that is a success: what it
	// holds for that column.
	check func(t *testing.T, c caller, s seeded, a mcpAnswer)
	// after, when set, runs on each answer that is a success, on the copy
	// of the database the cell wrote: call is a call as the cell's caller,
	// through the cell's entry, to read back what it wrote (M9/P2 design
	// 3.13.2).
	after func(t *testing.T, c caller, s seeded, call func(tool string, args map[string]any) mcpAnswer)
}

// mcpCell is how a call answers: an HTTP status before the SDK, or a tool
// result with its error code ("" for a success).
type mcpCell struct {
	http int
	code string
}

// String is the cell as a failure prints it.
func (c mcpCell) String() string {
	if c.http != 0 {
		return fmt.Sprintf("HTTP %d %s", c.http, c.code)
	}
	if c.code == "" {
		return "a result"
	}
	return "the tool's error " + c.code
}

// mcpRows are the rows, each module's: page's list, read (M9/P1 design
// 3.13) and open_notebook (3.17.3), and its writes (M9/P2 design 3.13.2),
// linking's backlinks and index (M9/P4 design 3.13), notebook's notebooks
// (3.17.2); S3's rows are 3.17.4's.
func mcpRows() []mcpRow {
	return slices.Concat(pageMCPRows(), pageWriteMCPRows(), linkingMCPRows(), notebookMCPRows())
}

// mcpToolCodes are the codes each MCP tool's error may carry besides
// internal_error (M9 design 4.7; M9/P1 S2 review C's table), beside the
// matrix's rows: each one the contract declares, each tool of tools.json
// with its row, and each code of a row answered by
// TestTheMCPToolsErrorsCarryTheirCodes. A call's error with a code its
// tool's row does not list fails the test that called it
// (mcpAnswer.toolError). Each step adds its tools' rows and their calls:
// S3's notebooks has no argument to refuse but one it does not take, and
// open_notebook only its notebook, unseen or no UUID (M9/P1 design
// 3.17); P2's writes the codes of the REST writes they are (M9/P2 design
// 3.13.1), its history's reads those of the REST reads they are
// (3.17.1); P3's batch the union of the writes' and its own
// page.batch_too_large (M9/P3 design 3.13.1); P4's grep and search those
// of their arguments and notebook, grep's slot server_busy, backlinks'
// and index's their path's page.not_found too (M9/P4 design 3.12). M10's
// mode guard adds an LLM Wiki's: write's page.virtual_file, move's and
// batch's page.protected, page.virtual_file and page.source_admin_only,
// delete's page.protected; edit has none in P1 (M10/P1 design 3.22).
func mcpToolCodes() map[string][]string {
	return map[string][]string{
		"list":          {"bad_request", "validation_failed", "notebook.not_found", "page.not_found"},
		"read":          {"bad_request", "validation_failed", "notebook.not_found", "page.not_found"},
		"notebooks":     {"bad_request"},
		"open_notebook": {"bad_request", "notebook.not_found"},
		"write": {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "server_busy",
			"page.title_taken", "page.too_deep", "page.revision_mismatch", "page.locked", "page.virtual_file"},
		"edit": {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "server_busy",
			"page.revision_mismatch", "page.locked"},
		"move": {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "server_busy",
			"page.title_taken", "page.cycle", "page.too_deep", "linking.pages_locked", "page.protected", "page.virtual_file",
			"page.source_admin_only"},
		"delete": {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "page.locked",
			"page.protected"},
		"batch": {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "server_busy",
			"page.title_taken", "page.cycle", "page.too_deep", "page.revision_mismatch", "page.locked", "linking.pages_locked",
			"page.batch_too_large", "page.protected", "page.virtual_file", "page.source_admin_only"},
		"versions":  {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "page.version_not_found"},
		"log":       {"bad_request", "validation_failed", "notebook.not_found", "page.not_found"},
		"changeset": {"bad_request", "validation_failed", "notebook.not_found", "page.changeset_not_found"},
		"grep":      {"bad_request", "validation_failed", "notebook.not_found", "server_busy"},
		"search":    {"bad_request", "validation_failed", "notebook.not_found"},
		"backlinks": {"bad_request", "validation_failed", "notebook.not_found", "page.not_found"},
		"index":     {"bad_request", "validation_failed", "notebook.not_found", "page.not_found"},
	}
}

// Every tool of tools.json, in either entry, has its row of codes, and
// no row is a tool's that the file has not; each code of a row is one the
// contract declares.
func TestEveryMCPToolHasItsCodes(t *testing.T) {
	rows := mcpToolCodes()
	listed := map[string]bool{}
	for entry, tools := range readToolLists(t) {
		for _, tool := range tools {
			spec, _ := tool.(map[string]any)
			name, _ := spec["name"].(string)
			if listed[name] = true; rows[name] == nil {
				t.Errorf("%s of the %s entry has no row of codes", name, entry)
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(rows)) {
		if !listed[name] {
			t.Errorf("the codes have a row of %s, which tools.json lists in no entry", name)
		}
		for _, code := range rows[name] {
			if err := undeclaredToolCode(code, declaredCodes(t)); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	}
}

// pageMCPRows are page's rows: list of the root's children and read of
// the column's page, each in both entries; a column that cannot see its
// notebook is its 404 (mcpRow.want's notebook.not_found, the notebook
// entry's edge 404 for it), the others' answers holding that page;
// open_notebook (openNotebookMCPRow); and the history's reads, as the
// readers' too, in both entries (M9/P2 design 3.17.2): versions of the
// column's page, its one version; log, the seeded rename first, as REST's
// listChangesets; changeset of the seeded rename, its one change with its
// node's path; grep and search by the mark, as the readers' too, in both
// entries (M9/P4 design 3.13), through the queue (queueSeeded): grep's
// hits the notebook's seeded pages not deleted, each at the mark's line,
// search's results its nodes markedNodes finds, counted exact.
func pageMCPRows() []mcpRow {
	readers := func(c caller) mcpCell {
		if _, seen := roleIn(c); seen {
			return mcpCell{}
		}
		return mcpCell{code: "notebook.not_found"}
	}
	return []mcpRow{
		{
			tool: "list", entries: mcpserver.BothEntries, want: readers,
			check: func(t *testing.T, c caller, _ seeded, a mcpAnswer) {
				t.Helper()
				var paths []string
				items, _ := a.result(t)["structuredContent"].(map[string]any)["items"].([]any)
				for _, item := range items {
					path, _ := item.(map[string]any)["path"].(string)
					paths = append(paths, path)
				}
				if want := rootPathsOf(notebookOf(c)); !slices.Equal(paths, want) {
					t.Errorf("listed %v, want the root's %v", paths, want)
				}
			},
		},
		{
			tool: "read", entries: mcpserver.BothEntries, want: readers,
			args: func(c caller, _ seeded) map[string]any { return map[string]any{"path": pagePathOf(c)} },
			check: func(t *testing.T, c caller, s seeded, a mcpAnswer) {
				t.Helper()
				content, _ := a.result(t)["content"].([]any)
				if len(content) != 2 {
					t.Fatalf("read answered %v, want two blocks", content)
				}
				meta, _ := content[0].(map[string]any)["text"].(string)
				text, _ := content[1].(map[string]any)["text"].(string)
				var m struct{ Path, Version string }
				_ = json.Unmarshal([]byte(meta), &m)
				if want := s.page(pageOf(c)).String() + ":1"; m.Path != pagePathOf(c) || m.Version != want || text != matrixMark+"\n" {
					t.Errorf("read %s and %q, want %s at %s and the seeded content", meta, text, pagePathOf(c), want)
				}
			},
		},
		openNotebookMCPRow(),
		{
			tool: "versions", entries: mcpserver.BothEntries, want: readers,
			args: func(c caller, _ seeded) map[string]any { return map[string]any{"path": pagePathOf(c)} },
			check: func(t *testing.T, c caller, s seeded, a mcpAnswer) {
				t.Helper()
				var v struct {
					Path           string `json:"path"`
					CurrentVersion string `json:"current_version"`
					Versions       []struct {
						Revision int `json:"revision"`
					} `json:"versions"`
				}
				structuredOf(t, a, &v)
				want := s.page(pageOf(c)).String() + ":1"
				if v.Path != pagePathOf(c) || v.CurrentVersion != want || len(v.Versions) != 1 || v.Versions[0].Revision != 1 {
					t.Errorf("versions answered %.500s, want %s's one version, revision 1, and %s", a.body, pagePathOf(c), want)
				}
			},
		},
		{
			tool: "log", entries: mcpserver.BothEntries, want: readers,
			check: func(t *testing.T, c caller, s seeded, a mcpAnswer) {
				t.Helper()
				var l struct {
					Changesets []changesetAnswer `json:"changesets"`
				}
				structuredOf(t, a, &l)
				if len(l.Changesets) == 0 || !l.Changesets[0].isRename(s.changeset(notebookOf(c))) {
					t.Errorf("log answered %.500s, want the seeded rename first", a.body)
				}
			},
		},
		{
			tool: "changeset", entries: mcpserver.BothEntries, want: readers,
			args: func(c caller, s seeded) map[string]any {
				return map[string]any{"id": s.changeset(notebookOf(c)).String()}
			},
			check: func(t *testing.T, c caller, s seeded, a mcpAnswer) {
				t.Helper()
				var p mcpChangesetPage
				structuredOf(t, a, &p)
				nb := notebookOf(c)
				switch {
				case p.Changeset == nil || p.Changeset.ID != s.changeset(nb).String() || !p.Changeset.Renames || p.Changeset.Moves:
					t.Errorf("changeset answered %.500s, want the seeded rename, renaming and moving nothing", a.body)
				case len(p.Changes) != 1 || p.Changes[0].Node.ID != s.page(renamedIn(nb)).String() || p.Changes[0].Node.Path == nil ||
					*p.Changes[0].Node.Path != renamedIn(nb)+".md":
					t.Errorf("changeset's changes are %.500s, want %s.md's rename", a.body, renamedIn(nb))
				}
			},
		},
		{
			tool: "grep", entries: mcpserver.BothEntries, want: readers,
			args: func(caller, seeded) map[string]any { return map[string]any{"pattern": matrixMark} },
			check: func(t *testing.T, c caller, _ seeded, a mcpAnswer) {
				t.Helper()
				var g struct {
					Hits []struct {
						Path string `json:"path"`
						Line int    `json:"line"`
						Text string `json:"text"`
					} `json:"hits"`
					PartialLines []any   `json:"partial_lines"`
					NextCursor   *string `json:"next_cursor"`
					Incomplete   bool    `json:"incomplete"`
				}
				structuredOf(t, a, &g)
				var got, want []string
				for _, h := range g.Hits {
					got = append(got, fmt.Sprintf("%s:%d:%s", h.Path, h.Line, h.Text))
				}
				for _, p := range matrixPages() {
					if p.notebook == notebookOf(c) {
						line := 1
						if p.name == tasksIn(p.notebook) {
							line = 3
						}
						want = append(want, fmt.Sprintf("%s:%d:%s", matrixPathOf(p, ".md"), line, matrixMark))
					}
				}
				slices.Sort(got)
				slices.Sort(want)
				if !slices.Equal(got, want) || len(g.PartialLines) != 0 || g.NextCursor != nil || g.Incomplete {
					t.Errorf("grep answered %.800s, want the hits %q, nothing more", a.body, want)
				}
			},
		},
		{
			tool: "search", entries: mcpserver.BothEntries, want: readers,
			args: func(caller, seeded) map[string]any { return map[string]any{"q": matrixMark, "limit": 50} },
			check: func(t *testing.T, c caller, _ seeded, a mcpAnswer) {
				t.Helper()
				var r struct {
					Results []struct {
						Path string `json:"path"`
					} `json:"results"`
					Count *struct {
						Value int    `json:"value"`
						Kind  string `json:"kind"`
					} `json:"count"`
					NextCursor *string `json:"next_cursor"`
					Building   bool    `json:"building"`
				}
				structuredOf(t, a, &r)
				var got, want []string
				for _, res := range r.Results {
					got = append(got, res.Path)
				}
				for _, p := range matrixPages() {
					if p.notebook == notebookOf(c) {
						want = append(want, matrixPathOf(p, ".md"))
					}
				}
				for _, p := range matrixAssets() {
					if p.notebook == notebookOf(c) && p.name == markedIn(p.notebook) {
						want = append(want, matrixPathOf(p, ""))
					}
				}
				slices.Sort(got)
				slices.Sort(want)
				if !slices.Equal(got, want) || r.Count == nil || r.Count.Value != len(want) || r.Count.Kind != "exact" ||
					r.NextCursor != nil || r.Building {
					t.Errorf("search answered %.800s, want the results %q, counted exact", a.body, want)
				}
			},
		},
	}
}

// matrixPathOf is a seeded node's path: its ancestors' titles and its own,
// then suffix, ".md" for a page.
func matrixPathOf(p matrixPage, suffix string) string {
	names := []string{p.name}
	if p.parent != "" {
		names = append(append(ancestorsOf(p.parent), p.parent), p.name)
	}
	return strings.Join(names, "/") + suffix
}

// mcpChangesetPage is changeset's answer, as these tests read it: the
// changeset on the first page, and its changes, each node with its path.
type mcpChangesetPage struct {
	Changeset  *changesetAnswer `json:"changeset"`
	Changes    []mcpChange      `json:"changes"`
	NextCursor *string          `json:"next_cursor"`
}

// mcpChange is a change of changeset's answer: REST's, its node's kind
// "attachment" for REST's "asset", with its path, null for a node in the
// trash or gone.
type mcpChange struct {
	Node struct {
		ID    string  `json:"id"`
		Kind  string  `json:"kind"`
		Name  string  `json:"name"`
		Alive bool    `json:"alive"`
		Path  *string `json:"path"`
	} `json:"node"`
	Kind string `json:"kind"`
}

// pagePathOf is the path of a notebook column's page (pageOf).
func pagePathOf(c caller) string {
	if notebookOf(c) == "priv" {
		return "priv-root/" + pageOf(c) + ".md"
	}
	return pageOf(c) + ".md"
}

// rootPathsOf is the paths of a notebook's seeded nodes at its root, in
// the tree's order: its pages' with .md, its attachments' as they are.
func rootPathsOf(notebook string) []string {
	var out []string
	for _, name := range treeOf(notebook) {
		if slices.Contains(matrixPages(), matrixPage{name, notebook, ""}) {
			out = append(out, name+".md")
		}
		if slices.Contains(matrixAssets(), matrixPage{name, notebook, ""}) {
			out = append(out, name)
		}
	}
	return out
}

// mcpMatrixViolations are the gaps between the rows and the tools the
// file lists: a file without exactly the two entries' keys, which would
// leave tools without a row unseen; a tool without exactly one row, or
// whose row's entries are not those listing it; a row of a tool no entry
// lists; a row of a tool that writes (readOnlyHint false) without write,
// whose cells would run on the reads' copy, or of one that only reads
// with it.
func mcpMatrixViolations(lists toolLists, rows []mcpRow) []string {
	var found []string
	for _, key := range slices.Sorted(maps.Keys(lists)) {
		if _, ok := mcpEntries()[key]; !ok {
			found = append(found, fmt.Sprintf("the file's %q is no entry: its keys are notebook and account", key))
		}
	}
	listedIn, readOnly := map[string]mcpserver.Entry{}, map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(mcpEntries())) {
		tools, ok := lists[name]
		if !ok {
			found = append(found, fmt.Sprintf("the file has no %q: its tools would have no row", name))
		}
		for _, tool := range tools {
			object, _ := tool.(map[string]any)
			toolName, _ := object["name"].(string)
			annotations, _ := object["annotations"].(map[string]any)
			listedIn[toolName] |= mcpEntries()[name]
			readOnly[toolName], _ = annotations["readOnlyHint"].(bool)
		}
	}
	rowsOf := map[string]int{}
	for _, r := range rows {
		rowsOf[r.tool]++
		entries, listed := listedIn[r.tool]
		switch {
		case !listed:
			found = append(found, fmt.Sprintf("row %s names no tool of the file", r.tool))
			continue
		case r.entries != entries:
			found = append(found, fmt.Sprintf("row %s is of the %s entries, the file lists it in %s", r.tool, r.entries, entries))
		}
		if !readOnly[r.tool] && !r.write {
			found = append(found, fmt.Sprintf("row %s: the tool writes, the row has no write: its cells would run on the reads' copy", r.tool))
		}
		if readOnly[r.tool] && r.write {
			found = append(found, fmt.Sprintf("row %s: the tool only reads, the row has write", r.tool))
		}
	}
	for _, tool := range slices.Sorted(maps.Keys(listedIn)) {
		if n := rowsOf[tool]; n != 1 {
			found = append(found, fmt.Sprintf("tool %s has %d rows, want one", tool, n))
		}
	}
	return found
}

func TestThePermissionMatrixCoversEveryTool(t *testing.T) {
	for _, v := range mcpMatrixViolations(readToolLists(t), mcpRows()) {
		t.Error(v)
	}
}

// Each kind of gap is reported: a synthetic file and rows, one gap at a
// time.
func TestMCPMatrixViolationsCatchesEachGap(t *testing.T) {
	tool := func(name string, readOnly bool) map[string]any {
		return map[string]any{"name": name, "annotations": map[string]any{"readOnlyHint": readOnly}}
	}
	lists := func() toolLists {
		return toolLists{"notebook": {tool("list", true)}, "account": {tool("list", true), tool("notebooks", true), tool("edit", false)}}
	}
	rows := []mcpRow{
		{tool: "list", entries: mcpserver.BothEntries}, {tool: "notebooks", entries: mcpserver.AccountEntry},
		{tool: "edit", entries: mcpserver.AccountEntry, write: true},
	}
	if found := mcpMatrixViolations(lists(), rows); len(found) != 0 {
		t.Fatalf("a complete matrix: %q", found)
	}
	changed := func(tool string, change func(*mcpRow)) []mcpRow {
		out := slices.Clone(rows)
		for i := range out {
			if out[i].tool == tool {
				change(&out[i])
			}
		}
		return out
	}
	tests := []struct {
		name  string
		lists toolLists
		rows  []mcpRow
		want  string
	}{
		{"a tool without a row", lists(), rows[:2], "tool edit has 0 rows, want one"},
		{"a tool with two rows", lists(), append(slices.Clone(rows), rows[0]), "tool list has 2 rows, want one"},
		{"a row of an entry more", lists(), changed("notebooks", func(r *mcpRow) { r.entries = mcpserver.BothEntries }),
			"row notebooks is of the both entries, the file lists it in account"},
		{"a row of no tool", lists(), append(slices.Clone(rows), mcpRow{tool: "nothing", entries: mcpserver.AccountEntry}),
			"row nothing names no tool of the file"},
		{"a write's row without write", lists(), changed("edit", func(r *mcpRow) { r.write = false }), "row edit: the tool writes"},
		{"a read's row with write", lists(), changed("list", func(r *mcpRow) { r.write = true }), "row list: the tool only reads"},
		{"the file without an entry", func() toolLists { l := lists(); delete(l, "account"); return l }(), rows, `the file has no "account"`},
		{"the file with a key of no entry", func() toolLists { l := lists(); l["acount"] = l["account"]; return l }(), rows,
			`the file's "acount" is no entry`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if found := mcpMatrixViolations(tt.lists, tt.rows); !slices.ContainsFunc(found, func(v string) bool { return strings.Contains(v, tt.want) }) {
				t.Errorf("violations = %q, want one with %q", found, tt.want)
			}
		})
	}
}

// takesNotebook are the tools whose account entry's schema in the file
// requires a notebook: the runner adds the column's notebook to that
// entry's call for those alone, a tool of no notebook (S3's notebooks)
// refusing one as an argument it has not (M9/P1 S1 review B-M1).
func takesNotebook(lists toolLists) map[string]bool {
	takes := map[string]bool{}
	for _, tool := range lists["account"] {
		spec, _ := tool.(map[string]any)
		schema, _ := spec["inputSchema"].(map[string]any)
		required, _ := schema["required"].([]any)
		name, _ := spec["name"].(string)
		takes[name] = slices.Contains(required, any("notebook"))
	}
	return takes
}

// The runner gives an account entry's call a notebook only when the
// file's schema of that entry requires one: list's, not notebooks', nor
// a schema whose notebook is optional (M9/P1 S1 review B-M1).
func TestTheMatrixGivesANotebookAsTheFileSays(t *testing.T) {
	schema := func(required ...any) map[string]any {
		return map[string]any{"type": "object", "required": required, "properties": map[string]any{"notebook": map[string]any{"type": "string"}}}
	}
	lists := toolLists{
		"notebook": {map[string]any{"name": "notebooks", "inputSchema": schema("notebook")}},
		"account": {
			map[string]any{"name": "list", "inputSchema": schema("notebook", "pattern")},
			map[string]any{"name": "notebooks", "inputSchema": map[string]any{"type": "object", "additionalProperties": false}},
			map[string]any{"name": "optional", "inputSchema": schema("path")},
		},
	}
	want := map[string]bool{"list": true, "notebooks": false, "optional": false}
	if got := takesNotebook(lists); !maps.Equal(got, want) {
		t.Errorf("takesNotebook() = %v, want %v", got, want)
	}
}

// gateCells are the notebook columns' cells of REST's listNodes, the
// rule node.list that lets a call into the notebook entry: a column it
// answers 404 is the notebook entry's 404 (M9/P1 design 3.6).
func gateCells(t *testing.T) map[caller]cell {
	t.Helper()
	for _, r := range pageMatrixRows() {
		if r.op == "listNodes" && r.variant == "" {
			return r.cells
		}
	}
	t.Fatal("the matrix has no row of listNodes")
	return nil
}

// Each cell of the matrix over MCP, in parallel, as TestPermissionMatrix
// runs REST's: each notebook column's personal access token, created
// here on the prepared data before it is copied, calls each row's tool
// through each entry the row is of, on its column's notebook (notebookOf):
// in the notebook entry a column REST's listNodes does not let in is the
// edge's 404, in the account entry the tool's notebook.not_found.
func TestThePermissionMatrixOverMCP(t *testing.T) {
	rows := mcpRows()
	takes := takesNotebook(readToolLists(t))
	d := prepareMatrix(t)
	pats := map[caller]string{}
	created := t.Run("tokens", func(t *testing.T) {
		contract := apitest.Load(t)
		base := startApp(t, d.config(t, d.url, nil), migrations.FS())
		for _, c := range notebookColumns() {
			pats[c] = createToken(t, contract, base, d.tokens[c])
		}
	})
	if !created || len(pats) != len(notebookColumns()) {
		t.Fatal("the tokens were not created: a -run of some cells must select tokens too, e.g. -run 'TestThePermissionMatrixOverMCP/(prepare|tokens|list)'")
	}
	gate := gateCells(t)
	// The reads' cells run in parallel on one app, at production's deadlines,
	// so that grep's cells answer their whole walk (productionDeadlines).
	reads := startApp(t, d.config(t, pgtest.NewDatabaseFrom(t, d.url), productionDeadlines), migrations.FS())
	apps := make(chan struct{}, matrixApps)
	var checked, toCheck, aftered, toAfter atomic.Int64
	t.Cleanup(func() {
		if !t.Failed() && checked.Load() != toCheck.Load() {
			t.Errorf("%d answers checked, want %d", checked.Load(), toCheck.Load())
		}
		if !t.Failed() && aftered.Load() != toAfter.Load() {
			t.Errorf("%d writes read back, want %d", aftered.Load(), toAfter.Load())
		}
	})
	for _, r := range rows {
		for _, entry := range []mcpserver.Entry{mcpserver.NotebookEntry, mcpserver.AccountEntry} {
			if r.entries&entry == 0 {
				continue
			}
			for _, c := range notebookColumns() {
				t.Run(r.tool+"/"+entry.String()+"/"+string(c), func(t *testing.T) {
					t.Parallel()
					want := r.want(c)
					if entry == mcpserver.NotebookEntry && gate[c].status == http.StatusNotFound {
						want = mcpCell{http: http.StatusNotFound, code: "notebook.not_found"}
					}
					if r.check != nil && want == (mcpCell{}) {
						toCheck.Add(1)
					}
					if r.after != nil && want == (mcpCell{}) {
						toAfter.Add(1)
					}
					m := &mcpApp{base: reads, pat: pats[c]}
					if r.write {
						apps <- struct{}{}
						t.Cleanup(func() { <-apps })
						m.base = startApp(t, d.config(t, pgtest.NewDatabaseFrom(t, d.url), nil), migrations.FS())
					}
					s := d.seeded.in(t)
					notebook := s.notebook(notebookOf(c)).String()
					args := map[string]any{}
					if r.args != nil {
						maps.Copy(args, r.args(c, s))
					}
					path := atNotebook(notebook)
					call := func(tool string, args map[string]any) mcpAnswer {
						if entry == mcpserver.AccountEntry && takes[tool] {
							args["notebook"] = notebook
						}
						return m.call(t, path, tool, args)
					}
					if entry == mcpserver.AccountEntry {
						path = mcpserver.Pattern
					}
					a := call(r.tool, args)
					got := mcpCellOf(t, a)
					if got != want {
						t.Errorf("%s at %s = %s (%d %.300s), want %s", r.tool, path, got, a.status, a.body, want)
						return
					}
					if r.check != nil && got == (mcpCell{}) {
						r.check(t, c, s, a)
						checked.Add(1)
					}
					if r.after != nil && got == (mcpCell{}) {
						r.after(t, c, s, call)
						aftered.Add(1)
					}
				})
			}
		}
	}
}

// mcpCellOf is the cell of a call's answer.
func mcpCellOf(t *testing.T, a mcpAnswer) mcpCell {
	t.Helper()
	if a.status != http.StatusOK {
		var p struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(a.body, &p)
		return mcpCell{http: a.status, code: p.Code}
	}
	if r := a.result(t); r["isError"] == true {
		code, _ := a.toolError(t)["code"].(string)
		return mcpCell{code: code}
	}
	return mcpCell{}
}
