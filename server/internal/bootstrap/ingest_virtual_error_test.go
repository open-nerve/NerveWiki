package bootstrap

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type ingestSourceQuery struct {
	SQL             string
	PID             uint32
	Snapshot        uint64
	Args            []any
	Err, ContextErr error
	TxStatus        byte
}
type ingestSourceQueryKey struct{}
type ingestSourceSnapshot struct {
	PID                                            uint32
	Snapshot                                       uint64
	Callback, Outer, CallbackContext, OuterContext error
}

// The tracer only observes the real SQL and cleanup. It never returns an
// injected error or changes the rows. The actual server cancels its waiter.
type ingestSourceTrace struct {
	mu        sync.Mutex
	active    bool
	started   chan ingestSourceQuery
	queries   []ingestSourceQuery
	snapshots []ingestSourceSnapshot
}

func (s *ingestSourceTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return ctx
	}
	q := ingestSourceQuery{SQL: d.SQL, PID: conn.PgConn().PID()}
	q.Snapshot, _ = ctx.Value(lintSnapshotKey{}).(uint64)
	for _, a := range d.Args {
		if _, mode := a.(pgx.QueryExecMode); mode {
			continue
		}
		if ids, ok := a.([]uuid.UUID); ok {
			a = slices.Clone(ids)
		}
		q.Args = append(q.Args, a)
	}
	if strings.HasPrefix(d.SQL, "-- name: FirstIngestSources ") {
		select {
		case s.started <- q:
		default:
		}
	}
	return context.WithValue(ctx, ingestSourceQueryKey{}, q)
}

func (s *ingestSourceTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryEndData) {
	q, ok := ctx.Value(ingestSourceQueryKey{}).(ingestSourceQuery)
	if !ok {
		return
	}
	q.Err, q.ContextErr, q.TxStatus = d.Err, ctx.Err(), conn.PgConn().TxStatus()
	s.mu.Lock()
	s.queries = append(s.queries, q)
	s.mu.Unlock()
}

type ingestSourceSnapshots struct {
	shared.Snapshots
	trace *ingestSourceTrace
}

func (s ingestSourceSnapshots) WithinSnapshot(ctx context.Context, fn func(context.Context) error) error {
	s.trace.mu.Lock()
	active := s.trace.active
	s.trace.mu.Unlock()
	if !active {
		return s.Snapshots.WithinSnapshot(ctx, fn)
	}
	var seen ingestSourceSnapshot
	err := s.Snapshots.WithinSnapshot(ctx, func(in context.Context) error {
		seen.Snapshot, _ = in.Value(lintSnapshotKey{}).(uint64)
		if tx, ok := postgres.TxFrom(in); ok {
			seen.PID = tx.Conn().PgConn().PID()
		}
		seen.Callback = fn(in)
		seen.CallbackContext = in.Err()
		return seen.Callback
	})
	seen.Outer, seen.OuterContext = err, ctx.Err()
	s.trace.mu.Lock()
	s.trace.snapshots = append(s.trace.snapshots, seen)
	s.trace.mu.Unlock()
	return err
}

func ingestSourcePGCause(t *testing.T, err error) *pgconn.PgError {
	t.Helper()
	var cause *pgconn.PgError
	if !errors.As(err, &cause) || cause.Code != "57014" {
		t.Fatalf("actual source PG cause=%v, want original 57014", err)
	}
	return cause
}

func (s *ingestSourceTrace) check(t *testing.T, waited ingestSourceQuery) {
	t.Helper()
	s.mu.Lock()
	s.active = false
	queries, snapshots := slices.Clone(s.queries), slices.Clone(s.snapshots)
	s.mu.Unlock()
	var failed *pgconn.PgError
	begins, sources, rollbacks, commits := 0, 0, 0, 0
	for _, q := range queries {
		if q.PID != waited.PID || q.Snapshot != waited.Snapshot {
			continue
		}
		switch {
		case strings.HasPrefix(q.SQL, "-- name: FirstIngestSources "):
			sources++
			failed = ingestSourcePGCause(t, q.Err)
			if q.ContextErr != nil || q.TxStatus != 'E' {
				t.Fatalf("PG cause was timeout/closed transaction: %+v", q)
			}
		case strings.HasPrefix(q.SQL, "begin"):
			begins++
			if q.Err != nil || !strings.Contains(q.SQL, "repeatable read") || !strings.Contains(q.SQL, "read only") {
				t.Fatalf("source read lost original RR: %+v", q)
			}
		case q.SQL == "rollback":
			rollbacks++
			if q.Err != nil || q.ContextErr != nil || q.TxStatus != 'I' {
				t.Fatalf("source cleanup did not restore the actual connection: %+v", q)
			}
		case q.SQL == "commit":
			commits++
		}
	}
	if begins != 1 || sources != 1 || rollbacks != 1 || commits != 0 || len(snapshots) != 1 {
		t.Fatalf("source lifecycle begin/query/rollback/commit/snapshot=%d/%d/%d/%d/%d: %+v", begins, sources, rollbacks, commits, len(snapshots), queries)
	}
	seen := snapshots[0]
	if seen.PID != waited.PID || seen.Snapshot != waited.Snapshot || seen.CallbackContext != nil || seen.OuterContext != nil ||
		ingestSourcePGCause(t, seen.Callback) != failed || ingestSourcePGCause(t, seen.Outer) != failed {
		t.Fatalf("source original error was lost across its snapshot: %+v", seen)
	}
}

