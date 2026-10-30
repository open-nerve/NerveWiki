package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// toolLists are the tools of each entry, by the name the contract's file
// keys it with, each the JSON object tools/list answers.
type toolLists map[string][]any

// mcpEntries are the entries by the names api/mcp/tools.json keys them
// with.
func mcpEntries() map[string]mcpserver.Entry {
	return map[string]mcpserver.Entry{"notebook": mcpserver.NotebookEntry, "account": mcpserver.AccountEntry}
}

// toolListsFile is api/mcp/tools.json, found from this file, three
// directories below the repository root (tests run without -trimpath).
func toolListsFile() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "api", "mcp", "tools.json")
}

// readToolLists is the contract's file, decoded.
func readToolLists(t testing.TB) toolLists {
	t.Helper()
	b, err := os.ReadFile(toolListsFile())
	if err != nil {
		t.Fatal(err)
	}
	var lists toolLists
	if err := json.Unmarshal(b, &lists); err != nil {
		t.Fatalf("api/mcp/tools.json: %v", err)
	}
	return lists
}

// listedTools are the tools each entry of m's app lists: a raw handshake
// and tools/list to each, the notebook entry's of alice's notebook, their
// result.tools as the wire has it, every member.
func (m *mcpApp) listedTools(t testing.TB) toolLists {
	t.Helper()
	lists := toolLists{}
	for _, name := range slices.Sorted(maps.Keys(mcpEntries())) {
		path := mcpserver.Pattern
		if mcpEntries()[name] == mcpserver.NotebookEntry {
			path = atNotebook(m.notebook)
		}
		m.post(t, context.Background(), path, m.pat, initializeBody(mcpVersion), nil).result(t)
		r := m.post(t, context.Background(), path, m.pat, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, afterHandshake()).result(t)
		tools, ok := r["tools"].([]any)
		if !ok {
			t.Fatalf("tools/list at %s = %v, want its tools", path, r)
		}
		lists[name] = tools
	}
	return lists
}

// toolListDiff is how got, the tools the entries list, differs from want,
// the file's: a key of the file that is no entry, or an entry it lacks; a
// tool listed and not in the file, or the other way; a tool whose object
// differs in any member, printed both ways, indented, the members sorted.
func toolListDiff(got, want toolLists) []string {
	var diff []string
	for _, key := range slices.Sorted(maps.Keys(want)) {
		if _, ok := mcpEntries()[key]; !ok {
			diff = append(diff, fmt.Sprintf("the file's %q is no entry: its keys are notebook and account", key))
		}
	}
	for _, entry := range slices.Sorted(maps.Keys(mcpEntries())) {
		if _, ok := want[entry]; !ok {
			diff = append(diff, fmt.Sprintf("the file has no %q", entry))
		}
		listed, d := toolsByName(got[entry], "the "+entry+" entry lists")
		diff = append(diff, d...)
		filed, d := toolsByName(want[entry], "the file's "+entry+" has")
		diff = append(diff, d...)
		names := slices.Collect(maps.Keys(listed))
		for name := range filed {
			if _, ok := listed[name]; !ok {
				names = append(names, name)
			}
		}
		slices.Sort(names)
		for _, name := range names {
			g, isListed := listed[name]
			f, isFiled := filed[name]
			switch {
			case !isFiled:
				diff = append(diff, fmt.Sprintf("the %s entry lists %s, which the file does not", entry, name))
			case !isListed:
				diff = append(diff, fmt.Sprintf("the %s entry does not list %s, which the file does", entry, name))
			case !reflect.DeepEqual(g, f):
				diff = append(diff, fmt.Sprintf("the %s entry's %s differs from the file's:\n got %s\nwant %s", entry, name, indented(g), indented(f)))
			}
		}
	}
	return diff
}

// toolsByName are tools by their names; what lists a tool without a name,
// or a name twice, is reported, said says by whom.
func toolsByName(tools []any, said string) (map[string]any, []string) {
	byName, diff := map[string]any{}, []string(nil)
	for i, tool := range tools {
		object, _ := tool.(map[string]any)
		name, ok := object["name"].(string)
		switch {
		case !ok:
			diff = append(diff, fmt.Sprintf("%s a tool without a name, at %d", said, i))
		case byName[name] != nil:
			diff = append(diff, fmt.Sprintf("%s %s twice", said, name))
		default:
			byName[name] = object
		}
	}
	return byName, diff
}

