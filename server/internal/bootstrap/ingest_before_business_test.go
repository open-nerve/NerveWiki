package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	linkingdomain "github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	pageapp "github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func ingestCapturedAfter(t *testing.T, u *ingestI34Unit) ingestI34CaptureSeen {
	t.Helper()
	select {
	case seen := <-u.capture:
		if seen.err != nil || seen.pid == 0 || !seen.indexHeld {
			t.Fatalf("same real transaction/index Capture: %+v", seen)
		}
		return seen
	case <-time.After(interleavingWait):
		t.Fatal("real additional Capture was not called")
	}
	return ingestI34CaptureSeen{}
}
func testIngestBeforeBusiness(t *testing.T) {
	t.Run("old_resolution", func(t *testing.T) {
		control := &ingestI34Control{}
		stable := &stableIndexControl{}
		w := stableIndexWire(t, control.wiring(t), stable)
		f := newIngestFixture(t, &w)
		r := f.tm.createPageWith(t, "alice", f.nb, f.raw, "R", "independent R")
		f.write(t, f.p, "[[raw/S]]")
		marker := f.tm.createPage(t, "alice", f.nb, "", "Stable marker")
		f.tm.settle(t, f.nb, marker)
		stream := openStream(t, f.tm.base, "alice", f.tm.tokens["alice"])
		stableUnit := newStableIndexUnit(f.p, f.nb, 3, "[[raw/S]] [[raw/R]]")
		stable.arm(stableUnit)
		u := newIngestI34Unit(t, f.p, f.s, f.s, "[[raw/S]] [[raw/R]]")
		u.extraCapture = true
		u.inspect = ingestI34IDs(f.p, f.s, f.t)
		control.arm(u)
		cs := f.batch(t, opWith(opMove("raw/S.md", "raw/Temp.md"), "update_links", false), opWith(opMove("raw/T.md", "raw/S.md"), "update_links", false), opWith(opMove("raw/Temp.md", "raw/T.md"), "update_links", false), opWriteOn("P.md", u.base.body, versionOf(f.p, 2)))
		stable.arm()
		seen := ingestCapturedAfter(t, u)
		old := ingestI34Direct(t, seen, f.p)
		if old.State != linkingapp.OldFactsKnown || len(old.References) != 1 || old.References[0].TargetID.String() != f.s {
			t.Fatalf("old resolved identity followed the final path: %+v", old)
		}
		assertIngestMarks(t, f.tm, cs, f.t, r)
		f.tm.resolves(t, f.p, f.t, r)
		checkPages(t, f.tm.pool)
		checkLinks(t, f.tm.pool)
		P, N, S, T, R := uuid.MustParse(f.p), uuid.MustParse(f.nb), uuid.MustParse(f.s), uuid.MustParse(f.t), uuid.MustParse(r)
		oldRows := []stableWiringRow{stableIndexRow(P, N, 2, 7, "wikilink", "raw/S", "s", S, false, false)}
		replacedRows := []stableWiringRow{oldRows[0], stableIndexRow(P, N, 12, 17, "wikilink", "raw/R", "r", uuid.Nil(), false, false)}
		finalRows := []stableWiringRow{stableIndexRow(P, N, 2, 7, "wikilink", "raw/S", "s", T, false, false), stableIndexRow(P, N, 12, 17, "wikilink", "raw/R", "r", R, false, false)}
		stableIndexCheck(t, f.tm, stableUnit, stableIndexWant{before: oldRows, replaced: replacedRows, final: finalRows, changed: stableIndexLiteralLinks(finalRows), dropped: []uuid.UUID{S}, keys: []string{"r", "s"}})
		stableIndexCheckCandidates(t, stableUnit, T, R)
		stableIndexCheckPath(t, stableUnit, P, "P")
		stableIndexEvents(t, f.tm, f.nb, marker, stream, true, 3, f.p, f.s, f.t, r)
		f.tm.checkRebuilt(t, f.nb)
	})
	t.Run("before_target_head", func(t *testing.T) {
		control := &ingestI34Control{}
		w := control.wiring(t)
		production := &ingestBeforeProductionTrace{}
		wired := w.page
		w.page = func(d *page.Deps) {
			wired(d)
			found := 0
			for i, o := range d.Observers {
				probe, ok := o.(ingestI34Observer)
				if !ok {
					continue
				}
				original, ok := probe.real.(linkIndex)
				if !ok {
					t.Fatal("B18 original linkIndex missing")
				}
				deps := *original.index.Ingest
				// The additional Capture saved its original Heads before this
				// wrapper. Only actual beginIngest/finishIngest calls reach it.
				deps.Heads = ingestBeforeProductionHeads{deps.Heads, production}
				if deps.OldBodies == nil {
					t.Fatal("B18 NewIndex did not wire the private old-F reader")
				}
				deps.OldBodies = ingestBeforeProductionOldBodies{deps.OldBodies, production}
				original.index.Ingest = &deps
				original.index.Store = ingestBeforeProductionStore{original.index.Store, production}
				if original.index.Preparations == nil {
					t.Fatal("B18 NewIndex did not wire source preparation")
				}
				original.index.Preparations = ingestBeforePreparations{original.index.Preparations, production}
				probe.real = original
				d.Observers[i] = probe
				found++
			}
			if found != 1 {
				t.Fatalf("B18 real production observers%d", found)
			}
		}
		stable := &stableIndexControl{}
		w = stableIndexWire(t, w, stable)
		f := newIngestFixture(t, &w)
		a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "asset")
		source := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Follow", "[A](a.txt)")
		q := f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/Follow]]")
		u := newIngestI34Unit(t, f.p, q, source, "[[raw/T]]")
		u.extraCapture = true
		u.inspect = ingestI34IDs(f.p, q, source)
		control.arm(u)
		stableUnit := newStableIndexUnit(f.p, f.nb, 2, u.base.body)
		stableUnit.observeCapture = true
		stable.arm(stableUnit)
		production.mu.Lock()
		production.target = uuid.MustParse(source)
		production.stable = stableUnit
		production.armed = true
		production.mu.Unlock()
		cs := f.batch(t, opMove("raw/a.txt", "raw/b.txt"), opWriteOn("Q.md", "kept [[raw/Follow]]", versionOf(q, 1)), opWriteOn("P.md", u.base.body, versionOf(f.p, 1)))
		stable.arm()
		seen := ingestCapturedAfter(t, u)
		production.mu.Lock()
		actual, replaced := slices.Clone(production.heads), production.replaced
		bodies := slices.Clone(production.bodies)
		production.mu.Unlock()
		if len(actual) == 0 || !replaced {
			t.Fatalf("B18 production observation not reached: heads%+v replaced%t", actual, replaced)
		}
		first := actual[0]
		if first.replaced || first.pid != seen.pid || !first.head.Present || first.head.PageRevision != 1 || first.head.IndexedRevision != 1 || first.head.Extractor != 2 {
			t.Fatalf("actual production first old source head was not before any Replace in original tx: %+v capturePID%d", first, seen.pid)
		}
		production.mu.Lock()
		prepared, residuals := slices.Clone(production.prepared), production.residuals
		production.mu.Unlock()
		if len(bodies) != 0 || len(prepared) != 1 || residuals != 1 {
			t.Fatalf("B18_PREPARED_ROUTE: fallback=%d admitted=%d residual=%d", len(bodies), len(prepared), residuals)
		}
		production.mu.Lock()
		got := *prepared[0]
		got.rows, got.assets, got.supplied = slices.Clone(got.rows), slices.Clone(got.assets), slices.Clone(got.supplied)
		production.mu.Unlock()
		if !got.admitted || !got.ready || got.replaced || got.pid != seen.pid || got.source.String() != source || got.notebook.String() != f.nb ||
			got.newQueries != 0 || got.queries != 1 || got.reads != 1 || got.applies != 1 || got.scans != 1 || got.eof != 1 || got.errs != 1 || got.closes != 1 || got.err != nil {
			t.Fatalf("B18_PREPARED_NATIVE: locked/proved complete one-source preparation differs: %+v", got)
		}
		wantEdge := linkingapp.BodyReference{EdgePos: linkingapp.EdgePos{SourceID: uuid.MustParse(source), Start: 4}, ResolvedID: uuid.MustParse(a.ID), ResolvedAsset: true}
		wantInput := linkingapp.Link{SourceID: uuid.MustParse(source), Start: 4, Target: "b.txt"}
		if !slices.Equal(got.rows, []linkingapp.BodyReference{wantEdge}) || !slices.Equal(got.assets, []uuid.UUID{uuid.MustParse(a.ID)}) || !got.hasSupply || !slices.Equal(got.supplied, []linkingapp.Link{wantInput}) {
			t.Fatalf("B18_PREPARED_VALUES: old native F or full changed input differs: rows=%+v assets=%v supplied=%+v", got.rows, got.assets, got.supplied)
		}
		old := ingestI34Direct(t, seen, q)
		before := ingestI34Before(t, seen.before, source)
		if old.State != linkingapp.OldFactsKnown || len(old.References) != 1 || old.References[0].TargetID.String() != source || !before.RevisionKnown || before.Revision != 1 || !before.PageBytesKnown || before.PageBytes != int64(len("[A](a.txt)")) {
			t.Fatalf("first source/head before real system rewrite: old%+v before%+v", old, before)
		}
		if c := f.tm.content(t, "alice", source); c.Revision != 2 {
			t.Fatalf("participant did not write realr2: %+v", c)
		}
		if count(t, f.tm.pool, "SELECT revision FROM indexed_pages WHERE node_id=$1", source) != 2 {
			t.Fatal("source final index notr2")
		}
		assertIngestMarks(t, f.tm, cs, f.t)
		f.tm.resolves(t, source, a.ID)
		checkPages(t, f.tm.pool)
		checkLinks(t, f.tm.pool)
		stableIndexCheckCapture(t, stableUnit)
	})
	t.Run("possible_full", func(t *testing.T) {
		control := &ingestI34Control{}
		w := control.wiring(t)
		f := newIngestFixture(t, &w)
		a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "asset")
		f.write(t, f.p, "[A](raw/a.txt) [[raw/S]]")
		u := newIngestI34Unit(t, f.p, f.s, f.s, "[A](raw/a.txt) [[raw/S]] [[raw/T]]")
		u.extraCapture = true
		u.inspect = ingestI34IDs(f.p, f.s, a.ID)
		control.arm(u)
		cs := f.write(t, f.p, u.base.body) // real SHARE: the untouched page target is unproven
		seen := ingestCapturedAfter(t, u)
		old := ingestI34Direct(t, seen, f.p)
		if old.State != linkingapp.OldFactsUnknown || old.Unknown.Extent != linkingapp.OldUnknownPossibleTargets || !slices.Equal(old.Unknown.PossibleTargets, ingestI34IDs(f.s, a.ID)) || len(old.References) != 1 || old.References[0].TargetID.String() != a.ID || !old.References[0].TargetIsAsset {
			t.Fatalf("complete old upper bound lost known asset or uncertain page: %+v", old)
		}
		assertIngestMarks(t, f.tm, cs, f.t)
		checkPages(t, f.tm.pool)
		checkLinks(t, f.tm.pool)
	})
}

