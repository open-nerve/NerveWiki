package httpserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"
)

// captureLogs returns a JSON logger and a function that decodes what it wrote.
func captureLogs(t *testing.T) (*slog.Logger, func() []map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, func() []map[string]any {
		var entries []map[string]any
		for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
			if line == "" {
				continue
			}
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatalf("log line %q: %v", line, err)
			}
			entries = append(entries, entry)
		}
		return entries
	}
}

func findLog(entries []map[string]any, msg string) map[string]any {
	for _, e := range entries {
		if e["msg"] == msg {
			return e
		}
	}
	return nil
}

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestRequestIDIsGeneratedWhenMissing(t *testing.T) {
	var seen string
	h := middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = RequestID(r.Context())
	}), slog.New(slog.DiscardHandler))

	rec := serve(h, httptest.NewRequest(http.MethodGet, "/", nil))

	id := rec.Result().Header.Get(HeaderRequestID)
	if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("X-Request-Id = %q, want a UUID: %v", id, err)
	}
	if seen != id {
		t.Errorf("handler saw request ID %q, response has %q", seen, id)
	}
}

func TestRequestIDFromCaller(t *testing.T) {
	tests := []struct {
		name, header string
		kept         bool
	}{
		{"safe token is kept", "req-42_a.b:c", true},
		{"spaces are rejected", "req 42", false},
		{"control characters are rejected", "req\n42", false},
		{"overlong IDs are rejected", strings.Repeat("a", 129), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := middleware(http.NotFoundHandler(), slog.New(slog.DiscardHandler))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(HeaderRequestID, tt.header)

			got := serve(h, req).Result().Header.Get(HeaderRequestID)
			if kept := got == tt.header; kept != tt.kept {
				t.Errorf("X-Request-Id = %q for caller ID %q, kept = %v, want %v", got, tt.header, kept, tt.kept)
			}
		})
	}
}

// wantSecurityHeaders are the headers every response carries.
func wantSecurityHeaders() map[string]string {
	return map[string]string{"X-Content-Type-Options": "nosniff", "Referrer-Policy": "same-origin", "X-Frame-Options": "DENY"}
}

// Every response carries the security headers, whatever the handler answers.
func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	handlers := map[string]http.Handler{
		"200": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }),
		"404": http.NotFoundHandler(),
		"problem": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			WriteProblem(w, Problem{Status: http.StatusBadRequest, Code: "bad_request"})
		}),
	}
	want := wantSecurityHeaders()
	for name, h := range handlers {
		rec := serve(middleware(h, slog.New(slog.DiscardHandler)), httptest.NewRequest(http.MethodGet, "/", nil))

		for header, value := range want {
			if got := rec.Result().Header.Get(header); got != value {
				t.Errorf("%s response: %s = %q, want %q", name, header, got, value)
			}
		}
	}
}

// Every response under /api/ carries Cache-Control: no-store: an answer, a
// problem, the platform's /api/ fallback and the 500 of a panic. Other
// responses get no caching from the chain: the web UI sets its own.
func TestAPIResponsesAreNotStored(t *testing.T) {
	discard := slog.New(slog.DiscardHandler)
	answer := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{}")) })
	page := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte("<!doctype html>"))
	})
	tests := []struct {
		name, path string
		h          http.Handler
		want       string
	}{
		{"an answer", "/api/v0/me", answer, "no-store"},
		{"a problem", "/api/v0/me", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			WriteProblem(w, Problem{Status: http.StatusUnauthorized, Code: "unauthorized"})
		}), "no-store"},
		{"the fallback", "/api/v0/nope", NewRouter(discard), "no-store"},
		{"a panic", "/api/v0/me", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), "no-store"},
		{"a page", "/notebooks", page, "no-cache"},
		{"a probe", "/healthz", NewRouter(discard), ""},
		{"a path that only starts like the API", "/apis", answer, ""},
	}
	for _, tt := range tests {
		rec := serve(middleware(tt.h, discard), httptest.NewRequest(http.MethodGet, tt.path, nil))

		if got := rec.Result().Header.Get("Cache-Control"); got != tt.want {
			t.Errorf("%s (GET %s, %d): Cache-Control = %q, want %q", tt.name, tt.path, rec.Code, got, tt.want)
		}
	}
}