// indented is v as JSON, indented, its members sorted.
func indented(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v (%v)", v, err)
	}
	return string(b)
}

// toolListDiffOfApp is toolListDiff of the lists of an app whose tools
// are the modules' and tools, against the file.
func toolListDiffOfApp(t *testing.T, tools []mcpserver.Tool) []string {
	t.Helper()
	return toolListDiff(startMCPApp(t, tools).listedTools(t), readToolLists(t))
}

// The tools each entry lists are the contract's, api/mcp/tools.json,
// member by member (M9 design 4.1): a tool added, dropped or reworded
// without the file, or the file without the code, fails here with the
// difference. Nothing rewrites the file.
func TestTheToolListsAreTheContract(t *testing.T) {
	for _, d := range toolListDiffOfApp(t, nil) {
		t.Error(d)
	}
}

// seam is a test's tool of entry e alone, without arguments, that
// answers an empty object.
func seam(name string, e mcpserver.Entry) mcpserver.Tool {
	return mcpserver.NewTool(mcpserver.Spec{Name: name, Title: "Seam", Description: "A test's tool.", Entry: e},
		func(context.Context, mcpserver.Call, struct{}) (mcpserver.Result, error) {
			return mcpserver.Result{Structured: map[string]any{}}, nil
		})
}

// unnamedTools are the tools of the file an entry's built-in instructions
// have no line of: a line that starts "- <name>:" or "- <name> " (M9/P1
// design 3.15.4). Anchored at the line's start, so that a tool's name as
// a word of the text, write or move as list is, names no tool.
func unnamedTools(lists toolLists, instructions map[string]string) []string {
	var out []string
	for _, entry := range slices.Sorted(maps.Keys(lists)) {
		for _, tool := range lists[entry] {
			object, _ := tool.(map[string]any)
			name, _ := object["name"].(string)
			if !regexp.MustCompile(`(?m)^- ` + regexp.QuoteMeta(name) + `[: ]`).MatchString(instructions[entry]) {
				out = append(out, fmt.Sprintf("the %s entry's built-in instructions name no %s", entry, name))
			}
		}
	}
	return out
}

// Each entry's built-in instructions, as its handshake sends them (the
// notebook entry's of a notebook without an AGENTS page), have a line of
// each tool the file lists in it (M9/P1 design 3.15.4, review B-M1): a
// phase that adds a tool without its line fails here.
func TestTheBuiltInInstructionsNameEveryTool(t *testing.T) {
	m := startMCPApp(t, nil)
	instructions := map[string]string{
		"notebook": m.instructionsAt(t, atNotebook(m.notebook)),
		"account":  m.instructionsAt(t, mcpserver.Pattern),
	}
	if instructions["notebook"] == instructions["account"] {
		t.Fatalf("the two entries' instructions are the same: %q", instructions["notebook"])
	}
	for _, v := range unnamedTools(readToolLists(t), instructions) {
		t.Error(v)
	}
}

