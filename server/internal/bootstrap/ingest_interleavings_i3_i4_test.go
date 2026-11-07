package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Only the original index Store.Lock and Metadata.Heads are wrapped. Store's
// other methods, the original bridge, ingest dependencies, and Writer remain
// those actually wired by serve. The app import is test-only and supplies the
// existing Store interface and real QualifiedBefore DTO, not an implementation
// or a fabricated capture answer.
type ingestI34Control struct {
	mu       sync.Mutex
	units    map[uuid.UUID]*ingestI34Unit
	bookGate *ingestI34BookGate
}

type ingestI34Unit struct {
	base          *ingestInterleaveUnit
	source        uuid.UUID
	inspect       []uuid.UUID
	stopIndex     bool
	extraCapture  bool
	pauseBatch    int
	batchNumber   int // one actual observer goroutine owns this counter
	indexEntry    chan ingestI34IndexSeen
	capture       chan ingestI34CaptureSeen
	batches       chan ingestI34BatchSeen
	allowIndex    chan struct{}
	allowCoverage chan struct{}
	indexOnce     sync.Once
	coverageOnce  sync.Once
}

type ingestI34IndexSeen struct {
	pid                 int32
	hasIndex, hasSource bool
	sourceRevision      int
	sourceBytes         int64
	before              []page.IngestNodeBefore
	err                 error
}

type ingestI34CaptureSeen struct {
	pid       int32
	indexHeld bool
	w         []uuid.UUID
	before    []page.IngestNodeBefore
	direct    []linkingapp.OldQualifiedFacts
	followed  []linkingapp.OldSourceFacts
	err       error
}

type ingestI34BatchSeen struct {
	number int
	after  *uuid.UUID
	batch  linking.IngestHeadBatch
	pid    int32
	err    error
}

type ingestI34BookGate struct {
	notebook, user uuid.UUID
	entered        chan int32
	resume         chan struct{}
	used           bool // protected by the control's mutex
	once           sync.Once
}

type ingestI34Event struct {
	u     *ingestI34Unit
	event page.Event
}
type ingestI34ContextKey struct{}

func newIngestI34Unit(t *testing.T, node, peer, source, body string) *ingestI34Unit {
	t.Helper()
	u := &ingestI34Unit{base: newIngestInterleaveUnit(t, node, peer, body), source: uuid.MustParse(source),
		indexEntry: make(chan ingestI34IndexSeen, 1), capture: make(chan ingestI34CaptureSeen, 1),
		batches: make(chan ingestI34BatchSeen, 8), allowIndex: make(chan struct{}), allowCoverage: make(chan struct{})}
	t.Cleanup(u.releaseIndex)
	t.Cleanup(u.releaseCoverage)
	return u
}

func (u *ingestI34Unit) releaseIndex()    { u.indexOnce.Do(func() { close(u.allowIndex) }) }
func (u *ingestI34Unit) releaseCoverage() { u.coverageOnce.Do(func() { close(u.allowCoverage) }) }

func (c *ingestI34Control) arm(units ...*ingestI34Unit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.units = map[uuid.UUID]*ingestI34Unit{}
	for _, u := range units {
		c.units[u.base.node] = u
	}
}

func (c *ingestI34Control) wiring(t *testing.T) wiring {
	t.Helper()
	return wiring{page: func(d *page.Deps) {
		d.Notebooks = ingestI34Notebooks{Notebooks: d.Notebooks, control: c}
		found := 0
		for i, o := range d.Observers {
			original, ok := o.(linkIndex)
			if !ok {
				continue
			}
			found++
			if original.index.Ingest == nil {
				t.Fatal("real index lacks ingest")
			}
			deps := *original.index.Ingest
			// Capture reuses exactly these real query ports. It is an EXTRA
			// observation before normal beginIngest, not its replacement.
			capture := linkingapp.QualifiedBefore{Heads: deps.Heads, Reads: deps.Reads}
			original.index.Store = ingestI34Store{Store: original.index.Store, capture: capture}
			deps.Metadata = ingestI34Metadata{real: deps.Metadata}
			original.index.Ingest = &deps
			d.Observers[i] = ingestI34Observer{real: original, control: c}
		}
		if found != 1 {
			t.Fatalf("real index observers=%d, want 1", found)
		}
	}}
}

type ingestI34Notebooks struct {
	page.Notebooks
	control *ingestI34Control
}

func (n ingestI34Notebooks) LockByID(ctx context.Context, id uuid.UUID) (page.NotebookState, bool, error) {
	actor, actorErr := shared.RequireActor(ctx)
	n.control.mu.Lock()
	g := n.control.bookGate
	intercept := actorErr == nil && g != nil && !g.used && g.notebook == id && g.user == actor.UserID
	if intercept {
		g.used = true
	}
	n.control.mu.Unlock()
	if intercept {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			return page.NotebookState{}, false, errors.New("I4 pre-lock gate has no transaction")
		}
		g.entered <- int32(tx.Conn().PgConn().PID())
		select {
		case <-g.resume:
		case <-ctx.Done():
			return page.NotebookState{}, false, ctx.Err()
		}
	}
	return n.Notebooks.LockByID(ctx, id)
}

