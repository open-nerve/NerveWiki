package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// This cancels after native First513 closes, before the buffered app tail.
// It does not cancel a running native statement or manufacture a driver error.
type ingestOldFBatchCancelState struct {
	armed  atomic.Bool
	cancel context.CancelFunc
	life   *c10DraftState
	stream *ingestStreamState
	calls  int
	fired  bool
	ctxErr error
}

type ingestOldFBatchCancelTx struct {
	page.Tx
	s *ingestOldFBatchCancelState
}

func (m ingestOldFBatchCancelTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if !m.s.armed.CompareAndSwap(true, false) {
		return m.Tx.WithinTx(ctx, fn)
	}
	// The original manager begins its real transaction with this context.
	// All callback work inherits it, including the next LintBatches.Next.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m.s.cancel = cancel
	return m.Tx.WithinTx(ctx, fn)
}

type ingestOldFBatchCancelBodies struct {
	linkingapp.IngestOldBodyReads
	s *ingestOldFBatchCancelState
}

func (r ingestOldFBatchCancelBodies) OldBodyReferences(ctx context.Context, notebook uuid.UUID, emitters []uuid.UUID, after *linkingapp.EdgePos, take int) ([]linkingapp.BodyReference, error) {
	s := r.s
	if !c10DraftActive(ctx, s.life) {
		return r.IngestOldBodyReads.OldBodyReferences(ctx, notebook, emitters, after, take)
	}
	s.calls++
	rows, err := r.IngestOldBodyReads.OldBodyReferences(ctx, notebook, emitters, after, take)
	if err != nil || after != nil {
		return rows, err
	}
	old := s.stream.oldF
	if s.calls != 1 || take != 16384 || !slices.Equal(emitters, []uuid.UUID{old.source}) || len(rows) != 513 || len(old.batches) != 1 ||
		old.batches[0].after || old.batches[0].rows != 513 || !old.batches[0].eof || !old.batches[0].errRead || !old.batches[0].closed || s.stream.replaces != 0 || ctx.Err() != nil || s.cancel == nil {
		return nil, fmt.Errorf("old-F cancel fixture: First513 is not complete/closed before cancellation")
	}
	tx, ok := postgres.TxFrom(ctx)
	if !ok || tx.Conn().PgConn().PID() != s.life.pid {
		return nil, fmt.Errorf("old-F cancel fixture: native reader escaped the original transaction")
	}
	// Rows are closed, so a same-Tx diagnostic is safe here. The actual source
	// has already been system-rewritten, while its old index still holds r1.
	var revision, head, extractor int
	err = tx.QueryRow(ctx, `SELECT pg_current_xact_id()::text,c.revision,i.revision,i.extractor
		FROM page_contents c JOIN indexed_pages i ON i.node_id=c.node_id WHERE c.node_id=$1`, old.source).
		Scan(&s.life.xid, &revision, &head, &extractor)
	if err != nil {
		return nil, fmt.Errorf("old-F cancel fixture: read real rewrite/head: %w", err)
	}
	if revision != 2 || head != 1 || extractor != 2 {
		return nil, fmt.Errorf("old-F cancel fixture: real rewrite/head=%d/%d/%d", revision, head, extractor)
	}
	s.cancel()
	s.fired, s.ctxErr = true, ctx.Err()
	s.life.note("old-F.cancel-after-close", s.ctxErr)
	// Return the original successful native batch. The first app prefix has
	// 512 rows; its next Next(ctx) must cancel before delivering buffered row513
	// or issuing After. This is not a native-query cancellation assertion.
	return rows, nil
}

