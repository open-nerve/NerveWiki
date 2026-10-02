package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// HeartbeatEditSession keeps an edit session alive: POST
// /api/v0/edit-sessions/{edit_session_id}/heartbeat (M4/P4 design 3.5).
type HeartbeatEditSession struct {
	sessions  Sessions
	notebooks Notebooks
	auth      shared.Authorizer
	clock     Clock
}

// NewHeartbeatEditSession returns the use case.
func NewHeartbeatEditSession(sessions Sessions, notebooks Notebooks, auth shared.Authorizer, clock Clock) *HeartbeatEditSession {
	return &HeartbeatEditSession{sessions: sessions, notebooks: notebooks, auth: auth, clock: clock}
}

// Execute keeps the caller's session id alive until EditSessionLease from
// now. It holds no workspace's or notebook's row (M4 design 4, "locks"):
// it reads the session and its notebook unlocked, decides on editing it as
// a read does, and moves the lease in one statement on the session's row,
// which finds it alive and the caller's again. A session that is missing,
// expired, someone else's, or of a notebook the caller sees no more is
// page.edit_session_not_found; one the caller may only read now is
// forbidden: its lease runs out.
func (h *HeartbeatEditSession) Execute(ctx context.Context, id uuid.UUID) (EditSession, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return EditSession{}, err
	}
	now := h.clock.Now()
	s, err := h.sessions.FindLiveSession(ctx, id, actor.UserID, now)
	if err != nil {
		return EditSession{}, found(err, domain.ErrEditSessionNotFound)
	}
	workspaceID, ok, err := h.notebooks.WorkspaceOf(ctx, s.NotebookID)
	switch {
	case err != nil:
		return EditSession{}, err
	case !ok:
		return EditSession{}, domain.ErrEditSessionNotFound
	}
	target := shared.Target{WorkspaceID: workspaceID, NotebookID: s.NotebookID}
	if _, err := authorize(ctx, h.auth, actor, domain.ActionEdit, target, domain.ErrEditSessionNotFound); err != nil {
		return EditSession{}, err
	}
	s, err = h.sessions.HeartbeatSession(ctx, id, actor.UserID, now, now.Add(domain.EditSessionLease))
	if err != nil {
		return EditSession{}, found(err, domain.ErrEditSessionNotFound)
	}
	return s, nil
}
