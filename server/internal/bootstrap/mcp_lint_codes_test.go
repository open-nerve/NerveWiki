package bootstrap

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

type mcpLintHTTPResult struct {
	answer mcpAnswer
	err    error
}

// Construct requests on the test goroutine; the asynchronous transport returns
// errors as data. The real MCP middleware, Gate, SDK and tool still run.
func startMCPLintCall(t *testing.T, m *mcpApp, ctx context.Context, entry, tool string, args map[string]any) <-chan mcpLintHTTPResult {
	t.Helper()
	path := atNotebook(m.notebook)
	args = maps.Clone(args)
	if entry == "account" {
		path = mcpserver.Pattern
		args["notebook"] = m.notebook
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.base+path, strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+m.pat)
	req.Header.Set("Mcp-Protocol-Version", mcpVersion)
	done := make(chan mcpLintHTTPResult, 1)
	go func() {
		start := time.Now()
		response, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			done <- mcpLintHTTPResult{err: err}
			return
		}
		defer func() { _ = response.Body.Close() }()
		body, err := io.ReadAll(response.Body)
		done <- mcpLintHTTPResult{answer: mcpAnswer{status: response.StatusCode, header: response.Header, body: body, took: time.Since(start), tool: tool}, err: err}
	}()
	return done
}

func receiveMCPLint(t *testing.T, done <-chan mcpLintHTTPResult) mcpLintHTTPResult {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(interleavingWait):
		t.Fatal("real MCP caller did not return within the existing interleaving wait")
		return mcpLintHTTPResult{}
	}
}

func mcpLintError(t *testing.T, a mcpAnswer, status int, code string) {
	t.Helper()
	if status != http.StatusOK {
		if got := a.problem(t, status); got != code {
			t.Fatalf("actual entry problem=%s, want %s", got, code)
		}
		return
	}
	member := a.toolError(t)
	message, _ := member["message"].(string)
	if member["code"] != code || !strings.HasPrefix(message, code+": ") {
		t.Fatalf("actual %s tool error=%v, want %s", a.tool, member, code)
	}
	result := a.result(t)["structuredContent"].(map[string]any)
	if len(result) != 1 {
		t.Fatalf("failed real tool returned partial health data: %v", result)
	}
	if code == "server_busy" && member["retry_after_seconds"] != float64(1) {
		t.Fatalf("actual busy retry=%v, want 1", member)
	}
}