// A tool without its line is reported, and a tool's name in the text
// names no tool: the instructions without read's line, and with list's
// line made a word of the text.
func TestUnnamedToolsCatchesEachGap(t *testing.T) {
	lists := toolLists{"notebook": {map[string]any{"name": "list"}, map[string]any{"name": "read"}},
		"account": {map[string]any{"name": "notebooks"}}}
	text := "Tools\n- list: the children.\n- read: a page's lines.\n"
	instructions := map[string]string{"notebook": text, "account": "- notebooks lists the notebooks.\n"}
	if found := unnamedTools(lists, instructions); len(found) != 0 {
		t.Fatalf("instructions of every tool: %q", found)
	}
	for _, tt := range []struct {
		name, notebook, want string
	}{
		{"without read's line", "Tools\n- list: the children.\n", "the notebook entry's built-in instructions name no read"},
		{"list a word of the text", "Tools\nTo list the children, call list.\n- read: a page's lines.\n",
			"the notebook entry's built-in instructions name no list"},
	} {
		got := unnamedTools(lists, map[string]string{"notebook": tt.notebook, "account": instructions["account"]})
		if !slices.Equal(got, []string{tt.want}) {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

// The check reads what the app lists and what the file holds, each entry
// under its own key: a tool of each entry the test registers beside the
// modules' is that entry's one difference, so that an entry listed for
// the other, or for both, fails too (M9/P1 design review A-M10, S1
// review C-M5).
func TestTheToolListCheckReadsTheApp(t *testing.T) {
	want := []string{"the account entry lists seam_account, which the file does not",
		"the notebook entry lists seam_notebook, which the file does not"}
	got := toolListDiffOfApp(t, []mcpserver.Tool{seam("seam_notebook", mcpserver.NotebookEntry), seam("seam_account", mcpserver.AccountEntry)})
	if !slices.Equal(got, want) {
		t.Errorf("the difference = %q, want %q", got, want)
	}
}

// Each kind of difference is reported: synthetic lists, one change at a
// time.
func TestTheToolListCheckCatchesEachDifference(t *testing.T) {
	tool := func(name string) map[string]any {
		return map[string]any{
			"name": name, "title": name, "description": "Lists the notebook's pages.",
			"inputSchema": map[string]any{
				"type": "object", "additionalProperties": false, "required": []any{"notebook", "pattern"},
				"properties": map[string]any{
					"notebook": map[string]any{"type": "string"},
					"pattern":  map[string]any{"type": []any{"null", "string"}},
				},
			},
			"annotations": map[string]any{"readOnlyHint": true, "idempotentHint": true, "openWorldHint": false},
		}
	}
	lists := func() toolLists {
		return toolLists{"notebook": {tool("list")}, "account": {tool("list"), tool("notebooks")}}
	}
	if d := toolListDiff(lists(), lists()); len(d) != 0 {
		t.Fatalf("the same lists differ: %q", d)
	}
	account := func(l toolLists, i int) map[string]any { return l["account"][i].(map[string]any) }
	schema := func(l toolLists) map[string]any { return account(l, 0)["inputSchema"].(map[string]any) }
	tests := []struct {
		name   string
		change func(got, want toolLists)
		diff   string
	}{
		{"a tool missing", func(got, _ toolLists) { got["notebook"] = nil }, "the notebook entry does not list list, which the file does"},
		{"a tool more", func(got, _ toolLists) { got["account"] = append(got["account"], tool("extra")) },
			"the account entry lists extra, which the file does not"},
		{"a description", func(got, _ toolLists) { account(got, 0)["description"] = "Lists." }, "the account entry's list differs"},
		{"a property's type", func(got, _ toolLists) {
			schema(got)["properties"].(map[string]any)["pattern"] = map[string]any{"type": "string"}
		}, "the account entry's list differs"},
		{"an annotation", func(got, _ toolLists) { account(got, 0)["annotations"].(map[string]any)["readOnlyHint"] = false },
			"the account entry's list differs"},
		{"a member more", func(got, _ toolLists) { account(got, 0)["outputSchema"] = map[string]any{"type": "object"} },
			"the account entry's list differs"},
		{"the required in another order", func(got, _ toolLists) { schema(got)["required"] = []any{"pattern", "notebook"} },
			"the account entry's list differs"},
		{"the entries swapped", func(got, _ toolLists) { got["notebook"], got["account"] = got["account"], got["notebook"] },
			"the notebook entry lists notebooks, which the file does not"},
		{"a tool twice", func(got, _ toolLists) { got["notebook"] = append(got["notebook"], tool("list")) }, "the notebook entry lists list twice"},
		{"the file without an entry", func(_, want toolLists) { delete(want, "account") }, `the file has no "account"`},
		{"the file with a key of no entry", func(_, want toolLists) { want["acount"] = want["account"] }, `the file's "acount" is no entry`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, want := lists(), lists()
			tt.change(got, want)
			if d := toolListDiff(got, want); !slices.ContainsFunc(d, func(s string) bool { return strings.Contains(s, tt.diff) }) {
				t.Errorf("the difference = %q, want one with %q", d, tt.diff)
			}
		})
	}
}

// The modules' tools build both servers on serve's path (buildApp and
// mcpTools), a panic of mcpserver.New reported by its words; each tool is
// listed in the entries its spec says, and in the account entry a tool of
// a notebook takes the notebook first among its required (M9/P1 design
// review A-I1, B-I4).
func TestTheMCPToolsBuildBothServers(t *testing.T) {
	url := pgtest.NewDatabase(t)
	var a *app
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("building the MCP servers panicked: %v", r)
			}
		}()
		a = buildApp(t, testConfig(t, url, false), migrations.FS())
	}()
	lists := runMCPApp(t, a, url, &syncBuffer{}).listedTools(t)
	for _, tool := range a.mcpTools {
		spec := tool.Spec()
		for _, name := range slices.Sorted(maps.Keys(mcpEntries())) {
			entry := mcpEntries()[name]
			byName, _ := toolsByName(lists[name], "")
			listed, _ := byName[spec.Name].(map[string]any)
			if (listed != nil) != (spec.Entry&entry != 0) {
				t.Errorf("%s: listed in the %s entry %v, want %v (its spec's entries: %s)", spec.Name, name, listed != nil, spec.Entry&entry != 0, spec.Entry)
			}
			if listed == nil || entry != mcpserver.AccountEntry || !spec.Notebook {
				continue
			}
			schema, _ := listed["inputSchema"].(map[string]any)
			if required, _ := schema["required"].([]any); len(required) == 0 || required[0] != "notebook" {
				t.Errorf("%s: the account entry's required = %v, want the notebook first", spec.Name, schema["required"])
			}
		}
	}
}

