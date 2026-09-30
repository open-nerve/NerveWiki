package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
)

// sessionIDs are the ids of the sessions left, in order.
func sessionIDs(t *testing.T, pool *pgxpool.Pool) []uuid.UUID {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT id FROM auth_sessions ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

// DeleteExpiredSessions deletes, batch by batch, the sessions of every
// account that expired before now; a session expiring at now is still
// live, and so are revoked ones that have not expired.
func TestDeleteExpiredSessions(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u, other := newUser("alice@corp.com"), newUser("bob@corp.com")
	mustCreate(t, s, u)
	mustCreate(t, s, other)
	session := func(userID uuid.UUID, expires time.Time) uuid.UUID {
		n := app.NewSession{ID: uuid.NewV7(), UserID: userID, TokenHash: secretHash(), ExpiresAt: expires, Now: now().Add(-time.Hour)}
		if err := s.CreateSession(ctx, n); err != nil {
			t.Fatal(err)
		}
		return n.ID
	}
	for range 3 {
		session(u.ID, now().Add(-time.Minute))
	}
	session(other.ID, now().Add(-time.Microsecond))
	atNow, live, revoked := session(u.ID, now()), session(u.ID, sessionEnd()), session(u.ID, sessionEnd())
	exec(t, pool, "UPDATE auth_sessions SET revoked_at = $1, revoke_reason = 'logout' WHERE id = $2", now(), revoked)

	var batches []int
	for {
		n, err := s.DeleteExpiredSessions(ctx, now(), 3)
		if err != nil {
			t.Fatal(err)
		}
		batches = append(batches, n)
		if n < 3 {
			break
		}
	}

	if !slices.Equal(batches, []int{3, 1}) {
		t.Errorf("batches of 3 deleted %v, want 3 then 1", batches)
	}
	if left, want := sessionIDs(t, pool), []uuid.UUID{atNow, live, revoked}; !slices.Equal(left, want) {
		t.Errorf("sessions left %v, want the one expiring at now, the live and the revoked one %v", left, want)
	}
}

// A session another transaction holds (a refresh, a logout) is skipped at
// once, not waited for, and deleted by a later run.
func TestDeleteExpiredSessionsSkipsASessionInUse(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	n := app.NewSession{ID: uuid.NewV7(), UserID: u.ID, TokenHash: secretHash(), ExpiresAt: now().Add(-time.Minute), Now: now().Add(-time.Hour)}
	if err := s.CreateSession(ctx, n); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT 1 FROM auth_sessions WHERE id = $1 FOR UPDATE", n.ID); err != nil {
		t.Fatal(err)
	}

	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	held, err := s.DeleteExpiredSessions(bounded, now(), 10)
	if err != nil || held != 0 {
		t.Fatalf("while the session is held: %d deleted, %v; want 0 at once", held, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if freed, err := s.DeleteExpiredSessions(ctx, now(), 10); err != nil || freed != 1 {
		t.Errorf("once the session is free: %d deleted, %v; want 1", freed, err)
	}
}
