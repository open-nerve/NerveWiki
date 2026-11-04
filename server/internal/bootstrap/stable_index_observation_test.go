package bootstrap

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	linkingapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	linkingdomain "github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// These observations reuse R1's nullable DTO and SQL-argument tracer. They never
// replace a resolver answer or change a successful production port's result.
// A unit is registered only after its API-created initial state is complete.
type stableIndexUnit struct {
	stableWiringState
	notebook                                         uuid.UUID
	revision                                         int
	body                                             string
	before, partial                                  []stableWiringRow
	incoming                                         []linkingdomain.Link
	writerBody                                       string
	writerRevision, beforeHead, afterHead, extractor int
	cs                                               uuid.UUID
	fresh                                            bool
	aliases                                          []linkingapp.Alias
	candidateNodes, pathNodes                        []linkingdomain.Node
	queries                                          []stableIndexQuery
	probeFailures                                    []error
	replaceCalls, setCalls, markCalls, publishCalls  int
	inspectCalls                                     int
	inspectRead                                      *stableIndexRowsSeen
	inspectAllowed                                   bool
	inspectTargets                                   []uuid.UUID
	markIDs                                          []uuid.UUID
	markResult                                       linking.IngestMarkResult
	storedMarks                                      int
	published                                        linking.LinksChanged
	publisherCause, cause                            error
	failureSQL                                       string
	failureProbe                                     bool
	failureHead, failureRevision, failureExtractor   int
	failureFrontmatter                               bool
	failureBody, failureProperty                     string
	failureContentHistory                            bool
	callbackErr, managerErr                          error
	managerDone                                      chan struct{}
	afterReplace, allowFresh                         chan struct{}
	releaseOnce                                      sync.Once
	observeCapture, captureReading                   bool
	captureRows                                      []*stableIndexRowsSeen
}

type stableIndexQuery struct {
	Name, SQL     string
	Start, End    int
	PID           uint32
	Err           error
	Done, Bounded bool
	ContextErr    error
}

type stableIndexRowsSeen struct {
	Name                 string
	Start, End           int
	PID                  uint32
	Rows                 int
	EOF, Closed          bool
	CloseOrder, ErrOrder int
	Err                  error
}

type stableIndexControl struct {
	mu     sync.Mutex
	units  map[uuid.UUID]*stableIndexUnit
	txUnit *stableIndexUnit
	pool   *pgxpool.Pool
}

type stableIndexUnitKey struct{}
type stableIndexQueryKey struct{}
type stableIndexQueryAt struct {
	unit  *stableIndexUnit
	index int
}

func newStableIndexUnit(pageID, notebook string, revision int, body string) *stableIndexUnit {
	u := &stableIndexUnit{notebook: uuid.MustParse(notebook), revision: revision, body: body,
		managerDone: make(chan struct{})}
	u.page = uuid.MustParse(pageID)
	return u
}
func (c *stableIndexControl) arm(units ...*stableIndexUnit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.units = make(map[uuid.UUID]*stableIndexUnit, len(units))
	for _, u := range units {
		c.units[u.page] = u
	}
}
func (u *stableIndexUnit) release() {
	if u.allowFresh != nil {
		u.releaseOnce.Do(func() { close(u.allowFresh) })
	}
}
func stableIndexCurrent(ctx context.Context) *stableIndexUnit {
	u, _ := ctx.Value(stableIndexUnitKey{}).(*stableIndexUnit)
	return u
}
func stableIndexMark(ctx context.Context, u *stableIndexUnit) context.Context {
	if stableIndexCurrent(ctx) == u {
		return ctx
	}
	ctx = context.WithValue(ctx, stableIndexUnitKey{}, u)
	ctx = context.WithValue(ctx, stableWiringUnitKey{}, &u.stableWiringState)
	return stableIndexContext{Context: ctx, unit: u}
}
func (u *stableIndexUnit) note(ctx context.Context, name string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.stageLocked(ctx, name)
}
func (u *stableIndexUnit) probe(err error) {
	if err == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.probeFailures = append(u.probeFailures, err)
}

// The original manager still owns the actual transaction and its end. This
// context forwards that transaction through an observational wrapper only.
type stableIndexContext struct {
	context.Context
	unit *stableIndexUnit
}

func (c stableIndexContext) Value(key any) any {
	v := c.Context.Value(key)
	if tx, ok := v.(pgx.Tx); ok {
		return stableIndexSQLTx{Tx: tx, unit: c.unit}
	}
	return v
}

type stableIndexSQLTx struct {
	pgx.Tx
	unit *stableIndexUnit
}

