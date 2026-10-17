package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},` +
	`"clientInfo":{"name":"test-client","version":"1"}}}`

// The entry of a POST: the account entry without notebook; the notebook
// entry with one that uuid.Parse takes, its id logged as uuids are
// written; any other query is notebook.not_found from the gate on the
// zero id, without a read: a query that does not parse never falls to the
// account entry.
func TestTheEntryOfAMalformedQueryIsNotFound(t *testing.T) {
	s := serveMCP(t, setup{})
	id := visible.String()
	tests := []struct {
		name, query string
		entry       string // "" for notebook.not_found
		reads       int32
	}{
		{"no notebook", "", "account", 0},
		{"another parameter", "?client=x", "account", 0},
		{"the notebook", "?notebook=" + id, "notebook", 1},
		{"its capitals", "?notebook=" + strings.ToUpper(id), "notebook", 1},
		{"in braces", "?notebook=%7B" + id + "%7D", "notebook", 1},
		{"as a URN", "?notebook=urn:uuid:" + id, "notebook", 1},
		{"a notebook alice cannot see", "?notebook=" + uuid.NewV7().String(), "", 1},
		{"twice", "?notebook=" + id + "&notebook=" + id, "", 0},
		{"empty", "?notebook=", "", 0},
		{"no uuid", "?notebook=nope", "", 0},
		{"an escape that does not parse", "?notebook=%zz", "", 0},
		{"a bad pair beside the notebook", "?notebook=" + id + "&x=%zz", "", 0},
		{"a semicolon", "?notebook=" + id + ";x=1", "", 0},
		{"a semicolon elsewhere", "?a=1;b=2", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reads := s.notebooks.reads.Load()
			a := s.post(t, context.Background(), Pattern+tt.query, patToken, initialize, nil)
			if got := s.notebooks.reads.Load() - reads; got != tt.reads {
				t.Errorf("the gate read %d notebooks, want %d", got, tt.reads)
			}
			if tt.entry == "" {
				if p := a.decoded(t); a.status != http.StatusNotFound || p["code"] != "notebook.not_found" {
					t.Errorf("POST %s = %d %s, want 404 notebook.not_found", tt.query, a.status, a.body)
				}
				return
			}
			result(t, a)
		})
	}
	// The log writes the notebook as uuids are written, whatever the query.
	var logged []string
	for _, e := range s.logs.entries(t) {
		if e["msg"] == "MCP initialize" && e["entry"] == "notebook" {
			logged = append(logged, e["notebook_id"].(string))
		}
	}
	if !slices.Equal(logged, []string{id, id, id, id}) {
		t.Errorf("the handshakes logged the notebooks %q, want %s four times", logged, id)
	}
}

// Only a POST is gated: GET and DELETE are the SDK's 405 without a read,
// a version it does not know its 400 before that (M9/P1 design 3.2.2),
// and a session's access token is 403 before any notebook is looked at.
func TestTheEntryGatesPostsOnly(t *testing.T) {
	s := serveMCP(t, setup{})
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		a := s.send(t, context.Background(), method, Pattern+"?notebook="+uuid.NewV7().String(), patToken, "", nil)
		if a.status != http.StatusMethodNotAllowed || a.header.Get("Allow") != "POST" {
			t.Errorf("%s = %d Allow %q, want 405 Allow POST", method, a.status, a.header.Get("Allow"))
		}
	}
	a := s.send(t, context.Background(), http.MethodGet, Pattern, patToken, "", http.Header{"Mcp-Protocol-Version": {"2024-01-01"}})
	if a.status != http.StatusBadRequest || !strings.HasPrefix(a.header.Get("Content-Type"), "text/plain") {
		t.Errorf("a GET of an unknown version = %d %s, want the SDK's 400 before its 405", a.status, a.body)
	}
	a = s.post(t, context.Background(), Pattern+"?notebook="+uuid.NewV7().String(), sessionToken, initialize, nil)
	if p := a.decoded(t); a.status != http.StatusForbidden || p["code"] != "forbidden" ||
		p["detail"] != "MCP takes a personal access token, not a session's access token." {
		t.Errorf("a session's access token = %d %s, want 403 forbidden", a.status, a.body)
	}
	if n := s.notebooks.gates.Load(); n != 0 {
		t.Errorf("the gate ran %d times, want none", n)
	}
}

// A request without a credential reached the server unauthenticated, a
// fault of its mount: 500 at error, never the 403 of a session's token.
func TestARequestWithoutACredentialIsAFault(t *testing.T) {
	s := serveMCP(t, setup{options: func(o *Options) {
		o.Credentials = func(context.Context) (Credential, bool) { return Credential{}, false }
	}})
	a := s.post(t, context.Background(), Pattern, patToken, initialize, nil)
	if p := a.decoded(t); a.status != http.StatusInternalServerError || p["code"] != "internal_error" {
		t.Errorf("no credential = %d %s, want 500 internal_error", a.status, a.body)
	}
	if errs := s.logs.at(t, "ERROR"); len(errs) != 1 || !strings.Contains(fmt.Sprint(errs[0]["error"]), "no credential") {
		t.Errorf("the log has %v at error, want the missing credential", errs)
	}
}

// pingOf is a ping of n bytes, n at least 40: spaces after the object.
func pingOf(n int) string {
	const ping = `{"jsonrpc":"2.0","id":3,"method":"ping"}`
	return ping + strings.Repeat(" ", n-len(ping))
}

// callOf is a tools/call of the tool named name, n bytes long, its
// arguments the account entry's notebook and spaces.
func callOf(name string, n int) string {
	call := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":` + strconv.Quote(name) +
		`,"arguments":{"notebook":"` + visible.String() + `"}}}`
	return call + strings.Repeat(" ", n-len(call))
}

