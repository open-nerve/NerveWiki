package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// sampleMigrations are two probe tables, for the tests of how the app applies
// and reports migrations, whatever the production set holds.
func sampleMigrations() fstest.MapFS {
	return fstest.MapFS{
		"00001_probe_create_widgets.sql": {Data: []byte("-- +goose Up\nCREATE TABLE widgets (id bigint);\n-- +goose Down\nDROP TABLE widgets;\n")},
		"00002_probe_create_gadgets.sql": {Data: []byte("-- +goose Up\nCREATE TABLE gadgets (id bigint);\n-- +goose Down\nDROP TABLE gadgets;\n")},
	}
}

const unreachableDB = "postgres://nobody@127.0.0.1:1/nowhere"

// ctypeC is a database whose LC_CTYPE is C: the database check refuses it.
const ctypeC = "LOCALE_PROVIDER builtin BUILTIN_LOCALE 'C.UTF-8' LC_COLLATE 'C' LC_CTYPE 'C'"

// client bounds every request of a test.
func client() *http.Client { return &http.Client{Timeout: 5 * time.Second} }

// testConfig listens on a port the system picks and reports it through
// server.addr_file.
func testConfig(t *testing.T, dbURL string, autoMigrate bool) config.Config {
	t.Helper()
	return config.Config{
		Env: config.EnvTest,
		Server: config.ServerConfig{
			Addr:              "127.0.0.1:0",
			AddrFile:          filepath.Join(t.TempDir(), "addr"),
			ReadHeaderTimeout: time.Second,
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      5 * time.Second,
			ShutdownTimeout:   5 * time.Second,
		},
		Database: config.DatabaseConfig{URL: dbURL, MaxConns: 4, AutoMigrate: autoMigrate, CommitTimeout: 2 * time.Second},
		Log:      config.LogConfig{Level: "error", Format: "text"},
	}
}

// buildApp wires the app and closes it when the test ends.
func buildApp(t *testing.T, cfg config.Config, migrations fs.FS) *app {
	t.Helper()
	a, err := newApp(context.Background(), cfg, slog.New(slog.DiscardHandler), migrations)
	if err != nil {
		t.Fatalf("newApp() error = %v", err)
	}
	t.Cleanup(a.close)
	return a
}

// waitHealthy returns the base URL of the server that writes addrFile once it
// answers /healthz, or fails the test when done yields first.
func waitHealthy(t *testing.T, addrFile string, done <-chan error) string {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		select {
		case err := <-done:
			t.Fatalf("the server stopped early: %v", err)
		default:
		}
		addr, err := os.ReadFile(addrFile)
		if err != nil {
			continue
		}
		base := "http://" + string(addr)
		if resp, err := client().Get(base + "/healthz"); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return base
			}
		}
	}
	t.Fatal("server did not become healthy")
	return ""
}

// startApp runs the app until the test ends and returns its base URL once it
// answers /healthz. The cleanup checks that run shut down cleanly.
func startApp(t *testing.T, cfg config.Config, migrations fs.FS) string {
	t.Helper()
	a := buildApp(t, cfg, migrations)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.run(ctx) }()
	// Cleanups run last-in first-out: run stops before buildApp's close.
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("run() = %v, want nil after cancel", err)
		}
	})
	return waitHealthy(t, cfg.Server.AddrFile, done)
}

func getReadyz(t *testing.T, h http.Handler) (int, httpserver.Problem) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	var p httpserver.Problem
	if rec.Code != http.StatusOK {
		if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
			t.Fatalf("decode problem: %v", err)
		}
	}
	return rec.Code, p
}

// serve migrates an empty database with the production migrations, checks
// it, answers /healthz and /readyz, and stops in order when cancelled: HTTP,
// then the pool.
func TestServeRunsUntilCancelled(t *testing.T) {
	cfg := testConfig(t, pgtest.NewEmptyDatabase(t), true)
	cfg.Log = config.LogConfig{Level: "info", Format: "text"}
	var logs syncBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, cfg, &logs) }()

	base := waitHealthy(t, cfg.Server.AddrFile, done)
	resp, err := client().Get(base + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	cancel()
	if err := <-done; err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /readyz = %d, Serve() = %v; want 200 and nil after cancel; logs:\n%s", resp.StatusCode, err, logs.String())
	}

	out := logs.String()
	last := -1
	for _, step := range []string{
		`msg="configuration loaded"`,
		`msg="database pool created"`,
		`msg="migration applied" version=1 source=00001_platform_pg_trgm.sql`,
		`msg="http server listening"`,
		`msg="http server stopped"`,
		`msg="database pool closed"`,
	} {
		at := strings.Index(out, step)
		if at <= last {
			t.Errorf("logs have %s at %d, want it after the step before (at %d):\n%s", step, at, last, out)
		}
		last = at
	}
}

