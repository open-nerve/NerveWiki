package bootstrap

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Only the existing transaction and snapshot ports are decorated. Every
// production precheck, writer, guard, participant and database stays installed.
// The competing source commits after the unlocked checks and before the actual
// write transaction begins. Removing the registered guard must change the result.
func TestSourceDuplicatesReachEveryLastHop(t *testing.T) {
	for _, scope := range []string{"notebook", "account"} {
		t.Run("batch/"+scope, func(t *testing.T) {
			port := &sourceLastHopPort{}
			tm := sourceLastHopTeam(t, port)
			wiki := tm.llmWiki(t, "Batch last hop")
			caller := tm.agent(t, "alice", wiki.id)
			const content = "source committed after the batch plan"
			args := batched(opWrite("Ordinary.md", "ordinary atomic predecessor"), opWrite("raw/Late.md", content))
			path := atNotebook(wiki.id)
			if scope == "account" {
				path, args["notebook"] = mcpserver.Pattern, wiki.id
			}
			body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call",
				"params": map[string]any{"name": "batch", "arguments": args}})
			if err != nil {
				t.Fatal(err)
			}
			req := newRequest(t, http.MethodPost, caller.base+path, caller.pat, body)
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("Mcp-Protocol-Version", mcpVersion)
			send := func() answer {
				res, err := client().Do(req)
				got := answer{req: req, res: res, err: err}
				if err == nil {
					payload, readErr := io.ReadAll(res.Body)
					_ = res.Body.Close()
					got.status, got.body, got.err = res.StatusCode, string(payload), readErr
				}
				return got
			}
			gate, done := sourceLastHopStart(t, port, send)
			target := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Known", content)
			before := sourceEntryRows(t, tm.pool)
			gate.open()
			got := sourceLastHopAnswer(t, done)
			result := (mcpAnswer{status: got.status, header: got.res.Header, body: []byte(got.body), tool: "batch"}).result(t)
			structured, _ := result["structuredContent"].(map[string]any)
			problem, _ := structured["error"].(map[string]any)
			message, _ := problem["message"].(string)
			if result["isError"] != true || problem["code"] != "page.source_duplicate" ||
				problem["op"] != float64(1) || problem["duplicate_of"] != target || !strings.Contains(message, "raw/Known.md") {
				t.Errorf("batch after unlocked plan = %s, want op 1 duplicate with committed target %s", got.body, target)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND name IN ('Ordinary','Late')", wiki.id); n != 0 {
				t.Errorf("refused batch retained %d of its two writes", n)
			}
			if after := sourceEntryRows(t, tm.pool); !maps.Equal(after, before) {
				t.Error("last-hop batch refusal changed persistent rows after the competitor committed")
			}
			sourceLastHopInvariants(t, tm, port)
		})
	}
	for _, operation := range []string{"upload", "restore", "revert"} {
		for _, kind := range []string{"page", "asset"} {
			if operation == "upload" && kind == "page" {
				continue
			}
			t.Run(operation+"/"+kind, func(t *testing.T) {
				port := &sourceLastHopPort{}
				tm := sourceLastHopTeam(t, port)
				wiki := tm.llmWiki(t, "REST last hop")
				const content = "source committed after the unlocked checks"
				var c step
				if operation == "upload" {
					c = assetUpload(t, "alice", wiki.id, wiki.raw, "late.bin", content)
				} else {
					var original string
					if kind == "page" {
						original = tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Original", content)
					} else {
						original = tm.upload(t, "alice", wiki.id, wiki.raw, "original.bin", content).ID
					}
					if operation == "restore" {
						tm.send(t, nodeDeletion("alice", original), http.StatusNoContent)
						c = nodeRestore("alice", original)
					} else {
						tm.send(t, nodeMove("alice", original, ""), http.StatusOK)
						c = changesetRevert("alice", tm.newest(t, wiki.id).ID)
					}
				}
				files := storedBlobs(t, tm.storage)
				snapshots := port.completed.Load()
				gate, done := sourceLastHopStart(t, port, tm.sender(t, c))
				if operation != "upload" && port.completed.Load() != snapshots+1 {
					t.Errorf("%s reached write without one completed source snapshot: before=%d after=%d", operation, snapshots, port.completed.Load())
				}
				if operation == "upload" {
					pending := storedBlobs(t, tm.storage)
					if len(pending) != len(files)+1 {
						t.Fatalf("upload reached write with %d files, want its completed Put after %d existing files", len(pending), len(files))
					}
				}
				var target string
				if kind == "page" {
					target = tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Known", content)
				} else {
					// upload's convenience helper checks the whole asset store.
					// The blocked request deliberately has a Put without its row
					// here; check the complete store only after both requests end.
					upload := assetUpload(t, "alice", wiki.id, wiki.raw, "known.bin", content)
					status, body := askTyped(t, tm.contract, upload.method, tm.base+upload.path,
						tm.tokens[upload.by], upload.contentType, upload.body)
					if status != http.StatusCreated {
						t.Fatalf("committed competing upload = %d %s", status, body)
					}
					var created uploadedAsset
					decodeAnswer(t, body, &created)
					target = created.ID
					// Only the competitor's committed file joins the expected set;
					// the blocked upload's already-Put file must be removed.
					ids := queryStrings(t, tm.pool, "SELECT id::text FROM asset_blobs WHERE node_id=$1", target)
					if len(ids) != 1 {
						t.Fatalf("committed target has blob IDs %v", ids)
					}
					all := storedBlobs(t, tm.storage)
					path, ok := all[ids[0]]
					if !ok {
						t.Fatal("committed competitor has no stored file")
					}
					files[ids[0]] = path
				}
				before := sourceEntryRows(t, tm.pool)
				gate.open()
				got := sourceLastHopAnswer(t, done)
				tm.contract.CheckResponse(t, got.req, got.res)
				var problem struct {
					Code        string  `json:"code"`
					DuplicateOf *string `json:"duplicate_of"`
				}
				decodeAnswer(t, got.body, &problem)
				if got.status != http.StatusConflict || problem.Code != "page.source_duplicate" || problem.DuplicateOf == nil || *problem.DuplicateOf != target {
					t.Errorf("%s last hop = %d %s, want duplicate committed target %s", operation, got.status, got.body, target)
				}
				if after := sourceEntryRows(t, tm.pool); !maps.Equal(after, before) {
					t.Error("last-hop refusal changed persisted tree, trash, history, bytes or metadata")
				}
				if after := storedBlobs(t, tm.storage); !maps.Equal(after, files) {
					t.Error("last-hop refusal left its uploaded file or changed a retained file")
				}
				sourceLastHopInvariants(t, tm, port)
			})
		}
	}
}