func testIngestOldFBatchCancel(t *testing.T) {
	old := &ingestOldFTailState{mode: "healthy", lastStart: -1}
	stream := &ingestStreamState{done: make(chan struct{}), mode: "healthy", oldF: old}
	life := &c10DraftState{done: make(chan struct{})}
	state := &ingestOldFBatchCancelState{life: life, stream: stream}
	wrapped := 0
	f := newIngestFixture(t, &wiring{queryTracer: c10DraftTracer{}, page: func(d *page.Deps) {
		life.pool = d.Pool
		d.Tx = ingestOldFBatchCancelTx{Tx: ingestStreamManager{Tx: c10DraftTx{Tx: d.Tx, s: life}, state: stream}, s: state}
		for i, observer := range d.Observers {
			if bridge, ok := observer.(linkIndex); ok {
				if bridge.index.Ingest == nil || bridge.index.Ingest.OldBodies == nil {
					t.Fatal("real NewIndex did not wire private OldBodies")
				}
				// Exercise the original First/After fallback; the prepared route has separate native coverage.
				bridge.index.Preparations = nil
				deps := *bridge.index.Ingest
				deps.OldBodies = ingestOldFBatchCancelBodies{deps.OldBodies, state}
				deps.Marks = c10DraftMarks{IngestMarks: deps.Marks, s: life}
				bridge.index.Ingest = &deps
				bridge.index.Publisher = c10DraftPublisher{Publisher: bridge.index.Publisher, s: life}
				d.Observers[i] = bridge
				wrapped++
			}
		}
	}})
	if wrapped != 1 {
		t.Fatalf("real producer observers=%d, want1", wrapped)
	}
	ids, wikiBody := ingestManyAssets(t, f, 513)
	wantF := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		wantF[i] = uuid.MustParse(id)
	}
	source := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Follow513", "Complete old F.\n"+strings.ReplaceAll(wikiBody, "[](raw/", "[]("))
	q := f.tm.createPageWith(t, "alice", f.nb, "", "Q513", "[[raw/Follow513]]")
	f.tm.resolves(t, source, ids...)
	f.tm.resolves(t, q, source)
	old.source, old.tail = uuid.MustParse(source), wantF[512]
	m := f.tm.agent(t, "alice", f.nb)
	m.handshakeAs(t, "ingest-old-f-batch-cancel")
	ops := []map[string]any{opMove("raw/A00000.txt", "raw/Z00000.txt"), opWriteOn("P.md", "[tail](raw/A00512.txt) [[raw/T]]", versionOf(f.p, 1))}
	before := ingestStreamImage(t, f)
	state.armed.Store(true)
	stream.armed.Store(true)
	life.armed.Store(true)
	answer := m.batch(t, ops...)
	select {
	case <-stream.done:
	case <-time.After(interleavingWait):
		t.Fatal("cancelled original TxManager did not finish; deadline is not success")
	}
	if !state.fired || state.calls != 1 || !errors.Is(state.ctxErr, context.Canceled) || life.fixtureErr != nil || life.pid == 0 || life.xid == "" ||
		!errors.Is(stream.callbackErr, context.Canceled) || !errors.Is(stream.managerErr, context.Canceled) ||
		!errors.Is(life.callbackErr, context.Canceled) || !errors.Is(life.managerErr, context.Canceled) {
		t.Fatalf("original batch cancellation lost: fired=%v calls=%d ctx=%v callback=%v manager=%v fixture=%v", state.fired, state.calls, state.ctxErr, life.callbackErr, life.managerErr, life.fixtureErr)
	}
	if len(old.batches) != 1 || old.batches[0].rows != 513 || !old.batches[0].eof || !old.batches[0].errRead || !old.batches[0].closed || !slices.Equal(old.ids, wantF) {
		t.Fatalf("real completed First513 before buffered-app cancellation: batches=%+v IDs=%v", old.batches, old.ids)
	}
	if stream.replaces != 0 || stream.calls != 0 || stream.adds != 0 {
		t.Fatalf("cancelled old-F continued: Replace=%d Novel=%d Add=%d", stream.replaces, stream.calls, stream.adds)
	}
	ingestOldFBatchCancelCheckEnd(t, life)
	if after := ingestStreamImage(t, f); after != before {
		t.Fatal("batch cancellation leaked real content/index/changeset/history/marks")
	}
	if answer.status != http.StatusOK || answer.result(t)["isError"] != true {
		t.Fatalf("batch cancellation reply: status=%d body=%s", answer.status, answer.body)
	}
	c10DraftWaitForRelease(t, f.tm.pool, life.pid, life.xid)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
	// A new request on the same real app takes a fresh context and commits the
	// same previously rolled-back operation. No cancellation flag is global.
	retry := answerOf[mcpBatched](t, m.batch(t, ops...))
	if !retry.Changed || retry.ChangesetID == nil || retry.ResultsCount != 2 {
		t.Fatalf("fresh request after batch cancellation: %+v", retry)
	}
	assertIngestMarks(t, f.tm, *retry.ChangesetID, f.t)
	if c := f.tm.content(t, "alice", source); c.Revision != 2 {
		t.Fatalf("fresh request source rewrite=%+v", c)
	}
	if count(t, f.tm.pool, "SELECT count(*) FROM page_revisions WHERE changeset_id=$1 AND node_id=$2 AND revision=2 AND rewrite", *retry.ChangesetID, source) != 1 {
		t.Fatal("fresh request lost source rewrite history")
	}
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}

func ingestOldFBatchCancelCheckEnd(t *testing.T, s *c10DraftState) {
	t.Helper()
	s.mu.Lock()
	events := slices.Clone(s.events)
	s.mu.Unlock()
	begin, first, firstEnded, after, commits, rollbacks, rollbackEnded, marks, publishers := 0, 0, 0, 0, 0, 0, 0, 0, 0
	firstEnd, cancelled, callbackIn, callbackOut, rollbackStart, rollbackEnd, managerOut := -1, -1, -1, -1, -1, -1, -1
	for i, e := range events {
		switch e.Kind {
		case "callback.enter":
			if callbackIn >= 0 {
				t.Fatal("multiple actual Writer callbacks")
			}
			callbackIn = i
		case "old-F.cancel-after-close":
			if cancelled >= 0 || !errors.Is(e.Err, context.Canceled) {
				t.Fatal("wrong/repeated batch cancellation")
			}
			cancelled = i
		case "callback.exit":
			if callbackOut >= 0 || !errors.Is(e.Err, context.Canceled) {
				t.Fatal("callback did not return original cancellation")
			}
			callbackOut = i
		case "manager.exit":
			if !errors.Is(e.Err, context.Canceled) {
				t.Fatal("manager did not return original cancellation")
			}
			managerOut = i
		case "marks":
			marks++
		case "publish":
			publishers++
		}
		if e.Kind != "sql.start" && e.Kind != "sql.end" {
			continue
		}
		if e.PID != s.pid {
			t.Fatalf("Writer SQL changed actual connection: %+v", e)
		}
		name := stableIndexSQLName(e.SQL)
		if cancelled >= 0 && e.Kind == "sql.start" && name != "rollback" {
			t.Fatalf("SQL continued after completed-batch cancellation: %+v", e)
		}
		if e.Kind == "sql.end" {
			if e.Err != nil {
				t.Fatalf("native SQL failed instead of completed-batch cancellation: %+v", e)
			}
			switch name {
			case "IngestOldBodyReferencesFirst":
				firstEnded++
				firstEnd = i
			case "rollback":
				rollbackEnded++
				rollbackEnd = i
			}
			continue
		}
		switch name {
		case "begin":
			begin++
		case "IngestOldBodyReferencesFirst":
			first++
		case "IngestOldBodyReferencesAfter":
			after++
		case "commit":
			commits++
		case "rollback":
			rollbacks++
			rollbackStart = i
			if e.ContextErr != nil || !e.HasDeadline {
				t.Fatalf("original bounded WithoutCancel rollback lost: %+v", e)
			}
		}
	}
	if begin != 1 || first != 1 || firstEnded != 1 || after != 0 || commits != 0 || rollbacks != 1 || rollbackEnded != 1 || marks != 0 || publishers != 0 ||
		callbackIn < 0 || firstEnd <= callbackIn || cancelled <= firstEnd || callbackOut <= cancelled || rollbackStart <= callbackOut || rollbackEnd <= rollbackStart || managerOut <= rollbackEnd {
		t.Fatalf("First-close/cancel/rollback contract: begin/first/end/after/commit/rollback/end/marks/publish=%d/%d/%d/%d/%d/%d/%d/%d/%d order=%d,%d,%d,%d,%d,%d,%d",
			begin, first, firstEnded, after, commits, rollbacks, rollbackEnded, marks, publishers, callbackIn, firstEnd, cancelled, callbackOut, rollbackStart, rollbackEnd, managerOut)
	}
}
