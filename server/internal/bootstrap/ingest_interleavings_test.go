package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// These wrappers keep the original composition root observer and its complete
// ingest bundle. In particular they do not build another Index, omit the real
// bridge, acquire an index lock themselves, or replace any query's answer.
type ingestInterleaveControl struct {
	mu    sync.Mutex
	units map[uuid.UUID]*ingestInterleaveUnit
}

type ingestInterleaveUnit struct {
	node, peer       uuid.UUID
	body             string
	revision         int
	failAfterRealAdd bool
	written          chan ingestWriteSeen
	beforeCoverage   chan ingestCoverageEntry
	coverageRead     chan ingestCoverageSeen
	marks            chan ingestMarksSeen
	observerResult   chan error
	continueCoverage chan struct{}
	releaseOnce      sync.Once
}

type ingestWriteSeen struct {
	pid          int32
	notebook, cs uuid.UUID
	body         string
	revision     int
	tree         bool
	kind, iso    string
	err          error
}

type ingestCoverageEntry struct {
	pid        int32
	holdsIndex bool
	err        error
}

type ingestCoverageSeen struct {
	peerContent                   string
	peerRevision, indexedRevision int
	extractor, metadataRevision   int
	metadataFound                 bool
	err                           error
}

type ingestMarksSeen struct {
	cs     uuid.UUID
	ids    []uuid.UUID
	result linking.IngestMarkResult
	stored int
	err    error
}

type ingestUnitContextKey struct{}

func newIngestInterleaveUnit(t *testing.T, node, peer, body string) *ingestInterleaveUnit {
	t.Helper()
	u := &ingestInterleaveUnit{
		node: uuid.MustParse(node), peer: uuid.MustParse(peer), body: body, revision: 2,
		written: make(chan ingestWriteSeen, 1), beforeCoverage: make(chan ingestCoverageEntry, 1),
		coverageRead: make(chan ingestCoverageSeen, 1), marks: make(chan ingestMarksSeen, 1),
		observerResult: make(chan error, 1), continueCoverage: make(chan struct{}),
	}
	t.Cleanup(u.release)
	return u
}

func (u *ingestInterleaveUnit) release() { u.releaseOnce.Do(func() { close(u.continueCoverage) }) }

func (c *ingestInterleaveControl) arm(units ...*ingestInterleaveUnit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.units = make(map[uuid.UUID]*ingestInterleaveUnit, len(units))
	for _, u := range units {
		c.units[u.node] = u
	}
}

func (c *ingestInterleaveControl) find(e page.Event) (*ingestInterleaveUnit, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var found *ingestInterleaveUnit
	for _, change := range e.Changes {
		if u := c.units[change.NodeID]; u != nil {
			if found != nil || len(e.Changes) != 1 || change.Revision != u.revision {
				return nil, errors.New("ingest interleaving: expected exactly one ordinary content change")
			}
			found = u
		}
	}
	return found, nil
}

func (c *ingestInterleaveControl) wiring(t *testing.T) wiring {
	t.Helper()
	return wiring{page: func(d *page.Deps) {
		found := 0
		for i, observer := range d.Observers {
			original, ok := observer.(linkIndex)
			if !ok {
				continue
			}
			found++
			if original.index.Ingest == nil || original.index.Ingest.Metadata == nil || original.index.Ingest.Marks == nil {
				t.Fatal("real linkIndex is missing ingest dependencies")
			}
			deps := *original.index.Ingest
			deps.Metadata = ingestInterleaveMetadata{real: deps.Metadata}
			deps.Marks = ingestInterleaveMarks{real: deps.Marks}
			original.index.Ingest = &deps
			d.Observers[i] = ingestInterleaveObserver{real: original, control: c}
		}
		if found != 1 {
			t.Fatalf("real linkIndex observer count = %d, want 1", found)
		}
	}}
}

type ingestInterleaveObserver struct {
	real    page.PageObserver
	control *ingestInterleaveControl
}

func (o ingestInterleaveObserver) PagesChanged(ctx context.Context, e page.Event) error {
	u, err := o.control.find(e)
	if err != nil || u == nil {
		if err != nil {
			return err
		}
		return o.real.PagesChanged(ctx, e)
	}
	seen := ingestWriteSeen{notebook: e.NotebookID, cs: e.ChangesetID, tree: e.Tree, kind: string(e.Kind)}
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		seen.err = errors.New("ingest interleaving: observer has no real transaction")
	} else {
		seen.pid = int32(tx.Conn().PgConn().PID())
		seen.err = tx.QueryRow(ctx, `SELECT content, revision, current_setting('transaction_isolation')
			FROM page_contents WHERE node_id=$1`, u.node).Scan(&seen.body, &seen.revision, &seen.iso)
	}
	u.written <- seen
	if seen.err != nil {
		u.observerResult <- seen.err
		return seen.err
	}
	err = o.real.PagesChanged(context.WithValue(ctx, ingestUnitContextKey{}, u), e)
	u.observerResult <- err
	return err
}

type ingestInterleaveMetadata struct{ real linking.IngestMetadata }

