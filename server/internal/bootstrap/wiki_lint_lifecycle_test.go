package bootstrap

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

type lintLifecycle struct {
	f         *lintRuntimeFixture
	app       *app
	parent    context.Context
	started   chan struct{}
	cleaned   chan struct{}
	done      <-chan lintCallResult
	stop      *lintStopContext
	poolProbe chan error
}

// The real router seeds accounts, a workspace and the wiki without app.run.
// This makes early startup failures and construct-close test the same actual
// runtime, not a worker which only existed after a different successful run.
func newLintLifecycle(t *testing.T, databaseURL string, files fs.FS, change func(*config.Config), probePool bool, holdCleanup <-chan struct{}) *lintLifecycle {
	t.Helper()
	l := &lintLifecycle{started: make(chan struct{}), cleaned: make(chan struct{}), poolProbe: make(chan error, 1)}
	parent, cancelParent := context.WithCancel(context.Background())
	l.parent = parent
	t.Cleanup(cancelParent)
	f, w := newLintRuntimeWiring(t, func(d *wiki.Deps) {
		d.Rules = lintAfterRules{Rules: d.Rules, after: func(ctx context.Context) error {
			close(l.started)
			<-ctx.Done()
			if holdCleanup != nil {
				<-holdCleanup
			}
			return ctx.Err()
		}}
		d.Snapshots = lintLifecycleSnapshot{Snapshots: d.Snapshots, cleaned: l.cleaned, after: func() {
			if probePool {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				l.poolProbe <- l.app.pool.Ping(ctx)
			}
		}}
	})
	l.f = f
	cfg := testConfig(t, databaseURL, false)
	cfg.Database.MaxConns = 6
	if change != nil {
		change(&cfg)
	}
	a, err := wireApp(parent, cfg, slog.New(slog.NewJSONHandler(&f.logs, nil)), files, testWebUI(), w)
	if err != nil {
		t.Fatal(err)
	}
	l.app = a
	l.stop = &lintStopContext{real: a.lintRuns}
	a.lintRuns = l.stop
	t.Cleanup(a.close)
	server := httptest.NewServer(a.router)
	t.Cleanup(server.Close)
	f.tm = acmeTeam{url: databaseURL, base: server.URL, storage: cfg.Storage.Dir,
		pool: connect(t, databaseURL), contract: apitest.Load(t), tokens: map[string]string{}}
	f.tm.tokens["alice"] = registerAccount(t, f.tm.contract, server.URL, "alice@example.com").AccessToken
	if status, body := ask(t, f.tm.contract, http.MethodPost, server.URL+"/api/v0/workspaces", f.tm.tokens["alice"],
		`{"name":"Acme","slug":"acme"}`); status != http.StatusCreated {
		t.Fatalf("seed actual workspace=%d %s", status, body)
	}
	nb, _, _ := f.seedPending(t)
	server.Close()
	l.done = f.start(f.actor(t, "alice"), wiki.LintScope{NotebookID: uuid.MustParse(nb.id)}, wiki.LintQuery{})
	select {
	case <-l.started:
	case <-time.After(interleavingWait):
		t.Fatal("constructed app's real Rules.Prepare never reached its worker barrier")
	}
	queries, _ := f.trace.copy()
	assetRead := false
	for _, query := range queries {
		assetRead = assetRead || query.Name == "AttachmentInfos"
	}
	if !assetRead || f.pool.Stat().AcquiredConns() != 1 || parent.Err() != nil {
		t.Fatal("startup fixture lacks its actual pending-asset transaction and live constructor context")
	}
	return l
}

type lintLifecycleSnapshot struct {
	shared.Snapshots
	cleaned chan struct{}
	after   func()
}

func (s lintLifecycleSnapshot) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	defer close(s.cleaned)
	defer s.after()
	return s.Snapshots.WithinSnapshot(ctx, fn)
}

type lintStopObservation struct {
	Canceled  bool
	Bounded   bool
	Remaining time.Duration
}

type lintStopContext struct {
	real lintStopper
	mu   sync.Mutex
	seen []lintStopObservation
}

