package bootstrap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

func startLintREST(t *testing.T, f *lintRuntimeFixture, token string, ctx context.Context, path string) <-chan mcpLintHTTPResult {
	t.Helper()
	req := newRequest(t, http.MethodGet, f.tm.base+path, token, nil).WithContext(ctx)
	done := make(chan mcpLintHTTPResult, 1)
	go func() {
		started := time.Now()
		res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			done <- mcpLintHTTPResult{err: err}
			return
		}
		defer func() { _ = res.Body.Close() }()
		data, err := io.ReadAll(res.Body)
		done <- mcpLintHTTPResult{answer: mcpAnswer{status: res.StatusCode, header: res.Header, body: data, took: time.Since(started)}, err: err}
	}()
	return done
}

func lintProtocolSummary(t *testing.T, name string, got mcpLintHTTPResult) map[string]any {
	t.Helper()
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.answer.status != http.StatusOK {
		t.Fatalf("real %s status=%d %.500s", name, got.answer.status, got.answer.body)
	}
	if name == "rest" {
		var summary map[string]any
		decodeAnswer(t, string(got.answer.body), &summary)
		return summary
	}
	return answerOf[map[string]any](t, got.answer)
}

func lintRESTAndMCPFollowers(t *testing.T) {
	for _, cancelled := range []string{"rest", "notebook", "account"} {
		t.Run("cancel-"+cancelled, func(t *testing.T) {
			f := newLintRuntime(t, nil)
			nb, _, asset := f.seedPending(t)
			f.tm.send(t, request("alice", http.MethodPatch, "/api/v0/notebooks/"+nb.id, `{"workspace_access":"viewer"}`), http.StatusOK)
			bob, carol := f.tm.agent(t, "bob", nb.id), f.tm.agent(t, "carol", nb.id)
			restCtx, restCancel := context.WithCancel(t.Context())
			defer restCancel()
			notebookCtx, notebookCancel := context.WithCancel(t.Context())
			defer notebookCancel()
			accountCtx, accountCancel := context.WithCancel(t.Context())
			defer accountCancel()
			release := f.holdAssets(t)
			rest := startLintREST(t, f, bob.pat, restCtx, "/api/v0/notebooks/"+nb.id+"/lint")
			pid := f.waitAssetLock(t)
			created := f.event(t, wiki.RunCreated, 0)
			notebook := startMCPLintCall(t, carol, notebookCtx, "notebook", "lint", map[string]any{})
			f.event(t, wiki.RunJoined, created.FlightID)
			account := startMCPLintCall(t, bob, accountCtx, "account", "lint", map[string]any{})
			f.event(t, wiki.RunJoined, created.FlightID)
			calls := map[string]<-chan mcpLintHTTPResult{"rest": rest, "notebook": notebook, "account": account}
			cancels := map[string]context.CancelFunc{"rest": restCancel, "notebook": notebookCancel, "account": accountCancel}
			cancels[cancelled]()
			if got := receiveMCPLint(t, calls[cancelled]); !errors.Is(got.err, context.Canceled) {
				t.Fatalf("actual %s cancellation=%v/status%d, want only its transport cancellation", cancelled, got.err, got.answer.status)
			}
			release()
			computed := ""
			for _, name := range []string{"rest", "notebook", "account"} {
				if name == cancelled {
					continue
				}
				got := lintProtocolSummary(t, name, receiveMCPLint(t, calls[name]))
				lintServeEnvelope(t, mcpLintWithoutPaths(got), "llm_wiki")
				stamp := lintServeString(t, got, "computed_at")
				if computed != "" && stamp != computed {
					t.Fatalf("same actual flight computed two snapshots: %s/%s", computed, stamp)
				}
				computed = stamp
				pending := lintServeRule(t, got, "undigested_sources")
				items := lintServeObjects(t, pending["items"])
				if pending["count"] != float64(1) || len(items) != 1 || items[0]["node_id"] != asset || items[0]["bytes"] != float64(12) {
					t.Fatalf("remaining %s reader lost actual pending asset: %v", name, pending)
				}
				if name != "rest" && items[0]["path"] != "raw/pending.txt" {
					t.Fatalf("real %s path=%v", name, items[0])
				}
			}
			f.event(t, wiki.RunFinished, created.FlightID)
			f.assertSnapshot(t, pid)
		})
	}
}

