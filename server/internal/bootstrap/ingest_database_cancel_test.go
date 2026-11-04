// This exercises a real PG statement inside the coverage Heads port, following
// the existing lint port's real-query cancellation pattern. It does not claim
// that the generated IngestHeadsFirst statement itself was blocked.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const (
	c10DraftSpace int32 = 0x433130
	c10DraftKey   int32 = 1
	c10DraftGate        = "/* C10 real coverage PG gate */ SELECT pg_advisory_xact_lock($1::integer, $2::integer)"
	c10DraftBody        = "# Digest\n\n[[raw/Source]]\n"
)

// Keep the healthy control and two independently selectable cancellation legs.
func TestIngestCoverageDatabaseCancel(t *testing.T) {
	for _, leg := range []string{"healthy", "statement_timeout", "context_cancel"} {
		t.Run(leg, func(t *testing.T) { c10DraftLeg(t, leg) })
	}
}

type c10DraftRunKey struct{}
type c10DraftSQLKey struct{}

type c10DraftEvent struct {
	Kind, SQL                                   string
	PID                                         uint32
	Err, ContextErr                             error
	AfterFailure, HasDeadline, ConnectionClosed bool
}

type c10DraftState struct {
	mu                                               sync.Mutex
	armed                                            atomic.Bool
	done                                             chan struct{}
	leg                                              string
	pool                                             *pgxpool.Pool // the real app pool, not the independent fixture pool
	output, source, notebook                         string
	cs                                               uuid.UUID
	pid                                              uint32
	xid                                              string
	callbackErr, managerErr, coverageErr, fixtureErr error
	prewrite                                         bool
	events                                           []c10DraftEvent
}

func (s *c10DraftState) note(kind string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, c10DraftEvent{Kind: kind, Err: err, AfterFailure: s.coverageErr != nil})
}

func c10DraftActive(ctx context.Context, s *c10DraftState) bool {
	return ctx.Value(c10DraftRunKey{}) == s
}

// This wrapper only observes the original manager; it never begins, commits,
// rolls back, joins causes, or changes the callback's error.
type c10DraftTx struct {
	page.Tx
	s *c10DraftState
}

func (w c10DraftTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if !w.s.armed.CompareAndSwap(true, false) {
		return w.Tx.WithinTx(ctx, fn)
	}
	s := w.s
	if w.InTx(ctx) {
		s.fixtureErr = fmt.Errorf("C10 fixture unexpectedly entered an existing transaction")
	}
	ctx = context.WithValue(ctx, c10DraftRunKey{}, s)
	err := w.Tx.WithinTx(ctx, func(ctx context.Context) error {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			s.fixtureErr = fmt.Errorf("C10 callback has no real PostgreSQL transaction")
		} else {
			s.pid = tx.Conn().PgConn().PID()
		}
		s.note("callback.enter", nil)
		err := fn(ctx)
		s.callbackErr = err
		s.note("callback.exit", err)
		return err
	})
	s.managerErr = err
	s.note("manager.exit", err)
	close(s.done)
	return err
}

// QueryTracer sees the first end attempt even on a closed connection in pinned
// pgx v5.11.0. See dependency-path.md for the exact scope and source pins.
// No test helper calls Commit/Rollback on the target transaction.
type c10DraftTracer struct{}

func (c10DraftTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	s, _ := ctx.Value(c10DraftRunKey{}).(*c10DraftState)
	if s == nil {
		return ctx
	}
	_, bounded := ctx.Deadline()
	s.mu.Lock()
	s.events = append(s.events, c10DraftEvent{Kind: "sql.start", SQL: d.SQL, PID: conn.PgConn().PID(),
		ContextErr: ctx.Err(), HasDeadline: bounded, ConnectionClosed: conn.IsClosed(), AfterFailure: s.coverageErr != nil})
	s.mu.Unlock()
	return context.WithValue(ctx, c10DraftSQLKey{}, d.SQL)
}

func (c10DraftTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryEndData) {
	s, _ := ctx.Value(c10DraftRunKey{}).(*c10DraftState)
	if s == nil {
		return
	}
	sql, _ := ctx.Value(c10DraftSQLKey{}).(string)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, c10DraftEvent{Kind: "sql.end", SQL: sql, PID: conn.PgConn().PID(), Err: d.Err,
		AfterFailure: s.coverageErr != nil})
}

