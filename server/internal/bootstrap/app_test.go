package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// testWebUI is a built frontend: its index.html and one hashed asset.
func testWebUI() fstest.MapFS {
	return fstest.MapFS{
		"index.html":         {Data: []byte("<!doctype html><title>Nerve Wiki</title>")},
		"assets/index-a1.js": {Data: []byte("export {};")},
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
			WriteTimeout:      10 * time.Second,
			ShutdownTimeout:   5 * time.Second,
			RequestTimeout:    4 * time.Second,
			MaxBodyBytes:      1 << 20,
		},
		Database: config.DatabaseConfig{URL: dbURL, MaxConns: 4, AutoMigrate: autoMigrate, CommitTimeout: 2 * time.Second},
		Auth: config.AuthConfig{
			SignupEnabled:   true,
			AccessTokenTTL:  15 * time.Minute,
			SessionTTL:      720 * time.Hour,
			RefreshDeadline: 3 * time.Second,
			// The runs of the cleanup the tests look for start with the jobs.
			SessionCleanupInterval: time.Hour,
			Password: config.PasswordConfig{
				Argon2MemoryKiB: 64, Argon2Iterations: 1, Argon2Parallelism: 1, MaxConcurrentHashes: 4, MaxWait: 2 * time.Second,
			},
		},
		RateLimit: roomyLimits(),
		Workspace: config.WorkspaceConfig{CreationEnabled: true},
		// So does the edit sessions' cleanup's. Unset, its interval of 0
		// would have River enqueue it without pause: nothing validates
		// this configuration.
		Page:   config.PageConfig{EditSessionCleanupInterval: time.Hour, ParseBudgetBytes: 8 << 20, ParseMaxWait: 2 * time.Second},
		Events: config.EventsConfig{HeartbeatInterval: 20 * time.Second},
		// The purge's first run starts with the jobs too.
		Jobs:    config.JobsConfig{ShutdownTimeout: 5 * time.Second, PurgeInterval: time.Hour, PurgeRetention: 1440 * time.Hour, ExportWorkers: 1},
		Storage: config.StorageConfig{Dir: t.TempDir()},
		Asset:   config.AssetConfig{MaxBytes: 50 << 20, UploadMinRate: 64 << 10},
		// The exports' expiry and sweep start with the jobs, the rescue
		// before them.
		Transfer: config.TransferConfig{ExportTTL: 24 * time.Hour, JobTimeout: 6 * time.Hour, HeartbeatTimeout: 5 * time.Minute, MaxQueued: 20},
		Log:      config.LogConfig{Level: "error", Format: "text"},
	}
}

// roomyLimits are buckets that the tests of this package never empty.
func roomyLimits() config.RateLimitConfig {
	roomy := config.BucketConfig{PerMinute: 600000, Burst: 100000}
	return config.RateLimitConfig{
		IPv6PrefixLen: 64, Anonymous: roomy, AuthFailure: roomy, Authenticated: roomy,
		LoginIP: roomy, LoginIPEmail: roomy, RegisterIP: roomy, PasswordUser: roomy, AssetContent: roomy,
	}
}

// buildApp wires the app and closes it when the test ends.
func buildApp(t *testing.T, cfg config.Config, migrations fs.FS) *app {
	t.Helper()
	return buildAppWith(t, cfg, migrations, slog.New(slog.DiscardHandler))
}

// buildAppWith is buildApp logging to logger.
func buildAppWith(t *testing.T, cfg config.Config, migrations fs.FS, logger *slog.Logger) *app {
	t.Helper()
	a, err := newApp(context.Background(), cfg, logger, migrations, testWebUI())
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
	return runApp(t, buildApp(t, cfg, migrations))
}

// runApp is startApp for an app the test built.
func runApp(t *testing.T, a *app) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	// run's result goes to both: waitHealthy takes it from done when the
	// server stops early, and the cleanup still finds it in stopped.
	done, stopped := make(chan error, 1), make(chan error, 1)
	go func() {
		err := a.run(ctx)
		done <- err
		stopped <- err
	}()
	// Cleanups run last-in first-out: run stops before buildApp's close.
	t.Cleanup(func() {
		cancel()
		if err := <-stopped; err != nil {
			t.Errorf("run() = %v, want nil after cancel", err)
		}
	})
	return waitHealthy(t, a.cfg.Server.AddrFile, done)
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
// it, answers /healthz and /readyz, runs the background jobs, and stops in
// order when cancelled: HTTP, the jobs, then the pool.
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
	logs.waitFor(t, `msg="jobs started"`, 10*time.Second)
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
		`msg="event listener stopped"`,
		`msg="jobs stopped"`,
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

// silentServer accepts TCP connections and never answers, as a database
// host behind a firewall that drops packets looks to a client.
func silentServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var conns []net.Conn
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})
	return "postgres://nobody@" + ln.Addr().String() + "/nowhere?sslmode=disable"
}

// Startup gives up on a database that does not answer after databaseWait,
// rather than when the operating system drops the connection.
func TestRunGivesUpOnASilentDatabase(t *testing.T) {
	a := buildApp(t, testConfig(t, silentServer(t), false), sampleMigrations())
	a.databaseWait = 200 * time.Millisecond
	begin := time.Now()

	err := a.run(context.Background())

	if err == nil || !strings.HasPrefix(err.Error(), "database unreachable (waited up to 200ms): ") {
		t.Errorf("run() = %v, want the database to be reported unreachable", err)
	}
	if elapsed := time.Since(begin); elapsed > 2*time.Second {
		t.Errorf("run() took %s, want about the 200ms wait", elapsed)
	}
}

// Without auto_migrate, serve still needs the database at startup: the
// database check runs before it listens.
func TestRunRefusesAnUnreachableDatabase(t *testing.T) {
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())

	err := a.run(context.Background())

	if err == nil || !strings.HasPrefix(err.Error(), "database unreachable") {
		t.Errorf("run() = %v, want the database to be reported unreachable", err)
	}
	if _, statErr := os.Stat(a.cfg.Server.AddrFile); statErr == nil {
		t.Error("run() listened without a database")
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
	a, err := newApp(context.Background(), cfg, slog.New(slog.NewTextHandler(&logs, nil)), sampleMigrations(), testWebUI())
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
	a, err := newApp(context.Background(), testConfig(t, pgtest.NewDatabase(t), false), slog.New(slog.NewTextHandler(&logs, nil)), sampleMigrations(), testWebUI())
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
