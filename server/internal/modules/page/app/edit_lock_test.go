package app_test

import (
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// lockOf is the lock member of err, a page.locked: the page and the
// holder's name; ok is false for any other error, or none, and for a
// holder whose id names says is not theirs.
func lockOf(err error, names fakeNames) (page uuid.UUID, holder string, ok bool) {
	var e *shared.Error
	if !errors.Is(err, domain.ErrLocked) || !errors.As(err, &e) || e.Lock == nil ||
		names[e.Lock.UserID] != e.Lock.DisplayName {
		return uuid.UUID{}, "", false
	}
	return e.Lock.PageID, e.Lock.DisplayName, true
}

// lockFixture is a tree Top > Mid > (Low, Side), and Free beside it: alice
// holds Top, bob holds Low and Side (Side's session the older), Mid's
// session expires at now, and Free's is a tombstone.
type lockFixture struct {
	*fixture
	bob                       uuid.UUID
	top, mid, low, side, free domain.Node
	alices                    app.EditSession
	lock                      *app.EditLock
}

func newLockFixture() lockFixture {
	f := lockFixture{fixture: newFixture(), bob: uuid.NewV7()}
	f.names[f.alice], f.names[f.bob] = "Alice", "Bob"
	f.top = f.page("Top", nil, 0)
	f.mid = f.page("Mid", &f.top.ID, 0)
	f.low = f.page("Low", &f.mid.ID, 0)
	f.side = f.page("Side", &f.mid.ID, 1)
	f.free = f.page("Free", nil, 1)
	f.alices = f.session(f.top.ID, f.alice, now().Add(time.Minute))
	f.session(f.low.ID, f.bob, now().Add(time.Minute))
	side := f.session(f.side.ID, f.bob, now().Add(time.Minute))
	side.CreatedAt = side.CreatedAt.Add(-time.Second)
	f.store.sessions[side.ID] = side
	f.session(f.mid.ID, f.bob, now())
	tomb := f.session(f.free.ID, f.bob, now().Add(time.Hour))
	tomb.EndedReason, tomb.EndedByID, tomb.EndedAt = domain.EndedUnlocked, f.alice, now().Add(-time.Second)
	f.store.sessions[tomb.ID] = tomb
	f.lock = app.NewEditLock(f.store, f.names)
	return f
}

// step is an operation by by at now on the nodes ids, in the session.
func step(op domain.Operation, by, session uuid.UUID, ids ...uuid.UUID) app.Step {
	changes := make([]domain.Change, len(ids))
	for i, id := range ids {
		changes[i] = domain.Change{NodeID: id}
	}
	return app.Step{Write: app.Write{By: by, At: now()}, Operation: op, Changes: changes, EditSessionID: session}
}

// The guard by operation (M5 design 4.4): a content write passes in the
// page's alive session, or when none is alive, and is refused in another
// or in none while one is; a deletion is refused for another account's
// alive session in it, naming the first in the subtree's order, level by
// level, not the oldest, and passes for the deleter's own, but for an
// agent's; the tree's other writes pass, a restore among them. The time
// is the step's.
func TestTheEditLockGuardsByOperation(t *testing.T) {
	f := newLockFixture()
	var none uuid.UUID
	later := step(domain.OpContent, f.bob, none, f.top.ID)
	later.At = f.alices.ExpiresAt
	agents := func(s app.Step) app.Step {
		s.Client = domain.MCPClient("x")
		return s
	}
	for _, tt := range []struct {
		name   string
		step   app.Step
		page   uuid.UUID
		holder string
	}{
		{"content in the lock's session", step(domain.OpContent, f.alice, f.alices.ID, f.top.ID), none, ""},
		{"content in no session while locked", step(domain.OpContent, f.alice, none, f.top.ID), f.top.ID, "Alice"},
		{"content in another session while locked", step(domain.OpContent, f.alice, uuid.NewV7(), f.top.ID), f.top.ID, "Alice"},
		{"someone else's content while locked", step(domain.OpContent, f.bob, none, f.top.ID), f.top.ID, "Alice"},
		{"content once the lease ran out", later, none, ""},
		{"content of a page whose session expired", step(domain.OpContent, f.alice, none, f.mid.ID), none, ""},
		{"content of a page whose session is a tombstone", step(domain.OpContent, f.alice, none, f.free.ID), none, ""},
		{"a deletion of what another holds", step(domain.OpDelete, f.alice, none, f.mid.ID, f.low.ID, f.side.ID), f.low.ID, "Bob"},
		{"a deletion of a subtree held at its top", step(domain.OpDelete, f.bob, none, f.top.ID, f.mid.ID, f.low.ID, f.side.ID),
			f.top.ID, "Alice"},
		{"a deletion of the deleter's own", step(domain.OpDelete, f.alice, none, f.top.ID), none, ""},
		{"a deletion of an expired session's page", step(domain.OpDelete, f.alice, none, f.mid.ID), none, ""},
		{"a deletion of a tombstone's page", step(domain.OpDelete, f.alice, none, f.free.ID), none, ""},
		// An agent's deletion is refused by its caller's own too (M9/P2 design 3.6).
		{"an agent's deletion of its caller's own", agents(step(domain.OpDelete, f.alice, none, f.top.ID)), f.top.ID, "Alice"},
		{"an agent's deletion of what none holds", agents(step(domain.OpDelete, f.alice, none, f.mid.ID, f.free.ID)), none, ""},
		{"a rename of a locked page", step(domain.OpRename, f.bob, none, f.top.ID), none, ""},
		{"a move of a locked page", step(domain.OpMove, f.bob, none, f.top.ID), none, ""},
		{"a creation under a locked page", step(domain.OpCreate, f.bob, none, uuid.NewV7()), none, ""},
		// A restore passes whatever the pages' sessions: a trash item's were deleted with it (M8 design 8).
		{"a restore of a page another holds", step(domain.OpRestore, f.bob, none, f.top.ID, f.low.ID), none, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := f.lock.GuardWrite(f.asAlice(), tt.step)
			page, holder, locked := lockOf(err, f.names)
			switch {
			case tt.page == none && err != nil:
				t.Errorf("GuardWrite = %v, want it to pass", err)
			case tt.page != none && (!locked || page != tt.page || holder != tt.holder):
				t.Errorf("GuardWrite = %v (page %s, %q), want page.locked of %s by %s", err, page, holder, tt.page, tt.holder)
			}
		})
	}
}

