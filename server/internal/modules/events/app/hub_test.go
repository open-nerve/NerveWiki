package app_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The accounts, workspaces and notebooks of the tests: alice is a member
// of acme and reads its notebook seen, not its notebook unseen; she is no
// member of other, whose notebook is elsewhere.
func alice() uuid.UUID    { return uuid.MustParse("01990000-0000-7000-8000-00000000000a") }
func bob() uuid.UUID      { return uuid.MustParse("01990000-0000-7000-8000-00000000000b") }
func acme() uuid.UUID     { return uuid.MustParse("01990000-0000-7000-8000-0000000000a1") }
func other() uuid.UUID    { return uuid.MustParse("01990000-0000-7000-8000-0000000000a2") }
func seen() uuid.UUID     { return uuid.MustParse("01990000-0000-7000-8000-0000000000b1") }
func unseen() uuid.UUID   { return uuid.MustParse("01990000-0000-7000-8000-0000000000b2") }
func elsewise() uuid.UUID { return uuid.MustParse("01990000-0000-7000-8000-0000000000b3") }

// fakeVisibility sees what the tests say: alice is a member of acme and
// reads seen there. during runs while it reads, as events that come in
// between; err fails the read.
type fakeVisibility struct {
	during func()
	err    error
}

func (v fakeVisibility) WorkspacesOf(_ context.Context, userID uuid.UUID) ([]app.Membership, error) {
	if v.during != nil {
		v.during()
	}
	if v.err != nil {
		return nil, v.err
	}
	if userID != alice() {
		return nil, nil
	}
	return []app.Membership{{WorkspaceID: acme(), Role: shared.WorkspaceMember}}, nil
}

func (v fakeVisibility) NotebooksIn(_ context.Context, workspaceID, userID uuid.UUID, role shared.WorkspaceRole) ([]uuid.UUID, error) {
	if workspaceID != acme() || userID != alice() || role != shared.WorkspaceMember {
		return nil, errors.New("asked about the wrong workspace")
	}
	return []uuid.UUID{seen()}, nil
}

func listeningHub() *app.Hub {
	h := app.NewHub(slog.New(slog.DiscardHandler))
	h.Listening(true)
	return h
}

