package domain_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
)

const (
	workspaceText = "01990000-0000-7000-8000-000000000001"
	notebookText  = "01990000-0000-7000-8000-000000000002"
	pageText      = "01990000-0000-7000-8000-000000000003"
)

func event(t *testing.T, typ domain.Type, notebook uuid.UUID, data any) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(typ, uuid.MustParse(workspaceText), notebook, data)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// An event goes through a payload as it was: the notebook's id only when
// it is of one.
func TestAnEventGoesThroughItsPayload(t *testing.T) {
	for _, e := range []domain.Event{
		event(t, domain.TypePages, uuid.MustParse(notebookText), domain.Pages{Tree: true, Pages: []domain.PageRevision{}}),
		event(t, domain.TypeAccess, uuid.Nil(), domain.Access{UserIDs: []uuid.UUID{uuid.MustParse(pageText)}}),
		event(t, "links", uuid.Nil(), map[string]string{"x": "y"}),
	} {
		payload, err := domain.Encode(e)
		if err != nil {
			t.Fatal(err)
		}
		got, err := domain.Decode(payload)
		if err != nil || got.Type != e.Type || got.WorkspaceID != e.WorkspaceID || got.NotebookID != e.NotebookID ||
			string(got.Data) != string(e.Data) {
			t.Errorf("Decode(Encode(%+v)) = %+v, %v", e, got, err)
		}
		if strings.Contains(payload, "notebook_id") != (e.NotebookID != uuid.Nil()) {
			t.Errorf("payload %s: the notebook's id there when the event is of one", payload)
		}
	}
}

// A payload of MaxPayload bytes encodes, one longer does not; a payload
// that is no event does not decode.
func TestAPayloadHasItsLimits(t *testing.T) {
	short := event(t, "x", uuid.Nil(), map[string]string{"s": ""})
	base, _ := domain.Encode(short)
	fill := domain.MaxPayload - len(base)
	if p, err := domain.Encode(event(t, "x", uuid.Nil(), map[string]string{"s": strings.Repeat("a", fill)})); err != nil || len(p) != domain.MaxPayload {
		t.Errorf("Encode() of %d bytes = %d, %v; want it", domain.MaxPayload, len(p), err)
	}
	if _, err := domain.Encode(event(t, "x", uuid.Nil(), map[string]string{"s": strings.Repeat("a", fill+1)})); !errors.Is(err, domain.ErrTooLong) {
		t.Errorf("Encode() of %d bytes = %v, want ErrTooLong", domain.MaxPayload+1, err)
	}
	for _, p := range []string{"", "{", `{"type":"pages"}`, `{"type":"pages","workspace_id":"` + workspaceText + `","data":[]}`} {
		if e, err := domain.Decode(p); err == nil {
			t.Errorf("Decode(%q) = %+v, want an error", p, e)
		}
	}
}

// noTypes are types no event can have: empty, with a colon or a line break,
// or a frame of the stream itself.
func noTypes() []domain.Type {
	return []domain.Type{"", "a:b", "a\nb", "a\rb", "hello", "reset"}
}

// An event of a type no event can have has no payload: its publisher's
// write fails, not the frame a stream would write.
func TestAnEventOfNoTypeHasNoPayload(t *testing.T) {
	for _, typ := range noTypes() {
		e := domain.Event{Type: typ, WorkspaceID: uuid.MustParse(workspaceText), Data: json.RawMessage(`{}`)}
		if p, err := domain.Encode(e); !errors.Is(err, domain.ErrNoType) {
			t.Errorf("Encode() of type %q = %q, %v; want ErrNoType", typ, p, err)
		}
	}
	if err := domain.CheckType("links"); err != nil {
		t.Errorf("CheckType(links) = %v, want a later M's type", err)
	}
}

// The frames are server-sent events: hello with the heartbeat in seconds,
// an event with its data and its workspace's and notebook's ids, a reset
// with its reason, the heartbeat as a comment.
func TestTheFrames(t *testing.T) {
	if got := string(domain.HelloFrame(20 * time.Second)); got != "event: hello\ndata: {\"heartbeat_seconds\":20}\n\n" {
		t.Errorf("HelloFrame() = %q", got)
	}
	f, err := domain.EventFrame(event(t, domain.TypeLock, uuid.MustParse(notebookText),
		domain.Lock{PageID: uuid.MustParse(pageText), SessionID: uuid.MustParse(pageText)}))
	head, data, ok := strings.Cut(strings.TrimSuffix(string(f), "\n\n"), "\ndata: ")
	var fields map[string]string
	if err != nil || !ok || head != "event: lock" || json.Unmarshal([]byte(data), &fields) != nil ||
		fields["workspace_id"] != workspaceText || fields["notebook_id"] != notebookText || fields["page_id"] != pageText {
		t.Errorf("EventFrame() = %q, %v", f, err)
	}
	f, err = domain.EventFrame(event(t, domain.TypeAccess, uuid.Nil(), domain.Access{Reached: true}))
	if err != nil || strings.Contains(string(f), "notebook_id") {
		t.Errorf("EventFrame() of a workspace's event = %q, %v; want no notebook", f, err)
	}
	if got := string(domain.ResetFrame(domain.ResetExpired)); got != "event: reset\ndata: {\"reason\":\"expired\"}\n\n" {
		t.Errorf("ResetFrame() = %q", got)
	}
	if got := string(domain.HeartbeatFrame()); got != ": heartbeat\n\n" {
		t.Errorf("HeartbeatFrame() = %q", got)
	}
	for _, typ := range noTypes() {
		if f, err := domain.EventFrame(domain.Event{Type: typ, Data: json.RawMessage(`{}`)}); !errors.Is(err, domain.ErrNoType) {
			t.Errorf("EventFrame() of type %q = %q, %v; want ErrNoType", typ, f, err)
		}
	}
	if f, err := domain.EventFrame(domain.Event{Type: "x", Data: json.RawMessage(`[]`)}); err == nil {
		t.Errorf("EventFrame() of data that is no object = %q, want an error", f)
	}
}

// An access change may reach everyone when it says so, or when it lists
// no one, its list shed; an empty list reaches no one but the reached.
func TestAnAccessChangeReachesEveryoneOrItsAccounts(t *testing.T) {
	for _, tt := range []struct {
		a    domain.Access
		want bool
	}{
		{domain.Access{Reached: true, UserIDs: []uuid.UUID{}}, true},
		{domain.Access{}, true},
		{domain.Access{UserIDs: []uuid.UUID{}}, false},
		{domain.Access{UserIDs: []uuid.UUID{uuid.MustParse(pageText)}}, false},
	} {
		if got := tt.a.Everyone(); got != tt.want {
			t.Errorf("%+v.Everyone() = %v, want %v", tt.a, got, tt.want)
		}
	}
}