func lintActualFitKeepsREST(t *testing.T) {
	f := newLintRuntime(t, nil)
	s := seedMCPLongHealth(t, f)
	reader := f.tm.agent(t, "bob", s.nb)
	release := f.holdAssets(t)
	rest := startLintREST(t, f, reader.pat, t.Context(), "/api/v0/notebooks/"+s.nb+"/lint")
	pid := f.waitAssetLock(t)
	created := f.event(t, wiki.RunCreated, 0)
	notebook := startMCPLintCall(t, reader, t.Context(), "notebook", "lint", map[string]any{})
	f.event(t, wiki.RunJoined, created.FlightID)
	account := startMCPLintCall(t, reader, t.Context(), "account", "lint", map[string]any{})
	f.event(t, wiki.RunJoined, created.FlightID)
	release()
	actualREST := lintProtocolSummary(t, "rest", receiveMCPLint(t, rest))
	for _, rule := range lintServeObjects(t, actualREST["rules"]) {
		name := lintServeString(t, rule, "rule")
		want := s.expected[name]
		if rule["count"] != float64(len(want)) || rule["count_capped"] != false {
			t.Fatalf("real REST shared result changed under Fit: %v", rule)
		}
		if (rule["next_cursor"] != nil) != (len(want) > 5) {
			t.Fatalf("real REST preview continuation changed: %v", rule)
		}
		mcpLintAssertItems(t, rule["items"], mcpLintRESTItems(want[:min(5, len(want))]))
	}
	for _, done := range []<-chan mcpLintHTTPResult{notebook, account} {
		response := receiveMCPLint(t, done)
		got := lintProtocolSummary(t, "mcp", response)
		mcpLintSDKBytes(t, response.answer)
		if got["computed_at"] != actualREST["computed_at"] {
			t.Fatal("actual REST and MCP did not receive the same flight snapshot")
		}
		for _, rule := range lintServeObjects(t, got["rules"]) {
			name := lintServeString(t, rule, "rule")
			if rule["count"] != float64(len(s.expected[name])) || rule["count_capped"] != false || len(lintServeObjects(t, rule["items"])) != 0 {
				t.Fatalf("actual MCP did not privately Fit0: %v", rule)
			}
			lintServeString(t, rule, "next_cursor")
		}
	}
	f.event(t, wiki.RunFinished, created.FlightID)
	f.assertSnapshot(t, pid)
}

func lintPendingSharesItsRuleFlight(t *testing.T) {
	f := newLintRuntime(t, nil)
	nb, _, asset := f.seedPending(t)
	f.tm.send(t, request("alice", http.MethodPatch, "/api/v0/notebooks/"+nb.id, `{"workspace_access":"viewer"}`), http.StatusOK)
	bob, carol := f.tm.agent(t, "bob", nb.id), f.tm.agent(t, "carol", nb.id)
	release := f.holdAssets(t)
	pending := startMCPLintCall(t, bob, t.Context(), "notebook", "pending_sources", map[string]any{"limit": 50})
	f.waitAssetLock(t)
	created := f.event(t, wiki.RunCreated, 0)
	lint := startMCPLintCall(t, carol, t.Context(), "account", "lint", map[string]any{"rule": "undigested_sources"})
	f.event(t, wiki.RunJoined, created.FlightID)
	rest := startLintREST(t, f, carol.pat, t.Context(), "/api/v0/notebooks/"+nb.id+"/lint/undigested_sources?limit=50")
	f.event(t, wiki.RunJoined, created.FlightID)
	other := startMCPLintCall(t, carol, t.Context(), "account", "lint", map[string]any{"rule": "wanted"})
	distinct := f.nextScopeEvent(t)
	if distinct.Kind != wiki.RunCreated || distinct.FlightID == created.FlightID {
		t.Fatalf("different real MCP rule joined pending flight: %+v", distinct)
	}
	release()
	var actualREST map[string]any
	var actualMCP []map[string]any
	for _, c := range []struct {
		name string
		done <-chan mcpLintHTTPResult
	}{{"mcp", pending}, {"mcp", lint}, {"rest", rest}} {
		got := lintProtocolSummary(t, c.name, receiveMCPLint(t, c.done))
		items := lintServeObjects(t, got["data"])
		if len(items) != 1 || items[0]["node_id"] != asset || items[0]["item_key"] != "n:"+asset || items[0]["bytes"] != float64(12) || got["next_cursor"] != nil {
			t.Fatalf("actual %s pending flight result=%v", c.name, got)
		}
		if c.name == "rest" {
			actualREST = got
		} else {
			actualMCP = append(actualMCP, got)
		}
	}
	for _, got := range actualMCP {
		if got["computed_at"] != actualREST["computed_at"] {
			t.Fatal("pending, lint and REST did not share one actual snapshot")
		}
		mcpLintCompareREST(t, got, actualREST)
	}
	got := lintProtocolSummary(t, "mcp", receiveMCPLint(t, other))
	if len(lintServeObjects(t, got["data"])) != 0 {
		t.Fatalf("different wanted query received pending items: %v", got)
	}
}
