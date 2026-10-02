package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

const (
	thingRoute  = "POST /api/v0/things"
	openRoute   = "POST /api/v0/open" // public
	eventsRoute = "GET /api/v0/events"
)

type callerKey struct{}

// fakeAuth accepts every token except "bad" (401), "expired" (401 with
// ExpiredCredential) and "boom" (a fault). The credential of token is
// session:<token>.
type fakeAuth struct {
	calls int
	ctx   context.Context // what Authenticate was given
}

func (f *fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	f.calls++
	f.ctx = ctx
	switch token {
	case "bad":
		return nil, "", problemErr{status: http.StatusUnauthorized, code: "unauthorized", detail: "Authentication is required."}
	case "expired":
		return nil, "", fmt.Errorf("check token: %w", expiredErr{problemErr{status: http.StatusUnauthorized, code: "unauthorized", detail: "expired"}})
	case "boom":
		return nil, "", errors.New("database is down")
	}
	return context.WithValue(ctx, callerKey{}, "caller-"+token), "session:" + token, nil
}

// expiredErr is the 401 of an access token whose exp has passed.
type expiredErr struct{ problemErr }

func (expiredErr) ExpiredCredential() bool { return true }

// fakeLimiter gives each key burst units and never refills them. It records
// the keys it took a unit of and the keys it gave one back for.
type fakeLimiter struct {
	mu      sync.Mutex
	burst   int
	used    map[string]int
	taken   []string
	refunds []string
}

func newFakeLimiter(burst int) *fakeLimiter {
	return &fakeLimiter{burst: burst, used: map[string]int{}}
}

// take is Allow under f.mu; a refusal waits 1.5 s, Retry-After 2.
func (f *fakeLimiter) take(key string) (time.Duration, bool) {
	if f.used[key] == f.burst {
		return 1500 * time.Millisecond, false
	}
	f.used[key]++
	f.taken = append(f.taken, key)
	return 0, true
}

func (f *fakeLimiter) Allow(key string) (time.Duration, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.take(key)
}

func (f *fakeLimiter) Reserve(key string) (func(), time.Duration, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if retry, ok := f.take(key); !ok {
		return nil, retry, false
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.used[key]--
			f.refunds = append(f.refunds, key)
		})
	}, 0, true
}

// left is what key has of its burst.
func (f *fakeLimiter) left(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.burst - f.used[key]
}

// thingBody is a table as bodyshapegen writes it: {"name": string}, closed.
func thingBody() *bodyshape.Table {
	return &bodyshape.Table{
		Nodes: []bodyshape.Node{
			{Types: bodyshape.Object, Extra: bodyshape.Closed, Items: bodyshape.Open, Props: map[string]int{"name": 1}, Required: []string{"name"}},
			{Types: bodyshape.String, Extra: bodyshape.Open, Items: bodyshape.Open},
		},
		Roots: map[string]int{thingRoute: 0, openRoute: 0},
	}
}

type reached struct {
	called bool
	at     time.Time // when the handler ran
	ctx    context.Context
	body   string
}

// mount registers a handler for thingRoute on a router behind api's
// middlewares, applied the way the generated code does: for each middleware
// in the list, h = m(h).
func mount(api *API) (*Router, *reached) {
	got := &reached{}
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.called, got.at, got.ctx = true, time.Now(), r.Context()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			api.Errors.BodyError(w, r, err)
			return
		}
		got.body = string(data)
		w.WriteHeader(http.StatusNoContent)
	})
	for _, m := range api.Middlewares(thingBody()) {
		h = m(h)
	}
	router := NewRouter(slog.New(slog.DiscardHandler))
	router.Handle(thingRoute, h)
	router.Handle(openRoute, h)
	return router, got
}

// testAPIConfig trusts the proxies of fd00::/8; openRoute is public; its
// buckets never run out in these tests.
func testAPIConfig(auth Authenticator, logger *slog.Logger) APIConfig {
	return APIConfig{
		Logger:           logger,
		Authenticator:    auth,
		PublicOperations: []string{openRoute},
		MaxBodyBytes:     64,
		RequestTimeout:   2 * time.Second,
		BodyReadTimeout:  3 * time.Second,
		TrustedProxies:   []netip.Prefix{netip.MustParsePrefix("fd00::/8")},
		IPv6PrefixLen:    64,
		Anonymous:        newFakeLimiter(100),
		Authenticated:    newFakeLimiter(100),
		AuthFailure:      newFakeLimiter(100),
	}
}

