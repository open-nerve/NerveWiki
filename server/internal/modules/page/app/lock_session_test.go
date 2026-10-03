package app_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The edit lock's sessions (M5 design 4.2, 4.3): an opening with the lock
// as its vetoer, a take-over, the forced unlock, the lock's read, and what
// a tombstone answers.

// takeOver runs openEditSession on the page id as alice from the web,
// taking her own session over.
func (f *fixture) takeOver(id uuid.UUID) (app.EditSession, error) {
	return app.NewOpenEditSession(f.writer(), f.store, f.logger()).Execute(f.asAlice(), id, domain.ClientWeb, true)
}

// unlock runs releaseEditLock on the page id as alice from the web.
func (f *fixture) unlock(id uuid.UUID) error {
	return app.NewReleaseEditLock(f.writer(), f.store, f.logger()).Execute(f.asAlice(), id, domain.ClientWeb)
}

// readLock runs getEditLock on the page id as alice, at at.
func (f *fixture) readLock(id uuid.UUID, at time.Time) (app.LockView, error) {
	return app.NewGetEditLock(f.notebooks, f.store, f.store, f.names, f.auth, fixedClock{at}).Execute(f.asAlice(), id)
}

// tombstone makes s a tombstone of reason, by by, a second before now.
func (f *fixture) tombstone(s app.EditSession, reason domain.EndReason, by uuid.UUID) app.EditSession {
	s.EndedReason, s.EndedByID, s.EndedAt = reason, by, now().Add(-time.Second)
	f.store.sessions[s.ID] = s
	return s
}

// An opening deletes the page's expired rows first, tombstones among them,
// and tells the subscribers it opened, after the session is written.
func TestAnOpeningDeletesTheExpiredRowsAndTellsItOpened(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionEdit)
	sub := &subscriber{recorder: f.rec}
	f.enders = []app.EditSessionSubscriber{sub}
	n, other := f.page("Notes", nil, 0), f.page("Other", nil, 1)
	expired := f.session(n.ID, f.alice, now())
	tomb := f.tombstone(f.session(n.ID, f.alice, now()), domain.EndedTakenOver, f.alice)
	kept := f.session(other.ID, f.alice, now())
	s, err := f.open(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"FindNode", "WorkspaceOf", "ShareWorkspace in tx", "ShareNotebook in tx", "Authorize page.edit in tx",
		"LockContent in tx", "DeleteExpiredSessionsOf in tx", "CreateSession in tx", "EditSessionOpened in tx"}
	if !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v", f.rec.calls, want)
	}
	if _, ok := f.store.sessions[expired.ID]; ok {
		t.Error("the expired session is kept")
	}
	if _, ok := f.store.sessions[tomb.ID]; ok {
		t.Error("the expired tombstone is kept")
	}
	if _, ok := f.store.sessions[kept.ID]; !ok {
		t.Error("another page's expired session is deleted")
	}
	opened := []app.SessionOpened{{SessionID: s.ID, WorkspaceID: f.acme, NotebookID: f.eng, PageID: n.ID, UserID: f.alice, At: now()}}
	if !reflect.DeepEqual(sub.opened, opened) {
		t.Errorf("the subscriber followed %+v, want %+v", sub.opened, opened)
	}
}

