package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// With every connection deadline at 200ms, an ordinary route cannot answer
// after 500ms, while a long-lived route pushes for a whole second: its writes
// still go out, and the read deadline does not cancel its context, as net/http
// lifts that deadline once the request body has been read.
func TestLongLivedOutlastsTheConnectionDeadlines(t *testing.T) {
	const ticks = 10
	cfg := config.ServerConfig{
		ReadHeaderTimeout: 200 * time.Millisecond,
		ReadTimeout:       200 * time.Millisecond,
		WriteTimeout:      200 * time.Millisecond,
		ShutdownTimeout:   time.Second,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /late", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = io.WriteString(w, "late")
	})
	mux.Handle("GET /stream", LongLived(slog.New(slog.DiscardHandler), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		for i := range ticks {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
			_, _ = fmt.Fprintf(w, "tick %d\n", i)
			_ = rc.Flush()
		}
	})))
	url, _, _ := startServerWith(t, cfg, mux)

	if resp, err := client.Get(url + "/late"); err == nil {
		_ = resp.Body.Close()
		t.Errorf("GET /late = %d, want the connection cut off after the 200ms write_timeout", resp.StatusCode)
	}

	resp, err := client.Get(url + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if got := strings.Count(string(body), "tick "); err != nil || got != ticks {
		t.Errorf("GET /stream received %d ticks (%v), want all %d: %q", got, err, ticks, body)
	}
}

// Shutdown cancels the context of a long-lived handler at once, so Serve
// returns well within shutdown_timeout; an ordinary request in flight at the
// same time finishes normally, its context intact.
func TestShutdownEndsLongLivedResponses(t *testing.T) {
	cfg := config.ServerConfig{
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		ShutdownTimeout:   5 * time.Second,
	}
	streaming, ended := make(chan struct{}), make(chan error, 1)
	slowStarted, release := make(chan struct{}), make(chan struct{})
	mux := http.NewServeMux()
	mux.Handle("GET /stream", LongLived(slog.New(slog.DiscardHandler), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = http.NewResponseController(w).Flush()
		close(streaming)
		<-r.Context().Done()
		ended <- r.Context().Err()
	})))
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		close(slowStarted)
		<-release
		if err := r.Context().Err(); err != nil {
			_, _ = fmt.Fprintf(w, "cancelled: %v", err)
			return
		}
		_, _ = io.WriteString(w, "finished")
	})
	url, cancel, done := startServerWith(t, cfg, mux)

	stream, err := client.Get(url + "/stream") // returns once the headers are flushed
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Body.Close() }()
	<-streaming
	slow := make(chan string, 1)
	go func() {
		resp, err := client.Get(url + "/slow")
		if err != nil {
			slow <- err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		slow <- string(body)
	}()
	<-slowStarted

	begin := time.Now()
	cancel()
	select {
	case err := <-ended:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("long-lived handler's context ended with %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not cancel the long-lived handler's context")
	}
	close(release)

	if got := <-slow; got != "finished" {
		t.Errorf("ordinary request in flight = %q, want it to finish", got)
	}
	if err := wait(t, done); err != nil {
		t.Errorf("Serve() = %v, want nil", err)
	}
	if elapsed := time.Since(begin); elapsed > 2*time.Second {
		t.Errorf("shutdown took %s, want far less than the 5s shutdown_timeout", elapsed)
	}
}

// A long-lived route still reads its request body under read_timeout: a
// client that announces a body and never sends it cannot hold the handler.
func TestLongLivedKeepsTheReadDeadlineForTheBody(t *testing.T) {
	cfg := config.ServerConfig{
		ReadHeaderTimeout: 200 * time.Millisecond,
		ReadTimeout:       200 * time.Millisecond,
		WriteTimeout:      200 * time.Millisecond,
		ShutdownTimeout:   time.Second,
	}
	read := make(chan error, 1)
	mux := http.NewServeMux()
	mux.Handle("POST /stream", LongLived(slog.New(slog.DiscardHandler), http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		read <- err
	})))
	url, _, _ := startServerWith(t, cfg, mux)
	conn, err := net.Dial("tcp", strings.TrimPrefix(url, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "POST /stream HTTP/1.1\r\nHost: localhost\r\nContent-Length: 5\r\n\r\n"); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-read:
		if ne, ok := errors.AsType[net.Error](err); !ok || !ne.Timeout() {
			t.Errorf("reading the body = %v, want a timeout", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the body read still waits after 3s, want it ended by the 200ms read_timeout")
	}
}

// httptest's recorder cannot set deadlines: LongLived answers 500 and logs
// the fault instead of running the handler with a deadline it cannot lift.
func TestLongLivedRejectsAWriterWithoutDeadlines(t *testing.T) {
	logger, logs := captureLogs(t)
	ran := false
	h := LongLived(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true }))

	rec := serve(h, httptest.NewRequest(http.MethodGet, "/events", nil))

	if ran || rec.Code != http.StatusInternalServerError || rec.Result().Header.Get("Content-Type") != ContentTypeProblem {
		t.Errorf("handler ran = %t, response = %d %q; want it not run and a 500 problem", ran, rec.Code, rec.Result().Header.Get("Content-Type"))
	}
	entry := findLog(logs(), "cannot lift the write deadline of a long-lived response")
	if entry == nil || entry["path"] != "/events" || entry["level"] != "ERROR" {
		t.Errorf("log = %v, want an error naming the path", entry)
	}
}
