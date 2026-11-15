package app

import (
	"encoding/json"
	"log/slog"
	"slices"
	"sync"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// BufferSize is how many events a stream holds for its client. A stream
// whose client falls further behind is reset: it neither holds the others
// up nor loses events unseen (M5 design 4.10).
const BufferSize = 64

// PendingSize is how many events a stream holds while it opens: every
// event of the server, its own or not, until it knows what it sees. More,
// and it is reset.
const PendingSize = 1024

// MaxCredentialStreams is how many streams a credential holds open: the
// opening of its fifth resets its oldest, superseded, its account's count
// unchanged (M12 design 4.6.5). A browser opens one; a laptop that slept
// leaves a half-open one, whose place its new tab takes.
const MaxCredentialStreams = 4

// MaxAccountStreams is how many streams an account holds open over all
// its credentials: an opening that would make one more is refused,
// domain.ErrTooManyStreams. A credential at its own bound takes the place
// of its oldest and is never refused.
const MaxAccountStreams = 64

// credential is the one an actor acts with: a sign-in's session, a
// personal access token or a background job, by kind and id.
type credential struct {
	kind string
	id   uuid.UUID
}

// credentialOf is a's credential (shared.Actor holds exactly one).
func credentialOf(a shared.Actor) credential {
	switch {
	case a.APITokenID != (uuid.UUID{}):
		return credential{kind: "pat", id: a.APITokenID}
	case a.JobID != (uuid.UUID{}):
		return credential{kind: "job", id: a.JobID}
	}
	return credential{kind: "session", id: a.SessionID}
}

// Hub holds this process's streams and hands each event to those that may
// see it (M5 design 4.10). The listener feeds it: Notified with each
// payload, Listening with its state, ResetAll after it reconnects. It
// never blocks on a stream. It counts each credential's streams and each
// account's (M12 design 4.6.5): a stream counts from its opening until it
// closes, or until a newer one of its credential supersedes it. One reset
// for another reason counts until its handler closes it: at once when its
// client reads, within a heartbeat when it does not. A credential or an
// account that holds none is forgotten, so the counts do not grow with
// the process's life.
type Hub struct {
	logger    *slog.Logger
	mu        sync.Mutex
	streams   map[*Stream]struct{}
	held      map[credential][]*Stream // each credential's streams counted, oldest first
	accounts  map[uuid.UUID]int        // each account's streams counted
	listening bool
}

// NewHub returns a hub with no stream, not listening.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{logger: logger, streams: make(map[*Stream]struct{}), held: make(map[credential][]*Stream),
		accounts: make(map[uuid.UUID]int)}
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

// open registers a stream of the actor, which holds the events it is
// offered until settle gives it what it sees (M12 design 4.6.5): a
// credential at MaxCredentialStreams has its oldest reset, superseded, no
// longer counted, and is never refused; else an account at
// MaxAccountStreams is refused, domain.ErrTooManyStreams. ErrNotReady when
// the hub is not listening.
func (h *Hub) open(a shared.Actor) (*Stream, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.listening {
		return nil, domain.ErrNotReady
	}
	c := credentialOf(a)
	switch held := h.held[c]; {
	case len(held) >= MaxCredentialStreams:
		// The oldest ends; its handler writes the events it holds, then the
		// reset, and closes it, which finds it counted no longer.
		oldest := held[0]
		h.held[c] = slices.Delete(held, 0, 1)
		h.accounts[a.UserID]--
		oldest.resetFor(domain.ResetSuperseded)
		h.logger.Debug("event stream superseded", slog.String("user_id", a.UserID.String()), slog.String("credential", c.kind),
			slog.Int("streams", MaxCredentialStreams))
	case h.accounts[a.UserID] >= MaxAccountStreams:
		h.logger.Debug("event streams of the account at their bound", slog.String("user_id", a.UserID.String()),
			slog.Int("streams", MaxAccountStreams))
		return nil, domain.ErrTooManyStreams
	}
	s := &Stream{hub: h, userID: a.UserID, credential: c, events: make(chan domain.Event, BufferSize), done: make(chan struct{})}
	h.streams[s] = struct{}{}
	h.held[c] = append(h.held[c], s)
	h.accounts[a.UserID]++
	return s, nil
}

// close forgets s, and its count when it is still counted: a superseded
// stream no longer is.
func (h *Hub) close(s *Stream) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.streams, s)
	held := h.held[s.credential]
	i := slices.Index(held, s)
	if i < 0 {
		return
	}
	if held = slices.Delete(held, i, i+1); len(held) == 0 {
		delete(h.held, s.credential)
	} else {
		h.held[s.credential] = held
	}
	if h.accounts[s.userID]--; h.accounts[s.userID] == 0 {
		delete(h.accounts, s.userID)
	}
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
