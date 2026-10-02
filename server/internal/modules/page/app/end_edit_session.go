package app

import (
	"context"
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
	clock       Clock
	subscribers []EditSessionSubscriber
	logger      *slog.Logger
}

// NewEndEditSession returns the use case.
func NewEndEditSession(tx Tx, sessions Sessions, clock Clock, subscribers []EditSessionSubscriber, logger *slog.Logger) *EndEditSession {
	return &EndEditSession{tx: tx, sessions: sessions, clock: clock, subscribers: subscribers, logger: logger}
}

// Execute ends the caller's session id: its row goes, and the subscribers
// follow, in one transaction that holds no workspace's or notebook's row.
// It asks only that the session is the caller's, alive: a session that is
// missing, expired, or someone else's is page.edit_session_not_found,
// which the editor takes for ended.
func (e *EndEditSession) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	now := e.clock.Now()
	var s EditSession
	err = e.tx.WithinTx(ctx, func(ctx context.Context) error {
		if s, err = e.sessions.EndSession(ctx, id, actor.UserID, now); err != nil {
			return found(err, domain.ErrEditSessionNotFound)
		}
		return tellEnded(ctx, e.subscribers, []EditSession{s}, domain.EndedByOwner, actor.UserID, now)
	})
	if err != nil {
		return err
	}
	e.logger.InfoContext(ctx, "edit session ended", sessionLogged(uuid.UUID{}, s)...)
	return nil
}