// The finite-closure case controls exactly one Before byte-certainty bit at
// the public port. Real I3 separately proves when SHARE can produce that
// uncertainty; this case exercises composition with a known K and full F.
type ingestBeforeUncertain struct {
	page.IngestBefore
	id       uuid.UUID
	observed *atomic.Bool
}

func (r ingestBeforeUncertain) NodesBefore(ctx context.Context, ids []uuid.UUID) ([]page.IngestNodeBefore, error) {
	rows, err := r.IngestBefore.NodesBefore(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ID == r.id {
			rows[i].PageBytesKnown = false
			r.observed.Store(true)
		}
	}
	return rows, nil
}

type ingestBeforeUncertainObserver struct {
	page.PageObserver
	armed    *atomic.Bool
	id       *uuid.UUID
	observed *atomic.Bool
}

func (o ingestBeforeUncertainObserver) PagesChanged(ctx context.Context, e page.Event) error {
	if o.armed.CompareAndSwap(true, false) {
		e.BeforeIngest = ingestBeforeUncertain{e.BeforeIngest, *o.id, o.observed}
	}
	return o.PageObserver.PagesChanged(ctx, e)
}
func testIngestFiniteFollowClosure(t *testing.T) {
	control := &ingestI34Control{}
	w := control.wiring(t)
	armed, observed := &atomic.Bool{}, &atomic.Bool{}
	var uncertain uuid.UUID
	original := w.page
	w.page = func(d *page.Deps) {
		original(d)
		for i, o := range d.Observers {
			if _, ok := o.(ingestI34Observer); ok {
				d.Observers[i] = ingestBeforeUncertainObserver{o, armed, &uncertain, observed}
			}
		}
	}
	f := newIngestFixture(t, &w)
	a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "asset A")
	b := f.tm.upload(t, "alice", f.nb, f.raw, "b.txt", "asset B")
	s := f.tm.createPageWith(t, "alice", f.nb, f.raw, "FollowS", "[A](a.txt)")
	k := f.tm.createPageWith(t, "alice", f.nb, f.raw, "FollowK", "[B](b.txt)")
	q := f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/FollowS]] [[raw/FollowK]]")
	u := newIngestI34Unit(t, f.p, q, s, "[A](raw/aa.txt) [B](raw/bb.txt) [[raw/T]]")
	u.extraCapture = true
	u.inspect = ingestI34IDs(f.p, q, s, k)
	control.arm(u)
	uncertain = uuid.MustParse(s)
	armed.Store(true)
	cs := f.batch(t, opMove("raw/a.txt", "raw/aa.txt"), opMove("raw/b.txt", "raw/bb.txt"), opWriteOn("Q.md", "old references removed", versionOf(q, 1)), opWriteOn("P.md", u.base.body, versionOf(f.p, 1)))
	seen := ingestCapturedAfter(t, u)
	old := ingestI34Direct(t, seen, q)
	if armed.Load() || !observed.Load() || old.State != linkingapp.OldFactsUnknown || old.Unknown.Extent != linkingapp.OldUnknownPossibleTargets || !slices.Equal(old.Unknown.PossibleTargets, ingestI34IDs(s, k)) || len(old.References) != 1 || old.References[0].TargetID.String() != k {
		t.Fatalf("finite old D must retain uncertain S and known K: %+v", old)
	}
	foundS, foundK := false, false
	for _, v := range seen.followed {
		switch v.PageID.String() {
		case s:
			foundS = true
			if v.State != linkingapp.OldFactsUnknown || v.Unknown.Extent != linkingapp.OldUnknownPossibleTargets || !slices.Equal(v.Unknown.PossibleTargets, ingestI34IDs(a.ID)) {
				t.Fatalf("complete possible F(S)=%+v", v)
			}
		case k:
			foundK = true
			if v.State != linkingapp.OldFactsKnown || !slices.Equal(v.AssetIDs, ingestI34IDs(b.ID)) {
				t.Fatalf("known complete F(K)=%+v", v)
			}
		}
	}
	if !foundS || !foundK {
		t.Fatalf("missing independent source F roles: %+v", seen.followed)
	}
	assertIngestMarks(t, f.tm, cs, f.t)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}

