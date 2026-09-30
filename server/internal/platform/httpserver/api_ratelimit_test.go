package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"
)

// request is a POST of a valid body to path with token, none when empty,
// from 203.0.113.7.
func request(path, token string) *http.Request {
	r := post(path, `{"name":"a"}`)
	r.Header.Del("Authorization")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

// A request takes a unit of anonymous by its IP key when it carries no
// credential, else of authenticated by its credential (M1/P2 design 3.2). A
// request that authentication turns away takes neither.
func TestRateLimitPicksTheBucketAndKey(t *testing.T) {
	tests := []struct {
		name, path, token string
		status            int
		anonymous         []string
		authenticated     []string
	}{
		{"public operation", "/api/v0/open", "", 204, []string{"203.0.113.7"}, nil},
		{"public operation with a token", "/api/v0/open", "tok", 204, []string{"203.0.113.7"}, nil},
		{"authenticated", "/api/v0/things", "tok", 204, nil, []string{"session:tok"}},
		{"no token", "/api/v0/things", "", 401, nil, nil},
		{"failed credential", "/api/v0/things", "bad", 401, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
			anonymous, authenticated := newFakeLimiter(5), newFakeLimiter(5)
			cfg.Anonymous, cfg.Authenticated = anonymous, authenticated
			router, _ := mount(buildAPI(t, cfg))

			rec := serve(router, request(tt.path, tt.token))

			if rec.Code != tt.status || !slices.Equal(anonymous.taken, tt.anonymous) || !slices.Equal(authenticated.taken, tt.authenticated) {
				t.Errorf("response %d, anonymous took %q, authenticated took %q; want %d, %q, %q",
					rec.Code, anonymous.taken, authenticated.taken, tt.status, tt.anonymous, tt.authenticated)
			}
		})
	}
}

// Over its rate, a request gets 429 with Retry-After in whole seconds,
// rounded up, before its body is looked at; the log names the bucket.
func TestRateLimitedRequestIs429(t *testing.T) {
	tests := []struct {
		name, path, token, bucket string
	}{
		{"anonymous", "/api/v0/open", "", "anonymous"},
		{"authenticated", "/api/v0/things", "tok", "authenticated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logs := captureLogs(t)
			cfg := testAPIConfig(&fakeAuth{}, logger)
			cfg.Anonymous, cfg.Authenticated = newFakeLimiter(0), newFakeLimiter(0)
			router, got := mount(buildAPI(t, cfg))
			req := request(tt.path, tt.token)
			req.Body = http.NoBody // the body check would answer 400

			rec := serve(router, req)

			want := `{"status":429,"code":"rate_limited","title":"Too Many Requests","detail":"Too many requests; retry later."}` + "\n"
			retry := rec.Result().Header.Get("Retry-After")
			if rec.Code != http.StatusTooManyRequests || rec.Body.String() != want || retry != "2" || got.called {
				t.Errorf("response = %d %s, Retry-After %q, handler called %v; want 429 %s, Retry-After 2",
					rec.Code, rec.Body, retry, got.called, want)
			}
			entry := findLog(logs(), "rate limited")
			if entry == nil || entry["level"] != "DEBUG" || entry["bucket"] != tt.bucket || entry["ip"] != "203.0.113.7" {
				t.Errorf("log = %v, want bucket %s and the client at debug level", entry, tt.bucket)
			}
		})
	}
}

// The failure gate reserves a unit before the authenticator runs; only a
// credential that fails keeps it (M1/P2 design 3.3).
func TestFailureGateKeepsTheUnitOfAFailedCredentialOnly(t *testing.T) {
	tests := []struct {
		name, path, token string
		status            int
		left              int // of the client's 3 units afterwards
		refunded          bool
	}{
		{"failed credential", "/api/v0/things", "bad", 401, 2, false},
		{"valid credential", "/api/v0/things", "tok", 204, 3, true},
		{"expired access token", "/api/v0/things", "expired", 401, 3, true},
		{"authenticator fault", "/api/v0/things", "boom", 500, 3, true},
		{"no token", "/api/v0/things", "", 401, 3, false},
		{"public operation", "/api/v0/open", "bad", 204, 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
			gate := newFakeLimiter(3)
			cfg.AuthFailure = gate
			router, _ := mount(buildAPI(t, cfg))

			rec := serve(router, request(tt.path, tt.token))

			if rec.Code != tt.status || gate.left("203.0.113.7") != tt.left || (len(gate.refunds) == 1) != tt.refunded {
				t.Errorf("response %d, units left %d, refunds %q; want %d, %d, refunded %v",
					rec.Code, gate.left("203.0.113.7"), gate.refunds, tt.status, tt.left, tt.refunded)
			}
		})
	}
}

