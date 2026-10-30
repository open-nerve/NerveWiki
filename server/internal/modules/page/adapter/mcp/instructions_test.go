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

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// Measure the SDK's actual text-string encoding with a test-only text tool.
// Bootstrap separately checks the production handshake and the complete list.
func TestTheBuiltInInstructionsAreShort(t *testing.T) {
	for name, text := range map[string]string{"the notebook entry's": NotebookInstructions, "the account entry's": AccountInstructions} {
		t.Run(name, func(t *testing.T) {
			api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: instructionAuth{}})
			server := mcpserver.New(mcpserver.Options{Name: "instruction-probe", Version: "test", Account: AccountInstructions,
				Credentials: func(context.Context) (mcpserver.Credential, bool) {
					return mcpserver.Credential{UserID: uuid.UUID{1}, TokenID: uuid.UUID{2}}, true
				},
				Notebooks: instructionNotebooks{}, Errors: api.Errors, RequestTimeout: 4 * time.Second, MaxBodyBytes: 1 << 20, Logger: slog.New(slog.DiscardHandler),
				Tools: []mcpserver.Tool{mcpserver.NewTool(mcpserver.Spec{Name: "instruction_probe", Entry: mcpserver.BothEntries}, func(context.Context, mcpserver.Call, struct{}) (mcpserver.Result, error) {
					return mcpserver.Result{Blocks: []mcpserver.Block{{Text: text}}}, nil
				})},
			})
			path := "/mcp"
			if name == "the notebook entry's" {
				path += "?notebook=" + uuid.UUID{3}.String()
			}
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"instruction_probe","arguments":{}}}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("MCP-Protocol-Version", "2025-11-25")
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			var got struct {
				Result struct {
					IsError bool `json:"isError"`
					Content []struct {
						Type string          `json:"type"`
						Text json.RawMessage `json:"text"`
					} `json:"content"`
				} `json:"result"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if rec.Code != 200 || got.Result.IsError || len(got.Result.Content) != 1 || got.Result.Content[0].Type != "text" {
				t.Fatalf("instruction SDK probe failed: %d %s", rec.Code, rec.Body.String())
			}
			raw := got.Result.Content[0].Text
			var decoded string
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			n := len(raw) - 2
			if decoded != text || n != mcpserver.TextBytes(text) {
				t.Fatal("SDK text differs from production instructions or TextBytes")
			}
			t.Logf("instruction bytes raw=%d SDK=%d remaining=%d", len(text), n, 4096-n)
			if n > 4096-300 {
				t.Errorf("instructions SDK bytes %d exceed 3796; fewer than 300 bytes remain", n)
			}
			if !strings.Contains(text, pathsAndTools) {
				t.Error("instructions lack the shared paths and tools")
			}
		})
	}
	if !strings.Contains(NotebookInstructions, "A root page titled AGENTS") || strings.Contains(AccountInstructions, "AGENTS") {
		t.Error("the notebook entry's instructions, and they alone, tell of the AGENTS page")
	}
}

type instructionAuth struct{}

func (instructionAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	return ctx, token, nil
}

type instructionNotebooks struct{}

func (instructionNotebooks) Gate(ctx context.Context, _ uuid.UUID) error { return ctx.Err() }
func (instructionNotebooks) Instructions(context.Context, uuid.UUID) (string, error) {
	return NotebookInstructions, nil
}