func (tx stableIndexSQLTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	stableIndexBeforeFailure(ctx, tx.Tx, tx.unit, sql)
	return tx.Tx.Exec(ctx, sql, args...)
}
func (tx stableIndexSQLTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Tx.Query(ctx, sql, args...)
	u := tx.unit
	u.mu.Lock()
	inspect := err == nil && stableIndexSQLName(sql) == "InspectPageLinksForRetention" && len(args) == 3 && args[0] == u.page
	capture := err == nil && u.observeCapture && u.captureReading && strings.HasPrefix(sql, "-- name: ")
	var seen *stableIndexRowsSeen
	if inspect || capture {
		u.clock++
		seen = &stableIndexRowsSeen{Name: stableIndexSQLName(sql), Start: u.clock, PID: tx.Conn().PgConn().PID()}
	}
	if inspect {
		u.inspectCalls++
		u.inspectRead = seen
	} else if capture {
		u.captureRows = append(u.captureRows, seen)
	}
	u.mu.Unlock()
	if inspect {
		return &stableIndexInspectRows{stableIndexRows: stableIndexRows{Rows: rows, unit: u, seen: seen}, allowed: true, targets: map[uuid.UUID]bool{}}, nil
	}
	if capture {
		return stableIndexRows{Rows: rows, unit: u, seen: seen}, nil
	}
	return rows, err
}

// Observe actual row values, not the collector's returned plan. The fixture's
// literal old rows and D remain the independent expected values below.
// Next/Err/Close continue through the same native forwarding observer.
type stableIndexInspectRows struct {
	stableIndexRows
	allowed bool
	targets map[uuid.UUID]bool
	scans   int
}

func (r *stableIndexInspectRows) Scan(dest ...any) error {
	if err := r.Rows.Scan(dest...); err != nil {
		return err
	}
	r.scans++
	valid := false
	if len(dest) == 13 {
		can, canOK := dest[0].(*bool)
		start, startOK := dest[1].(**int32)
		notebook, notebookOK := dest[3].(*uuid.UUID)
		resolved, resolvedOK := dest[12].(**uuid.UUID)
		valid = canOK && startOK && notebookOK && resolvedOK && can != nil && start != nil && notebook != nil && resolved != nil
		if valid {
			r.allowed = r.allowed && *can
			if *start != nil {
				r.allowed = r.allowed && *notebook == r.unit.notebook
				if *resolved != nil {
					r.targets[**resolved] = true
					r.allowed = r.allowed && **resolved != uuid.Nil()
				}
			}
		}
	}
	if !valid {
		r.allowed = false
		r.unit.probe(fmt.Errorf("stable actual Inspect Scan destination shape, count=%d", len(dest)))
	}
	return nil
}

func (r *stableIndexInspectRows) Close() {
	r.stableIndexRows.Close()
	r.unit.mu.Lock()
	defer r.unit.mu.Unlock()
	if !r.seen.EOF || r.seen.Err != nil || r.seen.ErrOrder == 0 || r.scans != r.seen.Rows {
		r.unit.probeFailures = append(r.unit.probeFailures, fmt.Errorf("stable Inspect closed before complete read: %+v scans=%d", *r.seen, r.scans))
		return
	}
	var targets []uuid.UUID
	for id := range r.targets {
		targets = append(targets, id)
	}
	slices.SortFunc(targets, uuid.UUID.Compare)
	r.unit.inspectAllowed, r.unit.inspectTargets = r.allowed, targets
}

// Called with the unit mutex held after the actual Replace has returned.
func stableIndexCheckInspect(t *testing.T, u *stableIndexUnit, oldRows int) {
	t.Helper()
	r := u.inspectRead
	if r == nil || r.Rows != max(1, oldRows) || !r.EOF || !r.Closed || r.Err != nil || r.ErrOrder <= r.End || r.CloseOrder <= r.ErrOrder {
		t.Fatalf("stable complete Inspect native read=%+v, want %d rows then EOF/Err/Close", r, max(1, oldRows))
	}
	for _, q := range u.queries {
		if q.Start > r.Start && q.Start < r.CloseOrder {
			t.Errorf("SQL began before Inspect Close: %+v, read=%+v", q, r)
		}
	}
}

type stableIndexRows struct {
	pgx.Rows
	unit *stableIndexUnit
	seen *stableIndexRowsSeen
}

func (r stableIndexRows) Next() bool {
	more := r.Rows.Next()
	r.unit.mu.Lock()
	defer r.unit.mu.Unlock()
	if more {
		r.seen.Rows++
	} else {
		r.seen.EOF = true
		r.unit.clock++
		r.seen.End = r.unit.clock
	}
	return more
}
func (r stableIndexRows) Close() {
	r.Rows.Close()
	r.unit.mu.Lock()
	defer r.unit.mu.Unlock()
	r.unit.clock++
	r.seen.Closed = true
	r.seen.CloseOrder = r.unit.clock
}
func (r stableIndexRows) Err() error {
	err := r.Rows.Err()
	r.unit.mu.Lock()
	r.unit.clock++
	r.seen.Err = err
	r.seen.ErrOrder = r.unit.clock
	r.unit.mu.Unlock()
	return err
}

