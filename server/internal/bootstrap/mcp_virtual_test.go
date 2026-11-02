package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// These are real tool calls in both scopes, also registered in the global
// error-code coverage. The assertions below are independent of that code table.
func virtualCodeCalls(t *testing.T, answered answeredCodes) map[string]bool {
	t.Helper()
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Virtual paths")
	m := tm.agent(t, "alice", wiki.id)
	tm.createPage(t, "alice", wiki.id, "", "Real")
	seen := map[string]bool{}
	for _, scope := range []string{"notebook", "account"} {
		for _, tt := range []struct {
			tool string
			args map[string]any
		}{
			{"list", map[string]any{"pattern": "index/"}},
			{"read", map[string]any{"path": "index"}},
			{"read", map[string]any{"path": "log.md.md"}},
			{"write", map[string]any{"path": "index/Child.md", "content": "x"}},
			{"edit", map[string]any{"path": "log.md", "old_string": "x", "new_string": "y"}},
			{"delete", map[string]any{"path": "index.md"}},
			{"move", map[string]any{"from": "log.md", "to": "Destination.md", "update_links": false}},
			{"versions", map[string]any{"path": "log.md"}},
			{"versions", map[string]any{"path": "index.md", "revision": 1}},
			{"log", map[string]any{"path": "index.md"}},
			{"backlinks", map[string]any{"path": "log.md"}},
			{"index", map[string]any{"parent": "index.md"}},
			{"batch", batched(opEdit("log.md", "x", "y"))},
			{"batch", batched(opWrite("index/Child.md", "x"))},
			{"batch", batched(opDelete("index.md"))},
			{"batch", batched(opMove("log.md", "Destination.md"))},
			{"batch", batched(opMove("Real.md", "index/Real.md"))},
			{"batch", batched(opMove("Real.md", "log.md"))},
		} {
			t.Run(scope+"/"+tt.tool, func(t *testing.T) {
				before := sourcePersistentRows(t, tm, wiki.id)
				e := sourceMCPCall(t, m, scope, tt.tool, tt.args).toolError(t)
				message, _ := e["message"].(string)
				if e["code"] != "page.virtual_file" || !strings.Contains(message, "generated") {
					t.Errorf("%s %s = %v; want generated-file refusal", scope, tt.tool, e)
				} else {
					seen[scope+"/"+tt.tool] = true
					answered.add(tt.tool, "page.virtual_file")
				}
				if !slices.Equal(sourcePersistentRows(t, tm, wiki.id), before) {
					t.Error("virtual refusal changed persistent rows")
				}
			})
		}
	}
	checkPages(t, tm.pool)
	return seen
}

