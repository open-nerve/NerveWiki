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

type sourcePathFunc func(context.Context, uuid.UUID, uuid.UUID) (string, error)

func (f sourcePathFunc) Execute(ctx context.Context, notebook, node uuid.UUID) (string, error) {
	return f(ctx, notebook, node)
}

// Path resolution is explanatory only: a failed read must not replace the refusal,
// and a rolled-back internal duplicate has no public target to resolve.
func TestDuplicateSourcesKeepTheirMCPCodeTargetAndPlace(t *testing.T) {
	notebook, target := uuid.NewV7(), uuid.NewV7()
	for _, scope := range []string{"notebook", "account"} {
		for _, resolution := range []string{"visible", "missing", "read-error", "within-operation"} {
			for _, op := range []string{"write", "move", "batch-write", "batch-move"} {
				t.Run(scope+"/"+resolution+"/"+op, func(t *testing.T) {
					id := &target
					if resolution == "within-operation" {
						id = nil
					}
					refusal := domain.Refuse(domain.SourceDuplicate(id), uuid.NewV7())
					reads := 0
					uc := UseCases{
						Write: writeFunc(func(uuid.UUID, app.PathWrite, domain.Client) (app.Written, error) { return app.Written{}, refusal }),
						Move:  moveFunc(func(uuid.UUID, app.PathMove, domain.Client) (app.Relocated, error) { return app.Relocated{}, refusal }),
						Batch: batchFunc(func(_ uuid.UUID, p app.PathBatch, _ domain.Client) (app.Batched, error) {
							kind := domain.OpKindWrite
							if p.Ops[1].Op == "move" {
								kind = domain.OpKindMove
							}
							return app.Batched{}, failedAt(1, kind, refusal)
						}),
						TargetPath: sourcePathFunc(func(_ context.Context, n, id uuid.UUID) (string, error) {
							reads++
							if n != notebook || id != target {
								t.Errorf("resolved %s/%s, want %s/%s", n, id, notebook, target)
							}
							switch resolution {
							case "visible":
								return "raw/folder/existing.md", nil
							case "missing":
								return "", domain.ErrNotFound
							default:
								return "", io.ErrUnexpectedEOF
							}
						}),
					}
					logger := slog.New(slog.NewTextHandler(io.Discard, nil))
					srv := mcpserver.New(mcpserver.Options{Name: "duplicate-errors", Version: "test", Tools: Tools(uc, nil), Notebooks: sourceNotebooks{}, Credentials: func(context.Context) (mcpserver.Credential, bool) {
						return mcpserver.Credential{UserID: uuid.NewV7(), TokenID: uuid.NewV7()}, true
					}, Errors: httpserver.NewAPIErrors(logger), RequestTimeout: time.Second, MaxBodyBytes: 1 << 20, Logger: logger})
					tool := op
					args := map[string]any{"path": "raw/new.md", "content": "same"}
					if op == "move" {
						args = map[string]any{"from": "outside.md", "to": "raw/new.md"}
					}
					if strings.HasPrefix(op, "batch-") {
						tool = "batch"
						item := map[string]any{"op": "write", "path": "raw/new.md", "content": "same"}
						if op == "batch-move" {
							item = map[string]any{"op": "move", "from": "outside.md", "to": "raw/new.md"}
						}
						args = map[string]any{"ops": []any{map[string]any{"op": "write", "path": "ordinary.md", "content": "ok"}, item}, "message": "add source"}
					}
					url := "/mcp?notebook=" + notebook.String()
					if scope == "account" {
						url = "/mcp"
						args["notebook"] = notebook.String()
					}
					body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
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
								Error map[string]json.RawMessage `json:"error"`
							} `json:"structuredContent"`
						} `json:"result"`
					}
					if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
						t.Fatal(err)
					}
					got := answer.Result.Structured.Error
					if w.Code != http.StatusOK || !answer.Result.IsError || string(got["code"]) != `"page.source_duplicate"` || len(answer.Result.Content) != 1 {
						t.Fatalf("answer = %d %s", w.Code, w.Body)
					}
					duplicate, exists := got["duplicate_of"]
					if id == nil && exists || id != nil && string(duplicate) != `"`+target.String()+`"` {
						t.Errorf("target lost or invented: %s", w.Body)
					}
					wantReads := 1
					if id == nil {
						wantReads = 0
					}
					if reads != wantReads {
						t.Errorf("path reads = %d, want %d", reads, wantReads)
					}
					message := answer.Result.Content[0].Text
					var structuredMessage string
					if err := json.Unmarshal(got["message"], &structuredMessage); err != nil {
						t.Fatal(err)
					}
					if message != structuredMessage || !strings.HasPrefix(message, "page.source_duplicate: ") {
						t.Errorf("inconsistent message: %s", w.Body)
					}
					if strings.Contains(message, "raw/folder/existing.md") != (resolution == "visible") {
						t.Errorf("visible path mismatch: %q", message)
					}
					if tool == "batch" {
						kind := "write"
						if op == "batch-move" {
							kind = "move"
						}
						if string(got["op"]) != "1" || !strings.Contains(message, "ops[1] ("+kind+")") {
							t.Errorf("batch lost place: %s", w.Body)
						}
					} else if _, ok := got["op"]; ok {
						t.Errorf("single tool acquired op: %s", w.Body)
					}
				})
			}
		}
	}
}
