package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestHealthzDoesNotRunChecks(t *testing.T) {
	failing := Check{Name: "database", Run: func(context.Context) error { return errors.New("down") }}
	router := NewRouter(slog.New(slog.DiscardHandler), failing)

	rec := serve(router, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != `{"status":"ok"}`+"\n" {
		t.Errorf("GET /healthz = %d %s, want 200 {\"status\":\"ok\"}", rec.Code, rec.Body)
	}
}

func TestReadyzWhenAllChecksPass(t *testing.T) {
	var ran []string
	check := func(name string) Check {
		return Check{Name: name, Run: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Errorf("check %s ran without a deadline", name)
			}
			ran = append(ran, name)
			return nil
		}}
	}
	router := NewRouter(slog.New(slog.DiscardHandler), check("database"), check("migrations"))

	rec := serve(router, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != `{"status":"ok"}`+"\n" {
		t.Errorf("GET /readyz = %d %s, want 200 {\"status\":\"ok\"}", rec.Code, rec.Body)
	}
	if strings.Join(ran, ",") != "database,migrations" {
		t.Errorf("checks ran = %v, want database then migrations", ran)
	}
}

func TestReadyzReportsFirstFailingCheck(t *testing.T) {
	logger, logs := captureLogs(t)
	secondRan := false
	router := NewRouter(logger,
		Check{Name: "database", Run: func(context.Context) error { return errors.New("connection refused") }},
		Check{Name: "migrations", Run: func(context.Context) error { secondRan = true; return nil }},
	)

	rec := serve(router, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if ct := rec.Result().Header.Get("Content-Type"); ct != ContentTypeProblem {
		t.Errorf("Content-Type = %q, want %q", ct, ContentTypeProblem)
	}
	want := `{"status":503,"code":"not_ready","title":"Service Unavailable","detail":"database is not ready"}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body, want)
	}
	if secondRan {
		t.Error("checks after the failing one ran")
	}
	entry := findLog(logs(), "readiness check failed")
	if entry == nil || entry["check"] != "database" || entry["error"] != "connection refused" {
		t.Errorf("log = %v, want the failing check and its error", entry)
	}
}

// A check that hangs until its context ends answers 503 once the readiness
// budget is spent, so a hung dependency cannot hang the probe.
func TestReadyzGivesUpOnAHungCheck(t *testing.T) {
	t.Parallel()
	router := NewRouter(slog.New(slog.DiscardHandler), Check{Name: "database", Run: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}})
	begin := time.Now()

	rec := serve(router, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	elapsed := time.Since(begin)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"detail":"database is not ready"`) {
		t.Errorf("GET /readyz = %d %s, want 503 for database", rec.Code, rec.Body)
	}
	if elapsed < readinessTimeout || elapsed > readinessTimeout+time.Second {
		t.Errorf("GET /readyz took %s, want about the %s budget", elapsed, readinessTimeout)
	}
}

func TestUnknownAPIPathIsProblem404(t *testing.T) {
	router := NewRouter(slog.New(slog.DiscardHandler))
	for _, target := range []string{"/api/", "/api/v0/nope"} {
		rec := serve(router, httptest.NewRequest(http.MethodPost, target, nil))

		if ct := rec.Result().Header.Get("Content-Type"); rec.Code != http.StatusNotFound || ct != ContentTypeProblem {
			t.Errorf("POST %s = %d %s, want 404 problem+json", target, rec.Code, ct)
		}
		want := `{"status":404,"code":"not_found","title":"Not Found","detail":"no API endpoint for POST ` + target + `"}` + "\n"
		if rec.Body.String() != want {
			t.Errorf("body = %s, want %s", rec.Body, want)
		}
	}
}

// The whole-program tests compare the registered API patterns with the
// contract (P4), so both registration methods must record.
func TestRouterRecordsEveryPattern(t *testing.T) {
	router := NewRouter(slog.New(slog.DiscardHandler))
	router.HandleFunc("GET /api/v0/things", func(http.ResponseWriter, *http.Request) {})
	router.Handle("/", http.NotFoundHandler())

	want := []string{"GET /healthz", "GET /readyz", "/api/", "GET /api/v0/things", "/"}
	if got := router.Patterns(); !slices.Equal(got, want) {
		t.Errorf("Patterns() = %q, want %q", got, want)
	}
}

func TestRouterRoutesToTheRegisteredHandler(t *testing.T) {
	router := NewRouter(slog.New(slog.DiscardHandler))
	var pattern string
	router.HandleFunc("GET /api/v0/things/{id}", func(_ http.ResponseWriter, r *http.Request) { pattern = r.Pattern })

	serve(router, httptest.NewRequest(http.MethodGet, "/api/v0/things/7", nil))

	if pattern != "GET /api/v0/things/{id}" {
		t.Errorf("r.Pattern = %q, want the registered pattern", pattern)
	}
}

func TestOtherPathsAreLeftForTheWebUI(t *testing.T) {
	rec := serve(NewRouter(slog.New(slog.DiscardHandler)), httptest.NewRequest(http.MethodGet, "/notebooks", nil))

	if ct := rec.Result().Header.Get("Content-Type"); rec.Code != http.StatusNotFound || ct == ContentTypeProblem {
		t.Errorf("GET /notebooks = %d %s, want the router's plain 404", rec.Code, ct)
	}
}