func (m ingestInterleaveMetadata) Now(ctx context.Context, notebook, root uuid.UUID, ids []uuid.UUID) ([]linking.IngestHead, error) {
	return m.real.Now(ctx, notebook, root, ids)
}

func (m ingestInterleaveMetadata) Heads(ctx context.Context, notebook, root uuid.UUID, after *uuid.UUID, take int) (linking.IngestHeadBatch, error) {
	u, _ := ctx.Value(ingestUnitContextKey{}).(*ingestInterleaveUnit)
	if u == nil || after != nil {
		return m.real.Heads(ctx, notebook, root, after, take)
	}
	tx, ok := postgres.TxFrom(ctx)
	entry := ingestCoverageEntry{}
	if !ok {
		entry.err = errors.New("ingest interleaving: coverage lost its write transaction")
	} else {
		entry.pid = int32(tx.Conn().PgConn().PID())
		space, key := ingestIndexKey(notebook)
		entry.err = tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory'
			AND database=(SELECT oid FROM pg_database WHERE datname=current_database())
			AND classid::bigint=$1 AND objid::bigint=$2 AND objsubid=2 AND granted)`,
			int64(uint32(space)), int64(uint32(key))).Scan(&entry.holdsIndex)
	}
	u.beforeCoverage <- entry
	if entry.err != nil {
		return linking.IngestHeadBatch{}, entry.err
	}
	select {
	case <-u.continueCoverage:
	case <-ctx.Done():
		return linking.IngestHeadBatch{}, ctx.Err()
	}
	// The production Heads query is executed unchanged, on its original ctx.
	// In the locked-Q mutation this is the call that must be caught as a
	// reciprocal PG wait (or SQLSTATE 40P01), not as a generic timeout.
	batch, err := m.real.Heads(ctx, notebook, root, after, take)
	seen := ingestCoverageSeen{err: err}
	if err == nil {
		for _, h := range batch.Rows {
			if h.ID == u.peer {
				seen.metadataFound, seen.metadataRevision = true, h.Revision
			}
		}
		// This diagnostic plain SELECT reads actual content and head in the
		// SAME A transaction. It neither supplies coverage's answer nor locks Q.
		// B cannot alter its index while A retains the real index advisory lock.
		seen.err = tx.QueryRow(ctx, `SELECT c.content,c.revision,i.revision,i.extractor
			FROM page_contents c JOIN indexed_pages i ON i.node_id=c.node_id
			WHERE c.node_id=$1 AND i.notebook_id=$2`, u.peer, notebook).
			Scan(&seen.peerContent, &seen.peerRevision, &seen.indexedRevision, &seen.extractor)
	}
	u.coverageRead <- seen
	if seen.err != nil {
		return linking.IngestHeadBatch{}, seen.err
	}
	return batch, nil
}

type ingestInterleaveMarks struct{ real linking.IngestMarks }

type ingestAfterMarksFault struct{}

func (ingestAfterMarksFault) Error() string {
	return "I1 controlled failure after real same-transaction marks"
}

func (m ingestInterleaveMarks) Add(ctx context.Context, notebook, changeset uuid.UUID, ids []uuid.UUID) (linking.IngestMarkResult, error) {
	result, err := m.real.Add(ctx, notebook, changeset, ids)
	u, _ := ctx.Value(ingestUnitContextKey{}).(*ingestInterleaveUnit)
	if u == nil {
		return result, err
	}
	seen := ingestMarksSeen{cs: changeset, ids: slices.Clone(ids), result: result, err: err}
	if err == nil {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			seen.err = errors.New("ingest interleaving: marks lost its transaction")
		} else {
			seen.err = tx.QueryRow(ctx, `SELECT count(*) FROM changeset_sources WHERE changeset_id=$1`, changeset).Scan(&seen.stored)
		}
	}
	u.marks <- seen
	if seen.err != nil {
		return result, seen.err
	}
	if u.failAfterRealAdd {
		return result, ingestAfterMarksFault{}
	}
	return result, nil
}

type ingestInterleaveFixture struct {
	tm                   acmeTeam
	nb, root, s, t, p, q string
	initialChangesets    int
	initialMarks         int
}

func newIngestInterleaveFixture(t *testing.T, control *ingestInterleaveControl, qOld string, withT bool) ingestInterleaveFixture {
	t.Helper()
	tm := newAcmeTeamWiring(t, "", "", control.wiring(t))
	nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Ingest interleaving","mode":"llm_wiki"}`)
	if nb.Mode != "llm_wiki" || nb.SourceRootID == nil {
		t.Fatalf("fixture notebook = %+v, want llm_wiki with source root", nb)
	}
	f := ingestInterleaveFixture{tm: tm, nb: nb.ID, root: *nb.SourceRootID}
	f.s = tm.createPageWith(t, "alice", f.nb, f.root, "S", "Independent source S.\n")
	if withT {
		f.t = tm.createPageWith(t, "alice", f.nb, f.root, "T", "Independent source T.\n")
	}
	f.p = tm.createPage(t, "alice", f.nb, "", "P")
	f.q = tm.createPageWith(t, "alice", f.nb, "", "Q", qOld)
	for _, source := range []struct{ id, content string }{{f.s, "Independent source S.\n"}, {f.t, "Independent source T.\n"}} {
		if source.id == "" {
			continue
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM notebooks b JOIN nodes n ON n.notebook_id=b.id
			JOIN page_contents c ON c.node_id=n.id JOIN indexed_pages i ON i.node_id=n.id
			WHERE b.id=$1 AND b.mode='llm_wiki' AND b.source_root_id=$2 AND b.deleted_at IS NULL
			AND n.id=$3 AND n.parent_id=$2 AND n.kind='page' AND n.deleted_at IS NULL
			AND c.content=$4 AND c.revision=1 AND c.byte_size>0 AND c.deleted_at IS NULL
			AND n.source_hash=sha256(convert_to($4,'UTF8'))
			AND i.notebook_id=b.id AND i.revision=c.revision AND i.extractor=2`,
			f.nb, f.root, source.id, source.content); n != 1 {
			t.Fatalf("source %s is not the expected nonempty current source", source.id)
		}
	}
	for _, p := range []struct{ id, body string }{{f.p, ""}, {f.q, qOld}} {
		if got := tm.content(t, "alice", p.id); got.Content != p.body || got.Revision != 1 {
			t.Fatalf("initial wiki %s = %+v, want %q at r1", p.id, got, p.body)
		}
	}
	f.initialChangesets = count(t, tm.pool, `SELECT count(*) FROM changesets WHERE notebook_id=$1`, f.nb)
	f.initialMarks = count(t, tm.pool, `SELECT count(*) FROM changeset_sources s JOIN changesets c ON c.id=s.changeset_id WHERE c.notebook_id=$1`, f.nb)
	ingestInterleaveInvariants(t, f)
	return f
}

func ingestIndexKey(notebook uuid.UUID) (int32, int32) {
	h := fnv.New32a()
	_, _ = h.Write(notebook[:])
	return int32(0x6c696e6b), int32(h.Sum32())
}

func ingestHoldIndex(t *testing.T, f ingestInterleaveFixture) (pgx.Tx, int32) {
	t.Helper()
	tx, err := f.tm.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	space, key := ingestIndexKey(uuid.MustParse(f.nb))
	if _, err := tx.Exec(t.Context(), `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, space, key); err != nil {
		t.Fatal(err)
	}
	return tx, int32(tx.Conn().PgConn().PID())
}

