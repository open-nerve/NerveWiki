package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type ingestOldFPurgeImage struct {
	deleted, contentDeleted, indexHeld       bool
	revision, headRevision, extractor, edges int
	xid                                      string
}

type ingestOldFPurgeGate struct {
	life          *c10DraftState
	stream        *ingestStreamState
	notebook      uuid.UUID
	first         chan ingestOldFPurgeImage
	resume        chan struct{}
	tx            pgx.Tx
	calls         int
	before, after ingestOldFPurgeImage
}

// This diagnostic runs after native First513 closes, before app consumption
// and any Replace. After is empty in this fixture; nonempty native After
// requires the separate cross-source production case. The same-Tx image sees
// actual uncommitted rewrite/delete, not a forged Before value.
func (g *ingestOldFPurgeGate) image(ctx context.Context, tx pgx.Tx) (ingestOldFPurgeImage, error) {
	var v ingestOldFPurgeImage
	space, key := ingestIndexKey(g.notebook)
	err := tx.QueryRow(ctx, `SELECT n.deleted_at IS NOT NULL,c.deleted_at IS NOT NULL,
		c.revision,i.revision,i.extractor,
		(SELECT count(*) FROM page_links l WHERE l.source_id=n.id),
		pg_current_xact_id()::text,
		EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory'
		 AND granted AND classid::bigint=$2 AND objid::bigint=$3 AND objsubid=2)
		FROM nodes n JOIN page_contents c ON c.node_id=n.id
		JOIN indexed_pages i ON i.node_id=n.id AND i.notebook_id=n.notebook_id
		WHERE n.id=$1`, g.stream.oldF.source, int64(uint32(space)), int64(uint32(key))).
		Scan(&v.deleted, &v.contentDeleted, &v.revision, &v.headRevision, &v.extractor, &v.edges, &v.xid, &v.indexHeld)
	return v, err
}

type ingestOldFPurgeBodies struct {
	linkingapp.IngestOldBodyReads
	g *ingestOldFPurgeGate
}