func TestVirtualPaths(t *testing.T) {
	t.Run("lookup", func(t *testing.T) {
		seen := virtualCodeCalls(t, answeredCodes{})
		for _, scope := range []string{"notebook", "account"} {
			for _, tool := range []string{"list", "read", "write", "edit", "delete", "move", "versions", "log", "backlinks", "index", "batch"} {
				if !seen[scope+"/"+tool] {
					t.Errorf("no actual virtual refusal for %s/%s", scope, tool)
				}
			}
		}
	})
	t.Run("batch", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		wiki := tm.llmWiki(t, "Virtual batch")
		locked := tm.createPageWith(t, "alice", wiki.id, "", "Locked", "original")
		tm.openSession(t, "bob", locked)
		m := tm.agent(t, "alice", wiki.id)
		for _, scope := range []string{"notebook", "account"} {
			for _, reverse := range []bool{false, true} {
				ops := []map[string]any{opEdit("Locked.md", "original", "changed"), opDelete("index.md")}
				want := float64(1)
				if reverse {
					slices.Reverse(ops)
					want = 0
				}
				before := sourcePersistentRows(t, tm, wiki.id)
				e := sourceMCPCall(t, m, scope, "batch", batched(ops...)).toolError(t)
				if e["code"] != "page.virtual_file" || e["op"] != want {
					t.Errorf("%s reverse=%v: %v, want virtual at %v before lock", scope, reverse, e, want)
				}
				if !slices.Equal(before, sourcePersistentRows(t, tm, wiki.id)) {
					t.Error("refused batch changed rows")
				}
			}
		}
		checkPages(t, tm.pool)
	})
	t.Run("move_destination", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		wiki := tm.llmWiki(t, "Virtual destination")
		m := tm.agent(t, "alice", wiki.id)
		tm.createPage(t, "alice", wiki.id, "", "Real")
		tm.createPage(t, "alice", wiki.id, "", "Folder")
		for _, scope := range []string{"notebook", "account"} {
			before := sourcePersistentRows(t, tm, wiki.id)
			e := sourceMCPCall(t, m, scope, "move", map[string]any{"from": "Real.md", "to": "index.md", "update_links": false}).toolError(t)
			if e["code"] != "page.virtual_file" {
				t.Errorf("destination: %v", e)
			}
			if !slices.Equal(before, sourcePersistentRows(t, tm, wiki.id)) {
				t.Error("destination refusal changed real position or history")
			}
			structured(t, sourceMCPCall(t, m, scope, "move", map[string]any{"from": "Real.md", "to": "Folder/index.md", "update_links": false}))
			structured(t, sourceMCPCall(t, m, scope, "move", map[string]any{"from": "Folder/index.md", "to": "Real.md", "update_links": false}))
		}
		checkPages(t, tm.pool)
	})
	t.Run("real_paths", virtualRealPaths)
	t.Run("list_directory", virtualListDirectory)
	t.Run("pagination", virtualPagination)
	t.Run("metadata", virtualMetadata)
	t.Run("overview", virtualOverview)
	t.Run("fit_boundary", virtualFitBoundary)
	t.Run("field_order", virtualFieldOrder)
	t.Run("read_window", virtualReadWindow)
	t.Run("read_fresh", virtualReadFresh)
	t.Run("tool_dependency", virtualToolDependency)
	t.Run("source_line", virtualSourceLine)
}

// Canonical reads reach the generator through both real MCP entries. Aliases
// remain refusals in virtualCodeCalls, and nested paths remain real above.
func virtualReadWindow(t *testing.T) {
	t.Run("renderer", virtualRenderedReadWindow)
	t.Run("synthetic_longline", virtualSyntheticReadWindow)
}

func virtualRenderedReadWindow(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Virtual read window")
	tm.createPageWith(t, "alice", wiki.id, "", "Visible", "---\ntype: note\nsummary: visible\n---\nbody")
	m := tm.agent(t, "alice", wiki.id)
	for _, scope := range []string{"notebook", "account"} {
		for _, path := range []string{"index.md", "log.md"} {
			t.Run(scope+"/"+path, func(t *testing.T) {
				res := sourceMCPCall(t, m, scope, "read", map[string]any{"path": path}).result(t)
				if res["isError"] == true {
					t.Fatalf("canonical virtual read refused: %v", res)
				}
				blocks, meta := blocksOf(t, res), metaOf(t, res)
				if len(blocks) != 2 || meta["virtual"] != true || meta["path"] != path || meta["kind"] != "page" {
					t.Fatalf("virtual read metadata or text missing: %v", res)
				}
				if _, ok := meta["version"]; ok {
					t.Fatal("virtual read exposed a version")
				}
				if text, _ := blocks[1]["text"].(string); !strings.Contains(text, "Visible") {
					t.Fatalf("virtual read omitted the real page: %q", text)
				}
			})
		}
	}
	// A hand-written first-line oracle catches a generator bypassing Cut.
	for _, scope := range []string{"notebook", "account"} {
		res := sourceMCPCall(t, m, scope, "read", map[string]any{"path": "index.md", "offset": 1, "limit": 1}).result(t)
		meta := metaOf(t, res)
		if blocksOf(t, res)[1]["text"] != "# Index\n" || meta["lines"] != float64(1) || meta["next_offset"] != float64(2) || meta["next_column"] != nil {
			t.Fatalf("hand-written header window: %v", res)
		}
	}
	// Valid long tags and ampersands expand on the actual SDK wire. These complete
	// long lines fit the file profile but need more than one read response.
	tags := make([]string, 20)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%02d%s", i, strings.Repeat("_", 240))
	}
	front, _ := json.Marshal(map[string]any{"type": "a", "tags": tags, "summary": strings.Repeat("&", 256)})
	for i := range 12 {
		tm.createPageWith(t, "alice", wiki.id, "", fmt.Sprintf("é漢%03d", i)+strings.Repeat("&", 220), "---\n"+string(front)+"\n---\nbody")
	}
	for _, scope := range []string{"notebook", "account"} {
		endpoint := atNotebook(wiki.id)
		args := map[string]any{"path": "index.md"}
		if scope == "account" {
			endpoint = mcpserver.Pattern
			args["notebook"] = wiki.id
		}
		client := mcpClient(t, m.base+endpoint, m.pat)
		res, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "read", Arguments: args})
		if err != nil || res.IsError || len(res.Content) != 2 {
			t.Fatalf("SDK read: %v %+v", err, res)
		}
		wire, err := json.Marshal(res)
		if err != nil || len(wire) > 48<<10 {
			t.Fatalf("SDK full result = %d bytes: %v", len(wire), err)
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &meta); err != nil {
			t.Fatal(err)
		}
		if meta["next_offset"] == nil {
			t.Fatalf("escaping fixture did not force a continued read: %d SDK bytes, metadata %v", len(wire), meta)
		}
		args["offset"], args["column"], args["limit"] = 7, 5, 1
		res, err = client.CallTool(context.Background(), &mcp.CallToolParams{Name: "read", Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("UTF-8 window: %v %+v", err, res)
		}
		text := res.Content[1].(*mcp.TextContent).Text
		if !utf8.ValidString(text) || !strings.HasPrefix(text, "漢000&&&&") || len(text) < 4000 || !strings.Contains(text, "tags: tag00") {
			t.Fatalf("byte column 5 did not start after é: %.120q", text)
		}
		if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &meta); err != nil {
			t.Fatal(err)
		}
		if meta["offset"] != float64(7) || meta["column"] != float64(5) || meta["lines"] != float64(1) || meta["next_offset"] != float64(8) || meta["next_column"] != nil {
			t.Fatalf("long-line continuation metadata: %v", meta)
		}
		args["column"] = 4
		res, err = client.CallTool(context.Background(), &mcp.CallToolParams{Name: "read", Arguments: args})
		if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "validation_failed") {
			t.Fatalf("interior UTF-8 column was accepted: %v %+v", err, res)
		}
	}
}

