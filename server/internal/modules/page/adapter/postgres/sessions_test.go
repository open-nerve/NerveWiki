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
	return f.openAt(t, id, notebook, user, now(), expires)
}

// openAt is open at created.
func (f fixture) openAt(t *testing.T, id, notebook, user uuid.UUID, created, expires time.Time) app.EditSession {
	t.Helper()
	s := app.EditSession{ID: uuid.NewV7(), NodeID: id, NotebookID: notebook, UserID: user, Client: domain.ClientWeb,
		CreatedAt: created, ExpiresAt: expires}
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

// end makes tombstones of the page id's sessions alive at at, for reason
// by by, kept a lease after at.
func (f fixture) end(t *testing.T, id uuid.UUID, user *uuid.UUID, reason domain.EndReason, by uuid.UUID, at time.Time) []app.EditSession {
	t.Helper()
	ended, err := f.s.EndAliveSessions(context.Background(), app.SessionsEnd{
		NodeID: id, UserID: user, Reason: reason, By: by, At: at, Until: at.Add(domain.EditSessionLease),
	})
	if err != nil {
		t.Fatal(err)
	}
	return ended
}

// inOrder are the sessions' ids in the order given.
func inOrder(sessions ...app.EditSession) []uuid.UUID {
	ids := make([]uuid.UUID, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
	}
	return ids
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

// A take-over ends the page's sessions of its owner alive at its time, an
// unlock anyone's (M5 design 4.2, 4.3): each becomes a tombstone of its
// reason, by whom and when, its lease kept or lengthened to a lease after
// the end, never shortened. Expired sessions, tombstones and other pages'
// sessions stay as they were.
func TestEndAliveSessionsLeavesTombstones(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	bob := f.user(t, "bob@corp.com")
	a, b := f.page(t, f.eng, nil, "A", 0), f.page(t, f.eng, nil, "B", 1)
	at := now().Add(10 * time.Second)
	short := f.open(t, a.ID, f.eng, f.alice, now().Add(time.Minute))
	long := f.open(t, a.ID, f.eng, f.alice, at.Add(time.Hour))
	expired := f.open(t, a.ID, f.eng, f.alice, at)
	bobs := f.open(t, a.ID, f.eng, bob, now().Add(time.Minute))
	otherPage := f.open(t, b.ID, f.eng, f.alice, now().Add(time.Minute))

	takenOver := f.end(t, a.ID, &f.alice, domain.EndedTakenOver, f.alice, at)
	if !slices.Equal(sessionIDs(takenOver), sessionIDs([]app.EditSession{short, long})) {
		t.Fatalf("take-over ended %v, want alice's two alive ones", sessionIDs(takenOver))
	}
	for _, tt := range []struct {
		s       app.EditSession
		expires time.Time
	}{{short, at.Add(domain.EditSessionLease)}, {long, long.ExpiresAt}} {
		got, err := f.s.LockSession(ctx, tt.s.ID)
		if err != nil || got.EndedReason != domain.EndedTakenOver || got.EndedByID != f.alice || !got.EndedAt.Equal(at) ||
			!got.ExpiresAt.Equal(tt.expires) {
			t.Errorf("taken over = %+v, %v; want taken_over by alice at %v, until %v", got, err, at, tt.expires)
		}
	}

	later := at.Add(time.Second)
	unlocked := f.end(t, a.ID, nil, domain.EndedUnlocked, f.alice, later)
	if len(unlocked) != 1 || unlocked[0].ID != bobs.ID || unlocked[0].EndedReason != domain.EndedUnlocked ||
		unlocked[0].EndedByID != f.alice || !unlocked[0].EndedAt.Equal(later) {
		t.Errorf("unlock ended %+v, want bob's alone, unlocked by alice at %v", unlocked, later)
	}
	for _, s := range []app.EditSession{expired, otherPage} {
		if got, err := f.s.LockSession(ctx, s.ID); err != nil || !reflect.DeepEqual(got, s) {
			t.Errorf("session %s = %+v, %v; want it as it was", s.ID, got, err)
		}
	}
	if got := f.count(t, "SELECT count(*) FROM edit_sessions WHERE ended_reason = 'taken_over'"); got != 2 {
		t.Errorf("%d taken_over rows after the unlock, want the take-over's two untouched", got)
	}
}

// A tombstone is no alive session: the lock's read leaves it out, the
// live read and the heartbeat find none and keep it as it is. Its owner
// alone finds it as ended, and ends it, whenever.
func TestATombstoneIsNoAliveSession(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	bob := f.user(t, "bob@corp.com")
	a := f.page(t, f.eng, nil, "A", 0)
	s := f.open(t, a.ID, f.eng, f.alice, now().Add(time.Minute))
	alive := f.open(t, a.ID, f.eng, bob, now().Add(time.Minute))
	at := now().Add(time.Second)
	f.end(t, a.ID, &f.alice, domain.EndedTakenOver, f.alice, at)
	tomb, err := f.s.LockSession(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}

	read := at.Add(time.Second)
	if got, err := f.s.AliveSessionsOf(ctx, []uuid.UUID{a.ID}, read); err != nil || !slices.Equal(inOrder(got...), inOrder(alive)) {
		t.Errorf("AliveSessionsOf() = %v, %v; want bob's alone", inOrder(got...), err)
	}
	if _, err := f.s.FindLiveSession(ctx, s.ID, f.alice, read); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("FindLiveSession(tombstone) = %v, want ErrNotFound", err)
	}
	if _, err := f.s.HeartbeatSession(ctx, s.ID, f.alice, read, read.Add(time.Hour)); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("HeartbeatSession(tombstone) = %v, want ErrNotFound", err)
	}
	if got, err := f.s.LockSession(ctx, s.ID); err != nil || !reflect.DeepEqual(got, tomb) {
		t.Errorf("after the heartbeat, the tombstone = %+v, %v; want %+v", got, err, tomb)
	}

	if got, err := f.s.FindEndedSession(ctx, s.ID, f.alice); err != nil || !reflect.DeepEqual(got, tomb) {
		t.Errorf("FindEndedSession(own tombstone) = %+v, %v; want %+v", got, err, tomb)
	}
	if _, err := f.s.FindEndedSession(ctx, s.ID, bob); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("FindEndedSession(someone else's tombstone) = %v, want ErrNotFound", err)
	}
	if _, err := f.s.FindEndedSession(ctx, alive.ID, bob); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("FindEndedSession(an alive session) = %v, want ErrNotFound", err)
	}
	if _, err := f.s.EndSession(ctx, s.ID, bob, read); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("EndSession(someone else's tombstone) = %v, want ErrNotFound", err)
	}
	past := tomb.ExpiresAt.Add(time.Hour)
	if got, err := f.s.EndSession(ctx, s.ID, f.alice, past); err != nil || !reflect.DeepEqual(got, tomb) {
		t.Errorf("EndSession(own tombstone, expired) = %+v, %v; want it deleted", got, err)
	}
	if got := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = $1", s.ID); got != 0 {
		t.Error("the ended tombstone is left")
	}
}