// openAs opens the stream of userID on h, closed when the test ends.
func openAs(t *testing.T, h *app.Hub, v fakeVisibility, userID uuid.UUID) *app.Stream {
	t.Helper()
	s, err := app.NewOpenStream(h, v).Execute(shared.WithActor(context.Background(), shared.Actor{UserID: userID, SessionID: uuid.NewV7()}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func ev(t *testing.T, typ domain.Type, workspace, notebook uuid.UUID, data any) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(typ, workspace, notebook, data)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// received drains the events s holds now.
func received(s *app.Stream) []domain.Event {
	var out []domain.Event
	for {
		select {
		case e := <-s.Events():
			out = append(out, e)
		default:
			return out
		}
	}
}

// resetFor reports why s was reset, "" when it was not.
func resetFor(s *app.Stream) domain.ResetReason {
	select {
	case <-s.Done():
		return s.Reason()
	default:
		return ""
	}
}

// A stream gets the events of the notebooks it reads and of the
// workspaces it is a member of, of any type, in order, and no other.
func TestAStreamGetsWhatItSees(t *testing.T) {
	h := listeningHub()
	s := openAs(t, h, fakeVisibility{}, alice())
	pages := domain.Pages{Pages: []domain.PageRevision{}}
	want := []domain.Event{
		ev(t, domain.TypePages, acme(), seen(), pages),
		ev(t, "links", acme(), uuid.Nil(), map[string]int{"n": 1}),
		ev(t, "links", acme(), seen(), map[string]int{"n": 2}),
	}
	for _, e := range []domain.Event{
		want[0],
		ev(t, domain.TypePages, acme(), unseen(), pages),
		ev(t, domain.TypePages, other(), elsewise(), pages),
		want[1],
		ev(t, "links", other(), uuid.Nil(), map[string]int{"n": 3}),
		want[2],
	} {
		h.Dispatch(e)
	}

	got := received(s)
	if len(got) != len(want) || resetFor(s) != "" {
		t.Fatalf("received %d events, reset %q; want %d and no reset", len(got), resetFor(s), len(want))
	}
	for i := range want {
		if got[i].Type != want[i].Type || got[i].NotebookID != want[i].NotebookID || string(got[i].Data) != string(want[i].Data) {
			t.Errorf("event %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// An access event resets the streams of the accounts it lists, and every
// stream of its workspace when it may reach everyone; a notebooks_deleted
// event resets the streams that read one of its notebooks, or that are of
// its workspace when it does not list them (M5 design 4.10).
func TestAccessAndDeletionResetTheStreamsTheyConcern(t *testing.T) {
	for _, tt := range []struct {
		name string
		e    func(t *testing.T) domain.Event
		want domain.ResetReason
	}{
		{"an access change of hers elsewhere", func(t *testing.T) domain.Event {
			return ev(t, domain.TypeAccess, other(), uuid.Nil(), domain.Access{UserIDs: []uuid.UUID{alice()}})
		}, domain.ResetAccess},
		{"an access change reaching her workspace", func(t *testing.T) domain.Event {
			return ev(t, domain.TypeAccess, acme(), uuid.Nil(), domain.Access{UserIDs: []uuid.UUID{}, Reached: true})
		}, domain.ResetAccess},
		{"an access change of her workspace, its accounts shed", func(t *testing.T) domain.Event {
			return ev(t, domain.TypeAccess, acme(), uuid.Nil(), domain.Access{})
		}, domain.ResetAccess},
		{"an access change of bob's in her workspace", func(t *testing.T) domain.Event {
			return ev(t, domain.TypeAccess, acme(), uuid.Nil(), domain.Access{UserIDs: []uuid.UUID{bob()}})
		}, ""},
		{"an access change reaching another workspace", func(t *testing.T) domain.Event {
			return ev(t, domain.TypeAccess, other(), uuid.Nil(), domain.Access{UserIDs: []uuid.UUID{}, Reached: true})
		}, ""},
		{"the deletion of a notebook she reads", func(t *testing.T) domain.Event {
			return ev(t, domain.TypeNotebooksDeleted, acme(), uuid.Nil(), domain.NotebooksDeleted{NotebookIDs: []uuid.UUID{unseen(), seen()}})
		}, domain.ResetNotebooksDeleted},
		{"the deletion of a notebook she does not read", func(t *testing.T) domain.Event {
			return ev(t, domain.TypeNotebooksDeleted, acme(), uuid.Nil(), domain.NotebooksDeleted{NotebookIDs: []uuid.UUID{unseen()}})
		}, ""},
		{"the deletion of her workspace's notebooks, unlisted", func(t *testing.T) domain.Event {
			return ev(t, domain.TypeNotebooksDeleted, acme(), uuid.Nil(), domain.NotebooksDeleted{})
		}, domain.ResetNotebooksDeleted},
		{"the deletion of another workspace's notebooks, unlisted", func(t *testing.T) domain.Event {
			return ev(t, domain.TypeNotebooksDeleted, other(), uuid.Nil(), domain.NotebooksDeleted{})
		}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := listeningHub()
			s := openAs(t, h, fakeVisibility{}, alice())

			h.Dispatch(tt.e(t))
			h.Dispatch(ev(t, domain.TypePages, acme(), seen(), domain.Pages{}))

			got := received(s)
			if resetFor(s) != tt.want || (tt.want == "") != (len(got) == 1) {
				t.Errorf("reset %q, then %d events; want %q, and the next event only without a reset", resetFor(s), len(got), tt.want)
			}
		})
	}
}

// A stream whose client is BufferSize events behind is reset; the events
// it held stay readable.
func TestAStreamBehindIsReset(t *testing.T) {
	h := listeningHub()
	s := openAs(t, h, fakeVisibility{}, alice())
	for range app.BufferSize + 1 {
		h.Dispatch(ev(t, domain.TypePages, acme(), seen(), domain.Pages{}))
	}

	if got := len(received(s)); resetFor(s) != domain.ResetOverflow || got != app.BufferSize {
		t.Errorf("reset %q with %d events held; want overflow with %d", resetFor(s), got, app.BufferSize)
	}
}

// ResetAll resets every stream; a stream reset keeps its first reason.
func TestResetAllResetsEveryStream(t *testing.T) {
	h := listeningHub()
	first, second := openAs(t, h, fakeVisibility{}, alice()), openAs(t, h, fakeVisibility{}, bob())
	h.Dispatch(ev(t, domain.TypeAccess, acme(), uuid.Nil(), domain.Access{UserIDs: []uuid.UUID{alice()}}))

	h.ResetAll(domain.ResetReconnected)

	if resetFor(first) != domain.ResetAccess || resetFor(second) != domain.ResetReconnected {
		t.Errorf("reset %q and %q; want access, kept, and reconnected", resetFor(first), resetFor(second))
	}
}

// No stream opens while the hub does not listen: it could miss what is
// sent before (M5 design 4.10). A closed stream leaves the hub.
func TestAStreamOpensOnlyWhileTheHubListens(t *testing.T) {
	h := app.NewHub(slog.New(slog.DiscardHandler))
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: alice(), SessionID: uuid.NewV7()})
	if s, err := app.NewOpenStream(h, fakeVisibility{}).Execute(ctx); !errors.Is(err, domain.ErrNotReady) {
		t.Fatalf("Execute() before the hub listens = %v, %v; want not_ready", s, err)
	}
	h.Listening(true)
	s, err := app.NewOpenStream(h, fakeVisibility{}).Execute(ctx)
	if err != nil || h.Streams() != 1 {
		t.Fatalf("Execute() once it listens = %v, %d streams; want one", err, h.Streams())
	}
	h.Listening(false)
	if _, err := app.NewOpenStream(h, fakeVisibility{}).Execute(ctx); !errors.Is(err, domain.ErrNotReady) {
		t.Errorf("Execute() once it stops = %v, want not_ready", err)
	}

	s.Close()
	h.Dispatch(ev(t, domain.TypePages, acme(), seen(), domain.Pages{}))
	if got := received(s); h.Streams() != 0 || len(got) != 0 {
		t.Errorf("%d streams, the closed one got %d events; want none", h.Streams(), len(got))
	}
}

// A stream is registered before what it sees is read: the events that
// come meanwhile wait, and an access or deletion among them that concerns
// it resets it once it knows what it sees; the others it sees reach it
// (M5 design 4.10). Too many waiting reset it too, and it gets none of
// them: the client refreshes all.
func TestWhatComesWhileAStreamOpens(t *testing.T) {
	for _, tt := range []struct {
		name   string
		during func(t *testing.T, h *app.Hub)
		want   domain.ResetReason
		events int
	}{
		{"an access change of hers", func(t *testing.T, h *app.Hub) {
			h.Dispatch(ev(t, domain.TypeAccess, other(), uuid.Nil(), domain.Access{UserIDs: []uuid.UUID{alice()}}))
		}, domain.ResetAccess, 0},
		{"an access change reaching her workspace", func(t *testing.T, h *app.Hub) {
			h.Dispatch(ev(t, domain.TypeAccess, acme(), uuid.Nil(), domain.Access{UserIDs: []uuid.UUID{}, Reached: true}))
		}, domain.ResetAccess, 0},
		{"the deletion of a notebook she reads", func(t *testing.T, h *app.Hub) {
			h.Dispatch(ev(t, domain.TypeNotebooksDeleted, acme(), uuid.Nil(), domain.NotebooksDeleted{NotebookIDs: []uuid.UUID{seen()}}))
		}, domain.ResetNotebooksDeleted, 0},
		{"bob's access change and pages", func(t *testing.T, h *app.Hub) {
			h.Dispatch(ev(t, domain.TypeAccess, acme(), uuid.Nil(), domain.Access{UserIDs: []uuid.UUID{bob()}}))
			h.Dispatch(ev(t, domain.TypePages, acme(), seen(), domain.Pages{}))
			h.Dispatch(ev(t, domain.TypePages, acme(), unseen(), domain.Pages{}))
		}, "", 1},
		{"more than a buffer", func(t *testing.T, h *app.Hub) {
			for range app.BufferSize + 1 {
				h.Dispatch(ev(t, domain.TypePages, acme(), seen(), domain.Pages{}))
			}
		}, domain.ResetOverflow, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := listeningHub()
			s := openAs(t, h, fakeVisibility{during: func() { tt.during(t, h) }}, alice())

			if got := len(received(s)); resetFor(s) != tt.want || got != tt.events {
				t.Errorf("reset %q with %d events; want %q with %d", resetFor(s), got, tt.want, tt.events)
			}
		})
	}
}

// A stream whose read fails is closed; one without an actor does not open.
func TestAStreamThatCannotReadWhatItSeesCloses(t *testing.T) {
	h := listeningHub()
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: alice(), SessionID: uuid.NewV7()})
	down := errors.New("database is down")

	if _, err := app.NewOpenStream(h, fakeVisibility{err: down}).Execute(ctx); !errors.Is(err, down) || h.Streams() != 0 {
		t.Errorf("Execute() = %v with %d streams; want the read's error and none", err, h.Streams())
	}
	if _, err := app.NewOpenStream(h, fakeVisibility{}).Execute(context.Background()); err == nil || h.Streams() != 0 {
		t.Errorf("Execute() without an actor = %v with %d streams; want an error and none", err, h.Streams())
	}
}

// A payload that does not decode, and an access event whose data does not,
// are dropped: no stream gets or is reset by them.
func TestTheHubDropsWhatItCannotRead(t *testing.T) {
	h := listeningHub()
	s := openAs(t, h, fakeVisibility{}, alice())

	h.Notified("not json")
	h.Dispatch(domain.Event{Type: domain.TypeAccess, WorkspaceID: acme(), Data: []byte(`{"user_ids":"x"}`)})
	payload, _ := domain.Encode(ev(t, domain.TypePages, acme(), seen(), domain.Pages{}))
	h.Notified(payload)

	if got := received(s); len(got) != 1 || resetFor(s) != "" {
		t.Errorf("%d events, reset %q; want the one that decodes, and no reset", len(got), resetFor(s))
	}
}
