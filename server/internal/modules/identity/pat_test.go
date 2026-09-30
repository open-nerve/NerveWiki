package identity_test

import (
	"context"
	"net/http"
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