// Only a tools/call of a tool its entry serves may be as long as the
// route's limit: any other body over 64 KiB is 413, as the route's limit
// answers, before the 403 of a session's access token as that limit is;
// method and name are read by their exact names, as the SDK reads them
// (M9/P1 S1 review A-M1), and so are the jsonrpc, the id and the _meta
// that the SDK's errors repeat (fix-check A F1, F2); the entry is the one
// the query asks for. The bound's rows are absolute: a bound of twice
// 64 KiB fails them (fix-check A, e-bound-128k).
func TestOnlyACallMayBeLong(t *testing.T) {
	none := func(context.Context, Call, struct{}) (Result, error) {
		return Result{Structured: map[string]any{}}, nil
	}
	s := serveMCP(t, setup{options: func(o *Options) {
		o.Tools = append(o.Tools, NewTool(Spec{Name: "nb_only", Entry: NotebookEntry}, none), NewTool(Spec{Name: "acct_only", Entry: AccountEntry}, none))
	}})
	nb := Pattern + "?notebook=" + visible.String()
	over := maxOtherBytes + 1
	long := callOf("echo", over+1024) // over the bound after any row's change
	tests := []struct {
		name, path, token, body string
		status                  int
	}{
		{"a ping of 64 KiB", Pattern, patToken, pingOf(64 << 10), http.StatusOK},
		{"a ping of 64 KiB and a byte", Pattern, patToken, pingOf(64<<10 + 1), http.StatusRequestEntityTooLarge},
		{"a handshake over the bound", Pattern, patToken, initialize + strings.Repeat(" ", over), http.StatusRequestEntityTooLarge},
		{"a call at the route's limit", Pattern, patToken, callOf("echo", bodyLimit), http.StatusOK},
		{"a call of no tool's name", Pattern, patToken, callOf("Echo", over), http.StatusRequestEntityTooLarge},
		{"a call of a name too long", Pattern, patToken, callOf(strings.Repeat("e", 33), over), http.StatusRequestEntityTooLarge},
		{"a call of a tool not served", Pattern, patToken, callOf("missing", over), http.StatusRequestEntityTooLarge},
		{"the notebook entry's tool at the notebook entry", nb, patToken, callOf("nb_only", over), http.StatusOK},
		{"the notebook entry's tool at the account entry", Pattern, patToken, callOf("nb_only", over), http.StatusRequestEntityTooLarge},
		{"the account entry's tool at the account entry", Pattern, patToken, callOf("acct_only", over), http.StatusOK},
		{"the account entry's tool at the notebook entry", nb, patToken, callOf("acct_only", over), http.StatusRequestEntityTooLarge},
		{"the account entry's tool and a parameter of the client's own", Pattern + "?x=1", patToken, callOf("acct_only", over), http.StatusOK},
		{"the account entry's tool and a query that does not parse", Pattern + "?notebook=%zz", patToken, callOf("acct_only", over),
			http.StatusRequestEntityTooLarge},
		{"the notebook entry's tool and a query that does not parse", Pattern + "?notebook=%zz", patToken, callOf("nb_only", over),
			http.StatusNotFound},
		{"a method in another case, after it", Pattern, patToken, strings.Replace(pingOf(over), `"method":"ping"`,
			`"method":"ping","METHOD":"tools/call","params":{"name":"echo"}`, 1), http.StatusRequestEntityTooLarge},
		{"a name in another case, after it", Pattern, patToken, strings.Replace(callOf("Echo", over), `"name":"Echo"`, `"name":"Echo","NAME":"echo"`, 1),
			http.StatusRequestEntityTooLarge},
		{"a jsonrpc of U+0080", Pattern, patToken, strings.Replace(long, `"jsonrpc":"2.0"`, `"jsonrpc":"`+string(rune(0x80))+`"`, 1),
			http.StatusRequestEntityTooLarge},
		{"no jsonrpc", Pattern, patToken, strings.Replace(long, `"jsonrpc":"2.0",`, "", 1), http.StatusRequestEntityTooLarge},
		{"an id of 256 bytes", Pattern, patToken, strings.Replace(long, `"id":3`, `"id":"`+strings.Repeat("i", 254)+`"`, 1), http.StatusOK},
		{"an id of 257 bytes, of U+2028", Pattern, patToken, strings.Replace(long, `"id":3`, `"id":"`+strings.Repeat(string(rune(0x2028)), 85)+`"`, 1),
			http.StatusRequestEntityTooLarge},
		{"a _meta a float64 holds", Pattern, patToken, strings.Replace(long, `"name":"echo"`, `"name":"echo","_meta":{"x":{"y":1e300}}`, 1), http.StatusOK},
		{"a _meta of a number no float64 holds", Pattern, patToken, strings.Replace(long, `"name":"echo"`, `"name":"echo","_meta":{"x":{"y":1e400}}`, 1),
			http.StatusRequestEntityTooLarge},
		{"a session's access token and a ping over the bound", Pattern, sessionToken, pingOf(over), http.StatusRequestEntityTooLarge},
		{"a session's access token and a long call", Pattern, sessionToken, callOf("echo", over), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := s.post(t, context.Background(), tt.path, tt.token, tt.body, v2025())
			if a.status != tt.status {
				t.Fatalf("a body of %d bytes = %d %.300s, want %d", len(tt.body), a.status, a.body, tt.status)
			}
			p := a.decoded(t)
			switch {
			case tt.status == http.StatusOK && p["result"] == nil:
				t.Errorf("a body of %d bytes = %.300s, want the SDK's result", len(tt.body), a.body)
			case tt.status == http.StatusRequestEntityTooLarge && p["code"] != "payload_too_large":
				t.Errorf("a body over its bound = %s, want payload_too_large", a.body)
			case tt.status == http.StatusNotFound && p["code"] != "notebook.not_found":
				t.Errorf("a body to a query that does not parse = %s, want notebook.not_found", a.body)
			}
		})
	}
}

