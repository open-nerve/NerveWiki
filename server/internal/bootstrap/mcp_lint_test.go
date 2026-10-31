package bootstrap

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"
)

// Strip only MCP's additional path members for the independent REST comparison.
// Leave identities, cursors, arrays/null and all discriminated payload members.
func mcpLintWithoutPaths(value map[string]any) map[string]any {
	out := make(map[string]any, len(value))
	for key, value := range value {
		if key == "path" {
			continue
		}
		switch v := value.(type) {
		case map[string]any:
			out[key] = mcpLintWithoutPaths(v)
		case []any:
			array := make([]any, len(v))
			for i, child := range v {
				if item, ok := child.(map[string]any); ok {
					array[i] = mcpLintWithoutPaths(item)
				} else {
					array[i] = child
				}
			}
			out[key] = array
		default:
			out[key] = v
		}
	}
	return out
}

func mcpLintCompareREST(t *testing.T, mcp, rest map[string]any) {
	t.Helper()
	got, want := mcpLintWithoutPaths(mcp), maps.Clone(rest)
	delete(got, "computed_at")
	delete(want, "computed_at")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("actual MCP differs from REST after path/time removal:\nMCP=%v\nREST=%v", got, want)
	}
}

func mcpLintHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

type mcpLintSeed struct {
	nb, mode string
	expected map[string][]map[string]any
}

// All fixture content goes through the real REST production writer/parser.
// The expected branches below are literal facts, not reconstructed rule logic.
func seedMCPHealth(t *testing.T, f *lintRuntimeFixture, mode string) mcpLintSeed {
	t.Helper()
	nb := f.tm.createWith(t, f.tm.tokens["alice"], `{"name":"MCP health","mode":"`+mode+`","workspace_access":"none"}`)
	f.tm.addNotebookMember(t, "alice", nb.ID, "bob", "reader")
	var raw string
	if mode == "llm_wiki" {
		raw = *nb.SourceRootID
	} else {
		raw = f.tm.createPage(t, "alice", nb.ID, "", "raw")
	}
	f.tm.createPageWith(t, "alice", nb.ID, raw, "Source", "source body\n")
	asset := f.tm.upload(t, "alice", nb.ID, raw, "pending.txt", "source bytes")
	var created time.Time
	if err := f.tm.pool.QueryRow(t.Context(), "SELECT created_at FROM nodes WHERE id=$1", asset.ID).Scan(&created); err != nil {
		t.Fatal(err)
	}
	a := f.tm.createPageWith(t, "alice", nb.ID, "", "Alias A", "---\ntype: note\nsummary: ready\naliases: [shared]\nsources: ['[[raw/Source]]']\n---\nbody\n")
	b := f.tm.createPageWith(t, "alice", nb.ID, "", "Alias B", "---\ntype: note\nsummary: ready\naliases: [shared]\nsources: ['[[raw/Source]]']\n---\nbody\n")
	emitter := f.tm.createPageWith(t, "alice", nb.ID, "", "Emitter", "---\ntype: note\nsummary: ready\nsources: [literal, '[[not-a-source]]']\n---\n[[ghost]] [[shared]]\n")
	plain := f.tm.createPageWith(t, "alice", nb.ID, "", "Plain", "plain body\n")
	broken := f.tm.createPageWith(t, "alice", nb.ID, "", "Broken", "---\ntype: [\n---\nbody\n")
	page := func(rule, id, path string) map[string]any {
		return map[string]any{"rule": rule, "item_key": "p:" + id, "page_id": id, "path": path}
	}
	// The equal alias candidates still resolve to the first (least UUID),
	// with Ambiguous=true. Alias A therefore has an actual inbound edge.
	if a >= b {
		t.Fatal("alias fixture creation did not preserve UUID order")
	}
	orphans := []map[string]any{page("orphans", b, "Alias B.md"), page("orphans", emitter, "Emitter.md"), page("orphans", plain, "Plain.md"), page("orphans", broken, "Broken.md")}
	slices.SortFunc(orphans, func(a, b map[string]any) int { return cmp.Compare(a["page_id"].(string), b["page_id"].(string)) })
	expected := map[string][]map[string]any{
		"wanted":              {{"rule": "wanted", "item_key": "w:" + mcpLintHash("ghost"), "target": "ghost", "target_cut": false, "pages": float64(1)}, {"rule": "wanted", "item_key": "w:" + mcpLintHash("not-a-source"), "target": "not-a-source", "target_cut": false, "pages": float64(1)}},
		"orphans":             orphans,
		"ambiguous":           {{"rule": "ambiguous", "item_key": "a:" + emitter + ":" + mcpLintHash("shared"), "page_id": emitter, "path": "Emitter.md", "target": "shared", "target_cut": false}},
		"invalid_frontmatter": {page("invalid_frontmatter", broken, "Broken.md")},
	}
	if mode == "llm_wiki" {
		expected["missing_type"] = []map[string]any{page("missing_type", plain, "Plain.md")}
		expected["missing_summary"] = []map[string]any{page("missing_summary", plain, "Plain.md")}
		expected["undigested_sources"] = []map[string]any{{"rule": "undigested_sources", "item_key": "n:" + asset.ID, "node_id": asset.ID, "path": "raw/pending.txt", "kind": "asset", "bytes": float64(12), "created_at": created.UTC().Format(time.RFC3339Nano)}}
		// Independently precomputed SHA256 of J + uint64be(9) + JSON "literal".
		expected["invalid_sources"] = []map[string]any{{"rule": "invalid_sources", "item_key": "s:" + emitter + ":3a186b591a145f6c6100f3792e8c1aad39703b95432295d93b7a9c48906908b3", "page_id": emitter, "path": "Emitter.md", "target": `"literal"`, "target_cut": false, "not_link": true}}
		expected["invalid_sources"] = append(expected["invalid_sources"], map[string]any{"rule": "invalid_sources", "item_key": "s:" + emitter + ":469a88ed5ab9348f5be1d6d650fce93f071db58eec692dc680d277ccd1bda3cc", "page_id": emitter, "path": "Emitter.md", "target": "not-a-source", "target_cut": false, "not_link": false})
		slices.SortFunc(expected["invalid_sources"], func(a, b map[string]any) int { return cmp.Compare(a["item_key"].(string), b["item_key"].(string)) })
	}
	return mcpLintSeed{nb: nb.ID, mode: mode, expected: expected}
}