type ingestAncestorFault struct {
	armed, applied atomic.Bool
	id, missing    uuid.UUID
	prewrite       bool
}

func (g *ingestAncestorFault) GuardWrite(_ context.Context, s page.Step) error {
	for i, c := range s.Changes {
		if c.NodeID == g.id && c.Before != nil && c.Revision > 0 && g.armed.CompareAndSwap(true, false) {
			before := *c.Before
			before.ParentID = &g.missing
			s.Changes[i].Before = &before
			g.applied.Store(true)
		}
	}
	return nil
}

type ingestAncestorObserver struct {
	page.PageObserver
	fault *ingestAncestorFault
}

func (o ingestAncestorObserver) PagesChanged(ctx context.Context, e page.Event) error {
	if o.fault.applied.Load() {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			return errors.New("ancestor fixture lost real transaction")
		}
		var revision int
		if err := tx.QueryRow(ctx, "SELECT revision FROM page_contents WHERE node_id=$1", o.fault.id).Scan(&revision); err != nil {
			return err
		}
		o.fault.prewrite = revision == 2
	}
	return o.PageObserver.PagesChanged(ctx, e)
}
func testIngestMissingAncestor(t *testing.T) {
	fault := &ingestAncestorFault{missing: uuid.NewV7()}
	state := &ingestStreamState{done: make(chan struct{}), mode: "healthy"}
	f := newIngestFixture(t, &wiring{page: func(d *page.Deps) {
		d.Tx = ingestStreamManager{Tx: d.Tx, state: state}
		d.Guards = append(d.Guards, fault)
		for i, o := range d.Observers {
			if _, ok := o.(linkIndex); ok {
				d.Observers[i] = ingestAncestorObserver{o, fault}
			}
		}
	}})
	fault.id = uuid.MustParse(f.p)
	state.missingID = fault.missing
	if n := count(t, f.tm.pool, "SELECT count(*) FROM nodes WHERE id=$1", fault.missing); n != 0 {
		t.Fatal("required parent fixture exists")
	}
	before := ingestStreamImage(t, f)
	fault.armed.Store(true)
	state.armed.Store(true)
	f.tm.send(t, contentWrite("alice", f.p, "[[raw/T]]", 1, ""), http.StatusInternalServerError)
	<-state.done
	if !fault.applied.Load() || !fault.prewrite || state.missingReads != 1 || !errors.Is(state.callbackErr, pageapp.ErrNotFound) || state.managerErr == nil || state.replaces != 0 || state.adds != 0 {
		t.Fatalf("missing initial ancestor fell back to final tree: fault%+v state%+v", fault, state)
	}
	if after := ingestStreamImage(t, f); after != before {
		t.Fatalf("missing ancestor changed real unit: before=%s after=%s", before, after)
	}
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}