func TestTheWikiToolsJudgeTheirEntryBeforeArguments(t *testing.T) {
	f := newLintRuntime(t, nil)
	visible := f.tm.createWith(t, f.tm.tokens["alice"], `{"name":"Visible MCP","workspace_access":"viewer"}`)
	hidden := f.tm.createWith(t, f.tm.tokens["alice"], `{"name":"Private MCP","workspace_access":"none"}`)
	f.tm.createPageWith(t, "alice", visible.ID, "", "Emitter", "[[foreign-cursor-item]]\n")
	page := lintServeGet(t, f.tm, f.tm.tokens["bob"], "/api/v0/notebooks/"+visible.ID+"/lint/wanted", http.StatusOK, "")
	items := lintServeObjects(t, page["data"])
	if len(items) != 1 {
		t.Fatalf("foreign cursor fixture=%v, want one real wanted item", page)
	}
	foreign := lintServeString(t, items[0], "cursor")
	reader := f.tm.agent(t, "bob", visible.ID)
	for _, entry := range []string{"notebook", "account"} {
		t.Run(entry, func(t *testing.T) {
			for _, tc := range []struct {
				name, notebook, tool, code string
				args                       map[string]any
				hidden                     bool
			}{
				{"hidden-invalid-cursor-rule-limit", hidden.ID, "lint", "bad_request", map[string]any{"rule": "unknown", "limit": 0, "cursor": "bad"}, true},
				{"hidden-fixed-cursor", hidden.ID, "lint", "notebook.not_found", map[string]any{"rule": "unknown", "limit": 0}, true},
				{"hidden-known-foreign", hidden.ID, "lint", "bad_request", map[string]any{"rule": "wanted", "cursor": foreign, "limit": 0}, true},
				{"hidden-unknown-valid-foreign", hidden.ID, "lint", "notebook.not_found", map[string]any{"rule": "unknown", "cursor": foreign, "limit": 0}, true},
				{"visible-invalid-cursor-rule-limit", visible.ID, "lint", "bad_request", map[string]any{"rule": "unknown", "cursor": "bad", "limit": 0}, false},
				{"visible-fixed-cursor", visible.ID, "lint", "wiki.lint_rule_not_found", map[string]any{"rule": "unknown", "limit": 0}, false},
				{"visible-unknown-valid-cursor", visible.ID, "lint", "wiki.lint_rule_not_found", map[string]any{"rule": "unknown", "cursor": foreign, "limit": 0}, false},
				{"visible-fixed-rule-exclusive-mode", visible.ID, "lint", "notebook.not_llm_wiki", map[string]any{"rule": "missing_type", "limit": 0}, false},
				{"visible-common-rule-limit", visible.ID, "lint", "validation_failed", map[string]any{"rule": "wanted", "limit": 0}, false},
				{"summary-invalid-cursor", visible.ID, "lint", "bad_request", map[string]any{"cursor": "bad", "limit": 0}, false},
				{"summary-limit", visible.ID, "lint", "validation_failed", map[string]any{"limit": 0}, false},
				{"summary-valid-cursor", visible.ID, "lint", "validation_failed", map[string]any{"cursor": foreign}, false},
				{"empty-rule-is-unknown", visible.ID, "lint", "wiki.lint_rule_not_found", map[string]any{"rule": ""}, false},
				{"pending-hidden-invalid-cursor", hidden.ID, "pending_sources", "bad_request", map[string]any{"cursor": "bad", "limit": 0}, true},
				{"pending-hidden-fixed-cursor", hidden.ID, "pending_sources", "notebook.not_found", map[string]any{"limit": 0}, true},
				{"pending-visible-invalid-cursor", visible.ID, "pending_sources", "bad_request", map[string]any{"cursor": "bad", "limit": 0}, false},
				{"pending-mode-before-limit", visible.ID, "pending_sources", "notebook.not_llm_wiki", map[string]any{"limit": 0}, false},
				{"pending-extra-rule", visible.ID, "pending_sources", "bad_request", map[string]any{"rule": "wanted"}, false},
				{"lint-no-explicit-contract", visible.ID, "lint", "bad_request", map[string]any{"contract": "llm_wiki"}, false},
				{"lint-no-proposed-root", visible.ID, "lint", "bad_request", map[string]any{"area_root_id": hidden.ID}, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					m := *reader
					m.notebook = tc.notebook
					wantStatus, wantCode := http.StatusOK, tc.code
					if entry == "notebook" && tc.hidden {
						wantStatus, wantCode = http.StatusNotFound, "notebook.not_found"
					}
					mcpLintError(t, sourceMCPCall(t, &m, entry, tc.tool, tc.args), wantStatus, wantCode)
				})
			}
			for _, tool := range []string{"lint", "pending_sources"} {
				t.Run("binding/"+tool, func(t *testing.T) {
					for _, args := range []map[string]any{{"limit": "two"}, {"cursor": false}} {
						mcpLintError(t, sourceMCPCall(t, reader, entry, tool, args), http.StatusOK, "bad_request")
					}
					if entry == "account" {
						for _, args := range []map[string]any{{}, {"notebook": nil}, {"notebook": 17}} {
							mcpLintError(t, reader.call(t, mcpserver.Pattern, tool, args), http.StatusOK, "bad_request")
						}
						mcpLintError(t, reader.call(t, mcpserver.Pattern, tool, map[string]any{"notebook": "not-a-uuid", "cursor": "bad"}), http.StatusOK, "bad_request")
						mcpLintError(t, reader.call(t, mcpserver.Pattern, tool, map[string]any{"notebook": "not-a-uuid"}), http.StatusOK, "notebook.not_found")
					} else {
						mcpLintError(t, reader.call(t, atNotebook(visible.ID), tool, map[string]any{"notebook": visible.ID}), http.StatusOK, "bad_request")
					}
				})
			}
			t.Run("nullable-scalars", func(t *testing.T) {
				got := answerOf[map[string]any](t, sourceMCPCall(t, reader, entry, "lint", map[string]any{"rule": nil, "limit": nil, "cursor": nil}))
				lintServeWanted(t, lintServeSummary(t, got, "standard"), "foreign-cursor-item")
			})
		})
	}
}

