package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

const (
	thingRoute  = "POST /api/v0/things"
	eventsRoute = "GET /api/v0/events"
)

// thingBody is a table as bodyshapegen writes it: {"name": string}, closed.
func thingBody() *bodyshape.Table {
	return &bodyshape.Table{
		Nodes: []bodyshape.Node{
			{Types: bodyshape.Object, Extra: bodyshape.Closed, Items: bodyshape.Open, Props: map[string]int{"name": 1}, Required: []string{"name"}},
			{Types: bodyshape.String, Extra: bodyshape.Open, Items: bodyshape.Open},
		},
		Roots: map[string]int{thingRoute: 0},
	}
}

type reached struct {
	called bool
	ctx    context.Context
	body   string
}

// mount registers a handler for thingRoute on a router behind api's
// middlewares, applied the way the generated code does: for each middleware
// in the list, h = m(h).
func mount(api *API) (*Router, *reached) {
	got := &reached{}
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.called, got.ctx = true, r.Context()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			api.Errors.BodyError(w, r, err)
			return
		}
		got.body = string(data)
		w.WriteHeader(http.StatusNoContent)
	})
	for _, m := range api.Middlewares(thingBody()) {
		h = m(h)
	}
	router := NewRouter(slog.New(slog.DiscardHandler))
	router.Handle(thingRoute, h)
	return router, got
}

func newTestAPI(t *testing.T) *API {
	t.Helper()
	api, err := NewAPI(APIConfig{Logger: slog.New(slog.DiscardHandler), MaxBodyBytes: 64, RequestTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewAPI() error = %v", err)
	}
	return api
}

func post(path, body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) Problem {
	t.Helper()
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body %q: %v", rec.Body, err)
	}
	return p
}

func TestNewAPIRequiresItsSettings(t *testing.T) {
	_, err := NewAPI(APIConfig{})

	want := "httpserver: APIConfig: no Logger\nMaxBodyBytes must be positive\nRequestTimeout must be positive"
	if err == nil || err.Error() != want {
		t.Errorf("NewAPI() error = %v, want %q", err, want)
	}
}

func TestValidBodyReachesTheHandler(t *testing.T) {
	router, got := mount(newTestAPI(t))

	rec := serve(router, post("/api/v0/things", `{"name":"a"}`))

	if rec.Code != http.StatusNoContent || !got.called || got.body != `{"name":"a"}` {
		t.Errorf("status = %d, handler called %v with %q; want 204 and the body unchanged", rec.Code, got.called, got.body)
	}
}

// server.write_timeout never cancels the request's context: the operation's
// own deadline does.
func TestOperationsRunUnderTheRequestDeadline(t *testing.T) {
	router, got := mount(newTestAPI(t))
	begin := time.Now()

	serve(router, post("/api/v0/things", `{"name":"a"}`))

	end := time.Now()
	deadline, ok := got.ctx.Deadline()
	if !ok || deadline.Before(begin.Add(2*time.Second)) || deadline.After(end.Add(2*time.Second)) {
		t.Errorf("handler deadline = %v (set %v), want 2s after the request arrived", deadline, ok)
	}
}

// A long-lived route is registered on the router directly, next to the API
// operations: it gets no request deadline, as it must hold its response
// open (M0/P3 design 3.4).
func TestLongLivedRoutesHaveNoRequestDeadline(t *testing.T) {
	router, _ := mount(newTestAPI(t))
	var streamCtx context.Context
	router.Handle(eventsRoute, LongLived(slog.New(slog.DiscardHandler), http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		streamCtx = r.Context()
	})))
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	resp, err := client().Get(srv.URL + "/api/v0/events")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if _, ok := streamCtx.Deadline(); ok || resp.StatusCode != http.StatusOK {
		t.Errorf("long-lived route answered %d with a deadline %v; want 200 without one", resp.StatusCode, ok)
	}
}

// The body check reads the body through the body limit: the limit runs
// before it, and the check's own read is what fails.
func TestBodyLimitRunsBeforeTheBodyCheck(t *testing.T) {
	router, got := mount(newTestAPI(t))

	rec := serve(router, post("/api/v0/things", `{"name":"`+strings.Repeat("a", 100)+`"}`))

	if p := decodeProblem(t, rec); rec.Code != http.StatusRequestEntityTooLarge || p.Code != CodePayloadTooLarge || got.called {
		t.Errorf("response = %d %+v, handler called %v; want 413 payload_too_large", rec.Code, p, got.called)
	}
}

func TestBodyCheckAnswersEveryProblemAs400(t *testing.T) {
	router, got := mount(newTestAPI(t))

	rec := serve(router, post("/api/v0/things", `{"extra":1}`))

	want := `{"status":400,"code":"bad_request","title":"Bad Request","detail":"The request body does not match the API description.",` +
		`"errors":[{"field":"extra","code":"not_allowed","message":"is not a property of this request"},{"field":"name","code":"required","message":"is required"}]}` + "\n"
	if rec.Code != http.StatusBadRequest || rec.Body.String() != want || got.called {
		t.Errorf("response = %d %s, handler called %v; want 400 %s", rec.Code, rec.Body, got.called, want)
	}
}

func TestBodyThatIsNotJSONIs400WithAGenericDetail(t *testing.T) {
	router, _ := mount(newTestAPI(t))

	rec := serve(router, post("/api/v0/things", `{"name":`))

	if p := decodeProblem(t, rec); rec.Code != http.StatusBadRequest || p.Detail != "The request body could not be decoded." || len(p.Errors) != 0 {
		t.Errorf("response = %d %+v, want 400 with the generic detail", rec.Code, p)
	}
}