type virtualLongText string

func (v virtualLongText) GenerateInSnapshot(_ context.Context, _ page.VirtualReadScope, name page.VirtualName) (string, error) {
	if name != page.VirtualIndex {
		return "", fmt.Errorf("synthetic virtual fixture: unexpected file")
	}
	return string(v), nil
}

// The renderer fixture covers complete lines. This isolated transport adds
// a synthetic long line to exercise column boundaries, using the
// real page.Tools/ReadPath/TxManager and SDK with synthetic file contents.
// Authentication is a supplied real actor here; the renderer test above goes
// through serve's real authentication and both production tool registrations.
func virtualSyntheticReadWindow(t *testing.T) {
	var deps page.Deps
	tm := newAcmeTeamWiring(t, "member", "", wiring{page: func(d *page.Deps) { deps = *d }})
	wiki := tm.llmWiki(t, "Synthetic SDK boundary")
	m := tm.agent(t, "alice", wiki.id)
	actor := shared.Actor{UserID: tm.userID(t, "alice")}
	if err := tm.pool.QueryRow(context.Background(), "SELECT id FROM api_tokens WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1", actor.UserID).Scan(&actor.APITokenID); err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("é漢&", 12000) + "ABCDE\nend\n"
	pg := page.New(deps)
	logger := slog.New(slog.DiscardHandler)
	server := mcpserver.New(mcpserver.Options{
		Name: "virtual-window-test", Version: "1", Tools: pg.Tools(page.ToolDeps{Attachments: virtualNoAttachments{}, Virtual: virtualLongText(content)}),
		Credentials: mcpCredentials, Notebooks: pg.MCPEntry(), Errors: httpserver.NewAPIErrors(logger),
		RequestTimeout: 4 * time.Second, MaxBodyBytes: mcpBodyLimit, Logger: logger,
	})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r.WithContext(shared.WithActor(r.Context(), actor)))
	}))
	t.Cleanup(httpServer.Close)
	for _, scope := range []string{"notebook", "account"} {
		t.Run(scope, func(t *testing.T) {
			endpoint := atNotebook(wiki.id)
			args := map[string]any{"path": "index.md"}
			if scope == "account" {
				endpoint = mcpserver.Pattern
				args["notebook"] = wiki.id
			}
			client := mcpClient(t, httpServer.URL+endpoint, m.pat)
			read := func() (map[string]any, string) {
				t.Helper()
				res, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "read", Arguments: args})
				if err != nil || res.IsError || len(res.Content) != 2 {
					t.Fatalf("synthetic SDK read: %v %+v", err, res)
				}
				wire, err := json.Marshal(res)
				if err != nil || len(wire) > 48<<10 {
					t.Fatalf("synthetic SDK full result: %d bytes, %v", len(wire), err)
				}
				var meta map[string]any
				if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &meta); err != nil {
					t.Fatal(err)
				}
				if meta["virtual"] != true || meta["version"] != nil {
					t.Fatalf("synthetic virtual metadata: %v", meta)
				}
				return meta, res.Content[1].(*mcp.TextContent).Text
			}
			meta, text := read()
			column, ok := meta["next_column"].(float64)
			if !ok || column <= 0 || column >= 72005 || meta["next_offset"] != float64(1) || meta["lines"] != float64(1) || !utf8.ValidString(text) || len(text) != int(column) || text != content[:int(column)] {
				t.Fatalf("long line lost byte continuation: %v, %d bytes", meta, len(text))
			}
			// The last repetition starts at byte 71994; é ends at 71996.
			// This literal suffix is independent of Cut and its wire measure.
			args["offset"], args["column"], args["limit"] = 1, 71996, 1
			meta, text = read()
			if text != "漢&ABCDE\n" || meta["next_offset"] != float64(2) || meta["next_column"] != nil {
				t.Fatalf("hand-written UTF-8 tail: %q %v", text, meta)
			}
			args["offset"], args["column"] = 2, 0
			meta, text = read()
			if text != "end\n" || meta["next_offset"] != nil || meta["next_column"] != nil {
				t.Fatalf("last line: %q %v", text, meta)
			}
			args["offset"], args["column"] = 1, 71995
			res, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "read", Arguments: args})
			if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "validation_failed") {
				t.Fatalf("split é accepted: %v %+v", err, res)
			}
		})
	}
}

