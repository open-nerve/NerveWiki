package httpadapter_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/events/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

const (
	workspaceText = "01990000-0000-7000-8000-0000000000a1"
	notebookText  = "01990000-0000-7000-8000-0000000000b1"
	unseenText    = "01990000-0000-7000-8000-0000000000b2"
	heartbeat     = 20 * time.Second
)

func now() time.Time { return time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC) }

// everyone sees the workspace and its notebook, not the other notebook.
type everyone struct{}

func (everyone) WorkspacesOf(context.Context, uuid.UUID) ([]app.Membership, error) {
	return []app.Membership{{WorkspaceID: uuid.MustParse(workspaceText), Role: shared.WorkspaceMember}}, nil
}

func (everyone) NotebooksIn(context.Context, uuid.UUID, uuid.UUID, shared.WorkspaceRole) ([]uuid.UUID, error) {
	return []uuid.UUID{uuid.MustParse(notebookText)}, nil
}

// tokens accepts "tok", and "short", which expires a minute from now;
// once revoked, it refuses both with a 401, once down with a fault.
type tokens struct {
	revoked, down atomic.Bool
}

func (a *tokens) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	switch {
	case a.down.Load():
		return nil, "", errors.New("database is down")
	case a.revoked.Load() || (token != "tok" && token != "short"):
		return nil, "", shared.Unauthenticated()
	}
	ctx = shared.WithActor(ctx, shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()})
	if token == "short" {
		ctx = httpserver.WithCredentialExpiry(ctx, now().Add(time.Minute))
	}
	return ctx, "session:" + token, nil
}

// timer is one the handler made: its duration and its channel.
type timer struct {
	d time.Duration
	c chan time.Time
}

// stream is the server of the tests, its hub, its tokens and its timers.
type stream struct {
	url    string
	hub    *app.Hub
	tokens *tokens
	timers chan timer
}

func newStream(t *testing.T) *stream {
	t.Helper()
	s := &stream{hub: app.NewHub(slog.New(slog.DiscardHandler)), tokens: &tokens{}, timers: make(chan timer, 16)}
	s.hub.Listening(true)
	logger := slog.New(slog.DiscardHandler)
	h := httpadapter.New(app.NewOpenStream(s.hub, everyone{}), httpadapter.Config{
		Errors: httpserver.NewAPIErrors(logger), Logger: logger, Heartbeat: heartbeat, Now: now,
		After: func(d time.Duration) <-chan time.Time {
			c := make(chan time.Time, 1)
			s.timers <- timer{d: d, c: c}
			return c
		},
	})
	router := httpserver.NewRouter(logger)
	router.Handle(httpadapter.Route, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: s.tokens}).LongLived(h))
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	s.url = srv.URL
	return s
}

// timer returns the next timer the handler made.
func (s *stream) timer(t *testing.T) timer {
	t.Helper()
	select {
	case tm := <-s.timers:
		return tm
	case <-time.After(5 * time.Second):
		t.Fatal("the handler made no timer")
		return timer{}
	}
}

// open opens a stream with token, checks the answer against the contract,
// and returns it with a reader of its frames. The answer's head must come
// within 5 s: a stream flushes it with its first frame.
func (s *stream) open(t *testing.T, token string) (*http.Response, *bufio.Reader) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/api/v0/events", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	late := time.AfterFunc(5*time.Second, cancel)
	res, err := http.DefaultClient.Do(req)
	late.Stop()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	apitest.Load(t).CheckResponse(t, req, res)
	return res, bufio.NewReader(res.Body)
}

// frame reads the next frame: its event and data, or a comment's text.
func frame(t *testing.T, r *bufio.Reader) (event, data string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				event = "EOF"
				return
			}
			line = strings.TrimSuffix(line, "\n")
			switch {
			case line == "":
				return
			case strings.HasPrefix(line, ": "):
				event = "comment " + strings.TrimPrefix(line, ": ")
			case strings.HasPrefix(line, "event: "):
				event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	select {
	case <-done:
		return event, data
	case <-time.After(5 * time.Second):
		t.Fatal("no frame came")
		return "", ""
	}
}

// expect reads the next frame and fails t unless it is event, its data
// valid against schema when schema is not "".
func expect(t *testing.T, r *bufio.Reader, event, schema string) string {
	t.Helper()
	got, data := frame(t, r)
	if got != event {
		t.Fatalf("frame %q %s, want %q", got, data, event)
	}
	if schema != "" {
		apitest.Load(t).CheckSchema(t, schema, []byte(data))
	}
	return data
}

func dispatch(t *testing.T, h *app.Hub, typ domain.Type, notebook string, data any) {
	t.Helper()
	e, err := domain.NewEvent(typ, uuid.MustParse(workspaceText), uuid.MustParse(notebook), data)
	if err != nil {
		t.Fatal(err)
	}
	h.Dispatch(e)
}

