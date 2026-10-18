package bootstrap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// The large request bodies' slots over the whole program (M9/P2 design
// 3.2): /mcp's calls past 64 KiB and REST's content past
// server.max_body_bytes share the server's two, each credential holding at
// most one, through the seam tool hold, which holds its call until the
// test lets it go.

// largeWait is how long the large bodies wait for a slot in these tests
// (wiring.largeBodyWait): the composition root's 2 seconds is
// TestTheAPIConfigBoundsTheLargeBodies'.
const largeWait = 250 * time.Millisecond

// holding is what the seam tool hold tells its test: each call that came
// in, by the key its s starts with, and the gates the test opens to let
// each go.
type holding struct {
	mu    sync.Mutex
	came  map[string]chan struct{}
	gates map[string]chan struct{}
}

func newHolding() *holding {
	return &holding{came: map[string]chan struct{}{}, gates: map[string]chan struct{}{}}
}

// of is key's channel of chans, made on its first use.
func (h *holding) of(chans map[string]chan struct{}, key string) chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := chans[key]
	if !ok {
		c = make(chan struct{})
		chans[key] = c
	}
	return c
}

// waitIn waits for the call of key to come in: it then holds its slot.
func (h *holding) waitIn(t *testing.T, key string) {
	t.Helper()
	select {
	case <-h.of(h.came, key):
	case <-time.After(10 * time.Second):
		t.Fatalf("hold's call of %s never came in", key)
	}
}

// let lets the call of key go.
func (h *holding) let(key string) { close(h.of(h.gates, key)) }

// holdArgs are hold's arguments: s, its key before a ":", then anything.
type holdArgs struct {
	S string `json:"s" jsonschema:"The call's key, a colon, then anything."`
}

// holdTool is the seam tool hold, in the account entry: it tells the test
// its call came in, so that a call past the threshold holds a slot, and
// waits for the test to let it go, or for its context to end. A call let
// go before it came answers at once; its answer is its key alone, however
// long its s.
func holdTool(h *holding) mcpserver.Tool {
	return mcpserver.NewTool(mcpserver.Spec{Name: "hold", Title: "Hold", Description: "Holds its call until the test lets it go.",
		Entry: mcpserver.AccountEntry},
		func(ctx context.Context, _ mcpserver.Call, in holdArgs) (mcpserver.Result, error) {
			key, _, _ := strings.Cut(in.S, ":")
			close(h.of(h.came, key))
			select {
			case <-h.of(h.gates, key):
				return mcpserver.Result{Structured: map[string]any{"held": key}}, nil
			case <-ctx.Done():
				return mcpserver.Result{}, ctx.Err()
			}
		})
}

// startHoldingApp is startMCPApp's app with the test's tools and hold,
// its large bodies waiting largeWait for a slot.
func startHoldingApp(t *testing.T, h *holding, tools ...mcpserver.Tool) *mcpApp {
	t.Helper()
	return startMCPAppWith(t, wiring{mcpTools: append(tools, holdTool(h)), largeBodyWait: largeWait})
}

// keyed is a tool's s of key and n bytes besides.
func keyed(key string, n int) map[string]any {
	return map[string]any{"s": key + ":" + strings.Repeat("x", n)}
}

// calling is a tools/call of tool with args to the account entry with
// token, sent on a goroutine of its own and its answer on the channel,
// its length declared: ctx ends it, an answer cut short of the status -1.
func (m *mcpApp) calling(t *testing.T, ctx context.Context, token, tool string, args map[string]any) <-chan mcpAnswer {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan mcpAnswer, 1)
	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.base+mcpserver.Pattern, bytes.NewReader(body))
		if err != nil {
			t.Error(err)
			done <- mcpAnswer{status: -1}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Mcp-Protocol-Version", mcpVersion)
		res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			if ctx.Err() == nil {
				t.Errorf("%s as %.12s: %v", tool, token, err)
			}
			done <- mcpAnswer{status: -1}
			return
		}
		defer func() { _ = res.Body.Close() }()
		b, err := io.ReadAll(res.Body)
		if err != nil && ctx.Err() == nil {
			t.Errorf("%s as %.12s: reading the answer: %v", tool, token, err)
		}
		done <- mcpAnswer{status: res.StatusCode, header: res.Header, body: b, tool: tool}
	}()
	return done
}

// isBusy reports whether a is the large bodies' 503: problem+json
// server_busy, Retry-After 1, not stored.
func isBusy(a mcpAnswer) bool {
	return a.status == http.StatusServiceUnavailable && a.header.Get("Content-Type") == "application/problem+json" &&
		a.code() == "server_busy" && a.header.Get("Retry-After") == "1" && a.header.Get("Cache-Control") == "no-store"
}

// resulted reports whether a is a tool's result that is no error.
func resulted(t *testing.T, a mcpAnswer) bool {
	t.Helper()
	if a.status != http.StatusOK {
		return false
	}
	r, _ := a.decoded(t)["result"].(map[string]any)
	return r != nil && r["isError"] != true
}

// saturatedOf is the saturated line of credential, the last, or nil.
func (m *mcpApp) saturatedOf(t *testing.T, credential string) map[string]any {
	t.Helper()
	var found map[string]any
	for _, e := range m.logLines(t) {
		if e["msg"] == "large request bodies are saturated" && e["credential"] == credential {
			found = e
		}
	}
	return found
}

