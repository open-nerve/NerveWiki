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

// open runs openEditSession on the page id as alice from the web.
func (f *fixture) open(id uuid.UUID) (app.EditSession, error) {
	return app.NewOpenEditSession(f.writer(), f.store, f.logger()).Execute(f.asAlice(), id, domain.ClientWeb)
}

// fixedClock reads at, whatever the times it is read.
type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

// beat runs heartbeatEditSession on the session id as alice, at at.
func (f *fixture) beat(id uuid.UUID, at time.Time) (app.EditSession, error) {
	return app.NewHeartbeatEditSession(f.store, f.notebooks, f.auth, fixedClock{at}).Execute(f.asAlice(), id)
}

// end runs endEditSession on the session id as alice, at at.
func (f *fixture) end(id uuid.UUID, at time.Time) error {
	return app.NewEndEditSession(f.tx, f.store, fixedClock{at}, f.enders, f.logger()).Execute(f.asAlice(), id)
}

// An opening finds the page unlocked, shares the workspace's row and the
// notebook's, decides on editing, locks the page's gate, and only then
// asks the vetoers and writes the session: alice's, from the web, alive a
// lease from the unit's time. It writes no changeset and tells no observer.
func TestOpenEditSessionLocksThenDecidesThenAsksTheVetoers(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionEdit)
	v, o := &vetoer{recorder: f.rec}, &observer{recorder: f.rec}
	f.vetoers, f.observers = []app.EditSessionVetoer{v}, []app.PageObserver{o}
	n := f.page("Notes", nil, 0)
	s, err := f.open(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"FindNode", "WorkspaceOf", "ShareWorkspace in tx", "ShareNotebook in tx", "Authorize page.edit in tx",
		"LockContent in tx", "VetoEditSession in tx", "CreateSession in tx"}
	if !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v", f.rec.calls, want)
	}
	stored := f.store.sessions[s.ID]
	if s != stored || s.NodeID != n.ID || s.NotebookID != f.eng || s.UserID != f.alice || s.Client != domain.ClientWeb ||
		!s.CreatedAt.Equal(now()) || !s.ExpiresAt.Equal(now().Add(domain.EditSessionLease)) || s.ChangesetID != (uuid.UUID{}) {
		t.Errorf("session = %+v, stored %+v; want alice's of Notes from the web, alive until %v", s, stored, now().Add(domain.EditSessionLease))
	}
	if len(v.openings) != 1 || v.openings[0].PageID != n.ID || v.openings[0].By != f.alice || v.openings[0].NotebookID != f.eng {
		t.Errorf("the vetoer saw %+v, want alice's opening of Notes", v.openings)
	}
	if len(f.store.changesets) != 0 || len(o.events) != 0 {
		t.Errorf("an opening wrote %d changesets, told %d events; want none", len(f.store.changesets), len(o.events))
	}
}

// An opening's codes: the page, the decision, the page gone under the
// lock, the vetoers last.
func TestOpenEditSessionAnswersItsCodesInOrder(t *testing.T) {
	locked := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	for _, tt := range []struct {
		name  string
		setup func(f *fixture, n domain.Node) uuid.UUID
		want  string
	}{
		{"an unknown page", func(f *fixture, n domain.Node) uuid.UUID { return uuid.NewV7() }, "page.not_found"},
		{"a notebook the caller cannot see", func(f *fixture, n domain.Node) uuid.UUID { return n.ID }, "page.not_found"},
		{"a reader", func(f *fixture, n domain.Node) uuid.UUID {
			f.auth.forbidden[domain.ActionEdit] = true
			f.vetoers = []app.EditSessionVetoer{&vetoer{recorder: f.rec, err: locked}}
			return n.ID
		}, "forbidden"},
		{"a page deleted while it waited", func(f *fixture, n domain.Node) uuid.UUID {
			f.grant(domain.ActionEdit)
			f.vetoers = []app.EditSessionVetoer{&vetoer{recorder: f.rec, err: locked}}
			delete(f.store.contents, n.ID)
			return n.ID
		}, "page.not_found"},
		{"a vetoer last", func(f *fixture, n domain.Node) uuid.UUID {
			f.grant(domain.ActionEdit)
			f.vetoers = []app.EditSessionVetoer{&vetoer{recorder: f.rec, err: locked}}
			return n.ID
		}, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			n := f.page("Notes", nil, 0)
			_, err := f.open(tt.setup(f, n))
			if got := codeOf(err); got != tt.want {
				t.Errorf("openEditSession = %q, want %q", got, tt.want)
			}
			if len(f.store.sessions) != 0 || f.logs.Len() != 0 {
				t.Errorf("a refused opening left %d sessions, logged %q; want neither", len(f.store.sessions), f.logs)
			}
		})
	}
}

