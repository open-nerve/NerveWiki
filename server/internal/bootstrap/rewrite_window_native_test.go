package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// These barriers retain results of actual reserved parses, not fake facts.
// The app tests separately gate inside the real Markdown Extract callback.
// Only the pageParticipants/NewRewrite instance produced by serve is wrapped.
type rewriteNativeKey struct{}
type rewriteNativeCPUKey struct{}
type rewriteNativeState struct {
	life                                                            *c10DraftState
	budget                                                          *markdown.Budget
	mode                                                            string
	ids                                                             []uuid.UUID
	old, next                                                       map[uuid.UUID]string
	reads                                                           map[uuid.UUID]int
	target, session                                                 string
	heartbeat                                                       pgx.Tx
	heartbeatPID                                                    uint32
	cancel                                                          context.CancelFunc
	mu                                                              sync.Mutex
	workers                                                         sync.WaitGroup
	active, peak, reserved, ordinary, sqlActive, sqlPeak            int
	badPID, cpuSQL, sqlWithWorker                                   bool
	callbackActive, participantActive, observers, writes, lockReads int
	participantErr, cause                                           error
	prewrite, overlap, guardBeforeJoin, bRead                       bool
	aParsed, bParsed, fault, releaseA, releaseB                     chan struct{}
	aOnce, bOnce, faultOnce, openAOnce, openBOnce                   sync.Once
}

func (s *rewriteNativeState) openA() { s.openAOnce.Do(func() { close(s.releaseA) }) }
func (s *rewriteNativeState) openB() { s.openBOnce.Do(func() { close(s.releaseB) }) }
func rewriteNativeActive(ctx context.Context, s *rewriteNativeState) bool {
	return ctx.Value(rewriteNativeKey{}) == s
}
func (s *rewriteNativeState) parsers() int { s.mu.Lock(); defer s.mu.Unlock(); return s.active }

// Keep the original manager's transaction/end behavior and C10's query log.
type rewriteNativeManager struct {
	page.Tx
	s *rewriteNativeState
}

func (m rewriteNativeManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if !m.s.life.armed.Load() {
		return m.Tx.WithinTx(ctx, fn)
	}
	s := m.s
	ctx, cancel := context.WithCancel(context.WithValue(ctx, rewriteNativeKey{}, s))
	defer cancel()
	s.cancel = cancel
	return (c10DraftTx{Tx: m.Tx, s: s.life}).WithinTx(ctx, func(ctx context.Context) error {
		err := fn(ctx)
		s.callbackActive = s.parsers()
		return err
	})
}

type rewriteNativeTracer struct{}

func (rewriteNativeTracer) TraceQueryStart(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	ctx = (c10DraftTracer{}).TraceQueryStart(ctx, c, d)
	if s, ok := ctx.Value(rewriteNativeKey{}).(*rewriteNativeState); ok {
		s.mu.Lock()
		s.sqlActive++
		s.sqlPeak = max(s.sqlPeak, s.sqlActive)
		s.badPID = s.badPID || s.life.pid != 0 && c.PgConn().PID() != s.life.pid
		s.sqlWithWorker = s.sqlWithWorker || s.active > 0
		s.cpuSQL = s.cpuSQL || ctx.Value(rewriteNativeCPUKey{}) != nil
		s.mu.Unlock()
	}
	return ctx
}
func (rewriteNativeTracer) TraceQueryEnd(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryEndData) {
	(c10DraftTracer{}).TraceQueryEnd(ctx, c, d)
	if s, ok := ctx.Value(rewriteNativeKey{}).(*rewriteNativeState); ok {
		s.mu.Lock()
		s.sqlActive--
		s.mu.Unlock()
	}
}

type rewriteNativeParser struct {
	linkingapp.RewriteParser
	leases linkingapp.RewriteReservations
	s      *rewriteNativeState
}

func (p rewriteNativeParser) ParseNow(ctx context.Context, content string) (linkingapp.Parsed, error) {
	if !rewriteNativeActive(ctx, p.s) {
		return p.RewriteParser.ParseNow(ctx, content)
	}
	p.s.mu.Lock()
	p.s.ordinary++
	p.s.mu.Unlock()
	return p.RewriteParser.ParseNow(context.WithValue(ctx, rewriteNativeCPUKey{}, true), content)
}
func (p rewriteNativeParser) TryReserve(n int) (linkingapp.RewriteReservation, bool) {
	r, ok := p.leases.TryReserve(n)
	if !ok {
		return nil, false
	}
	return rewriteNativeReservation{RewriteReservation: r, s: p.s}, true
}

