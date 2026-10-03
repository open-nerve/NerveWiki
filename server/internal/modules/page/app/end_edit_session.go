package app

import (
	"context"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// EndEditSession ends an edit session: DELETE
// /api/v0/edit-sessions/{edit_session_id} (M4/P4 design 3.5).
type EndEditSession struct {
	tx          Tx
	sessions    Sessions
	notebooks   Notebooks
	clock       Clock
	subscribers []EditSessionSubscriber
	logger      *slog.Logger
}

// NewEndEditSession returns the use case.
func NewEndEditSession(tx Tx, sessions Sessions, notebooks Notebooks, clock Clock, subscribers []EditSessionSubscriber,
	logger *slog.Logger,
) *EndEditSession {
	return &EndEditSession{tx: tx, sessions: sessions, notebooks: notebooks, clock: clock, subscribers: subscribers, logger: logger}
}

// Execute ends the caller's session id: its row goes, and the subscribers
// follow, in one transaction that holds no workspace's or notebook's row:
// its notebook's workspace is read unlocked. It asks only that the session
// is the caller's, alive or a tombstone (M5 design 4.3); a tombstone's row
// goes too, and tells no one, its end told already. A session that is
// missing, expired, or someone else's is page.edit_session_not_found,
// which the editor takes for ended.
func (e *EndEditSession) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	now := e.clock.Now()
	var s EditSession
	var workspaceID uuid.UUID
	err = e.tx.WithinTx(ctx, func(ctx context.Context) error {
		if s, err = e.sessions.EndSession(ctx, id, actor.UserID, now); err != nil {
			return found(err, domain.ErrEditSessionNotFound)
		}
		// The notebook's deletion deletes its sessions: the row was there,
		// so is the notebook.
		var ok bool
		switch workspaceID, ok, err = e.notebooks.WorkspaceOf(ctx, s.NotebookID); {
		case err != nil:
			return err
		case !ok:
			return fmt.Errorf("the notebook of the edit session %s is not there", s.ID)
		}
		return tellEnded(ctx, e.subscribers, workspaceID, []EditSession{s}, domain.EndedByOwner, actor.UserID, now)
	})
	if err != nil {
		return err
	}
	e.logger.InfoContext(ctx, "edit session ended", sessionLogged(workspaceID, s)...)
	return nil
}
