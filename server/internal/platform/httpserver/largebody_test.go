package httpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// The routes of the large bodies' tests: big and late take any body,
// shaped is checked by bodyshape, late's deadline is short; each takes a
// slot past 1 KiB.
const (
	bigRoute    = "POST /big"
	shapedRoute = "POST /shaped"
	lateRoute   = "POST /late"
	threshold   = 1 << 10
	routeLimit  = 1 << 20
)

// largeAPIConfig is testAPIConfig with slots of the large bodies, waiting
// wait: big, shaped and late are routes of BodyLimits of routeLimit, their
// threshold 1 KiB, late's deadline 1 s. A token's credential is
// session:<token> (fakeAuth), so two tokens are two credentials.
func largeAPIConfig(logger *slog.Logger, slots int, wait time.Duration) APIConfig {
	cfg := testAPIConfig(&fakeAuth{}, logger)
	cfg.BodyLimits = map[string]int64{bigRoute: routeLimit, shapedRoute: routeLimit, lateRoute: routeLimit}
	cfg.LargeBodyAbove = map[string]int64{bigRoute: threshold, shapedRoute: threshold, lateRoute: threshold}
	cfg.RequestTimeouts = map[string]time.Duration{lateRoute: time.Second}
	cfg.LargeBodies, cfg.LargeBodyWait = slots, wait
	return cfg
}

// largeServer serves the large bodies' routes and thingRoute behind the
// platform's chain. A request with X-Hold tells the test it holds its slot
// (entered) before it reads its body, then waits until the test releases
// its key or its context ends; X-Mode answers it otherwise than 200 with
// its body: refuse with a problem, panic with a panic.
type largeServer struct {
	api    *API
	url    string
	mu     sync.Mutex
	in     map[string]chan struct{}
	out    map[string]chan struct{}
	called map[string]int // the handler's calls, by token
}

func serveLarge(t *testing.T, cfg APIConfig) *largeServer {
	t.Helper()
	api := buildAPI(t, cfg)
	s := &largeServer{api: api, in: map[string]chan struct{}{}, out: map[string]chan struct{}{}, called: map[string]int{}}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.called[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]++
		s.mu.Unlock()
		key := r.Header.Get("X-Hold")
		if key != "" {
			close(s.gate(s.in, key))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			api.Errors.BodyError(w, r, err)
			return
		}
		if key != "" {
			select {
			case <-s.gate(s.out, key):
			case <-r.Context().Done():
				api.Errors.Write(w, r, fmt.Errorf("holding: %w", r.Context().Err()))
				return
			}
		}
		switch r.Header.Get("X-Mode") {
		case "refuse":
			api.Errors.Write(w, r, problemErr{status: http.StatusConflict, code: "thing.taken", detail: "Taken."})
		case "panic":
			panic("a handler's fault")
		default:
			_, _ = w.Write(body)
		}
	})
	shaped := &bodyshape.Table{
		Nodes: []bodyshape.Node{
			{Types: bodyshape.Object, Extra: bodyshape.Closed, Items: bodyshape.Open, Props: map[string]int{"name": 1}, Required: []string{"name"}},
			{Types: bodyshape.String, Extra: bodyshape.Open, Items: bodyshape.Open},
		},
		Roots: map[string]int{shapedRoute: 0, thingRoute: 0},
	}
	router := NewRouter(slog.New(slog.DiscardHandler))
	for _, route := range []string{bigRoute, shapedRoute, lateRoute, thingRoute} {
		router.Handle(route, api.Wrap(h, shaped))
	}
	srv := httptest.NewServer(middleware(router, slog.New(slog.DiscardHandler)))
	t.Cleanup(srv.Close)
	s.url = srv.URL
	return s
}

// gate is key's channel of m, made on first use.
func (s *largeServer) gate(m map[string]chan struct{}, key string) chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := m[key]
	if !ok {
		c = make(chan struct{})
		m[key] = c
	}
	return c
}

// calls is how many requests of token reached the handler.
func (s *largeServer) calls(token string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.called[token]
}

// release lets the held request of key answer.
func (s *largeServer) release(key string) { close(s.gate(s.out, key)) }