// The lock's read gives the pages' sessions alive at now, by when they
// opened: not the expired, the tombstones, or other pages'.
func TestAliveSessionsOf(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a, b, c := f.page(t, f.eng, nil, "A", 0), f.page(t, f.eng, nil, "B", 1), f.page(t, f.eng, nil, "C", 2)
	at := now().Add(time.Minute)
	second := f.openAt(t, a.ID, f.eng, f.alice, now(), at.Add(time.Second))
	first := f.openAt(t, a.ID, f.eng, f.alice, now().Add(-time.Second), at.Add(time.Second))
	f.open(t, a.ID, f.eng, f.alice, at)
	ofB := f.openAt(t, b.ID, f.eng, f.alice, now().Add(time.Second), at.Add(time.Second))
	f.open(t, c.ID, f.eng, f.alice, at.Add(time.Second))
	tomb := f.open(t, b.ID, f.eng, f.alice, at.Add(time.Hour))
	if _, err := f.pool.Exec(ctx, "UPDATE edit_sessions SET ended_reason = 'unlocked', ended_by_id = $2, ended_at = $3 WHERE id = $1",
		tomb.ID, f.alice, now()); err != nil {
		t.Fatal(err)
	}

	got, err := f.s.AliveSessionsOf(ctx, []uuid.UUID{a.ID, b.ID}, at)
	if want := inOrder(first, second, ofB); err != nil || !slices.Equal(inOrder(got...), want) {
		t.Errorf("AliveSessionsOf(A, B) = %v, %v; want %v", inOrder(got...), err, want)
	}
}

// An opening's cleanup deletes the page's sessions expired at now,
// tombstones among them; the page's alive sessions and tombstones still
// leased, and other pages' sessions, stay.
func TestDeleteExpiredSessionsOf(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a, b := f.page(t, f.eng, nil, "A", 0), f.page(t, f.eng, nil, "B", 1)
	at := now().Add(domain.EditSessionLease + time.Minute)
	expiredTomb := f.open(t, a.ID, f.eng, f.alice, now().Add(time.Second))
	leasedTomb := f.open(t, a.ID, f.eng, f.alice, at.Add(time.Hour))
	f.end(t, a.ID, &f.alice, domain.EndedTakenOver, f.alice, now())
	expired := f.open(t, a.ID, f.eng, f.alice, at)
	alive := f.open(t, a.ID, f.eng, f.alice, at.Add(time.Second))
	ofB := f.open(t, b.ID, f.eng, f.alice, at)
	if got := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = ANY($1) AND ended_reason IS NOT NULL",
		[]uuid.UUID{expiredTomb.ID, leasedTomb.ID}); got != 2 {
		t.Fatalf("%d tombstones, want 2", got)
	}
	if got := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = $1 AND expires_at <= $2", expiredTomb.ID, at); got != 1 {
		t.Fatal("the short tombstone is not expired at the cleanup's time")
	}

	if err := f.s.DeleteExpiredSessionsOf(ctx, a.ID, at); err != nil {
		t.Fatal(err)
	}
	left := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = ANY($1)", []uuid.UUID{expired.ID, expiredTomb.ID})
	kept := f.count(t, "SELECT count(*) FROM edit_sessions WHERE id = ANY($1)", []uuid.UUID{leasedTomb.ID, alive.ID, ofB.ID})
	if left != 0 || kept != 3 {
		t.Errorf("after the cleanup, %d of A's expired rows are left and %d of the others kept; want 0 and 3", left, kept)
	}
}
