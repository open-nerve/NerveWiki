package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// The unit tests serve the server as bootstrap mounts it, through the
// platform's chain and API.Wrap's middlewares, on a port of its own: an
// authenticator, the notebook entry's gate and the tools are doubles, and
// no database runs.

// The tokens the fake authenticator takes: a personal access token of
// alice, and a session's access token of hers. Any other is 401.
const (
	patToken     = "nwk_pat_alice"
	sessionToken = "session_alice"
)

// alice and her token are the credential of patToken.
var (
	alice    = uuid.MustParse("01920000-0000-7000-8000-00000000a11c") //nolint:gochecknoglobals // read only
	aliceTok = uuid.MustParse("01920000-0000-7000-8000-0000000070c0") //nolint:gochecknoglobals // read only
)

type credentialKey struct{}

// fakeAuth puts the credential of a token it takes in the context, as
// identity's Authenticate puts the actor.
type fakeAuth struct{}

func (fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	switch token {
	case patToken:
		return context.WithValue(ctx, credentialKey{}, Credential{UserID: alice, TokenID: aliceTok}), "pat:" + aliceTok.String(), nil
	case sessionToken:
		return context.WithValue(ctx, credentialKey{}, Credential{UserID: alice}), "session:alice", nil
	}
	return nil, "", problem{status: http.StatusUnauthorized, code: "unauthorized", detail: "The bearer token is invalid."}
}

// credentialsOf reads fakeAuth's credential, as bootstrap's adapter reads
// the actor.
func credentialsOf(ctx context.Context) (Credential, bool) {
	c, ok := ctx.Value(credentialKey{}).(Credential)
	return c, ok
}

// problem is a module's error as httpserver.ProblemError sees it.
type problem struct {
	status       int
	code, detail string
	fields       []error
	retry        time.Duration
}

func (p problem) Error() string             { return p.detail }
func (p problem) ProblemStatus() int        { return p.status }
func (p problem) ProblemCode() string       { return p.code }
func (p problem) ProblemFields() []error    { return p.fields }
func (p problem) RetryAfter() time.Duration { return p.retry }

type fieldProblem struct{ field, code, message string }

func (f fieldProblem) Error() string        { return f.message }
func (f fieldProblem) ProblemField() string { return f.field }
func (f fieldProblem) ProblemCode() string  { return f.code }

// notebookNotFound is page's notebook.not_found.
func notebookNotFound() error {
	return problem{status: http.StatusNotFound, code: "notebook.not_found", detail: "No such notebook."}
}

// visible is the notebook alice sees; any other is not found.
var visible = uuid.MustParse("01920000-0000-7000-8000-0000000000b0") //nolint:gochecknoglobals // read only

// fakeNotebooks gates visible alone, and counts the gate's calls; the
// zero id is not found without a read, as page's gate answers it.
type fakeNotebooks struct {
	gates atomic.Int32
	reads atomic.Int32 // the gate's calls on an id that is not zero
}

func (f *fakeNotebooks) Gate(_ context.Context, id uuid.UUID) error {
	f.gates.Add(1)
	if id == (uuid.UUID{}) {
		return notebookNotFound()
	}
	f.reads.Add(1)
	if id != visible {
		return notebookNotFound()
	}
	return nil
}

func (f *fakeNotebooks) Instructions(context.Context, uuid.UUID) (string, error) { return "", nil }

// nilMap is a map that is nil: writing to it panics, as a tool's fault
// would.
func nilMap() map[string]int { return nil }

// discard is a logger that writes nothing.
func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// unlimited is a bucket that always has a unit.
type unlimited struct{}

func (unlimited) Allow(string) (time.Duration, bool)           { return 0, true }
func (unlimited) Reserve(string) (func(), time.Duration, bool) { return func() {}, 0, true }