type ingestMarksAfterFault struct {
	linking.IngestMarks
	armed *atomic.Bool
	cause error
	ran   *atomic.Bool
}

func (m ingestMarksAfterFault) Add(ctx context.Context, nb, cs uuid.UUID, ids []uuid.UUID) (linking.IngestMarkResult, error) {
	result, err := m.IngestMarks.Add(ctx, nb, cs, ids)
	if err != nil {
		return result, err
	}
	if m.armed.CompareAndSwap(true, false) {
		if result.Added != 1 || result.Count != 2 {
			return result, fmt.Errorf("session rollback fixture Add=%+v", result)
		}
		m.ran.Store(true)
		return result, m.cause
	}
	return result, nil
}
func testIngestSessionMarksRollback(t *testing.T) {
	armed, ran := &atomic.Bool{}, &atomic.Bool{}
	cause := errors.New("after real session mark")
	state := &ingestStreamState{done: make(chan struct{}), mode: "healthy"}
	probe := &ingestMarksTxProbe{t: t}
	stable := &stableIndexControl{}
	w := stableIndexWire(t, wiring{queryTracer: probe, page: func(d *page.Deps) {
		if d.Pool.Config().MaxConns < 2 {
			t.Fatal("U09 observation requires at least two actual serve-pool connections")
		}
		d.Tx = ingestStreamManager{Tx: d.Tx, state: state}
		for i, o := range d.Observers {
			if original, ok := o.(linkIndex); ok {
				deps := *original.index.Ingest
				deps.Marks = ingestMarksTxWitness{IngestMarks: ingestMarksAfterFault{deps.Marks, armed, cause, ran}, probe: probe}
				original.index.Ingest = &deps
				d.Observers[i] = original
			}
		}
	}}, stable)
	f := newIngestFixture(t, &w)
	session := f.tm.openSession(t, "alice", f.p)
	f.tm.send(t, contentWrite("alice", f.p, "[[raw/S]]", 1, session), http.StatusOK)
	cs := ingestCS(t, f.tm, f.p, 2)
	assertIngestMarks(t, f.tm, cs, f.s)
	before := ingestStreamImage(t, f)
	stableUnit := newStableIndexUnit(f.p, f.nb, 3, "[[raw/S]] [[raw/T]]")
	stable.arm(stableUnit)
	armed.Store(true)
	state.armed.Store(true)
	probe.active.Store(true)
	reply := f.tm.sender(t, contentWrite("alice", f.p, "[[raw/S]] [[raw/T]]", 2, session))()
	<-state.done
	stable.arm()
	probe.active.Store(false)
	seen := probe.observations()
	if len(seen) != 1 || seen[0].OwnerPID == 0 || seen[0].ActualPID == 0 || !seen[0].ParentHealthy {
		t.Fatalf("U09 observation did not reach one live actual Add: %+v", seen)
	}
	if seen[0].OwnerPID != seen[0].ActualPID {
		// TraceQueryStart already reported the independent business failure,
		// BEFORE cancelling its own child. Verify cleanup, not a timeout kill.
		if !seen[0].CleanupIssued || !seen[0].ParentStillHealthy || state.adds != 0 ||
			!errors.Is(state.callbackErr, context.Canceled) || !errors.Is(state.managerErr, context.Canceled) {
			t.Errorf("U09 wrong-executor cleanup ownership: observed=%+v state=%+v", seen, state)
		}
		if after := ingestStreamImage(t, f); after != before {
			t.Errorf("U09 wrong-executor cleanup failed original unit rollback: before=%s after=%s", before, after)
		}
		if reply.err != nil || reply.status != http.StatusInternalServerError {
			t.Errorf("U09 wrong-executor cleanup response: %d %s err=%v", reply.status, reply.body, reply.err)
		}
		t.Logf("U09_WRONG_TX_CLEANUP: original Writer callback/manager returned; identities=%+v", seen)
		return // this leaf is already FAIL; no retry of the intentionally cancelled fault
	}
	if seen[0].CleanupIssued {
		t.Fatal("U09 observation cancelled a correct executor")
	}
	if !ran.Load() || state.adds != 1 || !errors.Is(state.callbackErr, cause) || !errors.Is(state.managerErr, cause) {
		t.Fatalf("actual existing-cs Add failure lost: ran%t state%+v", ran.Load(), state)
	}
	if after := ingestStreamImage(t, f); after != before {
		t.Fatalf("existing session mark leaked outside failed unit: before=%s after=%s", before, after)
	}
	if reply.err != nil || reply.status != http.StatusInternalServerError {
		t.Fatalf("actual Add-failure response: %d %s err=%v", reply.status, reply.body, reply.err)
	}
	assertIngestMarks(t, f.tm, cs, f.s)
	stableIndexCheckSessionRetention(t, stableUnit, uuid.MustParse(f.p), uuid.MustParse(f.nb), uuid.MustParse(f.s), uuid.MustParse(f.t))
	f.tm.send(t, contentWrite("alice", f.p, "[[raw/S]] [[raw/T]]", 2, session), http.StatusOK)
	if ingestCS(t, f.tm, f.p, 3) != cs {
		t.Fatal("healthy retry left original session")
	}
	assertIngestMarks(t, f.tm, cs, f.s, f.t)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
}