// An empty gate answers 429 without running the authenticator, for every
// token from that client, valid ones too; other clients are not affected.
func TestFailureGateTurnsAwayWithoutAuthenticating(t *testing.T) {
	logger, logs := captureLogs(t)
	auth := &fakeAuth{}
	cfg := testAPIConfig(auth, logger)
	cfg.AuthFailure = newFakeLimiter(2)
	router, got := mount(buildAPI(t, cfg))
	for range 2 {
		serve(router, request("/api/v0/things", "bad"))
	}

	for _, token := range []string{"bad", "tok"} {
		rec := serve(router, request("/api/v0/things", token))

		retry := rec.Result().Header.Get("Retry-After")
		if p := decodeProblem(t, rec); rec.Code != http.StatusTooManyRequests || p.Code != CodeRateLimited || retry != "2" {
			t.Errorf("token %s: response = %d %+v, Retry-After %q; want 429 rate_limited, Retry-After 2", token, rec.Code, p, retry)
		}
	}
	if auth.calls != 2 || got.called {
		t.Errorf("authenticator called %d times, handler called %v; want 2 and false", auth.calls, got.called)
	}
	entry := findLog(logs(), "rate limited")
	if entry == nil || entry["level"] != "DEBUG" || entry["bucket"] != "auth_failure" || entry["ip"] != "203.0.113.7" {
		t.Errorf("log = %v, want the auth_failure bucket and the client at debug level", entry)
	}

	other := request("/api/v0/things", "tok")
	other.RemoteAddr = "198.51.100.1:5555"
	if rec := serve(router, other); rec.Code != http.StatusNoContent {
		t.Errorf("another client: status = %d, want 204", rec.Code)
	}
}

// Reserving before authenticating holds concurrent failures to the burst:
// 50 invalid tokens at once, 3 units, 3 authentications.
func TestFailureGateHoldsUnderConcurrency(t *testing.T) {
	auth := &slowFailingAuth{}
	cfg := testAPIConfig(auth, slog.New(slog.DiscardHandler))
	cfg.AuthFailure = newFakeLimiter(3)
	router, _ := mount(buildAPI(t, cfg))

	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for range 50 {
		wg.Go(func() {
			rec := serve(router, request("/api/v0/things", "bad"))
			mu.Lock()
			defer mu.Unlock()
			codes[rec.Code]++
		})
	}
	wg.Wait()

	if auth.calls() != 3 || codes[http.StatusUnauthorized] != 3 || codes[http.StatusTooManyRequests] != 47 {
		t.Errorf("authenticator called %d times, responses %v; want 3, three 401 and 47 429", auth.calls(), codes)
	}
}

// slowFailingAuth rejects every token, slowly enough that concurrent
// requests are inside it together: a gate that took its unit only after
// authenticating would let them all through.
type slowFailingAuth struct {
	mu sync.Mutex
	n  int
}

func (s *slowFailingAuth) Authenticate(context.Context, string) (context.Context, string, error) {
	s.mu.Lock()
	s.n++
	s.mu.Unlock()
	time.Sleep(20 * time.Millisecond)
	return nil, "", problemErr{status: http.StatusUnauthorized, code: "unauthorized", detail: "no such session"}
}

func (s *slowFailingAuth) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.n
}

// The gate counts a client by its IP key: two addresses of one IPv6 /64 are
// one client.
func TestFailureGateCountsByTheIPKey(t *testing.T) {
	cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
	gate := newFakeLimiter(1)
	cfg.AuthFailure = gate
	router, _ := mount(buildAPI(t, cfg))
	first := request("/api/v0/things", "bad")
	first.RemoteAddr = "[2001:db8:1:2::7]:443"
	second := request("/api/v0/things", "bad")
	second.RemoteAddr = "[2001:db8:1:2::8]:443"

	serve(router, first)
	rec := serve(router, second)

	if rec.Code != http.StatusTooManyRequests || len(gate.taken) != 1 || gate.taken[0] != "2001:db8:1:2::/64" {
		t.Errorf("second address: status %d, units taken for %q; want 429 and one unit of 2001:db8:1:2::/64", rec.Code, gate.taken)
	}
}