// A stream answers text/event-stream, unbuffered by proxies; it starts
// with hello, the heartbeat in seconds, then writes the events of what
// its caller sees, each valid against the contract, and no other.
func TestAStreamWritesHelloThenItsEvents(t *testing.T) {
	s := newStream(t)
	res, r := s.open(t, "tok")
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/event-stream" || res.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("GET = %d %v", res.StatusCode, res.Header)
	}
	if data := expect(t, r, "hello", "EventHello"); data != `{"heartbeat_seconds":20}` {
		t.Errorf("hello = %s", data)
	}

	page := uuid.NewV7()
	dispatch(t, s.hub, domain.TypePages, unseenText, domain.Pages{Pages: []domain.PageRevision{}})
	dispatch(t, s.hub, domain.TypePages, notebookText, domain.Pages{Tree: true, Pages: []domain.PageRevision{{ID: page, Revision: 2}}})
	dispatch(t, s.hub, domain.TypePages, notebookText, domain.Pages{})
	dispatch(t, s.hub, domain.TypeLock, notebookText, domain.Lock{PageID: page, SessionID: uuid.NewV7()})

	var pages struct {
		NotebookID string `json:"notebook_id"`
		Pages      []struct {
			ID string `json:"id"`
		} `json:"pages"`
	}
	if err := json.Unmarshal([]byte(expect(t, r, "pages", "EventPages")), &pages); err != nil || pages.NotebookID != notebookText ||
		len(pages.Pages) != 1 || pages.Pages[0].ID != page.String() {
		t.Errorf("pages = %+v, %v; want the seen notebook's write", pages, err)
	}
	expect(t, r, "pages", "EventPages")
	expect(t, r, "lock", "EventLock")
}

// Each heartbeat authenticates the credential again: a comment line while
// it is valid, the next timer the heartbeat again; reset unauthenticated
// once it is revoked, and the stream ends.
func TestAHeartbeatAuthenticatesAgain(t *testing.T) {
	s := newStream(t)
	_, r := s.open(t, "tok")
	expect(t, r, "hello", "")

	beat := s.timer(t)
	beat.c <- now()
	expect(t, r, "comment heartbeat", "")
	next := s.timer(t)
	if beat.d != heartbeat || next.d != heartbeat {
		t.Errorf("timers of %v, %v; want the heartbeat's", beat.d, next.d)
	}
	s.tokens.revoked.Store(true)
	next.c <- now()

	if data := expect(t, r, "reset", "EventReset"); data != `{"reason":"unauthenticated"}` {
		t.Errorf("reset = %s", data)
	}
	expect(t, r, "EOF", "")
}

// A fault at a heartbeat ends the stream without a frame: the client
// reconnects, and the credential is no reason to refresh.
func TestAFaultAtAHeartbeatEndsTheStream(t *testing.T) {
	s := newStream(t)
	_, r := s.open(t, "tok")
	expect(t, r, "hello", "")
	s.tokens.down.Store(true)
	s.timer(t).c <- now()

	expect(t, r, "EOF", "")
}

// A stream ends at its credential's expiry: reset expired. Its first
// timer is the expiry's, from the clock's now.
func TestAStreamEndsAtItsCredentialsExpiry(t *testing.T) {
	s := newStream(t)
	_, r := s.open(t, "short")
	expect(t, r, "hello", "")

	expiry := s.timer(t)
	if beat := s.timer(t); expiry.d != time.Minute || beat.d != heartbeat {
		t.Fatalf("timers of %v, %v; want the expiry's minute, then the heartbeat", expiry.d, beat.d)
	}
	expiry.c <- now()

	if data := expect(t, r, "reset", "EventReset"); data != `{"reason":"expired"}` {
		t.Errorf("reset = %s", data)
	}
	expect(t, r, "EOF", "")
}

// A stream the hub resets ends with its reason; it then leaves the hub.
func TestAResetStreamSaysWhy(t *testing.T) {
	s := newStream(t)
	_, r := s.open(t, "tok")
	expect(t, r, "hello", "")

	s.hub.ResetAll(domain.ResetReconnected)

	if data := expect(t, r, "reset", "EventReset"); data != `{"reason":"reconnected"}` {
		t.Errorf("reset = %s", data)
	}
	expect(t, r, "EOF", "")
	waitForNoStream(t, s.hub)
}

// A client that goes ends its stream, which leaves the hub.
func TestAStreamEndsWhenItsClientGoes(t *testing.T) {
	s := newStream(t)
	res, r := s.open(t, "tok")
	expect(t, r, "hello", "")

	_ = res.Body.Close()

	waitForNoStream(t, s.hub)
}

func waitForNoStream(t *testing.T, h *app.Hub) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); h.Streams() != 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%d streams still open", h.Streams())
		}
	}
}

// A stream opened while the hub does not listen is 503 not_ready, retry
// in a second; one without a valid token is 401. Both are problems the
// contract documents.
func TestAStreamThatCannotOpenIsAProblem(t *testing.T) {
	s := newStream(t)
	s.hub.Listening(false)
	res, r := s.open(t, "tok")
	body, _ := io.ReadAll(r)
	if res.StatusCode != http.StatusServiceUnavailable || res.Header.Get("Retry-After") != "1" || !strings.Contains(string(body), `"not_ready"`) {
		t.Errorf("GET while not listening = %d %v %s; want 503 not_ready, Retry-After 1", res.StatusCode, res.Header, body)
	}
	if res, _ := s.open(t, "nope"); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET with a bad token = %d, want 401", res.StatusCode)
	}
}
