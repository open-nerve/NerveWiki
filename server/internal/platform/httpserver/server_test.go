package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

var client = &http.Client{Timeout: 5 * time.Second}

// startServer runs a Server on a random local port. Cancelling the returned
// context starts the shutdown; done yields the result of Serve.
func startServer(t *testing.T, shutdownTimeout time.Duration, h http.Handler) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	cfg := config.ServerConfig{
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		ShutdownTimeout:   shutdownTimeout,
	}
	return startServerWith(t, cfg, h)
}

// startServerWith is startServer with the given timeouts; cfg.Addr is ignored.
func startServerWith(t *testing.T, cfg config.ServerConfig, h http.Handler) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Addr = ln.Addr().String()
	srv := NewServer(cfg, h, slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln) }()
	return "http://" + ln.Addr().String(), cancel, done
}

func wait(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return")
		return nil
	}
}

func TestServeAppliesMiddlewareAndStopsOnCancel(t *testing.T) {
	url, cancel, done := startServer(t, time.Second, NewRouter(slog.New(slog.DiscardHandler)))

	resp, err := client.Get(url + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get(HeaderRequestID) == "" {
		t.Errorf("GET /healthz = %d, X-Request-Id %q; want 200 with a request ID", resp.StatusCode, resp.Header.Get(HeaderRequestID))
	}

	cancel()
	if err := wait(t, done); err != nil {
		t.Errorf("Serve() = %v, want nil after a clean shutdown", err)
	}
}

func TestShutdownDrainsInFlightRequests(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	url, cancel, done := startServer(t, 5*time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "finished")
	}))
	type result struct {
		body string
		err  error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := client.Get(url + "/slow")
		if err != nil {
			got <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		got <- result{string(body), err}
	}()
	<-started

	cancel()
	addr := strings.TrimPrefix(url, "http://")
	for deadline := time.Now().Add(2 * time.Second); ; {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			break // the listener is closed: no new connections
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("server still accepts connections after shutdown began")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)

	if r := <-got; r.err != nil || r.body != "finished" {
		t.Errorf("in-flight request = %q, %v; want it to finish", r.body, r.err)
	}
	if err := wait(t, done); err != nil {
		t.Errorf("Serve() = %v, want nil", err)
	}
}

func TestShutdownGivesUpAfterTimeout(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	url, cancel, done := startServer(t, 100*time.Millisecond, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}))
	go func() {
		if resp, err := client.Get(url + "/stuck"); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started

	begin := time.Now()
	cancel()
	err := wait(t, done)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Serve() = %v, want a shutdown deadline error", err)
	}
	if elapsed := time.Since(begin); elapsed > 2*time.Second {
		t.Errorf("shutdown took %s, want about the 100ms timeout", elapsed)
	}
}

// read_header_timeout stops at the headers. A client that sends complete
// headers but never the body it announced must still let go of the
// connection: read_timeout bounds the whole request.
func TestReadTimeoutReleasesARequestWhoseBodyNeverArrives(t *testing.T) {
	cfg := config.ServerConfig{
		ReadHeaderTimeout: 100 * time.Millisecond,
		ReadTimeout:       300 * time.Millisecond,
		WriteTimeout:      5 * time.Second,
		ShutdownTimeout:   time.Second,
	}
	url, _, _ := startServerWith(t, cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	conn, err := net.Dial("tcp", strings.TrimPrefix(url, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 1\r\n\r\n"); err != nil {
		t.Fatal(err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	reply, err := io.ReadAll(conn) // returns once the server closes the connection
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		t.Fatal("the server still holds the connection after 3s, want it closed after the 300ms read_timeout")
	}
	// The handler answered, so the headers were read in time: it was the
	// missing body, not the header timeout, that ended the connection.
	if !strings.HasPrefix(string(reply), "HTTP/1.1 200 ") {
		t.Errorf("reply = %q, want the handler's 200 before the connection closed", reply)
	}
}

// write_timeout bounds how long the response may take to produce and send: a
// handler that answers late, or a client that stops reading, cannot hold the
// response past it. It fails the writes; it neither stops the handler nor
// cancels its context.
func TestWriteTimeoutCutsOffALateResponse(t *testing.T) {
	cfg := config.ServerConfig{
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       time.Second,
		WriteTimeout:      200 * time.Millisecond,
		ShutdownTimeout:   time.Second,
	}
	url, _, _ := startServerWith(t, cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = io.WriteString(w, "late")
	}))

	resp, err := client.Get(url + "/late")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatalf("GET /late = %d, want the connection cut off after the 200ms write_timeout", resp.StatusCode)
	}
}

func TestListenAndServeReportsListenError(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = taken.Close() }()
	cfg := config.ServerConfig{Addr: taken.Addr().String(), ReadHeaderTimeout: time.Second, ShutdownTimeout: time.Second}

	err = NewServer(cfg, http.NotFoundHandler(), slog.New(slog.DiscardHandler)).ListenAndServe(context.Background())

	if err == nil || !strings.Contains(err.Error(), "listen on "+taken.Addr().String()) {
		t.Errorf("ListenAndServe() = %v, want a listen error", err)
	}
}

// Listening on port 0, the server tells where it listens through
// server.addr_file; the end-to-end fixture reads it instead of guessing a
// free port (P6).
func TestListenAndServeWritesTheAddrFile(t *testing.T) {
	addrFile := filepath.Join(t.TempDir(), "addr")
	cfg := config.ServerConfig{Addr: "127.0.0.1:0", AddrFile: addrFile, ReadHeaderTimeout: time.Second, ShutdownTimeout: time.Second}
	srv := NewServer(cfg, NewRouter(slog.New(slog.DiscardHandler)), slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()

	var addr []byte
	for deadline := time.Now().Add(5 * time.Second); len(addr) == 0 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		addr, _ = os.ReadFile(addrFile)
	}
	resp, err := client.Get("http://" + string(addr) + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz at the address in addr_file %q: %v", addr, err)
	}
	_ = resp.Body.Close()
	cancel()
	if err := wait(t, done); err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("GET /healthz = %d, ListenAndServe() = %v; want 200 and nil", resp.StatusCode, err)
	}
}

// The error names the key, never the path: *_file keys are not logged.
func TestListenAndServeReportsAnUnwritableAddrFile(t *testing.T) {
	addrFile := filepath.Join(t.TempDir(), "secret-dir", "missing", "addr")
	cfg := config.ServerConfig{Addr: "127.0.0.1:0", AddrFile: addrFile, ReadHeaderTimeout: time.Second, ShutdownTimeout: time.Second}

	err := NewServer(cfg, http.NotFoundHandler(), slog.New(slog.DiscardHandler)).ListenAndServe(context.Background())

	if err == nil || !strings.HasPrefix(err.Error(), "server.addr_file: ") || strings.Contains(err.Error(), "secret-dir") {
		t.Errorf("ListenAndServe() = %v, want a server.addr_file error without the path", err)
	}
}
