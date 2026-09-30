package identity_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	argon2adapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/argon2"
)

// login signs email in with password from 198.51.100.7.
func login(h http.Handler, email, password string) (*httptest.ResponseRecorder, authTokens) {
	req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/login",
		strings.NewReader(`{"email":"`+email+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "agent/2")
	req.RemoteAddr = "198.51.100.7:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var tokens authTokens
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	return rec, tokens
}

// A sign-in starts a session of its own, beside the registration's, and
// its access token reads the account.
func TestLoginStartsASession(t *testing.T) {
	h, pool := newServer(t)
	register(h, "alice@corp.com")

	rec, tokens := login(h, " Alice@Corp.com", "correct horse battery")

	if rec.Code != http.StatusOK || getMe(h, tokens.AccessToken).Code != http.StatusOK {
		t.Fatalf("login = %d %s, want 200 with a working access token", rec.Code, rec.Body)
	}
	var sessions int
	var userAgent, ip string
	err := pool.QueryRow(context.Background(), `SELECT count(*) OVER (), user_agent, host(ip) FROM auth_sessions
		ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&sessions, &userAgent, &ip)
	if err != nil {
		t.Fatal(err)
	}
	if sessions != 2 || userAgent != "agent/2" || ip != "198.51.100.7" {
		t.Errorf("sessions = %d, the newest from %q at %s; want 2, agent/2 at 198.51.100.7", sessions, userAgent, ip)
	}
}

// A wrong password and an unknown address get the very same answer
// (M1/P2 design 3.4); neither writes anything.
func TestLoginFailsAlikeForAWrongPasswordAndAnUnknownAddress(t *testing.T) {
	h, pool := newServer(t)
	register(h, "alice@corp.com")

	wrong, _ := login(h, "alice@corp.com", "not the password")
	unknown, _ := login(h, "nobody@corp.com", "correct horse battery")

	if wrong.Code != http.StatusUnauthorized || unknown.Code != wrong.Code || unknown.Body.String() != wrong.Body.String() ||
		unknown.Header().Get("WWW-Authenticate") != wrong.Header().Get("WWW-Authenticate") {
		t.Errorf("wrong password = %d %s, unknown address = %d %s; want the same 401", wrong.Code, wrong.Body, unknown.Code, unknown.Body)
	}
	if !strings.Contains(wrong.Body.String(), `"code":"identity.invalid_credentials"`) {
		t.Errorf("body = %s, want identity.invalid_credentials", wrong.Body)
	}
	var sessions int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM auth_sessions`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 {
		t.Errorf("sessions = %d, want only the registration's", sessions)
	}
}

// A deactivated account answers identity.account_deactivated to its
// password only.
func TestLoginOfADeactivatedAccount(t *testing.T) {
	h, pool := newServer(t)
	register(h, "alice@corp.com")
	if _, err := pool.Exec(context.Background(), `UPDATE users SET is_active = false`); err != nil {
		t.Fatal(err)
	}

	wrong, _ := login(h, "alice@corp.com", "not the password")
	right, _ := login(h, "alice@corp.com", "correct horse battery")

	if wrong.Code != http.StatusUnauthorized || right.Code != http.StatusForbidden ||
		!strings.Contains(right.Body.String(), `"code":"identity.account_deactivated"`) {
		t.Errorf("wrong password = %d, right password = %d %s; want 401, then 403 identity.account_deactivated", wrong.Code, right.Code, right.Body)
	}
}

// A hash of other parameters is replaced at the next sign-in; concurrent
// sign-ins all succeed, whichever writes the new hash (the loser verifies
// again against it, M1/P2 design 3.4).
func TestLoginRehashesAnOldHash(t *testing.T) {
	h, pool := newServer(t)
	register(h, "alice@corp.com")
	old := argon2adapter.New(argon2adapter.Params{MemoryKiB: 32, Iterations: 2, Parallelism: 1, MaxConcurrent: 1, MaxWait: time.Second},
		slog.New(slog.DiscardHandler))
	oldHash, err := old.Hash(context.Background(), "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE users SET password = $1`, oldHash); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	codes := make([]int, 4)
	for i := range codes {
		wg.Go(func() {
			rec, _ := login(h, "alice@corp.com", "correct horse battery")
			codes[i] = rec.Code
		})
	}
	wg.Wait()

	var hash string
	var sessions int
	if err := pool.QueryRow(context.Background(), `SELECT password, (SELECT count(*) FROM auth_sessions) FROM users`).Scan(&hash, &sessions); err != nil {
		t.Fatal(err)
	}
	for _, code := range codes {
		if code != http.StatusOK {
			t.Errorf("concurrent sign-ins answered %v, want 200 each", codes)
			break
		}
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=64,t=1,p=1$") || sessions != 5 {
		t.Errorf("hash %s, %d sessions; want the current parameters (m=64,t=1,p=1) and 5 sessions", hash, sessions)
	}
}