// largeAnswer is an answer as the tests read it, or the client's error.
type largeAnswer struct {
	status int
	header http.Header
	body   string
	err    error
}

// largeRequest is a request of the tests: a token, its route's path, a
// body of a declared length (-1 for none: chunked), and its headers.
type largeRequest struct {
	token, path string
	body        io.Reader
	length      int64
	header      map[string]string
}

// do sends q.
func (s *largeServer) do(q largeRequest) largeAnswer {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.url+q.path, q.body)
	if err != nil {
		return largeAnswer{err: err}
	}
	req.ContentLength = q.length
	if q.token != "" {
		req.Header.Set("Authorization", "Bearer "+q.token)
	}
	for k, v := range q.header {
		req.Header.Set(k, v)
	}
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return largeAnswer{err: err}
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	return largeAnswer{status: res.StatusCode, header: res.Header, body: string(b), err: err}
}

// sized is a request of token to path with a body of n bytes, declared:
// a JSON object, as shaped takes.
func sized(token, path string, n int) largeRequest {
	body := `{"name":"` + strings.Repeat("a", n-len(`{"name":""}`)) + `"}`
	return largeRequest{token: token, path: path, body: strings.NewReader(body), length: int64(n)}
}

// held is sized holding its slot under key.
func held(token, path string, n int, key string) largeRequest {
	q := sized(token, path, n)
	q.header = map[string]string{"X-Hold": key}
	return q
}

// start sends q, its answer on the channel; when q holds, start returns
// once the handler has it.
func (s *largeServer) start(t *testing.T, q largeRequest) <-chan largeAnswer {
	t.Helper()
	done := make(chan largeAnswer, 1)
	go func() { done <- s.do(q) }()
	if key := q.header["X-Hold"]; key != "" {
		select {
		case <-s.gate(s.in, key):
		case a := <-done:
			t.Fatalf("the request of %s answered %d %s (%v) before it held its slot", key, a.status, a.body, a.err)
		case <-time.After(10 * time.Second):
			t.Fatalf("the request of %s never held its slot", key)
		}
	}
	return done
}

// answered is the answer on done, within 10 s.
func answered(t *testing.T, done <-chan largeAnswer) largeAnswer {
	t.Helper()
	select {
	case a := <-done:
		return a
	case <-time.After(10 * time.Second):
		t.Fatal("no answer within 10 s")
		return largeAnswer{}
	}
}

// waitUntil polls cond until it holds, failing past 10 s or when done
// answers first: a request the test waits for must still be waiting.
func waitUntil(t *testing.T, what string, done <-chan largeAnswer, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		select {
		case a := <-done:
			t.Fatalf("answered %d %s (%v) before %s", a.status, a.body, a.err, what)
		case <-time.After(time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("not %s within 10 s", what)
		}
	}
}

// refs is how many requests of credential hold or wait for its slot.
func (l *largeBodies) refs(credential string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if c, ok := l.credentials[credential]; ok {
		return c.refs
	}
	return 0
}

// idle is whether no slot is held or waited for, and no holder recorded:
// a holder left behind would be named by every saturated line after
// (M9/P2 S1 review C-M2).
func (l *largeBodies) idle() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.server) == 0 && len(l.credentials) == 0 && len(l.holders) == 0
}

// isBusy reports whether a is the slots' 503: server_busy, Retry-After 1,
// problem+json.
func isBusy(a largeAnswer) bool {
	return a.err == nil && a.status == http.StatusServiceUnavailable && a.header.Get("Retry-After") == "1" &&
		a.header.Get("Content-Type") == ContentTypeProblem && strings.Contains(a.body, `"code":"server_busy"`)
}

