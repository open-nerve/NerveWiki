package app

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// OpenStream opens the caller's event stream: GET /api/v0/events (M5
// design 4.10).
type OpenStream struct {
	hub        *Hub
	visibility Visibility
}

// NewOpenStream returns the use case.
func NewOpenStream(hub *Hub, visibility Visibility) *OpenStream {
	return &OpenStream{hub: hub, visibility: visibility}
}

// Execute registers the caller's stream on the hub first, then reads what
// the caller sees, without locks, and settles the stream with it: an
// access or notebooks_deleted event that came in between and concerns the
// caller resets it, the others it sees wait for it. Reading first would
// leave a window in which what the caller sees could change unnoticed.
// While the hub does not listen it is not_ready's 503; a read that
// fails closes the stream. The caller closes the stream it gets.
func (o *OpenStream) Execute(ctx context.Context) (*Stream, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	s, err := o.hub.open(actor.UserID)
	if err != nil {
		return nil, err
	}
	v, err := o.seenBy(ctx, actor.UserID)
	if err != nil {
		s.Close()
		return nil, err
	}
	s.settle(v)
	return s, nil
}

// seenBy reads the workspaces userID is a member of and the notebooks it
// may read in each.
func (o *OpenStream) seenBy(ctx context.Context, userID uuid.UUID) (seen, error) {
	memberships, err := o.visibility.WorkspacesOf(ctx, userID)
	if err != nil {
		return seen{}, fmt.Errorf("events: the workspaces of %s: %w", userID, err)
	}
	v := seen{workspaces: make(map[uuid.UUID]bool, len(memberships)), notebooks: make(map[uuid.UUID]bool)}
	for _, m := range memberships {
		v.workspaces[m.WorkspaceID] = true
		ids, err := o.visibility.NotebooksIn(ctx, m.WorkspaceID, userID, m.Role)
		if err != nil {
			return seen{}, fmt.Errorf("events: the notebooks of %s in %s: %w", userID, m.WorkspaceID, err)
		}
		for _, id := range ids {
			v.notebooks[id] = true
		}
	}
	return v, nil
}