// Closing done publishes the immutable result, which several independent
// observers can inspect without consuming the later HTTP assertion's value.
type ingestPending struct {
	done   chan struct{}
	result answer
}

func ingestStartWrite(t *testing.T, f ingestInterleaveFixture, u *ingestInterleaveUnit) *ingestPending {
	t.Helper()
	send := f.tm.sender(t, contentWrite("alice", u.node.String(), u.body, u.revision-1, ""))
	pending := &ingestPending{done: make(chan struct{})}
	go func() {
		pending.result = send()
		close(pending.done)
	}()
	return pending
}

func ingestNext[T any](t *testing.T, label string, ch <-chan T, pending *ingestPending) T {
	t.Helper()
	deadline := time.NewTimer(interleavingWait)
	defer deadline.Stop()
	var requestDone <-chan struct{}
	if pending != nil {
		requestDone = pending.done
	}
	select {
	case v := <-ch:
		return v
	case <-requestDone:
		// A hook may have enqueued its observation immediately before HTTP
		// completed. Always consume that observation if it is already there.
		select {
		case v := <-ch:
			return v
		default:
			a := pending.result
			t.Fatalf("%s: request ended before the observation: %d %s (%v)", label, a.status, a.body, a.err)
		}
	case <-deadline.C:
		t.Fatalf("%s: observation deadline (infrastructure/liveness failure; NOT a mutation-kill oracle)", label)
	}
	var zero T
	return zero
}

func ingestWritten(t *testing.T, f ingestInterleaveFixture, u *ingestInterleaveUnit, done *ingestPending) ingestWriteSeen {
	t.Helper()
	w := ingestNext(t, "own content already written", u.written, done)
	if w.err != nil || w.pid == 0 || w.notebook.String() != f.nb || w.cs == uuid.Nil() || w.tree || w.kind != "edit" ||
		w.iso != "read committed" || w.body != u.body || w.revision != u.revision {
		t.Fatalf("ordinary SHARE write observation = %+v; want own r%d body %q, edit, READ COMMITTED, Tree=false", w, u.revision, u.body)
	}
	return w
}