type c10DraftMetadata struct {
	linking.IngestMetadata
	s *c10DraftState
}

func (m c10DraftMetadata) Heads(ctx context.Context, notebook, root uuid.UUID, after *uuid.UUID, take int) (linking.IngestHeadBatch, error) {
	s := m.s
	if !c10DraftActive(ctx, s) {
		return m.IngestMetadata.Heads(ctx, notebook, root, after, take)
	}
	s.note("coverage.enter", nil)
	if after == nil {
		// Prove actual writes and index updates exist INSIDE this real unit
		// before aborting it. An untouched/empty transaction is not a rollback oracle.
		var content, index, link, revision bool
		err := postgres.DB(ctx, s.pool).QueryRow(ctx, `SELECT (SELECT backend_xid::text FROM pg_stat_activity WHERE pid=pg_backend_pid()),
			EXISTS (SELECT 1 FROM page_contents WHERE node_id=$1 AND revision=2 AND content=$2),
			EXISTS (SELECT 1 FROM indexed_pages WHERE node_id=$1 AND notebook_id=$3 AND revision=2 AND extractor=2),
			EXISTS (SELECT 1 FROM page_links WHERE source_id=$1 AND resolved_id=$4),
			EXISTS (SELECT 1 FROM page_revisions r JOIN changesets c ON c.id=r.changeset_id
				WHERE r.node_id=$1 AND r.revision=2 AND r.content=$2 AND c.id=$5 AND c.notebook_id=$3 AND c.kind='edit')`,
			s.output, c10DraftBody, s.notebook, s.source, s.cs).Scan(&s.xid, &content, &index, &link, &revision)
		s.prewrite = content && index && link && revision
		if err != nil {
			s.fixtureErr = err
			return linking.IngestHeadBatch{}, err
		}
		if !s.prewrite {
			s.fixtureErr = fmt.Errorf("C10 real in-unit content/index/link/revision precondition is false")
			return linking.IngestHeadBatch{}, s.fixtureErr
		}
		if s.leg == "statement_timeout" {
			// Narrow only this transaction, below the unchanged 4s request bound.
			if _, err := postgres.DB(ctx, s.pool).Exec(ctx, "SET LOCAL statement_timeout = '1000ms'"); err != nil {
				s.fixtureErr = err
				return linking.IngestHeadBatch{}, err
			}
		}
		// The lock is held by an independent real transaction. This is a real
		// PG read at the coverage port, not a manufactured PgError or ctx.Err.
		rows, err := postgres.DB(ctx, s.pool).Query(ctx, c10DraftGate, c10DraftSpace, c10DraftKey)
		if err == nil {
			for rows.Next() {
			}
			err = rows.Err()
			rows.Close()
		}
		if err != nil {
			s.mu.Lock()
			s.coverageErr = err
			s.mu.Unlock()
			s.note("coverage.error", err)
			return linking.IngestHeadBatch{}, err // exact driver cause, no test-side Join
		}
	}
	s.note("heads.delegate", nil)
	return m.IngestMetadata.Heads(ctx, notebook, root, after, take)
}

type c10DraftMarks struct {
	linking.IngestMarks
	s *c10DraftState
}

func (m c10DraftMarks) Add(ctx context.Context, notebook, cs uuid.UUID, ids []uuid.UUID) (linking.IngestMarkResult, error) {
	if c10DraftActive(ctx, m.s) {
		m.s.note("marks", nil)
	}
	return m.IngestMarks.Add(ctx, notebook, cs, ids)
}

type c10DraftPublisher struct {
	linking.Publisher
	s *c10DraftState
}

func (p c10DraftPublisher) LinksChanged(ctx context.Context, e linking.LinksChanged) error {
	if c10DraftActive(ctx, p.s) {
		p.s.note("publish", nil)
	}
	return p.Publisher.LinksChanged(ctx, e)
}

type c10DraftObserver struct {
	page.PageObserver
	s *c10DraftState
}

func (o c10DraftObserver) PagesChanged(ctx context.Context, e page.Event) error {
	if c10DraftActive(ctx, o.s) {
		o.s.cs = e.ChangesetID
		o.s.note("page-observer", nil)
	}
	return o.PageObserver.PagesChanged(ctx, e)
}

