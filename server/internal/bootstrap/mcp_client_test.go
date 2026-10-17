package bootstrap

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The whole program's MCP clients (M9/P1 design 3.4.9): the app with the
// tools a test registers beside the modules' (wiring.mcpTools), the
// accounts and notebooks its calls aim at, raw JSON-RPC requests, and the
// official Go client, its token on every request it sends.

// mcpVersion is the protocol version the tests' requests carry after a
// handshake, the latest the SDK speaks.
const mcpVersion = "2025-11-25"

// initializeBody is a client's handshake of version.
func initializeBody(version string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"` + version +
		`","capabilities":{},"clientInfo":{"name":"whole-program","version":"1"}}}`
}

// mcpApp is a running app and what its MCP calls are made with.
type mcpApp struct {
	base     string
	url      string // its database
	logs     *syncBuffer
	contract *apitest.Contract
	access   string // alice's session access token
	pat      string // alice's personal access token
	notebook string // alice's notebook in acme
	hidden   string // bob's notebook, in a workspace alice is no member of
	deleted  string // a notebook of alice's, deleted
}

// startMCPApp runs the app on a database of its own with tools beside the
// modules', logging JSON lines from debug up, and prepares what its calls
// aim at: alice and her personal access token, acme and its notebook, a
// notebook of hers deleted, and bob's notebook in beta.
func startMCPApp(t *testing.T, tools []mcpserver.Tool) *mcpApp {
	t.Helper()
	url := pgtest.NewDatabase(t)
	logs := &syncBuffer{}
	return runMCPApp(t, wireMCPApp(t, testConfig(t, url, false), logs, tools), url, logs)
}

// runMCPApp runs a, on the database at url, logging to logs, and prepares
// what startMCPApp's calls aim at.
func runMCPApp(t *testing.T, a *app, url string, logs *syncBuffer) *mcpApp {
	t.Helper()
	m := &mcpApp{base: runApp(t, a), url: url, logs: logs, contract: apitest.Load(t)}
	m.access = registerAccount(t, m.contract, m.base, "alice@example.com").AccessToken
	m.pat = createToken(t, m.contract, m.base, m.access)
	m.notebook = m.createNotebook(t, m.access, "acme", "Notes")
	m.deleted = m.createNotebook(t, m.access, "acme", "Old")
	if status, answer := ask(t, m.contract, http.MethodDelete, m.base+"/api/v0/notebooks/"+m.deleted, m.access, ""); status != http.StatusNoContent {
		t.Fatalf("delete Old = %d %s", status, answer)
	}
	bob := registerAccount(t, m.contract, m.base, "bob@example.com").AccessToken
	m.hidden = m.createNotebook(t, bob, "beta", "Private")
	return m
}

// wireMCPApp wires the app of cfg with tools beside the modules', logging
// to logs, and closes it when the test ends.
func wireMCPApp(t *testing.T, cfg config.Config, logs *syncBuffer, tools []mcpserver.Tool) *app {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a, err := wireApp(context.Background(), cfg, logger, migrations.FS(), testWebUI(), wiring{mcpTools: tools})
	if err != nil {
		t.Fatalf("wiring the app: %v", err)
	}
	t.Cleanup(a.close)
	return a
}

// createNotebook creates the workspace slug, unless the token's account
// has it, and a notebook named name in it, through the API, and returns
// the notebook's id.
func (m *mcpApp) createNotebook(t *testing.T, token, slug, name string) string {
	t.Helper()
	if status, _ := ask(t, m.contract, http.MethodGet, m.base+"/api/v0/workspaces/"+slug, token, ""); status == http.StatusNotFound {
		if status, answer := ask(t, m.contract, http.MethodPost, m.base+"/api/v0/workspaces", token,
			`{"name":"`+slug+`","slug":"`+slug+`"}`); status != http.StatusCreated {
			t.Fatalf("create %s = %d %s", slug, status, answer)
		}
	}
	status, answer := ask(t, m.contract, http.MethodPost, m.base+"/api/v0/workspaces/"+slug+"/notebooks", token, `{"name":"`+name+`"}`)
	var n struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(answer), &n); status != http.StatusCreated || err != nil {
		t.Fatalf("create %s in %s = %d %s", name, slug, status, answer)
	}
	return n.ID
}