// Exact PID and exact notebook key: another database or a background service
// cannot satisfy this barrier. Polls execute real SQL; no sleep orders writers.
func ingestWaitIndex(t *testing.T, f ingestInterleaveFixture, u *ingestInterleaveUnit, w ingestWriteSeen, holderPID int32, done *ingestPending) {
	t.Helper()
	space, key := ingestIndexKey(uuid.MustParse(f.nb))
	ctx, cancel := context.WithTimeout(t.Context(), interleavingWait)
	defer cancel()
	for {
		select {
		case e := <-u.beforeCoverage:
			t.Fatalf("U01 index serialization: writer %d reached coverage before its required index wait; entry=%+v", w.pid, e)
		case <-done.done:
			a := done.result
			t.Fatalf("writer %d completed before its required index wait: %d %s (%v)", w.pid, a.status, a.body, a.err)
		default:
		}
		var waiting, held bool
		err := f.tm.pool.QueryRow(ctx, `SELECT
			EXISTS(SELECT 1 FROM pg_locks l WHERE l.pid=$1 AND l.locktype='advisory' AND NOT l.granted
			 AND l.database=(SELECT oid FROM pg_database WHERE datname=current_database())
			 AND l.classid::bigint=$3 AND l.objid::bigint=$4 AND l.objsubid=2),
			EXISTS(SELECT 1 FROM pg_locks l WHERE l.pid=$2 AND l.locktype='advisory' AND l.granted
			 AND l.database=(SELECT oid FROM pg_database WHERE datname=current_database())
			 AND l.classid::bigint=$3 AND l.objid::bigint=$4 AND l.objsubid=2)`,
			w.pid, holderPID, int64(uint32(space)), int64(uint32(key))).Scan(&waiting, &held)
		if err != nil {
			t.Fatalf("observe exact index wait: %v (a deadline is NOT a successful reverse control)", err)
		}
		if waiting && held {
			return
		}
		runtime.Gosched()
	}
}

func ingestCoverageEntryOf(t *testing.T, u *ingestInterleaveUnit, w ingestWriteSeen, done *ingestPending) {
	t.Helper()
	e := ingestNext(t, "coverage before its first production query", u.beforeCoverage, done)
	if e.err != nil || e.pid != w.pid || !e.holdsIndex {
		t.Fatalf("U01 coverage does not hold its real notebook index lock: entry=%+v writer=%+v", e, w)
	}
}

// The observer already read the new content in B's own transaction. NOWAIT
// independently proves that content row is locked; it does not manufacture a
// wait, change data, or put another content lock inside A's transaction.
func ingestAssertWriterLocks(t *testing.T, f ingestInterleaveFixture, nodes ...uuid.UUID) {
	t.Helper()
	probe := func(sql string, id any, blocked bool) {
		t.Helper()
		tx, err := f.tm.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		_, err = tx.Exec(t.Context(), sql, id)
		if blocked {
			var pg *pgconn.PgError
			if !errors.As(err, &pg) || pg.Code != "55P03" {
				t.Fatalf("NOWAIT content/notebook lock proof = %v, want SQLSTATE 55P03", err)
			}
		} else if err != nil {
			t.Fatalf("ordinary writer did not permit concurrent notebook FOR SHARE: %v", err)
		}
	}
	probe(`SELECT 1 FROM notebooks WHERE id=$1 FOR SHARE NOWAIT`, f.nb, false)
	probe(`SELECT 1 FROM notebooks WHERE id=$1 FOR NO KEY UPDATE NOWAIT`, f.nb, true)
	for _, node := range nodes {
		probe(`SELECT 1 FROM page_contents WHERE node_id=$1 FOR NO KEY UPDATE NOWAIT`, node, true)
	}
}

// A production content-lock regression either exposes the concrete A->B and
// B->A PG wait graph here, or returns PostgreSQL's explicit deadlock SQLSTATE.
// A request/observation timeout never counts as that evidence.
func ingestReadWhilePeerWaits(t *testing.T, f ingestInterleaveFixture, a, b ingestWriteSeen, u *ingestInterleaveUnit, done *ingestPending) ingestCoverageSeen {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), interleavingWait)
	defer cancel()
	space, key := ingestIndexKey(uuid.MustParse(f.nb))
	for {
		select {
		case seen := <-u.coverageRead:
			var pg *pgconn.PgError
			if errors.As(seen.err, &pg) && pg.Code == "40P01" {
				t.Fatalf("U02 content lock cycle: A pid=%d held index while B pid=%d held Q and waited for index; PostgreSQL returned 40P01: %v", a.pid, b.pid, seen.err)
			}
			return seen
		case <-done.done:
			select {
			case seen := <-u.coverageRead:
				return seen
			default:
				got := done.result
				t.Fatalf("U02 A ended without completing real coverage: %d %s (%v); not a deadlock oracle by itself", got.status, got.body, got.err)
			}
		default:
		}
		var aWaitsForB, bWaitsForA, contentWait, aIndexHeld, bIndexWait bool
		err := f.tm.pool.QueryRow(ctx, `WITH index_locks AS (
			SELECT pid,granted FROM pg_locks WHERE locktype='advisory'
			AND database=(SELECT oid FROM pg_database WHERE datname=current_database())
			AND classid::bigint=$3 AND objid::bigint=$4 AND objsubid=2
		) SELECT
			$2::integer=ANY(pg_blocking_pids($1::integer)),
			$1::integer=ANY(pg_blocking_pids($2::integer)),
			EXISTS(SELECT 1 FROM pg_locks l WHERE l.pid=$1 AND
			 ((l.locktype='tuple' AND l.relation='page_contents'::regclass)
			 OR (l.locktype='transactionid' AND NOT l.granted))),
			EXISTS(SELECT 1 FROM index_locks WHERE pid=$1 AND granted),
			EXISTS(SELECT 1 FROM index_locks WHERE pid=$2 AND NOT granted)`,
			a.pid, b.pid, int64(uint32(space)), int64(uint32(key))).
			Scan(&aWaitsForB, &bWaitsForA, &contentWait, &aIndexHeld, &bIndexWait)
		if err != nil {
			t.Fatalf("U02 lock observer failed: %v (NOT a successful reverse control)", err)
		}
		if aWaitsForB && bWaitsForA && contentWait && aIndexHeld && bIndexWait {
			t.Fatalf("U02 forbidden lock cycle observed in PostgreSQL: A pid=%d -> B pid=%d content; B -> A notebook index", a.pid, b.pid)
		}
		runtime.Gosched()
	}
}