// Two credentials' large bodies hold the two slots at once; a third's
// waits, then is 503 server_busy; a body at the threshold is not large;
// once a slot is back, the third's next one takes it: its 503 gave its
// credential's slot back.
func TestTwoLargeBodiesAreServedAtOnce(t *testing.T) {
	s := serveLarge(t, largeAPIConfig(slog.New(slog.DiscardHandler), 2, 50*time.Millisecond))
	a := s.start(t, held("a", "/big", 2*threshold, "a1"))
	b := s.start(t, held("b", "/big", 2*threshold, "b1"))

	if got := s.do(sized("c", "/big", 2*threshold)); !isBusy(got) {
		t.Errorf("a third credential's large body = %d %v %s (%v), want 503 server_busy, Retry-After 1", got.status, got.header, got.body, got.err)
	}
	if got := s.do(sized("c", "/big", threshold)); got.status != http.StatusOK || len(got.body) != threshold {
		t.Errorf("a body at the threshold = %d %s (%v), want 200: it is not large", got.status, got.body, got.err)
	}
	s.release("a1")
	if got := answered(t, a); got.status != http.StatusOK {
		t.Errorf("the first large body = %d %s (%v), want 200", got.status, got.body, got.err)
	}
	waitUntil(t, "the first slot back", nil, func() bool { return len(s.api.large.server) == 1 })
	if got := s.do(sized("c", "/big", 2*threshold)); got.status != http.StatusOK {
		t.Errorf("the third credential's next large body = %d %s (%v), want 200: its slot came back", got.status, got.body, got.err)
	}
	s.release("b1")
	answered(t, b)
}

// A credential holds one slot: its second large body waits for its first,
// not for the server's slot left, which another credential takes; it is
// 503 when its first holds on, and takes its credential's slot when the
// first gives it back.
func TestACredentialsSecondLargeBodyWaitsForItsFirst(t *testing.T) {
	t.Run("its first holds on", func(t *testing.T) {
		s := serveLarge(t, largeAPIConfig(slog.New(slog.DiscardHandler), 2, 50*time.Millisecond))
		first := s.start(t, held("a", "/big", 2*threshold, "a1"))
		if got := s.do(sized("a", "/big", 2*threshold)); !isBusy(got) {
			t.Errorf("a's second large body = %d %s (%v), want 503: it waits for a's first, not the slot left", got.status, got.body, got.err)
		}
		if got := s.do(sized("b", "/big", 2*threshold)); got.status != http.StatusOK {
			t.Errorf("b's large body = %d %s (%v), want 200: the slot left is b's to take", got.status, got.body, got.err)
		}
		s.release("a1")
		answered(t, first)
	})
	t.Run("its first gives it back", func(t *testing.T) {
		s := serveLarge(t, largeAPIConfig(slog.New(slog.DiscardHandler), 2, 10*time.Second))
		first := s.start(t, held("a", "/big", 2*threshold, "a1"))
		second := s.start(t, sized("a", "/big", 2*threshold))
		waitUntil(t, "a's second waiting", second, func() bool { return s.api.large.refs("session:a") == 2 && s.api.large.waiting.Load() == 1 })
		s.release("a1")
		answered(t, first)
		if got := answered(t, second); got.status != http.StatusOK {
			t.Errorf("a's second large body = %d %s (%v), want 200: it took the slot its first gave back", got.status, got.body, got.err)
		}
	})
}

// The 503 of the slots logs one info line: the route, this request's
// credential, whether its own first holds its credential's slot, how many
// waited, and the holders' credentials and routes; never the body.
func TestTheSaturatedLineNamesTheHolders(t *testing.T) {
	logger, logs := captureLogs(t)
	s := serveLarge(t, largeAPIConfig(logger, 2, 50*time.Millisecond))
	a := s.start(t, held("a", "/big", 2*threshold, "a1"))
	b := s.start(t, held("b", "/shaped", 2*threshold, "b1"))
	marker := "SECRETBODY"
	body := marker + strings.Repeat("x", 2*threshold)
	s.do(largeRequest{token: "c", path: "/big", body: strings.NewReader(body), length: int64(len(body))})
	s.do(largeRequest{token: "a", path: "/late", body: strings.NewReader(body), length: int64(len(body))})
	s.release("a1")
	s.release("b1")
	answered(t, a)
	answered(t, b)

	var lines []map[string]any
	for _, e := range logs() {
		if e["msg"] == "large request bodies are saturated" {
			lines = append(lines, e)
		}
		if strings.Contains(fmt.Sprint(e), marker) {
			t.Errorf("a log line has the body: %v", e)
		}
	}
	holders := []any{map[string]any{"credential": "session:a", "route": bigRoute}, map[string]any{"credential": "session:b", "route": shapedRoute}}
	want := []map[string]any{
		{"level": "INFO", "route": bigRoute, "credential": "session:c", "own": false, "waiting": float64(1), "holders": holders},
		{"level": "INFO", "route": lateRoute, "credential": "session:a", "own": true, "waiting": float64(1), "holders": holders},
	}
	if len(lines) != len(want) {
		t.Fatalf("saturated lines = %v, want %d", lines, len(want))
	}
	for i, w := range want {
		for k, v := range w {
			if fmt.Sprint(lines[i][k]) != fmt.Sprint(v) {
				t.Errorf("line %d's %s = %v, want %v (%v)", i, k, lines[i][k], v, lines[i])
			}
		}
	}
}