func (r ingestOldFPurgeBodies) OldBodyReferences(ctx context.Context, notebook uuid.UUID, emitters []uuid.UUID, after *linkingapp.EdgePos, take int) ([]linkingapp.BodyReference, error) {
	g := r.g
	if !c10DraftActive(ctx, g.life) {
		return r.IngestOldBodyReads.OldBodyReferences(ctx, notebook, emitters, after, take)
	}
	tx, ok := postgres.TxFrom(ctx)
	if !ok || tx.Conn().PgConn().PID() != g.life.pid || notebook != g.notebook || take != 16384 || !slices.Equal(emitters, []uuid.UUID{g.stream.oldF.source}) {
		return nil, fmt.Errorf("old-F purge fixture: wrong actual transaction/source relation")
	}
	if g.calls == 0 {
		g.tx = tx
	} else if tx != g.tx {
		return nil, fmt.Errorf("old-F purge fixture: transaction changed between batches")
	}
	g.calls++
	rows, err := r.IngestOldBodyReads.OldBodyReferences(ctx, notebook, emitters, after, take)
	if err != nil || after != nil {
		return rows, err
	}
	old := g.stream.oldF
	if g.calls != 1 || len(rows) != 513 || len(old.batches) != 1 || old.batches[0].after || old.batches[0].rows != 513 || !old.batches[0].eof || !old.batches[0].errRead || !old.batches[0].closed || g.stream.replaces != 0 {
		return nil, fmt.Errorf("old-F purge fixture: First513 has not fully closed before Replace")
	}
	g.before, err = g.image(ctx, tx)
	if err != nil {
		return nil, err
	}
	g.first <- g.before
	select {
	case <-g.resume:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	g.after, err = g.image(ctx, tx)
	return rows, err
}

// Preserve C10's observation of the original Writer manager and first end.
// Separately record the real River worker's page purgers, without issuing SQL
// or pausing a busy connection from either tracer callback.
type ingestOldFPurgeQuery struct {
	name string
	pid  uint32
	rows int64
	err  error
}
type ingestOldFPurgeQueryKey struct{}
type ingestOldFPurgeTracer struct {
	c10DraftTracer
	enabled atomic.Bool
	mu      sync.Mutex
	queries []ingestOldFPurgeQuery
}

func (tr *ingestOldFPurgeTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	ctx = tr.c10DraftTracer.TraceQueryStart(ctx, conn, d)
	if tr.enabled.Load() {
		switch name := stableIndexSQLName(d.SQL); name {
		case "PurgeChangesetItems", "PurgePageRevisions", "PurgePageContents", "PurgeNodeLeaves", "PurgeChangesetSources", "PurgeChangesets", "PurgeTrimmedChangesets":
			return context.WithValue(ctx, ingestOldFPurgeQueryKey{}, ingestOldFPurgeQuery{name: name, pid: conn.PgConn().PID()})
		}
	}
	return ctx
}
func (tr *ingestOldFPurgeTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryEndData) {
	tr.c10DraftTracer.TraceQueryEnd(ctx, conn, d)
	if q, ok := ctx.Value(ingestOldFPurgeQueryKey{}).(ingestOldFPurgeQuery); ok {
		q.rows, q.err = d.CommandTag.RowsAffected(), d.Err
		tr.mu.Lock()
		tr.queries = append(tr.queries, q)
		tr.mu.Unlock()
	}
}

// Poll only durable job completion. The channel gate, not polling duration,
// establishes concurrency; the original Writer request deadline is unchanged.
func ingestOldFPurgeRunRiver(t *testing.T, f ingestFixture, writerPID uint32, writerDone <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), interleavingWait)
	defer cancel()
	prior := count(t, f.tm.pool, "SELECT coalesce(max(id),0) FROM river_job WHERE kind=$1", jobs.PurgeKind)
	inserter, err := jobs.NewInserter(f.tm.pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := f.tm.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if conn.Conn().PgConn().PID() == writerPID {
		t.Fatal("purge insertion reused the held Writer connection")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("roll back purge insertion: %v", err)
		}
	}()
	if err := inserter.InsertTx(ctx, tx, ingestPurgeArgs{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var all, completed, failed int
		err := conn.QueryRow(ctx, `SELECT count(*),
			count(*) FILTER (WHERE state='completed' AND attempt=1),
			count(*) FILTER (WHERE cardinality(errors)>0 OR attempt>1 OR state IN ('cancelled','discarded'))
			FROM river_job WHERE kind=$1 AND id>$2`, jobs.PurgeKind, prior).Scan(&all, &completed, &failed)
		if err != nil || all > 1 || failed != 0 {
			t.Fatalf("actual River purge first attempt: jobs=%d completed=%d failed=%d err=%v", all, completed, failed, err)
		}
		if all == 1 && completed == 1 {
			return
		}
		select {
		case <-writerDone:
			t.Fatal("Writer ended while First was gated; this is not a successful purge interleaving")
		case <-ctx.Done():
			t.Fatal("River purge did not complete while Writer was gated; deadline is not a semantic success")
		case <-tick.C:
		}
	}
}