func virtualSourceLine(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Source file line")
	tm.createPageWith(t, "alice", wiki.id, wiki.raw, "One", "one")
	tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Two", "two")
	tm.createPage(t, "alice", wiki.id, wiki.raw, "Empty")
	tm.upload(t, "alice", wiki.id, wiki.raw, "asset.txt", "x")
	m := tm.agent(t, "alice", wiki.id)
	for _, scope := range []string{"notebook", "account"} {
		res := sourceMCPCall(t, m, scope, "read", map[string]any{"path": "index.md"}).result(t)
		text := blocksOf(t, res)[1]["text"].(string)
		if !strings.Contains(text, "Source area: [raw.md](<raw.md>) — 3 sources.") || !strings.Contains(text, "Showing 2 of 2 pages.") || strings.Contains(text, "raw/One.md") || strings.Contains(text, "raw/Empty.md") {
			t.Fatalf("wired source membership/count: %q", text)
		}
	}
}

func virtualReadFresh(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Fresh virtual reads")
	m := tm.agent(t, "alice", wiki.id)
	for _, scope := range []string{"notebook", "account"} {
		before := map[string]string{}
		for _, path := range []string{"index.md", "log.md"} {
			before[path] = blocksOf(t, sourceMCPCall(t, m, scope, "read", map[string]any{"path": path}).result(t))[1]["text"].(string)
		}
		name := "Fresh" + scope
		structured(t, sourceMCPCall(t, m, scope, "write", map[string]any{"path": name + ".md", "content": "---\ntype: note\n---\nfresh"}))
		for _, path := range []string{"index.md", "log.md"} {
			after := blocksOf(t, sourceMCPCall(t, m, scope, "read", map[string]any{"path": path}).result(t))[1]["text"].(string)
			if before[path] == after || strings.Contains(before[path], name) || !strings.Contains(after, name+".md") {
				t.Fatalf("%s/%s did not regenerate after a real write", scope, path)
			}
		}
	}
}

type virtualNoAttachments struct{}