// The params the SDK reads into a request's are shaped before it: an
// object or null; a string name, objects or null for its arguments and
// its _meta, no inputResponses or requestState; every other member, and
// the message's own, unchecked. A tools/call can then not fail the SDK's
// reading, whose error would repeat it, escaped (M9/P1 S1 review A-M1).
func TestTheParamsAreShapedBeforeTheSDK(t *testing.T) {
	s := serveMCP(t, setup{})
	nb := Pattern + "?notebook=" + visible.String()
	tests := []struct {
		name, params string
		field        string // the 400's, "" for the SDK's answer
	}{
		{"a string", `"TOP"`, "params"},
		{"an array", `[]`, "params"},
		{"null", `null`, ""},
		{"a name that is no string", `{"name":1}`, "params.name"},
		{"a name that is null", `{"name":null}`, "params.name"},
		{"arguments that are no object", `{"name":"echo","arguments":"TOP"}`, "params.arguments"},
		{"arguments that are null", `{"name":"echo","arguments":null}`, ""},
		{"a _meta that is no object", `{"name":"echo","_meta":[1]}`, "params._meta"},
		{"a _meta that is null", `{"name":"echo","_meta":null}`, ""},
		{"inputResponses", `{"name":"echo","inputResponses":{}}`, "params.inputResponses"},
		{"a requestState", `{"name":"echo","requestState":"x"}`, "params.requestState"},
		{"members of their own", `{"name":"echo","cursor":1,"task":{"ttl":1}}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := s.post(t, context.Background(), nb, patToken, `{"jsonrpc":"2.0","id":2,"method":"tools/call","x":1,"params":`+tt.params+`}`, v2025())
			if tt.field == "" {
				if a.status != http.StatusOK {
					t.Errorf("params %s = %d %s, want the SDK's answer", tt.params, a.status, a.body)
				}
				return
			}
			p := a.decoded(t)
			fields, _ := p["errors"].([]any)
			if a.status != http.StatusBadRequest || p["code"] != "bad_request" || len(fields) != 1 || fields[0].(map[string]any)["field"] != tt.field {
				t.Errorf("params %s = %d %s, want 400 bad_request at %s", tt.params, a.status, a.body, tt.field)
			}
		})
	}
}

// A reverse proxy on the same host forwards its own Host to the loopback
// listener: it is served, the SDK's protection of localhost being off
// (M9/P1 design 3.2.2); with it on, the SDK would answer 403 "invalid Host
// header" to every request of such a deployment.
func TestAProxysHostIsServed(t *testing.T) {
	s := serveMCP(t, setup{})
	for _, path := range []string{Pattern, Pattern + "?notebook=" + visible.String()} {
		a := s.post(t, context.Background(), path, patToken, initialize, http.Header{"Host": {"wiki.example.com"}})
		if r := result(t, a); r["protocolVersion"] != "2025-11-25" {
			t.Errorf("POST %s with Host wiki.example.com = %v, want its handshake", path, r)
		}
	}
}

// deadlineIn reports the call context's deadline, from the moment the
// handler ran.
type deadlineSeen struct {
	left time.Duration
	ok   bool
}

// Every method's context, the handshake's among them, has the earlier of
// the request's deadline and the body's read plus request_timeout; the
// SDK's own has none (M9/P1 design 3.4.1).
func TestTheCallContextTakesTheEarlierDeadline(t *testing.T) {
	tests := []struct {
		name  string
		setup setup
		want  time.Duration // about
	}{
		{"request_timeout first", setup{options: func(o *Options) { o.RequestTimeout = 2 * time.Second }}, 2 * time.Second},
		{"the request's deadline first", setup{noBodyLimit: true, requestTimeout: 3 * time.Second}, 3 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(chan deadlineSeen, 8)
			sdk := make(chan bool, 8)
			report := toolOf("deadline", func(ctx context.Context, _ Call) (Result, error) {
				d, ok := ctx.Deadline()
				seen <- deadlineSeen{time.Until(d), ok}
				return Result{Structured: map[string]any{}}, nil
			})
			change := tt.setup.options
			tt.setup.options = func(o *Options) {
				if change != nil {
					change(o)
				}
				o.Tools = append(o.Tools, report)
			}
			s := serveMCP(t, tt.setup)
			// Outermost, outside outer: what the SDK hands every method.
			s.srv.accountEntry.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					_, ok := ctx.Deadline()
					sdk <- ok
					return next(ctx, method, req)
				}
			})
			result(t, s.call(t, Pattern, "deadline", nil))
			got := <-seen
			if !got.ok || got.left > tt.want || got.left < tt.want-time.Second {
				t.Errorf("the handler's deadline is %v away (%v), want about %v", got.left, got.ok, tt.want)
			}
			if <-sdk {
				t.Errorf("the SDK's context has a deadline: the call's would not be needed")
			}
		})
	}
}

// outer gives every method the call's context: initialize, ping,
// tools/list and the rest, the handshake within it.
func TestEveryMethodGetsTheCallContext(t *testing.T) {
	s := New(bare(discard()))
	ctx := context.WithValue(context.Background(), requestKey{}, &request{ctx: context.Background(), read: time.Now()})
	for _, method := range []string{"initialize", "ping", "tools/list", "tools/call", "subscriptions/listen", "prompts/get"} {
		var ok bool
		_, _ = s.outer(s.handshake(func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
			_, ok = ctx.Deadline()
			return nil, nil
		}))(ctx, method, nil)
		if !ok {
			t.Errorf("%s ran without the call's deadline", method)
		}
	}
}

// A client that goes away ends its call long before the call's deadline:
// the request's end cancels it, the one cancellation there is stateless.
func TestTheCallStopsWhenTheClientGoes(t *testing.T) {
	ended := make(chan time.Time, 1)
	slow := toolOf("slow", func(ctx context.Context, _ Call) (Result, error) {
		<-ctx.Done()
		ended <- time.Now()
		return Result{}, ctx.Err()
	})
	s := serveMCP(t, setup{options: func(o *Options) { o.Tools = append(o.Tools, slow); o.RequestTimeout = 8 * time.Second }})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	s.post(t, ctx, Pattern, patToken, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"slow","arguments":{}}}`, v2025())
	select {
	case at := <-ended:
		if took := at.Sub(start); took > 1500*time.Millisecond {
			t.Errorf("the call ended %v after it began, want soon after the client went at 300 ms", took)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the call did not end within 5 s of the client going")
	}
}

// An answer the SDK never writes, a method outliving the request's own
// deadline, is a 500 problem, no-store, at warn, as REST answers a
// deadline; one of a client that went away is its debug line, no error
// (M9/P1 design 3.2.2).
func TestAnUnwrittenAnswerIsAProblem(t *testing.T) {
	stubborn := toolOf("stubborn", func(context.Context, Call) (Result, error) {
		time.Sleep(1500 * time.Millisecond) // past the request's deadline, whatever its context says
		return Result{Structured: map[string]any{}}, nil
	})
	slow := toolOf("slow", func(ctx context.Context, _ Call) (Result, error) {
		<-ctx.Done()
		return Result{}, ctx.Err()
	})
	s := serveMCP(t, setup{noBodyLimit: true, requestTimeout: 500 * time.Millisecond,
		options: func(o *Options) { o.Tools = append(o.Tools, stubborn, slow) }})
	a := s.call(t, Pattern, "stubborn", nil)
	if p := a.decoded(t); a.status != http.StatusInternalServerError || p["code"] != "internal_error" ||
		a.header.Get("Content-Type") != "application/problem+json" || a.header.Get("Cache-Control") != "no-store" {
		t.Errorf("an unwritten answer = %d %v %s, want 500 internal_error, problem+json and no-store", a.status, a.header, a.body)
	}
	if e := s.logs.find(t, "API request deadline exceeded"); e == nil || e["level"] != "WARN" {
		t.Errorf("the log has %v, want the deadline at warn", e)
	}

	// A tool that outlives the client (stubborn) leaves the answer
	// unwritten, which ServeHTTP logs; one that stops on its context
	// (slow) logs its call's client gone, and the SDK may or may not
	// write its error into the closed connection first, so the unwritten
	// line is not its to promise.
	gone := serveMCP(t, setup{options: func(o *Options) { o.Tools = append(o.Tools, stubborn, slow) }})
	for _, tool := range []string{"stubborn", "slow"} {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		gone.post(t, ctx, Pattern, patToken, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"`+tool+`","arguments":{}}}`, v2025())
		cancel()
	}
	deadline := time.Now().Add(5 * time.Second)
	for (gone.logs.find(t, "client went away") == nil || gone.logs.find(t, "MCP client went away") == nil ||
		callLine(t, gone.logs, "stubborn") == nil) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if e := gone.logs.find(t, "client went away"); e == nil || e["level"] != "DEBUG" {
		t.Errorf("the log has %v, want the answer never written, the client gone, at debug", e)
	}
	if e := gone.logs.find(t, "MCP client went away"); e == nil || e["level"] != "DEBUG" {
		t.Errorf("the log has %v, want the call's client gone at debug", e)
	}
	if errs := gone.logs.at(t, "ERROR"); len(errs) != 0 {
		t.Errorf("a client that went away logged errors %v", errs)
	}
}

// panickingNames panics as it cleans a name: a fault outside a tool's
// handler, in the handshake and in a tool's wrapper before its own
// recovery.
type panickingNames struct{}

func (panickingNames) Clean(string) string { panic("names: cleaning") }
func (panickingNames) Recorded(context.Context, uuid.UUID) (string, error) {
	return "", nil
}
func (panickingNames) Record(context.Context, uuid.UUID, string) (bool, error) { return false, nil }

// A panic is answered as its method answers a fault: tools/call with a
// tool error, any other method with JSON-RPC's internal error; it is
// logged with its stack, and the next call is served.
func TestRecoveryAnswersEachMethod(t *testing.T) {
	boom := toolOf("boom", func(context.Context, Call) (Result, error) {
		nilMap()["x"]++
		return Result{}, nil
	})
	panicky := Prompt{Name: "panicky", Get: func(context.Context, Call, map[string]string) (string, []string, error) {
		panic("prompt: getting")
	}}
	s := serveMCP(t, setup{options: func(o *Options) { o.Tools = append(o.Tools, boom); o.Prompts = []Prompt{panicky} }})
	r := result(t, s.call(t, Pattern, "boom", nil))
	if member := errorOf(t, r); member["code"] != "internal_error" || !strings.HasPrefix(toolText(t, r), "internal_error: the server failed; request id ") {
		t.Errorf("a panicking tool = %v, want internal_error", r)
	}
	if e := s.logs.find(t, "MCP method panicked"); e == nil || e["level"] != "ERROR" || e["tool"] != "boom" ||
		!strings.Contains(e["stack"].(string), "goroutine") || e["request_id"] == "" {
		t.Errorf("the log has %v, want the panic at error with the tool, its stack and the request id", e)
	}
	// The wrapper's own recovery, before its call line: that line says
	// what was answered.
	if e := callLine(t, s.logs, "boom"); e == nil || e["outcome"] != "internal_error" {
		t.Errorf("boom's call line = %v, want its outcome internal_error", e)
	}
	a := s.post(t, context.Background(), Pattern, patToken, `{"jsonrpc":"2.0","id":3,"method":"prompts/get","params":{"name":"panicky"}}`, v2025())
	if code := rpcErrorCode(t, a); code != -32603 {
		t.Errorf("a panicking prompts/get = %d %s, want the JSON-RPC internal error -32603", a.status, a.body)
	}
	result(t, s.call(t, Pattern, "echo", nil))

	names := serveMCP(t, setup{options: func(o *Options) { o.Names = panickingNames{} }})
	if code := rpcErrorCode(t, names.post(t, context.Background(), Pattern, patToken, initialize, nil)); code != -32603 {
		t.Errorf("a panicking initialize answered the code %d, want -32603", code)
	}
	r = result(t, names.post(t, context.Background(), Pattern, patToken, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":`+
		`{"name":"echo","arguments":{"notebook":"x"},"_meta":{"io.modelcontextprotocol/clientInfo":{"name":"meta","version":"1"}}}}`, v2025()))
	if member := errorOf(t, r); member["code"] != "internal_error" {
		t.Errorf("a tools/call that panicked outside its handler = %v, want a tool error internal_error", r)
	}
}

