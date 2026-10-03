package app

import (
	"context"
	"errors"
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
	names     Names
	clock     Clock
}

// NewHeartbeatEditSession returns the use case.
func NewHeartbeatEditSession(sessions Sessions, notebooks Notebooks, auth shared.Authorizer, names Names,
	clock Clock,
) *HeartbeatEditSession {
	return &HeartbeatEditSession{sessions: sessions, notebooks: notebooks, auth: auth, names: names, clock: clock}
}

// Execute keeps the caller's session id alive until EditSessionLease from
// now. It holds no workspace's or notebook's row (M4 design 4, "locks"):
// it reads the session and its notebook unlocked, decides on editing it as
// a read does, and moves the lease in one statement on the session's row,
// which finds it alive and the caller's again. The caller's tombstone,
// found at either step, says why it ended (M5 design 4.3), once decided on
// as an alive session is: the update waits behind a take-over or an
// unlock of the row, and finds it ended. A session that is missing,
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
		return EditSession{}, h.notAlive(ctx, err, actor, id)
	}
	if err := h.decide(ctx, actor, s.NotebookID); err != nil {
		return EditSession{}, err
	}
	s, err = h.sessions.HeartbeatSession(ctx, id, actor.UserID, now, now.Add(domain.EditSessionLease))
	if err != nil {
		return EditSession{}, h.notAlive(ctx, err, actor, id)
	}
	return s, nil
}

// decide decides on editing in the notebook notebookID, unlocked, as a
// read does: page.edit_session_not_found for a notebook gone or not seen.
func (h *HeartbeatEditSession) decide(ctx context.Context, actor shared.Actor, notebookID uuid.UUID) error {
	workspaceID, ok, err := h.notebooks.WorkspaceOf(ctx, notebookID)
	switch {
	case err != nil:
		return err
	case !ok:
		return domain.ErrEditSessionNotFound
	}
	target := shared.Target{WorkspaceID: workspaceID, NotebookID: notebookID}
	_, err = authorize(ctx, h.auth, actor, domain.ActionEdit, target, domain.ErrEditSessionNotFound)
	return err
}

// notAlive is the answer when err found no alive session id of the
// caller's: why it ended when it is their tombstone and they may still
// edit its notebook, so that no one who may not learns who released it
// (M5/P1 review m2); page.edit_session_not_found otherwise.
func (h *HeartbeatEditSession) notAlive(ctx context.Context, err error, actor shared.Actor, id uuid.UUID) error {
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	s, err := h.sessions.FindEndedSession(ctx, id, actor.UserID)
	if err != nil {
		return found(err, domain.ErrEditSessionNotFound)
	}
	if err := h.decide(ctx, actor, s.NotebookID); err != nil {
		return err
	}
	return endedError(ctx, h.names, s)
}