// These B18 wrappers delegate every original query and mutation. The separate
// extra Capture retains its pre-wrapper ports, so it cannot satisfy this oracle.
type ingestBeforeProductionTrace struct {
	mu                      sync.Mutex
	armed, locked, replaced bool
	tx                      pgx.Tx
	target                  uuid.UUID
	heads                   []ingestBeforeProductionHead
	bodies                  []ingestBeforeProductionBody
	prepared                []*ingestBeforePrepared
	residuals               int
	stable                  *stableIndexUnit
}
type ingestBeforeProductionHead struct {
	head     linkingapp.IngestIndexedHead
	replaced bool
	pid      int32
}
type ingestBeforeProductionHeads struct {
	linkingapp.IngestIndexHeads
	trace *ingestBeforeProductionTrace
}

func (r ingestBeforeProductionHeads) IndexedHeads(ctx context.Context, nb uuid.UUID, ids []linkingapp.IngestRevision) ([]linkingapp.IngestIndexedHead, error) {
	r.trace.mu.Lock()
	stable, armed := r.trace.stable, r.trace.armed
	r.trace.mu.Unlock()
	watch := false
	if stable != nil && armed {
		stable.mu.Lock()
		watch = stableIndexStage(stable, "replace_enter") == 0
		if watch {
			stable.captureReading = true
			stable.stageLocked(ctx, "capture_heads_enter")
		}
		stable.mu.Unlock()
	}
	rows, err := r.IngestIndexHeads.IndexedHeads(ctx, nb, ids)
	if watch {
		stable.mu.Lock()
		stable.stageLocked(ctx, "capture_heads_done")
		stable.mu.Unlock()
		stable.probe(err)
	}
	if err != nil {
		return nil, err
	}
	r.trace.mu.Lock()
	defer r.trace.mu.Unlock()
	if r.trace.armed {
		for _, h := range rows {
			if h.ID == r.trace.target {
				tx, ok := postgres.TxFrom(ctx)
				var pid int32
				if ok {
					pid = int32(tx.Conn().PgConn().PID())
				}
				r.trace.heads = append(r.trace.heads, ingestBeforeProductionHead{h, r.trace.replaced, pid})
			}
		}
	}
	return rows, nil
}

