package bootstrap

import (
	"net/http"
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
		if retry := res.Header.Get("Retry-After"); s.want == http.StatusTooManyRequests && retry != "60" {
			t.Errorf("%s: Retry-After %q, want 60 (a unit a minute)", s.name, retry)
		}
	}
}