func ingestCheckRead(t *testing.T, seen ingestCoverageSeen, content string, revision int) {
	t.Helper()
	var pg *pgconn.PgError
	if errors.As(seen.err, &pg) && pg.Code == "40P01" {
		t.Fatalf("U02 real coverage returned explicit PostgreSQL deadlock 40P01: %v", seen.err)
	}
	if seen.err != nil || !seen.metadataFound || seen.metadataRevision != revision || seen.peerContent != content ||
		seen.peerRevision != revision || seen.indexedRevision != revision || seen.extractor != 2 {
		t.Fatalf("U02 committed content/head pair = %+v; want body=%q metadata/content/index r%d, extractor=2", seen, content, revision)
	}
}

func ingestAnswer(t *testing.T, f ingestInterleaveFixture, u *ingestInterleaveUnit, done *ingestPending, status int) {
	t.Helper()
	var got answer
	select {
	case <-done.done:
		got = done.result
	case <-time.After(interleavingWait):
		t.Fatal("HTTP completion deadline (NOT a successful reverse control)")
	}
	if got.err != nil || got.res == nil || got.status != status {
		// PostgreSQL may choose the INDEX waiter as its deadlock victim.
		// Preserve that real adapter cause instead of counting an opaque 500
		// or a deadline as the U02 reverse-control oracle.
		select {
		case err := <-u.observerResult:
			var pg *pgconn.PgError
			if errors.As(err, &pg) && pg.Code == "40P01" {
				t.Fatalf("U02 real write observer for %s returned PostgreSQL deadlock 40P01: %v", u.node, err)
			}
		default:
		}
		t.Fatalf("write = %d %s (%v), want %d", got.status, got.body, got.err, status)
	}
	f.tm.contract.CheckResponse(t, got.req, got.res)
}

func ingestCheckCommitted(t *testing.T, f ingestInterleaveFixture, u *ingestInterleaveUnit, w ingestWriteSeen, wantIDs ...string) {
	t.Helper()
	if got := f.tm.content(t, "alice", u.node.String()); got.Content != u.body || got.Revision != u.revision {
		t.Fatalf("committed content = %+v, want body %q r%d", got, u.body, u.revision)
	}
	if got := queryStrings(t, f.tm.pool, `SELECT r.changeset_id::text FROM page_revisions r
		JOIN changesets c ON c.id=r.changeset_id WHERE r.node_id=$1 AND r.revision=$2
		AND r.base_revision=$3 AND r.content=$4 AND c.notebook_id=$5
		AND c.kind='edit' AND c.deleted_at IS NULL AND NOT c.trimmed`,
		u.node, u.revision, u.revision-1, u.body, f.nb); !slices.Equal(got, []string{w.cs.String()}) {
		t.Fatalf("exact revision's changeset = %v, want observer's own %s", got, w.cs)
	}
	wantIDs = slices.Clone(wantIDs)
	slices.Sort(wantIDs)
	want := make([]string, len(wantIDs))
	for i, id := range wantIDs {
		want[i] = fmt.Sprintf("%d:%s", i+1, id)
	}
	if got := queryStrings(t, f.tm.pool, `SELECT seq::text||':'||node_id::text FROM changeset_sources WHERE changeset_id=$1 ORDER BY seq`, w.cs); !slices.Equal(got, want) {
		t.Fatalf("unit %s producer marks = %v, want exact %v", u.node, got, want)
	}
	if n := count(t, f.tm.pool, `SELECT ingest_count FROM changesets WHERE id=$1`, w.cs); n != len(wantIDs) {
		t.Fatalf("unit %s ingest_count=%d, want %d", u.node, n, len(wantIDs))
	}
}

func ingestInterleaveInvariants(t *testing.T, f ingestInterleaveFixture) {
	t.Helper()
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
	if n := count(t, f.tm.pool, sourceHashViolationSQL); n != 0 {
		t.Errorf("source_hash SQL invariant violations=%d", n)
	}
	if bad := sourceHashRebuildMismatches(t, t.Context(), f.tm.pool); len(bad) != 0 {
		t.Errorf("source_hash independent rebuild: %v", bad)
	}
	if bad := ingestViolations(t, f.tm.pool, nil); len(bad) != 0 {
		t.Errorf("ingest invariants: %v", bad)
	}
}