type ingestBeforeProductionBody struct {
	notebook        uuid.UUID
	emitters        []uuid.UUID
	after           *linkingapp.EdgePos
	rows            []linkingapp.BodyReference
	take            int
	pid             int32
	ready, replaced bool
	err             error
}
type ingestBeforeProductionOldBodies struct {
	linkingapp.IngestOldBodyReads
	trace *ingestBeforeProductionTrace
}

func (r ingestBeforeProductionOldBodies) OldBodyReferences(ctx context.Context, notebook uuid.UUID, emitters []uuid.UUID, after *linkingapp.EdgePos, take int) ([]linkingapp.BodyReference, error) {
	r.trace.mu.Lock()
	armed := r.trace.armed
	r.trace.mu.Unlock()
	if !armed {
		return r.IngestOldBodyReads.OldBodyReferences(ctx, notebook, emitters, after, take)
	}
	b := ingestBeforeProductionBody{notebook: notebook, emitters: slices.Clone(emitters), take: take}
	if after != nil {
		p := *after
		b.after = &p
	}
	tx, ok := postgres.TxFrom(ctx)
	if ok {
		b.pid = int32(tx.Conn().PgConn().PID())
	}
	r.trace.mu.Lock()
	b.ready = ok && r.trace.locked && r.trace.tx == tx && len(r.trace.heads) > 0
	b.replaced = r.trace.replaced
	r.trace.mu.Unlock()
	rows, err := r.IngestOldBodyReads.OldBodyReferences(ctx, notebook, emitters, after, take)
	b.rows, b.err = slices.Clone(rows), err
	r.trace.mu.Lock()
	b.replaced = b.replaced || r.trace.replaced
	r.trace.bodies = append(r.trace.bodies, b)
	r.trace.mu.Unlock()
	return rows, err
}

type ingestBeforeProductionStore struct {
	linkingapp.Store
	trace *ingestBeforeProductionTrace
}

func (s ingestBeforeProductionStore) Lock(ctx context.Context, notebook uuid.UUID) error {
	err := s.Store.Lock(ctx, notebook)
	s.trace.mu.Lock()
	if s.trace.armed && err == nil {
		s.trace.tx, s.trace.locked = postgres.TxFrom(ctx)
	}
	s.trace.mu.Unlock()
	return err
}

func (s ingestBeforeProductionStore) ReplacePage(ctx context.Context, p linkingapp.Page, f linkingdomain.Facts) (linkingapp.Dropped, error) {
	s.trace.mu.Lock()
	if s.trace.armed {
		s.trace.replaced = true
	}
	s.trace.mu.Unlock()
	return s.Store.ReplacePage(ctx, p, f)
}
