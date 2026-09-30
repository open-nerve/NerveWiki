package identity_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock/clocktest"
)

// Personal access tokens authenticating on a real database (M1/P3 design
// 3.2).

// insertToken stores a token of the only account, expiring at expires (nil:
// never), and returns it with its id.
func insertToken(t *testing.T, pool *pgxpool.Pool, expires *time.Time) (string, uuid.UUID) {
	t.Helper()
	var userID uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM users`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var pat domain.PAT
	copy(pat[:], uuid.NewV7().String())
	id := uuid.NewV7()
	err := postgresadapter.New(pool).CreateAPIToken(context.Background(), app.NewAPIToken{
		ID: id, UserID: userID, TokenHash: pat.Hash(), Name: "agent", ExpiresAt: expires, Now: testStart(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return pat.String(), id
}

func lastUsed(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	if err := pool.QueryRow(context.Background(), `SELECT last_used_at FROM api_tokens WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

// A token authenticates as its account; its use is recorded at most once a
// minute.
func TestAPersonalAccessTokenAuthenticates(t *testing.T) {
	pool := newPool(t)
	clock := clocktest.At(testStart())
	h := newServerOn(t, pool, clock)
	register(h, "alice@corp.com")
	token, id := insertToken(t, pool, nil)

	if rec := getMe(h, token); rec.Code != http.StatusOK {
		t.Fatalf("GET /me with the token = %d %s, want 200", rec.Code, rec.Body)
	}
	if at := lastUsed(t, pool, id); at == nil || !at.Equal(testStart()) {
		t.Errorf("last_used_at = %v, want %v", at, testStart())
	}
	clock.Advance(time.Minute)
	getMe(h, token)
	if at := lastUsed(t, pool, id); !at.Equal(testStart()) {
		t.Errorf("last_used_at a minute later = %v, want it still %v", at, testStart())
	}
	clock.Advance(time.Second)
	getMe(h, token)
	if at := lastUsed(t, pool, id); !at.Equal(testStart().Add(time.Minute + time.Second)) {
		t.Errorf("last_used_at past the minute = %v, want it written again", at)
	}
}

// A token stops at its expiry, at its revocation, and while its account is
// deactivated.
func TestAPersonalAccessTokenStops(t *testing.T) {
	for _, tt := range []struct {
		name string
		stop func(t *testing.T, pool *pgxpool.Pool, clock *clocktest.Fixed)
	}{
		{"at its expiry", func(_ *testing.T, _ *pgxpool.Pool, clock *clocktest.Fixed) { clock.Advance(time.Hour) }},
		{"revoked", func(t *testing.T, pool *pgxpool.Pool, _ *clocktest.Fixed) {
			execSQL(t, pool, `UPDATE api_tokens SET revoked_at = now()`)
		}},
		{"with its account deactivated", func(t *testing.T, pool *pgxpool.Pool, _ *clocktest.Fixed) {
			execSQL(t, pool, `UPDATE users SET is_active = false`)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pool := newPool(t)
			clock := clocktest.At(testStart())
			h := newServerOn(t, pool, clock)
			register(h, "alice@corp.com")
			expires := testStart().Add(time.Hour)
			token, _ := insertToken(t, pool, &expires)
			if rec := getMe(h, token); rec.Code != http.StatusOK {
				t.Fatalf("GET /me before = %d, want 200", rec.Code)
			}

			tt.stop(t, pool, clock)

			if rec := getMe(h, token); rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != `Bearer error="invalid_token"` {
				t.Errorf("GET /me after = %d, WWW-Authenticate %q; want 401 with invalid_token", rec.Code, rec.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func execSQL(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Fatal(err)
	}
}

// send makes a request with a bearer token and a JSON body ("" for none).
func send(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "203.0.113.7:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// createdToken is the answer of createApiToken.
type createdToken struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	ExpiresAt *string `json:"expires_at"`
	Token     string  `json:"token"`
}

// createToken creates a token named name with credential, the password
// being register's.
func createToken(t *testing.T, h http.Handler, credential, name string) createdToken {
	t.Helper()
	rec := send(h, http.MethodPost, "/api/v0/me/api-tokens", credential,
		`{"name":"`+name+`","current_password":"correct horse battery"}`)
	var c createdToken
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &c) != nil {
		t.Fatalf("create a token = %d %s, want 201", rec.Code, rec.Body)
	}
	return c
}

// A token made with the current password works at once; it can make
// another; the list shows both, never a token or its hash.
func TestCreateAndListTokens(t *testing.T) {
	h, pool := newServer(t)
	_, session := register(h, "alice@corp.com")

	first := createToken(t, h, session.AccessToken, "CI")
	second := createToken(t, h, first.Token, "agent")
	list := send(h, http.MethodGet, "/api/v0/me/api-tokens", second.Token, "")

	if !strings.HasPrefix(first.Token, "nwk_pat_") || getMe(h, second.Token).Code != http.StatusOK {
		t.Errorf("tokens %q, %q; want nwk_pat_ tokens that authenticate", first.Token, second.Token)
	}
	var listed struct {
		Data []createdToken `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || list.Code != http.StatusOK || len(listed.Data) != 2 ||
		listed.Data[0].ID != second.ID || listed.Data[1].ID != first.ID {
		t.Errorf("list = %d %s, want agent, then CI", list.Code, list.Body)
	}
	var hash []byte
	if err := pool.QueryRow(context.Background(), `SELECT token_hash FROM api_tokens WHERE id = $1`, first.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{first.Token, second.Token, hex.EncodeToString(hash), base64.StdEncoding.EncodeToString(hash)} {
		if strings.Contains(list.Body.String(), secret) {
			t.Errorf("the list holds %q", secret)
		}
	}
}

// A wrong password makes no token and answers 422, not 401: the client
// keeps its session.
func TestCreateATokenWithAWrongPassword(t *testing.T) {
	h, pool := newServer(t)
	_, session := register(h, "alice@corp.com")

	rec := send(h, http.MethodPost, "/api/v0/me/api-tokens", session.AccessToken, `{"name":"CI","current_password":"not the password"}`)

	var tokens int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM api_tokens`).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"code":"identity.current_password_incorrect"`) || tokens != 0 {
		t.Errorf("create = %d %s with %d tokens, want 422 identity.current_password_incorrect and none", rec.Code, rec.Body, tokens)
	}
	if getMe(h, session.AccessToken).Code != http.StatusOK {
		t.Error("the session stopped working")
	}
}

// Revoking stops a token at once; a token may revoke itself; another
// account's token, or one revoked already, is not found.
func TestRevokeTokens(t *testing.T) {
	h, _ := newServer(t)
	_, alice := register(h, "alice@corp.com")
	_, bob := register(h, "bob@corp.com")
	ci := createToken(t, h, alice.AccessToken, "CI")
	agent := createToken(t, h, alice.AccessToken, "agent")
	bobs := createToken(t, h, bob.AccessToken, "bob's")

	others := send(h, http.MethodDelete, "/api/v0/api-tokens/"+bobs.ID, alice.AccessToken, "")
	revoked := send(h, http.MethodDelete, "/api/v0/api-tokens/"+ci.ID, alice.AccessToken, "")
	again := send(h, http.MethodDelete, "/api/v0/api-tokens/"+ci.ID, alice.AccessToken, "")
	itself := send(h, http.MethodDelete, "/api/v0/api-tokens/"+agent.ID, agent.Token, "")

	if others.Code != http.StatusNotFound || revoked.Code != http.StatusNoContent || again.Code != http.StatusNotFound || itself.Code != http.StatusNoContent {
		t.Errorf("revoke bob's = %d, CI = %d, CI again = %d, agent by itself = %d; want 404, 204, 404, 204",
			others.Code, revoked.Code, again.Code, itself.Code)
	}
	if getMe(h, ci.Token).Code != http.StatusUnauthorized || getMe(h, agent.Token).Code != http.StatusUnauthorized || getMe(h, bobs.Token).Code != http.StatusOK {
		t.Error("want CI and agent refused, bob's token working")
	}
}