func buildAPI(t *testing.T, cfg APIConfig) *API {
	t.Helper()
	api, err := NewAPI(cfg)
	if err != nil {
		t.Fatalf("NewAPI() error = %v", err)
	}
	return api
}

func newTestAPI(t *testing.T) *API {
	t.Helper()
	return buildAPI(t, testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler)))
}

// post is a request with a valid token, from 203.0.113.7.
func post(path, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.RemoteAddr = "203.0.113.7:5555"
	r.Header.Set("User-Agent", "agent/1.0")
	r.Header.Set("Authorization", "Bearer tok")
	return r
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) Problem {
	t.Helper()
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body %q: %v", rec.Body, err)
	}
	return p
}

func TestNewAPIRequiresItsSettings(t *testing.T) {
	_, err := NewAPI(APIConfig{})

	want := "httpserver: APIConfig: no Logger\nno Authenticator\nno Anonymous\nno Authenticated\nno AuthFailure\n" +
		"IPv6PrefixLen 0 is outside 1-128\nMaxBodyBytes must be positive\nRequestTimeout must be positive"
	if err == nil || err.Error() != want {
		t.Errorf("NewAPI() error = %v, want %q", err, want)
	}
}

// A prefix length that cannot key an IPv6 client is refused at startup.
func TestNewAPIRequiresAnIPv6PrefixLenFrom1To128(t *testing.T) {
	for n, ok := range map[int]bool{0: false, 1: true, 128: true, 129: false} {
		cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
		cfg.IPv6PrefixLen = n
		want := fmt.Sprintf("httpserver: APIConfig: IPv6PrefixLen %d is outside 1-128", n)
		if _, err := NewAPI(cfg); ok != (err == nil) || (err != nil && err.Error() != want) {
			t.Errorf("NewAPI(IPv6PrefixLen %d) error = %v, want ok %v", n, err, ok)
		}
	}
}

func TestValidBodyReachesTheHandler(t *testing.T) {
	router, got := mount(newTestAPI(t))

	rec := serve(router, post("/api/v0/things", `{"name":"a"}`))

	if rec.Code != http.StatusNoContent || !got.called || got.body != `{"name":"a"}` {
		t.Errorf("status = %d, handler called %v with %q; want 204 and the body unchanged", rec.Code, got.called, got.body)
	}
}

// server.write_timeout never cancels the request's context: the operation's
// own deadline does.
func TestOperationsRunUnderTheRequestDeadline(t *testing.T) {
	router, got := mount(newTestAPI(t))
	begin := time.Now()

	serve(router, post("/api/v0/things", `{"name":"a"}`))

	end := time.Now()
	deadline, ok := got.ctx.Deadline()
	if !ok || deadline.Before(begin.Add(2*time.Second)) || deadline.After(end.Add(2*time.Second)) {
		t.Errorf("handler deadline = %v (set %v), want 2s after the request arrived", deadline, ok)
	}
}

// A route of RequestTimeouts runs under its own, shorter deadline; the
// others keep RequestTimeout. When it expires, the answer and the log are
// those of any request deadline (M1/P2 review M1): a warning, not a fault.
func TestARouteTimeoutBoundsItsRoute(t *testing.T) {
	logger, logs := captureLogs(t)
	cfg := testAPIConfig(&fakeAuth{}, logger)
	cfg.RequestTimeouts = map[string]time.Duration{openRoute: 50 * time.Millisecond}
	api := buildAPI(t, cfg)
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // a statement waiting for a lock
		api.Errors.Write(w, r, fmt.Errorf("rotate session: %w", r.Context().Err()))
	})
	for _, m := range api.Middlewares(thingBody()) {
		h = m(h)
	}
	router := NewRouter(slog.New(slog.DiscardHandler))
	router.Handle(openRoute, h)
	begin := time.Now()

	rec := serve(router, post("/api/v0/open", `{"name":"a"}`))

	if took := time.Since(begin); took < 50*time.Millisecond || took > time.Second {
		t.Errorf("the route answered after %v, want its 50ms deadline", took)
	}
	if p := decodeProblem(t, rec); rec.Code != http.StatusInternalServerError || p.Code != CodeInternal {
		t.Errorf("response = %d %+v, want 500 internal_error", rec.Code, p)
	}
	if entry := findLog(logs(), "API request deadline exceeded"); entry == nil || entry["level"] != "WARN" {
		t.Errorf("logs = %v, want a warning that the request ran out of time", logs())
	}
	other, got := mount(api)
	serve(other, post("/api/v0/things", `{"name":"a"}`))
	if deadline, ok := got.ctx.Deadline(); !ok || time.Until(deadline) < time.Second {
		t.Errorf("another route's deadline = %v (set %v), want RequestTimeout's 2s", deadline, ok)
	}
}