// A take-over ends the opener's alive sessions of the page as tombstones,
// telling the subscribers, before the vetoers see the opening; with the
// lock as vetoer, the opening then passes. Someone else's lock it does not
// pass: page.locked, naming them, and nothing ends.
func TestATakeOverEndsTheOpenersOwnSessionsAlone(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionEdit)
	bob := uuid.NewV7()
	f.names[f.alice], f.names[bob] = "Alice", "Bob"
	sub := &subscriber{recorder: f.rec}
	v := &vetoer{recorder: f.rec}
	f.enders, f.vetoers = []app.EditSessionSubscriber{sub}, []app.EditSessionVetoer{app.NewEditLock(f.store, f.names), v}
	n := f.page("Notes", nil, 0)
	old := f.session(n.ID, f.alice, now().Add(time.Minute))

	if _, err := f.open(n.ID); codeOf(err) != "page.locked" {
		t.Fatalf("an opening of a page alice holds = %v, want page.locked: she did not take it over", err)
	}
	f.rec.calls = nil
	s, err := f.takeOver(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"FindNode", "WorkspaceOf", "ShareWorkspace in tx", "ShareNotebook in tx", "Authorize page.edit in tx",
		"LockContent in tx", "DeleteExpiredSessionsOf in tx", "EndAliveSessions in tx", "EditSessionEnded in tx",
		"AliveSessionsOf in tx", "VetoEditSession in tx", "CreateSession in tx", "EditSessionOpened in tx"}
	if !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v", f.rec.calls, want)
	}
	at := s.CreatedAt // the unit's time: the clock ticked for the refused opening
	ended := f.store.sessions[old.ID]
	if ended.EndedReason != domain.EndedTakenOver || ended.EndedByID != f.alice || !ended.EndedAt.Equal(at) ||
		!ended.ExpiresAt.Equal(at.Add(domain.EditSessionLease)) {
		t.Errorf("the old session = %+v, want taken over by alice at %v, kept a lease", ended, at)
	}
	told := []app.SessionEnded{{SessionID: old.ID, WorkspaceID: f.acme, NotebookID: f.eng, PageID: n.ID, UserID: f.alice,
		Reason: domain.EndedTakenOver, By: f.alice, At: at}}
	if !reflect.DeepEqual(sub.ended, told) || len(v.openings) != 1 || !v.openings[0].TakeOver || !f.store.sessions[s.ID].Alive(at) {
		t.Errorf("told %+v, the vetoer saw %+v; want %+v, a take-over, and the new session alive", sub.ended, v.openings, told)
	}

	bobs := f.page("Bob's", nil, 1)
	held := f.session(bobs.ID, bob, now().Add(time.Minute))
	sub.ended = nil
	_, err = f.takeOver(bobs.ID)
	if page, holder, ok := lockOf(err, f.names); !ok || page != bobs.ID || holder != "Bob" {
		t.Errorf("a take-over of bob's lock = %v, want page.locked by Bob", err)
	}
	if f.store.sessions[held.ID] != held || len(sub.ended) != 0 {
		t.Errorf("bob's session = %+v, told %+v; want it as it was, nothing told", f.store.sessions[held.ID], sub.ended)
	}
}

// The forced unlock: an admin's deletes the page's expired rows, then ends
// every alive session of the page as a tombstone by them, telling the
// subscribers, under the page's gate, and logs how many; it writes no
// changeset and tells no observer. A page no one holds answers no error. A
// session that opened after the unlock's time, while the unlock waited at
// the gate, ends at its opening, and is told so (M5/P1 review I1). A page
// missing is not found, and anyone but an admin is forbidden.
func TestAnUnlockEndsThePagesSessions(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionReleaseEditLock)
	sub, o := &subscriber{recorder: f.rec}, &observer{recorder: f.rec}
	f.enders, f.observers = []app.EditSessionSubscriber{sub}, []app.PageObserver{o}
	bob := uuid.NewV7()
	n := f.page("Notes", nil, 0)
	held := f.session(n.ID, bob, now().Add(time.Minute))
	expired := f.session(n.ID, bob, now())
	if err := f.unlock(n.ID); err != nil {
		t.Fatal(err)
	}
	want := []string{"FindNode", "WorkspaceOf", "ShareWorkspace in tx", "ShareNotebook in tx", "Authorize page.release_edit_lock in tx",
		"LockContent in tx", "DeleteExpiredSessionsOf in tx", "EndAliveSessions in tx", "EditSessionEnded in tx"}
	if !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v", f.rec.calls, want)
	}
	ended := f.store.sessions[held.ID]
	_, kept := f.store.sessions[expired.ID]
	if ended.EndedReason != domain.EndedUnlocked || ended.EndedByID != f.alice || kept {
		t.Errorf("bob's session = %+v, the expired one kept %v; want it unlocked by alice, the expired one deleted", ended, kept)
	}
	told := []app.SessionEnded{{SessionID: held.ID, WorkspaceID: f.acme, NotebookID: f.eng, PageID: n.ID, UserID: bob,
		Reason: domain.EndedUnlocked, By: f.alice, At: now()}}
	if !reflect.DeepEqual(sub.ended, told) || len(f.store.changesets) != 0 || len(o.events) != 0 {
		t.Errorf("told %+v, %d changesets, %d events; want %+v, none and none", sub.ended, len(f.store.changesets), len(o.events), told)
	}
	if logs := f.logs.String(); !strings.Contains(logs, "edit lock released") || !strings.Contains(logs, "sessions=1") ||
		!strings.Contains(logs, "node_id="+n.ID.String()) {
		t.Errorf("log %q, want the release with the page and how many ended", logs)
	}
	if err := f.unlock(n.ID); err != nil || len(sub.ended) != 1 {
		t.Errorf("an unlock of a page no one holds = %v, told %d; want no error, nothing more told", err, len(sub.ended))
	}
	later := f.session(n.ID, bob, now().Add(time.Minute))
	later.CreatedAt = now().Add(time.Second)
	f.store.sessions[later.ID] = later
	sub.ended = nil
	if err := f.unlock(n.ID); err != nil || len(sub.ended) != 1 || !sub.ended[0].At.Equal(later.CreatedAt) ||
		!f.store.sessions[later.ID].EndedAt.Equal(later.CreatedAt) {
		t.Errorf("an unlock timed before the opening = %v, told %+v; want it ended and told at the opening", err, sub.ended)
	}

	for _, tt := range []struct {
		name  string
		setup func(f *fixture, n domain.Node) uuid.UUID
		want  string
	}{
		{"a page missing", func(f *fixture, n domain.Node) uuid.UUID { return uuid.NewV7() }, "page.not_found"},
		{"a notebook not seen", func(f *fixture, n domain.Node) uuid.UUID { return n.ID }, "page.not_found"},
		{"an editor", func(f *fixture, n domain.Node) uuid.UUID {
			f.auth.forbidden[domain.ActionReleaseEditLock] = true
			return n.ID
		}, "forbidden"},
		{"a page deleted while it waited", func(f *fixture, n domain.Node) uuid.UUID {
			f.grant(domain.ActionReleaseEditLock)
			delete(f.store.contents, n.ID)
			return n.ID
		}, "page.not_found"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			n := f.page("Notes", nil, 0)
			s := f.session(n.ID, bob, now().Add(time.Minute))
			if err := f.unlock(tt.setup(f, n)); codeOf(err) != tt.want || f.store.sessions[s.ID] != s {
				t.Errorf("releaseEditLock = %v, session %+v; want %s and the session as it was", err, f.store.sessions[s.ID], tt.want)
			}
		})
	}
}

