package bootstrap

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// connect opens a pool of its own on url for the test's own statements.
func connect(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: url, MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// count runs a count query on pool.
func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := client().Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// With database.auto_migrate off, serve starts on a database behind the
// migrations and waits, not ready (M0); the background jobs wait with it,
// since River needs its tables, and start once the operator has applied the
// migrations, without a restart (M1/P4 design 3.4).
func TestJobsStartOnceTheMigrationsAreApplied(t *testing.T) {
	url := pgtest.NewEmptyDatabase(t)
	var logs syncBuffer
	a := buildAppWith(t, testConfig(t, url, false), migrations.FS(), slog.New(slog.NewTextHandler(&logs, nil)))
	a.migrationPoll = 50 * time.Millisecond
	base := runApp(t, a)

	logs.waitFor(t, `msg="background jobs wait for the pending migrations"`, 10*time.Second)
	time.Sleep(200 * time.Millisecond) // a few polls
	if status := getStatus(t, base+"/readyz"); status != http.StatusServiceUnavailable || strings.Contains(logs.String(), `msg="jobs started"`) {
		t.Fatalf("GET /readyz = %d with the jobs started: %v; want 503 and the jobs waiting:\n%s",
			status, strings.Contains(logs.String(), `msg="jobs started"`), logs.String())
	}

	m, err := postgres.NewMigrator(connect(t, url), migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	if _, err := m.Up(context.Background()); err != nil {
		t.Fatal(err)
	}

	logs.waitFor(t, `msg="jobs started"`, 10*time.Second)
	if status := getStatus(t, base+"/readyz"); status != http.StatusOK {
		t.Errorf("GET /readyz = %d after the migrations, want 200", status)
	}
}

// The session cleanup runs when serve starts and then every
// auth.session_cleanup_interval: with a second, a session that expires
// after the first run is gone within a few seconds, while a live one stays.
func TestTheSessionCleanupRunsAsConfigured(t *testing.T) {
	url := pgtest.NewDatabase(t)
	cfg := testConfig(t, url, false)
	cfg.Auth.SessionCleanupInterval = time.Second
	base := startApp(t, cfg, migrations.FS())
	pool := connect(t, url)
	tokens := registerAccount(t, apitest.Load(t), base, "alice@example.com")
	const completed = "SELECT count(*) FROM river_job WHERE kind = 'identity.cleanup_expired_sessions' AND state = 'completed'"
	for deadline := time.Now().Add(15 * time.Second); count(t, pool, completed) == 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("no cleanup completed within 15s of the start")
		}
	}

	if _, err := pool.Exec(context.Background(), `
		INSERT INTO auth_sessions (id, user_id, token_hash, expires_at, created_at, updated_at)
		SELECT gen_random_uuid(), id, sha256('expired'), now() - interval '1 second', now() - interval '1 hour', now() - interval '1 hour'
		FROM users`); err != nil {
		t.Fatal(err)
	}
	const expired = "SELECT count(*) FROM auth_sessions WHERE expires_at < now()"
	for deadline := time.Now().Add(10 * time.Second); count(t, pool, expired) > 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the expired session is still there 10s later, want it deleted by a run every second")
		}
	}
	if res, body := sendRequest(t, newRequest(t, http.MethodGet, base+"/api/v0/me", tokens.AccessToken, nil)); res.StatusCode != http.StatusOK {
		t.Errorf("GET /me with the live session = %d %s, want 200", res.StatusCode, body)
	}
}

// Stopping serve stops HTTP first: a request in flight finishes, and only
// then do the jobs stop, which it might still need (M1/P4 design 3.4). A
// sign-in waits on the account row that the test holds while serve is
// cancelled; the jobs are still running when the test lets it go.
func TestRunStopsTheJobsAfterHTTP(t *testing.T) {
	url := pgtest.NewDatabase(t)
	var logs syncBuffer
	a := buildAppWith(t, testConfig(t, url, false), migrations.FS(), slog.New(slog.NewTextHandler(&logs, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.run(ctx) }()
	base := waitHealthy(t, a.cfg.Server.AddrFile, done)
	registerAccount(t, apitest.Load(t), base, "alice@example.com")
	logs.waitFor(t, `msg="jobs started"`, 10*time.Second)

	pool := connect(t, url)
	holder, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err := holder.Exec(context.Background(), "SELECT 1 FROM users FOR NO KEY UPDATE"); err != nil {
		t.Fatal(err)
	}
	signedIn := make(chan int, 1)
	go func() {
		body := bytes.NewReader([]byte(`{"email":"alice@example.com","password":"Tr0ub4dor&3"}`))
		res, err := client().Post(base+"/api/v0/auth/login", "application/json", body)
		if err != nil {
			signedIn <- 0
			return
		}
		_ = res.Body.Close()
		signedIn <- res.StatusCode
	}()
	// River may wait on locks of its own: count the waits on users only.
	pgtest.WaitForLockWaitsOn(t, pool, "users", 1, 10*time.Second)
	cancel()
	logs.waitFor(t, `msg="http server shutting down"`, 5*time.Second)
	time.Sleep(time.Second) // the jobs would stop meanwhile if they did not wait for HTTP
	stoppedEarly := strings.Contains(logs.String(), `msg="jobs stopped"`)
	if err := holder.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}

	if status := <-signedIn; status != http.StatusOK || stoppedEarly {
		t.Errorf("the sign-in in flight = %d, the jobs stopped before it: %v; want 200, the jobs still running", status, stoppedEarly)
	}
	if err := <-done; err != nil {
		t.Fatalf("run() = %v, want nil", err)
	}
	out := logs.String()
	if httpAt, jobsAt := strings.Index(out, `msg="http server stopped"`), strings.Index(out, `msg="jobs stopped"`); httpAt < 0 || jobsAt < httpAt {
		t.Errorf("logs have the HTTP stop at %d and the jobs' at %d, want the jobs after:\n%s", httpAt, jobsAt, out)
	}
}