func ingestCheckTotals(t *testing.T, f ingestInterleaveFixture, changesets, marks int) {
	t.Helper()
	if n := count(t, f.tm.pool, `SELECT count(*) FROM changesets WHERE notebook_id=$1`, f.nb); n != f.initialChangesets+changesets {
		t.Errorf("new changesets=%d, want %d", n-f.initialChangesets, changesets)
	}
	if n := count(t, f.tm.pool, `SELECT count(*) FROM changeset_sources s JOIN changesets c ON c.id=s.changeset_id WHERE c.notebook_id=$1`, f.nb); n != f.initialMarks+marks {
		t.Errorf("new marks=%d, want %d", n-f.initialMarks, marks)
	}
	ingestInterleaveInvariants(t, f)
}

// All committed state is still the fixture while B is held before coverage
// and A has rolled back. This compares actual old rows, including timestamps,
// versions, items, all link-index relations, and source hashes.
func ingestCommittedState(t *testing.T, f ingestInterleaveFixture) []string {
	t.Helper()
	got := queryStrings(t, f.tm.pool, `SELECT 'nodes:'||to_jsonb(n)::text FROM nodes n WHERE notebook_id=$1
		UNION ALL SELECT 'content:'||to_jsonb(c)::text FROM page_contents c JOIN nodes n ON n.id=c.node_id WHERE n.notebook_id=$1
		UNION ALL SELECT 'version:'||to_jsonb(r)::text FROM page_revisions r JOIN nodes n ON n.id=r.node_id WHERE n.notebook_id=$1
		UNION ALL SELECT 'cs:'||to_jsonb(c)::text FROM changesets c WHERE notebook_id=$1
		UNION ALL SELECT 'item:'||to_jsonb(i)::text FROM changeset_items i JOIN changesets c ON c.id=i.changeset_id WHERE c.notebook_id=$1
		UNION ALL SELECT 'mark:'||to_jsonb(s)::text FROM changeset_sources s JOIN changesets c ON c.id=s.changeset_id WHERE c.notebook_id=$1
		UNION ALL SELECT 'queue:'||to_jsonb(q)::text FROM page_search_queue q WHERE q.notebook_id=$1 ORDER BY 1`, f.nb)
	return append(got, f.tm.originIndex(t, f.nb)...)
}

// A committed marker closes a real LISTEN/NOTIFY observation window. There is
// no sleep and no "no event before timeout" assertion. The listener is opened
// after API fixture creation and before either write begins.
func ingestNotifications(t *testing.T, f ingestInterleaveFixture, conn *pgx.Conn) []json.RawMessage {
	t.Helper()
	marker := "ingest-interleaving-end-" + uuid.NewV7().String()
	if _, err := f.tm.pool.Exec(t.Context(), `SELECT pg_notify('nwiki_events',$1)`, marker); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), interleavingWait)
	defer cancel()
	var out []json.RawMessage
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n.Payload == marker {
			return out
		}
		out = append(out, json.RawMessage(n.Payload))
	}
}