// A heartbeat keeps the caller's session alive a lease from the
// heartbeat's time, a moment before it would expire; at its expiry it is
// gone. It reads the session and decides on editing, holding no lock.
func TestAHeartbeatKeepsTheLeaseFromItsTime(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionEdit)
	n := f.page("Notes", nil, 0)
	opened, err := f.open(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.rec.calls = nil
	almost := opened.ExpiresAt.Add(-time.Microsecond)
	s, err := f.beat(opened.ID, almost)
	if err != nil || !s.ExpiresAt.Equal(almost.Add(domain.EditSessionLease)) {
		t.Fatalf("a heartbeat a moment before the expiry = %+v, %v; want it alive until %v", s, err, almost.Add(domain.EditSessionLease))
	}
	if want := []string{"FindLiveSession", "WorkspaceOf", "Authorize page.edit", "HeartbeatSession"}; !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v, none in a transaction", f.rec.calls, want)
	}
	if _, err := f.beat(opened.ID, s.ExpiresAt); codeOf(err) != "page.edit_session_not_found" {
		t.Errorf("a heartbeat at the new expiry = %v, want page.edit_session_not_found", err)
	}
}

// A heartbeat's codes: a session missing, expired, someone else's or of a
// notebook not seen is not found; a reader's is forbidden.
func TestAHeartbeatAnswersItsCodesInOrder(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(f *fixture, n domain.Node) uuid.UUID
		want  string
	}{
		{"a session missing", func(f *fixture, n domain.Node) uuid.UUID { return uuid.NewV7() }, "page.edit_session_not_found"},
		{"someone else's", func(f *fixture, n domain.Node) uuid.UUID {
			return f.session(n.ID, uuid.NewV7(), now().Add(time.Minute)).ID
		}, "page.edit_session_not_found"},
		{"expired at now", func(f *fixture, n domain.Node) uuid.UUID {
			return f.session(n.ID, f.alice, now()).ID
		}, "page.edit_session_not_found"},
		{"of a notebook not seen", func(f *fixture, n domain.Node) uuid.UUID {
			f.auth.grants[domain.ActionEdit] = false
			return f.session(n.ID, f.alice, now().Add(time.Minute)).ID
		}, "page.edit_session_not_found"},
		{"of a notebook deleted", func(f *fixture, n domain.Node) uuid.UUID {
			delete(f.notebooks.workspaces, f.eng)
			return f.session(n.ID, f.alice, now().Add(time.Minute)).ID
		}, "page.edit_session_not_found"},
		{"a reader's", func(f *fixture, n domain.Node) uuid.UUID {
			f.auth.grants[domain.ActionEdit] = false
			f.auth.forbidden[domain.ActionEdit] = true
			return f.session(n.ID, f.alice, now().Add(time.Minute)).ID
		}, "forbidden"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionEdit)
			n := f.page("Notes", nil, 0)
			id := tt.setup(f, n)
			before := f.store.sessions[id]
			_, err := f.beat(id, now())
			if got := codeOf(err); got != tt.want {
				t.Errorf("heartbeatEditSession = %q, want %q", got, tt.want)
			}
			if after := f.store.sessions[id]; after != before {
				t.Errorf("a refused heartbeat changed the session to %+v", after)
			}
		})
	}
}

