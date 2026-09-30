package identity_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/clock/clocktest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The session rotation of M1/P2 design 3.5 and the logout of 3.6 on a real
// database, through the HTTP operations.

// postToken posts {"refresh_token": token} to path.
func postToken(h http.Handler, path, token string) (*httptest.ResponseRecorder, authTokens) {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"refresh_token":"`+token+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.7:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var tokens authTokens
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	return rec, tokens
}

func refresh(h http.Handler, token string) (*httptest.ResponseRecorder, authTokens) {
	return postToken(h, "/api/v0/auth/refresh", token)
}

// sessionRow is the one session's generation, revoke reason and
// last_refreshed_at.
type sessionRow struct {
	generation    int
	reason        *string
	lastRefreshed *time.Time
}

func onlySession(t *testing.T, pool *pgxpool.Pool) sessionRow {
	t.Helper()
	var s sessionRow
	err := pool.QueryRow(context.Background(), `SELECT generation, revoke_reason, last_refreshed_at FROM auth_sessions`).
		Scan(&s.generation, &s.reason, &s.lastRefreshed)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// forgedOlder is generation g of token's session with a random secret and
// tag: what someone who saw only the session id could make.
func forgedOlder(t *testing.T, token string, g uint32) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, "nwk_rt_"))
	if err != nil || len(raw) != 68 {
		t.Fatalf("refresh token %q: %v", token, err)
	}
	binary.BigEndian.PutUint32(raw[16:20], g)
	_, _ = rand.Read(raw[20:68])
	return "nwk_rt_" + base64.RawURLEncoding.EncodeToString(raw)
}

// A refresh rotates the session; the retired token, used again, revokes
// it, and every token of it fails from then on (A5).
func TestRefreshRotatesAndCatchesReuse(t *testing.T) {
	h, pool := newServer(t)
	_, first := register(h, "alice@corp.com")

	rec, second := refresh(h, first.RefreshToken)
	if rec.Code != http.StatusOK || second.RefreshToken == first.RefreshToken || getMe(h, second.AccessToken).Code != http.StatusOK {
		t.Fatalf("refresh = %d %s, want 200 with new, working tokens", rec.Code, rec.Body)
	}
	if s := onlySession(t, pool); s.generation != 1 || s.reason != nil || s.lastRefreshed == nil || !s.lastRefreshed.Equal(testStart()) {
		t.Errorf("session = %+v, want generation 1, live, refreshed at the clock's now", s)
	}

	reused, _ := refresh(h, first.RefreshToken)

	if reused.Code != http.StatusUnauthorized || !strings.Contains(reused.Body.String(), `"code":"identity.refresh_token_invalid"`) {
		t.Errorf("reuse = %d %s, want 401 identity.refresh_token_invalid", reused.Code, reused.Body)
	}
	if s := onlySession(t, pool); s.reason == nil || *s.reason != "reuse_detected" {
		t.Errorf("session = %+v, want revoked for reuse_detected", s)
	}
	if rec, _ := refresh(h, second.RefreshToken); rec.Code != http.StatusUnauthorized {
		t.Errorf("the current token after the reuse = %d, want 401", rec.Code)
	}
	if rec := getMe(h, second.AccessToken); rec.Code != http.StatusUnauthorized {
		t.Errorf("the access token after the reuse = %d, want 401", rec.Code)
	}
}

// A forged older generation, knowing only the session id, answers 401 and
// cannot end someone else's session (A5).
func TestAForgedOlderGenerationRevokesNothing(t *testing.T) {
	h, pool := newServer(t)
	_, first := register(h, "alice@corp.com")
	_, second := refresh(h, first.RefreshToken)

	rec, _ := refresh(h, forgedOlder(t, second.RefreshToken, 0))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("forged token = %d, want 401", rec.Code)
	}
	if s := onlySession(t, pool); s.generation != 1 || s.reason != nil {
		t.Errorf("session = %+v, want it untouched at generation 1", s)
	}
	if rec, _ := refresh(h, second.RefreshToken); rec.Code != http.StatusOK {
		t.Errorf("the current token after the forgery = %d, want 200", rec.Code)
	}
}