// A panic's log names the tool only by a tool's name: a client's name of
// no tool, unbounded, is left out.
func TestAPanicLogsOnlyAToolsName(t *testing.T) {
	logs := &logBuffer{}
	s := New(bare(jsonLogger(logs)))
	for _, name := range []string{"echo", strings.Repeat("x", 33), "Echo", `a"b`} {
		func() {
			var res mcp.Result
			var err error
			defer s.recover(context.Background(), methodCallTool, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: name}}, &res, &err)
			panic("a tool's fault")
		}()
	}
	var tools []any
	for _, e := range logs.entries(t) {
		if e["msg"] == "MCP method panicked" {
			tools = append(tools, e["tool"])
		}
	}
	if len(tools) != 4 || tools[0] != "echo" || tools[1] != nil || tools[2] != nil || tools[3] != nil {
		t.Errorf("the panics logged the tools %v, want echo and none else", tools)
	}
}

// rpcErrorCode is the code of a JSON-RPC error answer.
func rpcErrorCode(t testing.TB, a answer) int {
	t.Helper()
	e, ok := a.decoded(t)["error"].(map[string]any)
	if a.status != http.StatusOK || !ok {
		t.Fatalf("answer = %d %s, want a JSON-RPC error", a.status, a.body)
	}
	return int(e["code"].(float64))
}

