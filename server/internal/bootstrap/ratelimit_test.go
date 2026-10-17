package bootstrap

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The platform's buckets are sized by the configuration (M1/P2 design 3.2,
// 3.3): anonymous for the public operations, authenticated for the rest,
// auth_failure before authentication. Each answers 429 rate_limited with
// Retry-After once empty, as the contract declares.
func TestTheConfiguredBucketsLimitTheWholeApp(t *testing.T) {
	contract := apitest.Load(t)
	cfg := testConfig(t, pgtest.NewDatabase(t), true)
	cfg.RateLimit.Anonymous = config.BucketConfig{PerMinute: 1, Burst: 2}
	cfg.RateLimit.Authenticated = config.BucketConfig{PerMinute: 1, Burst: 2}
	cfg.RateLimit.AuthFailure = config.BucketConfig{PerMinute: 1, Burst: 1}
	base := startApp(t, cfg, migrations.FS())
	tokens := registerAccount(t, contract, base, "alice@example.com") // anonymous: 1 of 2

	steps := []struct {
		name, method, path, token string
		want                      int
	}{
		{"authenticated", http.MethodGet, "/api/v0/me", tokens.AccessToken, http.StatusOK},
		{"authenticated", http.MethodGet, "/api/v0/me", tokens.AccessToken, http.StatusOK},
		{"authenticated, empty", http.MethodGet, "/api/v0/me", tokens.AccessToken, http.StatusTooManyRequests},
		{"a failed credential", http.MethodGet, "/api/v0/me", "not-a-token", http.StatusUnauthorized},
		{"auth_failure, empty", http.MethodGet, "/api/v0/me", "not-a-token", http.StatusTooManyRequests},
		{"anonymous", http.MethodGet, "/api/v0/instance", "", http.StatusOK},
		{"anonymous, empty", http.MethodGet, "/api/v0/instance", "", http.StatusTooManyRequests},
	}
	for _, s := range steps {
		req := newRequest(t, s.method, base+s.path, s.token, nil)
		res, body := sendRequest(t, req)
		contract.CheckResponse(t, req, res)

		if res.StatusCode != s.want {
			t.Errorf("%s: %s %s = %d %s, want %d", s.name, s.method, s.path, res.StatusCode, body, s.want)
		}
		// A unit a minute: 60 seconds from the first unit taken, rounded up;
		// 59 once a second has passed since (M1/P2 review N2).
		if retry := res.Header.Get("Retry-After"); s.want == http.StatusTooManyRequests && retry != "60" && retry != "59" {
			t.Errorf("%s: Retry-After %q, want 59 or 60 (a unit a minute)", s.name, retry)
		}
	}
}

// The module's buckets are sized by the configuration too: login_ip and
// register_ip each refuse the client once empty.
func TestTheConfiguredModuleBucketsLimitSignInAndSignUp(t *testing.T) {
	contract := apitest.Load(t)
	cfg := testConfig(t, pgtest.NewDatabase(t), true)
	cfg.RateLimit.RegisterIP = config.BucketConfig{PerMinute: 1, Burst: 1}
	cfg.RateLimit.LoginIP = config.BucketConfig{PerMinute: 1, Burst: 1}
	base := startApp(t, cfg, migrations.FS())
	registerAccount(t, contract, base, "alice@example.com")

	steps := []struct {
		name, path, body string
		want             int
	}{
		{"register_ip, empty", "/api/v0/auth/register", `{"email":"bob@example.com","password":"Tr0ub4dor&3"}`, http.StatusTooManyRequests},
		{"login_ip", "/api/v0/auth/login", `{"email":"alice@example.com","password":"Tr0ub4dor&3"}`, http.StatusOK},
		{"login_ip, empty", "/api/v0/auth/login", `{"email":"alice@example.com","password":"Tr0ub4dor&3"}`, http.StatusTooManyRequests},
	}
	for _, s := range steps {
		req := newRequest(t, http.MethodPost, base+s.path, "", []byte(s.body))
		res, body := sendRequest(t, req)
		contract.CheckResponse(t, req, res)

		if res.StatusCode != s.want {
			t.Errorf("%s: POST %s = %d %s, want %d", s.name, s.path, res.StatusCode, body, s.want)
		}
	}
}

// /mcp and OAuth's registration take the platform's buckets as REST does
// (M9 design 4.4; 13.1 item 20): each POST to /mcp a unit of the personal
// access token's authenticated bucket, a token that fails a unit of
// auth_failure, the registration a unit of anonymous; each answers 429
// rate_limited with Retry-After once empty, never stored, before the SDK.
func TestTheMCPRoutesTakeTheBuckets(t *testing.T) {
	contract := apitest.Load(t)
	cfg := testConfig(t, pgtest.NewDatabase(t), true)
	cfg.RateLimit.Anonymous = config.BucketConfig{PerMinute: 1, Burst: 2}
	cfg.RateLimit.Authenticated = config.BucketConfig{PerMinute: 1, Burst: 2}
	cfg.RateLimit.AuthFailure = config.BucketConfig{PerMinute: 1, Burst: 1}
	base := startApp(t, cfg, migrations.FS())
	access := registerAccount(t, contract, base, "alice@example.com").AccessToken // anonymous: 1 of 2
	m := &mcpApp{base: base}
	m.pat = createToken(t, contract, base, access) // the session's bucket, not the token's
	steps := []struct {
		name, path, token, body string
		want                    int
	}{
		{"the token's bucket", "/mcp", m.pat, initializeBody(mcpVersion), http.StatusOK},
		{"the token's bucket", "/mcp", m.pat, initializeBody(mcpVersion), http.StatusOK},
		{"the token's bucket, empty", "/mcp", m.pat, initializeBody(mcpVersion), http.StatusTooManyRequests},
		{"a token that fails", "/mcp", "nwk_pat_" + strings.Repeat("x", 43), initializeBody(mcpVersion), http.StatusUnauthorized},
		{"auth_failure, empty", "/mcp", "nwk_pat_" + strings.Repeat("x", 43), initializeBody(mcpVersion), http.StatusTooManyRequests},
		{"the registration", "/register", "", "{}", http.StatusBadRequest},
		{"the registration, anonymous empty", "/register", "", "{}", http.StatusTooManyRequests},
	}
	for _, s := range steps {
		a := m.post(t, context.Background(), s.path, s.token, s.body, nil)
		if a.status != s.want || a.header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s: POST %s = %d %s, Cache-Control %q; want %d, no-store", s.name, s.path, a.status, a.body, a.header.Get("Cache-Control"), s.want)
		}
		if s.want == http.StatusTooManyRequests && (a.code() != "rate_limited" || a.header.Get("Retry-After") == "") {
			t.Errorf("%s: POST %s = %s, Retry-After %q; want rate_limited with Retry-After", s.name, s.path, a.body, a.header.Get("Retry-After"))
		}
	}
}