type sourceLastHopPort struct {
	page.Tx
	shared.Snapshots
	mu        sync.Mutex
	gate      *sourceWriteGate
	entered   atomic.Int32
	completed atomic.Int32
}

type sourceWriteGate struct {
	reached, release chan struct{}
	once             sync.Once
}

func (g *sourceWriteGate) open() { g.once.Do(func() { close(g.release) }) }

func (p *sourceLastHopPort) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	p.mu.Lock()
	g := p.gate
	p.gate = nil
	p.mu.Unlock()
	if g != nil {
		p.entered.Add(1)
		close(g.reached)
		select {
		case <-g.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return p.Tx.WithinTx(ctx, fn)
}

func (p *sourceLastHopPort) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	err := p.Snapshots.WithinSnapshot(ctx, fn)
	if err == nil {
		p.completed.Add(1)
	}
	return err
}

func sourceLastHopTeam(t *testing.T, port *sourceLastHopPort) acmeTeam {
	t.Helper()
	return newAcmeTeamWiring(t, "member", "", wiring{page: func(d *page.Deps) {
		port.Tx, port.Snapshots = d.Tx, d.Snapshots
		d.Tx, d.Snapshots = port, port
	}})
}

func sourceLastHopStart(t *testing.T, port *sourceLastHopPort, send func() answer) (*sourceWriteGate, <-chan answer) {
	t.Helper()
	gate := &sourceWriteGate{reached: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(gate.open)
	port.mu.Lock()
	port.gate = gate
	port.mu.Unlock()
	done := make(chan answer, 1)
	go func() { done <- send() }()
	select {
	case <-gate.reached:
	case got := <-done:
		t.Fatalf("request answered before the actual write transaction: %d %s (%v)", got.status, got.body, got.err)
	case <-time.After(interleavingWait):
		t.Fatal("request did not reach the actual write transaction")
	}
	return gate, done
}

func sourceLastHopAnswer(t *testing.T, done <-chan answer) answer {
	t.Helper()
	select {
	case got := <-done:
		if got.err != nil || got.res == nil {
			t.Fatalf("last-hop request failed: %v", got.err)
		}
		return got
	case <-time.After(interleavingWait):
		t.Fatal("last-hop request did not answer after release")
		return answer{}
	}
}

func sourceLastHopInvariants(t *testing.T, tm acmeTeam, port *sourceLastHopPort) {
	t.Helper()
	if got := port.entered.Load(); got != 1 {
		t.Errorf("last-hop barrier entered %d times, want exactly once", got)
	}
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}