// Independent shape expectations for the two new complete contract objects.
// The existing toolListDiff also compares every member with real tools/list.
func TestTheWikiToolsContract(t *testing.T) {
	lists := readToolLists(t)
	for _, entry := range []string{"notebook", "account"} {
		wantCount := 16
		if entry == "account" {
			wantCount = 18
		}
		if len(lists[entry]) != wantCount {
			t.Fatalf("%s contract has %d tools, want %d", entry, len(lists[entry]), wantCount)
		}
		objects, _ := toolsByName(lists[entry], entry)
		for _, name := range []string{"lint", "pending_sources"} {
			tool, ok := objects[name].(map[string]any)
			if !ok {
				t.Fatalf("%s missing complete tool %s", entry, name)
			}
			schema, ok := tool["inputSchema"].(map[string]any)
			if !ok || schema["type"] != "object" || schema["additionalProperties"] != false {
				t.Fatalf("%s/%s schema=%v", entry, name, schema)
			}
			props, _ := schema["properties"].(map[string]any)
			expected := []string{"cursor", "limit"}
			if name == "lint" {
				expected = append(expected, "rule")
			}
			if entry == "account" {
				expected = append(expected, "notebook")
			}
			slices.Sort(expected)
			if !slices.Equal(slices.Sorted(maps.Keys(props)), expected) {
				t.Fatalf("%s/%s properties=%v, want %v", entry, name, props, expected)
			}
			for _, field := range expected {
				p, ok := props[field].(map[string]any)
				if !ok {
					t.Fatalf("%s/%s field %s is no object", entry, name, field)
				}
				typ := "string"
				if field == "limit" {
					typ = "integer"
				}
				var expectedType any = []any{"null", typ}
				if field == "notebook" {
					expectedType = "string"
				}
				if !reflect.DeepEqual(p["type"], expectedType) || len(p) != 2 || p["description"] == "" {
					t.Fatalf("%s/%s %s=%v, want only nullable scalar/type and description", entry, name, field, p)
				}
			}
			if entry == "account" {
				if !reflect.DeepEqual(schema["required"], []any{"notebook"}) {
					t.Fatalf("%s account required=%v, want notebook first and alone", name, schema["required"])
				}
			} else if _, present := schema["required"]; present {
				t.Fatalf("%s notebook entry has required fields: %v", name, schema)
			}
			annotations := map[string]any{"idempotentHint": false, "openWorldHint": false, "readOnlyHint": true}
			if !reflect.DeepEqual(tool["annotations"], annotations) {
				t.Fatalf("%s/%s annotations=%v", entry, name, tool["annotations"])
			}
			if _, present := tool["outputSchema"]; present {
				t.Fatalf("%s/%s added an output schema the platform does not emit", entry, name)
			}
		}
	}
}
