package app

import (
	"context"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// EditLock is the edit lock (M5 design 4.1, 4.4): a page's alive edit
// session holds it. As a vetoer it refuses an opening of a locked page; as
// a guard, a content write in another session than the lock's, a deletion
// of pages that another account's sessions hold, and an agent's deletion
// of pages any session holds (M9 design 4.10). Its reads take no lock: the
// unit that runs it holds what serializes it with an opening, the page's
// content row for a content write and an opening, the notebook's row FOR
// NO KEY UPDATE for a deletion.
type EditLock struct {
	sessions AliveSessions
	names    Names
}

// NewEditLock returns the lock.
func NewEditLock(sessions AliveSessions, names Names) *EditLock {
	return &EditLock{sessions: sessions, names: names}
}

// VetoEditSession refuses the opening of a page that has an alive session:
// page.locked, naming it. A take-over has ended the opener's own before
// (Unit.OpenSession), so the one left is someone else's, or the opener's
// when they did not take it over.
func (l *EditLock) VetoEditSession(ctx context.Context, o SessionOpening) error {
	alive, err := l.sessions.AliveSessionsOf(ctx, []uuid.UUID{o.PageID}, o.At)
	if err != nil || len(alive) == 0 {
		return err
	}
	return l.locked(ctx, alive[0])
}

// GuardWrite refuses a content write when the page's alive session is not
// the one it is made in; a deletion when another account's alive session
// holds a page of it, naming the first in the subtree's order, the
// deleter's own sessions ending with their pages; and an agent's deletion
// when any alive session holds one, its caller's own too, naming each in
// the subtree's order: an agent ends no edit (M9 design 4.10). Any other
// operation passes: it does not touch content.
func (l *EditLock) GuardWrite(ctx context.Context, s Step) error {
	agent := s.Operation == domain.OpDelete && s.Client.IsMCP()
	var holds func(EditSession) bool
	switch {
	case s.Operation == domain.OpContent:
		holds = func(a EditSession) bool { return a.ID != s.EditSessionID }
	case agent:
		holds = func(EditSession) bool { return true }
	case s.Operation == domain.OpDelete:
		holds = func(a EditSession) bool { return a.UserID != s.By }
	default:
		return nil
	}
	ids := make([]uuid.UUID, len(s.Changes))
	for i, c := range s.Changes {
		ids[i] = c.NodeID
	}
	alive, err := l.sessions.AliveSessionsOf(ctx, ids, s.At)
	if err != nil {
		return err
	}
	held := make(map[uuid.UUID]EditSession)
	for _, a := range alive {
		if _, ok := held[a.NodeID]; !ok && holds(a) {
			held[a.NodeID] = a
		}
	}
	if agent {
		return l.lockedAll(ctx, ids, held)
	}
	for _, id := range ids {
		if a, ok := held[id]; ok {
			return l.locked(ctx, a)
		}
	}
	return nil
}

// lockedAll is page.locked naming every page of ids that held holds, in
// ids' order (the subtree's: level by level), their owners named in one
// read: the first is its Lock, all its Locks. Nil when held holds none.
func (l *EditLock) lockedAll(ctx context.Context, ids []uuid.UUID, held map[uuid.UUID]EditSession) error {
	var sessions []EditSession
	var users []uuid.UUID
	for _, id := range ids {
		if a, ok := held[id]; ok {
			sessions = append(sessions, a)
			if !slices.Contains(users, a.UserID) {
				users = append(users, a.UserID)
			}
		}
	}
	if len(sessions) == 0 {
		return nil
	}
	names, err := l.names.DisplayNames(ctx, users)
	if err != nil {
		return err
	}
	holders := make([]shared.LockHolder, len(sessions))
	for i, a := range sessions {
		holders[i] = shared.LockHolder{PageID: a.NodeID, UserID: a.UserID, DisplayName: names[a.UserID]}
	}
	return domain.LockedAll(holders)
}

// locked is page.locked naming s's page and owner.
func (l *EditLock) locked(ctx context.Context, s EditSession) error {
	names, err := l.names.DisplayNames(ctx, []uuid.UUID{s.UserID})
	if err != nil {
		return err
	}
	return domain.Locked(s.NodeID, s.UserID, names[s.UserID])
}
