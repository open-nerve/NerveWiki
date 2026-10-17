package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// The whole program's MCP edge, transport and recovery (M9/P1 design
// 3.4.9), over serve and a real database, through the test's tools.

// mcpBodyLimit's value: a page's content of 5 MiB and 1 MiB besides, six
// bytes each, and 64 KiB.
const mcpBodyBytes = 37_814_272

// The edge answers in its order (M9/P1 design 3.2.2): the credential's
// 401; the body's 413 and 400; a session's access token's 403, before any
// notebook is looked at; the SDK's 405 for any method but POST, before
// the notebook; then the notebook entry's 404.
func TestTheMCPEdgeAnswersInOrder(t *testing.T) {
	m := startMCPApp(t, mcpTestTools(newMCPSeen()))
	revokedID, revoked := createTokenWithID(t, m.contract, m.base, m.access)
	if status, answer := ask(t, m.contract, http.MethodDelete, m.base+"/api/v0/api-tokens/"+revokedID, m.access, ""); status != http.StatusNoContent {
		t.Fatalf("revoke a token = %d %s", status, answer)
	}
	initialize := initializeBody(mcpVersion)
	tests := []struct {
		name, method, path, token, body string
		status                          int
		code                            string // the problem's; "" for the SDK's answer
	}{
		{"no token", http.MethodPost, "/mcp", "", initialize, http.StatusUnauthorized, "unauthorized"},
		{"a token that is none", http.MethodPost, "/mcp", "nwk_pat_" + strings.Repeat("x", 43), initialize, http.StatusUnauthorized, "unauthorized"},
		{"a revoked token", http.MethodPost, "/mcp", revoked, initialize, http.StatusUnauthorized, "unauthorized"},
		{"no token and a broken body", http.MethodPost, "/mcp", "", `{"jsonrpc":`, http.StatusUnauthorized, "unauthorized"},
		{"no token and a batch", http.MethodPost, atNotebook(m.hidden), "", "[" + initialize + "]", http.StatusUnauthorized, "unauthorized"},
		{"a session's access token", http.MethodPost, "/mcp", m.access, initialize, http.StatusForbidden, "forbidden"},
		{"a session's access token and a notebook not seen", http.MethodPost, atNotebook(m.hidden), m.access, initialize,
			http.StatusForbidden, "forbidden"},
		{"a notebook not seen", http.MethodPost, atNotebook(m.hidden), m.pat, initialize, http.StatusNotFound, "notebook.not_found"},
		{"a deleted notebook", http.MethodPost, atNotebook(m.deleted), m.pat, initialize, http.StatusNotFound, "notebook.not_found"},
		{"no uuid", http.MethodPost, atNotebook("nope"), m.pat, initialize, http.StatusNotFound, "notebook.not_found"},
		{"twice", http.MethodPost, atNotebook(m.notebook) + "&notebook=" + m.notebook, m.pat, initialize, http.StatusNotFound, "notebook.not_found"},
		{"empty", http.MethodPost, atNotebook(""), m.pat, initialize, http.StatusNotFound, "notebook.not_found"},
		{"an escape that does not parse", http.MethodPost, "/mcp?notebook=%zz", m.pat, initialize, http.StatusNotFound, "notebook.not_found"},
		{"a semicolon", http.MethodPost, atNotebook(m.notebook) + ";x=1", m.pat, initialize, http.StatusNotFound, "notebook.not_found"},
		{"a broken body to a notebook not seen", http.MethodPost, atNotebook(m.hidden), m.pat, `{"jsonrpc":`, http.StatusBadRequest, "bad_request"},
		{"a session's access token and a broken body", http.MethodPost, "/mcp", m.access, `{"jsonrpc":`, http.StatusBadRequest, "bad_request"},
		{"a session's GET", http.MethodGet, "/mcp", m.access, "", http.StatusForbidden, "forbidden"},
		{"GET of a notebook not seen", http.MethodGet, atNotebook(m.hidden), m.pat, "", http.StatusMethodNotAllowed, ""},
		{"DELETE", http.MethodDelete, "/mcp", m.pat, "", http.StatusMethodNotAllowed, ""},
		{"the account entry", http.MethodPost, "/mcp", m.pat, initialize, http.StatusOK, ""},
		{"the notebook entry", http.MethodPost, atNotebook(m.notebook), m.pat, initialize, http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := m.send(t, context.Background(), tt.method, tt.path, tt.token, strings.NewReader(tt.body), nil)
			switch {
			case tt.code != "":
				if code := a.problem(t, tt.status); code != tt.code {
					t.Errorf("%s %s = %d %s, want %d %s", tt.method, tt.path, a.status, a.body, tt.status, tt.code)
				}
			case a.status != tt.status:
				t.Errorf("%s %s = %d %s, want %d", tt.method, tt.path, a.status, a.body, tt.status)
			}
			if challenge := a.header.Get("WWW-Authenticate"); (tt.status == http.StatusUnauthorized) != strings.HasPrefix(challenge, "Bearer") {
				t.Errorf("%s %s: WWW-Authenticate %q, want a Bearer challenge on a 401 only", tt.method, tt.path, challenge)
			}
			if tt.status == http.StatusMethodNotAllowed && a.header.Get("Allow") != http.MethodPost {
				t.Errorf("%s %s: Allow %q, want POST", tt.method, tt.path, a.header.Get("Allow"))
			}
		})
	}
	if p := m.call(t, "/mcp", "echo", map[string]any{"notebook": m.notebook}); p.status != http.StatusOK {
		t.Errorf("a call after the refusals = %d %s", p.status, p.body)
	}

	// A body of the limit is read, the SDK's own limit no lower; a byte
	// more is 413 as it streams, with no Content-Length to refuse it by.
	ping := `{"jsonrpc":"2.0","id":3,"method":"ping"}`
	for _, tt := range []struct {
		size   int
		status int
	}{{mcpBodyBytes, http.StatusOK}, {mcpBodyBytes + 1, http.StatusRequestEntityTooLarge}} {
		body := io.MultiReader(strings.NewReader(ping), io.LimitReader(spaces{}, int64(tt.size-len(ping))))
		a := m.send(t, context.Background(), http.MethodPost, "/mcp", m.pat, body, afterHandshake())
		if a.status != tt.status {
			t.Errorf("a body of %d bytes = %d %.200s, want %d", tt.size, a.status, a.body, tt.status)
		}
		if tt.status == http.StatusRequestEntityTooLarge {
			if code := a.problem(t, tt.status); code != "payload_too_large" {
				t.Errorf("a body over the limit = %s, want payload_too_large", code)
			}
		}
	}
}