// A body of no declared length (chunked) is read up to its threshold: one
// past it takes a slot, one at it does not; the handler reads what was
// sent, byte for byte, the prefix put back; a route bodyshape checks too.
func TestABodyOfNoLengthTakesASlotPastItsThreshold(t *testing.T) {
	chunked := func(token, path, body, key string) largeRequest {
		half := len(body) / 2
		q := largeRequest{token: token, path: path, body: io.MultiReader(strings.NewReader(body[:half]), strings.NewReader(body[half:])), length: -1}
		if key != "" {
			q.header = map[string]string{"X-Hold": key}
		}
		return q
	}
	large := strings.Repeat("0123456789abcdef", 2*threshold/16) + "z"
	shapedBody := `{"name":"` + strings.Repeat("é", threshold) + `"}`
	for _, tt := range []struct{ name, path, body string }{{"a route of any body", "/big", large}, {"a route bodyshape checks", "/shaped", shapedBody}} {
		t.Run(tt.name, func(t *testing.T) {
			s := serveLarge(t, largeAPIConfig(slog.New(slog.DiscardHandler), 1, 50*time.Millisecond))
			done := s.start(t, chunked("a", tt.path, tt.body, "a1"))
			if got := s.do(sized("p", "/big", 2*threshold)); !isBusy(got) {
				t.Errorf("a probe while the chunked body holds = %d %s (%v), want 503: it took the slot", got.status, got.body, got.err)
			}
			at := strings.Repeat("y", threshold)
			if got := s.do(chunked("b", "/big", at, "")); got.status != http.StatusOK || got.body != at {
				t.Errorf("a chunked body at the threshold = %d (%v), want 200 and its bytes: it takes no slot", got.status, got.err)
			}
			s.release("a1")
			if got := answered(t, done); got.status != http.StatusOK || got.body != tt.body {
				t.Errorf("the chunked body = %d, %d bytes back (%v), want 200 and the %d bytes sent", got.status, len(got.body), got.err, len(tt.body))
			}
		})
	}
}

// A declared length past the route's limit is 413 at once, while the slots
// are taken: no slot, no handler. A length at the limit waits as any large
// body does.
func TestADeclaredLengthPastTheLimitIs413WithoutASlot(t *testing.T) {
	s := serveLarge(t, largeAPIConfig(slog.New(slog.DiscardHandler), 2, 50*time.Millisecond))
	a := s.start(t, held("a", "/big", 2*threshold, "a1"))
	b := s.start(t, held("b", "/big", 2*threshold, "b1"))

	got := s.do(sized("c", "/big", routeLimit+1))
	if got.status != http.StatusRequestEntityTooLarge || !strings.Contains(got.body, `"code":"payload_too_large"`) {
		t.Errorf("a declared length past the limit = %d %s (%v), want 413 payload_too_large", got.status, got.body, got.err)
	}
	if n := s.calls("c"); n != 0 || len(s.api.large.server) != 2 || s.api.large.refs("session:c") != 0 {
		t.Errorf("the handler was called %d times, %d slots held, c counted %d; want none, the two, none", n, len(s.api.large.server), s.api.large.refs("session:c"))
	}
	if got := s.do(sized("c", "/big", routeLimit)); !isBusy(got) {
		t.Errorf("a declared length at the limit = %d %s (%v), want 503: it waits for a slot", got.status, got.body, got.err)
	}
	s.release("a1")
	s.release("b1")
	answered(t, a)
	answered(t, b)
}