// After a change of the signing key (another server on the same database)
// the current generation still rotates, its stored hash proves it; an
// older one no longer counts as issued, so it no longer revokes.
func TestRefreshAcrossAChangeOfTheSigningKey(t *testing.T) {
	pool := newPool(t)
	before := newServerOn(t, pool, clocktest.At(testStart()))
	_, first := register(before, "alice@corp.com")
	_, second := refresh(before, first.RefreshToken)
	after := newServerOn(t, pool, clocktest.At(testStart()))

	old, _ := refresh(after, first.RefreshToken)
	current, third := refresh(after, second.RefreshToken)

	if old.Code != http.StatusUnauthorized || current.Code != http.StatusOK || getMe(after, third.AccessToken).Code != http.StatusOK {
		t.Errorf("older generation = %d, current = %d %s; want 401, then 200 with working tokens", old.Code, current.Code, current.Body)
	}
	if s := onlySession(t, pool); s.generation != 2 || s.reason != nil {
		t.Errorf("session = %+v, want generation 2, not revoked", s)
	}
}

// Two refreshes with one token read the session at once: both judge the
// rotation. The conditional UPDATE lets one through; the other misses, reads
// the row again, finds an older generation the session issued, and revokes
// the session for reuse (M1/P2 design 3.5). The order is forced: the test
// holds the session's row until both UPDATEs wait for it.
func TestConcurrentRefreshesOfOneToken(t *testing.T) {
	ctx := context.Background()
	h, pool := newServer(t)
	_, tokens := register(h, "alice@corp.com")
	held, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Rollback(ctx) }()
	if _, err := held.Exec(ctx, `SELECT 1 FROM auth_sessions FOR UPDATE`); err != nil {
		t.Fatal(err)
	}

	answers := make(chan int, 2)
	for range 2 {
		go func() {
			rec, _ := refresh(h, tokens.RefreshToken)
			answers <- rec.Code
		}()
	}
	pgtest.WaitForLockWaits(t, pool, 2, 10*time.Second)
	if err := held.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	codes := []int{<-answers, <-answers}

	slices.Sort(codes)
	s := onlySession(t, pool)
	if !slices.Equal(codes, []int{http.StatusOK, http.StatusUnauthorized}) || s.generation != 1 || s.reason == nil || *s.reason != "reuse_detected" {
		t.Errorf("answers %v, session %+v; want one 200 and one 401, generation 1, revoked for reuse_detected", codes, s)
	}
}

// A session ends at its expiry, however recently it was refreshed.
func TestRefreshEndsWithTheSession(t *testing.T) {
	pool := newPool(t)
	clock := clocktest.At(testStart())
	h := newServerOn(t, pool, clock)
	_, first := register(h, "alice@corp.com")
	clock.Advance(720*time.Hour - time.Minute)
	_, second := refresh(h, first.RefreshToken)
	clock.Advance(time.Minute)

	rec, _ := refresh(h, second.RefreshToken)

	if second.RefreshToken == "" || rec.Code != http.StatusUnauthorized {
		t.Errorf("refresh a minute before the end = %q, at the end = %d; want tokens, then 401", second.RefreshToken, rec.Code)
	}
	if s := onlySession(t, pool); s.reason != nil {
		t.Errorf("session = %+v, want it expired, not revoked", s)
	}
}

// Logout ends the session of the current token: its refresh token and its
// access token stop working. Any other token changes nothing, with the same
// answer (A6).
func TestLogoutEndsTheSession(t *testing.T) {
	h, pool := newServer(t)
	_, first := register(h, "alice@corp.com")
	_, second := refresh(h, first.RefreshToken)

	older, _ := postToken(h, "/api/v0/auth/logout", first.RefreshToken)
	garbage, _ := postToken(h, "/api/v0/auth/logout", "not a token")
	if s := onlySession(t, pool); older.Code != http.StatusNoContent || garbage.Code != http.StatusNoContent || s.reason != nil {
		t.Fatalf("logout with an older token = %d, with garbage = %d, session %+v; want 204 twice and the session live", older.Code, garbage.Code, s)
	}

	rec, _ := postToken(h, "/api/v0/auth/logout", second.RefreshToken)

	if s := onlySession(t, pool); rec.Code != http.StatusNoContent || s.reason == nil || *s.reason != "logout" {
		t.Errorf("logout = %d, session %+v; want 204 and revoked for logout", rec.Code, s)
	}
	if rec, _ := refresh(h, second.RefreshToken); rec.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout = %d, want 401", rec.Code)
	}
	if rec := getMe(h, second.AccessToken); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /me after logout = %d, want 401", rec.Code)
	}
}