// spaces reads as JSON's whitespace without end.
type spaces struct{}

func (spaces) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

// A JSON-RPC batch is refused at the edge as bad_request, whatever
// protocol version the request says: the SDK runs one of a version before
// 2025-06-18, and refuses one of 2025-11-25 itself in plain text, so the
// problem is asserted, and echo counts that nothing ran (M9/P1 design
// review A-M8).
func TestTheMCPRefusesABatch(t *testing.T) {
	seen := newMCPSeen()
	m := startMCPApp(t, mcpTestTools(seen))
	call := `{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"echo","arguments":{}}}`
	batch := "[" + strings.Replace(call, "%d", "1", 1) + "," + strings.Replace(call, "%d", "2", 1) + "]"
	for _, version := range []string{"", "2025-03-26", mcpVersion} {
		header := http.Header{}
		if version != "" {
			header.Set("Mcp-Protocol-Version", version)
		}
		a := m.post(t, context.Background(), atNotebook(m.notebook), m.pat, batch, header)
		if a.status != http.StatusBadRequest || a.code() != "bad_request" {
			t.Errorf("a batch of version %q = %d %q %s, want the 400 problem bad_request", version, a.status, a.header.Get("Content-Type"), a.body)
		}
	}
	if n := seen.echoes.Load(); n != 0 {
		t.Errorf("echo ran %d times, want none", n)
	}
	if a := m.call(t, atNotebook(m.notebook), "echo", map[string]any{}); a.status != http.StatusOK || seen.echoes.Load() != 1 {
		t.Errorf("one call = %d %s, echo ran %d times; want it run once", a.status, a.body, seen.echoes.Load())
	}
}

