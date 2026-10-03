package httpadapter_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// slowly sees nothing until its caller gives up.
type slowly struct{}

func (slowly) WorkspacesOf(ctx context.Context, _ uuid.UUID) ([]app.Membership, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (slowly) NotebooksIn(context.Context, uuid.UUID, uuid.UUID, shared.WorkspaceRole) ([]uuid.UUID, error) {
	return nil, nil
}

// tokens accepts "tok", and "short", which expires a minute from now;
// once revoked, it refuses both with a 401, once down with a fault; once
// hung, it answers when its caller gives up, and tells hanging it has
// begun to.
type tokens struct {
	revoked, down, hung atomic.Bool
	hanging             chan struct{}
}

func (a *tokens) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	switch {
	case a.hung.Load():
		a.hanging <- struct{}{}
		<-ctx.Done()
		return nil, "", ctx.Err()
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

// logs is what the handler logged.
type logs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// stream is the server of the tests, its hub, its tokens, its timers and
// its handler's logs. A test that sets held before it opens a stream holds
// the handler in its first timer's making, past hello, until it closes
// held.
type stream struct {
	url    string
	hub    *app.Hub
	tokens *tokens
	timers chan timer
	held   chan struct{}
	logs   *logs
}

// options are a test server's heartbeat, opening timeout and visibility,
// when not the usual ones.
type options struct {
	heartbeat, openTimeout time.Duration
	visibility             app.Visibility
}

func newStream(t *testing.T) *stream {
	t.Helper()
	return newStreamWith(t, options{})
}

func newStreamWith(t *testing.T, o options) *stream {
	t.Helper()
	if o.heartbeat == 0 {
		o.heartbeat = heartbeat
	}
	if o.openTimeout == 0 {
		o.openTimeout = 5 * time.Second
	}
	if o.visibility == nil {
		o.visibility = everyone{}
	}
	s := &stream{
		hub: app.NewHub(slog.New(slog.DiscardHandler)), tokens: &tokens{hanging: make(chan struct{}, 1)},
		timers: make(chan timer, 16), logs: &logs{},
	}
	s.hub.Listening(true)
	logger := slog.New(slog.NewTextHandler(s.logs, nil))
	h := httpadapter.New(app.NewOpenStream(s.hub, o.visibility), httpadapter.Config{
		Errors: httpserver.NewAPIErrors(logger), Logger: logger, Heartbeat: o.heartbeat, OpenTimeout: o.openTimeout, Now: now,
		After: func(d time.Duration) <-chan time.Time {
			if s.held != nil {
				<-s.held
			}
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

// A reset stream first writes the events it holds, routed before the
// reset: its frames keep the order of the events, the reset last. The
// handler is held past hello while they are routed, so that it finds them
// all, and the reset, at once.
func TestAResetStreamWritesItsEventsFirst(t *testing.T) {
	s := newStream(t)
	s.held = make(chan struct{})
	_, r := s.open(t, "tok")
	expect(t, r, "hello", "")
	for range 10 {
		dispatch(t, s.hub, domain.TypePages, notebookText, domain.Pages{Pages: []domain.PageRevision{}})
	}
	s.hub.ResetAll(domain.ResetReconnected)
	close(s.held)

	for range 10 {
		expect(t, r, "pages", "")
	}
	if data := expect(t, r, "reset", "EventReset"); data != `{"reason":"reconnected"}` {
		t.Errorf("reset = %s", data)
	}
	expect(t, r, "EOF", "")
}

// A client that stops reading ends its stream once a frame waits a
// heartbeat to be written: the handler does not hang on it, and the stream
// leaves the hub. Its events come one a millisecond, so that the client's
// buffers fill before the stream's own does.
func TestAClientThatStopsReadingEndsItsStream(t *testing.T) {
	s := newStreamWith(t, options{heartbeat: 200 * time.Millisecond})
	conn, err := net.Dial("tcp", strings.TrimPrefix(s.url, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.(*net.TCPConn).SetReadBuffer(4096); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(conn, "GET /api/v0/events HTTP/1.1\r\nHost: test\r\nAuthorization: Bearer tok\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); s.hub.Streams() == 0; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the stream did not open")
		}
	}
	big := domain.Event{Type: "big", WorkspaceID: uuid.MustParse(workspaceText), NotebookID: uuid.MustParse(notebookText),
		Data: json.RawMessage(`{"filler":"` + strings.Repeat("x", 32*1024) + `"}`)}

	for deadline := time.Now().Add(10 * time.Second); s.hub.Streams() != 0; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the stream of a client that stopped reading is still open after 10 s")
		}
		s.hub.Dispatch(big)
	}
}

// A stream that ends at a heartbeat without a frame, a fault, ends its
// body cleanly well after its last frame: no write deadline is left over
// to cut its end short.
func TestAStreamEndsCleanlyLongAfterItsLastFrame(t *testing.T) {
	s := newStreamWith(t, options{heartbeat: 100 * time.Millisecond})
	_, r := s.open(t, "tok")
	expect(t, r, "hello", "")
	time.Sleep(300 * time.Millisecond)
	s.tokens.down.Store(true)

	s.timer(t).c <- now()

	if rest, err := io.ReadAll(r); err != nil || len(rest) != 0 {
		t.Errorf("the rest of the body: %q, %v; want its clean end", rest, err)
	}
}

// A heartbeat whose authentication the client's going cuts short is no
// fault: the stream ends without an error logged.
func TestAHeartbeatCutShortIsNoFault(t *testing.T) {
	s := newStream(t)
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/api/v0/events", nil)
	req.Header.Set("Authorization", "Bearer tok")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	expect(t, bufio.NewReader(res.Body), "hello", "")
	s.tokens.hung.Store(true)
	s.timer(t).c <- now()
	<-s.tokens.hanging

	cancel()

	waitForNoStream(t, s.hub)
	if logged := s.logs.String(); strings.Contains(logged, "level=ERROR") {
		t.Errorf("logged %s; want no error", logged)
	}
}

// An opening that cannot read what its caller sees within its timeout,
// server.request_timeout, ends: 500, and no stream is left.
func TestASlowOpeningEndsAtItsTimeout(t *testing.T) {
	s := newStreamWith(t, options{openTimeout: 100 * time.Millisecond, visibility: slowly{}})

	res, r := s.open(t, "tok")

	body, _ := io.ReadAll(r)
	if res.StatusCode != http.StatusInternalServerError || !strings.Contains(string(body), `"internal_error"`) {
		t.Errorf("GET = %d %s; want 500 internal_error", res.StatusCode, body)
	}
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
