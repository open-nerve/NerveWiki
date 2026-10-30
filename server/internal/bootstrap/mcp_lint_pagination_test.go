package bootstrap

import (
	"cmp"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

// Start with real persisted facts, then move/rename their nodes through REST
// into legal depth-10, 255-byte-title paths. Ordinary ancestors have complete
// metadata and are included in the independent orphan facts. No budgets, cursor
// fields or projection values are forged here.
func seedMCPLongHealth(t *testing.T, f *lintRuntimeFixture) mcpLintSeed {
	t.Helper()
	s := seedMCPHealth(t, f, "llm_wiki")
	title := strings.Repeat("&", 255)
	parent, prefix := "", ""
	var ancestors []map[string]any
	for range 9 {
		parent = f.tm.createPageWith(t, "alice", s.nb, parent, title, "---\ntype: note\nsummary: folder\n---\n")
		prefix += title + "/"
		ancestors = append(ancestors, map[string]any{"rule": "orphans", "item_key": "p:" + parent, "page_id": parent, "path": strings.TrimSuffix(prefix, "/") + ".md"})
	}
	ids := map[string]string{}
	for _, items := range s.expected {
		for _, item := range items {
			if id, ok := item["page_id"].(string); ok {
				ids[id] = item["path"].(string)
			}
		}
	}
	// Alias A has an inbound edge but must have the same long path length as
	// Alias B, to preserve the independently known alias tie/least-ID winner.
	var aliasA string
	if err := f.tm.pool.QueryRow(t.Context(), "SELECT id::text FROM nodes WHERE notebook_id=$1 AND name='Alias A'", s.nb).Scan(&aliasA); err != nil {
		t.Fatal(err)
	}
	ids[aliasA] = "Alias A.md"
	names := map[string]string{"Alias A.md": "a", "Alias B.md": "b", "Emitter.md": "e", "Plain.md": "p", "Broken.md": "x"}
	for id, old := range ids {
		name := strings.Repeat("&", 254) + names[old]
		f.tm.send(t, nodeMove("alice", id, parent), http.StatusOK)
		f.tm.send(t, nodeRename("alice", id, name), http.StatusOK)
		for _, items := range s.expected {
			for _, item := range items {
				if item["page_id"] == id {
					item["path"] = prefix + name + ".md"
				}
			}
		}
	}
	s.expected["orphans"] = append(s.expected["orphans"], ancestors...)
	slices.SortFunc(s.expected["orphans"], func(a, b map[string]any) int { return cmp.Compare(a["page_id"].(string), b["page_id"].(string)) })
	// Moves deliberately rewrite existing links. Restore the literal ambiguous
	// link only after all equal-length candidates reached their final locations.
	emitter := s.expected["ambiguous"][0]["page_id"].(string)
	var revision int
	if err := f.tm.pool.QueryRow(t.Context(), "SELECT revision FROM page_contents WHERE node_id=$1", emitter).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	f.tm.send(t, contentWrite("alice", emitter, "---\ntype: note\nsummary: ready\nsources: [literal, '[[not-a-source]]']\n---\n[[ghost]] [[shared]]\n", revision, ""), http.StatusOK)
	var raw string
	if err := f.tm.pool.QueryRow(t.Context(), "SELECT source_root_id::text FROM notebooks WHERE id=$1", s.nb).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	parent, prefix = raw, "raw/"
	for range 9 {
		parent = f.tm.createPage(t, "alice", s.nb, parent, title)
		prefix += title + "/"
	}
	pending := s.expected["undigested_sources"][0]
	name := strings.Repeat("&", 251) + ".bin"
	id := pending["node_id"].(string)
	f.tm.send(t, nodeMove("alice", id, parent), http.StatusOK)
	f.tm.send(t, nodeRename("alice", id, name), http.StatusOK)
	pending["path"] = prefix + name
	// Keep the saved creation oracle and require a distinct real update time.
	var created, updated time.Time
	if err := f.tm.pool.QueryRow(t.Context(), "SELECT created_at, updated_at FROM nodes WHERE id=$1", id).Scan(&created, &updated); err != nil {
		t.Fatal(err)
	}
	if got := created.UTC().Format(time.RFC3339Nano); got != pending["created_at"] {
		t.Fatalf("moved asset created_at=%s, want original %v", got, pending["created_at"])
	}
	if updated.Equal(created) {
		t.Fatalf("moved asset needs distinct timestamps: created_at=%s updated_at=%s", created.UTC().Format(time.RFC3339Nano), updated.UTC().Format(time.RFC3339Nano))
	}
	t.Logf("real moved asset timestamps: created_at=%s updated_at=%s", created.UTC().Format(time.RFC3339Nano), updated.UTC().Format(time.RFC3339Nano))
	// Assert exact legal persisted path lengths, not an arbitrary giant string.
	if len(s.expected["missing_type"][0]["path"].(string)) != 2562 || len(pending["path"].(string)) != 2563 {
		t.Fatalf("long real paths have unexpected sizes: page=%d asset=%d", len(s.expected["missing_type"][0]["path"].(string)), len(pending["path"].(string)))
	}
	return s
}

func mcpLintRESTItems(expected []map[string]any) []map[string]any {
	items := make([]map[string]any, len(expected))
	for i, item := range expected {
		items[i] = maps.Clone(item)
		delete(items[i], "path")
	}
	return items
}

func mcpLintSDKBytes(t *testing.T, a mcpAnswer) int {
	t.Helper()
	// Measure each actual SDK-written raw text JSON string before unescaping.
	var envelope struct {
		Result struct {
			Content []struct {
				Type string          `json:"type"`
				Text json.RawMessage `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(a.body, &envelope); err != nil {
		t.Fatal(err)
	}
	total, blocks := 0, 0
	for _, item := range envelope.Result.Content {
		if item.Type != "text" {
			continue
		}
		if len(item.Text) < 2 || item.Text[0] != '"' || item.Text[len(item.Text)-1] != '"' {
			t.Fatal("SDK text block is not a raw JSON string")
		}
		blocks++
		total += len(item.Text) - 2
	}
	if blocks == 0 || total == 0 || total > 49152 {
		t.Fatalf("actual SDK health text=%d bytes/%d blocks, want nonempty <=49152", total, blocks)
	}
	return total
}

func TestTheWikiToolsResumeAcrossProtocols(t *testing.T) {
	f := newLintRuntime(t, nil)
	s := seedMCPLongHealth(t, f)
	reader := f.tm.agent(t, "bob", s.nb)
	base := "/api/v0/notebooks/" + s.nb + "/lint"
	for _, entry := range []string{"notebook", "account"} {
		t.Run(entry, func(t *testing.T) {
			a := sourceMCPCall(t, reader, entry, "lint", map[string]any{})
			summary := answerOf[map[string]any](t, a)
			t.Logf("actual %s Fit0 summary SDK bytes=%d", entry, mcpLintSDKBytes(t, a))
			lintServeSummary(t, summary, "llm_wiki")
			for _, rule := range lintServeObjects(t, summary["rules"]) {
				name := lintServeString(t, rule, "rule")
				t.Run("Begin/"+name, func(t *testing.T) {
					expected := s.expected[name]
					if len(expected) == 0 || rule["count"] != float64(len(expected)) || rule["count_capped"] != false || len(lintServeObjects(t, rule["items"])) != 0 {
						t.Fatalf("real summary did not reach common Fit0 with independent count: %v", rule)
					}
					begin := lintServeString(t, rule, "next_cursor")
					rest := lintServeGet(t, f.tm, reader.pat, base+"/"+name+"?limit=1&cursor="+url.QueryEscape(begin), http.StatusOK, "")
					mcpLintAssertItems(t, rest["data"], mcpLintRESTItems(expected[:1]))
					for _, receiver := range []string{"notebook", "account"} {
						got := answerOf[map[string]any](t, sourceMCPCall(t, reader, receiver, "lint", map[string]any{"rule": name, "limit": 1, "cursor": begin}))
						mcpLintAssertItems(t, got["data"], expected[:1])
						mcpLintCompareREST(t, got, rest)
					}
					if name == "undigested_sources" {
						got := answerOf[map[string]any](t, sourceMCPCall(t, reader, entry, "pending_sources", map[string]any{"limit": 1, "cursor": begin}))
						mcpLintAssertItems(t, got["data"], expected[:1])
						mcpLintCompareREST(t, got, rest)
					}
				})
			}
			t.Run("MCP-middle-to-REST", func(t *testing.T) {
				a := sourceMCPCall(t, reader, entry, "lint", map[string]any{"rule": "orphans", "limit": 100})
				first := answerOf[map[string]any](t, a)
				items := lintServeObjects(t, first["data"])
				all := s.expected["orphans"]
				if len(items) < 1 || len(items) >= len(all) {
					t.Fatalf("real Fit must cut a nonfinal positive prefix; got %d of %d", len(items), len(all))
				}
				mcpLintAssertItems(t, first["data"], all[:len(items)])
				cursor := lintServeString(t, items[len(items)-1], "cursor")
				if first["next_cursor"] != cursor {
					t.Fatalf("Fit continuation=%v, want last emitted cursor", first["next_cursor"])
				}
				t.Logf("actual %s midpoint=%d/%d SDKbytes=%d", entry, len(items), len(all), mcpLintSDKBytes(t, a))
				rest := lintServeGet(t, f.tm, reader.pat, base+"/orphans?limit=100&cursor="+url.QueryEscape(cursor), http.StatusOK, "")
				mcpLintAssertItems(t, rest["data"], mcpLintRESTItems(all[len(items):]))
				if rest["next_cursor"] != nil {
					t.Fatalf("REST did not terminate the actual MCP tail: %v", rest)
				}
			})
			t.Run("REST-middle-to-MCP", func(t *testing.T) {
				first := lintServeGet(t, f.tm, reader.pat, base+"/orphans?limit=1", http.StatusOK, "")
				all := s.expected["orphans"]
				mcpLintAssertItems(t, first["data"], mcpLintRESTItems(all[:1]))
				cursor := lintServeString(t, first, "next_cursor")
				offset := 1
				seen := map[string]bool{}
				for cursor != "" {
					if seen[cursor] {
						t.Fatalf("cross-protocol continuation loop at %q", cursor)
					}
					seen[cursor] = true
					a := sourceMCPCall(t, reader, entry, "lint", map[string]any{"rule": "orphans", "limit": 100, "cursor": cursor})
					got := answerOf[map[string]any](t, a)
					items := lintServeObjects(t, got["data"])
					if len(items) == 0 || offset+len(items) > len(all) {
						t.Fatalf("real continuation did not advance: offset=%d result=%v", offset, got)
					}
					mcpLintAssertItems(t, got["data"], all[offset:offset+len(items)])
					mcpLintSDKBytes(t, a)
					offset += len(items)
					cursor = ""
					if got["next_cursor"] != nil {
						cursor = lintServeString(t, got, "next_cursor")
					}
				}
				if offset != len(all) {
					t.Fatalf("REST/MCP lost items: %d of %d", offset, len(all))
				}
			})
		})
	}
	f.invariants(t)
}