func TestNotReadyWithPendingMigrations(t *testing.T) {
	a := buildApp(t, testConfig(t, pgtest.NewEmptyDatabase(t), false), sampleMigrations())

	status, p := getReadyz(t, a.router)
	if status != http.StatusServiceUnavailable || p.Code != httpserver.CodeNotReady || p.Detail != "migrations is not ready" {
		t.Errorf("GET /readyz = %d %+v, want 503 not_ready for migrations", status, p)
	}
}

func TestNotReadyWhenDatabaseIsUnavailable(t *testing.T) {
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())

	status, p := getReadyz(t, a.router)
	if status != http.StatusServiceUnavailable || p.Code != httpserver.CodeNotReady || p.Detail != "database is not ready" {
		t.Errorf("GET /readyz = %d %+v, want 503 not_ready for database", status, p)
	}
}

// With database.auto_migrate off, serve still starts on a database behind
// the migrations, and /readyz tells the orchestrator to wait.
func TestServesNotReadyWithoutAutoMigrate(t *testing.T) {
	base := startApp(t, testConfig(t, pgtest.NewEmptyDatabase(t), false), sampleMigrations())

	resp, err := client().Get(base + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz = %d, want 503", resp.StatusCode)
	}
}

func TestRunFailsWhenAutoMigrateFails(t *testing.T) {
	a := buildApp(t, testConfig(t, unreachableDB, true), sampleMigrations())
	// Bounded: should run succeed instead, it would serve until ctx is done.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.run(ctx); err == nil {
		t.Error("run() = nil, want the migration error")
	}
}

// serve refuses a database that fails the check, before it listens, and
// logs the refusal as a structured record.
func TestServeRefusesADatabaseThatFailsTheCheck(t *testing.T) {
	cfg := testConfig(t, pgtest.NewEmptyDatabaseWith(t, ctypeC), true)
	cfg.Log = config.LogConfig{Level: "error", Format: "json"}
	var logs bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := Serve(ctx, cfg, &logs)

	if err == nil || !strings.Contains(err.Error(), "is not set up for nervewiki") {
		t.Fatalf("Serve() = %v, want the database check's error", err)
	}
	var entry map[string]any
	if jsonErr := json.Unmarshal(logs.Bytes(), &entry); jsonErr != nil || entry["msg"] != "nervewiki serve failed" || entry["error"] != err.Error() {
		t.Errorf("logs = %s, want one record of the failure with its error", logs.String())
	}
	if _, statErr := os.Stat(cfg.Server.AddrFile); statErr == nil {
		t.Error("serve listened before the database check")
	}
}

// database.url is masked as a whole in the configuration log, so newApp logs
// where the pool connects, as pgx parsed it, and never the password.
func TestNewAppLogsTheDatabaseTarget(t *testing.T) {
	var logs bytes.Buffer
	cfg := testConfig(t, "postgres://nobody:secret@127.0.0.1:1/nowhere?password=secret;more", false)
	a, err := newApp(context.Background(), cfg, slog.New(slog.NewTextHandler(&logs, nil)), sampleMigrations())
	if err != nil {
		t.Fatalf("newApp() error = %v", err)
	}
	defer a.close()

	out := logs.String()
	if strings.Contains(out, "secret") {
		t.Errorf("log output leaks the password: %s", out)
	}
	if !strings.Contains(out, `msg="database pool created" host=127.0.0.1 port=1 database=nowhere user=nobody`) {
		t.Errorf("log output lacks the database target: %s", out)
	}
}

// close waits for the pool's connections for poolCloseTimeout at most: a
// handler that ignores its context may hold one.
func TestCloseDoesNotWaitForAConnectionInUse(t *testing.T) {
	var logs bytes.Buffer
	a, err := newApp(context.Background(), testConfig(t, pgtest.NewDatabase(t), false), slog.New(slog.NewTextHandler(&logs, nil)), sampleMigrations())
	if err != nil {
		t.Fatalf("newApp() error = %v", err)
	}
	if a.poolCloseTimeout != 5*time.Second {
		t.Errorf("newApp waits %s for the pool, want 5s", a.poolCloseTimeout)
	}
	a.poolCloseTimeout = 200 * time.Millisecond
	conn, err := a.pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()

	closing := time.Now()
	closed := make(chan struct{})
	go func() {
		a.close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("close() still waits for the connection after 2s")
	}

	if took := time.Since(closing); took < 200*time.Millisecond {
		t.Errorf("close() returned after %s, want it to wait 200ms for the connection", took)
	}
	if !strings.Contains(logs.String(), `msg="database pool not closed: connections still in use" waited=200ms`) {
		t.Errorf("logs lack the warning:\n%s", logs.String())
	}
}
