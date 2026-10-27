package bootstrap

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// sourceCodeCalls contributes real calls to the existing per-tool code
// coverage, while retaining scope in its own observed results. The independent
// assertion below detects dropping one entry even when the other still answers.
func sourceCodeCalls(t *testing.T, answered answeredCodes) map[string]bool {
	t.Helper()
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Source codes")
	page := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Source", "source original\n")
	asset := tm.upload(t, "alice", wiki.id, wiki.raw, "source.txt", "source asset")
	tm.createPageWith(t, "alice", wiki.id, "", "Outside", "source original\n")
	empty := tm.createPage(t, "bob", wiki.id, wiki.raw, "Empty")
	tm.upload(t, "alice", wiki.id, "", "outside.txt", "source asset")
	large := tm.createPage(t, "alice", wiki.id, "", "Large")
	tm.addSourceEntryPages(t, large, 10000)
	checkPages(t, tm.pool)
	writer := tm.agent(t, "bob", wiki.id)
	seen := map[string]bool{}
	for _, scope := range []string{"notebook", "account"} {
		for _, tt := range []struct {
			name, tool, code string
			args             map[string]any
			op               int
			opTool           string
			duplicate        string
		}{
			{"write existing", "write", "page.source_immutable", map[string]any{"path": "raw/Source.md", "content": "replacement", "base_version": page + ":1"}, -1, "", ""},
			{"edit existing", "edit", "page.source_immutable", map[string]any{"path": "raw/Source.md", "old_string": "original", "new_string": "replacement", "base_version": page + ":1"}, -1, "", ""},
			{"batch actual second item", "batch", "page.source_immutable", batched(opWrite("Unwritten.md", "unwritten"), opEdit("raw/Source.md", "original", "replacement")), 1, "edit", ""},
			{"delete source page", "delete", "page.source_admin_only", map[string]any{"path": "raw/Source.md"}, -1, "", ""},
			{"delete source asset", "delete", "page.source_admin_only", map[string]any{"path": "raw/source.txt"}, -1, "", ""},
			{"move source entry too large", "move", "page.source_too_large", map[string]any{"from": "Large.md", "to": "raw/Large.md", "update_links": false}, -1, "", ""},
			{"batch actual second move too large", "batch", "page.source_too_large", batched(opWrite("Unwritten.md", "unwritten"), opMove("Large.md", "raw/Large.md")), 1, "move", ""},
			{"write new duplicate", "write", "page.source_duplicate", map[string]any{"path": "raw/Duplicate.md", "content": "source original\n"}, -1, "", page},
			{"write first duplicate", "write", "page.source_duplicate", map[string]any{"path": "raw/Empty.md", "content": "source original\n", "base_version": empty + ":1"}, -1, "", page},
			{"move duplicate page", "move", "page.source_duplicate", map[string]any{"from": "Outside.md", "to": "raw/Outside.md", "update_links": false}, -1, "", page},
			{"move duplicate attachment", "move", "page.source_duplicate", map[string]any{"from": "outside.txt", "to": "raw/outside.txt", "update_links": false}, -1, "", asset.ID},
			{"batch second duplicate write", "batch", "page.source_duplicate", batched(opWrite("Unwritten.md", "unwritten"), opWrite("raw/Duplicate.md", "source original\n")), 1, "write", page},
			{"batch second duplicate move", "batch", "page.source_duplicate", batched(opWrite("Unwritten.md", "unwritten"), opMove("Outside.md", "raw/Outside.md")), 1, "move", page},
		} {
			t.Run(scope+"/"+tt.name, func(t *testing.T) {
				before := sourcePersistentRows(t, tm, wiki.id)
				a := sourceMCPCall(t, writer, scope, tt.tool, tt.args)
				member := a.toolError(t)
				message, _ := member["message"].(string)
				if member["code"] != tt.code || !strings.HasPrefix(message, tt.code+": ") {
					t.Errorf("%s %s = %v, want %s", scope, tt.tool, member, tt.code)
				} else {
					seen[scope+"/"+tt.tool+"/"+tt.code] = true
					answered.add(tt.tool, tt.code)
				}
				if tt.op >= 0 && (member["op"] != float64(tt.op) || !strings.Contains(message, fmt.Sprintf("ops[%d] (%s)", tt.op, tt.opTool))) {
					t.Errorf("batch refusal = %v, want actual ops[%d] (%s)", member, tt.op, tt.opTool)
				}
				if tt.duplicate != "" {
					path := "raw/Source.md"
					if tt.duplicate == asset.ID {
						path = "raw/source.txt"
					}
					if member["duplicate_of"] != tt.duplicate || !strings.Contains(message, path) {
						t.Errorf("duplicate target = %v, want %s with visible path %q", member, tt.duplicate, path)
					}
				}
				if after := sourcePersistentRows(t, tm, wiki.id); !slices.Equal(after, before) {
					t.Errorf("refused %s %s changed persistent nodes, bodies, versions, history or blobs", scope, tt.tool)
				}
			})
		}
	}
	checkPages(t, tm.pool)
	return seen
}