// A tool's panic is its call's internal_error, logged at error with its
// stack: the process and the app go on.
func TestTheMCPRecoversAToolsPanic(t *testing.T) {
	m := startMCPApp(t, mcpTestTools(newMCPSeen()))
	r := m.call(t, atNotebook(m.notebook), "boom", map[string]any{}).result(t)
	if e := toolError(t, r); e["code"] != "internal_error" || !strings.HasPrefix(e["message"].(string), "internal_error: the server failed; request id ") {
		t.Errorf("a panic = %v, want internal_error with the request id", e)
	}
	e := m.waitLogged(t, "MCP method panicked", time.Second)
	if stack, _ := e["stack"].(string); e == nil || e["level"] != "ERROR" || e["tool"] != "boom" || !strings.Contains(stack, "mcpTestTools") {
		t.Errorf("the panic logged %v, want an error with the tool and its stack", e)
	}
	r = m.call(t, atNotebook(m.notebook), "echo", map[string]any{}).result(t)
	if sc, _ := r["structuredContent"].(map[string]any); sc["notebook"] != m.notebook {
		t.Errorf("echo after the panic = %v, want its notebook", r)
	}
}

// A call that outlives server.request_timeout (4 s in the tests) answers
// internal_error at warn, before the request's own deadline, which the
// body's read lengthens by server.read_timeout.
func TestTheMCPCallRunsOutOfTime(t *testing.T) {
	m := startMCPApp(t, mcpTestTools(newMCPSeen()))
	a := m.call(t, atNotebook(m.notebook), "slow", map[string]any{})
	e := toolError(t, a.result(t))
	if !strings.HasPrefix(e["message"].(string), "internal_error: the call ran out of time; request id ") {
		t.Errorf("a call out of time = %v, want internal_error: the call ran out of time", e)
	}
	if a.took < 4*time.Second || a.took > 6*time.Second {
		t.Errorf("the call answered after %v, want at server.request_timeout's 4 s", a.took)
	}
	if line := m.waitLogged(t, "MCP call deadline exceeded", time.Second); line == nil || line["level"] != "WARN" {
		t.Errorf("the log has %v, want the deadline at warn", line)
	}
}

// A client that goes away ends its call long before the call's deadline:
// the request's end cancels it (AfterFunc), the client gone at debug, the
// deadline's warn never written, and no line at error: the answer never
// written is the client's, not a fault (M9/P1 design review A-M12, A N-2).
func TestTheMCPCallStopsWhenTheClientGoes(t *testing.T) {
	seen := newMCPSeen()
	m := startMCPApp(t, mcpTestTools(seen))
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	m.post(t, ctx, atNotebook(m.notebook), m.pat, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"slow","arguments":{}}}`, afterHandshake())
	select {
	case at := <-seen.slowEnded:
		if took := at.Sub(start); took > time.Second {
			t.Errorf("the call ended %v after it began, want within 1 s of the client going at 300 ms", took)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("the call did not end within 6 s")
	}
	if e := m.waitLogged(t, "MCP client went away", 2*time.Second); e == nil || e["level"] != "DEBUG" {
		t.Errorf("the log has %v, want the client gone at debug", e)
	}
	if e := m.waitLogged(t, "client went away", 2*time.Second); e == nil || e["level"] != "DEBUG" {
		t.Errorf("the log has %v, want the answer never written at debug", e)
	}
	if e := m.logged(t, "MCP call deadline exceeded"); e != nil {
		t.Errorf("the log has %v: the call ran to its deadline", e)
	}
	if errs := m.loggedAt(t, "ERROR"); len(errs) != 0 {
		t.Errorf("a client that went away logged errors %v", errs)
	}
}

// Every answer of the MCP routes is no-store: the SDK's result, its
// notification's 202 and 405, the edge's problems, the 404s below /mcp/
// and /.well-known/, and OAuth's registration's error.
func TestEveryMCPAnswerIsNotStored(t *testing.T) {
	m := startMCPApp(t, mcpTestTools(newMCPSeen()))
	notification := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	tests := []struct {
		name, method, path, token, body string
		status                          int
	}{
		{"a result", http.MethodPost, atNotebook(m.notebook), m.pat,
			`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"echo","arguments":{}}}`, http.StatusOK},
		{"a notification", http.MethodPost, "/mcp", m.pat, notification, http.StatusAccepted},
		{"a GET", http.MethodGet, "/mcp", m.pat, "", http.StatusMethodNotAllowed},
		{"no token", http.MethodPost, "/mcp", "", notification, http.StatusUnauthorized},
		{"a session's access token", http.MethodPost, "/mcp", m.access, notification, http.StatusForbidden},
		{"a notebook not seen", http.MethodPost, atNotebook(m.hidden), m.pat, notification, http.StatusNotFound},
		{"OAuth's metadata", http.MethodGet, "/.well-known/oauth-protected-resource", "", "", http.StatusNotFound},
		{"below /mcp/", http.MethodGet, "/mcp/x", "", "", http.StatusNotFound},
		{"OAuth's registration", http.MethodPost, "/register", "", "{}", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := m.send(t, context.Background(), tt.method, tt.path, tt.token, strings.NewReader(tt.body), afterHandshake())
			if a.status != tt.status || a.header.Get("Cache-Control") != "no-store" {
				t.Errorf("%s %s = %d Cache-Control %q, want %d no-store", tt.method, tt.path, a.status, a.header.Get("Cache-Control"), tt.status)
			}
		})
	}
}

// The official Go client on its default options negotiates the latest
// version the SDK speaks; a handshake of 2025-06-18 keeps it; a request of
// a version the SDK does not speak is the SDK's 400. The capabilities are
// the tools alone, unchanging: no logging, no prompts.
func TestTheMCPNegotiatesItsVersion(t *testing.T) {
	m := startMCPApp(t, mcpTestTools(newMCPSeen()))
	cs := mcpClient(t, m.base+atNotebook(m.notebook), m.pat)
	hello := cs.InitializeResult()
	if hello.ProtocolVersion != mcpVersion {
		t.Errorf("the Go client negotiated %q, want %s", hello.ProtocolVersion, mcpVersion)
	}
	if caps, err := json.Marshal(hello.Capabilities); err != nil || string(caps) != `{"tools":{}}` {
		t.Errorf("the capabilities = %s, want the tools alone, unchanging", caps)
	}
	r := m.post(t, context.Background(), "/mcp", m.pat, initializeBody("2025-06-18"), nil).result(t)
	if r["protocolVersion"] != "2025-06-18" {
		t.Errorf("a handshake of 2025-06-18 = %v, want it kept", r["protocolVersion"])
	}
	a := m.post(t, context.Background(), atNotebook(m.notebook), m.pat,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"echo","arguments":{}}}`,
		http.Header{"Mcp-Protocol-Version": {"2026-07-28"}})
	if a.status != http.StatusBadRequest {
		t.Errorf("a call of 2026-07-28 = %d %s, want the SDK's 400", a.status, a.body)
	}
}