// childEnv selects what the test binary does when TestAPanicDoesNotStopTheProcess runs it.
const childEnv = "NWIKI_MCP_PANIC_CHILD"

// The SDK recovers nothing: a tool's panic in its goroutine ends the
// process, exit 2, as a child running the SDK's server without mcpserver
// shows; through mcpserver the same panic is a tool error, and the child
// lives.
func TestAPanicDoesNotStopTheProcess(t *testing.T) {
	if mode := os.Getenv(childEnv); mode != "" {
		panicChild(t, mode)
		return
	}
	for _, tt := range []struct {
		mode string
		exit int
	}{{"sdk", 2}, {"mcpserver", 0}} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAPanicDoesNotStopTheProcess$", "-test.count=1")
		cmd.Env = append(os.Environ(), childEnv+"="+tt.mode)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		code := 0
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		if code != tt.exit {
			t.Errorf("a panicking tool of %s: the process exited %d, want %d\n%s", tt.mode, code, tt.exit, out)
		}
	}
}

// panicChild calls a panicking tool once: of the SDK's server, mounted
// alone, or of mcpserver's.
func panicChild(t *testing.T, mode string) {
	boom := func() { nilMap()["x"]++ }
	var s *served
	if mode == "sdk" {
		sdk := mcp.NewServer(&mcp.Implementation{Name: "sdk", Version: "1"}, nil)
		sdk.AddTool(&mcp.Tool{Name: "boom", InputSchema: map[string]any{"type": "object"}},
			func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { boom(); return nil, nil })
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: mcp.NewStreamableHTTPHandler(
			func(*http.Request) *mcp.Server { return sdk }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})}
		go func() { _ = srv.Serve(ln) }()
		s = &served{base: "http://" + ln.Addr().String()}
	} else {
		s = serveMCP(t, setup{options: func(o *Options) {
			o.Tools = append(o.Tools, toolOf("boom", func(context.Context, Call) (Result, error) { boom(); return Result{}, nil }))
		}})
	}
	r := result(t, s.call(t, Pattern, "boom", nil))
	if r["isError"] != true {
		t.Fatalf("boom = %v, want a tool error", r)
	}
}
