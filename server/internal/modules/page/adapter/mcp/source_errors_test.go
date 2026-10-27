package mcpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type sourceNotebooks struct{}

func (sourceNotebooks) Gate(context.Context, uuid.UUID) error                   { return nil }
func (sourceNotebooks) Instructions(context.Context, uuid.UUID) (string, error) { return "", nil }

// The adapters must retain a wrapped source refusal through the actual MCP
// error encoder, including the failed op's place. The source rules and each
// scope's real authorization are independently exercised in bootstrap.
func TestTheSourceRefusalsKeepTheirMCPStructure(t *testing.T) {
	notebook, node := uuid.NewV7(), uuid.NewV7()
	immutable := domain.Refuse(shared.NewError(shared.KindConflict, "page.source_immutable", "Sources are immutable."), node)
	adminOnly := domain.Refuse(domain.ErrSourceAdminOnly, node)
	tooLarge := domain.Refuse(domain.ErrSourceTooLarge, node)
	uc := UseCases{
		Write:  writeFunc(func(uuid.UUID, app.PathWrite, domain.Client) (app.Written, error) { return app.Written{}, immutable }),
		Edit:   editFunc(func(uuid.UUID, app.PageEdit, domain.Client) (app.Edited, error) { return app.Edited{}, immutable }),
		Move:   moveFunc(func(uuid.UUID, app.PathMove, domain.Client) (app.Relocated, error) { return app.Relocated{}, tooLarge }),
		Delete: deleteFunc(func(uuid.UUID, app.PathDelete, domain.Client) (app.Deleted, error) { return app.Deleted{}, adminOnly }),
		Batch: batchFunc(func(_ uuid.UUID, p app.PathBatch, _ domain.Client) (app.Batched, error) {
			if len(p.Ops) > 1 && p.Ops[1].Op == "move" {
				return app.Batched{}, failedAt(1, domain.OpKindMove, tooLarge)
			}
			return app.Batched{}, failedAt(1, domain.OpKindWrite, immutable)
		}),
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := mcpserver.New(mcpserver.Options{
		Name: "source-errors", Version: "test", Tools: Tools(uc, nil), Notebooks: sourceNotebooks{},
		Credentials: func(context.Context) (mcpserver.Credential, bool) {
			return mcpserver.Credential{UserID: uuid.NewV7(), TokenID: uuid.NewV7()}, true
		},
		Errors: httpserver.NewAPIErrors(logger), RequestTimeout: time.Second, MaxBodyBytes: 1 << 20, Logger: logger,
	})
	for _, scope := range []string{"notebook", "account"} {
		for _, tt := range []struct {
			tool, args, code string
		}{
			{"write", `{"path":"raw/source.md","content":"changed","base_version":"x:1"}`, "page.source_immutable"},
			{"edit", `{"path":"raw/source.md","old_string":"old","new_string":"new"}`, "page.source_immutable"},
			{"delete", `{"path":"raw/evidence.pdf"}`, "page.source_admin_only"},
			{"move", `{"from":"Large.md","to":"raw/Large.md"}`, "page.source_too_large"},
			{"batch", `{"ops":[{"op":"write","path":"ordinary.md","content":"ok"},{"op":"move","from":"Large.md","to":"raw/Large.md"}],"message":"move source"}`, "page.source_too_large"},
			{"batch", `{"ops":[{"op":"write","path":"ordinary.md","content":"ok"},{"op":"write","path":"raw/source.md","content":"new"}],"message":"change"}`, "page.source_immutable"},
		} {
			t.Run(scope+"/"+tt.tool+"/"+tt.code, func(t *testing.T) {
				var args map[string]any
				if err := json.Unmarshal([]byte(tt.args), &args); err != nil {
					t.Fatal(err)
				}
				url := "/mcp?notebook=" + notebook.String()
				if scope == "account" {
					url, args["notebook"] = "/mcp", notebook.String()
				}
				body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tt.tool, "arguments": args}})
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				req.Header.Set("Mcp-Protocol-Version", "2025-11-25")
				w := httptest.NewRecorder()
				srv.ServeHTTP(w, req)
				var answer struct {
					Result struct {
						IsError bool `json:"isError"`
						Content []struct {
							Text string `json:"text"`
						} `json:"content"`
						Structured struct {
							Error struct {
								Code    string `json:"code"`
								Message string `json:"message"`
								Op      *int   `json:"op"`
							} `json:"error"`
						} `json:"structuredContent"`
					} `json:"result"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
					t.Fatal(err)
				}
				got := answer.Result.Structured.Error
				if w.Code != http.StatusOK || !answer.Result.IsError || got.Code != tt.code || len(answer.Result.Content) != 1 {
					t.Fatalf("answer = %d %s, want structured %s", w.Code, w.Body, tt.code)
				}
				if answer.Result.Content[0].Text != got.Message || !strings.HasPrefix(got.Message, tt.code+": ") {
					t.Fatalf("text and structured error disagree: %s", w.Body)
				}
				if tt.tool == "batch" {
					wantOp := "ops[1] (write)"
					if tt.code == "page.source_too_large" {
						wantOp = "ops[1] (move)"
					}
					if got.Op == nil || *got.Op != 1 || !strings.Contains(got.Message, wantOp) {
						t.Fatalf("batch lost its op: %s", w.Body)
					}
				} else if got.Op != nil {
					t.Fatalf("single tool acquired an op: %s", w.Body)
				}
			})
		}
	}
}
