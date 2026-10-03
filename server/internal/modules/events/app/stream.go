package app

import (
	"slices"
	"sync"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
)

// Stream is one open event stream of an account: the events it may see,
// in order, until it is reset. The handler writes them out and closes it.
type Stream struct {
	hub    *Hub
	userID uuid.UUID
	events chan domain.Event
	done   chan struct{}

	mu sync.Mutex
	// seen is what the stream sees, nil until the opening settles it; the
	// events offered meanwhile wait in pending.
	seen    *seen
	pending []routed
	reason  domain.ResetReason
}

// Events are the events for the client, BufferSize at most.
func (s *Stream) Events() <-chan domain.Event { return s.events }

// Done is closed once the stream is reset; Reason tells why.
func (s *Stream) Done() <-chan struct{} { return s.done }

// Reason is why the stream was reset, "" while it is not.
func (s *Stream) Reason() domain.ResetReason {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

// Close takes the stream off the hub.
func (s *Stream) Close() { s.hub.close(s) }

// offer gives the stream an event, which waits while the stream does not
// know what it sees; a stream whose client is behind, or one waiting with
// PendingSize events, is reset.
func (s *Stream) offer(r routed) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.reason != "":
	case s.seen == nil && len(s.pending) == PendingSize:
		s.reset(domain.ResetOverflow)
	case s.seen == nil:
		s.pending = append(s.pending, r)
	default:
		s.route(r)
	}
}

// settle gives the stream what it sees, and routes the events that waited
// for it: an access or notebooks_deleted event among them that concerns
// it resets it (M5 design 4.10).
func (s *Stream) settle(v seen) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = &v
	for _, r := range s.pending {
		if s.reason != "" {
			break
		}
		s.route(r)
	}
	s.pending = nil
}

// route resets the stream for an access or notebooks_deleted event that
// concerns it, and passes it any other event it sees. Under s.mu, once
// settled.
func (s *Stream) route(r routed) {
	switch {
	case r.access != nil:
		if slices.Contains(r.access.UserIDs, s.userID) || r.access.Everyone() && s.seen.workspaces[r.WorkspaceID] {
			s.reset(domain.ResetAccess)
		}
	case r.deleted != nil:
		if s.seen.anyOf(r.WorkspaceID, r.deleted.NotebookIDs) {
			s.reset(domain.ResetNotebooksDeleted)
		}
	case s.seen.has(r.Event):
		select {
		case s.events <- r.Event:
		default:
			s.reset(domain.ResetOverflow)
		}
	}
}

// resetFor resets the stream for reason, unless it is reset already.
func (s *Stream) resetFor(reason domain.ResetReason) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reset(reason)
}

// reset is resetFor under s.mu.
func (s *Stream) reset(reason domain.ResetReason) {
	if s.reason == "" {
		s.reason = reason
		close(s.done)
	}
}

// seen is what a stream sees: the workspaces its account is a member of,
// and the notebooks it may read in them.
type seen struct {
	workspaces map[uuid.UUID]bool
	notebooks  map[uuid.UUID]bool
}

// has reports whether the stream sees e: its notebook, or its workspace
// when it is of no notebook.
func (v seen) has(e domain.Event) bool {
	if e.NotebookID != uuid.Nil() {
		return v.notebooks[e.NotebookID]
	}
	return v.workspaces[e.WorkspaceID]
}

// anyOf reports whether the stream sees one of the notebooks, or, when
// they are not listed (nil), the workspace.
func (v seen) anyOf(workspaceID uuid.UUID, notebooks []uuid.UUID) bool {
	if notebooks == nil {
		return v.workspaces[workspaceID]
	}
	return slices.ContainsFunc(notebooks, func(id uuid.UUID) bool { return v.notebooks[id] })
}