// Swallowing FirstIngestSources' error would publish an incomplete log. Both
// consumers must instead preserve the server's cause, roll back the same RR,
// and recover with the five literal slots after the owned blocker is released.
func TestIngestVirtualLogSourceError(t *testing.T) {
	for _, consumer := range []string{"read", "export"} {
		t.Run(consumer, func(t *testing.T) {
			trace := &ingestSourceTrace{started: make(chan ingestSourceQuery, 1)}
			load := newVirtualExportLoadConfigured(t, "SourceErrorWiki", nil, trace, nil)
			load.read.Snapshots = ingestSourceSnapshots{load.read.Snapshots, trace}
			load.export.Snapshots = ingestSourceSnapshots{load.export.Snapshots, trace}
			awaitJob(t, load.tm.pool, jobs.PurgeKind)
			f := ingestVirtualSources(t, load.tm, load.book, false)
			f.produce(t)
			f.read(t, false)
			f.zip(t, "SourceErrorWiki", false)

			hold, err := f.tm.pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = hold.Rollback(context.Background()) }()
			if _, err := hold.Exec(t.Context(), "LOCK TABLE changeset_sources IN ACCESS EXCLUSIVE MODE"); err != nil {
				t.Fatal(err)
			}
			blocker := hold.Conn().PgConn().PID()
			trace.mu.Lock()
			trace.active = true
			trace.mu.Unlock()
			type outcome struct {
				read mcpAnswer
				job  transferJob
			}
			done := make(chan outcome, 1)
			go func() {
				defer close(done)
				var got outcome
				if consumer == "read" {
					got.read = f.m.use(t, "read", map[string]any{"path": "log.md"})
				} else {
					j := f.tm.startExport(t, "alice", f.book.id, "")
					got.job = f.tm.endedJob(t, "alice", j.ID)
				}
				done <- got
			}()
			var waited ingestSourceQuery
			select {
			case waited = <-trace.started:
			case got := <-done:
				t.Fatalf("consumer ended before the actual source query: %+v", got)
			case <-time.After(interleavingWait):
				t.Fatal("consumer never reached FirstIngestSources")
			}
			if waited.PID == 0 || waited.PID == blocker || waited.Snapshot == 0 || len(waited.Args) != 2 || waited.Args[1] != int32(5) {
				t.Fatalf("unbound source waiter: %+v", waited)
			}
			ids, ok := waited.Args[0].([]uuid.UUID)
			if !ok || !slices.Contains(ids, uuid.MustParse(f.cs)) {
				t.Fatalf("actual source preview omitted producer %s: %#v", f.cs, waited.Args)
			}
			pgtest.WaitForTableLockWaits(t, f.tm.pool, "changeset_sources", 1, interleavingWait)
			var blocked bool
			if err := f.tm.pool.QueryRow(t.Context(), `SELECT EXISTS (
 SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid=a.pid
 WHERE a.datname=current_database() AND a.pid=$1 AND a.state='active'
 AND a.wait_event_type='Lock' AND l.locktype='relation' AND NOT l.granted
 AND l.mode='AccessShareLock' AND l.relation='changeset_sources'::regclass
 AND $2::integer=ANY(pg_blocking_pids(a.pid)))`, waited.PID, blocker).Scan(&blocked); err != nil || !blocked {
				t.Fatalf("owned PID %d is not blocked by %d on its source query: %t %v", waited.PID, blocker, blocked, err)
			}
			var cancelled bool
			if err := f.tm.pool.QueryRow(t.Context(), "SELECT pg_cancel_backend($1)", waited.PID).Scan(&cancelled); err != nil || !cancelled {
				t.Fatalf("cancel owned blocked PID%d: %t %v", waited.PID, cancelled, err)
			}
			var got outcome
			select {
			case value, ok := <-done:
				if !ok {
					t.Fatal("consumer helper exited without a result")
				}
				got = value
			case <-time.After(interleavingWait):
				t.Fatal("cancelled source consumer did not finish")
			}
			trace.check(t, waited)
			if err := hold.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
			if consumer == "read" {
				member := got.read.toolError(t)
				requestID, _ := member["request_id"].(string)
				result := got.read.result(t)
				blocks, _ := result["content"].([]any)
				structured, _ := result["structuredContent"].(map[string]any)
				if member["code"] != "internal_error" || requestID == "" || len(blocks) != 1 || len(structured) != 1 || strings.Contains(string(got.read.body), "# Log") {
					t.Fatalf("source failure published a partial read: %s", got.read.body)
				}
			} else if j := got.job; j.State != "failed" || j.Report == nil || j.Report.Failure == nil || *j.Report.Failure != "internal" || j.Download != nil {
				t.Fatalf("source failure published an archive or became a timeout: %+v", j)
			}
			f.marks(t)
			if consumer == "read" {
				f.read(t, false)
			} else {
				f.zip(t, "SourceErrorWiki", false)
			}
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
}