// An agent's deletion is refused by any alive session in it, its caller's
// own too, naming each page held in the step's order, the subtree's level
// by level, not the pages' ids' (M9/P2 design 3.6; the pages' ids go Top,
// Mid, Low, Side, the step's Top, Mid, Side, Low): the first its lock, all
// its locks, each with its holder's name. The web's deletion of the same
// step meets the first page another account holds and names it alone. An
// agent's deletion of what no alive session holds passes; its content
// write is refused as any is; its creation, rename and move pass.
func TestTheEditLockRefusesAnAgentsDeletionByAnySession(t *testing.T) {
	f := newLockFixture()
	from := func(client domain.Client, op domain.Operation, ids ...uuid.UUID) app.Step {
		s := step(op, f.alice, uuid.UUID{}, ids...)
		s.Client = client
		return s
	}
	agent := domain.MCPClient("x")
	order := []uuid.UUID{f.top.ID, f.mid.ID, f.side.ID, f.low.ID}
	err := f.lock.GuardWrite(f.asAlice(), from(agent, domain.OpDelete, order...))
	var e *shared.Error
	want := []shared.LockHolder{
		{PageID: f.top.ID, UserID: f.alice, DisplayName: "Alice"},
		{PageID: f.side.ID, UserID: f.bob, DisplayName: "Bob"},
		{PageID: f.low.ID, UserID: f.bob, DisplayName: "Bob"},
	}
	if !errors.Is(err, domain.ErrLocked) || !errors.As(err, &e) || e.Lock == nil || *e.Lock != want[0] || !slices.Equal(e.Locks, want) {
		t.Errorf("an agent's deletion = %v, %+v; want page.locked by %+v", err, e, want)
	}
	err = f.lock.GuardWrite(f.asAlice(), from(domain.ClientWeb, domain.OpDelete, order...))
	if page, holder, locked := lockOf(err, f.names); !locked || page != f.side.ID || holder != "Bob" || !errors.As(err, &e) || e.Locks != nil {
		t.Errorf("the web's deletion = %v, want page.locked by Bob's Side alone", err)
	}
	if err := f.lock.GuardWrite(f.asAlice(), from(agent, domain.OpDelete, f.free.ID)); err != nil {
		t.Errorf("an agent's deletion of what no alive session holds = %v, want it to pass", err)
	}
	if page, holder, locked := lockOf(f.lock.GuardWrite(f.asAlice(), from(agent, domain.OpContent, f.top.ID)), f.names); !locked ||
		page != f.top.ID || holder != "Alice" {
		t.Errorf("an agent's content write of a held page = %s %q %v, want page.locked by Alice", page, holder, locked)
	}
	for _, op := range []domain.Operation{domain.OpCreate, domain.OpRename, domain.OpMove} {
		if err := f.lock.GuardWrite(f.asAlice(), from(agent, op, f.top.ID)); err != nil {
			t.Errorf("an agent's %s of a held page = %v, want it to pass", op, err)
		}
	}
}

// The vetoer refuses an opening of a page an alive session holds, naming
// its holder, whoever opens it; an expired session or a tombstone holds
// nothing, and the time is the opening's.
func TestTheEditLockVetoesAnOpeningOfAHeldPage(t *testing.T) {
	f := newLockFixture()
	opening := func(by, page uuid.UUID, at time.Time) app.SessionOpening {
		return app.SessionOpening{Write: app.Write{By: by, At: at}, PageID: page}
	}
	for _, tt := range []struct {
		name    string
		opening app.SessionOpening
		holder  string
	}{
		{"by someone else", opening(f.bob, f.top.ID, now()), "Alice"},
		{"by the holder", opening(f.alice, f.top.ID, now()), "Alice"},
		{"once the lease ran out", opening(f.bob, f.top.ID, f.alices.ExpiresAt), ""},
		{"of an expired session's page", opening(f.alice, f.mid.ID, now()), ""},
		{"of a tombstone's page", opening(f.alice, f.free.ID, now()), ""},
	} {
		err := f.lock.VetoEditSession(f.asAlice(), tt.opening)
		page, holder, locked := lockOf(err, f.names)
		switch {
		case tt.holder == "" && err != nil:
			t.Errorf("%s: VetoEditSession = %v, want it to pass", tt.name, err)
		case tt.holder != "" && (!locked || page != tt.opening.PageID || holder != tt.holder):
			t.Errorf("%s: VetoEditSession = %v (page %s, %q), want page.locked by %s", tt.name, err, page, holder, tt.holder)
		}
	}
}