func testIngestOldFPurge(t *testing.T) {
	for _, leg := range []string{"rewrite", "delete"} {
		t.Run(leg, func(t *testing.T) {
			old := &ingestOldFTailState{mode: "healthy", lastStart: -1}
			stream := &ingestStreamState{done: make(chan struct{}), mode: "healthy", oldF: old}
			life := &c10DraftState{done: make(chan struct{})}
			gate := &ingestOldFPurgeGate{life: life, stream: stream, first: make(chan ingestOldFPurgeImage, 1), resume: make(chan struct{})}
			trace := &ingestOldFPurgeTracer{}
			wrapped := 0
			f := newIngestFixture(t, &wiring{queryTracer: trace, page: func(d *page.Deps) {
				life.pool = d.Pool
				d.Tx = ingestStreamManager{Tx: c10DraftTx{Tx: d.Tx, s: life}, state: stream}
				for i, observer := range d.Observers {
					if bridge, ok := observer.(linkIndex); ok {
						if bridge.index.Ingest == nil || bridge.index.Ingest.OldBodies == nil {
							t.Fatal("real NewIndex did not wire private OldBodies")
						}
						// Exercise the original First/After fallback; the prepared route has separate native coverage.
						bridge.index.Preparations = nil
						deps := *bridge.index.Ingest
						deps.OldBodies = ingestOldFPurgeBodies{deps.OldBodies, gate}
						bridge.index.Ingest = &deps
						d.Observers[i] = bridge
						wrapped++
					}
				}
			}})
			if wrapped != 1 {
				t.Fatalf("wrapped real producer observers=%d, want1", wrapped)
			}
			awaitJob(t, f.tm.pool, jobs.PurgeKind)
			// One unrelated real trash page is past the production retention.
			// Its normal notebook has no ingestion marks to disturb our image.
			trashBook := f.tm.openNotebook(t, "alice", "Unrelated expired trash")
			trash := f.tm.createPageWith(t, "alice", trashBook, "", "Expired", "independent expired content")
			f.tm.send(t, nodeDeletion("alice", trash), http.StatusNoContent)
			f.tm.trashedAgo(t, trash, 61*24*time.Hour)
			if count(t, f.tm.pool, `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
				WHERE n.id=$1 AND n.deleted_at<now()-interval '60 days' AND c.deleted_at<now()-interval '60 days'`, trash) != 1 {
				t.Fatal("expired-trash positive control is absent")
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
			old.source, old.tail, gate.notebook = uuid.MustParse(source), wantF[512], uuid.MustParse(f.nb)
			const body = "[tail](raw/A00512.txt) [[raw/T]]"
			ops := []map[string]any{opMove("raw/A00000.txt", "raw/Z00000.txt"), opWriteOn("P.md", body, versionOf(f.p, 1))}
			if leg == "delete" {
				// Both old D(Q) and old F(S) participate, as in old_follow_delete_both.
				ops = []map[string]any{opDelete("Q513.md"), opDelete("raw/Follow513.md"), opWriteOn("P.md", body, versionOf(f.p, 1))}
			}
			m := f.tm.agent(t, "alice", f.nb)
			m.handshakeAs(t, "ingest-old-f-purge")
			before := ingestStreamImage(t, f)
			beforeChangesets := count(t, f.tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", f.nb)
			resume := sync.OnceFunc(func() { close(gate.resume) })
			done := make(chan struct{})
			var answer mcpAnswer
			t.Cleanup(func() {
				resume()
				select {
				case <-done:
				case <-time.After(interleavingWait):
					t.Error("gated Writer goroutine did not end during cleanup")
				}
			})
			stream.armed.Store(true)
			life.armed.Store(true)
			go func() { defer close(done); answer = m.batch(t, ops...) }()
			var paused ingestOldFPurgeImage
			select {
			case paused = <-gate.first:
			case <-done:
				t.Fatalf("Writer ended before closed First513: status=%d body=%s", answer.status, answer.body)
			case <-time.After(interleavingWait):
				t.Fatal("First513 gate not reached; deadline is not a semantic success")
			}
			wantRevision, wantDeleted := 2, false
			if leg == "delete" {
				wantRevision, wantDeleted = 1, true
			}
			if life.pid == 0 || paused.xid == "" || !paused.indexHeld || paused.revision != wantRevision || paused.deleted != wantDeleted || paused.contentDeleted != wantDeleted || paused.headRevision != 1 || paused.extractor != 2 || paused.edges != 513 {
				t.Fatalf("actual locked source before purge: %+v", paused)
			}
			assertPaused := func() {
				t.Helper()
				if gate.calls != 1 || stream.replaces != 0 || len(old.batches) != 1 || !old.batches[0].closed || !slices.Equal(old.ids, wantF) {
					t.Fatal("old-F advanced or replaced while First was gated")
				}
				life.mu.Lock()
				events := slices.Clone(life.events)
				life.mu.Unlock()
				first := 0
				for _, e := range events {
					if e.Kind == "sql.start" && stableIndexSQLName(e.SQL) == "IngestOldBodyReferencesAfter" {
						t.Fatal("native After was sent before purge completed")
					}
					if e.Kind == "sql.end" && stableIndexSQLName(e.SQL) == "IngestOldBodyReferencesFirst" && e.PID == life.pid && e.Err == nil {
						first++
					}
				}
				if first != 1 {
					t.Fatalf("native First completions=%d, want1", first)
				}
				if visible := ingestStreamImage(t, f); visible != before {
					t.Fatal("gated Writer leaked content/index/history/marks or purge altered its protected notebook")
				}
			}
			assertPaused()
			trace.enabled.Store(true)
			ingestOldFPurgeRunRiver(t, f, life.pid, done)
			trace.enabled.Store(false)
			assertPaused()
			for _, table := range []string{"nodes", "page_contents", "page_revisions", "changeset_items"} {
				column := "node_id"
				if table == "nodes" {
					column = "id"
				}
				if count(t, f.tm.pool, "SELECT count(*) FROM "+table+" WHERE "+column+"=$1", trash) != 0 {
					t.Fatalf("actual purge left independent expired %s", table)
				}
			}
			trace.mu.Lock()
			purged := slices.Clone(trace.queries)
			trace.mu.Unlock()
			rows := map[string]int64{}
			for _, p := range purged {
				if p.pid == 0 || p.pid == life.pid || p.err != nil {
					t.Fatalf("purger did not finish on an independent real connection: %+v", p)
				}
				rows[p.name] += p.rows
			}
			if len(rows) != 7 || rows["PurgePageContents"] != 1 || rows["PurgeNodeLeaves"] != 1 {
				t.Fatalf("actual page purgers lack nonempty independent trash control: %v", rows)
			}
			resume()
			select {
			case <-done:
			case <-time.After(interleavingWait):
				t.Fatal("resumed Writer did not finish")
			}
			select {
			case <-stream.done:
			case <-time.After(interleavingWait):
				t.Fatal("original Writer transaction did not finish")
			}
			result := answerOf[mcpBatched](t, answer)
			if gate.after != paused || gate.calls != 2 || !slices.Equal(old.ids, wantF) || len(old.batches) != 2 {
				t.Fatalf("protected source/F changed: before=%+v after=%+v calls=%d IDs=%v", paused, gate.after, gate.calls, old.ids)
			}
			for i, wantRows := range []int{513, 0} {
				b := old.batches[i]
				if b.after != (i > 0) || b.rows != wantRows || !b.eof || !b.errRead || !b.closed {
					t.Fatalf("old-F complete closed batch %d: %+v", i, b)
				}
			}
			if life.fixtureErr != nil || life.callbackErr != nil || life.managerErr != nil || stream.callbackErr != nil || stream.managerErr != nil || stream.replaces == 0 || stream.calls != 1 || stream.adds != 1 || !result.Changed || result.ChangesetID == nil || result.ResultsCount != len(ops) {
				t.Fatalf("real Writer did not commit its original unit: life=%+v stream=%+v result=%+v", life, stream, result)
			}
			ingestOldFPurgeCheckWriter(t, life)
			assertIngestMarks(t, f.tm, *result.ChangesetID, f.t)
			if count(t, f.tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", f.nb) != beforeChangesets+1 {
				t.Fatal("Writer did not retain exactly one new changeset")
			}
			if got := f.tm.content(t, "alice", f.p); got.Revision != 2 || got.Content != body {
				t.Fatalf("committed P content=%+v", got)
			}
			if leg == "rewrite" {
				if got := f.tm.content(t, "alice", source); got.Revision != 2 {
					t.Fatalf("real source rewrite=%+v", got)
				}
				if count(t, f.tm.pool, "SELECT count(*) FROM page_revisions WHERE changeset_id=$1 AND node_id=$2 AND revision=2 AND rewrite", *result.ChangesetID, source) != 1 {
					t.Fatal("source rewrite history was lost")
				}
				f.tm.resolves(t, source, ids...)
			} else {
				if count(t, f.tm.pool, `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
					WHERE n.id=$1 AND n.deleted_at IS NOT NULL AND c.deleted_at IS NOT NULL AND c.revision=1`, source) != 1 ||
					count(t, f.tm.pool, `SELECT count(*) FROM changeset_items WHERE changeset_id=$1 AND node_id=$2
					AND before_name='Follow513' AND after_name IS NULL AND revision=1 AND deleted_at IS NOT NULL`, *result.ChangesetID, source) != 1 {
					t.Fatal("real source deletion/history was lost or physically purged")
				}
				if count(t, f.tm.pool, "SELECT count(*) FROM indexed_pages WHERE node_id IN ($1,$2)", source, q) != 0 || count(t, f.tm.pool, "SELECT count(*) FROM page_links WHERE source_id=$1", source) != 0 {
					t.Fatal("committed source/Q deletion left index rows")
				}
			}
			f.tm.resolves(t, f.p, ids[512], f.t)
			c10DraftWaitForRelease(t, f.tm.pool, life.pid, paused.xid)
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
}

func ingestOldFPurgeCheckWriter(t *testing.T, s *c10DraftState) {
	t.Helper()
	s.mu.Lock()
	events := slices.Clone(s.events)
	s.mu.Unlock()
	var bodies []string
	begin, commit, rollback, commitEnd := 0, 0, 0, 0
	lockEnd, headEnd, firstBody, lastBodyEnd, firstMutation, commitStart := -1, -1, -1, -1, -1, -1
	for i, e := range events {
		if e.Kind != "sql.start" && e.Kind != "sql.end" {
			continue
		}
		if e.PID != s.pid || e.Err != nil {
			t.Fatalf("Writer SQL escaped its real Tx or failed: %+v", e)
		}
		name := stableIndexSQLName(e.SQL)
		if e.Kind == "sql.end" {
			if name == "LockNotebook" && strings.Contains(e.SQL, "pg_advisory_xact_lock") {
				lockEnd = i
			}
			if name == "IngestIndexedHeads" && firstBody < 0 {
				headEnd = i
			}
			if name == "IngestOldBodyReferencesFirst" || name == "IngestOldBodyReferencesAfter" {
				lastBodyEnd = i
			}
			if name == "commit" {
				commitEnd++
			}
			continue
		}
		switch name {
		case "begin":
			begin++
		case "commit":
			commit++
			commitStart = i
		case "rollback":
			rollback++
		case "IngestOldBodyReferencesFirst", "IngestOldBodyReferencesAfter":
			if firstBody < 0 {
				firstBody = i
			}
			bodies = append(bodies, name)
		case "DeleteLinksOf", "DeleteIndexedPages", "DeleteTagsOf", "DeletePropertiesOf", "DeleteAliasesOf", "InsertIndexedPage", "DeleteMissingPageLinks", "WritePageLinksRetainingEqualPrefiltered":
			if firstMutation < 0 {
				firstMutation = i
			}
		}
	}
	if !slices.Equal(bodies, []string{"IngestOldBodyReferencesFirst", "IngestOldBodyReferencesAfter"}) ||
		lockEnd < 0 || headEnd <= lockEnd || firstBody <= headEnd || lastBodyEnd <= firstBody || firstMutation <= lastBodyEnd || commitStart <= firstMutation ||
		begin != 1 || commit != 1 || rollback != 0 || commitEnd != 1 {
		t.Fatalf("native locked old-F/EOF/mutation/commit order: bodies=%v lock=%d head=%d first=%d eof=%d mutation=%d commit=%d begins/commits/rollbacks/commitEnds=%d/%d/%d/%d",
			bodies, lockEnd, headEnd, firstBody, lastBodyEnd, firstMutation, commitStart, begin, commit, rollback, commitEnd)
	}
}