func TestPanicBecomes500Problem(t *testing.T) {
	logger, logs := captureLogs(t)
	h := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}), logger)
	req := httptest.NewRequest(http.MethodGet, "/explode", nil)
	req.Header.Set(HeaderRequestID, "req-1")

	rec := serve(h, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Result().Header.Get("Content-Type"); ct != ContentTypeProblem {
		t.Errorf("Content-Type = %q, want %q", ct, ContentTypeProblem)
	}
	want := `{"status":500,"code":"internal_error","title":"Internal Server Error"}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body, want)
	}
	entries := logs()
	panicLog := findLog(entries, "panic serving request")
	if panicLog == nil || panicLog["panic"] != "boom" || panicLog["request_id"] != "req-1" || panicLog["stack"] == "" {
		t.Errorf("panic log = %v, want panic, request_id and stack", panicLog)
	}
	access := findLog(entries, "http request")
	if access == nil || access["status"] != float64(500) || access["request_id"] != "req-1" {
		t.Errorf("access log = %v, want status 500 and request_id req-1", access)
	}
}

func TestPanicDiscardsHeadersSetBeforeIt(t *testing.T) {
	// A real server: a stale Content-Length would truncate the problem body.
	srv := httptest.NewServer(middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Set-Cookie", "a=b")
		w.Header().Set("Content-Length", "5")
		panic("boom")
	}), slog.New(slog.DiscardHandler)))
	defer srv.Close()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(HeaderRequestID, "req-3")

	resp, err := client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError || resp.Header.Get("Content-Type") != ContentTypeProblem {
		t.Errorf("response = %d %q, want 500 %s", resp.StatusCode, resp.Header.Get("Content-Type"), ContentTypeProblem)
	}
	if cookies := resp.Header.Values("Set-Cookie"); len(cookies) != 0 {
		t.Errorf("Set-Cookie = %q, want none: the handler's headers must not leak", cookies)
	}
	if id := resp.Header.Get(HeaderRequestID); id != "req-3" {
		t.Errorf("X-Request-Id = %q, want req-3", id)
	}
	for header, value := range wantSecurityHeaders() {
		if got := resp.Header.Get(header); got != value {
			t.Errorf("%s = %q, want %q: the 500 of a panic keeps the security headers", header, got, value)
		}
	}
	var p Problem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil || p.Code != CodeInternal {
		t.Errorf("body = %+v, %v; want the complete internal_error problem", p, err)
	}
}

func TestPanicAfterResponseStartedAbortsConnection(t *testing.T) {
	h := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial"))
		panic("boom")
	}), slog.New(slog.DiscardHandler))

	defer func() {
		if v := recover(); v != http.ErrAbortHandler { //nolint:errorlint // net/http recognises only this exact value
			t.Errorf("recovered %v, want http.ErrAbortHandler", v)
		}
	}()
	serve(h, httptest.NewRequest(http.MethodGet, "/", nil))
	t.Error("ServeHTTP returned normally, want a panic")
}

// A flush commits the response as a write does: a panic after it aborts the
// connection too, rather than appending a 500 problem to the 200 the client
// has already received. Event streams flush without writing first.
func TestPanicAfterFlushAbortsConnection(t *testing.T) {
	for name, flush := range map[string]func(http.ResponseWriter){
		"ResponseController": func(w http.ResponseWriter) { _ = http.NewResponseController(w).Flush() },
		"http.Flusher":       func(w http.ResponseWriter) { w.(http.Flusher).Flush() },
	} {
		t.Run(name, func(t *testing.T) {
			h := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				flush(w)
				panic("boom")
			}), slog.New(slog.DiscardHandler))
			rec := httptest.NewRecorder()

			defer func() {
				if v := recover(); v != http.ErrAbortHandler { //nolint:errorlint // net/http recognises only this exact value
					t.Errorf("recovered %v, want http.ErrAbortHandler", v)
				}
				if !rec.Flushed || rec.Code != http.StatusOK || rec.Body.Len() != 0 {
					t.Errorf("response = %d flushed %t body %q, want the flushed 200 alone", rec.Code, rec.Flushed, rec.Body)
				}
			}()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/events", nil))
			t.Error("ServeHTTP returned normally, want a panic")
		})
	}
}

func TestAbortHandlerPanicIsNotRecovered(t *testing.T) {
	logger, logs := captureLogs(t)
	h := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}), logger)

	defer func() {
		if v := recover(); v != http.ErrAbortHandler { //nolint:errorlint // net/http recognises only this exact value
			t.Errorf("recovered %v, want http.ErrAbortHandler", v)
		}
		if findLog(logs(), "panic serving request") != nil {
			t.Error("a deliberate abort was logged as a panic")
		}
	}()
	serve(h, httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestAccessLog(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    float64
	}{
		{"explicit status", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }, 201},
		{"implicit 200", func(http.ResponseWriter, *http.Request) {}, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logs := captureLogs(t)
			req := httptest.NewRequest(http.MethodPost, "/things?x=1", nil)
			req.Header.Set(HeaderRequestID, "req-2")

			serve(middleware(tt.handler, logger), req)

			entry := findLog(logs(), "http request")
			if entry == nil {
				t.Fatal("no access log entry")
			}
			if entry["method"] != "POST" || entry["path"] != "/things" || entry["status"] != tt.want ||
				entry["request_id"] != "req-2" || entry["duration"] == nil || entry["level"] != "INFO" {
				t.Errorf("access log = %v", entry)
			}
		})
	}
}

// Orchestrators probe every few seconds: the probes' access log lines are
// debug level.
func TestAccessLogOfProbesIsDebug(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz"} {
		logger, logs := captureLogs(t)

		serve(middleware(http.NotFoundHandler(), logger), httptest.NewRequest(http.MethodGet, path, nil))

		if entry := findLog(logs(), "http request"); entry == nil || entry["level"] != "DEBUG" {
			t.Errorf("GET %s access log = %v, want level DEBUG", path, entry)
		}
	}
}