func mcpLintAssertItems(t *testing.T, actual any, expected []map[string]any) {
	t.Helper()
	items := lintServeObjects(t, actual)
	if len(items) != len(expected) {
		t.Fatalf("real MCP items=%v, want %d independently seeded items %v", items, len(expected), expected)
	}
	seen := map[string]bool{}
	for i, item := range items {
		cursor := lintServeString(t, item, "cursor")
		if seen[cursor] {
			t.Fatalf("real MCP repeated cursor %q", cursor)
		}
		seen[cursor] = true
		got := maps.Clone(item)
		delete(got, "cursor")
		if !reflect.DeepEqual(got, expected[i]) {
			t.Fatalf("real MCP item %d=%v, want independently seeded %v", i, got, expected[i])
		}
	}
}

func TestTheWikiToolsReachServe(t *testing.T) {
	for _, mode := range []string{"standard", "llm_wiki"} {
		t.Run(mode, func(t *testing.T) {
			f := newLintRuntime(t, nil)
			s := seedMCPHealth(t, f, mode)
			reader := f.tm.agent(t, "bob", s.nb)
			for _, difference := range toolListDiff(reader.listedTools(t), readToolLists(t)) {
				t.Error(difference)
			}
			for _, entry := range []string{"notebook", "account"} {
				t.Run(entry, func(t *testing.T) {
					summary := answerOf[map[string]any](t, sourceMCPCall(t, reader, entry, "lint", map[string]any{}))
					lintServeEnvelope(t, mcpLintWithoutPaths(summary), mode)
					lintServeSummary(t, summary, mode)
					for _, rule := range lintServeObjects(t, summary["rules"]) {
						name := lintServeString(t, rule, "rule")
						t.Run(name, func(t *testing.T) {
							want := s.expected[name]
							if len(want) == 0 {
								t.Fatalf("missing independent nonempty oracle for %s", name)
							}
							if rule["count"] != float64(len(want)) || rule["count_capped"] != false || rule["next_cursor"] != nil {
								t.Fatalf("actual summary %s=%v, want exact independent count %d", name, rule, len(want))
							}
							mcpLintAssertItems(t, rule["items"], want)
							page := answerOf[map[string]any](t, sourceMCPCall(t, reader, entry, "lint", map[string]any{"rule": name}))
							lintServeEnvelope(t, mcpLintWithoutPaths(page), mode)
							mcpLintAssertItems(t, page["data"], want)
							if page["next_cursor"] != nil {
								t.Fatalf("real small rule page did not terminate: %v", page)
							}
							rest := lintServeGet(t, f.tm, reader.pat, "/api/v0/notebooks/"+s.nb+"/lint/"+name, http.StatusOK, "")
							mcpLintCompareREST(t, page, rest)
						})
					}
					rest := lintServeGet(t, f.tm, reader.pat, "/api/v0/notebooks/"+s.nb+"/lint", http.StatusOK, "")
					mcpLintCompareREST(t, summary, rest)
					if mode == "llm_wiki" {
						pending := answerOf[map[string]any](t, sourceMCPCall(t, reader, entry, "pending_sources", map[string]any{"limit": nil, "cursor": nil}))
						mcpLintAssertItems(t, pending["data"], s.expected["undigested_sources"])
						rest := lintServeGet(t, f.tm, reader.pat, "/api/v0/notebooks/"+s.nb+"/lint/undigested_sources", http.StatusOK, "")
						mcpLintCompareREST(t, pending, rest)
					} else if got := sourceMCPCall(t, reader, entry, "pending_sources", map[string]any{}).toolError(t)["code"]; got != "notebook.not_llm_wiki" {
						t.Fatalf("standard pending=%v", got)
					}
				})
			}
			f.invariants(t)
		})
	}
}