// Every way a request ends gives its slots back. With one slot, a request
// on its way holds it, which a probe of another credential finds taken;
// once it ended, another credential's request and one of its own take it
// again. A slot left out would keep them waiting until 503: the test waits
// for the slots to come back, then reads the answers.
func TestTheSlotComesBackOnEveryPath(t *testing.T) {
	refused := func(q largeRequest) largeRequest { q.header["X-Mode"] = "refuse"; return q }
	panicking := func(q largeRequest) largeRequest { q.header["X-Mode"] = "panic"; return q }
	tests := []struct {
		name   string
		q      largeRequest
		end    func(s *largeServer, q largeRequest)
		status int // the answer the client reads, 0 when it reads none
	}{
		{"its handler answers", held("a", "/big", 2*threshold, "k"), func(s *largeServer, _ largeRequest) { s.release("k") }, http.StatusOK},
		{"its handler refuses", refused(held("a", "/big", 2*threshold, "k")), func(s *largeServer, _ largeRequest) { s.release("k") }, http.StatusConflict},
		{"its handler panics", panicking(held("a", "/big", 2*threshold, "k")), func(s *largeServer, _ largeRequest) { s.release("k") }, http.StatusInternalServerError},
		{"its deadline passes", held("a", "/late", 2*threshold, "k"), func(*largeServer, largeRequest) {}, http.StatusInternalServerError},
		{"its client goes in its body", largeRequest{}, func(_ *largeServer, q largeRequest) {
			_ = q.body.(*io.PipeReader).CloseWithError(errors.New("the client went"))
		}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serveLarge(t, largeAPIConfig(slog.New(slog.DiscardHandler), 1, 50*time.Millisecond))
			q := tt.q
			if q.path == "" { // a body of a declared 2 KiB, half of it sent
				pr, pw := io.Pipe()
				go func() { _, _ = pw.Write(bytes.Repeat([]byte("a"), threshold)) }()
				q = largeRequest{token: "a", path: "/big", body: pr, length: 2 * threshold, header: map[string]string{"X-Hold": "k"}}
			}
			done := s.start(t, q)
			if got := s.do(sized("p", "/big", 2*threshold)); !isBusy(got) {
				t.Errorf("a probe while the request is on its way = %d %s (%v), want 503: it holds the slot", got.status, got.body, got.err)
			}
			tt.end(s, q)
			if got := answered(t, done); tt.status != 0 && got.status != tt.status {
				t.Errorf("the request = %d %s (%v), want %d", got.status, got.body, got.err, tt.status)
			}
			waitUntil(t, "the slots back", nil, s.api.large.idle)
			for _, token := range []string{"q", "a"} {
				if got := s.do(sized(token, "/big", 2*threshold)); got.status != http.StatusOK {
					t.Errorf("%s's next large body = %d %s (%v), want 200: the slots came back", token, got.status, got.body, got.err)
				}
			}
		})
	}
}

// A large body waits for its turn: with both slots held by other
// credentials, a third credential's waits, and takes the slot one of them
// gives back.
func TestALargeBodyWaitsForItsTurn(t *testing.T) {
	s := serveLarge(t, largeAPIConfig(slog.New(slog.DiscardHandler), 2, 10*time.Second))
	a := s.start(t, held("a", "/big", 2*threshold, "a1"))
	b := s.start(t, held("b", "/big", 2*threshold, "b1"))
	c := s.start(t, sized("c", "/big", 2*threshold))
	waitUntil(t, "c waiting", c, func() bool { return s.api.large.refs("session:c") == 1 && s.api.large.waiting.Load() == 1 })
	s.release("a1")
	answered(t, a)
	if got := answered(t, c); got.status != http.StatusOK {
		t.Errorf("the waiting large body = %d %s (%v), want 200: it took the slot given back", got.status, got.body, got.err)
	}
	s.release("b1")
	answered(t, b)
}