// An end deletes the caller's alive session and tells the subscribers why,
// who and when, in one transaction; it decides on nothing. A session
// missing, expired or someone else's is not found and stays.
func TestAnEndTellsTheSubscribers(t *testing.T) {
	f := newFixture()
	sub := &subscriber{recorder: f.rec}
	f.enders = []app.EditSessionSubscriber{sub}
	n := f.page("Notes", nil, 0)
	s := f.session(n.ID, f.alice, now().Add(time.Minute))
	if err := f.end(s.ID, now()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"EndSession in tx", "EditSessionEnded in tx"}; !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v", f.rec.calls, want)
	}
	want := []app.SessionEnded{{SessionID: s.ID, NotebookID: f.eng, PageID: n.ID, UserID: f.alice, Reason: domain.EndedByOwner, By: f.alice, At: now()}}
	if !reflect.DeepEqual(sub.ended, want) || len(f.store.sessions) != 0 {
		t.Errorf("the subscriber followed %+v, %d sessions left; want %+v and none", sub.ended, len(f.store.sessions), want)
	}
	if logs := f.logs.String(); !strings.Contains(logs, "edit session ended") || !strings.Contains(logs, s.ID.String()) {
		t.Errorf("log %q, want the end with the session's id", logs)
	}
	for name, s := range map[string]app.EditSession{
		"someone else's": f.session(n.ID, uuid.NewV7(), now().Add(time.Minute)), "expired": f.session(n.ID, f.alice, now()),
	} {
		if err := f.end(s.ID, now()); codeOf(err) != "page.edit_session_not_found" || f.store.sessions[s.ID] != s {
			t.Errorf("an end of %s = %v, want page.edit_session_not_found and the session kept", name, err)
		}
	}
	if err := f.end(uuid.NewV7(), now()); codeOf(err) != "page.edit_session_not_found" || len(sub.ended) != 1 {
		t.Errorf("an end of a session missing = %v, told %d; want page.edit_session_not_found, nothing told", err, len(sub.ended))
	}
}

// A subscriber's error rolls the end back and is its answer.
func TestASubscribersErrorRollsTheEndBack(t *testing.T) {
	f := newFixture()
	down := errors.New("the event stream is down")
	f.enders = []app.EditSessionSubscriber{&subscriber{recorder: f.rec, err: down}}
	n := f.page("Notes", nil, 0)
	s := f.session(n.ID, f.alice, now().Add(time.Minute))
	if err := f.end(s.ID, now()); !errors.Is(err, down) || !f.tx.rolledBack || f.logs.Len() != 0 {
		t.Errorf("end = %v, rolled back %v, logged %q; want the subscriber's error, rolled back, nothing logged", err, f.tx.rolledBack, f.logs)
	}
}

// Deleting a subtree deletes its pages' sessions after the nodes, and
// tells the subscribers of the ones alive at the unit's time, by the
// deleter; the expired one ended with its lease and is not told.
func TestDeletingASubtreeEndsItsSessions(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionDelete)
	sub := &subscriber{recorder: f.rec}
	f.enders = []app.EditSessionSubscriber{sub}
	root := f.page("Root", nil, 0)
	child := f.page("Child", &root.ID, 0)
	other := f.page("Other", nil, 1)
	bob := uuid.NewV7()
	alive := f.session(child.ID, bob, now().Add(time.Minute))
	f.session(root.ID, f.alice, now())
	kept := f.session(other.ID, f.alice, now().Add(time.Minute))
	if err := app.NewDeleteNode(f.writer(), f.store, f.logger()).Execute(f.asAlice(), root.ID, domain.ClientWeb); err != nil {
		t.Fatal(err)
	}
	if i, j := slices.Index(f.rec.calls, "DeleteNodes in tx"), slices.Index(f.rec.calls, "DeleteNodeSessions in tx"); i < 0 || j < i {
		t.Errorf("calls = %v, want the sessions deleted after the nodes", f.rec.calls)
	}
	want := []app.SessionEnded{{SessionID: alive.ID, NotebookID: f.eng, PageID: child.ID, UserID: bob, Reason: domain.EndedWithPage,
		By: f.alice, At: now()}}
	if !reflect.DeepEqual(sub.ended, want) {
		t.Errorf("the subscriber followed %+v, want %+v", sub.ended, want)
	}
	if len(f.store.sessions) != 1 || f.store.sessions[kept.ID] != kept {
		t.Errorf("sessions left %+v, want Other's alone", f.store.sessions)
	}
}