func stableIndexSQLName(sql string) string {
	if strings.HasPrefix(sql, "-- name: ") {
		return strings.Fields(sql)[2]
	}
	return strings.ToLower(strings.TrimSpace(sql))
}

// Keep prior tracers, notably U09's actual executor witness, and every context
// value they return. No new query is issued from a pgx tracer callback.
type stableIndexTracer struct{ prior pgx.QueryTracer }

func (tr stableIndexTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if tr.prior != nil {
		ctx = tr.prior.TraceQueryStart(ctx, conn, d)
	}
	u := stableIndexCurrent(ctx)
	if u == nil {
		return ctx
	}
	ctx = u.TraceQueryStart(ctx, conn, d)
	_, bounded := ctx.Deadline()
	u.mu.Lock()
	u.clock++
	at := len(u.queries)
	u.queries = append(u.queries, stableIndexQuery{Name: stableIndexSQLName(d.SQL), SQL: d.SQL,
		Start: u.clock, PID: conn.PgConn().PID(), Bounded: bounded, ContextErr: ctx.Err()})
	u.mu.Unlock()
	return context.WithValue(ctx, stableIndexQueryKey{}, stableIndexQueryAt{u, at})
}
func (tr stableIndexTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryEndData) {
	if at, ok := ctx.Value(stableIndexQueryKey{}).(stableIndexQueryAt); ok {
		u := at.unit
		u.TraceQueryEnd(ctx, conn, d)
		u.mu.Lock()
		u.clock++
		q := &u.queries[at.index]
		q.End, q.Done, q.Err = u.clock, true, d.Err
		if d.Err != nil && q.Name == u.failureSQL && u.cause == nil {
			u.cause = d.Err
		}
		u.mu.Unlock()
	}
	if tr.prior != nil {
		tr.prior.TraceQueryEnd(ctx, conn, d)
	}
}

// Extend existing real observer wrappers in place, retaining their positions and
// Before/ingest behavior. No participant, including rewrite, is replaced.
func stableIndexWire(t *testing.T, w wiring, c *stableIndexControl) wiring {
	t.Helper()
	prior := w.page
	w.queryTracer = stableIndexTracer{prior: w.queryTracer}
	w.page = func(d *page.Deps) {
		if prior != nil {
			prior(d)
		}
		c.pool = d.Pool
		d.Tx = stableIndexManager{Tx: d.Tx, control: c}
		found := 0
		var wrap func(page.PageObserver) page.PageObserver
		wrap = func(observer page.PageObserver) page.PageObserver {
			switch o := observer.(type) {
			case linkIndex:
				found++
				o.index.Store = stableIndexStore{Store: o.index.Store}
				o.index.Pages = stableIndexPages{Pages: o.index.Pages}
				o.index.Publisher = stableIndexPublisher{Publisher: o.index.Publisher}
				if o.index.Ingest != nil {
					ingest := *o.index.Ingest
					ingest.Marks = stableIndexMarks{IngestMarks: ingest.Marks}
					o.index.Ingest = &ingest
				}
				return stableIndexObserver{real: o, control: c}
			case ingestI34Observer:
				o.real = wrap(o.real)
				return o
			case ingestInterleaveObserver:
				o.real = wrap(o.real)
				return o
			default:
				return observer
			}
		}
		for i, observer := range d.Observers {
			d.Observers[i] = wrap(observer)
		}
		if found != 1 {
			t.Fatalf("stable Index fixture has %d original Index observers", found)
		}
	}
	return w
}

type stableIndexManager struct {
	page.Tx
	control *stableIndexControl
}

func (m stableIndexManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	m.control.mu.Lock()
	u := m.control.txUnit
	m.control.txUnit = nil
	m.control.mu.Unlock()
	if u == nil {
		return m.Tx.WithinTx(ctx, fn)
	}
	ctx = stableIndexMark(ctx, u)
	err := m.Tx.WithinTx(ctx, func(inner context.Context) error {
		inner = stableIndexContext{Context: inner, unit: u}
		u.note(inner, "callback_enter")
		err := fn(inner)
		u.mu.Lock()
		u.callbackErr = err
		u.stageLocked(inner, "callback_exit")
		u.mu.Unlock()
		return err
	})
	u.mu.Lock()
	u.managerErr = err
	u.clock++
	u.stages = append(u.stages, stableWiringStage{Name: "manager_exit", Order: u.clock})
	u.mu.Unlock()
	close(u.managerDone)
	return err
}

type stableIndexObserver struct {
	real    linkIndex
	control *stableIndexControl
}