func (s *lintStopContext) Stop(ctx context.Context) error {
	deadline, bounded := ctx.Deadline()
	s.mu.Lock()
	s.seen = append(s.seen, lintStopObservation{Canceled: ctx.Err() != nil, Bounded: bounded, Remaining: time.Until(deadline)})
	s.mu.Unlock()
	return s.real.Stop(ctx)
}

func (l *lintLifecycle) assertStopped(t *testing.T, probePool bool) {
	t.Helper()
	if l.parent.Err() != nil {
		t.Fatal("constructor parent was cancelled; it cannot prove app.run/close ownership")
	}
	select {
	case <-l.cleaned:
	default:
		t.Fatal("app returned before its actual lint transaction cleanup and connection Release")
	}
	got := lintReceive(t, l.done)
	if got.err == nil || len(got.result.Rules) != 0 {
		t.Fatalf("stopped worker delivered success/partial result: %+v %v", got.result, got.err)
	}
	if l.f.pool.Stat().AcquiredConns() != 0 {
		t.Fatal("app returned with the lint snapshot connection still acquired")
	}
	if probePool {
		if err := <-l.poolProbe; err != nil {
			t.Fatalf("database pool closed before the worker's real snapshot cleanup: %v", err)
		}
	}
	l.stop.mu.Lock()
	seen := append([]lintStopObservation(nil), l.stop.seen...)
	l.stop.mu.Unlock()
	want := l.app.cfg.Database.CommitTimeout + l.app.poolCloseTimeout
	if len(seen) == 0 || seen[0].Canceled || !seen[0].Bounded || seen[0].Remaining <= 0 || seen[0].Remaining > want {
		t.Fatalf("Stop received no independent finite cleanup context: %+v, bound=%s", seen, want)
	}
	t.Logf("actual worker drained before app exit; constructor ctx live; independent Stop remaining=%s (bound=%s)", seen[0].Remaining, want)
}