// The listen of the protocol before 2025-11-25, a real subscription to
// the lists' changes, ends within its request: the lists never change
// (ListChanged false), so nothing holds it to the call's deadline (M9/P1
// design review A-M9).
func TestTheMCPListenEndsAtOnce(t *testing.T) {
	m := startMCPApp(t, mcpTestTools(newMCPSeen()))
	a := m.post(t, context.Background(), "/mcp", m.pat,
		`{"jsonrpc":"2.0","id":6,"method":"subscriptions/listen","params":{"notifications":{"toolsListChanged":true,"promptsListChanged":true}}}`,
		afterHandshake())
	if a.status != http.StatusOK || !strings.HasPrefix(a.header.Get("Content-Type"), "text/event-stream") {
		t.Errorf("the listen = %d %q %s, want 200 text/event-stream", a.status, a.header.Get("Content-Type"), a.body)
	}
	if a.took > time.Second {
		t.Errorf("the listen ended after %v, want within 1 s", a.took)
	}
}

// No OAuth: the paths a client looks for its metadata at, and those
// below /mcp/, are 404 problems, never the web app's page, which a client
// would read as metadata.
func TestTheMCPHasNoOAuthMetadata(t *testing.T) {
	m := startMCPApp(t, nil)
	for _, path := range []string{
		"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp",
		"/.well-known/oauth-authorization-server", "/.well-known/openid-configuration", "/.well-known/x/y", "/mcp/x",
	} {
		a := m.send(t, context.Background(), http.MethodGet, path, "", http.NoBody, nil)
		if code := a.problem(t, http.StatusNotFound); code != "not_found" || bytes.Contains(a.body, []byte("<title>")) {
			t.Errorf("GET %s = %d %s, want the 404 problem", path, a.status, a.body)
		}
	}
}