func c10DraftLeg(t *testing.T, leg string) {
	t.Helper()
	s := &c10DraftState{leg: leg, done: make(chan struct{})}
	wrapped := 0
	tm := newAcmeTeamWiring(t, "", "", wiring{queryTracer: c10DraftTracer{}, page: func(d *page.Deps) {
		s.pool = d.Pool
		d.Tx = c10DraftTx{Tx: d.Tx, s: s}
		for i, original := range d.Observers {
			if index, ok := original.(linkIndex); ok {
				if index.index.Ingest == nil {
					t.Fatal("C10 fixture has no real producer dependency bundle")
				}
				// Preserve the existing observer position and all real dependencies.
				ingest := *index.index.Ingest
				ingest.Metadata = c10DraftMetadata{IngestMetadata: ingest.Metadata, s: s}
				ingest.Marks = c10DraftMarks{IngestMarks: ingest.Marks, s: s}
				index.index.Ingest = &ingest
				index.index.Publisher = c10DraftPublisher{Publisher: index.index.Publisher, s: s}
				original = index
				wrapped++
			}
			d.Observers[i] = c10DraftObserver{PageObserver: original, s: s}
		}
	}})
	if wrapped != 1 {
		t.Fatalf("C10 fixture wrapped %d real producer observers, want 1", wrapped)
	}
	wiki := tm.createWith(t, tm.tokens["alice"], `{"name":"C10 real PG cancellation","mode":"llm_wiki"}`)
	if wiki.Mode != "llm_wiki" || wiki.SourceRootID == nil {
		t.Fatal("C10 fixture lacks source root")
	}
	s.notebook = wiki.ID
	s.source = tm.createPageWith(t, "alice", wiki.ID, *wiki.SourceRootID, "Source", "Independent C10 evidence.\n")
	s.output = tm.createPage(t, "alice", wiki.ID, "", "Digest")
	if c := tm.content(t, "alice", s.output); c.Content != "" || c.Revision != 1 {
		t.Fatalf("C10 fixture output=%+v", c)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
		JOIN indexed_pages i ON i.node_id=n.id WHERE n.id=$1 AND n.parent_id=$2 AND n.deleted_at IS NULL
		AND c.byte_size>0 AND c.revision=1 AND c.deleted_at IS NULL AND i.revision=c.revision AND i.extractor=2`,
		s.source, *wiki.SourceRootID); n != 1 {
		t.Fatalf("C10 fixture source heads=%d, want 1", n)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM changeset_sources"); n != 0 {
		t.Fatalf("C10 fixture marks=%d, want 0", n)
	}
	before := c10DraftImage(t, tm.pool, wiki.ID)
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)

	holder, err := tm.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), interleavingWait)
			defer cancel()
			if err := holder.Rollback(ctx); err != nil {
				t.Errorf("C10 fixture lock-holder cleanup: %v", err)
			}
		})
	}
	t.Cleanup(release)
	holderPID := holder.Conn().PgConn().PID()
	if _, err := holder.Exec(t.Context(), "SELECT pg_advisory_xact_lock($1::integer, $2::integer)", c10DraftSpace, c10DraftKey); err != nil {
		t.Fatal(err)
	}

	write := contentWrite("alice", s.output, c10DraftBody, 1, "")
	requestCtx, cancelRequest := context.WithCancel(t.Context())
	t.Cleanup(cancelRequest)
	req := newRequest(t, write.method, tm.base+write.path, tm.tokens[write.by], []byte(write.body)).WithContext(requestCtx)
	s.armed.Store(true)
	response := make(chan error, 1)
	go func() {
		res, err := client().Do(req)
		if err == nil {
			_, readErr := io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
			if readErr != nil {
				err = readErr
			}
			if leg == "healthy" && res.StatusCode != http.StatusOK {
				err = fmt.Errorf("healthy HTTP status=%d", res.StatusCode)
			}
		}
		response <- err
	}()
	c10DraftWaitForGate(t, tm.pool, s, holderPID)
	switch leg {
	case "healthy":
		release()
	case "context_cancel":
		cancelRequest() // after actual backend lock wait, not a pre-cancelled request
	case "statement_timeout": // PostgreSQL's local statement_timeout ends the wait.
	default:
		t.Fatalf("unknown C10 draft leg %q", leg)
	}
	select {
	case <-s.done:
	case <-time.After(interleavingWait):
		t.Fatal("C10_FIXTURE: real TxManager did not finish; timeout is not a mutation kill")
	}
	release()
	var httpErr error
	select {
	case httpErr = <-response:
	case <-time.After(interleavingWait):
		t.Fatal("C10_FIXTURE: HTTP call did not finish; timeout is not a mutation kill")
	}
	if s.fixtureErr != nil || !s.prewrite || s.cs == (uuid.UUID{}) || s.pid == 0 || s.xid == "" {
		t.Fatalf("C10_FIXTURE: prewrite=%v cs=%s pid=%d xid=%s err=%v", s.prewrite, s.cs, s.pid, s.xid, s.fixtureErr)
	}
	s.mu.Lock()
	events := append([]c10DraftEvent(nil), s.events...)
	s.mu.Unlock()

	if leg == "healthy" {
		if httpErr != nil || s.coverageErr != nil || s.callbackErr != nil || s.managerErr != nil {
			t.Fatalf("C10_HEALTHY: HTTP=%v coverage=%v callback=%v manager=%v", httpErr, s.coverageErr, s.callbackErr, s.managerErr)
		}
		if c := tm.content(t, "alice", s.output); c.Content != c10DraftBody || c.Revision != 2 {
			t.Fatalf("C10_HEALTHY: content=%+v", c)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1 AND seq=1 AND node_id=$2", s.cs, s.source); n != 1 {
			t.Fatalf("C10_HEALTHY: exact mark=%d, want 1", n)
		}
		if n := count(t, tm.pool, "SELECT ingest_count FROM changesets WHERE id=$1", s.cs); n != 1 {
			t.Fatalf("C10_HEALTHY: ingest_count=%d, want 1", n)
		}
	} else {
		// Designated business oracles precede HTTP/final generic transaction errors.
		if leg == "statement_timeout" {
			var original, callback *pgconn.PgError
			if !errors.As(s.coverageErr, &original) || original.Code != "57014" {
				t.Fatalf("C10_REAL_TIMEOUT: coverage=%v, want real PgError 57014", s.coverageErr)
			}
			if !errors.As(s.callbackErr, &callback) || callback != original || !errors.Is(s.callbackErr, s.coverageErr) {
				t.Fatalf("C10_ORIGINAL_CAUSE: callback=%v lost original coverage PgError=%v", s.callbackErr, s.coverageErr)
			}
		} else {
			if !errors.Is(s.coverageErr, context.Canceled) {
				t.Fatalf("C10_REAL_CTX_CANCEL: coverage=%v, want context.Canceled; do not manufacture/join a cause in the test", s.coverageErr)
			}
			if !errors.Is(s.callbackErr, context.Canceled) || !errors.Is(s.callbackErr, s.coverageErr) {
				t.Fatalf("C10_ORIGINAL_CAUSE: callback=%v lost original coverage cancellation=%v", s.callbackErr, s.coverageErr)
			}
		}
		for _, e := range events {
			if !e.AfterFailure {
				continue
			}
			forbidden := e.Kind == "marks" || e.Kind == "publish" || e.Kind == "page-observer"
			if e.Kind == "sql.start" {
				sql := strings.ToLower(strings.TrimSpace(e.SQL))
				forbidden = forbidden || sql == "commit" || strings.Contains(sql, "pg_notify")
			}
			if forbidden {
				t.Fatalf("C10_NO_FOLLOWUP: event after coverage error: %+v", e)
			}
		}
	}
	c10DraftCheckEnd(t, s, events)
	c10DraftWaitForRelease(t, tm.pool, s.pid, s.xid)
	if leg != "healthy" {
		if after := c10DraftImage(t, tm.pool, wiki.ID); after != before {
			t.Fatalf("C10_ROLLBACK: content/index/changeset/revision/marks image changed\nbefore=%s\nafter=%s", before, after)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM changeset_sources WHERE changeset_id=$1", s.cs); n != 0 {
			t.Fatalf("C10_ROLLBACK: orphan marks of rolled back changeset=%d", n)
		}
	}
	// Ask the ORIGINAL app pool for a usable connection; a fresh PID is legal.
	ctx, cancel := context.WithTimeout(t.Context(), interleavingWait)
	defer cancel()
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("C10_POOL_RECOVERY: %v", err)
	}
	defer conn.Release()
	var one int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("C10_POOL_RECOVERY: one=%d err=%v", one, err)
	}
	t.Logf("C10 leg=%s target_pid=%d recovery_pid=%d xid=%s events=%+v", leg, s.pid, conn.Conn().PgConn().PID(), s.xid, events)
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
}

func c10DraftCheckEnd(t *testing.T, s *c10DraftState, events []c10DraftEvent) {
	t.Helper()
	begin, commits, rollbacks, delegates, marks, publishers, nativeHeads, endRecords := 0, 0, 0, 0, 0, 0, 0, 0
	callbackIn, callbackOut, firstEnd, managerOut := -1, -1, -1, -1
	var endErr error
	for i, e := range events {
		switch e.Kind {
		case "callback.enter":
			if callbackIn >= 0 {
				t.Fatal("C10_FIXTURE: multiple callbacks")
			}
			callbackIn = i
		case "callback.exit":
			if callbackOut >= 0 {
				t.Fatal("C10_FIXTURE: multiple callback exits")
			}
			callbackOut = i
		case "manager.exit":
			managerOut = i
		case "heads.delegate":
			delegates++
		case "marks":
			marks++
		case "publish":
			publishers++
		}
		if e.Kind != "sql.start" && e.Kind != "sql.end" {
			continue
		}
		if e.PID != s.pid {
			t.Fatalf("C10_FIXTURE: target transaction changed PID: %+v, want %d", e, s.pid)
		}
		sql := strings.ToLower(strings.TrimSpace(e.SQL))
		if e.Kind == "sql.end" {
			if sql == "commit" || sql == "rollback" {
				endErr = e.Err
				endRecords++
			}
			continue
		}
		if strings.Contains(e.SQL, "-- name: IngestHeadsFirst") {
			nativeHeads++
		}
		switch sql {
		case "begin":
			begin++
			if callbackIn >= 0 {
				t.Fatal("C10_FIXTURE: begin after callback entry")
			}
		case "commit", "rollback":
			if firstEnd != -1 {
				t.Fatal("C10_FIXTURE: more than one end; first-end tracer proof does not cover repeated closed-tx calls")
			}
			firstEnd = i
			if sql == "commit" {
				commits++
			} else {
				rollbacks++
			}
			if e.ContextErr != nil || !e.HasDeadline {
				t.Fatalf("C10_END_CONTEXT: original bounded WithoutCancel contract lost: %+v", e)
			}
		default:
			if strings.HasPrefix(sql, "savepoint") || strings.HasPrefix(sql, "release savepoint") || strings.HasPrefix(sql, "rollback to") {
				t.Fatalf("C10_FIXTURE: unexpected nested/end statement %q", e.SQL)
			}
		}
	}
	if begin != 1 || endRecords != 1 || callbackIn < 0 || callbackOut <= callbackIn || firstEnd <= callbackOut || managerOut <= firstEnd {
		t.Fatalf("C10_END_ORDER: begin=%d callback=%d..%d first_end=%d end_records=%d manager=%d", begin, callbackIn, callbackOut, firstEnd, endRecords, managerOut)
	}
	if s.leg == "healthy" {
		if commits != 1 || rollbacks != 0 || endErr != nil || delegates == 0 || nativeHeads == 0 || marks != 1 || publishers != 1 {
			t.Fatalf("C10_HEALTHY_OBSERVER: commit=%d rollback=%d end=%v delegate=%d nativeHeads=%d marks=%d publish=%d", commits, rollbacks, endErr, delegates, nativeHeads, marks, publishers)
		}
		return
	}
	if commits != 0 || rollbacks != 1 || marks != 0 || publishers != 0 || delegates != 0 {
		t.Fatalf("C10_NO_FOLLOWUP: commit=%d rollback=%d marks=%d publish=%d delegate=%d", commits, rollbacks, marks, publishers, delegates)
	}
	if errors.Is(s.managerErr, pgx.ErrTxCommitRollback) {
		t.Fatalf("C10_ORIGINAL_CAUSE: manager returned ErrTxCommitRollback: %v", s.managerErr)
	}
	if endErr == nil {
		if !errors.Is(s.managerErr, s.coverageErr) {
			t.Fatalf("C10_ORIGINAL_CAUSE: successful rollback lost original cause: %v", s.managerErr)
		}
	} else {
		// Preserve infrastructure-error priority. The original cause has already
		// been checked at coverage/callback; do not require it inside this wrapper.
		if !errors.Is(s.managerErr, endErr) {
			t.Fatalf("C10_INFRA_PRIORITY: manager=%v lacks rollback cause=%v", s.managerErr, endErr)
		}
		if s.leg == "statement_timeout" {
			t.Fatalf("C10_FIXTURE: unexpected timeout-leg rollback failure: %v", endErr)
		}
	}
	if s.leg == "context_cancel" && !errors.Is(s.managerErr, context.Canceled) {
		t.Fatalf("C10_ORIGINAL_CAUSE: unit lost ctx identity: %v", s.managerErr)
	}
}

func c10DraftWaitForGate(t *testing.T, pool *pgxpool.Pool, s *c10DraftState, holderPID uint32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), interleavingWait)
	defer cancel()
	for {
		var pid uint32
		s.mu.Lock()
		for _, e := range s.events {
			if e.Kind == "sql.start" && e.SQL == c10DraftGate {
				pid = e.PID
			}
		}
		s.mu.Unlock()
		if pid != 0 {
			var waiting bool
			err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid=a.pid
				WHERE a.pid=$1 AND a.datname=current_database() AND a.state='active' AND a.wait_event_type='Lock'
				AND $2::integer=ANY(pg_blocking_pids(a.pid)) AND l.locktype='advisory' AND NOT l.granted
				AND l.classid::bigint=$3 AND l.objid::bigint=$4 AND l.objsubid=2)`,
				pid, holderPID, c10DraftSpace, c10DraftKey).Scan(&waiting)
			if err != nil {
				t.Fatalf("C10_FIXTURE: lock-wait probe: %v", err)
			}
			if waiting {
				t.Logf("C10 real coverage query pid=%d blocked by holder=%d", pid, holderPID)
				return
			}
		}
		select {
		case <-s.done:
			t.Fatal("C10_FIXTURE: transaction finished before actual coverage lock wait was observed")
		case <-ctx.Done():
			t.Fatal("C10_FIXTURE: no actual coverage lock wait; timeout is not a mutation kill")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func c10DraftWaitForRelease(t *testing.T, pool *pgxpool.Pool, pid uint32, xid string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), interleavingWait)
	defer cancel()
	for {
		var active bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND backend_xid::text=$2)", pid, xid).Scan(&active); err != nil {
			t.Fatalf("C10_ROLLBACK: old transaction probe: %v", err)
		}
		if !active {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("C10_ROLLBACK: original transaction still exists")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func c10DraftImage(t *testing.T, pool *pgxpool.Pool, notebook string) string {
	t.Helper()
	var out string
	err := pool.QueryRow(t.Context(), `SELECT jsonb_build_object(
		'content', (SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY c.node_id),'[]'::jsonb) FROM page_contents c JOIN nodes n ON n.id=c.node_id WHERE n.notebook_id=$1),
		'index', (SELECT coalesce(jsonb_agg(to_jsonb(i) ORDER BY i.node_id),'[]'::jsonb) FROM indexed_pages i WHERE i.notebook_id=$1),
		'links', (SELECT coalesce(jsonb_agg(to_jsonb(l) ORDER BY l.source_id,l.range_start),'[]'::jsonb) FROM page_links l WHERE l.notebook_id=$1),
		'changesets', (SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY c.id),'[]'::jsonb) FROM changesets c WHERE c.notebook_id=$1),
		'revisions', (SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY r.id),'[]'::jsonb) FROM page_revisions r JOIN nodes n ON n.id=r.node_id WHERE n.notebook_id=$1),
		'marks', (SELECT coalesce(jsonb_agg(to_jsonb(m) ORDER BY m.changeset_id,m.seq),'[]'::jsonb) FROM changeset_sources m)
	)::text`, notebook).Scan(&out)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