// A route timeout must be positive and no longer than RequestTimeout.
func TestNewAPIChecksRouteTimeouts(t *testing.T) {
	for d, ok := range map[time.Duration]bool{0: false, time.Millisecond: true, 2 * time.Second: true, 3 * time.Second: false} {
		cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
		cfg.RequestTimeouts = map[string]time.Duration{openRoute: d}
		want := fmt.Sprintf(`httpserver: APIConfig: RequestTimeouts["POST /api/v0/open"] %v is outside (0, RequestTimeout 2s]`, d)
		if _, err := NewAPI(cfg); ok != (err == nil) || (err != nil && err.Error() != want) {
			t.Errorf("NewAPI(RequestTimeouts %v) error = %v, want ok %v", d, err, ok)
		}
	}
}

// A route of BodyLimits takes a body up to its own limit, larger than
// MaxBodyBytes, and no more; the other routes keep MaxBodyBytes; a route
// limit below MaxBodyBytes does not tighten it.
func TestARouteBodyLimitRelaxesItsRoute(t *testing.T) {
	body := func(n int) string { return `{"name":"` + strings.Repeat("a", n-len(`{"name":""}`)) + `"}` }
	for _, tt := range []struct {
		name  string
		limit int64
		route string
		size  int
		want  int
	}{
		{"at the route's limit", 100, openRoute, 100, http.StatusNoContent},
		{"past the route's limit", 100, openRoute, 101, http.StatusRequestEntityTooLarge},
		{"another route", 100, thingRoute, 65, http.StatusRequestEntityTooLarge},
		{"a route limit below MaxBodyBytes", 10, openRoute, 64, http.StatusNoContent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
			cfg.BodyLimits = map[string]int64{openRoute: tt.limit}
			router, _ := mount(buildAPI(t, cfg))
			path := "/api/v0/things"
			if tt.route == openRoute {
				path = "/api/v0/open"
			}
			if rec := serve(router, post(path, body(tt.size))); rec.Code != tt.want {
				t.Errorf("a body of %d bytes = %d %s, want %d", tt.size, rec.Code, rec.Body, tt.want)
			}
		})
	}
}

// A route of BodyLimits gives its body BodyReadTimeout to arrive on top of
// RequestTimeout: a slow upload of a large body leaves its handler the
// whole of RequestTimeout (M4/P4 review P3). The other routes keep
// RequestTimeout.
func TestARouteBodyLimitLengthensItsDeadline(t *testing.T) {
	cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
	cfg.BodyLimits = map[string]int64{openRoute: 100}
	for path, want := range map[string]time.Duration{"/api/v0/open": 5 * time.Second, "/api/v0/things": 2 * time.Second} {
		router, got := mount(buildAPI(t, cfg))
		begin := time.Now()
		serve(router, post(path, `{"name":"a"}`))
		end := time.Now()
		if deadline, ok := got.ctx.Deadline(); !ok || deadline.Before(begin.Add(want)) || deadline.After(end.Add(want)) {
			t.Errorf("%s: handler deadline = %v (set %v), want %v after the request arrived", path, deadline, ok, want)
		}
	}
}

// A route body limit must be positive, and needs a body read timeout.
func TestNewAPIChecksRouteBodyLimits(t *testing.T) {
	cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
	cfg.BodyLimits, cfg.BodyReadTimeout = map[string]int64{openRoute: 100}, 0
	if _, err := NewAPI(cfg); err == nil || err.Error() != "httpserver: APIConfig: BodyReadTimeout must be positive when BodyLimits relaxes a route" {
		t.Errorf("NewAPI(BodyLimits without BodyReadTimeout) error = %v", err)
	}
	for n, ok := range map[int64]bool{-1: false, 0: false, 1: true, 1 << 30: true} {
		cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
		cfg.BodyLimits = map[string]int64{openRoute: n}
		want := fmt.Sprintf(`httpserver: APIConfig: BodyLimits["POST /api/v0/open"] %d must be positive`, n)
		if _, err := NewAPI(cfg); ok != (err == nil) || (err != nil && err.Error() != want) {
			t.Errorf("NewAPI(BodyLimits %d) error = %v, want ok %v", n, err, ok)
		}
	}
}

// slowBody delivers its content only after a pause, as a slow client does.
type slowBody struct {
	pause time.Duration
	r     io.Reader
}