func (virtualNoAttachments) Read(context.Context, uuid.UUID) (page.Attachment, error) {
	return page.Attachment{}, page.ErrNotFound
}
func (virtualNoAttachments) Content(context.Context, page.Attachment, int64) ([]byte, int, int, error) {
	return nil, 0, 0, page.ErrNotFound
}

func virtualToolDependency(t *testing.T) {
	var deps page.Deps
	tm := newAcmeTeamWiring(t, "member", "", wiring{page: func(d *page.Deps) { deps = *d }})
	wiki := tm.llmWiki(t, "Tool dependency")
	m := tm.agent(t, "alice", wiki.id)
	tm.createPageWith(t, "alice", wiki.id, "", "Real", "ordinary")
	res := sourceMCPCall(t, m, "notebook", "read", map[string]any{"path": "Real.md"}).result(t)
	if blocksOf(t, res)[1]["text"] != "ordinary" {
		t.Fatal("provided dependency changed an ordinary read")
	}
	func() {
		defer func() {
			if got := recover(); got != "page: no virtual files for the MCP tools" {
				t.Errorf("Tools accepted missing Virtual or panicked elsewhere: %v", got)
			}
		}()
		page.New(deps).Tools(page.ToolDeps{Attachments: virtualNoAttachments{}})
	}()
}

func virtualRealPaths(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Nested real paths")
	standard := tm.createWith(t, tm.tokens["alice"], `{"name":"Standard real paths","workspace_access":"editor"}`)
	for _, scope := range []string{"notebook", "account"} {
		for _, nb := range []string{wiki.id, standard.ID} {
			prefix := ""
			if nb == wiki.id {
				prefix = "Folder" + scope + "/"
			} else {
				prefix = ""
			}
			m := tm.agent(t, "alice", nb)
			structured(t, sourceMCPCall(t, m, scope, "write", map[string]any{"path": prefix + "index.md", "content": "original"}))
			for _, tt := range []struct {
				tool string
				args map[string]any
			}{
				{"read", map[string]any{"path": prefix + "index.md"}},
				{"versions", map[string]any{"path": prefix + "index.md"}},
				{"log", map[string]any{"path": prefix + "index.md"}},
				{"backlinks", map[string]any{"path": prefix + "index.md"}},
				{"index", map[string]any{"parent": prefix + "index.md"}},
				{"list", map[string]any{"pattern": prefix + "index/"}},
				{"edit", map[string]any{"path": prefix + "index.md", "old_string": "original", "new_string": "changed"}},
				{"move", map[string]any{"from": prefix + "index.md", "to": prefix + "log.md", "update_links": false}},
				{"batch", batched(opEdit(prefix+"log.md", "changed", "final"))},
				{"delete", map[string]any{"path": prefix + "log.md"}},
			} {
				a := sourceMCPCall(t, m, scope, tt.tool, tt.args)
				if a.result(t)["isError"] == true {
					t.Fatalf("real %s %s: %.500s", scope, tt.tool, a.body)
				}
			}
		}
	}
	checkPages(t, tm.pool)
}

func virtualMetadata(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Virtual metadata")
	empty := tm.createPage(t, "alice", wiki.id, "", "Empty")
	tm.upload(t, "alice", wiki.id, "", "one.txt", "x")
	m := tm.agent(t, "alice", wiki.id)
	for _, scope := range []string{"notebook", "account"} {
		a := structured(t, sourceMCPCall(t, m, scope, "list", map[string]any{}))
		items := a["items"].([]any)
		if len(items) != 6 {
			t.Fatalf("list has %d items, want two virtual + three pages + one asset", len(items))
		}
		for i, name := range []string{"index.md", "log.md"} {
			item := items[i].(map[string]any)
			if item["path"] != name || item["virtual"] != true || item["kind"] != "page" {
				t.Errorf("virtual metadata: %v", item)
			}
			for _, field := range []string{"version", "byte_size", "updated_at"} {
				if _, ok := item[field]; ok {
					t.Errorf("virtual item exposes %s", field)
				}
			}
		}
		sawEmpty := false
		for _, raw := range items {
			item := raw.(map[string]any)
			if item["path"] == "Empty.md" {
				sawEmpty = true
				if item["byte_size"] != float64(0) || item["version"] != empty+":1" {
					t.Errorf("real empty metadata lost: %v", item)
				}
			}
		}
		if !sawEmpty {
			t.Error("empty real page missing")
		}
		for _, path := range []string{"index.md", "log.md", "Empty.md"} {
			res := sourceMCPCall(t, m, scope, "read", map[string]any{"path": path}).result(t)
			meta := metaOf(t, res)
			if path == "Empty.md" {
				if meta["version"] != empty+":1" || meta["byte_size"] != float64(0) || meta["virtual"] != nil {
					t.Errorf("real empty read lost metadata: %v", meta)
				}
			} else if _, exists := meta["version"]; exists || meta["virtual"] != true {
				t.Errorf("virtual read identity: %v", meta)
			}
		}
	}
	a := structured(t, m.call(t, mcpserver.Pattern, "open_notebook", map[string]any{"notebook": wiki.id}))
	o := a["overview"].(map[string]any)
	if o["pages"] != float64(3) || o["attachments"] != float64(1) {
		t.Errorf("overview counts virtual files: %v", o)
	}
	root := o["root"].([]any)
	if root[0].(map[string]any)["path"] != "index.md" || root[1].(map[string]any)["path"] != "log.md" {
		t.Errorf("overview virtual prefix missing: %v", root)
	}
	checkPages(t, tm.pool)
}

