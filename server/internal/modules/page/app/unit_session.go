package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Opening an edit session in a unit (M4/P4 design 3.4), and telling the
// sessions' ends.

// OpenSession opens the writer's edit session of the page id under its
// gate, the content row's lock: 404 for a page that is not in the
// notebook; then the vetoers, which may refuse it (M5: someone else's
// alive session). The session lives EditSessionLease from the unit's time.
// It writes no changeset and tells no observer: the session's writes do.
func (u *Unit) OpenSession(ctx context.Context, id uuid.UUID) (EditSession, error) {
	if err := u.inUnit(ctx); err != nil {
		return EditSession{}, err
	}
	if _, err := u.w.d.Nodes.LockContent(ctx, u.write.NotebookID, id); err != nil {
		return EditSession{}, found(err, domain.ErrNotFound)
	}
	for _, v := range u.w.d.SessionVetoers {
		if err := v.VetoEditSession(ctx, SessionOpening{Write: u.write, PageID: id}); err != nil {
			return EditSession{}, err
		}
	}
	s := EditSession{
		ID: uuid.NewV7(), NodeID: id, NotebookID: u.write.NotebookID, UserID: u.write.By, Client: u.write.Client,
		CreatedAt: u.write.At, ExpiresAt: u.write.At.Add(domain.EditSessionLease),
	}
	return s, u.w.d.SessionWriter.CreateSession(ctx, s)
}

// tellEnded has the subscribers follow the end of each session, of the
// workspace workspaceID, alive at at, for reason, by by: an expired one
// ended with its lease, which tells no one.
func tellEnded(ctx context.Context, subscribers []EditSessionSubscriber, workspaceID uuid.UUID, sessions []EditSession,
	reason domain.EndReason, by uuid.UUID, at time.Time,
) error {
	for _, s := range sessions {
		if !s.Alive(at) {
			continue
		}
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