// The slots are taken after the failure gate, the authentication and the
// buckets: with both held, a large body without a token, with a bad one
// or of a spent bucket is 401 or 429, not the 503 of a wait: only a known
// account takes a slot (Wrap's order).
func TestTheSlotsAreTakenAfterTheBuckets(t *testing.T) {
	cfg := largeAPIConfig(slog.New(slog.DiscardHandler), 2, 50*time.Millisecond)
	buckets := newFakeLimiter(1)
	cfg.Authenticated = buckets
	s := serveLarge(t, cfg)
	a := s.start(t, held("a", "/big", 2*threshold, "a1"))
	b := s.start(t, held("b", "/big", 2*threshold, "b1"))
	buckets.Allow("session:c")
	for _, tt := range []struct {
		name   string
		token  string
		status int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"a bad token", "bad", http.StatusUnauthorized},
		{"a spent bucket", "c", http.StatusTooManyRequests},
	} {
		if got := s.do(sized(tt.token, "/big", 2*threshold)); got.status != tt.status {
			t.Errorf("%s = %d %s (%v), want %d", tt.name, got.status, got.body, got.err, tt.status)
		}
	}
	s.release("a1")
	s.release("b1")
	answered(t, a)
	answered(t, b)
}

// A route outside BodyLimits takes no slot, whatever its body's length.
func TestARouteOutsideBodyLimitsTakesNoSlot(t *testing.T) {
	cfg := largeAPIConfig(slog.New(slog.DiscardHandler), 2, 50*time.Millisecond)
	cfg.MaxBodyBytes = 2 << 20
	s := serveLarge(t, cfg)
	a := s.start(t, held("a", "/big", 2*threshold, "a1"))
	b := s.start(t, held("b", "/big", 2*threshold, "b1"))
	body := `{"name":"` + strings.Repeat("a", 1<<20) + `"}`
	if got := s.do(largeRequest{token: "c", path: "/api/v0/things", body: strings.NewReader(body), length: int64(len(body))}); got.status != http.StatusOK {
		t.Errorf("thingRoute's 1 MiB body = %d (%v), want 200 while the slots are taken", got.status, got.err)
	}
	s.release("a1")
	s.release("b1")
	answered(t, a)
	answered(t, b)
}

// NewAPI refuses slots below zero, slots without a wait, and a threshold
// of no route of BodyLimits or outside (0, its route's limit).
func TestNewAPIChecksTheLargeBodies(t *testing.T) {
	const lead = "httpserver: APIConfig: "
	for _, tt := range []struct {
		name   string
		change func(*APIConfig)
		want   string
	}{
		{"slots below zero", func(c *APIConfig) { c.LargeBodies = -1 }, "LargeBodies -1 must not be negative"},
		{"slots without a wait", func(c *APIConfig) { c.LargeBodyWait = 0 }, "LargeBodyWait must be positive when LargeBodies bounds the large bodies"},
		{"a threshold of no route of BodyLimits", func(c *APIConfig) { c.LargeBodyAbove[thingRoute] = 10 },
			`LargeBodyAbove["POST /api/v0/things"] is no route of BodyLimits`},
		{"a threshold of zero", func(c *APIConfig) { c.LargeBodyAbove[bigRoute] = 0 }, `LargeBodyAbove["POST /big"] 0 is outside (0, the route's limit 1048576)`},
		{"a threshold at the limit", func(c *APIConfig) { c.LargeBodyAbove[bigRoute] = routeLimit },
			`LargeBodyAbove["POST /big"] 1048576 is outside (0, the route's limit 1048576)`},
		{"no slots, no wait", func(c *APIConfig) { c.LargeBodies, c.LargeBodyWait = 0, 0 }, ""},
		{"a threshold below the limit", func(c *APIConfig) { c.LargeBodyAbove[bigRoute] = routeLimit - 1 }, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := largeAPIConfig(slog.New(slog.DiscardHandler), 2, time.Second)
			tt.change(&cfg)
			_, err := NewAPI(cfg)
			if tt.want == "" && err != nil || tt.want != "" && (err == nil || err.Error() != lead+tt.want) {
				t.Errorf("NewAPI() error = %v, want %q", err, tt.want)
			}
		})
	}
}
