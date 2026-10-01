package identity_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock/clocktest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/ratelimit"
)

// The module wired as bootstrap wires it, on a real database: registration,
// authentication and the caller's account through their HTTP operations.

type openSignup struct{}

func (openSignup) AllowSignup(context.Context, identity.SignupAttempt) (bool, error) {
	return true, nil
}

// testStart is the instant the tests' clocks start at, in whole
// microseconds, as timestamptz stores them.
func testStart() time.Time { return time.Date(2026, 9, 25, 10, 0, 0, 123456000, time.UTC) }

// newServer serves the module on a new database of its own.
func newServer(t *testing.T) (http.Handler, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(t)
	return newServerOn(t, pool, clocktest.At(testStart())), pool
}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// newServerOn serves the module on pool with clock, and an ephemeral
// signing key of its own: two servers on one pool are one database before
// and after a change of the key.
func newServerOn(t *testing.T, pool *pgxpool.Pool, clock *clocktest.Fixed) http.Handler {
	t.Helper()
	return newServerWith(t, pool, clock, nil)
}

// newServerWithDeadline is newServerOn with auth.refresh_deadline set to
// refreshDeadline.
func newServerWithDeadline(t *testing.T, pool *pgxpool.Pool, clock *clocktest.Fixed, refreshDeadline time.Duration) http.Handler {
	t.Helper()
	return newServerWith(t, pool, clock, func(d *identity.Deps) { d.RefreshDeadline = refreshDeadline })
}

// newServerWith is newServerOn with the module's Deps changed by change,
// when it is not nil.
func newServerWith(t *testing.T, pool *pgxpool.Pool, clock *clocktest.Fixed, change func(*identity.Deps)) http.Handler {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	limiter := ratelimit.New(time.Now)
	limit := limiter.Bucket("test", ratelimit.Rate{PerMinute: 600000, Burst: 100000})
	keys, err := identity.LoadSigningKeys(nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	deps := identity.Deps{
		Pool: pool, Tx: postgres.NewTxManager(pool, 2*time.Second), Clock: clock, Logger: logger, SigningKeys: keys,
		SignupPolicy: openSignup{}, AccessTokenTTL: 15 * time.Minute, SessionTTL: 720 * time.Hour, RefreshDeadline: 4 * time.Second,
		Password:   testPassword(),
		RateLimits: identity.RateLimits{Limiter: limiter, LoginIP: limit, LoginIPEmail: limit, RegisterIP: limit, PasswordUser: limit},
	}
	if change != nil {
		change(&deps)
	}
	m, err := identity.New(deps)
	if err != nil {
		t.Fatal(err)
	}
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{
		Authenticator: m.Authenticator(), PublicOperations: m.PublicOperations(), RequestTimeouts: m.RequestTimeouts(),
	})
	router := httpserver.NewRouter(logger)
	m.Register(router, api)
	return router
}

// testPassword is the test profile's argon2id: cheap.
func testPassword() identity.PasswordHashing {
	return identity.PasswordHashing{MemoryKiB: 64, Iterations: 1, Parallelism: 1, MaxConcurrent: 4, MaxWait: 2 * time.Second}
}

type authTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func register(h http.Handler, email string) (*httptest.ResponseRecorder, authTokens) {
	req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/register",
		strings.NewReader(`{"email":"`+email+`","password":"correct horse battery"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "agent/1")
	req.RemoteAddr = "203.0.113.7:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var tokens authTokens
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	return rec, tokens
}

func getMe(h http.Handler, accessToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v0/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRegisterThenReadTheAccount(t *testing.T) {
	h, pool := newServer(t)

	rec, tokens := register(h, " Alice@Corp.COM ")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register = %d %s, want 201", rec.Code, rec.Body)
	}
	me := getMe(h, tokens.AccessToken)

	var user struct {
		ID              string   `json:"id"`
		Email           string   `json:"email"`
		DisplayName     string   `json:"display_name"`
		OnboardingSteps []string `json:"onboarding_steps"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &user); err != nil || me.Code != http.StatusOK ||
		user.Email != "alice@corp.com" || user.DisplayName != "alice" || user.OnboardingSteps == nil || len(user.OnboardingSteps) != 0 {
		t.Fatalf("GET /me = %d %s, want alice@corp.com, alice and no step", me.Code, me.Body)
	}

	// The session row: generation 0, the client, 30 days from the sign-in,
	// and the hash of the refresh token's secret.
	var generation int
	var userAgent, ip string
	var lifetime time.Duration
	var tokenHash []byte
	err := pool.QueryRow(context.Background(), `SELECT generation, user_agent, host(ip), expires_at - created_at, token_hash
		FROM auth_sessions WHERE user_id = $1`, user.ID).Scan(&generation, &userAgent, &ip, &lifetime, &tokenHash)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(tokens.RefreshToken, "nwk_rt_"))
	secretHash := sha256.Sum256(raw[20:52])
	if generation != 0 || userAgent != "agent/1" || ip != "203.0.113.7" || lifetime != 720*time.Hour || string(tokenHash) != string(secretHash[:]) {
		t.Errorf("session = generation %d, %q, %s, lasting %v, hash %x; want 0, agent/1, 203.0.113.7, 720h, %x",
			generation, userAgent, ip, lifetime, tokenHash, secretHash)
	}
}

// Of concurrent registrations of one address, in any case, exactly one
// succeeds: the unique constraint decides, and the others answer 409 with
// nothing written.
func TestConcurrentRegistrationsOfOneAddress(t *testing.T) {
	h, pool := newServer(t)
	emails := []string{"bob@corp.com", "Bob@corp.com", "BOB@CORP.COM", " bob@Corp.com", "bob@corp.COM", "bOb@corp.com"}

	var wg sync.WaitGroup
	codes := make([]int, len(emails))
	for i, email := range emails {
		wg.Go(func() {
			rec, _ := register(h, email)
			codes[i] = rec.Code
		})
	}
	wg.Wait()

	created, conflicts := 0, 0
	for _, code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		}
	}
	var users, sessions int
	if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM auth_sessions)`).Scan(&users, &sessions); err != nil {
		t.Fatal(err)
	}
	if created != 1 || conflicts != len(emails)-1 || users != 1 || sessions != 1 {
		t.Errorf("answers %v, %d users, %d sessions; want one 201, the rest 409, one of each row", codes, users, sessions)
	}
}

// A valid access token of a session that can no longer be used answers 401
// with the invalid_token challenge; the database decides, row by row.
func TestAuthenticationChecksTheSessionAndTheAccount(t *testing.T) {
	tests := []struct {
		name   string
		update string // run with the account's id as $1
	}{
		{"revoked session", `UPDATE auth_sessions SET revoked_at = created_at, revoke_reason = 'logout' WHERE user_id = $1`},
		{"expired session", `UPDATE auth_sessions SET expires_at = created_at WHERE user_id = $1`},
		{"deactivated account", `UPDATE users SET is_active = false WHERE id = $1`},
		{"deleted account", `DELETE FROM users WHERE id = $1`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, pool := newServer(t)
			_, tokens := register(h, "carol@corp.com")
			var userID string
			if err := pool.QueryRow(context.Background(), `SELECT id FROM users`).Scan(&userID); err != nil {
				t.Fatal(err)
			}
			if rec := getMe(h, tokens.AccessToken); rec.Code != http.StatusOK {
				t.Fatalf("GET /me before = %d, want 200", rec.Code)
			}
			if _, err := pool.Exec(context.Background(), tt.update, userID); err != nil {
				t.Fatal(err)
			}

			rec := getMe(h, tokens.AccessToken)

			if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != `Bearer error="invalid_token"` {
				t.Errorf("GET /me = %d, WWW-Authenticate %q; want 401 with the invalid_token challenge", rec.Code, rec.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

// Another instance's key, or this one's after a restart without a key file,
// signs tokens this module does not accept.
func TestATokenOfAnotherKeyIsRefused(t *testing.T) {
	h, _ := newServer(t)
	other, _ := newServer(t)
	_, tokens := register(other, "dave@corp.com")

	if rec := getMe(h, tokens.AccessToken); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /me with another key's token = %d, want 401", rec.Code)
	}
}