// The lock's read gives the holder by name and the seconds left, rounded
// up; a reader may read it; no holder for a page no alive session holds.
// It reads without a transaction.
func TestTheLocksRead(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	bob := uuid.NewV7()
	f.names[bob] = "Bob"
	n := f.page("Notes", nil, 0)
	held := f.session(n.ID, bob, now().Add(90*time.Second+time.Millisecond))
	got, err := f.readLock(n.ID, now())
	want := app.LockView{Holder: &app.LockHolder{UserID: bob, DisplayName: "Bob"}, ExpiresIn: 91}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("getEditLock = %+v, %v; want %+v", got, err, want)
	}
	if calls := f.rec.calls; slices.ContainsFunc(calls, func(c string) bool { return strings.HasSuffix(c, " in tx") }) {
		t.Errorf("calls = %v, want none in a transaction", calls)
	}
	if got, err := f.readLock(n.ID, held.ExpiresAt.Add(-90*time.Second)); err != nil || got.ExpiresIn != 90 {
		t.Errorf("getEditLock with 90 seconds left = %+v, %v; want 90, not rounded up past a whole second", got, err)
	}
	for name, at := range map[string]time.Time{"expired": held.ExpiresAt, "never held": now()} {
		id := n.ID
		if name == "never held" {
			id = f.page("Free", nil, 1).ID
		}
		if got, err := f.readLock(id, at); err != nil || got != (app.LockView{}) {
			t.Errorf("getEditLock of a page %s = %+v, %v; want no holder", name, got, err)
		}
	}
	f.tombstone(held, domain.EndedUnlocked, f.alice)
	if got, err := f.readLock(n.ID, now()); err != nil || got != (app.LockView{}) {
		t.Errorf("getEditLock of a tombstone's page = %+v, %v; want no holder", got, err)
	}
	f.auth.grants[domain.ActionRead] = false
	if _, err := f.readLock(n.ID, now()); codeOf(err) != "page.not_found" {
		t.Errorf("getEditLock of a notebook not seen = %v, want page.not_found", err)
	}
}

