package bootstrap

import (
	"encoding/json"
	"path"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// writers is the MCP form of editorsOnly: a notebook's admins and editors
// get a result, its readers forbidden, the rest notebook.not_found (the
// notebook entry's edge 404 for those the gate turns away).
func writers(c caller) mcpCell {
	switch role, seen := roleIn(c); {
	case role == "admin" || role == "editor":
		return mcpCell{}
	case seen:
		return mcpCell{code: "forbidden"}
	}
	return mcpCell{code: "notebook.not_found"}
}

// readBack is read's answer of a page: its path, its version and its
// content; the code of a tool's error instead.
func readBack(t *testing.T, a mcpAnswer) (meta struct{ Path, Version string }, text, code string) {
	t.Helper()
	r := a.result(t)
	if r["isError"] == true {
		code, _ = a.toolError(t)["code"].(string)
		return meta, "", code
	}
	content, _ := r["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("read answered %v, want two blocks", content)
	}
	first, _ := content[0].(map[string]any)["text"].(string)
	text, _ = content[1].(map[string]any)["text"].(string)
	if err := json.Unmarshal([]byte(first), &meta); err != nil {
		t.Fatal(err)
	}
	return meta, text, ""
}

// pageWriteMCPRows are page's writes over MCP (M9/P2 design 3.13.2), each
// in both entries, writers' alone: write creates a page, edit, move and
// delete act on the column's page, which no seeded session or link holds;
// each cell's after reads back what it wrote.
func pageWriteMCPRows() []mcpRow {
	const written = "mcp-written.md"
	moved := func(c caller) string {
		p := pagePathOf(c)
		dir := path.Dir(p)
		if dir == "." {
			return pageOf(c) + " moved.md"
		}
		return dir + "/" + pageOf(c) + " moved.md"
	}
	return []mcpRow{
		{
			tool: "write", entries: mcpserver.BothEntries, write: true, want: writers,
			args: func(caller, seeded) map[string]any {
				return map[string]any{"path": written, "content": matrixMark + ", written\n"}
			},
			check: func(t *testing.T, _ caller, _ seeded, a mcpAnswer) {
				t.Helper()
				var got struct {
					Path         string `json:"path"`
					Changed      bool   `json:"changed"`
					CreatedCount int    `json:"created_count"`
				}
				structuredOf(t, a, &got)
				if got.Path != written || !got.Changed || got.CreatedCount != 1 {
					t.Errorf("write answered %.300s, want %s created", a.body, written)
				}
			},
			after: func(t *testing.T, _ caller, _ seeded, call func(string, map[string]any) mcpAnswer) {
				t.Helper()
				meta, text, code := readBack(t, call("read", map[string]any{"path": written}))
				if code != "" || text != matrixMark+", written\n" || !strings.HasSuffix(meta.Version, ":1") {
					t.Errorf("read of the page written = %+v %q %s, want its content at its first version", meta, text, code)
				}
			},
		},
		{
			tool: "edit", entries: mcpserver.BothEntries, write: true, want: writers,
			args: func(c caller, _ seeded) map[string]any {
				return map[string]any{"path": pagePathOf(c), "old_string": matrixMark, "new_string": matrixMark + ", edited"}
			},
			check: func(t *testing.T, c caller, s seeded, a mcpAnswer) {
				t.Helper()
				var got struct {
					Replacements int    `json:"replacements"`
					Version      string `json:"version"`
				}
				structuredOf(t, a, &got)
				if got.Replacements != 1 || got.Version != s.page(pageOf(c)).String()+":2" {
					t.Errorf("edit answered %.300s, want one replacement and the page's version 2", a.body)
				}
			},
			after: func(t *testing.T, c caller, _ seeded, call func(string, map[string]any) mcpAnswer) {
				t.Helper()
				if _, text, code := readBack(t, call("read", map[string]any{"path": pagePathOf(c)})); code != "" ||
					text != matrixMark+", edited\n" {
					t.Errorf("read of the page edited = %q %s", text, code)
				}
			},
		},
		{
			tool: "move", entries: mcpserver.BothEntries, write: true, want: writers,
			args: func(c caller, _ seeded) map[string]any { return map[string]any{"from": pagePathOf(c), "to": moved(c)} },
			check: func(t *testing.T, c caller, s seeded, a mcpAnswer) {
				t.Helper()
				var got struct {
					Path   string `json:"path"`
					NodeID string `json:"node_id"`
				}
				structuredOf(t, a, &got)
				if got.Path != moved(c) || got.NodeID != s.page(pageOf(c)).String() {
					t.Errorf("move answered %.300s, want the page at %s", a.body, moved(c))
				}
			},
			after: func(t *testing.T, c caller, _ seeded, call func(string, map[string]any) mcpAnswer) {
				t.Helper()
				if _, text, code := readBack(t, call("read", map[string]any{"path": moved(c)})); code != "" || text != matrixMark+"\n" {
					t.Errorf("read of the page moved = %q %s", text, code)
				}
				if _, _, code := readBack(t, call("read", map[string]any{"path": pagePathOf(c)})); code != "page.not_found" {
					t.Errorf("read of the path it left = %s, want page.not_found", code)
				}
			},
		},
		{
			tool: "delete", entries: mcpserver.BothEntries, write: true, want: writers,
			args: func(c caller, _ seeded) map[string]any {
				return map[string]any{"path": pagePathOf(c), "recursive": true}
			},
			check: func(t *testing.T, c caller, s seeded, a mcpAnswer) {
				t.Helper()
				var got struct {
					Pages  int    `json:"pages"`
					NodeID string `json:"node_id"`
				}
				structuredOf(t, a, &got)
				if got.Pages != 1 || got.NodeID != s.page(pageOf(c)).String() {
					t.Errorf("delete answered %.300s, want the page alone in the trash", a.body)
				}
			},
			after: func(t *testing.T, c caller, _ seeded, call func(string, map[string]any) mcpAnswer) {
				t.Helper()
				if _, _, code := readBack(t, call("read", map[string]any{"path": pagePathOf(c)})); code != "page.not_found" {
					t.Errorf("read of the page deleted = %s, want page.not_found", code)
				}
			},
		},
	}
}
