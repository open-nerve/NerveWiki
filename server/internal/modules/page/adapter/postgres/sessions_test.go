package postgresadapter_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// open opens an edit session of the page id by user, alive until expires.
func (f fixture) open(t *testing.T, id, notebook, user uuid.UUID, expires time.Time) app.EditSession {
	t.Helper()
	s := app.EditSession{ID: uuid.NewV7(), NodeID: id, NotebookID: notebook, UserID: user, Client: domain.ClientWeb,
		CreatedAt: now(), ExpiresAt: expires}
	if err := f.s.CreateSession(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}

// user adds an account and returns its id.
func (f fixture) user(t *testing.T, email string) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, $2, 'x', 'x', $3, $3)",
		id, email, now())
	return id
}

func sessionIDs(sessions []app.EditSession) []uuid.UUID {
	ids := make([]uuid.UUID, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return a.Compare(b) })
	return ids
}

// A session reads back as opened, without a write; a write sets its
// changeset and revision together, and fails for a session not there.
func TestSessionsReadBack(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a := f.page(t, f.eng, nil, "A", 0)
	opened := f.open(t, a.ID, f.eng, f.alice, now().Add(time.Minute))
	got, err := f.s.LockSession(ctx, opened.ID)
	if err != nil || !reflect.DeepEqual(got, opened) {
		t.Errorf("LockSession() = %+v, %v; want %+v", got, err, opened)
	}
	cs := uuid.NewV7()
	if err := f.s.SetSessionWrite(ctx, opened.ID, cs, 7); err != nil {
		t.Fatal(err)
	}
	got, err = f.s.LockSession(ctx, opened.ID)
	if err != nil || got.ChangesetID != cs || got.Revision != 7 {
		t.Errorf("after a write, LockSession() = %+v, %v; want changeset %s at revision 7", got, err, cs)
	}
	if _, err := f.s.LockSession(ctx, uuid.NewV7()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("LockSession(none) = %v, want ErrNotFound", err)
	}
	if err := f.s.SetSessionWrite(ctx, uuid.NewV7(), cs, 8); err == nil {
		t.Error("SetSessionWrite(none) = nil, want an error: a unit sets the write of the session it holds")
	}
}

// A heartbeat, an end and the unlocked read take the caller's own session
// alive at now: one that expires at now, or someone else's, is not found
// and stays as it was.
func TestHeartbeatAndEndTakeTheOwnersAliveSession(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	bob := f.user(t, "bob@corp.com")
	a := f.page(t, f.eng, nil, "A", 0)
	expires := now().Add(time.Minute)
	s := f.open(t, a.ID, f.eng, f.alice, expires)
	until := expires.Add(time.Minute)
	for _, tt := range []struct {
		name string
		user uuid.UUID
		at   time.Time
	}{{"someone else's", bob, now()}, {"expired at now", f.alice, expires}, {"long expired", f.alice, until}} {
		if _, err := f.s.FindLiveSession(ctx, s.ID, tt.user, tt.at); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("FindLiveSession(%s) = %v, want ErrNotFound", tt.name, err)
		}
		if _, err := f.s.HeartbeatSession(ctx, s.ID, tt.user, tt.at, until); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("HeartbeatSession(%s) = %v, want ErrNotFound", tt.name, err)
		}
		if _, err := f.s.EndSession(ctx, s.ID, tt.user, tt.at); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("EndSession(%s) = %v, want ErrNotFound", tt.name, err)
		}
	}
	if got := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = $1 AND expires_at = $2", s.ID, expires); got != 1 {
		t.Fatal("a refused heartbeat or end changed the session")
	}

	alive := expires.Add(-time.Microsecond)
	found, err := f.s.FindLiveSession(ctx, s.ID, f.alice, alive)
	if err != nil || found.ID != s.ID {
		t.Errorf("FindLiveSession(a moment before it expires) = %+v, %v; want it", found, err)
	}
	beat, err := f.s.HeartbeatSession(ctx, s.ID, f.alice, alive, until)
	if err != nil || !beat.ExpiresAt.Equal(until) {
		t.Errorf("HeartbeatSession() = %+v, %v; want it kept until %v", beat, err, until)
	}
	ended, err := f.s.EndSession(ctx, s.ID, f.alice, expires)
	if err != nil || ended.ID != s.ID || !ended.ExpiresAt.Equal(until) {
		t.Errorf("EndSession() after the heartbeat = %+v, %v; want it, until %v", ended, err, until)
	}
	if got := f.count(t, "SELECT count(*) FROM edit_sessions"); got != 0 {
		t.Errorf("%d sessions after the end, want none", got)
	}
}