// A heartbeat of the caller's tombstone says why it ended: taken over, or
// unlocked by whom; someone else's tombstone is not found. It decides on
// the notebook first, as for an alive session: a notebook the caller sees
// no more is not found, a reader's is forbidden, and neither says who
// released it. A session taken over between the heartbeat's read and its
// update answers the same, and the heartbeat keeps nothing alive.
func TestAHeartbeatOfATombstoneSaysWhy(t *testing.T) {
	bob := uuid.NewV7()
	for _, tt := range []struct {
		name   string
		reason domain.EndReason
		owner  func(f *fixture) uuid.UUID
		want   string
		by     string
	}{
		{"taken over", domain.EndedTakenOver, func(f *fixture) uuid.UUID { return f.alice }, "page.edit_session_taken_over", ""},
		{"unlocked", domain.EndedUnlocked, func(f *fixture) uuid.UUID { return f.alice }, "page.edit_session_unlocked", "Bob"},
		{"someone else's", domain.EndedUnlocked, func(f *fixture) uuid.UUID { return bob }, "page.edit_session_not_found", ""},
		{"of a notebook not seen", domain.EndedUnlocked, func(f *fixture) uuid.UUID {
			f.auth.grants[domain.ActionEdit] = false
			return f.alice
		}, "page.edit_session_not_found", ""},
		{"a reader's", domain.EndedUnlocked, func(f *fixture) uuid.UUID {
			f.auth.grants[domain.ActionEdit] = false
			f.auth.forbidden[domain.ActionEdit] = true
			return f.alice
		}, "forbidden", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionEdit)
			f.names[bob] = "Bob"
			n := f.page("Notes", nil, 0)
			s := f.tombstone(f.session(n.ID, tt.owner(f), now().Add(time.Minute)), tt.reason, bob)
			_, err := f.beat(s.ID, now())
			if codeOf(err) != tt.want || endedBy(err) != tt.by || f.store.sessions[s.ID] != s {
				t.Errorf("heartbeatEditSession = %v (ended by %q), session %+v; want %s by %q, the tombstone as it was",
					err, endedBy(err), f.store.sessions[s.ID], tt.want, tt.by)
			}
		})
	}

	f := newFixture()
	f.grant(domain.ActionEdit)
	n := f.page("Notes", nil, 0)
	s := f.session(n.ID, f.alice, now().Add(time.Minute))
	between := takenOverBetween{fakeStore: f.store, f: f, id: s.ID}
	_, err := app.NewHeartbeatEditSession(between, f.notebooks, f.auth, f.names, fixedClock{now()}).Execute(f.asAlice(), s.ID)
	if after := f.store.sessions[s.ID]; codeOf(err) != "page.edit_session_taken_over" || after.EndedReason != domain.EndedTakenOver ||
		!after.ExpiresAt.Equal(s.ExpiresAt) {
		t.Errorf("a heartbeat whose session was taken over meanwhile = %v, session %+v; want page.edit_session_taken_over",
			err, f.store.sessions[s.ID])
	}
}

// takenOverBetween is the store, with the session id taken over by its
// owner between a heartbeat's read and its update.
type takenOverBetween struct {
	*fakeStore
	f  *fixture
	id uuid.UUID
}

func (b takenOverBetween) HeartbeatSession(ctx context.Context, id, userID uuid.UUID, now, until time.Time) (app.EditSession, error) {
	b.f.tombstone(b.sessions[b.id], domain.EndedTakenOver, userID)
	return b.fakeStore.HeartbeatSession(ctx, id, userID, now, until)
}

// endedBy is the ended_by member's name of err, if it has one.
func endedBy(err error) string {
	var e *shared.Error
	if !errors.As(err, &e) || e.EndedBy == nil {
		return ""
	}
	return e.EndedBy.DisplayName
}

// An end of the caller's tombstone deletes it and tells no one; someone
// else's is not found and stays.
func TestAnEndOfATombstoneTellsNoOne(t *testing.T) {
	f := newFixture()
	sub := &subscriber{recorder: f.rec}
	f.enders = []app.EditSessionSubscriber{sub}
	n := f.page("Notes", nil, 0)
	tomb := f.tombstone(f.session(n.ID, f.alice, now().Add(time.Minute)), domain.EndedTakenOver, f.alice)
	if err := f.end(tomb.ID, now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.store.sessions[tomb.ID]; ok || len(sub.ended) != 0 {
		t.Errorf("the tombstone kept %v, told %+v; want it deleted, nothing told", ok, sub.ended)
	}
	others := f.tombstone(f.session(n.ID, uuid.NewV7(), now().Add(time.Minute)), domain.EndedUnlocked, f.alice)
	if err := f.end(others.ID, now()); codeOf(err) != "page.edit_session_not_found" || f.store.sessions[others.ID] != others {
		t.Errorf("an end of someone else's tombstone = %v, want page.edit_session_not_found and it kept", err)
	}
}