type ingestI34Observer struct {
	real    page.PageObserver
	control *ingestI34Control
}

func (o ingestI34Observer) PagesChanged(ctx context.Context, e page.Event) error {
	o.control.mu.Lock()
	var unit *ingestI34Unit
	for _, change := range e.Changes {
		if u := o.control.units[change.NodeID]; u != nil {
			if unit != nil {
				o.control.mu.Unlock()
				return errors.New("I3/I4 event matches two primary probes")
			}
			unit = u
		}
	}
	o.control.mu.Unlock()
	if unit == nil {
		return o.real.PagesChanged(ctx, e)
	}
	w := ingestWriteSeen{notebook: e.NotebookID, cs: e.ChangesetID, tree: e.Tree, kind: string(e.Kind)}
	tx, ok := postgres.TxFrom(ctx)
	if !ok || e.BeforeIngest == nil {
		w.err = errors.New("I3/I4 real observer lost tx or BeforeIngest")
	} else {
		w.pid = int32(tx.Conn().PgConn().PID())
		w.err = tx.QueryRow(ctx, `SELECT content,revision,current_setting('transaction_isolation') FROM page_contents WHERE node_id=$1`, unit.base.node).
			Scan(&w.body, &w.revision, &w.iso)
	}
	unit.base.written <- w
	if w.err != nil {
		unit.base.observerResult <- w.err
		return w.err
	}
	err := o.real.PagesChanged(context.WithValue(ctx, ingestI34ContextKey{}, ingestI34Event{u: unit, event: e}), e)
	unit.base.observerResult <- err
	return err
}

type ingestI34Store struct {
	linkingapp.Store
	capture linkingapp.QualifiedBefore
}