// logBuffer collects the JSON log lines of a test; it is safe for the
// SDK's goroutines.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// entries are the lines so far, decoded.
func (l *logBuffer) entries(t testing.TB) []map[string]any {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var entries []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(l.buf.String()), "\n") {
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

// find is the first line of msg, or nil.
func (l *logBuffer) find(t testing.TB, msg string) map[string]any {
	t.Helper()
	for _, e := range l.entries(t) {
		if e["msg"] == msg {
			return e
		}
	}
	return nil
}

// at are the lines of level.
func (l *logBuffer) at(t testing.TB, level string) []map[string]any {
	t.Helper()
	var found []map[string]any
	for _, e := range l.entries(t) {
		if e["level"] == level {
			found = append(found, e)
		}
	}
	return found
}

// setup is what a test changes of its server: the options, the request
// deadline of the per-route middlewares and whether Pattern's body limit
// lengthens it.
type setup struct {
	options func(*Options)
	// requestTimeout is server.request_timeout of the per-route
	// middlewares, 10 s when zero; without a body limit (noBodyLimit) it is
	// the request's deadline, with one, server.read_timeout (5 s) longer.
	requestTimeout time.Duration
	noBodyLimit    bool
}

// served is a running server.
type served struct {
	base      string
	srv       *Server
	notebooks *fakeNotebooks
	logs      *logBuffer
}

// bodyLimit is the route's body limit in the unit tests.
const bodyLimit = 1 << 20

// serveMCP serves the server of s's options on a port of its own until
// the test ends.
func serveMCP(t *testing.T, s setup) *served {
	t.Helper()
	logs := &logBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	timeout := s.requestTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	cfg := httpserver.APIConfig{
		Logger: logger, Authenticator: fakeAuth{}, MaxBodyBytes: 64 << 10, RequestTimeout: timeout,
		BodyReadTimeout: 5 * time.Second, IPv6PrefixLen: 64,
		Anonymous: unlimited{}, Authenticated: unlimited{}, AuthFailure: unlimited{},
	}
	if !s.noBodyLimit {
		cfg.BodyLimits = map[string]int64{Pattern: bodyLimit}
	}
	api, err := httpserver.NewAPI(cfg)
	if err != nil {
		t.Fatal(err)
	}
	notebooks := &fakeNotebooks{}
	o := Options{
		Name: "nervewiki", Version: "test", Tools: testTools(), Credentials: credentialsOf, Notebooks: notebooks,
		Errors: api.Errors, RequestTimeout: 10 * time.Second, MaxBodyBytes: bodyLimit, Logger: logger,
	}
	if s.options != nil {
		s.options(&o)
	}
	srv := New(o)
	router := httpserver.NewRouter(logger)
	router.Handle(Pattern, api.Wrap(srv, Bodies()))
	router.Handle(Pattern+"/", Subtree())
	router.Handle("/.well-known/", WellKnown())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	server := httpserver.NewServer(config.ServerConfig{ReadHeaderTimeout: time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 60 * time.Second, ShutdownTimeout: 10 * time.Second}, router, logger)
	go func() { done <- server.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve() = %v", err)
		}
	})
	return &served{base: "http://" + ln.Addr().String(), srv: srv, notebooks: notebooks, logs: logs}
}

// answer is an HTTP answer as a test reads it.
type answer struct {
	status int
	header http.Header
	body   []byte
}

// decoded is the answer's JSON-RPC result or error, or its problem.
func (a answer) decoded(t testing.TB) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(a.body, &v); err != nil {
		t.Fatalf("answer %d %s: %v", a.status, a.body, err)
	}
	return v
}

// v2025 is the header of a request after a handshake of 2025-11-25.
func v2025() http.Header { return http.Header{"Mcp-Protocol-Version": {"2025-11-25"}} }

// post sends body to path, a JSON POST as a client sends one, with token
// and header; ctx ends it.
func (s *served) post(t testing.TB, ctx context.Context, path, token, body string, header http.Header) answer {
	t.Helper()
	return s.send(t, ctx, http.MethodPost, path, token, body, header)
}

// send is post of any method.
func (s *served) send(t testing.TB, ctx context.Context, method, path, token, body string, header http.Header) answer {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, s.base+path, strings.NewReader(body))
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
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return answer{status: -1}
		}
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("%s %s: reading the answer: %v", method, path, err)
	}
	return answer{status: res.StatusCode, header: res.Header, body: b}
}

// call is a raw tools/call of tool with args, after a handshake of
// 2025-11-25, to path.
func (s *served) call(t testing.TB, path, tool string, args any) answer {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	return s.post(t, context.Background(), path, patToken, string(body), v2025())
}

// result is a JSON-RPC answer's result, failing on anything else.
func result(t testing.TB, a answer) map[string]any {
	t.Helper()
	v := a.decoded(t)
	r, ok := v["result"].(map[string]any)
	if a.status != http.StatusOK || !ok {
		t.Fatalf("answer = %d %s, want a result", a.status, a.body)
	}
	return r
}

// toolText is the text of a tool answer's first block.
func toolText(t testing.TB, r map[string]any) string {
	t.Helper()
	content, _ := r["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("result %v has no content", r)
	}
	block, _ := content[0].(map[string]any)
	text, ok := block["text"].(string)
	if !ok {
		t.Fatalf("block %v has no text", block)
	}
	return text
}

// errorOf is structuredContent.error of a tool's error answer, failing
// on a result that is no error.
func errorOf(t testing.TB, r map[string]any) map[string]any {
	t.Helper()
	sc, _ := r["structuredContent"].(map[string]any)
	member, ok := sc["error"].(map[string]any)
	if r["isError"] != true || !ok {
		t.Fatalf("result %v is no tool error", r)
	}
	return member
}

// bearer adds a token to every request of a client.
type bearer struct {
	token string
	next  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.next.RoundTrip(r)
}

// connect is the official Go client's session of path, alice's personal
// access token on its every request, closed when the test ends.
func (s *served) connect(t testing.TB, path string, opts *mcp.ClientSessionOptions) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "go-test-client", Version: "1"}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: s.base + path,
		HTTPClient: &http.Client{Transport: bearer{patToken, http.DefaultTransport}}}
	cs, err := client.Connect(context.Background(), transport, opts)
	if err != nil {
		t.Fatalf("connecting to %s: %v", path, err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}