// Two large bodies at once, whosever, one a credential (M9/P2 design
// 3.2.5): alice's three personal access tokens are three credentials. P1's
// and P2's holds take the two slots; P3's is turned away after the wait,
// and so is its REST content past server.max_body_bytes, a line naming
// who holds them; what is under the thresholds is served. P1's goes: P2's
// second waits for its own and is turned away, the server's free slot
// left for P3's, which takes it. P2's client goes: its slot comes back to
// P1's next. The answers alone tell, never a time.
func TestTheLargeBodiesShareTwoSlots(t *testing.T) {
	h := newHolding()
	m := startHoldingApp(t, h)
	var ids, pats [3]string
	for i := range pats {
		ids[i], pats[i] = createTokenWithID(t, m.contract, m.base, m.access)
	}
	const large, small = 70 << 10, 60 << 10
	ctx := context.Background()
	passing := func(token, key string, n int) mcpAnswer { // a hold let go before it comes
		t.Helper()
		h.let(key)
		return <-m.calling(t, ctx, token, "hold", keyed(key, n))
	}
	p1 := m.calling(t, ctx, pats[0], "hold", keyed("p1", large))
	p2ctx, leave := context.WithCancel(ctx)
	defer leave()
	p2 := m.calling(t, p2ctx, pats[1], "hold", keyed("p2", large))
	h.waitIn(t, "p1")
	h.waitIn(t, "p2")

	if a := <-m.calling(t, ctx, pats[2], "hold", keyed("p3-away", large)); !isBusy(a) {
		t.Errorf("P3's large call with both slots taken = %d %v %s, want 503 server_busy", a.status, a.header, a.body)
	}
	line := m.saturatedOf(t, "pat:"+ids[2])
	holders, _ := json.Marshal(line["holders"])
	for _, id := range ids[:2] {
		if !strings.Contains(string(holders), `{"credential":"pat:`+id+`","route":"`+mcpserver.Pattern+`"}`) {
			t.Errorf("the saturated line %v does not name pat:%s at %s", line, id, mcpserver.Pattern)
		}
	}
	if line == nil || line["own"] != false || line["route"] != mcpserver.Pattern {
		t.Errorf("the saturated line of P3 = %v, want P3's at /mcp, not its own slot", line)
	}

	page := idOf(t, answer{body: mustAsk(t, m, http.MethodPost, "/api/v0/notebooks/"+m.notebook+"/pages", m.access,
		`{"parent_id":null,"title":"Bodies"}`, http.StatusCreated)})
	content := func(n, base int) string {
		b, _ := json.Marshal(map[string]any{"content": strings.Repeat("y", n), "base_revision": base})
		return string(b)
	}
	status, got := ask(t, m.contract, http.MethodPut, m.base+"/api/v0/pages/"+page+"/content", pats[2], content(1100<<10, 1))
	if status != http.StatusServiceUnavailable || problemCode(t, got) != "server_busy" {
		t.Errorf("P3's REST content of 1.1 MiB = %d %s, want 503 server_busy", status, got)
	}
	if status, got := ask(t, m.contract, http.MethodPut, m.base+"/api/v0/pages/"+page+"/content", pats[2], content(100<<10, 1)); status != http.StatusOK {
		t.Errorf("P3's REST content of 100 KiB = %d %.200s, want 200", status, got)
	}
	if a := passing(pats[2], "p3-small", small); !resulted(t, a) {
		t.Errorf("P3's call of 60 KiB = %d %.200s, want its result", a.status, a.body)
	}

	h.let("p1")
	if a := <-p1; !resulted(t, a) {
		t.Errorf("P1's hold, let go = %d %.200s, want its result", a.status, a.body)
	}
	if a := passing(pats[1], "p2-second", large); !isBusy(a) {
		t.Errorf("P2's second large call = %d %.200s, want 503: it waits for its own", a.status, a.body)
	}
	if line := m.saturatedOf(t, "pat:"+ids[1]); line == nil || line["own"] != true {
		t.Errorf("the saturated line of P2's second = %v, want its own slot", line)
	}
	p3 := m.calling(t, ctx, pats[2], "hold", keyed("p3", large))
	h.waitIn(t, "p3")

	leave()
	if a := <-p2; a.status != -1 {
		t.Errorf("P2's hold, its client gone = %d %s", a.status, a.body)
	}
	if a := passing(pats[0], "p1-again", large); !resulted(t, a) {
		t.Errorf("P1's large call once P2's client went = %d %.200s, want its result", a.status, a.body)
	}
	h.let("p3")
	if a := <-p3; !resulted(t, a) {
		t.Errorf("P3's hold, let go = %d %.200s, want its result", a.status, a.body)
	}
}

// mustAsk is ask of m's app, the answer's status want, its body.
func mustAsk(t *testing.T, m *mcpApp, method, path, token, body string, want int) string {
	t.Helper()
	status, got := ask(t, m.contract, method, m.base+path, token, body)
	if status != want {
		t.Fatalf("%s %s = %d %s, want %d", method, path, status, got, want)
	}
	return got
}

// declaredOnly sends a POST to /mcp that declares length and sends none of
// it, over a connection of its own, and reads the answer: the slots'
// refusal comes before the body is read, so none is needed.
func (m *mcpApp) declaredOnly(t *testing.T, token string, length int64) *http.Response {
	t.Helper()
	u, err := url.Parse(m.base)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", u.Host, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	head := "POST " + mcpserver.Pattern + " HTTP/1.1\r\nHost: " + u.Host + "\r\nContent-Type: application/json\r\n" +
		"Accept: application/json, text/event-stream\r\nAuthorization: Bearer " + token + "\r\n" +
		"Mcp-Protocol-Version: " + mcpVersion + "\r\nContent-Length: " + strconv.FormatInt(length, 10) + "\r\n\r\n"
	if _, err := io.WriteString(conn, head); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}