// The notebook module's deletion deletes the notebooks' sessions after
// their pages, and tells the subscribers of the alive ones, by the
// deletion's actor at its time.
func TestANotebookDeletionEndsItsSessions(t *testing.T) {
	f := newFixture()
	sub := &subscriber{recorder: f.rec}
	n := f.page("Notes", nil, 0)
	alive := f.session(n.ID, f.alice, now().Add(time.Minute))
	f.session(n.ID, f.alice, now())
	by, at := uuid.NewV7(), now().Add(time.Second)
	d := app.NotebookDeletion{Pages: f.store, Subscribers: []app.EditSessionSubscriber{sub}}
	if err := d.NotebookDeleted(context.Background(), app.NotebookDeleted{WorkspaceID: f.acme, NotebookIDs: []uuid.UUID{f.eng}, By: by, At: at}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"DeleteNotebooksPages", "DeleteNotebookSessions", "EditSessionEnded"}; !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v", f.rec.calls, want)
	}
	want := []app.SessionEnded{{SessionID: alive.ID, NotebookID: f.eng, PageID: n.ID, UserID: f.alice, Reason: domain.EndedWithPage, By: by, At: at}}
	if !reflect.DeepEqual(sub.ended, want) || len(f.store.sessions) != 0 {
		t.Errorf("the subscriber followed %+v, %d sessions left; want %+v, none", sub.ended, len(f.store.sessions), want)
	}
}

// The cleanup deletes the sessions expired at its time, batch after batch
// until one comes back short, skips a held one, tells no subscriber, and
// logs how many when it deleted any.
func TestTheCleanupDeletesTheExpiredSessions(t *testing.T) {
	f := newFixture()
	n := f.page("Notes", nil, 0)
	for range 1500 {
		f.session(n.ID, f.alice, now())
	}
	held := f.session(n.ID, f.alice, now().Add(-time.Second))
	f.store.held[held.ID] = true
	alive := f.session(n.ID, f.alice, now().Add(time.Microsecond))
	deleted, err := app.NewCleanupEditSessions(f.store, fixedClock{now()}, f.logger()).Execute(context.Background())
	if err != nil || deleted != 1500 {
		t.Errorf("cleanup = %d, %v; want the 1500 expired", deleted, err)
	}
	if want := []string{"DeleteExpiredSessions", "DeleteExpiredSessions"}; !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want a full batch, then a short one", f.rec.calls)
	}
	if len(f.store.sessions) != 2 || f.store.sessions[alive.ID] != alive || f.store.sessions[held.ID] != held {
		t.Errorf("sessions left %d, want the held one and the alive one", len(f.store.sessions))
	}
	if logs := f.logs.String(); !strings.Contains(logs, "expired edit sessions deleted") || !strings.Contains(logs, "deleted=1500") {
		t.Errorf("log %q, want how many were deleted", logs)
	}
	f.logs.Reset()
	if deleted, err := app.NewCleanupEditSessions(f.store, fixedClock{now()}, f.logger()).Execute(context.Background()); err != nil ||
		deleted != 0 || f.logs.Len() != 0 {
		t.Errorf("a cleanup with nothing to delete = %d, %v, logged %q; want 0 and no log", deleted, err, f.logs)
	}
}

// The opening's log tells the session's ids, never the title.
func TestOpenEditSessionLogsTheIDs(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionEdit)
	n := f.page("Zebrafish", nil, 0)
	s, err := f.open(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	logs := f.logs.String()
	for _, want := range []string{"edit session opened", s.ID.String(), f.acme.String(), f.eng.String(), n.ID.String(), f.alice.String(), "client=web"} {
		if !strings.Contains(logs, want) {
			t.Errorf("log %q lacks %q", logs, want)
		}
	}
	if strings.Contains(logs, "Zebrafish") {
		t.Errorf("log %q tells the title", logs)
	}
}