// mcpSeen is what the test's tools saw: echo's calls, and when slow's
// context ended.
type mcpSeen struct {
	echoes    atomic.Int64
	slowEnded chan time.Time
}

func newMCPSeen() *mcpSeen { return &mcpSeen{slowEnded: make(chan time.Time, 8)} }

// echoArgs are echo's arguments: one optional string.
type echoArgs struct {
	S *string `json:"s,omitempty" jsonschema:"Any string."`
}

// mcpTestTools are the whole-program tests' tools, in both entries: echo
// answers what its call was told, and counts its calls; boom panics; slow
// waits for its context to end and answers its error.
func mcpTestTools(seen *mcpSeen) []mcpserver.Tool {
	echo := mcpserver.NewTool(mcpserver.Spec{Name: "echo", Title: "Echo", Description: "Answers what its call was told.",
		Annotations: mcpserver.Annotations{ReadOnly: true, Idempotent: true}, Entry: mcpserver.BothEntries, Notebook: true},
		func(_ context.Context, c mcpserver.Call, in echoArgs) (mcpserver.Result, error) {
			seen.echoes.Add(1)
			out := map[string]any{"entry": c.Entry.String(), "notebook": c.Notebook.String(), "client": c.Client}
			if in.S != nil {
				out["s"] = *in.S
			}
			return mcpserver.Result{Structured: out}, nil
		})
	boom := mcpserver.NewTool(mcpserver.Spec{Name: "boom", Title: "Boom", Description: "Panics.", Entry: mcpserver.BothEntries},
		func(context.Context, mcpserver.Call, struct{}) (mcpserver.Result, error) { panic("boom") })
	slow := mcpserver.NewTool(mcpserver.Spec{Name: "slow", Title: "Slow", Description: "Waits for its context to end.", Entry: mcpserver.BothEntries},
		func(ctx context.Context, _ mcpserver.Call, _ struct{}) (mcpserver.Result, error) {
			<-ctx.Done()
			seen.slowEnded <- time.Now()
			return mcpserver.Result{}, ctx.Err()
		})
	return []mcpserver.Tool{echo, boom, slow}
}

// mcpAnswer is an answer of the MCP routes as a test reads it.
type mcpAnswer struct {
	status int
	header http.Header
	body   []byte
	took   time.Duration // from the request's start to its body's end
}

// afterHandshake is the header of a client's request after a handshake
// of mcpVersion.
func afterHandshake() http.Header { return http.Header{"Mcp-Protocol-Version": {mcpVersion}} }

// post sends body to path, with its query, as a client posts JSON-RPC,
// with token and header; ctx ends it: an answer cut short has the status
// -1.
func (m *mcpApp) post(t testing.TB, ctx context.Context, path, token, body string, header http.Header) mcpAnswer {
	t.Helper()
	return m.send(t, ctx, http.MethodPost, path, token, strings.NewReader(body), header)
}

// send is post of any method and body.
func (m *mcpApp) send(t testing.TB, ctx context.Context, method, path, token string, body io.Reader, header http.Header) mcpAnswer {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, m.base+path, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	start := time.Now()
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return mcpAnswer{status: -1}
		}
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		if ctx.Err() != nil {
			return mcpAnswer{status: -1}
		}
		t.Fatalf("%s %s: reading the answer: %v", method, path, err)
	}
	return mcpAnswer{status: res.StatusCode, header: res.Header, body: b, took: time.Since(start)}
}