func (o stableIndexObserver) PagesChanged(ctx context.Context, e page.Event) error {
	o.control.mu.Lock()
	var u *stableIndexUnit
	for _, change := range e.Changes {
		if candidate := o.control.units[change.NodeID]; candidate != nil && change.Revision == candidate.revision {
			if u != nil {
				o.control.mu.Unlock()
				return fmt.Errorf("stable fixture matched two primary units")
			}
			u = candidate
		}
	}
	o.control.mu.Unlock()
	if u == nil {
		return o.real.PagesChanged(ctx, e)
	}
	ctx = stableIndexMark(ctx, u)
	var body string
	var revision int
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		u.probe(fmt.Errorf("stable observer lost original Tx"))
	} else {
		u.probe(tx.QueryRow(ctx, `SELECT content,revision FROM page_contents WHERE node_id=$1`, u.page).Scan(&body, &revision))
	}
	u.mu.Lock()
	u.units++
	u.cs = e.ChangesetID
	u.writerBody, u.writerRevision = body, revision
	u.stageLocked(ctx, "observer")
	u.mu.Unlock()
	return o.real.PagesChanged(ctx, e)
}

type stableIndexStore struct{ linkingapp.Store }

func (s stableIndexStore) Lock(ctx context.Context, nb uuid.UUID) error {
	u := stableIndexCurrent(ctx)
	if u != nil {
		u.note(ctx, "lock_enter")
	}
	err := s.Store.Lock(ctx, nb)
	if u != nil {
		u.note(ctx, "lock_done")
	}
	return err
}
func (s stableIndexStore) ReplacePage(ctx context.Context, p linkingapp.Page, f linkingdomain.Facts) (linkingapp.Dropped, error) {
	u := stableIndexCurrent(ctx)
	if u != nil {
		u.mu.Lock()
		u.captureReading = false
		u.stageLocked(ctx, "replace_enter")
		u.mu.Unlock()
	}
	if u != nil && p.ID == u.page {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			u.probe(fmt.Errorf("stable Replace lost original Tx"))
		} else {
			rows, err := stableWiringRows(ctx, tx, p.ID)
			u.probe(err)
			var head int
			u.probe(tx.QueryRow(ctx, `SELECT revision FROM indexed_pages WHERE node_id=$1`, p.ID).Scan(&head))
			u.mu.Lock()
			u.before = rows
			u.beforeHead = head
			u.incoming = slices.Clone(f.Links)
			u.replaceCalls++
			u.mu.Unlock()
		}
	}
	dropped, err := s.Store.ReplacePage(ctx, p, f)
	if u == nil || p.ID != u.page || err != nil {
		return dropped, err
	}
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		u.probe(fmt.Errorf("stable Replace return lost original Tx"))
		return dropped, err
	}
	rows, probe := stableWiringRows(ctx, tx, p.ID)
	u.probe(probe)
	var head, extractor int
	u.probe(tx.QueryRow(ctx, `SELECT revision,extractor FROM indexed_pages WHERE node_id=$1`, p.ID).Scan(&head, &extractor))
	u.mu.Lock()
	u.replaced = rows
	u.afterHead, u.extractor = head, extractor
	u.dropped = linkingapp.Dropped{Targets: slices.Clone(dropped.Targets), AliasKeys: slices.Clone(dropped.AliasKeys)}
	u.stageLocked(ctx, "replaced")
	u.mu.Unlock()
	if u.afterReplace != nil {
		close(u.afterReplace)
		select {
		case <-u.allowFresh:
		case <-ctx.Done():
		}
	}
	return dropped, err
}
func (s stableIndexStore) Links(ctx context.Context, nb uuid.UUID, reach linkingdomain.Reach) ([]linkingapp.Link, error) {
	got, err := s.Store.Links(ctx, nb, reach)
	if u := stableIndexCurrent(ctx); u != nil {
		u.mu.Lock()
		u.links = slices.Clone(got)
		u.reach = linkingdomain.Reach{Sources: slices.Clone(reach.Sources), Targets: slices.Clone(reach.Targets), Keys: slices.Clone(reach.Keys)}
		u.fresh = true
		u.stageLocked(ctx, "links")
		u.mu.Unlock()
	}
	return got, err
}
func (s stableIndexStore) Aliases(ctx context.Context, nb uuid.UUID, keys []string) ([]linkingapp.Alias, error) {
	got, err := s.Store.Aliases(ctx, nb, keys)
	if u := stableIndexCurrent(ctx); u != nil {
		u.mu.Lock()
		if u.fresh {
			u.aliases = slices.Clone(got)
			u.stageLocked(ctx, "aliases")
		}
		u.mu.Unlock()
	}
	return got, err
}
func (s stableIndexStore) SetResolutions(ctx context.Context, links []linkingapp.Link) error {
	u := stableIndexCurrent(ctx)
	if u != nil {
		u.mu.Lock()
		u.fresh = false
		u.changed = slices.Clone(links)
		u.setCalls++
		u.stageLocked(ctx, "set")
		u.mu.Unlock()
	}
	err := s.Store.SetResolutions(ctx, links)
	if u != nil && err == nil {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			u.probe(fmt.Errorf("stable Set lost original Tx"))
		} else {
			rows, probe := stableWiringRows(ctx, tx, u.page)
			u.probe(probe)
			u.mu.Lock()
			u.afterSet = rows
			u.stageLocked(ctx, "set_finished")
			u.mu.Unlock()
		}
	}
	return err
}

