package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
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

// fakeAuth accepts every token except "bad" (401) and "boom" (a fault).
type fakeAuth struct {
	calls int
	ctx   context.Context // what Authenticate was given
}

func (f *fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, error) {
	f.calls++
	f.ctx = ctx
	switch token {
	case "bad":
		return nil, problemErr{status: http.StatusUnauthorized, code: "unauthorized", detail: "Authentication is required."}
	case "boom":
		return nil, errors.New("database is down")
	}
	return context.WithValue(ctx, callerKey{}, "caller-"+token), nil
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

// testAPIConfig trusts the proxies of fd00::/8; openRoute is public.
func testAPIConfig(auth Authenticator, logger *slog.Logger) APIConfig {
	return APIConfig{
		Logger:           logger,
		Authenticator:    auth,
		PublicOperations: []string{openRoute},
		MaxBodyBytes:     64,
		RequestTimeout:   2 * time.Second,
		TrustedProxies:   []netip.Prefix{netip.MustParsePrefix("fd00::/8")},
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

	want := "httpserver: APIConfig: no Logger\nno Authenticator\nMaxBodyBytes must be positive\nRequestTimeout must be positive"
	if err == nil || err.Error() != want {
		t.Errorf("NewAPI() error = %v, want %q", err, want)
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

func TestAuthenticationDeniesByDefault(t *testing.T) {
	tests := []struct {
		name         string
		path, header string
		status       int
		challenge    string
		authCalls    int
	}{
		{"no token", "/api/v0/things", "", 401, "Bearer", 0},
		{"another scheme", "/api/v0/things", "Basic dXNlcjpwYXNz", 401, "Bearer", 0},
		{"empty bearer", "/api/v0/things", "Bearer ", 401, "Bearer", 0},
		{"a token with a space", "/api/v0/things", "Bearer tok en", 401, "Bearer", 0},
		{"invalid token", "/api/v0/things", "Bearer bad", 401, `Bearer error="invalid_token"`, 1},
		{"valid token", "/api/v0/things", "Bearer tok", 204, "", 1},
		{"scheme in lower case", "/api/v0/things", "bearer tok", 204, "", 1},
		{"authenticator fault", "/api/v0/things", "Bearer boom", 500, "", 1},
		{"public without a token", "/api/v0/open", "", 204, "", 0},
		{"public ignores a bad token", "/api/v0/open", "Bearer bad", 204, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &fakeAuth{}
			router, got := mount(buildAPI(t, testAPIConfig(auth, slog.New(slog.DiscardHandler))))
			req := post(tt.path, `{"name":"a"}`)
			req.Header.Set("Authorization", tt.header)

			rec := serve(router, req)

			challenge := rec.Result().Header.Get("WWW-Authenticate")
			if rec.Code != tt.status || challenge != tt.challenge || auth.calls != tt.authCalls || got.called != (tt.status == 204) {
				t.Errorf("response = %d, WWW-Authenticate %q, authenticator called %d times, handler called %v; want %d, %q, %d",
					rec.Code, challenge, auth.calls, got.called, tt.status, tt.challenge, tt.authCalls)
			}
			if tt.status == 401 {
				if p := decodeProblem(t, rec); p.Code != CodeUnauthorized {
					t.Errorf("problem code = %q, want unauthorized", p.Code)
				}
			}
		})
	}
}

// The handler runs with the context the authenticator returned: the caller
// that authentication put in it.
func TestTheHandlerSeesTheAuthenticatedCaller(t *testing.T) {
	router, got := mount(newTestAPI(t))

	serve(router, post("/api/v0/things", `{"name":"a"}`))

	if caller, _ := got.ctx.Value(callerKey{}).(string); caller != "caller-tok" {
		t.Errorf("caller = %q, want caller-tok", caller)
	}
}

// Why a credential failed goes to the debug log, never into the response.
func TestAuthenticationFailureIsLoggedAtDebugLevel(t *testing.T) {
	logger, logs := captureLogs(t)
	router, _ := mount(buildAPI(t, testAPIConfig(&fakeAuth{}, logger)))
	req := post("/api/v0/things", `{"name":"a"}`)
	req.Header.Set("Authorization", "Bearer bad")

	rec := serve(router, req)

	entry := findLog(logs(), "authentication failed")
	if entry == nil || entry["level"] != "DEBUG" || entry["route"] != thingRoute || entry["error"] != "Authentication is required." {
		t.Errorf("log = %v, want the reason at debug level", entry)
	}
	if strings.Contains(rec.Body.String(), "Authentication is required.") {
		t.Errorf("body %s carries the authenticator's reason", rec.Body)
	}
}

// Authentication runs inside the request deadline, with the request meta
// in its context, and before the body is read: a request without a token
// never gets its body checked.
func TestAuthenticationRunsUnderTheDeadlineBeforeTheBodyCheck(t *testing.T) {
	auth := &fakeAuth{}
	router, _ := mount(buildAPI(t, testAPIConfig(auth, slog.New(slog.DiscardHandler))))

	serve(router, post("/api/v0/things", `{"name":"a"}`))
	if _, ok := auth.ctx.Deadline(); !ok || !RequestMetaFrom(auth.ctx).ClientIP.IsValid() {
		t.Errorf("the authenticator's context has deadline %v and meta %+v; want both", ok, RequestMetaFrom(auth.ctx))
	}

	req := post("/api/v0/things", `{"extra":1}`)
	req.Header.Del("Authorization")
	if rec := serve(router, req); rec.Code != http.StatusUnauthorized {
		t.Errorf("a broken body without a token: status %d, want 401", rec.Code)
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

	want := RequestMeta{ClientIP: netip.MustParseAddr("2001:db8:1:2::7"), UserAgent: "agent/1.0"}
	if meta := RequestMetaFrom(got.ctx); meta != want {
		t.Errorf("meta = %+v, want %+v", meta, want)
	}
}