// call is a raw tools/call of tool with args to path, with alice's
// personal access token, after a handshake.
func (m *mcpApp) call(t testing.TB, path, tool string, args any) mcpAnswer {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	return m.post(t, context.Background(), path, m.pat, string(body), afterHandshake())
}

// atNotebook is the path of the notebook entry of id.
func atNotebook(id string) string { return mcpserver.Pattern + "?notebook=" + id }

// decoded is the answer's JSON: a JSON-RPC answer, or a problem.
func (a mcpAnswer) decoded(t testing.TB) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(a.body, &v); err != nil {
		t.Fatalf("answer %d %s: %v", a.status, a.body, err)
	}
	return v
}

// problem is the answer's problem code, failing unless it is a problem
// with status.
func (a mcpAnswer) problem(t testing.TB, status int) string {
	t.Helper()
	if a.status != status || a.header.Get("Content-Type") != "application/problem+json" {
		t.Fatalf("answer = %d %q %s, want a problem %d", a.status, a.header.Get("Content-Type"), a.body, status)
	}
	code, _ := a.decoded(t)["code"].(string)
	return code
}

// code is the answer's problem code, "" for an answer that is no
// problem.
func (a mcpAnswer) code() string {
	var p struct {
		Code string `json:"code"`
	}
	if a.header.Get("Content-Type") != "application/problem+json" || json.Unmarshal(a.body, &p) != nil {
		return ""
	}
	return p.Code
}

// result is a JSON-RPC answer's result, failing on anything else.
func (a mcpAnswer) result(t testing.TB) map[string]any {
	t.Helper()
	r, ok := a.decoded(t)["result"].(map[string]any)
	if a.status != http.StatusOK || !ok {
		t.Fatalf("answer = %d %s, want a result", a.status, a.body)
	}
	return r
}

// toolError is structuredContent.error of a tool's error, failing on a
// result that is no error.
func toolError(t testing.TB, r map[string]any) map[string]any {
	t.Helper()
	sc, _ := r["structuredContent"].(map[string]any)
	member, ok := sc["error"].(map[string]any)
	if r["isError"] != true || !ok {
		t.Fatalf("result %v is no tool error", r)
	}
	return member
}

// withToken adds a token to every request of a client.
type withToken struct {
	token string
	next  http.RoundTripper
}

func (w withToken) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+w.token)
	return w.next.RoundTrip(r)
}

// mcpClient is the official Go client's session of endpoint with token,
// on the default options, closed when the test ends.
func mcpClient(t testing.TB, endpoint, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "whole-program", Version: "1"}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: endpoint,
		HTTPClient: &http.Client{Transport: withToken{token, http.DefaultTransport}, Timeout: 30 * time.Second}}
	cs, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("connecting to %s: %v", endpoint, err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// logLines are the app's log lines, decoded.
func (m *mcpApp) logLines(t testing.TB) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(m.logs.String()), "\n") {
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		lines = append(lines, e)
	}
	return lines
}

// logged is the first log line of msg, or nil.
func (m *mcpApp) logged(t testing.TB, msg string) map[string]any {
	t.Helper()
	for _, e := range m.logLines(t) {
		if e["msg"] == msg {
			return e
		}
	}
	return nil
}

// waitLogged waits up to limit for a log line of msg and returns it, or
// nil.
func (m *mcpApp) waitLogged(t testing.TB, msg string, limit time.Duration) map[string]any {
	t.Helper()
	for deadline := time.Now().Add(limit); ; time.Sleep(10 * time.Millisecond) {
		if e := m.logged(t, msg); e != nil || time.Now().After(deadline) {
			return e
		}
	}
}

// loggedAt are the log lines of level.
func (m *mcpApp) loggedAt(t testing.TB, level string) []map[string]any {
	t.Helper()
	var found []map[string]any
	for _, e := range m.logLines(t) {
		if e["level"] == level {
			found = append(found, e)
		}
	}
	return found
}