// Deleting pages or notebooks deletes their sessions, alive or not, and
// returns them; the others stay.
func TestDeleteSessionsWithTheirPagesAndNotebooks(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a, b, c := f.page(t, f.eng, nil, "A", 0), f.page(t, f.eng, nil, "B", 1), f.page(t, f.eng, nil, "C", 2)
	o := f.page(t, f.ops, nil, "O", 0)
	ofA := []app.EditSession{f.open(t, a.ID, f.eng, f.alice, now().Add(time.Minute)), f.open(t, a.ID, f.eng, f.alice, now().Add(time.Second))}
	ofB := f.open(t, b.ID, f.eng, f.alice, now().Add(time.Minute))
	ofC := f.open(t, c.ID, f.eng, f.alice, now().Add(time.Minute))
	ofO := f.open(t, o.ID, f.ops, f.alice, now().Add(time.Minute))

	deleted, err := f.s.DeleteNodeSessions(ctx, []uuid.UUID{a.ID, b.ID})
	if want := sessionIDs(append(ofA, ofB)); err != nil || !slices.Equal(sessionIDs(deleted), want) {
		t.Errorf("DeleteNodeSessions(A, B) = %v, %v; want %v", sessionIDs(deleted), err, want)
	}
	deleted, err = f.s.DeleteNotebookSessions(ctx, []uuid.UUID{f.eng})
	if err != nil || !slices.Equal(sessionIDs(deleted), []uuid.UUID{ofC.ID}) {
		t.Errorf("DeleteNotebookSessions(eng) = %v, %v; want C's", sessionIDs(deleted), err)
	}
	if got := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = $1", ofO.ID); got != 1 {
		t.Error("ops' session is deleted with eng's")
	}
}

// The cleanup deletes the sessions expired at now, a batch at most, and
// skips a row another transaction holds without waiting for it; an alive
// session stays.
func TestDeleteExpiredSessions(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a := f.page(t, f.eng, nil, "A", 0)
	at := now().Add(time.Hour)
	held := f.open(t, a.ID, f.eng, f.alice, at)
	expired := []app.EditSession{f.open(t, a.ID, f.eng, f.alice, now().Add(time.Second)), f.open(t, a.ID, f.eng, f.alice, now().Add(time.Minute))}
	alive := f.open(t, a.ID, f.eng, f.alice, at.Add(time.Microsecond))

	holder, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	if _, err := holder.Exec(ctx, "SELECT 1 FROM edit_sessions WHERE id = $1 FOR UPDATE", held.ID); err != nil {
		t.Fatal(err)
	}
	quick, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if n, err := f.s.DeleteExpiredSessions(quick, at, 1); err != nil || n != 1 {
		t.Errorf("DeleteExpiredSessions(batch 1) = %d, %v; want 1", n, err)
	}
	if n, err := f.s.DeleteExpiredSessions(quick, at, 10); err != nil || n != 1 {
		t.Errorf("DeleteExpiredSessions() beside the held one = %d, %v; want the other expired one, not waiting", n, err)
	}
	if got := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = ANY($1)", []uuid.UUID{expired[0].ID, expired[1].ID}); got != 0 {
		t.Errorf("%d expired sessions are left, want none", got)
	}
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if n, err := f.s.DeleteExpiredSessions(ctx, at, 10); err != nil || n != 1 {
		t.Errorf("DeleteExpiredSessions() once let go = %d, %v; want the held one", n, err)
	}
	if got := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = $1", alive.ID); got != 1 {
		t.Error("the session alive at now is deleted")
	}
}