type stableIndexPages struct{ linkingapp.Pages }

func stableIndexNodes(nodes []linkingdomain.Node) []linkingdomain.Node {
	out := slices.Clone(nodes)
	for i := range out {
		out[i].Path = slices.Clone(out[i].Path)
	}
	return out
}
func (p stableIndexPages) ByKeys(ctx context.Context, nb uuid.UUID, keys []string) ([]linkingdomain.Node, error) {
	got, err := p.Pages.ByKeys(ctx, nb, keys)
	if u := stableIndexCurrent(ctx); u != nil {
		u.mu.Lock()
		if u.fresh {
			u.keys = slices.Clone(keys)
			u.candidateNodes = stableIndexNodes(got)
			u.stageLocked(ctx, "bykeys")
		}
		u.mu.Unlock()
	}
	return got, err
}
func (p stableIndexPages) Paths(ctx context.Context, nb uuid.UUID, ids []uuid.UUID) ([]linkingdomain.Node, error) {
	got, err := p.Pages.Paths(ctx, nb, ids)
	if u := stableIndexCurrent(ctx); u != nil {
		u.mu.Lock()
		if u.fresh {
			u.pathInputs = slices.Clone(ids)
			u.pathNodes = stableIndexNodes(got)
			u.stageLocked(ctx, "paths")
		}
		u.mu.Unlock()
	}
	return got, err
}

type stableIndexMarks struct{ linking.IngestMarks }

func (m stableIndexMarks) Add(ctx context.Context, nb, cs uuid.UUID, ids []uuid.UUID) (linking.IngestMarkResult, error) {
	u := stableIndexCurrent(ctx)
	if u != nil {
		u.mu.Lock()
		u.markCalls++
		u.markIDs = slices.Clone(ids)
		u.stageLocked(ctx, "marks")
		u.mu.Unlock()
	}
	result, err := m.IngestMarks.Add(ctx, nb, cs, ids)
	if u != nil && err == nil {
		var count int
		if tx, ok := postgres.TxFrom(ctx); ok {
			u.probe(tx.QueryRow(ctx, `SELECT count(*) FROM changeset_sources WHERE changeset_id=$1`, cs).Scan(&count))
		} else {
			u.probe(fmt.Errorf("stable Add lost Tx"))
		}
		u.mu.Lock()
		u.markResult, u.storedMarks = result, count
		u.stageLocked(ctx, "marks_finished")
		u.mu.Unlock()
	}
	return result, err
}

type stableIndexPublisher struct{ linking.Publisher }

func (p stableIndexPublisher) LinksChanged(ctx context.Context, event linking.LinksChanged) error {
	u := stableIndexCurrent(ctx)
	if u != nil {
		u.mu.Lock()
		u.publishCalls++
		u.published = event
		u.published.Pages = slices.Clone(event.Pages)
		u.published.Targets = slices.Clone(event.Targets)
		u.stageLocked(ctx, "publish")
		u.mu.Unlock()
	}
	err := p.Publisher.LinksChanged(ctx, event)
	if u == nil || err != nil {
		return err
	}
	u.mu.Lock()
	u.stageLocked(ctx, "published")
	cause := u.publisherCause
	if cause != nil {
		u.cause = cause
		u.stageLocked(ctx, "publisher_error")
	}
	u.mu.Unlock()
	return cause
}

// The expected rows below are literal test data, projected only into each
// public DTO's fields. No parse, resolver, key builder, or production SQL is used.
func stableIndexRow(p, nb uuid.UUID, start, end int32, kind, target, key string, id uuid.UUID, ambiguous, asset bool) stableWiringRow {
	r := stableWiringRow{Source: p, Notebook: nb, Start: start, End: end, Kind: kind, Target: target, Key: stableWiringText(key), Ambiguous: ambiguous, Asset: asset}
	if id != uuid.Nil() {
		r.Resolved = &id
	}
	return r
}
func stableIndexLiteralFacts(rows []stableWiringRow) []linkingdomain.Link {
	var out []linkingdomain.Link
	for _, r := range rows {
		out = append(out, linkingdomain.Link{Start: int(r.Start), End: int(r.End), Kind: r.Kind, Target: r.Target})
	}
	return out
}
func stableIndexLiteralLinks(rows []stableWiringRow) []linkingapp.Link {
	var out []linkingapp.Link
	for _, r := range rows {
		l := linkingapp.Link{SourceID: r.Source, Start: int(r.Start), Target: r.Target}
		if r.Resolved != nil {
			l.Resolution = linkingdomain.Resolution{ID: *r.Resolved, Ambiguous: r.Ambiguous, Asset: r.Asset}
		}
		out = append(out, l)
	}
	return out
}
func stableIndexIDs(ids ...uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}
func stableIndexStage(u *stableIndexUnit, name string) int {
	for _, s := range u.stages {
		if s.Name == name {
			return s.Order
		}
	}
	return 0
}

