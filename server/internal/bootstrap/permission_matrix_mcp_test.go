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
	args    func(c caller, s seeded) map[string]any // the call's arguments, the account entry's notebook added by the runner
	want    func(c caller) mcpCell                  // the cell
	// check, when set, runs on each answer that is a success: what it
	// holds for that column.
	check func(t *testing.T, c caller, s seeded, a mcpAnswer)
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

// mcpRows are the rows, each module's from its file: none before M9/P1
// S2's list and read.
func mcpRows() []mcpRow { return nil }

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
	if len(rows) == 0 {
		t.Skip("no tool has a row yet: M9/P1 S2 adds list and read")
	}
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
	reads := startApp(t, d.config(t, pgtest.NewDatabaseFrom(t, d.url), nil), migrations.FS())
	apps := make(chan struct{}, matrixApps)
	var checked, toCheck atomic.Int64
	t.Cleanup(func() {
		if !t.Failed() && checked.Load() != toCheck.Load() {
			t.Errorf("%d answers checked, want %d", checked.Load(), toCheck.Load())
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
					if entry == mcpserver.AccountEntry {
						path, args["notebook"] = mcpserver.Pattern, notebook
					}
					a := m.call(t, path, r.tool, args)
					got := mcpCellOf(t, a)
					if got != want {
						t.Errorf("%s at %s = %s (%d %.300s), want %s", r.tool, path, got, a.status, a.body, want)
						return
					}
					if r.check != nil && got == (mcpCell{}) {
						r.check(t, c, s, a)
						checked.Add(1)
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
	r := a.result(t)
	if r["isError"] == true {
		code, _ := toolError(t, r)["code"].(string)
		return mcpCell{code: code}
	}
	return mcpCell{}
}