func (s ingestI34Store) Lock(ctx context.Context, notebook uuid.UUID) error {
	probe, observed := ctx.Value(ingestI34ContextKey{}).(ingestI34Event)
	if !observed {
		return s.Store.Lock(ctx, notebook)
	}
	u := probe.u
	if u.stopIndex {
		seen := ingestI34IndexSeen{}
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			return errors.New("I3 index gate has no transaction")
		}
		seen.pid = int32(tx.Conn().PgConn().PID())
		space, key := ingestIndexKey(notebook)
		seen.err = tx.QueryRow(ctx, `SELECT
			EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory' AND granted
			 AND classid::bigint=$1 AND objid::bigint=$2 AND objsubid=2),
			EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory' AND granted
			 AND classid::bigint=$3 AND objid::bigint=$4 AND objsubid=2),
			c.revision,c.byte_size FROM page_contents c WHERE c.node_id=$5`,
			int64(uint32(space)), int64(uint32(key)), int64(0x73726365), int64(uint32(page.SourceLockKey(notebook))), u.source).
			Scan(&seen.hasIndex, &seen.hasSource, &seen.sourceRevision, &seen.sourceBytes)
		if seen.err == nil {
			seen.before, seen.err = probe.event.BeforeIngest.NodesBefore(ctx, u.inspect)
		}
		u.indexEntry <- seen
		if seen.err != nil {
			return seen.err
		}
		select {
		case <-u.allowIndex:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := s.Store.Lock(ctx, notebook); err != nil {
		return err
	}
	if !u.extraCapture {
		return nil
	}
	seen := ingestI34CaptureSeen{}
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		return errors.New("I3/I4 extra Capture has no transaction")
	}
	seen.pid = int32(tx.Conn().PgConn().PID())
	space, key := ingestIndexKey(notebook)
	seen.err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid()
		AND locktype='advisory' AND granted AND classid::bigint=$1 AND objid::bigint=$2 AND objsubid=2)`,
		int64(uint32(space)), int64(uint32(key))).Scan(&seen.indexHeld)
	for _, c := range probe.event.Changes {
		if c.After != nil && c.Revision != 0 || c.Before != nil && c.After == nil {
			seen.w = append(seen.w, c.NodeID)
		}
	}
	slices.SortFunc(seen.w, uuid.UUID.Compare)
	seen.w = slices.Compact(seen.w)
	if seen.err == nil {
		seen.before, seen.err = probe.event.BeforeIngest.NodesBefore(ctx, u.inspect)
	}
	if seen.err == nil {
		seen.err = s.capture.Capture(ctx, notebook, linkIngestBefore{probe.event.BeforeIngest}, seen.w,
			func(f linkingapp.OldQualifiedFacts) error {
				f.References = slices.Clone(f.References)
				f.Unknown.PossibleTargets = slices.Clone(f.Unknown.PossibleTargets)
				seen.direct = append(seen.direct, f)
				return nil
			}, func(f linkingapp.OldSourceFacts) error {
				f.AssetIDs = slices.Clone(f.AssetIDs)
				f.Unknown.PossibleTargets = slices.Clone(f.Unknown.PossibleTargets)
				seen.followed = append(seen.followed, f)
				return nil
			})
	}
	u.capture <- seen
	return seen.err // normal Index.beginIngest still executes its own real Capture
}

type ingestI34Metadata struct{ real linking.IngestMetadata }

func (m ingestI34Metadata) Now(ctx context.Context, nb, root uuid.UUID, ids []uuid.UUID) ([]linking.IngestHead, error) {
	return m.real.Now(ctx, nb, root, ids)
}

func (m ingestI34Metadata) Heads(ctx context.Context, nb, root uuid.UUID, after *uuid.UUID, take int) (linking.IngestHeadBatch, error) {
	batch, err := m.real.Heads(ctx, nb, root, after, take)
	probe, ok := ctx.Value(ingestI34ContextKey{}).(ingestI34Event)
	if !ok || probe.u.pauseBatch == 0 {
		return batch, err
	}
	u := probe.u
	u.batchNumber++
	seen := ingestI34BatchSeen{number: u.batchNumber, batch: batch, err: err}
	seen.batch.Rows = slices.Clone(batch.Rows)
	if after != nil {
		v := *after
		seen.after = &v
	}
	if batch.Last != nil {
		v := *batch.Last
		seen.batch.Last = &v
	}
	if tx, ok := postgres.TxFrom(ctx); ok {
		seen.pid = int32(tx.Conn().PgConn().PID())
	}
	u.batches <- seen
	if err == nil && u.batchNumber == u.pauseBatch {
		select {
		case <-u.allowCoverage:
		case <-ctx.Done():
			return linking.IngestHeadBatch{}, ctx.Err()
		}
	}
	return batch, err
}

func ingestI34Start(t *testing.T, tm acmeTeam, action step) *ingestPending {
	t.Helper()
	send := tm.sender(t, action)
	p := &ingestPending{done: make(chan struct{})}
	go func() { p.result = send(); close(p.done) }()
	return p
}

func ingestI34Answer(t *testing.T, tm acmeTeam, pending *ingestPending, status int, code string) answer {
	t.Helper()
	select {
	case <-pending.done:
	case <-time.After(interleavingWait):
		t.Fatal("I3/I4 request/command deadline; NOT a reverse-control success")
	}
	a := pending.result
	if a.err != nil {
		t.Fatal(a.err)
	}
	if status != 0 {
		if a.res == nil || !a.is(status, code) {
			t.Fatalf("I3/I4 request=%d %s, want %d %s", a.status, a.body, status, code)
		}
		tm.contract.CheckResponse(t, a.req, a.res)
	}
	return a
}

func ingestI34Written(t *testing.T, f ingestInterleaveFixture, u *ingestI34Unit, done *ingestPending, tree bool) ingestWriteSeen {
	t.Helper()
	w := ingestNext(t, "I3/I4 primary content written", u.base.written, done)
	if w.err != nil || w.pid == 0 || w.notebook.String() != f.nb || w.cs == uuid.Nil() || w.tree != tree || w.kind != "edit" ||
		w.body != u.base.body || w.revision != 2 || w.iso != "read committed" {
		t.Fatalf("I3/I4 real primary write=%+v, want Tree=%v own new r2", w, tree)
	}
	return w
}

func ingestI34IDs(ids ...string) []uuid.UUID {
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.MustParse(id)
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return slices.Compact(out)
}

func ingestI34Before(t *testing.T, rows []page.IngestNodeBefore, id string) page.IngestNodeBefore {
	t.Helper()
	for _, row := range rows {
		if row.ID.String() == id {
			return row
		}
	}
	t.Fatalf("Before DTO omitted %s: %+v", id, rows)
	return page.IngestNodeBefore{}
}

func ingestI34Direct(t *testing.T, seen ingestI34CaptureSeen, id string) linkingapp.OldQualifiedFacts {
	t.Helper()
	if seen.err != nil || !seen.indexHeld || seen.pid == 0 {
		t.Fatalf("extra same-tx/index Capture failed: %+v", seen)
	}
	for _, f := range seen.direct {
		if f.PageID.String() == id {
			return f
		}
	}
	t.Fatalf("real Capture omitted D(%s): %+v", id, seen)
	return linkingapp.OldQualifiedFacts{}
}

func newIngestI34Fixture(t *testing.T, c *ingestI34Control, emptyS bool, pBody, qBody string) ingestInterleaveFixture {
	t.Helper()
	tm := newAcmeTeamWiring(t, "member", "", c.wiring(t))
	b := tm.createWith(t, tm.tokens["alice"], `{"name":"I3 I4 evidence","mode":"llm_wiki","workspace_access":"editor"}`)
	if b.Mode != "llm_wiki" || b.SourceRootID == nil {
		t.Fatalf("fixture=%+v", b)
	}
	f := ingestInterleaveFixture{tm: tm, nb: b.ID, root: *b.SourceRootID}
	sBody := "Independent source S.\n"
	if emptyS {
		sBody = ""
	}
	f.s = tm.createPageWith(t, "alice", f.nb, f.root, "S", sBody)
	f.t = tm.createPageWith(t, "alice", f.nb, f.root, "T", "Independent source T.\n")
	f.p = tm.createPageWith(t, "alice", f.nb, "", "P", pBody)
	f.q = tm.createPageWith(t, "alice", f.nb, "", "Q", qBody)
	for _, source := range []struct{ id, body string }{{f.s, sBody}, {f.t, "Independent source T.\n"}} {
		if n := count(t, tm.pool, `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
			JOIN indexed_pages i ON i.node_id=n.id WHERE n.id=$1 AND n.notebook_id=$2 AND n.parent_id=$3
			AND n.deleted_at IS NULL AND c.deleted_at IS NULL AND c.content=$4 AND c.revision=1
			AND c.byte_size=octet_length(convert_to($4,'UTF8')) AND i.revision=1 AND i.extractor=2 AND i.notebook_id=$2
			AND n.source_hash IS NOT DISTINCT FROM CASE WHEN $4::text='' THEN NULL::bytea ELSE sha256(convert_to($4,'UTF8')) END`,
			source.id, f.nb, f.root, source.body); n != 1 {
			t.Fatalf("invalid initial source %s", source.id)
		}
	}
	ingestI34Baseline(t, &f)
	ingestInterleaveInvariants(t, f)
	return f
}

func ingestI34Baseline(t *testing.T, f *ingestInterleaveFixture) {
	t.Helper()
	f.initialChangesets = count(t, f.tm.pool, `SELECT count(*) FROM changesets WHERE notebook_id=$1`, f.nb)
	f.initialMarks = count(t, f.tm.pool, `SELECT count(*) FROM changeset_sources s JOIN changesets c ON c.id=s.changeset_id WHERE c.notebook_id=$1`, f.nb)
}

func testIngestSourceFirstBytes(t *testing.T) {
	const oldP = "[[raw/S]]\n"
	const newP = "[[raw/S]]\n[[raw/T]]\n"
	const firstSourceBody = "The source's first nonempty content.\n"
	c := &ingestI34Control{}
	f := newIngestI34Fixture(t, c, true, oldP, "")
	f.tm.resolves(t, f.p, f.s)
	if f.initialMarks != 0 || count(t, f.tm.pool, `SELECT count(*) FROM page_links WHERE notebook_id=$1 AND resolved_id=$2`, f.nb, f.t) != 0 {
		t.Fatal("I3 requires no old consumption mark and independent unreferenced T")
	}
	a := newIngestI34Unit(t, f.p, f.s, f.s, newP)
	b := newIngestI34Unit(t, f.s, f.p, f.s, firstSourceBody)
	a.stopIndex, a.extraCapture, b.stopIndex = true, true, true
	a.inspect, b.inspect = ingestI34IDs(f.p, f.s), ingestI34IDs(f.s)
	c.arm(a, b)
	aDone := ingestI34Start(t, f.tm, contentWrite("alice", f.p, newP, 1, ""))
	aw := ingestI34Written(t, f, a, aDone, false)
	beforeA := ingestNext(t, "A before index, after real content memo/write", a.indexEntry, aDone)
	pBefore := ingestI34Before(t, beforeA.before, f.p)
	if beforeA.err != nil || beforeA.pid != aw.pid || beforeA.hasIndex || beforeA.hasSource || beforeA.sourceRevision != 1 || beforeA.sourceBytes != 0 ||
		!pBefore.Exists || pBefore.InSource || pBefore.IsSourceRoot || pBefore.IsRootAgents ||
		!pBefore.RevisionKnown || pBefore.Revision != 1 || !pBefore.PageBytesKnown || pBefore.PageBytes != int64(len(oldP)) {
		t.Fatalf("I3 A's real pre-index/first-memo facts=%+v P=%+v", beforeA, pBefore)
	}
	bDone := ingestI34Start(t, f.tm, contentWrite("alice", f.s, firstSourceBody, 1, ""))
	bw := ingestI34Written(t, f, b, bDone, false)
	beforeB := ingestNext(t, "B's first-source write before index", b.indexEntry, bDone)
	oldSource := ingestI34Before(t, beforeB.before, f.s)
	if beforeB.err != nil || beforeB.pid != bw.pid || beforeB.hasIndex || !beforeB.hasSource || beforeB.sourceRevision != 2 || beforeB.sourceBytes != int64(len(firstSourceBody)) || aw.pid == bw.pid {
		t.Fatalf("I3 B did not make a real SHARE first write under source-hash lock: %+v", beforeB)
	}
	if !oldSource.Exists || !oldSource.InSource || !oldSource.PageBytesKnown || oldSource.PageBytes != 0 || !oldSource.RevisionKnown || oldSource.Revision != 1 {
		t.Fatalf("I3 B did not retain its actual empty r1 source memo: %+v", oldSource)
	}
	ingestAssertWriterLocks(t, f, a.base.node, b.base.node)
	b.releaseIndex()
	ingestI34Answer(t, f.tm, bDone, http.StatusOK, "")
	ingestCheckCommitted(t, f, b.base, bw) // a source write itself marks nothing
	if p := f.tm.content(t, "alice", f.p); p.Content != oldP || p.Revision != 1 {
		t.Fatalf("A committed prematurely: %+v", p)
	}
	a.releaseIndex()
	capture := ingestNext(t, "real QualifiedBefore after B committed", a.capture, aDone)
	old := ingestI34Direct(t, capture, f.p)
	sBefore := ingestI34Before(t, capture.before, f.s)
	pBefore = ingestI34Before(t, capture.before, f.p)
	if capture.pid != aw.pid || !slices.Equal(capture.w, []uuid.UUID{a.base.node}) ||
		old.State != linkingapp.OldFactsUnknown || old.Unknown.Extent != linkingapp.OldUnknownPossibleTargets ||
		!slices.Equal(old.Unknown.PossibleTargets, []uuid.UUID{uuid.MustParse(f.s)}) || len(old.References) != 0 {
		t.Fatalf("B09/U03 actual old DTO=%+v W=%v; want Unknown/Possible{S}, zero certain refs", old, capture.w)
	}
	if !sBefore.Exists || !sBefore.InSource || sBefore.IsSourceRoot || sBefore.PageBytesKnown || sBefore.RevisionKnown ||
		sBefore.PageBytes != int64(len(firstSourceBody)) || sBefore.Revision != 2 ||
		!pBefore.PageBytesKnown || pBefore.PageBytes != int64(len(oldP)) || !pBefore.RevisionKnown || pBefore.Revision != 1 {
		t.Fatalf("late target must remain unproven; P retains first locked revision: S=%+v P=%+v", sBefore, pBefore)
	}
	ingestI34Answer(t, f.tm, aDone, http.StatusOK, "")
	ingestCheckCommitted(t, f, a.base, aw, f.t)
	f.tm.resolves(t, f.p, f.s, f.t)
	ingestCheckTotals(t, f, 2, 1)
	// The reverse body transition is NOT an invented concurrency fixture.
	// A real ordinary PUT of nonempty S -> empty must hit source_immutable.
	beforeRefusal := ingestCommittedState(t, f)
	refused := ingestI34Start(t, f.tm, contentWrite("alice", f.s, "", 2, ""))
	ingestI34Answer(t, f.tm, refused, http.StatusConflict, "page.source_immutable")
	if after := ingestCommittedState(t, f); !slices.Equal(after, beforeRefusal) {
		t.Fatal("immutable source refusal changed committed state")
	}
	ingestInterleaveInvariants(t, f)
}

// All filler pages are created by the real MCP batch producer before the
// interleaved requests. No index rows, IDs, history or consumption are seeded
// with SQL. Each call retains the normal 100-op limit and request deadline.
func ingestI34Fill(t *testing.T, f ingestInterleaveFixture, target int) {
	t.Helper()
	m := f.tm.agent(t, "alice", f.nb)
	current := count(t, f.tm.pool, `SELECT count(*) FROM nodes WHERE notebook_id=$1`, f.nb)
	serial := 0
	for current < target {
		n := min(100, target-current)
		ops := make([]map[string]any, n)
		for i := range ops {
			ops[i] = opWrite(fmt.Sprintf("I4 filler %04d.md", serial), "")
			serial++
		}
		done := m.batchDone(t, ops...)
		if !done.Changed || done.ChangesetID == nil || len(done.Results) != n || done.ResultsCount != n {
			t.Fatalf("real filler batch=%+v", done)
		}
		for _, op := range done.Results {
			if !op.Changed || op.NodeID == "" || op.Version == nil || *op.Version != versionOf(op.NodeID, 1) {
				t.Fatalf("real filler op=%+v", op)
			}
		}
		current += n
	}
	if count(t, f.tm.pool, `SELECT count(*) FROM nodes WHERE notebook_id=$1`, f.nb) != target ||
		count(t, f.tm.pool, `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
		JOIN indexed_pages i ON i.node_id=n.id WHERE n.notebook_id=$1 AND n.deleted_at IS NULL
		AND c.deleted_at IS NULL AND i.notebook_id=$1 AND i.revision=c.revision AND i.extractor=2`, f.nb) != target {
		t.Fatal("I4 filler is not the exact healthy current raw-page population")
	}
}

// A real wait on this notebook, with A as a direct blocker. Unlike an arbitrary
// count of waiting backends, this cannot be satisfied by another book/job.
func ingestI34BookWait(t *testing.T, f ingestInterleaveFixture, aPID, expectedB int32, pending *ingestPending) int32 {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), interleavingWait)
	defer cancel()
	space, key := ingestIndexKey(uuid.MustParse(f.nb))
	for {
		select {
		case <-pending.done:
			t.Fatalf("I4 B completed before waiting for A's notebook lock: %+v", pending.result)
		default:
		}
		rows, err := f.tm.pool.Query(ctx, `SELECT DISTINCT p.pid FROM pg_stat_activity p JOIN pg_locks l ON l.pid=p.pid
			WHERE p.datname=current_database() AND p.wait_event_type='Lock'
			AND l.locktype='tuple' AND l.relation='notebooks'::regclass
			AND $1::integer=ANY(pg_blocking_pids(p.pid)) ORDER BY p.pid`, aPID)
		if err != nil {
			t.Fatalf("I4 lock observation failed: %v; timeout is not a kill", err)
		}
		var ids []int32
		for rows.Next() {
			var id int32
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) > 1 {
			t.Fatalf("I4 unexpected extra notebook waiters=%v", ids)
		}
		if len(ids) == 1 {
			if expectedB != 0 && ids[0] != expectedB {
				t.Fatalf("I4 waiter=%d, want actual planned B pid=%d", ids[0], expectedB)
			}
			return ids[0]
		}
		// A missing notebook lock often moves B's wait to index instead of
		// completing B. Capture that precise protocol failure without waiting
		// for A's ordinary request timeout to release the index.
		var wrongIndexWait int
		err = f.tm.pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks l
			WHERE l.locktype='advisory' AND NOT l.granted
			AND l.database=(SELECT oid FROM pg_database WHERE datname=current_database())
			AND l.classid::bigint=$2 AND l.objid::bigint=$3 AND l.objsubid=2
			AND $1::integer=ANY(pg_blocking_pids(l.pid))`, aPID, int64(uint32(space)), int64(uint32(key))).Scan(&wrongIndexWait)
		if err != nil {
			t.Fatal(err)
		}
		if wrongIndexWait != 0 {
			t.Fatalf("I4 B reached A's index while A's coverage was still active; wanted a notebook-row wait, found %d index waiters", wrongIndexWait)
		}
		runtime.Gosched()
	}
}