func ingestCommittedEvents(t *testing.T, f ingestInterleaveFixture, conn *pgx.Conn, node string) {
	t.Helper()
	events := ingestNotifications(t, f, conn)
	if len(events) != 2 {
		t.Fatalf("successful B notifications=%s, want one pages and one links event", events)
	}
	pages, links := 0, 0
	for _, raw := range events {
		var event struct {
			Type       string `json:"type"`
			NotebookID string `json:"notebook_id"`
			Data       struct {
				Tree  bool `json:"tree"`
				Pages []struct {
					ID       string `json:"id"`
					Revision int    `json:"revision"`
				} `json:"pages"`
				Targets []string `json:"targets"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &event); err != nil || event.NotebookID != f.nb {
			t.Fatalf("committed notification=%s (%v)", raw, err)
		}
		switch event.Type {
		case "pages":
			pages++
			if event.Data.Tree || len(event.Data.Pages) != 1 || event.Data.Pages[0].ID != node || event.Data.Pages[0].Revision != 2 {
				t.Fatalf("B pages event=%s; want only B's r2", raw)
			}
		case "links":
			links++
			if !slices.Equal(event.Data.Targets, []string{f.s}) {
				t.Fatalf("B links event=%s; want exact source S target", raw)
			}
		default:
			t.Fatalf("unexpected committed event=%s", raw)
		}
	}
	if pages != 1 || links != 1 {
		t.Fatalf("committed event types: pages=%d links=%d", pages, links)
	}
}

// I1/U01, with the I1/U09 rollback leg nested here. There is intentionally no
// second TestIngestUnit declaration and no SQL-created consumption fixture.
func testIngestIndexSerial(t *testing.T) {
	for _, tc := range []struct {
		name          string
		qFirst, fault bool
	}{{"P_then_Q", false, false}, {"Q_then_P", true, false}, {"marks_rollback_P_then_Q", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			control := &ingestInterleaveControl{}
			f := newIngestInterleaveFixture(t, control, "", false)
			if f.initialMarks != 0 || count(t, f.tm.pool, `SELECT count(*) FROM page_links WHERE notebook_id=$1 AND resolved_id=$2`, f.nb, f.s) != 0 {
				t.Fatal("I1 fixture already contains consumption of S")
			}
			firstNode, secondNode := f.p, f.q
			if tc.qFirst {
				firstNode, secondNode = secondNode, firstNode
			}
			a := newIngestInterleaveUnit(t, firstNode, secondNode, "[[raw/S]]\n")
			b := newIngestInterleaveUnit(t, secondNode, firstNode, "[[raw/S]]\n")
			a.failAfterRealAdd = tc.fault
			control.arm(a, b)
			before := ingestCommittedState(t, f)
			listener := f.tm.originListen(t)
			holder, holderPID := ingestHoldIndex(t, f)
			aDone := ingestStartWrite(t, f, a)
			aw := ingestWritten(t, f, a, aDone)
			ingestWaitIndex(t, f, a, aw, holderPID, aDone)
			bDone := ingestStartWrite(t, f, b)
			bw := ingestWritten(t, f, b, bDone)
			if aw.pid == bw.pid || aw.cs == bw.cs {
				t.Fatal("I1 did not create two distinct live write transactions and changesets")
			}
			ingestWaitIndex(t, f, b, bw, holderPID, bDone)
			space, key := ingestIndexKey(uuid.MustParse(f.nb))
			pgtest.WaitForAdvisoryLockWaits(t, f.tm.pool, space, key, 2, interleavingWait)
			ingestAssertWriterLocks(t, f, a.node, b.node)
			if err := holder.Commit(t.Context()); err != nil {
				t.Fatal(err)
			}
			// Queue A first, then explicitly observe A owning the actual index
			// while B still waits. B's later coverage gate keeps B uncommitted
			// until A's real commit/rollback has been independently checked.
			ingestCoverageEntryOf(t, a, aw, aDone)
			ingestWaitIndex(t, f, b, bw, aw.pid, bDone)
			a.release()
			ingestCheckRead(t, ingestReadWhilePeerWaits(t, f, aw, bw, a, aDone), "", 1)
			mark := ingestNext(t, "A's real Add", a.marks, aDone)
			if mark.err != nil || mark.cs != aw.cs || !slices.Equal(mark.ids, []uuid.UUID{uuid.MustParse(f.s)}) ||
				mark.result.Added != 1 || mark.result.Count != 1 || mark.stored != 1 {
				t.Fatalf("A did not actually write one same-transaction S mark before finishing: %+v", mark)
			}
			wantStatus := http.StatusOK
			if tc.fault {
				wantStatus = http.StatusInternalServerError
			}
			ingestAnswer(t, f, a, aDone, wantStatus)
			ingestCoverageEntryOf(t, b, bw, bDone)
			observerErr := ingestNext(t, "A observer result", a.observerResult, nil)
			if tc.fault {
				var marker ingestAfterMarksFault
				if !errors.As(observerErr, &marker) {
					t.Fatalf("A observer error=%v, want the specific after-real-Add failure", observerErr)
				}
				if got := ingestCommittedState(t, f); !slices.Equal(got, before) {
					t.Fatalf("U09 A rollback changed committed content/version/index/changeset/marks/search rows: got=%v want=%v", got, before)
				}
				if n := count(t, f.tm.pool, `SELECT count(*) FROM changesets WHERE id=$1`, aw.cs); n != 0 {
					t.Fatalf("A's aborted changeset persisted: %s", aw.cs)
				}
				if got := ingestNotifications(t, f, listener); len(got) != 0 {
					t.Fatalf("U09 failed A published committed events before B was released: %s", got)
				}
			} else {
				if observerErr != nil {
					t.Fatal(observerErr)
				}
				ingestCheckCommitted(t, f, a, aw, f.s)
			}
			b.release()
			bRead := ingestNext(t, "B's actual coverage", b.coverageRead, bDone)
			if tc.fault {
				ingestCheckRead(t, bRead, "", 1)
			} else {
				ingestCheckRead(t, bRead, a.body, 2)
			}
			ingestAnswer(t, f, b, bDone, http.StatusOK)
			if tc.fault {
				ingestCheckCommitted(t, f, b, bw, f.s)
				f.tm.resolves(t, a.node.String())
				ingestCommittedEvents(t, f, listener, b.node.String())
				ingestCheckTotals(t, f, 1, 1)
			} else {
				ingestCheckCommitted(t, f, b, bw)
				f.tm.resolves(t, a.node.String(), f.s)
				ingestCheckTotals(t, f, 2, 1)
			}
			f.tm.resolves(t, b.node.String(), f.s)
		})
	}
}

// I2/U02. P always adds S and independent T. The hand-written oracle depends
// on Q's committed facts, never the metadata/qualification algorithm itself:
//
//	Q old -> new       A first (A; B)          B first (B; A)
//	S     -> T        [T]; []                 [T]; [S]
//	none  -> S        [S,T]; []               [S]; [T]
//
// Q's initial API-created history, including any initial mark, is retained and
// counted separately; it is not deleted or manufactured by SQL.
func testIngestContentWait(t *testing.T) {
	for _, shape := range []struct {
		name, oldQ, newQ string
		oldHasS          bool
	}{{"S_to_T", "[[raw/S]]\n", "[[raw/T]]\n", true},
		{"none_to_S", "Q before, without a source reference.\n", "[[raw/S]]\n", false}} {
		for _, aFirst := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/A_first_%v", shape.name, aFirst), func(t *testing.T) {
				control := &ingestInterleaveControl{}
				f := newIngestInterleaveFixture(t, control, shape.oldQ, true)
				f.tm.resolves(t, f.p)
				if shape.oldHasS {
					f.tm.resolves(t, f.q, f.s)
				} else {
					f.tm.resolves(t, f.q)
				}
				if n := count(t, f.tm.pool, `SELECT count(*) FROM page_links WHERE notebook_id=$1 AND resolved_id=$2`, f.nb, f.t); n != 0 {
					t.Fatalf("independent T already has %d references", n)
				}
				a := newIngestInterleaveUnit(t, f.p, f.q, "[[raw/S]]\n[[raw/T]]\n")
				b := newIngestInterleaveUnit(t, f.q, f.p, shape.newQ)
				control.arm(a, b)
				var aw, bw ingestWriteSeen
				if aFirst {
					aDone := ingestStartWrite(t, f, a)
					aw = ingestWritten(t, f, a, aDone)
					ingestCoverageEntryOf(t, a, aw, aDone)
					bDone := ingestStartWrite(t, f, b)
					bw = ingestWritten(t, f, b, bDone)
					ingestWaitIndex(t, f, b, bw, aw.pid, bDone)
					ingestAssertWriterLocks(t, f, a.node, b.node)
					// B's same-transaction probe saw NEW Q at r2, but its real
					// index update is still waiting. A must observe OLD Q/r1.
					if got := f.tm.content(t, "alice", f.q); got.Content != shape.oldQ || got.Revision != 1 {
						t.Fatalf("external committed Q changed while B waited: %+v", got)
					}
					a.release()
					ingestCheckRead(t, ingestReadWhilePeerWaits(t, f, aw, bw, a, aDone), shape.oldQ, 1)
					ingestAnswer(t, f, a, aDone, http.StatusOK)
					ingestCoverageEntryOf(t, b, bw, bDone)
					if shape.oldHasS {
						ingestCheckCommitted(t, f, a, aw, f.t)
					} else {
						ingestCheckCommitted(t, f, a, aw, f.s, f.t)
					}
					b.release()
					ingestCheckRead(t, ingestNext(t, "B coverage after A commit", b.coverageRead, bDone), a.body, 2)
					ingestAnswer(t, f, b, bDone, http.StatusOK)
					ingestCheckCommitted(t, f, b, bw)
				} else {
					// B takes index first. A has already made its own content
					// write and waits at that index before B is released.
					bDone := ingestStartWrite(t, f, b)
					bw = ingestWritten(t, f, b, bDone)
					ingestCoverageEntryOf(t, b, bw, bDone)
					aDone := ingestStartWrite(t, f, a)
					aw = ingestWritten(t, f, a, aDone)
					ingestWaitIndex(t, f, a, aw, bw.pid, aDone)
					ingestAssertWriterLocks(t, f, a.node, b.node)
					b.release()
					ingestCheckRead(t, ingestReadWhilePeerWaits(t, f, bw, aw, b, bDone), "", 1)
					ingestAnswer(t, f, b, bDone, http.StatusOK)
					ingestCoverageEntryOf(t, a, aw, aDone)
					if shape.oldHasS {
						ingestCheckCommitted(t, f, b, bw, f.t)
					} else {
						ingestCheckCommitted(t, f, b, bw, f.s)
					}
					a.release()
					ingestCheckRead(t, ingestNext(t, "A coverage after B commit", a.coverageRead, aDone), shape.newQ, 2)
					ingestAnswer(t, f, a, aDone, http.StatusOK)
					if shape.oldHasS {
						ingestCheckCommitted(t, f, a, aw, f.s)
					} else {
						ingestCheckCommitted(t, f, a, aw, f.t)
					}
				}
				if aw.pid == bw.pid || aw.cs == bw.cs {
					t.Fatal("I2 did not run two distinct write transactions")
				}
				f.tm.resolves(t, f.p, f.s, f.t)
				if shape.oldHasS {
					f.tm.resolves(t, f.q, f.t)
				} else {
					f.tm.resolves(t, f.q, f.s)
				}
				marks := 2
				if aFirst && shape.oldHasS {
					marks = 1
				}
				ingestCheckTotals(t, f, 2, marks)
			})
		}
	}
}

// The wrappers retain the public production ports.
var (
	_ page.PageObserver      = ingestInterleaveObserver{}
	_ linking.IngestMetadata = ingestInterleaveMetadata{}
	_ linking.IngestMarks    = ingestInterleaveMarks{}
)