func virtualOverview(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Virtual overview")
	m := tm.agent(t, "alice", wiki.id)
	var seeds []sqlPage
	for i := range 96 {
		seeds = append(seeds, sqlPage{name: fmt.Sprintf("P%03d", i)})
	}
	tm.seedSQL(t, wiki.id, seeds)
	for _, n := range []int{98, 99, 100} {
		if n > 98 {
			tm.seedSQL(t, wiki.id, []sqlPage{{name: fmt.Sprintf("P%03d", n)}})
		}
		a := structured(t, m.call(t, mcpserver.Pattern, "open_notebook", map[string]any{"notebook": wiki.id}))
		o := a["overview"].(map[string]any)
		root := o["root"].([]any)
		if len(root) != 100 || a["truncated"] != (n > 98) || o["pages"] != float64(n) || root[0].(map[string]any)["path"] != "index.md" || root[1].(map[string]any)["path"] != "log.md" {
			t.Errorf("%d roots: len=%d truncated=%v counts=%v", n, len(root), a["truncated"], o["pages"])
		}
	}
	checkPages(t, tm.pool)
}

func virtualFieldOrder(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Virtual field order")
	m := tm.agent(t, "alice", wiki.id)
	unseen := uuid.NewV7().String()
	for _, tt := range []struct {
		tool string
		args map[string]any
		want string
	}{
		{"list", map[string]any{"pattern": "index/", "cursor": "bad", "limit": 0}, "bad_request"},
		{"list", map[string]any{"pattern": "index/", "limit": 0}, "validation_failed"},
		{"read", map[string]any{"path": "index", "offset": 0}, "validation_failed"},
		{"edit", map[string]any{"path": "log.md", "old_string": "", "new_string": "x"}, "validation_failed"},
	} {
		for _, scope := range []string{"notebook", "account"} {
			if e := sourceMCPCall(t, m, scope, tt.tool, tt.args).toolError(t); e["code"] != tt.want {
				t.Errorf("field order %s %s: %v", scope, tt.tool, e)
			}
		}
	}
	for _, scope := range []string{"notebook", "account"} {
		args := map[string]any{"pattern": "index/"}
		path := atNotebook(unseen)
		if scope == "account" {
			path = mcpserver.Pattern
			args["notebook"] = unseen
		}
		a := m.call(t, path, "list", args)
		if scope == "notebook" {
			if a.problem(t, http.StatusNotFound) != "notebook.not_found" {
				t.Error("notebook gate leaked virtual path")
			}
		} else if e := a.toolError(t); e["code"] != "notebook.not_found" {
			t.Errorf("account leaked virtual path: %v", e)
		}
	}
	hidden := tm.createWith(t, tm.tokens["alice"], `{"name":"Hidden virtual","mode":"llm_wiki","workspace_access":"none"}`)
	reader := tm.agent(t, "bob", hidden.ID)
	for _, scope := range []string{"notebook", "account"} {
		a := sourceMCPCall(t, reader, scope, "list", map[string]any{"pattern": "index/"})
		if scope == "notebook" {
			if a.problem(t, http.StatusNotFound) != "notebook.not_found" {
				t.Error("hidden notebook gate leaked")
			}
		} else if e := a.toolError(t); e["code"] != "notebook.not_found" {
			t.Errorf("hidden account path leaked: %v", e)
		}
		bad := sourceMCPCall(t, reader, scope, "list", map[string]any{"pattern": "index/", "cursor": "bad", "limit": 0})
		if scope == "notebook" {
			if bad.problem(t, http.StatusNotFound) != "notebook.not_found" {
				t.Error("outer gate order changed")
			}
		} else if e := bad.toolError(t); e["code"] != "bad_request" {
			t.Errorf("account cursor order changed: %v", e)
		}
		anon := *m
		anon.pat = ""
		if a := sourceMCPCall(t, &anon, scope, "list", map[string]any{"pattern": "index/"}); a.status != http.StatusUnauthorized {
			t.Errorf("anonymous %s status=%d", scope, a.status)
		}
	}

}