type stableIndexWant struct {
	before, replaced, final []stableWiringRow
	changed                 []linkingapp.Link
	dropped                 []uuid.UUID
	keys                    []string
}

func stableIndexCheck(t *testing.T, tm acmeTeam, u *stableIndexUnit, want stableIndexWant) {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.units != 1 || u.replaceCalls != 1 || u.setCalls != 1 || len(u.probeFailures) != 0 {
		t.Fatalf("stable actual Index unit: units=%d Replace=%d Set=%d probes=%v", u.units, u.replaceCalls, u.setCalls, u.probeFailures)
	}
	if u.writerBody != u.body || u.writerRevision != u.revision || u.beforeHead != u.revision-1 || u.afterHead != u.revision || u.extractor != 2 {
		t.Fatalf("stable Writer/head fixture changed: body=%q revision=%d oldhead=%d newhead=%d extractor=%d", u.writerBody, u.writerRevision, u.beforeHead, u.afterHead, u.extractor)
	}
	if !reflect.DeepEqual(u.before, want.before) || !reflect.DeepEqual(u.incoming, stableIndexLiteralFacts(want.replaced)) {
		t.Fatalf("stable facts fixture was rewritten: old=%+v want=%+v actual incoming=%+v want=%+v", u.before, want.before, u.incoming, stableIndexLiteralFacts(want.replaced))
	}
	if !reflect.DeepEqual(u.replaced, want.replaced) {
		t.Errorf("stable retained baseline=%+v want full literal=%+v", u.replaced, want.replaced)
	}
	stableIndexCheckInspect(t, u, len(want.before))
	if u.inspectCalls != 1 || !u.inspectAllowed || !slices.Equal(u.inspectTargets, want.dropped) {
		t.Errorf("stable actual Inspect result: calls=%d allowed=%t old D=%v want=%v", u.inspectCalls, u.inspectAllowed, u.inspectTargets, want.dropped)
	}
	if !slices.Equal(u.dropped.Targets, want.dropped) || len(u.dropped.AliasKeys) != 0 {
		t.Errorf("stable complete old D=%+v want targets=%v and no old aliases", u.dropped, want.dropped)
	}
	if !slices.Equal(u.links, stableIndexLiteralLinks(want.replaced)) || !slices.Contains(u.reach.Sources, u.page) {
		t.Errorf("stable full fresh input=%+v reach=%+v want=%+v", u.links, u.reach, stableIndexLiteralLinks(want.replaced))
	}
	if !slices.Equal(u.changed, want.changed) || !slices.Equal(u.keys, want.keys) {
		t.Errorf("stable fresh Set=%+v keys=%v want=%+v keys=%v", u.changed, u.keys, want.changed, want.keys)
	}
	if !reflect.DeepEqual(u.afterSet, want.final) {
		t.Errorf("stable full post-Set rows=%+v want=%+v", u.afterSet, want.final)
	}
	committed, err := stableWiringRows(t.Context(), tm.pool, u.page)
	if err != nil || !reflect.DeepEqual(committed, want.final) {
		t.Errorf("stable full committed rows=%+v err=%v want=%+v", committed, err, want.final)
	}
	if got := tm.content(t, "alice", u.page.String()); got.Content != u.body || got.Revision != u.revision {
		t.Errorf("stable committed Writer body=%+v", got)
	}
	pid := uint32(0)
	for _, stage := range u.stages {
		if stage.Name == "manager_exit" {
			continue
		}
		if stage.PID == 0 || pid != 0 && stage.PID != pid {
			t.Fatalf("stable stage PID mismatch: %+v", u.stages)
		}
		pid = stage.PID
	}
	counts, fresh := map[string]int{}, map[string]int{}
	first := map[string]int{}
	linksOrder, setOrder := stableIndexStage(u, "links"), stableIndexStage(u, "set")
	var sets []stableWiringSQL
	for _, q := range u.sql {
		counts[q.Name]++
		if first[q.Name] == 0 {
			first[q.Name] = q.Order
		}
		if !q.Done || q.Err != nil || q.PID != pid {
			t.Errorf("stable SQL completion/PID: %+v", q)
		}
		if linksOrder < q.Order && q.Order < setOrder {
			fresh[q.Name]++
		}
		if q.Name == "SetResolutions" {
			sets = append(sets, q)
		}
	}
	for _, name := range []string{"InspectPageLinksForRetention", "DeleteMissingPageLinks", "WritePageLinksRetainingEqualPrefiltered"} {
		if counts[name] != 1 {
			t.Errorf("stable actual %s count=%d want1", name, counts[name])
		}
	}
	for _, name := range []string{"DeleteLinksOf", "InsertLinks", "WritePageLinksRetainingEqual"} {
		if counts[name] != 0 {
			t.Errorf("stable unexpected legacy SQL %s count=%d", name, counts[name])
		}
	}
	for _, name := range []string{"LinkTargetsByKeys", "AliasesByKeys", "LinkTargetsByIDs"} {
		if fresh[name] != 1 {
			t.Errorf("stable fresh window %s count=%d want1, whole-unit=%d", name, fresh[name], counts[name])
		}
	}
	sequence := []int{stableIndexStage(u, "observer"), stableIndexStage(u, "lock_done"), first["InspectPageLinksForRetention"], first["DeleteMissingPageLinks"], first["WritePageLinksRetainingEqualPrefiltered"], stableIndexStage(u, "replaced"), linksOrder, stableIndexStage(u, "bykeys"), stableIndexStage(u, "aliases"), stableIndexStage(u, "paths"), setOrder, stableIndexStage(u, "set_finished")}
	for i, n := range sequence {
		if n == 0 || i > 0 && n <= sequence[i-1] {
			t.Errorf("stable maintenance/fresh order=%v", sequence)
			break
		}
	}
	if len(sets) != 1 {
		t.Errorf("stable real Set SQL count=%d want1", len(sets))
	} else {
		q := sets[0]
		if q.Mode != pgx.QueryExecModeDescribeExec || q.BadArgs != "" || q.Rows != int64(len(want.changed)) || len(q.Sources) != len(want.changed) || len(q.Starts) != len(want.changed) || len(q.Targets) != len(want.changed) || len(q.Ambiguous) != len(want.changed) || len(q.Assets) != len(want.changed) {
			t.Errorf("stable actual Set parameters/count=%+v want=%+v", q, want.changed)
		} else {
			for i, l := range want.changed {
				if q.Sources[i] != l.SourceID || q.Starts[i] != int32(l.Start) || q.Targets[i] != l.Resolution.ID || q.Ambiguous[i] != l.Resolution.Ambiguous || q.Assets[i] != l.Resolution.Asset {
					t.Errorf("stable Set row%d=%+v want=%+v", i, q, l)
				}
			}
		}
	}
}