type rewriteNativeReservation struct {
	linkingapp.RewriteReservation
	s *rewriteNativeState
}

func (r rewriteNativeReservation) ParseNow(ctx context.Context, content string) (linkingapp.Parsed, error) {
	s := r.s
	if !rewriteNativeActive(ctx, s) {
		return r.RewriteReservation.ParseNow(ctx, content)
	}
	s.workers.Add(1)
	defer s.workers.Done()
	s.mu.Lock()
	s.active++
	s.reserved++
	s.peak = max(s.peak, s.active)
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.active--; s.mu.Unlock() }()
	out, err := r.RewriteReservation.ParseNow(context.WithValue(ctx, rewriteNativeCPUKey{}, true), content)
	if err != nil {
		return out, err
	}
	if content == s.next[s.ids[0]] && s.mode == "future_sql_error" {
		s.aOnce.Do(func() { close(s.aParsed) })
		select {
		case <-s.releaseA:
		case <-ctx.Done():
		}
	}
	if content == s.old[s.ids[1]] && s.mode != "cancel_after_rewrite" {
		s.bOnce.Do(func() { close(s.bParsed) })
		select {
		case <-s.releaseB:
		case <-ctx.Done():
		}
	}
	return out, nil
}

type rewriteNativeContents struct {
	linking.PageContents
	s *rewriteNativeState
}

func (c rewriteNativeContents) Content(ctx context.Context, id uuid.UUID) (string, int, bool, error) {
	s := c.s
	if !rewriteNativeActive(ctx, s) {
		return c.PageContents.Content(ctx, id)
	}
	s.mu.Lock()
	s.reads[id]++
	s.mu.Unlock()
	text, revision, ok, err := c.PageContents.Content(ctx, id)
	if err != nil {
		return text, revision, ok, err
	}
	if id == s.ids[1] {
		s.bRead = true
		if s.mode == "future_sql_error" {
			// This added diagnostic statement is real PG on the original Tx;
			// it does not pretend the native PageContent query itself failed.
			select {
			case <-s.aParsed:
			case <-ctx.Done():
				return "", 0, false, ctx.Err()
			}
			_, s.cause = postgres.DB(ctx, s.life.pool).Exec(ctx, "/* rewrite window native fault */ SELECT 1/0")
			s.faultOnce.Do(func() { close(s.fault) })
			return "", 0, false, s.cause
		}
	}
	return text, revision, ok, nil
}

type rewriteNativeLocks struct {
	linking.Locks
	s *rewriteNativeState
}

func (l rewriteNativeLocks) Of(ctx context.Context, ids []uuid.UUID, at time.Time) ([]shared.LockHolder, error) {
	out, err := l.Locks.Of(ctx, ids, at)
	s := l.s
	if !rewriteNativeActive(ctx, s) || s.mode != "guard_before_join" {
		return out, err
	}
	s.lockReads++
	if err != nil {
		return out, err
	}
	switch s.lockReads {
	case 1:
		if len(out) != 0 || s.heartbeatPID == s.life.pid {
			return nil, errors.New("WN_HEARTBEAT: precheck or independent PID")
		}
		// The same legal heartbeat window as the existing native rewrite test.
		heartbeatCtx, cancel := context.WithTimeout(context.Background(), interleavingWait)
		defer cancel()
		tag, e := s.heartbeat.Exec(heartbeatCtx, "UPDATE edit_sessions SET expires_at=now()+interval '1 minute' WHERE id=$1", s.session)
		if e != nil {
			return nil, e
		}
		if tag.RowsAffected() != 1 {
			return nil, errors.New("WN_HEARTBEAT: no real session updated")
		}
		if e = s.heartbeat.Commit(heartbeatCtx); e != nil {
			return nil, e
		}
	case 2:
		s.guardBeforeJoin = s.parsers() > 0 && ctx.Err() == nil && len(out) == 1 && out[0].PageID == s.ids[0]
		// Cancel only AFTER the original guardLocked SQL has succeeded. The
		// worker then finishes; no sleep or guessed heartbeat timing is used.
		s.cancel()
		s.openB()
	}
	return out, nil
}

type rewriteNativeAppender struct {
	page.Appender
	s *rewriteNativeState
}