// sourceMCPCall sends a real call through the requested entry. Account calls
// name the notebook in their arguments; notebook calls only in the endpoint.
func sourceMCPCall(t *testing.T, m *mcpApp, scope, tool string, args map[string]any) mcpAnswer {
	t.Helper()
	path := atNotebook(m.notebook)
	args = maps.Clone(args)
	if scope == "account" {
		path, args["notebook"] = mcpserver.Pattern, m.notebook
	}
	return m.call(t, path, tool, args)
}

// Full persistent rows catch a refused operation that mutates an existing
// node/body without adding a version, as well as ordinary extra history rows.
// Search-index rows are outside this transaction's observable write contract.
func sourcePersistentRows(t *testing.T, tm acmeTeam, notebook string) []string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT kind || ':' || doc::text FROM (
		SELECT 'node' AS kind, n.id::text AS id, to_jsonb(n) AS doc FROM nodes n WHERE n.notebook_id = $1
		UNION ALL SELECT 'content', p.node_id::text, to_jsonb(p) FROM page_contents p JOIN nodes n ON n.id = p.node_id WHERE n.notebook_id = $1
		UNION ALL SELECT 'revision', p.id::text, to_jsonb(p) FROM page_revisions p JOIN nodes n ON n.id = p.node_id WHERE n.notebook_id = $1
		UNION ALL SELECT 'changeset', c.id::text, to_jsonb(c) FROM changesets c WHERE c.notebook_id = $1
		UNION ALL SELECT 'item', i.id::text, to_jsonb(i) FROM changeset_items i JOIN changesets c ON c.id = i.changeset_id WHERE c.notebook_id = $1
		UNION ALL SELECT 'blob', b.id::text, to_jsonb(b) FROM asset_blobs b WHERE b.notebook_id = $1
	) rows ORDER BY kind, id`, notebook)
}

func TestSourceMCPToolsAnswerEveryAddedCode(t *testing.T) {
	seen := sourceCodeCalls(t, answeredCodes{})
	// Independent of the call table and mcpToolCodes: deleting a code declaration,
	// a scope's real call, or both must not turn a missing behavior into success.
	for _, tuple := range []string{
		"notebook/write/page.source_immutable", "account/write/page.source_immutable",
		"notebook/edit/page.source_immutable", "account/edit/page.source_immutable",
		"notebook/batch/page.source_immutable", "account/batch/page.source_immutable",
		"notebook/delete/page.source_admin_only", "account/delete/page.source_admin_only",
		"notebook/move/page.source_too_large", "account/move/page.source_too_large",
		"notebook/batch/page.source_too_large", "account/batch/page.source_too_large",
		"notebook/write/page.source_duplicate", "account/write/page.source_duplicate",
		"notebook/move/page.source_duplicate", "account/move/page.source_duplicate",
		"notebook/batch/page.source_duplicate", "account/batch/page.source_duplicate",
	} {
		if !seen[tuple] {
			t.Errorf("no actual source-policy answer for %s", tuple)
		}
	}
}

// Both entries can create an empty source and supply its first body with a
// legal write. The create carries the final body once, at revision 1; a
// neighboring same-byte write likewise leaves only one persistent version.
func TestSourceMCPBatchFirstBodyAndSameBytesWriteOnce(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Source batches")
	writer := tm.agent(t, "bob", wiki.id)
	for _, scope := range []string{"notebook", "account"} {
		for _, tt := range []struct {
			name, first, second string
			secondChanged       bool
		}{
			{"empty first body", "", scope + " B", true},
			{"same bytes", scope + " A", scope + " A", false},
		} {
			t.Run(scope+"/"+tt.name, func(t *testing.T) {
				path := "raw/" + scope + " " + tt.name + ".md"
				before := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", wiki.id)
				out := answerOf[mcpBatched](t, sourceMCPCall(t, writer, scope, "batch", batched(opWrite(path, tt.first), opWrite(path, tt.second))))
				if !out.Changed || out.ChangesetID == nil || len(out.Results) != 2 || out.ResultsCount != 2 {
					t.Fatalf("batch = %+v, want two results in one changeset", out)
				}
				id := out.Results[0].NodeID
				if id == "" || out.Results[1].NodeID != id || out.Results[1].Changed != tt.secondChanged {
					t.Fatalf("results = %+v, want same node and second changed=%v", out.Results, tt.secondChanged)
				}
				for _, result := range out.Results {
					if result.Version == nil || *result.Version != id+":1" {
						t.Errorf("result version = %v, want %s:1", result.Version, id)
					}
				}
				if got := queryStrings(t, tm.pool, "SELECT content || ':' || revision::text FROM page_contents WHERE node_id = $1", id); !slices.Equal(got, []string{tt.second + ":1"}) {
					t.Errorf("persisted body = %q, want %q at revision 1", got, tt.second)
				}
				if versions := count(t, tm.pool, "SELECT count(*) FROM page_revisions WHERE node_id = $1", id); versions != 1 {
					t.Errorf("versions = %d, want 1", versions)
				}
				if after := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1", wiki.id); after != before+1 {
					t.Errorf("changesets before=%d after=%d, want exactly one added", before, after)
				}
			})
		}
	}
	checkPages(t, tm.pool)
}