func ingestI34FirstTwo(t *testing.T, u *ingestI34Unit, pid int32, pending *ingestPending) []ingestI34BatchSeen {
	t.Helper()
	first := ingestNext(t, "I4 first real 512-row coverage batch", u.batches, pending)
	second := ingestNext(t, "I4 second real 512-row coverage batch", u.batches, pending)
	if first.err != nil || second.err != nil || first.pid != pid || second.pid != pid || first.number != 1 || second.number != 2 ||
		first.after != nil || first.batch.Last == nil || second.after == nil || *second.after != *first.batch.Last ||
		len(first.batch.Rows) != 512 || len(second.batch.Rows) != 512 || first.batch.Done || second.batch.Done || second.batch.Last == nil {
		t.Fatalf("I4 did not reach two complete real raw batches: first=%+v second=%+v", first, second)
	}
	return []ingestI34BatchSeen{first, second}
}

func ingestI34FullScan(t *testing.T, u *ingestI34Unit, pending *ingestPending, firstTwo []ingestI34BatchSeen, expected []string) map[uuid.UUID]linking.IngestHead {
	t.Helper()
	last := ingestNext(t, "I4 terminal production coverage batch", u.batches, pending)
	if last.err != nil || last.pid != firstTwo[0].pid || last.number != 3 || last.after == nil || *last.after != *firstTwo[1].batch.Last ||
		len(last.batch.Rows) != 1 || !last.batch.Done || last.batch.Last == nil || *last.batch.Last != last.batch.Rows[0].ID {
		t.Fatalf("I4 expected raw continuation and real EOF after 1025 rows: %+v", last)
	}
	var got []string
	heads := map[uuid.UUID]linking.IngestHead{}
	for _, batch := range append(firstTwo, last) {
		for _, row := range batch.batch.Rows {
			got = append(got, row.ID.String())
			heads[row.ID] = row
		}
	}
	if !slices.Equal(got, expected) || len(heads) != 1025 {
		t.Fatalf("I4 raw scan omitted, duplicated or mixed a concurrent node: count=%d unique=%d expected=%d", len(got), len(heads), len(expected))
	}
	return heads
}

