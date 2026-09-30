package postgresadapter_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
)

func secretHash() []byte {
	h := sha256.Sum256([]byte("secret 0"))
	return h[:]
}

func nextHash() []byte {
	h := sha256.Sum256([]byte("secret 1"))
	return h[:]
}

func sessionEnd() time.Time { return now().Add(time.Hour) }
func later() time.Time      { return now().Add(time.Minute) }

// sessionRow is what the refresh tests read back of a session.
type sessionRow struct {
	generation                int32
	tokenHash                 []byte
	expires, created, updated time.Time
	lastRefreshed, revoked    *time.Time
	reason                    *string
}

// newLiveSession inserts a session of a new account at generation 0 with
// secretHash, until sessionEnd.
func newLiveSession(t *testing.T, s *postgresadapter.Store) app.NewSession {
	t.Helper()
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	n := app.NewSession{ID: uuid.NewV7(), UserID: u.ID, TokenHash: secretHash(), ExpiresAt: sessionEnd(), Now: now()}
	if err := s.CreateSession(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	return n
}

func readSession(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) sessionRow {
	t.Helper()
	var r sessionRow
	err := pool.QueryRow(context.Background(), `SELECT generation, token_hash, expires_at, created_at, updated_at,
		last_refreshed_at, revoked_at, revoke_reason FROM auth_sessions WHERE id = $1`, id).
		Scan(&r.generation, &r.tokenHash, &r.expires, &r.created, &r.updated, &r.lastRefreshed, &r.revoked, &r.reason)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func TestSessionForRefresh(t *testing.T) {
	s, pool := newStore(t)
	n := newLiveSession(t, s)

	got, err := s.SessionForRefresh(context.Background(), n.ID)

	if err != nil || got.UserID != n.UserID || got.State.Generation != 0 || !bytes.Equal(got.State.TokenHash, secretHash()) ||
		got.State.Revoked || !got.State.ExpiresAt.Equal(sessionEnd()) {
		t.Errorf("SessionForRefresh() = %+v, %v; want generation 0 of the account with the hash, live until %v", got, err, sessionEnd())
	}
	exec(t, pool, "UPDATE auth_sessions SET generation = 7, revoked_at = now(), revoke_reason = 'logout'")
	if got, err := s.SessionForRefresh(context.Background(), n.ID); err != nil || got.State.Generation != 7 || !got.State.Revoked {
		t.Errorf("SessionForRefresh() after a change = %+v, %v; want generation 7, revoked", got, err)
	}
	if _, err := s.SessionForRefresh(context.Background(), uuid.NewV7()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("SessionForRefresh(unknown) = %v, want app.ErrNotFound", err)
	}
}

// missedConditions break one condition of rotation and logout each (M1/P2
// design 3.5): neither statement may touch the row.
func missedConditions() []struct {
	name  string
	g     func(*app.SessionGeneration)
	setup string
} {
	return []struct {
		name  string
		g     func(*app.SessionGeneration)
		setup string
	}{
		{"another generation", func(g *app.SessionGeneration) { g.Generation = 1 }, ""},
		{"another hash", func(g *app.SessionGeneration) { g.TokenHash = nextHash() }, ""},
		{"an unknown session", func(g *app.SessionGeneration) { g.ID = uuid.NewV7() }, ""},
		{"a revoked session", nil, "UPDATE auth_sessions SET revoked_at = now(), revoke_reason = 'reuse_detected'"},
		{"a session ending at this instant", func(g *app.SessionGeneration) { g.Now = sessionEnd() }, ""},
	}
}

func TestRotateSession(t *testing.T) {
	s, pool := newStore(t)
	n := newLiveSession(t, s)
	g := app.SessionGeneration{ID: n.ID, Generation: 0, TokenHash: secretHash(), Now: later()}

	rotated, err := s.RotateSession(context.Background(), g, nextHash())

	r := readSession(t, pool, n.ID)
	if err != nil || !rotated || r.generation != 1 || !bytes.Equal(r.tokenHash, nextHash()) || r.lastRefreshed == nil || !r.lastRefreshed.Equal(later()) ||
		!r.updated.Equal(later()) || !r.expires.Equal(sessionEnd()) || !r.created.Equal(now()) || r.revoked != nil {
		t.Errorf("RotateSession() = %v, %v; row %+v; want generation 1 with the new hash, refreshed and updated at %v, the end unchanged",
			rotated, err, r, later())
	}
	if again, err := s.RotateSession(context.Background(), g, nextHash()); again || err != nil {
		t.Errorf("RotateSession() of the old generation again = %v, %v; want false", again, err)
	}
}

func TestRotateSessionMisses(t *testing.T) {
	for _, tt := range missedConditions() {
		t.Run(tt.name, func(t *testing.T) {
			s, pool := newStore(t)
			n := newLiveSession(t, s)
			if tt.setup != "" {
				exec(t, pool, tt.setup)
			}
			g := app.SessionGeneration{ID: n.ID, Generation: 0, TokenHash: secretHash(), Now: later()}
			if tt.g != nil {
				tt.g(&g)
			}
			before := readSession(t, pool, n.ID)

			rotated, err := s.RotateSession(context.Background(), g, nextHash())

			if after := readSession(t, pool, n.ID); rotated || err != nil || after.generation != before.generation || !after.updated.Equal(before.updated) {
				t.Errorf("RotateSession() = %v, %v; row %+v, want it unchanged from %+v", rotated, err, after, before)
			}
		})
	}
}

func TestRevokeForReuse(t *testing.T) {
	s, pool := newStore(t)
	n := newLiveSession(t, s)

	if err := s.RevokeForReuse(context.Background(), n.ID, later()); err != nil {
		t.Fatal(err)
	}

	r := readSession(t, pool, n.ID)
	if r.revoked == nil || !r.revoked.Equal(later()) || r.reason == nil || *r.reason != "reuse_detected" || !r.updated.Equal(later()) {
		t.Errorf("row = %+v, want revoked at %v for reuse_detected", r, later())
	}
}

// A session revoked for another reason keeps it.
func TestRevokeForReuseKeepsAnEarlierRevocation(t *testing.T) {
	s, pool := newStore(t)
	n := newLiveSession(t, s)
	exec(t, pool, "UPDATE auth_sessions SET revoked_at = $1, revoke_reason = 'logout'", now())

	if err := s.RevokeForReuse(context.Background(), n.ID, later()); err != nil {
		t.Fatal(err)
	}

	if r := readSession(t, pool, n.ID); !r.revoked.Equal(now()) || *r.reason != "logout" {
		t.Errorf("row = %+v, want the logout at %v kept", r, now())
	}
}

func TestEndSession(t *testing.T) {
	s, pool := newStore(t)
	n := newLiveSession(t, s)

	ended, err := s.EndSession(context.Background(), app.SessionGeneration{ID: n.ID, Generation: 0, TokenHash: secretHash(), Now: later()})

	r := readSession(t, pool, n.ID)
	if err != nil || !ended || r.revoked == nil || !r.revoked.Equal(later()) || *r.reason != "logout" || !r.updated.Equal(later()) || r.generation != 0 {
		t.Errorf("EndSession() = %v, %v; row %+v; want revoked at %v for logout", ended, err, r, later())
	}
}

func TestEndSessionMisses(t *testing.T) {
	for _, tt := range missedConditions() {
		t.Run(tt.name, func(t *testing.T) {
			s, pool := newStore(t)
			n := newLiveSession(t, s)
			if tt.setup != "" {
				exec(t, pool, tt.setup)
			}
			g := app.SessionGeneration{ID: n.ID, Generation: 0, TokenHash: secretHash(), Now: later()}
			if tt.g != nil {
				tt.g(&g)
			}
			before := readSession(t, pool, n.ID)

			ended, err := s.EndSession(context.Background(), g)

			after := readSession(t, pool, n.ID)
			if ended || err != nil || !after.updated.Equal(before.updated) || (before.reason == nil) != (after.reason == nil) {
				t.Errorf("EndSession() = %v, %v; row %+v, want it unchanged from %+v", ended, err, after, before)
			}
		})
	}
}