// Each row here is an actual call. The independent tuple oracle below and the
// original all-tools aggregation both consume it, so dropping an entry or code
// cannot be hidden by the other entry answering that code.
func wikiToolCodeCalls(t *testing.T, answered answeredCodes) map[string]bool {
	t.Helper()
	seen := map[string]bool{}
	record := func(t *testing.T, entry, tool, code string, a mcpAnswer) {
		t.Helper()
		mcpLintError(t, a, http.StatusOK, code)
		seen[entry+"/"+tool+"/"+code] = true
		answered.add(tool, code)
	}
	t.Run("wiki-codes", func(t *testing.T) {
		f := newLintRuntime(t, nil)
		standard := f.tm.createWith(t, f.tm.tokens["alice"], `{"name":"Standard codes","workspace_access":"viewer"}`)
		nb, _, _ := f.seedPending(t)
		reader := f.tm.agent(t, "bob", nb.id)
		for _, entry := range []string{"notebook", "account"} {
			for _, tc := range []struct {
				tool, code, notebook string
				args                 map[string]any
			}{
				{"lint", "bad_request", nb.id, map[string]any{"rule": "wanted", "cursor": "bad"}},
				{"lint", "validation_failed", nb.id, map[string]any{"rule": "wanted", "limit": 0}},
				{"lint", "wiki.lint_rule_not_found", nb.id, map[string]any{"rule": "unknown"}},
				{"lint", "notebook.not_llm_wiki", standard.ID, map[string]any{"rule": "missing_type"}},
				{"pending_sources", "bad_request", nb.id, map[string]any{"cursor": "bad"}},
				{"pending_sources", "validation_failed", nb.id, map[string]any{"limit": 0}},
				{"pending_sources", "notebook.not_llm_wiki", standard.ID, map[string]any{}},
			} {
				t.Run(entry+"/"+tc.tool+"/"+tc.code, func(t *testing.T) {
					m := *reader
					m.notebook = tc.notebook
					record(t, entry, tc.tool, tc.code, sourceMCPCall(t, &m, entry, tc.tool, tc.args))
				})
			}
		}
	})
	for _, entry := range []string{"notebook", "account"} {
		for _, tool := range []string{"lint", "pending_sources"} {
			t.Run("wiki-codes/"+entry+"/"+tool+"/notebook.not_found-after-gate", func(t *testing.T) {
				f := newLintRuntime(t, nil)
				nb, _, _ := f.seedPending(t)
				f.tm.send(t, request("alice", http.MethodPatch, "/api/v0/notebooks/"+nb.id, `{"workspace_access":"none"}`), http.StatusOK)
				membership := f.tm.addNotebookMember(t, "alice", nb.id, "bob", "reader")
				reader := f.tm.agent(t, "bob", nb.id)
				release := f.holdAssets(t)
				args := map[string]any{}
				if tool == "lint" {
					args["rule"] = "undigested_sources"
				}
				done := startMCPLintCall(t, reader, t.Context(), entry, tool, args)
				// This same real request has passed Gate, authorization and started its
				// snapshot before permission is revoked through the actual REST API.
				pid := f.waitAssetLock(t)
				f.tm.send(t, request("alice", http.MethodDelete, "/api/v0/notebook-members/"+membership, ""), http.StatusNoContent)
				release()
				got := receiveMCPLint(t, done)
				if got.err != nil {
					t.Fatal(got.err)
				}
				record(t, entry, tool, "notebook.not_found", got.answer)
				f.assertSnapshot(t, pid)
			})
			t.Run("wiki-codes/"+entry+"/"+tool+"/server_busy", func(t *testing.T) {
				f := newLintRuntime(t, nil)
				nb, _, _ := f.seedPending(t)
				reader := f.tm.agent(t, "bob", nb.id)
				release := f.holdAssets(t)
				args := map[string]any{}
				if tool == "lint" {
					args["rule"] = "undigested_sources"
				}
				done := startMCPLintCall(t, reader, t.Context(), entry, tool, args)
				f.waitAssetLock(t)
				got := receiveMCPLint(t, done)
				if got.err != nil {
					t.Fatal(got.err)
				}
				record(t, entry, tool, "server_busy", got.answer)
				release()
			})
		}
	}
	return seen
}

func TestTheWikiToolsAnswerEveryCode(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: needs Docker")
	}
	seen := wikiToolCodeCalls(t, answeredCodes{})
	for _, tuple := range []string{
		"notebook/lint/bad_request", "account/lint/bad_request",
		"notebook/lint/validation_failed", "account/lint/validation_failed",
		"notebook/lint/notebook.not_found", "account/lint/notebook.not_found",
		"notebook/lint/wiki.lint_rule_not_found", "account/lint/wiki.lint_rule_not_found",
		"notebook/lint/notebook.not_llm_wiki", "account/lint/notebook.not_llm_wiki",
		"notebook/lint/server_busy", "account/lint/server_busy",
		"notebook/pending_sources/bad_request", "account/pending_sources/bad_request",
		"notebook/pending_sources/validation_failed", "account/pending_sources/validation_failed",
		"notebook/pending_sources/notebook.not_found", "account/pending_sources/notebook.not_found",
		"notebook/pending_sources/notebook.not_llm_wiki", "account/pending_sources/notebook.not_llm_wiki",
		"notebook/pending_sources/server_busy", "account/pending_sources/server_busy",
	} {
		if !seen[tuple] {
			t.Errorf("no actual wiki tool error for %s", tuple)
		}
	}
}