// OAuth's registration at the origin, where an OAuth client turns after
// the 404s under /.well-known/, answers OAuth's error, whose description
// the client shows as it is: the 404s' sentence, without a token, never
// stored. Its other methods are a path's nobody serves: the web app's.
func TestTheMCPRefusesOAuthRegistration(t *testing.T) {
	m := startMCPApp(t, nil)
	hint, _ := m.send(t, context.Background(), http.MethodGet, "/.well-known/oauth-authorization-server", "", http.NoBody, nil).decoded(t)["detail"].(string)
	if !strings.Contains(hint, "personal access token") {
		t.Fatalf("the 404's detail = %q, want the hint", hint)
	}
	a := m.send(t, context.Background(), http.MethodPost, "/register", "",
		strings.NewReader(`{"client_name":"whole-program","redirect_uris":["http://127.0.0.1:9/callback"],"token_endpoint_auth_method":"none"}`), nil)
	want := `{"error":"invalid_client_metadata","error_description":"` + hint + `"}` + "\n"
	if a.status != http.StatusBadRequest || a.header.Get("Content-Type") != "application/json" ||
		a.header.Get("Cache-Control") != "no-store" || string(a.body) != want {
		t.Errorf("POST /register = %d %q %q %q, want 400 application/json no-store %q",
			a.status, a.header.Get("Content-Type"), a.header.Get("Cache-Control"), a.body, want)
	}
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		got := m.send(t, context.Background(), method, "/register", "", http.NoBody, nil)
		other := m.send(t, context.Background(), method, "/no-such-page", "", http.NoBody, nil)
		if got.status != other.status || got.header.Get("Content-Type") != other.header.Get("Content-Type") {
			t.Errorf("%s /register = %d %q, want what %s of a path nobody serves answers: %d %q",
				method, got.status, got.header.Get("Content-Type"), method, other.status, other.header.Get("Content-Type"))
		}
	}
}

// The transport is stateless: a client whose server restarts behind the
// same address calls on, without a handshake of its own.
func TestTheMCPOutlivesARestart(t *testing.T) {
	m := startMCPApp(t, mcpTestTools(newMCPSeen()))
	var target atomic.Pointer[url.URL]
	first, err := url.Parse(m.base)
	if err != nil {
		t.Fatal(err)
	}
	target.Store(first)
	proxy := httptest.NewServer(&httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(target.Load()) }})
	t.Cleanup(proxy.Close)
	cs := mcpClient(t, proxy.URL+atNotebook(m.notebook), m.pat)
	echo := func(s string) {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"s": s}})
		if err != nil {
			t.Fatalf("echo %s: %v", s, err)
		}
		if sc, _ := res.StructuredContent.(map[string]any); res.IsError || sc["s"] != s {
			t.Fatalf("echo %s = %+v", s, res)
		}
	}
	echo("before")

	logs := &syncBuffer{}
	second := wireMCPApp(t, testConfig(t, m.url, false), logs, mcpTestTools(newMCPSeen()))
	next, err := url.Parse(runApp(t, second))
	if err != nil {
		t.Fatal(err)
	}
	target.Store(next)
	echo("after")
	if strings.Contains(logs.String(), `"msg":"MCP initialize"`) || !strings.Contains(logs.String(), `"msg":"MCP tool call"`) {
		t.Errorf("the second server's log, want a call and no handshake:\n%s", logs.String())
	}
}

// The notebook entry's tool is given the address's notebook; the account
// entry's takes its notebook as a required argument.
func TestTheMCPEntriesGiveTheNotebook(t *testing.T) {
	m := startMCPApp(t, mcpTestTools(newMCPSeen()))
	call := func(endpoint string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := mcpClient(t, m.base+endpoint, m.pat).CallTool(context.Background(), &mcp.CallToolParams{Name: "echo", Arguments: args})
		if err != nil {
			t.Fatalf("echo at %s: %v", endpoint, err)
		}
		return res
	}
	for _, tt := range []struct {
		endpoint string
		args     map[string]any
		entry    string
	}{
		{atNotebook(m.notebook), map[string]any{}, "notebook"},
		{"/mcp", map[string]any{"notebook": m.notebook}, "account"},
	} {
		sc, _ := call(tt.endpoint, tt.args).StructuredContent.(map[string]any)
		if sc["entry"] != tt.entry || sc["notebook"] != m.notebook || sc["client"] != mcpserver.UnknownClient {
			t.Errorf("echo at %s = %v, want the %s entry's %s", tt.endpoint, sc, tt.entry, m.notebook)
		}
	}
	res := call("/mcp", map[string]any{})
	sc, _ := res.StructuredContent.(map[string]any)
	e, _ := sc["error"].(map[string]any)
	if fields, _ := e["fields"].([]any); !res.IsError || e["code"] != "bad_request" || len(fields) != 1 || fields[0].(map[string]any)["field"] != "notebook" {
		t.Errorf("echo at the account entry without a notebook = %+v, want bad_request at notebook", sc)
	}
}