func testIngestTreeCoverage(t *testing.T) {
	for _, scenario := range []string{"create_lower_uuid", "move_Q_into_source", "delete_source_root", "downgrade", "reindex", "same_unit_delete_Q", "same_unit_move_Q_without_content"} {
		t.Run(scenario, func(t *testing.T) {
			c := &ingestI34Control{}
			f := newIngestI34Fixture(t, c, false, "", "[[raw/S]]\n")
			f.tm.resolves(t, f.q, f.s)
			if count(t, f.tm.pool, `SELECT count(*) FROM page_links WHERE notebook_id=$1 AND resolved_id=$2`, f.nb, f.t) != 0 {
				t.Fatal("independent T already referenced")
			}
			target := 1025
			if scenario == "create_lower_uuid" {
				target = 1023
			}
			ingestI34Fill(t, f, target)
			// Build MCP client(s) and reindex configuration outside A's deadline.
			m := f.tm.agent(t, "alice", f.nb)
			bob := f.tm.agent(t, "bob", f.nb)
			cfg := testConfig(t, f.tm.url, false)
			var out, errOut bytes.Buffer
			var aMCP, bMCP mcpAnswer
			var bDone *ingestPending
			var gate *ingestI34BookGate
			var bPID int32
			var lastOldID string
			if scenario == "create_lower_uuid" {
				lastOldID = queryStrings(t, f.tm.pool, `SELECT id::text FROM nodes WHERE notebook_id=$1 ORDER BY id DESC LIMIT 1`, f.nb)[0]
				var user uuid.UUID
				if err := f.tm.pool.QueryRow(t.Context(), `SELECT id FROM users WHERE email='bob@example.com'`).Scan(&user); err != nil {
					t.Fatal(err)
				}
				gate = &ingestI34BookGate{notebook: uuid.MustParse(f.nb), user: user, entered: make(chan int32, 1), resume: make(chan struct{})}
				t.Cleanup(func() { gate.once.Do(func() { close(gate.resume) }) })
				c.mu.Lock()
				c.bookGate = gate
				c.mu.Unlock()
				// Real ApplyBatch.planned allocates IDs before Writer.Run calls
				// LockByID. The path Earlier is not changed by either later page.
				bDone = ingestI34Start(t, f.tm, bob.mcpStep(t, "bob", "batch", batched(opWrite("Earlier.md", "")), &bMCP))
				bPID = ingestNext(t, "B planned, before real notebook lock", gate.entered, bDone)
				f.tm.createPage(t, "alice", f.nb, "", "Later One")
				f.tm.createPage(t, "alice", f.nb, "", "Later Two")
			}
			expected := queryStrings(t, f.tm.pool, `SELECT id::text FROM nodes WHERE notebook_id=$1 ORDER BY id`, f.nb)
			if len(expected) != 1025 {
				t.Fatalf("I4 raw fixture=%d, want 1025", len(expected))
			}
			ingestI34Baseline(t, &f)
			a := newIngestI34Unit(t, f.p, f.q, f.s, "[[raw/S]]\n[[raw/T]]\n")
			a.pauseBatch, a.extraCapture = 2, true
			a.inspect = ingestI34IDs(f.p, f.q, f.s, f.t)
			c.arm(a)
			action := contentWrite("alice", f.p, a.base.body, 1, "")
			tree := false
			switch scenario {
			case "same_unit_delete_Q":
				tree = true
				action = m.mcpStep(t, "alice", "batch", batched(opDelete("Q.md"), opWriteOn("P.md", a.base.body, versionOf(f.p, 1))), &aMCP)
			case "same_unit_move_Q_without_content":
				tree = true
				action = m.mcpStep(t, "alice", "batch", batched(opWith(opMove("Q.md", "raw/Q.md"), "update_links", false), opWriteOn("P.md", a.base.body, versionOf(f.p, 1))), &aMCP)
			}
			aDone := ingestI34Start(t, f.tm, action)
			aw := ingestI34Written(t, f, a, aDone, tree)
			capture := ingestNext(t, "I4 extra real old-fact capture", a.capture, aDone)
			if capture.err != nil || capture.pid != aw.pid || !capture.indexHeld {
				t.Fatalf("I4 Capture did not run in A's real transaction under index: %+v writer=%+v", capture, aw)
			}
			batches := ingestI34FirstTwo(t, a, aw.pid, aDone)
			// Probe compatible SHARE before enqueueing B's conflicting row
			// lock; a later SHARE probe can itself queue behind B's tuple lock.
			if !tree {
				ingestAssertWriterLocks(t, f, a.base.node)
			}
			if scenario == "create_lower_uuid" {
				gate.once.Do(func() { close(gate.resume) })
			} else {
				var other step
				switch scenario {
				case "move_Q_into_source":
					other = nodeMove("alice", f.q, f.root)
				case "delete_source_root":
					other = nodeDeletion("alice", f.root)
				case "downgrade":
					other = request("alice", http.MethodPost, "/api/v0/notebooks/"+f.nb+"/downgrade", "")
				case "reindex":
					other = step{command: func() error { return Reindex(t.Context(), cfg, &errOut, &out, uuid.MustParse(f.nb)) }}
				default:
					other = pageCreation("alice", f.nb, "After A")
				}
				bDone = ingestI34Start(t, f.tm, other)
			}
			ingestI34BookWait(t, f, aw.pid, bPID, bDone)
			// No new B node is visible; Q and the source root still have A's
			// initial committed tree while its multibatch coverage is paused.
			if count(t, f.tm.pool, `SELECT count(*) FROM nodes WHERE notebook_id=$1`, f.nb) != 1025 ||
				count(t, f.tm.pool, `SELECT count(*) FROM notebooks WHERE id=$1 AND mode='llm_wiki' AND source_root_id=$2`, f.nb, f.root) != 1 {
				t.Fatal("I4 concurrent tree/mode change entered A's ongoing scan")
			}
			wantMarks := []string{f.t}
			if scenario == "same_unit_delete_Q" {
				oldQ := ingestI34Direct(t, capture, f.q)
				qBefore := ingestI34Before(t, capture.before, f.q)
				if !slices.Equal(capture.w, ingestI34IDs(f.p, f.q)) || !qBefore.Exists || qBefore.InSource || !qBefore.RevisionKnown || qBefore.Revision != 1 ||
					!qBefore.PageBytesKnown || qBefore.PageBytes != int64(len("[[raw/S]]\n")) ||
					oldQ.State != linkingapp.OldFactsKnown || len(oldQ.References) != 1 || oldQ.References[0].TargetID.String() != f.s ||
					oldQ.References[0].TargetIsAsset || oldQ.References[0].Origin != linkingapp.ReferenceBody {
					t.Fatalf("I4 deleted Q was not preserved as real Known old D={S}: before=%+v old=%+v W=%v", qBefore, oldQ, capture.w)
				}
			} else {
				oldP := ingestI34Direct(t, capture, f.p)
				if !slices.Equal(capture.w, []uuid.UUID{a.base.node}) || oldP.State != linkingapp.OldFactsKnown || len(oldP.References) != 0 {
					t.Fatalf("I4 P old facts=%+v W=%v", oldP, capture.w)
				}
				if scenario == "same_unit_move_Q_without_content" {
					qBefore := ingestI34Before(t, capture.before, f.q)
					if !qBefore.Exists || qBefore.InSource || !qBefore.RevisionKnown || qBefore.Revision != 1 || len(capture.direct) != 1 {
						t.Fatalf("I4 moved-only Q must retain initial memo yet stay outside W: %+v %+v", qBefore, capture)
					}
					wantMarks = []string{f.s, f.t}
				}
			}
			a.releaseCoverage()
			if tree {
				ingestI34Answer(t, f.tm, aDone, 0, "")
				done := answerOf[mcpBatched](t, aMCP)
				if done.ChangesetID == nil || *done.ChangesetID != aw.cs.String() || !done.Changed || done.ResultsCount != 2 {
					t.Fatalf("I4 actual batch=%+v, want observed own cs with two ops", done)
				}
			} else {
				ingestI34Answer(t, f.tm, aDone, http.StatusOK, "")
			}
			heads := ingestI34FullScan(t, a, aDone, batches, expected)
			q := heads[uuid.MustParse(f.q)]
			if scenario == "same_unit_delete_Q" {
				if q.Live {
					t.Fatal("I4 raw scan did not expose this unit's deleted Q")
				}
			} else if scenario == "same_unit_move_Q_without_content" {
				if !q.Live || !q.InSource || q.Revision != 1 {
					t.Fatalf("I4 untouched content's final-tree source membership=%+v", q)
				}
			} else if !q.Live || q.InSource || q.Revision != 1 {
				t.Fatalf("I4 concurrent B polluted Q during coverage: %+v", q)
			}
			ingestCheckCommitted(t, f, a.base, aw, wantMarks...)
			changesets := 1
			switch scenario {
			case "create_lower_uuid":
				ingestI34Answer(t, f.tm, bDone, 0, "")
				done := answerOf[mcpBatched](t, bMCP)
				if !done.Changed || done.ResultsCount != 1 || len(done.Results) != 1 || done.Results[0].NodeID == "" || done.ChangesetID == nil {
					t.Fatalf("B create=%+v", done)
				}
				id := uuid.MustParse(done.Results[0].NodeID)
				if id.Compare(*batches[1].batch.Last) >= 0 {
					t.Fatalf("I4 fixture did not create the real preplanned lower UUID: oldMax=%s created=%s after=%s; do not claim this leg", lastOldID, id, *batches[1].batch.Last)
				}
				if _, included := heads[id]; included {
					t.Fatal("B's later inserted ID was mixed into A's scan")
				}
				if n := count(t, f.tm.pool, `SELECT count(*) FROM nodes WHERE id=$1 AND notebook_id=$2 AND deleted_at IS NULL AND name='Earlier'`, id, f.nb); n != 1 {
					t.Fatal("B did not persist its actual new live page")
				}
				changesets++
			case "move_Q_into_source":
				ingestI34Answer(t, f.tm, bDone, http.StatusOK, "")
				if count(t, f.tm.pool, `SELECT count(*) FROM nodes WHERE id=$1 AND parent_id=$2 AND deleted_at IS NULL AND source_hash IS NOT NULL`, f.q, f.root) != 1 {
					t.Fatal("B's real move did not enter source")
				}
				changesets++
			case "delete_source_root":
				ingestI34Answer(t, f.tm, bDone, http.StatusConflict, "page.protected")
				if count(t, f.tm.pool, `SELECT count(*) FROM nodes WHERE id=$1 AND deleted_at IS NULL`, f.root) != 1 {
					t.Fatal("protected root disappeared")
				}
			case "downgrade":
				ingestI34Answer(t, f.tm, bDone, http.StatusOK, "")
				if count(t, f.tm.pool, `SELECT count(*) FROM notebooks WHERE id=$1 AND mode='standard' AND source_root_id IS NULL`, f.nb) != 1 ||
					count(t, f.tm.pool, `SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL`, f.nb) != 0 {
					t.Fatal("B downgrade did not atomically clear source mode/hash")
				}
			case "reindex":
				ingestI34Answer(t, f.tm, bDone, 0, "")
				if errOut.Len() != 0 || !strings.Contains(out.String(), "notebook "+f.nb+":") {
					t.Fatalf("real reindex stdout=%q stderr=%q", out.String(), errOut.String())
				}
			default:
				ingestI34Answer(t, f.tm, bDone, http.StatusCreated, "")
				changesets++
			}
			if scenario == "same_unit_delete_Q" && count(t, f.tm.pool, `SELECT count(*) FROM nodes WHERE id=$1 AND deleted_at IS NOT NULL`, f.q) != 1 {
				t.Fatal("A did not really delete Q")
			}
			if scenario == "same_unit_move_Q_without_content" {
				if qNow := f.tm.content(t, "alice", f.q); qNow.Content != "[[raw/S]]\n" || qNow.Revision != 1 {
					t.Fatalf("move-only control unexpectedly wrote Q content: %+v", qNow)
				}
			}
			f.tm.resolves(t, f.p, f.s, f.t)
			ingestCheckTotals(t, f, changesets, len(wantMarks))
		})
	}
}

var (
	_ linkingapp.Store       = ingestI34Store{}
	_ page.PageObserver      = ingestI34Observer{}
	_ page.Notebooks         = ingestI34Notebooks{}
	_ linking.IngestMetadata = ingestI34Metadata{}
)