// The official SDK sees an actual Fit reduction with index as the final
// included item, then resumes at log. The filler varies only inside a legal
// 255-byte root title; no path or test deadline is enlarged.
func virtualFitBoundary(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Virtual fit")
	m := tm.agent(t, "alice", wiki.id)
	var seeds []sqlPage
	for i := range 26 {
		seeds = append(seeds, sqlPage{name: fmt.Sprintf("%03d-", i) + strings.Repeat("&", 251)})
	}
	for i := range 10 {
		seeds = append(seeds, sqlPage{name: fmt.Sprintf("ib%02d", i)})
	}
	seeds = append(seeds, sqlPage{name: "Fill0"}, sqlPage{name: "Fill1"})
	for _, seed := range seeds {
		tm.createPage(t, "alice", wiki.id, "", seed.name)
	}
	fillers := queryStrings(t, tm.pool, `SELECT id::text FROM nodes WHERE notebook_id=$1 AND name IN ('Fill0','Fill1') ORDER BY name`, wiki.id)
	for _, scope := range []string{"notebook", "account"} {
		endpoint := atNotebook(wiki.id)
		args := map[string]any{"pattern": "*.md", "limit": 1000}
		if scope == "account" {
			endpoint = mcpserver.Pattern
			args["notebook"] = wiki.id
		}
		client := mcpClient(t, m.base+endpoint, m.pat)
		call := func() (*mcp.CallToolResult, map[string]any) {
			res, err := client.CallTool(t.Context(), &mcp.CallToolParams{Name: "list", Arguments: args})
			if err != nil || res.IsError {
				t.Fatalf("SDK list: %+v %v", res, err)
			}
			sc, ok := res.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("SDK structured: %T", res.StructuredContent)
			}
			return res, sc
		}
		var result *mcp.CallToolResult
		var answer map[string]any
		found := false
		var fillNames []string
		for lo, hi := 0, 250; lo <= hi; {
			n := (lo + hi) / 2
			fillNames = []string{"Fill0" + strings.Repeat("&", n), "Fill1" + strings.Repeat("&", n)}
			for i, id := range fillers {
				tm.originExec(t, `UPDATE nodes SET name=$2,name_key=lower($2) WHERE id=$1`, id, fillNames[i])
			}
			result, answer = call()
			items := answer["items"].([]any)
			last := items[len(items)-1].(map[string]any)["path"].(string)
			if last == "index.md" {
				found = true
				break
			}
			hasIndex := slices.ContainsFunc(items, func(v any) bool { return v.(map[string]any)["path"] == "index.md" })
			if hasIndex {
				lo = n + 1
			} else {
				hi = n - 1
			}
		}
		if !found {
			items := answer["items"].([]any)
			t.Fatalf("fixture did not reach index/log Fit boundary: %d items, tail %v", len(items), items[len(items)-1].(map[string]any)["path"])
		}
		items := answer["items"].([]any)
		var want, got []string
		for i := range 26 {
			want = append(want, fmt.Sprintf("%03d-", i)+strings.Repeat("&", 251)+".md")
		}
		want = append(want, "AGENTS.md", fillNames[0]+".md", fillNames[1]+".md")
		for i := range 10 {
			want = append(want, fmt.Sprintf("ib%02d.md", i))
		}
		want = append(want, "index.md")
		for _, item := range items {
			got = append(got, item.(map[string]any)["path"].(string))
		}
		if !slices.Equal(got, want) {
			t.Fatalf("Fit first segment differs from all %d seeded paths through index: got %d", len(want), len(got))
		}
		if len(items) != 40 || answer["next_cursor"] == nil {
			t.Fatalf("Fit did not shrink its 42 candidates: %d next=%v", len(items), answer["next_cursor"])
		}
		bytes := 0
		for _, block := range result.Content {
			if text, ok := block.(*mcp.TextContent); ok {
				encoded, err := json.Marshal(text.Text)
				if err != nil {
					t.Fatal(err)
				}
				bytes += len(encoded) - 2
			}
		}
		if bytes > 48<<10 {
			t.Fatalf("SDK text serialization = %d bytes", bytes)
		}
		t.Logf("%s: SDK Fit kept %d/42, tail index.md, serialized text %d bytes", scope, len(items), bytes)
		args["cursor"] = answer["next_cursor"]
		_, next := call()
		tail := next["items"].([]any)
		if len(tail) != 2 || tail[0].(map[string]any)["path"] != "log.md" || tail[1].(map[string]any)["path"] != "raw.md" || next["next_cursor"] != nil {
			t.Fatalf("Fit resume: %v", next)
		}
	}
	checkPages(t, tm.pool)
}