func (a rewriteNativeAppender) WriteContent(ctx context.Context, w page.ContentWrite) (int, error) {
	s := a.s
	if !rewriteNativeActive(ctx, s) {
		return a.Appender.WriteContent(ctx, w)
	}
	s.writes++
	if w.NodeID == s.ids[0] && s.mode != "cancel_after_rewrite" {
		// Serial controls must reach their real write and fail WN_RESERVATIONS,
		// not hang on a barrier for an optional worker they never started.
		s.mu.Lock()
		reserved := s.reserved
		s.mu.Unlock()
		if reserved > 0 {
			select {
			case <-s.bParsed:
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
		s.overlap = s.parsers() > 0
	}
	revision, err := a.Appender.WriteContent(ctx, w)
	if w.NodeID == s.ids[0] && err == nil {
		if s.mode == "cancel_after_rewrite" {
			s.cancel()
		}
		s.openB()
	}
	return revision, err
}

type rewriteNativeParticipant struct {
	inner linkRewrite
	s     *rewriteNativeState
}

func (p rewriteNativeParticipant) Participate(ctx context.Context, step page.Step, u page.Appender) error {
	if !rewriteNativeActive(ctx, p.s) {
		return p.inner.Participate(ctx, step, u)
	}
	s := p.s
	var name string
	if err := postgres.DB(ctx, s.life.pool).QueryRow(ctx, "SELECT name FROM nodes WHERE id=$1", s.target).Scan(&name); err != nil {
		return err
	}
	s.prewrite = name == "B"
	err := p.inner.Participate(ctx, step, rewriteNativeAppender{Appender: u, s: s})
	s.participantErr, s.participantActive = err, s.parsers()
	s.life.note("window.participant.exit", err)
	return err
}

type rewriteNativeObserver struct {
	page.PageObserver
	s *rewriteNativeState
}

func (o rewriteNativeObserver) PagesChanged(ctx context.Context, e page.Event) error {
	if rewriteNativeActive(ctx, o.s) {
		o.s.observers++
	}
	return o.PageObserver.PagesChanged(ctx, e)
}

func TestRewriteWindowNativeWriter(t *testing.T) {
	for _, mode := range []string{"healthy", "future_sql_error", "guard_before_join", "cancel_after_rewrite"} {
		t.Run(mode, func(t *testing.T) {
			s := &rewriteNativeState{mode: mode, old: map[uuid.UUID]string{}, next: map[uuid.UUID]string{}, reads: map[uuid.UUID]int{},
				aParsed: make(chan struct{}), bParsed: make(chan struct{}), fault: make(chan struct{}), releaseA: make(chan struct{}), releaseB: make(chan struct{}),
				life: &c10DraftState{done: make(chan struct{})}}
			tm := newAcmeTeamWiring(t, "member", "", wiring{queryTracer: rewriteNativeTracer{}, page: func(d *page.Deps) {
				s.life.pool, s.budget = d.Pool, d.Budget
				d.Tx = rewriteNativeManager{Tx: d.Tx, s: s}
				found := 0
				for i, part := range d.Participants {
					if real, ok := part.(linkRewrite); ok {
						leases, ok := real.rewrite.Parser.(linkingapp.RewriteReservations)
						if !ok {
							t.Fatal("WN_CONSTRUCTOR: actual NewRewrite parser has no reservation capability")
						}
						real.rewrite.Parser = rewriteNativeParser{RewriteParser: real.rewrite.Parser, leases: leases, s: s}
						real.rewrite.Contents = rewriteNativeContents{PageContents: real.rewrite.Contents, s: s}
						real.rewrite.Locks = rewriteNativeLocks{Locks: real.rewrite.Locks, s: s}
						d.Participants[i] = rewriteNativeParticipant{inner: real, s: s}
						found++
					}
				}
				if found != 1 {
					t.Fatalf("WN_CONSTRUCTOR: actual participants=%d", found)
				}
				for i, o := range d.Observers {
					d.Observers[i] = rewriteNativeObserver{PageObserver: o, s: s}
				}
			}})
			t.Cleanup(func() { s.openA(); s.openB() }) // Registered after app.close: unblock workers before server cleanup.
			nb := tm.openNotebook(t, "alice", "Eng")
			s.target = tm.createPage(t, "alice", nb, "", "A")
			for i := range 2 {
				body := fmt.Sprintf("slot%d [[A]]\n", i)
				id := uuid.MustParse(tm.createPageWith(t, "alice", nb, "", fmt.Sprintf("Source%d", i), body))
				s.ids = append(s.ids, id)
				s.old[id] = body
				s.next[id] = strings.ReplaceAll(body, "[[A]]", "[[B]]")
			}
			slices.SortFunc(s.ids, uuid.UUID.Compare)
			if mode == "guard_before_join" {
				s.session = tm.openSession(t, "bob", s.ids[0].String())
				var err error
				s.heartbeat, err = tm.pool.Begin(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				s.heartbeatPID = s.heartbeat.Conn().PgConn().PID()
				t.Cleanup(func() { _ = s.heartbeat.Rollback(context.Background()) })
				if _, err = s.heartbeat.Exec(t.Context(), "SELECT 1 FROM edit_sessions WHERE id=$1 FOR KEY SHARE", s.session); err != nil {
					t.Fatal(err)
				}
				if _, err = tm.pool.Exec(t.Context(), "UPDATE edit_sessions SET created_at=now()-interval '1 minute',expires_at=now()-interval '1 second' WHERE id=$1", s.session); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "cancel_after_rewrite" {
				if _, err := tm.pool.Exec(t.Context(), "UPDATE indexed_pages SET revision=77 WHERE node_id=$1", s.ids[1]); err != nil {
					t.Fatal(err)
				}
			}
			image := c10DraftImage(t, tm.pool, nb)
			rows := sourceRollbackRows(t, tm.pool)
			s.life.armed.Store(true)
			send := tm.sender(t, nodeRename("alice", s.target, "B"))
			answered := make(chan answer, 1)
			requestDone := make(chan struct{})
			go func() { defer close(requestDone); answered <- send() }()
			t.Cleanup(func() {
				s.openA()
				s.openB()
				select {
				case <-requestDone:
				case <-time.After(interleavingWait):
					t.Error("WN_CLEANUP: request failed to join; infrastructure failure")
					return
				}
				s.workers.Wait() // Both gates are open; real parses are tiny and every reserved call is accounted.
			})
			if mode == "future_sql_error" {
				select {
				case <-s.fault:
				case <-time.After(interleavingWait):
					t.Fatal("WN_NATIVE_FAULT: gate missing; timeout is not a mutation oracle")
				}
				if s.parsers() == 0 {
					t.Error("WN_NATIVE_FAULT: no actual reserved result awaited join at native failure")
				}
				s.openA()
			}
			var got answer
			select {
			case got = <-answered:
			case <-time.After(interleavingWait):
				t.Fatal("WN_NATIVE_END: timeout is not a mutation oracle")
			}
			select {
			case <-s.life.done:
			case <-time.After(interleavingWait):
				t.Fatal("WN_NATIVE_TX: original manager did not end")
			}
			if got.res == nil {
				t.Fatalf("HTTP request: %v", got.err)
			}
			tm.contract.CheckResponse(t, got.req, got.res)
			wantParses := map[string]int{"healthy": 4, "future_sql_error": 2, "guard_before_join": 3, "cancel_after_rewrite": 2}[mode]
			if s.reserved != wantParses || s.ordinary != 0 {
				t.Fatalf("WN_RESERVATIONS: actual reserved/plain parses=%d/%d, want %d/0", s.reserved, s.ordinary, wantParses)
			}
			for _, id := range s.ids {
				if s.reads[id] != 1 {
					t.Fatalf("WN_CONTENT_ONCE: page%s read%d", id, s.reads[id])
				}
			}
			if s.active != 0 || s.callbackActive != 0 || s.participantActive != 0 || s.peak > 2 {
				t.Fatalf("WN_JOIN: active/callback/participant/peak=%d/%d/%d/%d", s.active, s.callbackActive, s.participantActive, s.peak)
			}
			if s.sqlActive != 0 || s.sqlPeak != 1 || s.badPID || s.cpuSQL || !s.prewrite || !s.bRead {
				t.Fatalf("WN_REAL_TX: sql=%d/%d wrongPID=%t cpuSQL=%t prewrite=%t readB=%t", s.sqlActive, s.sqlPeak, s.badPID, s.cpuSQL, s.prewrite, s.bRead)
			}
			commits, rollbacks := 0, 0
			for _, e := range s.life.events {
				if e.Kind == "sql.start" {
					if mode != "healthy" && strings.Contains(strings.ToLower(e.SQL), "pg_notify") {
						t.Fatal("WN_NO_PUBLISH: failed Writer attempted native publish")
					}
					if e.PID != s.life.pid {
						t.Fatalf("WN_REAL_PID: SQL %s PID%d != Tx%d", stableIndexSQLName(e.SQL), e.PID, s.life.pid)
					}
					switch strings.ToLower(strings.TrimSpace(e.SQL)) {
					case "commit":
						commits++
					case "rollback":
						rollbacks++
					}
				}
			}
			if mode == "healthy" {
				if got.status != http.StatusOK || s.life.managerErr != nil || commits != 1 || rollbacks != 0 || !s.overlap || !s.sqlWithWorker || s.writes != 2 || s.observers == 0 {
					t.Fatalf("WN_HEALTHY: reply=%d err=%v ends=%d/%d overlap=%t writes=%d observers=%d", got.status, s.life.managerErr, commits, rollbacks, s.overlap, s.writes, s.observers)
				}
				for _, id := range s.ids {
					tm.wrote(t, id.String(), s.next[id], 2)
				}
				checkPages(t, tm.pool)
				checkLinks(t, tm.pool)
			} else {
				if commits != 0 || rollbacks != 1 || s.observers != 0 || image != c10DraftImage(t, tm.pool, nb) || !maps.Equal(rows, sourceRollbackRows(t, tm.pool)) {
					t.Fatalf("WN_ROLLBACK: ends=%d/%d observers=%d or full committed image changed", commits, rollbacks, s.observers)
				}
				switch mode {
				case "future_sql_error":
					var pgerr *pgconn.PgError
					if !errors.As(s.cause, &pgerr) || pgerr.Code != "22012" || !errors.Is(s.life.callbackErr, s.cause) || !errors.Is(s.life.managerErr, s.cause) || s.writes != 0 {
						t.Fatalf("WN_NATIVE_CAUSE: cause=%v callback=%v manager=%v writes=%d", s.cause, s.life.callbackErr, s.life.managerErr, s.writes)
					}
				case "guard_before_join":
					pagesLocked(t, "window guard after native precheck", got, map[string]string{s.ids[0].String(): "bob"})
					var problem *shared.Error
					if !s.guardBeforeJoin || s.lockReads != 2 || !errors.As(s.life.callbackErr, &problem) || problem.Code != "linking.pages_locked" || errors.Is(s.life.managerErr, context.Canceled) {
						t.Fatalf("WN_GUARD_BEFORE_JOIN: before=%t reads=%d callback=%v manager=%v", s.guardBeforeJoin, s.lockReads, s.life.callbackErr, s.life.managerErr)
					}
				case "cancel_after_rewrite":
					// The real rename callback rereads the node after Rewrite returns.
					// Observe its first native query, without assuming markMoves ran.
					afterRewrite := false
					var nextStart, nextEnd *c10DraftEvent
					for i := range s.life.events {
						e := &s.life.events[i]
						if e.Kind == "window.participant.exit" {
							afterRewrite = true
						}
						if !afterRewrite {
							continue
						}
						if e.Kind == "callback.exit" {
							break
						}
						if e.Kind == "sql.start" && nextStart == nil {
							switch strings.ToLower(strings.TrimSpace(e.SQL)) {
							case "commit", "rollback":
								continue
							}
							nextStart = e
						}
						if nextStart != nil && e.Kind == "sql.end" && e.SQL == nextStart.SQL {
							nextEnd = e
							break
						}
					}
					if nextStart == nil || nextEnd == nil {
						t.Fatal("WN_ACTUAL_LAST_SQL: no complete native query between Rewrite and callback exit")
					}
					if s.participantErr != nil || !errors.Is(nextStart.ContextErr, context.Canceled) || !errors.Is(nextEnd.Err, context.Canceled) || !errors.Is(s.life.callbackErr, context.Canceled) || !errors.Is(s.life.managerErr, context.Canceled) || !errors.Is(s.life.callbackErr, nextEnd.Err) || !errors.Is(s.life.managerErr, nextEnd.Err) || s.writes != 1 {
						t.Fatalf("WN_ACTUAL_LAST_SQL: rewrite=%v nextSQL=%s startCtx=%v native=%v callback=%v manager=%v writes=%d", s.participantErr, stableIndexSQLName(nextStart.SQL), nextStart.ContextErr, nextEnd.Err, s.life.callbackErr, s.life.managerErr, s.writes)
					}
					t.Logf("WN_ACTUAL_LAST_SQL: first query after Rewrite=%s; native cancellation cause preserved", stableIndexSQLName(nextStart.SQL))
				}
			}
			h, err := s.budget.TakeNow(t.Context(), s.budget.Size())
			if err != nil {
				t.Fatalf("WN_RELEASE: original whole budget not returned: %v", err)
			}
			h.Release()
			if count(t, tm.pool, "SELECT 1") != 1 {
				t.Fatal("real pool did not recover")
			}
		})
	}
}