func stableIndexCheckPath(t *testing.T, u *stableIndexUnit, id uuid.UUID, names ...string) {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	found := 0
	for _, n := range u.pathNodes {
		if n.ID != id {
			continue
		}
		found++
		var got []string
		for _, s := range n.Path {
			got = append(got, s.Name)
		}
		if !slices.Equal(got, names) {
			t.Errorf("stable actual fresh path %s=%v want=%v", id, got, names)
		}
	}
	if found != 1 || !slices.Contains(u.pathInputs, id) {
		t.Errorf("stable fresh path %s present%d inputs=%v", id, found, u.pathInputs)
	}
}
func stableIndexCheckCandidates(t *testing.T, u *stableIndexUnit, ids ...uuid.UUID) {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	var got []uuid.UUID
	for _, n := range u.candidateNodes {
		got = append(got, n.ID)
	}
	slices.SortFunc(got, uuid.UUID.Compare)
	if !slices.Equal(got, stableIndexIDs(ids...)) {
		t.Errorf("stable actual candidates=%v want=%v", got, ids)
	}
}
func stableIndexEvents(t *testing.T, tm acmeTeam, nb, marker string, stream *eventStream, tree bool, revision int, p string, targets ...string) {
	t.Helper()
	e := stream.pages(t, nb)
	if e.Tree != tree || !reflect.DeepEqual(e.written(), map[string]int{p: revision}) {
		t.Errorf("stable real pages event=%+v", e)
	}
	stream.links(t, nb, []string{}, targets)
	tm.quiet(t, nb, marker, stream)
}