func virtualListDirectory(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Virtual directory")
	parent := tm.createPage(t, "alice", wiki.id, "", "Real")
	child := tm.createPage(t, "alice", wiki.id, parent, "index")
	m := tm.agent(t, "alice", wiki.id)
	for _, scope := range []string{"notebook", "account"} {
		for _, pattern := range []string{"index/", "log/", "index.md/", "log.md/"} {
			e := sourceMCPCall(t, m, scope, "list", map[string]any{"pattern": pattern}).toolError(t)
			if e["code"] != "page.virtual_file" {
				t.Errorf("%s %s: %v", scope, pattern, e)
			}
		}
		a := structured(t, sourceMCPCall(t, m, scope, "list", map[string]any{"pattern": "Real/"}))
		items := a["items"].([]any)
		if len(items) != 1 || items[0].(map[string]any)["version"] != child+":1" {
			t.Errorf("real directory lost its child: %v", a)
		}
	}
}
func virtualPagination(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Virtual pagination")
	m := tm.agent(t, "alice", wiki.id)
	var ids []string
	for i, order := range []int{-3, -2, -1, 0, 1} {
		id := tm.createPage(t, "alice", wiki.id, "", fmt.Sprintf("Real%d", i))
		ids = append(ids, id)
		tm.originExec(t, `UPDATE nodes SET sort_order=$2 WHERE id=$1`, id, order)
	}
	tm.originExec(t, `UPDATE nodes SET sort_order=10 WHERE id=$1`, wiki.raw)
	tm.originExec(t, `UPDATE nodes SET sort_order=11 WHERE id=$1`, wiki.agents)
	wantIDs := append(slices.Clone(ids), wiki.raw, wiki.agents)
	want := []string{"index.md", "log.md", "Real0.md", "Real1.md", "Real2.md", "Real3.md", "Real4.md", "raw.md", "AGENTS.md"}
	for _, scope := range []string{"notebook", "account"} {
		for _, limit := range []int{1, 2} {
			args := map[string]any{"limit": limit}
			var paths, gotIDs []string
			ended := false
			for range 10 {
				a := structured(t, sourceMCPCall(t, m, scope, "list", args))
				for _, v := range a["items"].([]any) {
					item := v.(map[string]any)
					paths = append(paths, item["path"].(string))
					if version, ok := item["version"].(string); ok {
						gotIDs = append(gotIDs, strings.Split(version, ":")[0])
					}
				}
				if a["next_cursor"] == nil {
					ended = true
					break
				}
				args["cursor"] = a["next_cursor"]
			}
			if !ended || !slices.Equal(paths, want) || !slices.Equal(gotIDs, wantIDs) {
				t.Errorf("%s limit%d ended=%v paths=%v IDs=%v", scope, limit, ended, paths, gotIDs)
			}
		}
	}
	checkPages(t, tm.pool)
}