func (b *slowBody) Read(p []byte) (int, error) {
	time.Sleep(b.pause)
	b.pause = 0
	return b.r.Read(p)
}

// The deadline is the outermost per-route middleware: the time spent reading
// the body (in the body check) counts against it, as the time M1's
// authentication and rate limiting spend will.
func TestTheRequestDeadlineCoversReadingTheBody(t *testing.T) {
	router, got := mount(newTestAPI(t))
	const pause = 300 * time.Millisecond
	req := post("/api/v0/things", "")
	req.Body = io.NopCloser(&slowBody{pause: pause, r: strings.NewReader(`{"name":"a"}`)})

	serve(router, req)

	deadline, ok := got.ctx.Deadline()
	if left := deadline.Sub(got.at); !ok || left > 2*time.Second-pause {
		t.Errorf("the handler had %v of the 2s budget left (deadline set %v), want at most %v: the body was read before the deadline started", left, ok, 2*time.Second-pause)
	}
}

// A long-lived route is registered on the router directly, next to the API
// operations: behind the server's whole middleware chain it gets no request
// deadline, as it must hold its response open (M0/P3 design 3.4).
func TestLongLivedRoutesHaveNoRequestDeadline(t *testing.T) {
	router, _ := mount(newTestAPI(t))
	var streamCtx context.Context
	router.Handle(eventsRoute, LongLived(slog.New(slog.DiscardHandler), http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		streamCtx = r.Context()
	})))
	srv := httptest.NewServer(middleware(router, slog.New(slog.DiscardHandler)))
	t.Cleanup(srv.Close)

	resp, err := client().Get(srv.URL + "/api/v0/events")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if _, ok := streamCtx.Deadline(); ok || resp.StatusCode != http.StatusOK {
		t.Errorf("long-lived route answered %d with a deadline %v; want 200 without one", resp.StatusCode, ok)
	}
}

// The body check reads the body through the body limit: the limit runs
// before it, and the check's own read is what fails.
func TestBodyLimitRunsBeforeTheBodyCheck(t *testing.T) {
	router, got := mount(newTestAPI(t))

	rec := serve(router, post("/api/v0/things", `{"name":"`+strings.Repeat("a", 100)+`"}`))

	if p := decodeProblem(t, rec); rec.Code != http.StatusRequestEntityTooLarge || p.Code != CodePayloadTooLarge || got.called {
		t.Errorf("response = %d %+v, handler called %v; want 413 payload_too_large", rec.Code, p, got.called)
	}
}

func TestBodyCheckAnswersEveryProblemAs400(t *testing.T) {
	router, got := mount(newTestAPI(t))

	rec := serve(router, post("/api/v0/things", `{"extra":1}`))

	want := `{"status":400,"code":"bad_request","title":"Bad Request","detail":"The request body does not match the API description.",` +
		`"errors":[{"field":"extra","code":"not_allowed","message":"is not a property of this request"},{"field":"name","code":"required","message":"is required"}]}` + "\n"
	if rec.Code != http.StatusBadRequest || rec.Body.String() != want || got.called {
		t.Errorf("response = %d %s, handler called %v; want 400 %s", rec.Code, rec.Body, got.called, want)
	}
}

func TestBodyThatIsNotJSONIs400WithAGenericDetail(t *testing.T) {
	router, _ := mount(newTestAPI(t))

	rec := serve(router, post("/api/v0/things", `{"name":`))

	if p := decodeProblem(t, rec); rec.Code != http.StatusBadRequest || p.Detail != "The request body could not be decoded." || len(p.Errors) != 0 {
		t.Errorf("response = %d %+v, want 400 with the generic detail", rec.Code, p)
	}
}

// The request meta middleware puts the client in the context: the address a
// trusted proxy forwarded, and the User-Agent.
func TestRequestMetaCarriesTheClient(t *testing.T) {
	router, got := mount(newTestAPI(t))
	req := post("/api/v0/things", `{"name":"a"}`)
	req.RemoteAddr = "[fd00::1]:443"
	req.Header.Set("X-Forwarded-For", "2001:db8:1:2::7")

	serve(router, req)

	want := RequestMeta{ClientIP: netip.MustParseAddr("2001:db8:1:2::7"), IPKey: "2001:db8:1:2::/64", UserAgent: "agent/1.0"}
	if meta := RequestMetaFrom(got.ctx); meta != want {
		t.Errorf("meta = %+v, want %+v", meta, want)
	}
}