func TestTheLintStopsWhenAppRunEnds(t *testing.T) {
	t.Run("run-exits", func(t *testing.T) {
		for _, name := range []string{"cancel", "listen-fails", "jobs-start-fails", "pre-cancelled", "ping-fails", "migration-fails", "database-check-fails"} {
			t.Run(name, func(t *testing.T) {
				url := ""
				files := migrations.FS()
				var change func(*config.Config)
				wantError := ""
				switch name {
				case "jobs-start-fails":
					roles := newSplitRoles(t)
					url = roles.serverURL
					if _, err := roles.owner.Exec(t.Context(), "REVOKE ALL ON river_queue FROM nervewiki_runtime"); err != nil {
						t.Fatal(err)
					}
					wantError = "permission denied for table river_queue"
				case "database-check-fails":
					url = pgtest.NewEmptyDatabaseWith(t, ctypeC)
					m, err := postgres.NewMigrator(connect(t, url), migrations.FS())
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = m.Close() })
					if _, err := m.Up(t.Context()); err != nil {
						t.Fatal(err)
					}
					wantError = "is not set up for nervewiki"
				default:
					url = pgtest.NewDatabase(t)
				}
				switch name {
				case "listen-fails":
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = listener.Close() })
					change = func(cfg *config.Config) { cfg.Server.Addr = listener.Addr().String() }
					wantError = "address already in use"
				case "pre-cancelled", "ping-fails":
					wantError = "database unreachable"
				case "migration-fails":
					files = lintFailingMigration(t)
					change = func(cfg *config.Config) { cfg.Database.AutoMigrate = true }
					wantError = "division by zero"
				}
				probePool := name != "ping-fails"
				l := newLintLifecycle(t, url, files, change, probePool, nil)
				var held []*pgxpool.Conn
				if name == "ping-fails" {
					l.app.databaseWait = 100 * time.Millisecond
					for range int(l.app.cfg.Database.MaxConns) {
						conn, err := l.app.pool.Acquire(t.Context())
						if err != nil {
							t.Fatal(err)
						}
						held = append(held, conn)
					}
					defer func() {
						for _, conn := range held {
							conn.Release()
						}
					}()
				}
				runCtx, cancelRun := context.WithCancel(context.Background())
				defer cancelRun()
				if name == "pre-cancelled" {
					cancelRun()
				}
				done := make(chan error, 1)
				go func() { done <- l.app.run(runCtx) }()
				if name == "cancel" {
					waitHealthy(t, l.app.cfg.Server.AddrFile, done)
					cancelRun()
				}
				var err error
				select {
				case err = <-done:
				case <-time.After(interleavingWait):
					t.Fatal("actual app.run exit did not return")
				}
				if wantError == "" && err != nil || wantError != "" && (err == nil || !strings.Contains(err.Error(), wantError)) {
					t.Fatalf("actual %s exit=%v, want %q", name, err, wantError)
				}
				if name == "jobs-start-fails" && !strings.Contains(err.Error(), "start the jobs:") {
					t.Fatalf("failure did not reach actual jobs.Start: %v", err)
				}
				if name == "migration-fails" {
					var coded interface{ SQLState() string }
					if !strings.Contains(err.Error(), "type:sql,version:99999") || !errors.As(err, &coded) || coded.SQLState() != "22012" {
						t.Fatalf("failure did not reach the actual added migration's division: %v", err)
					}
				}
				if name == "database-check-fails" && !strings.Contains(err.Error(), "LC_CTYPE") {
					t.Fatalf("failure did not reach actual locale checks: %v", err)
				}
				t.Logf("actual run exit %s: %v", name, err)
				l.assertStopped(t, probePool)
				// close reuses the already drained owner. It is intentionally
				// outside the assertion of what run itself accomplished.
				for _, conn := range held {
					conn.Release()
				}
				held = nil
				l.app.close()
			})
		}
	})
	t.Run("construct-close", func(t *testing.T) {
		l := newLintLifecycle(t, pgtest.NewDatabase(t), migrations.FS(), nil, true, nil)
		l.app.close()
		l.assertStopped(t, true)
		l.app.close()
	})
	t.Run("stop-timeout", func(t *testing.T) {
		release := make(chan struct{})
		var once sync.Once
		defer once.Do(func() { close(release) })
		l := newLintLifecycle(t, pgtest.NewDatabase(t), migrations.FS(), nil, false, release)
		l.app.cfg.Database.CommitTimeout = 25 * time.Millisecond
		l.app.poolCloseTimeout = 25 * time.Millisecond
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		start := time.Now()
		err := l.app.run(ctx)
		if !errors.Is(err, context.Canceled) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("run did not join its real startup failure and bounded Stop timeout: %v", err)
		}
		if time.Since(start) > time.Second || l.parent.Err() != nil {
			t.Fatal("Stop ignored its independent cleanup bound or cancelled the constructor context")
		}
		select {
		case <-l.cleaned:
			t.Fatal("Stop timeout falsely claimed the held worker cleanup finished")
		default:
		}
		conn, err := l.app.pool.Acquire(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		start = time.Now()
		l.app.close()
		if elapsed := time.Since(start); elapsed < 50*time.Millisecond || elapsed > time.Second {
			t.Fatalf("close ignored bounded Stop + pool cleanup: %s", elapsed)
		}
		conn.Release()
		if !strings.Contains(l.f.logs.String(), "lint runtime did not stop before database cleanup") ||
			!strings.Contains(l.f.logs.String(), "database pool not closed: connections still in use") {
			t.Fatal("bounded close did not preserve both actual cleanup warnings")
		}
		once.Do(func() { close(release) })
		if err := l.f.module.Stop(t.Context()); err != nil {
			t.Fatal(err)
		}
		if l.f.pool.Stat().AcquiredConns() != 0 {
			t.Fatal("late worker cleanup failed to release its actual transaction")
		}
	})
}

func lintFailingMigration(t *testing.T) fs.FS {
	t.Helper()
	source := migrations.FS()
	files := fstest.MapFS{}
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := fs.ReadFile(source, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = &fstest.MapFile{Data: data}
	}
	files["99999_lint_runtime_failure.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1 / 0;\n-- +goose Down\nSELECT 1;\n")}
	return files
}
