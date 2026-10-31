package mcpadapter

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type lintUseFunc func(context.Context, app.LintScope, app.LintQuery) (domain.LintResult, error)

func (f lintUseFunc) Execute(ctx context.Context, scope app.LintScope, query app.LintQuery) (domain.LintResult, error) {
	return f(ctx, scope, query)
}

func lintID(n byte) uuid.UUID { var id uuid.UUID; id[15] = n; return id }
func lintTime() time.Time     { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC) }

type lintNotebookGate struct{}

type lintAuth struct{}

func (lintAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	return shared.WithActor(ctx, shared.Actor{UserID: lintID(1), APITokenID: lintID(2)}), token, nil
}

func (lintNotebookGate) Gate(ctx context.Context, _ uuid.UUID) error             { return ctx.Err() }
func (lintNotebookGate) Instructions(context.Context, uuid.UUID) (string, error) { return "", nil }

func lintServer(t *testing.T, uc LintUseCase) http.Handler {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: lintAuth{}})
	server := mcpserver.New(mcpserver.Options{Name: "wiki-test", Version: "test", Tools: Tools(uc), Credentials: func(context.Context) (mcpserver.Credential, bool) {
		return mcpserver.Credential{UserID: lintID(1), TokenID: lintID(2)}, true
	}, Notebooks: lintNotebookGate{}, Errors: api.Errors, RequestTimeout: 4 * time.Second, MaxBodyBytes: 1 << 20, Logger: log})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r.WithContext(shared.WithActor(r.Context(), shared.Actor{UserID: lintID(1), APITokenID: lintID(2)})))
	})
}

type lintSDKResult struct {
	IsError bool `json:"isError"`
	Content []struct {
		Type string          `json:"type"`
		Text json.RawMessage `json:"text"`
	} `json:"content"`
	Structured json.RawMessage `json:"structuredContent"`
}

func lintRPC(t *testing.T, handler http.Handler, path, method string, params any) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(envelope.Result) == 0 || len(envelope.Error) != 0 {
		t.Fatalf("SDK HTTP/RPC response: %d %s", rec.Code, rec.Body.String())
	}
	return envelope.Result
}

func lintCall(t *testing.T, handler http.Handler, account bool, name string, args map[string]any) (lintSDKResult, int) {
	t.Helper()
	path := "/mcp?notebook=" + lintID(10).String()
	if args == nil {
		args = map[string]any{}
	}
	if account {
		path = "/mcp"
		args["notebook"] = lintID(10).String()
	}
	raw := lintRPC(t, handler, path, "tools/call", map[string]any{"name": name, "arguments": args})
	var result lintSDKResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) != 1 || len(result.Structured) == 0 {
		t.Fatalf("nonempty successful SDK result required: %s", raw)
	}
	n := 0
	for _, block := range result.Content {
		if block.Type != "text" || len(block.Text) < 2 {
			t.Fatalf("expected SDK text block: %s", raw)
		}
		var text string
		if err := json.Unmarshal(block.Text, &text); err != nil {
			t.Fatal(err)
		}
		encoded := len(block.Text) - 2
		if encoded != mcpserver.TextBytes(text) {
			t.Fatalf("actual SDK text bytes %d != TextBytes %d", encoded, mcpserver.TextBytes(text))
		}
		n += encoded
		var content, structured any
		if err := json.Unmarshal([]byte(text), &content); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(result.Structured, &structured); err != nil {
			t.Fatal(err)
		}
		a, _ := json.Marshal(content)
		b, _ := json.Marshal(structured)
		if string(a) != string(b) {
			t.Fatalf("SDK content and structured answer differ: %s", raw)
		}
	}
	if n > 49152 {
		t.Fatalf("actual SDK text bytes %d exceed 49152", n)
	}
	return result, n
}

func TestLintPassesItsArgumentsAndProjects(t *testing.T) {
	for _, account := range []bool{false, true} {
		for _, name := range []string{"lint", "pending_sources"} {
			t.Run(name+"/"+map[bool]string{false: "notebook", true: "account"}[account], func(t *testing.T) {
				id := lintID(0)
				path := "Notes/zero.md"
				rule := domain.RuleOrphans
				if name == "pending_sources" {
					rule = domain.RuleUndigestedSources
					path = "Notes/zero.bin"
				}
				position := linking.Position{ID: &id}
				if name == "pending_sources" {
					position.CreatedAt = lintTime()
				}
				cursor, err := linking.EncodeLintCursor(linking.CursorBinding{NotebookID: lintID(10)}, linking.Rule(rule), position)
				if err != nil {
					t.Fatal(err)
				}
				begin, err := linking.EncodeLintCursor(linking.CursorBinding{NotebookID: lintID(10)}, linking.Rule(rule), linking.Position{Begin: true})
				if err != nil {
					t.Fatal(err)
				}
				item := domain.LintItem{Rule: rule, ItemKey: "p:" + id.String(), Cursor: cursor, PageID: &id, Path: &path}
				if name == "pending_sources" {
					item.PageID = nil
					item.NodeID = &id
					item.Kind = domain.NodeAsset
					item.CreatedAt = lintTime()
					item.ItemKey = "n:" + id.String()
				}
				calls := 0
				server := lintServer(t, lintUseFunc(func(_ context.Context, scope app.LintScope, query app.LintQuery) (domain.LintResult, error) {
					calls++
					if scope.NotebookID != lintID(10) || scope.Contract != nil || scope.AreaRootID != nil || query.Rule == nil || *query.Rule != string(rule) || query.Limit == nil || *query.Limit != 1 || query.Cursor == nil || *query.Cursor != begin {
						t.Errorf("tool input changed: %+v %+v", scope, query)
					}
					return domain.LintResult{Kind: domain.LintItems, Mode: domain.ModeLLMWiki, ComputedAt: lintTime(), StalePages: 3, Rules: []domain.LintRuleResult{{Rule: rule, Count: domain.LintCount{Value: 1}, Items: []domain.LintItem{item}, StartCursor: &begin}}}, nil
				}))
				args := map[string]any{"limit": 1, "cursor": begin}
				if name == "lint" {
					args["rule"] = "orphans"
				}
				result, _ := lintCall(t, server, account, name, args)
				var page struct {
					Mode  string           `json:"mode"`
					At    string           `json:"computed_at"`
					Stale int              `json:"stale_pages"`
					Data  []map[string]any `json:"data"`
					Next  *string          `json:"next_cursor"`
				}
				if err := json.Unmarshal(result.Structured, &page); err != nil {
					t.Fatal(err)
				}
				if calls != 1 || page.Mode != "llm_wiki" || page.At != "2026-01-02T03:04:05Z" || page.Stale != 3 || len(page.Data) != 1 || page.Next != nil {
					t.Fatalf("tool projection: calls%d %s", calls, result.Structured)
				}
				if page.Data[0]["path"] != path || page.Data[0]["item_key"] != item.ItemKey || page.Data[0]["cursor"] != cursor {
					t.Fatalf("identity/path projection: %s", result.Structured)
				}
			})
		}
	}
}