func stableIndexCheckCapture(t *testing.T, u *stableIndexUnit) {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	firstInspect := 0
	pid := uint32(0)
	for _, stage := range u.stages {
		if stage.Name == "observer" {
			pid = stage.PID
		}
	}
	for _, q := range u.sql {
		if q.Name == "InspectPageLinksForRetention" && (firstInspect == 0 || q.Order < firstInspect) {
			firstInspect = q.Order
		}
	}
	firstHeads, lastHeads := stableIndexStage(u, "capture_heads_enter"), 0
	for _, stage := range u.stages {
		if stage.Name == "capture_heads_done" {
			lastHeads = stage.Order
		}
	}
	if len(u.probeFailures) != 0 || pid == 0 || firstInspect == 0 || stableIndexStage(u, "lock_done") >= firstHeads || firstHeads >= lastHeads || lastHeads >= stableIndexStage(u, "replace_enter") || stableIndexStage(u, "replace_enter") >= firstInspect {
		t.Fatalf("stable production Capture/Inspect order: inspect=%d heads=%d..%d stages=%+v probes=%v", firstInspect, firstHeads, lastHeads, u.stages, u.probeFailures)
	}
	rows := 0
	for _, read := range u.captureRows {
		rows += read.Rows
		if read.PID != pid || !read.EOF || !read.Closed || read.Err != nil || read.Start <= firstHeads || read.End <= read.Start || read.CloseOrder <= read.Start || read.ErrOrder <= read.Start || read.End >= stableIndexStage(u, "replace_enter") || read.CloseOrder >= stableIndexStage(u, "replace_enter") || read.ErrOrder >= stableIndexStage(u, "replace_enter") {
			t.Errorf("stable production old read was not complete before Inspect: %+v firstInspect=%d", read, firstInspect)
		}
	}
	if len(u.captureRows) == 0 || rows == 0 {
		t.Fatal("stable Capture observation has no actual nonempty production old reads")
	}
	for _, q := range u.queries {
		if q.Start > firstHeads && q.Start < firstInspect && (q.PID != pid || !q.Done || q.Err != nil || q.End >= firstInspect) {
			t.Errorf("stable production query crossed Inspect: %+v", q)
		}
	}
	// beginIngest calls the private Capture before Replace; the query evidence
	// records its real old readers, not a pretend public Capture-return hook.
}

func stableIndexCheckSessionRetention(t *testing.T, u *stableIndexUnit, p, nb, s, target uuid.UUID) {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	old := []stableWiringRow{stableIndexRow(p, nb, 2, 7, "wikilink", "raw/S", "s", s, false, false)}
	replaced := []stableWiringRow{old[0], stableIndexRow(p, nb, 12, 17, "wikilink", "raw/T", "t", uuid.Nil(), false, false)}
	final := []stableWiringRow{old[0], stableIndexRow(p, nb, 12, 17, "wikilink", "raw/T", "t", target, false, false)}
	if u.units != 1 || u.replaceCalls != 1 || u.beforeHead != 2 || u.afterHead != 3 || u.extractor != 2 || len(u.probeFailures) != 0 || u.writerBody != "[[raw/S]] [[raw/T]]" || u.writerRevision != 3 {
		t.Fatalf("U09 stable baseline fixture: units=%d replaces=%d heads=%d/%d extractor=%d body=%q revision=%d probes=%v", u.units, u.replaceCalls, u.beforeHead, u.afterHead, u.extractor, u.writerBody, u.writerRevision, u.probeFailures)
	}
	if !reflect.DeepEqual(u.before, old) || !reflect.DeepEqual(u.incoming, stableIndexLiteralFacts(replaced)) || !reflect.DeepEqual(u.replaced, replaced) || !reflect.DeepEqual(u.afterSet, final) {
		t.Errorf("U09 stable complete old/retained/fresh tuples: old=%+v incoming=%+v retained=%+v fresh=%+v", u.before, u.incoming, u.replaced, u.afterSet)
	}
	stableIndexCheckInspect(t, u, len(old))
	if u.inspectCalls != 1 || !u.inspectAllowed || !slices.Equal(u.inspectTargets, []uuid.UUID{s}) || !slices.Equal(u.dropped.Targets, []uuid.UUID{s}) || len(u.dropped.AliasKeys) != 0 || !slices.Equal(u.links, stableIndexLiteralLinks(replaced)) || !slices.Equal(u.changed, stableIndexLiteralLinks(final[1:])) {
		t.Errorf("U09 stable complete D/fresh input: inspect=%d/%t/%v D=%+v links=%+v changed=%+v", u.inspectCalls, u.inspectAllowed, u.inspectTargets, u.dropped, u.links, u.changed)
	}
	counts := map[string]int{}
	for _, q := range u.sql {
		counts[q.Name]++
	}
	for _, name := range []string{"InspectPageLinksForRetention", "DeleteMissingPageLinks", "WritePageLinksRetainingEqualPrefiltered"} {
		if counts[name] != 1 {
			t.Errorf("U09 stable real %s=%d want1", name, counts[name])
		}
	}
	if counts["DeleteLinksOf"] != 0 || counts["InsertLinks"] != 0 {
		t.Errorf("U09 stable used legacy SQL: %v", counts)
	}
	// This is an intermediate observation only. The ORIGINAL U09 oracle above
	// already checked rollback and actual executor identity; committed state is
	// intentionally the old session's S, not these in-transaction S/T rows.
}
