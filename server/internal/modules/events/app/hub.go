package app

import (
	"encoding/json"
	"log/slog"
	"sync"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
)

// BufferSize is how many events a stream holds for its client. A stream
// whose client falls further behind is reset: it neither holds the others
// up nor loses events unseen (M5 design 4.10).
const BufferSize = 64

// PendingSize is how many events a stream holds while it opens: every
// event of the server, its own or not, until it knows what it sees. More,
// and it is reset.
const PendingSize = 1024

// Hub holds this process's streams and hands each event to those that may
// see it (M5 design 4.10). The listener feeds it: Notified with each
// payload, Listening with its state, ResetAll after it reconnects. It
// never blocks on a stream.
type Hub struct {
	logger    *slog.Logger
	mu        sync.Mutex
	streams   map[*Stream]struct{}
	listening bool
}

// NewHub returns a hub with no stream, not listening.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{logger: logger, streams: make(map[*Stream]struct{})}
}

// Listening records whether the listener listens: a stream opens only
// while it does, so that it misses nothing sent after it opened.
func (h *Hub) Listening(on bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.listening = on
}

// Notified hands the event of payload to the streams; a payload that does
// not decode is logged and dropped.
func (h *Hub) Notified(payload string) {
	e, err := domain.Decode(payload)
	if err != nil {
		h.logger.Warn("event dropped", slog.Any("error", err))
		return
	}
	h.Dispatch(e)
}

// Dispatch hands e to every stream that may have it: an event of a
// notebook to the streams that see the notebook, one of a workspace to
// those that see the workspace. An access event resets the streams of
// its accounts, and of the whole workspace when it may reach everyone; a
// notebooks_deleted event resets the streams that see one of the
// notebooks, or the workspace when the event does not list them. An
// access or notebooks_deleted event whose data does not decode is logged
// and dropped.
func (h *Hub) Dispatch(e domain.Event) {
	r := routed{Event: e}
	var err error
	switch e.Type {
	case domain.TypeAccess:
		r.access = &domain.Access{}
		err = json.Unmarshal(e.Data, r.access)
	case domain.TypeNotebooksDeleted:
		r.deleted = &domain.NotebooksDeleted{}
		err = json.Unmarshal(e.Data, r.deleted)
	}
	if err != nil {
		h.logger.Warn("event dropped", slog.String("type", string(e.Type)), slog.Any("error", err))
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.streams {
		s.offer(r)
	}
}

// ResetAll resets every stream for reason: after the listener reconnected,
// what was sent meanwhile is lost.
func (h *Hub) ResetAll(reason domain.ResetReason) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.streams {
		s.resetFor(reason)
	}
}

// open registers a stream of userID, which holds the events it is offered
// until settle gives it what it sees; ErrNotReady when the hub is not
// listening.
func (h *Hub) open(userID uuid.UUID) (*Stream, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.listening {
		return nil, domain.ErrNotReady
	}
	s := &Stream{hub: h, userID: userID, events: make(chan domain.Event, BufferSize), done: make(chan struct{})}
	h.streams[s] = struct{}{}
	return s, nil
}

// close forgets s.
func (h *Hub) close(s *Stream) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.streams, s)
}

// Streams is how many streams are open.
func (h *Hub) Streams() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.streams)
}

// routed is an event with the data the hub reads decoded.
type routed struct {
	domain.Event
	access  *domain.Access
	deleted *domain.NotebooksDeleted
}
