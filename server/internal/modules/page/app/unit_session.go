package app

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Opening an edit session and releasing a page's lock in a unit (M4/P4
// design 3.4; M5 design 4.2, 4.3), and telling the sessions' ends and
// openings.

// OpenSession opens the writer's edit session of the page id under its
// gate, the content row's lock: 404 for a page that is not in the
// notebook. The page's expired rows go first, so that a heartbeat that
// read an earlier time finds no row to keep alive (M5 design 4.1); a
// take-over then ends the writer's own alive sessions of the page; then
// the vetoers, which may refuse it (M5: an alive session of the page). The
// session lives EditSessionLease from the unit's time, and the
// subscribers follow its opening. It writes no changeset and tells no
// observer: the session's writes do.
func (u *Unit) OpenSession(ctx context.Context, id uuid.UUID, takeOver bool) (EditSession, error) {
	if err := u.inUnit(ctx); err != nil {
		return EditSession{}, err
	}
	if _, err := u.w.d.Nodes.LockContent(ctx, u.write.NotebookID, id); err != nil {
		return EditSession{}, found(err, domain.ErrNotFound)
	}
	if err := u.w.d.SessionWriter.DeleteExpiredSessionsOf(ctx, id, u.write.At); err != nil {
		return EditSession{}, err
	}
	if takeOver {
		if _, err := u.endAlive(ctx, id, &u.write.By, domain.EndedTakenOver); err != nil {
			return EditSession{}, err
		}
	}
	for _, v := range u.w.d.SessionVetoers {
		if err := v.VetoEditSession(ctx, SessionOpening{Write: u.write, PageID: id, TakeOver: takeOver}); err != nil {
			return EditSession{}, err
		}
	}
	s := EditSession{
		ID: uuid.NewV7(), NodeID: id, NotebookID: u.write.NotebookID, UserID: u.write.By, Client: u.write.Client,
		CreatedAt: u.write.At, ExpiresAt: u.write.At.Add(domain.EditSessionLease),
	}
	if err := u.w.d.SessionWriter.CreateSession(ctx, s); err != nil {
		return EditSession{}, err
	}
	o := SessionOpened{SessionID: s.ID, WorkspaceID: u.write.WorkspaceID, NotebookID: s.NotebookID, PageID: id, UserID: s.UserID,
		At: u.write.At}
	for _, sub := range u.w.d.SessionSubscribers {
		if err := sub.EditSessionOpened(ctx, o); err != nil {
			return EditSession{}, err
		}
	}
	return s, nil
}

// Unlock ends whichever sessions hold the lock of the page id, the unit's
// writer releasing it, under its gate: 404 for a page that is not in the
// notebook. Each becomes a tombstone (M5 design 4.3), so that its tab
// learns who released it, and the subscribers follow its end. It returns
// how many ended: none for a page no session held.
func (u *Unit) Unlock(ctx context.Context, id uuid.UUID) (int, error) {
	if err := u.inUnit(ctx); err != nil {
		return 0, err
	}
	if _, err := u.w.d.Nodes.LockContent(ctx, u.write.NotebookID, id); err != nil {
		return 0, found(err, domain.ErrNotFound)
	}
	return u.endAlive(ctx, id, nil, domain.EndedUnlocked)
}

// endAlive makes tombstones of the page's sessions alive at the unit's
// time, of user alone when it is not nil, for reason by the unit's writer,
// kept a lease after it, and tells the subscribers: each was alive until
// now. It returns how many ended.
func (u *Unit) endAlive(ctx context.Context, id uuid.UUID, user *uuid.UUID, reason domain.EndReason) (int, error) {
	ended, err := u.w.d.SessionWriter.EndAliveSessions(ctx, SessionsEnd{
		NodeID: id, UserID: user, Reason: reason, By: u.write.By, At: u.write.At, Until: u.write.At.Add(domain.EditSessionLease),
	})
	if err != nil {
		return 0, err
	}
	return len(ended), tell(ctx, u.w.d.SessionSubscribers, u.write.WorkspaceID, ended, reason, u.write.By, u.write.At)
}

// tellEnded has the subscribers follow the end of each session, of the
// workspace workspaceID, alive at at, for reason, by by: an expired one
// ended with its lease, and a tombstone with its own end, which tell no
// one.
func tellEnded(ctx context.Context, subscribers []EditSessionSubscriber, workspaceID uuid.UUID, sessions []EditSession,
	reason domain.EndReason, by uuid.UUID, at time.Time,
) error {
	alive := make([]EditSession, 0, len(sessions))
	for _, s := range sessions {
		if s.Alive(at) {
			alive = append(alive, s)
		}
	}
	return tell(ctx, subscribers, workspaceID, alive, reason, by, at)
}

// tell has the subscribers follow the end of each session, which was alive
// until it ended at at.
func tell(ctx context.Context, subscribers []EditSessionSubscriber, workspaceID uuid.UUID, sessions []EditSession,
	reason domain.EndReason, by uuid.UUID, at time.Time,
) error {
	for _, s := range sessions {
		e := SessionEnded{SessionID: s.ID, WorkspaceID: workspaceID, NotebookID: s.NotebookID, PageID: s.NodeID, UserID: s.UserID,
			Reason: reason, By: by, At: at}
		for _, sub := range subscribers {
			if err := sub.EditSessionEnded(ctx, e); err != nil {
				return err
			}
		}
	}
	return nil
}

// endedError is why the caller's tombstone s ended, as its problem tells
// it (M5 design 4.3): page.edit_session_taken_over, or
// page.edit_session_unlocked naming who released the lock.
func endedError(ctx context.Context, names Names, s EditSession) error {
	switch s.EndedReason {
	case domain.EndedTakenOver:
		return domain.ErrEditSessionTakenOver
	case domain.EndedUnlocked:
		got, err := names.DisplayNames(ctx, []uuid.UUID{s.EndedByID})
		if err != nil {
			return err
		}
		return domain.Unlocked(s.EndedByID, got[s.EndedByID])
	}
	return fmt.Errorf("edit session %s ended for %q: no such reason", s.ID, s.EndedReason)
}
