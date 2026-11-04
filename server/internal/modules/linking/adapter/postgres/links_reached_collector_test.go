package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// These tests run the generated SQL on migrated PostgreSQL. The wrappers
// forward native Next/Err/Close; only the named Scan-fault cases alter Scan.
func newLinksCollectorPool(t *testing.T) (*pgxpool.Pool, *linksCollectorTrace) {
	t.Helper()
	base, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	cfg := base.Config()
	base.Close()
	trace := &linksCollectorTrace{}
	cfg.ConnConfig.Tracer = trace // Keep the platform's real AfterConnect.
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, trace
}

func linksCollectorCopy(t *testing.T, pool *pgxpool.Pool, rows [][]any) {
	t.Helper()
	if len(rows) == 0 {
		return
	}
	n, err := pool.CopyFrom(context.Background(), pgx.Identifier{"page_links"}, []string{
		"source_id", "range_start", "range_end", "notebook_id", "kind", "target", "target_key", "target_alt_key",
		"resolved_id", "ambiguous", "resolved_asset", "aliases",
	}, pgx.CopyFromRows(rows))
	if err != nil || n != int64(len(rows)) {
		t.Fatalf("seed %d/%d links: %v", n, len(rows), err)
	}
}

func TestLinksReachedCollectorPreservesPGValues(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	book, foreign := uuid.NewV7(), uuid.NewV7()
	p1, p2, p3 := uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 3}
	r1, r2 := uuid.UUID{15: 11}, uuid.UUID{15: 12}
	linksCollectorCopy(t, pool, [][]any{
		{p3, int32(0), int32(1), foreign, "link", "foreign", "reach", nil, r1, false, false, false},
		{p2, int32(2147483646), int32(2147483647), book, "image", "zero UUID", nil, nil, uuid.UUID{}, true, false, true},
		{p2, int32(3), int32(4), book, "link", "page", nil, nil, r1, false, true, false},
		{p1, int32(20), int32(21), book, "wikilink", "unresolved", nil, "reach", nil, false, false, false},
		{p1, int32(10), int32(11), book, "embed", "Straße/雪/\"\\", "reach", nil, r2, true, true, true},
		{p1, int32(0), int32(1), book, "wikilink", "page", "reach", nil, r1, false, false, false},
	})
	want := []app.Link{
		{SourceID: p1, Start: 0, Target: "page", Resolution: domain.Resolution{ID: r1}},
		{SourceID: p1, Start: 10, Target: "Straße/雪/\"\\", Resolution: domain.Resolution{ID: r2, Ambiguous: true, Asset: true}, Aliases: true},
		{SourceID: p1, Start: 20, Target: "unresolved"},
		{SourceID: p2, Start: 3, Target: "page", Resolution: domain.Resolution{ID: r1, Asset: true}},
		{SourceID: p2, Start: 2147483646, Target: "zero UUID", Resolution: domain.Resolution{Ambiguous: true}, Aliases: true},
	}
	reach := domain.Reach{Keys: []string{"reach"}, Targets: []uuid.UUID{r1}, Sources: []uuid.UUID{p2}}
	got, err := New(pool).Links(context.Background(), book, reach)
	if err != nil || !slices.Equal(got, want) || cap(got) != len(want) {
		t.Fatalf("Links=%+v, %v; want literal %+v with exact capacity", got, err, want)
	}
	// In particular, NULL after a resolved row clears the whole Resolution;
	// a non-NULL zero UUID with Ambiguous=true is not the NULL case.
	if got[2].Resolution != (domain.Resolution{}) || !got[4].Resolution.Ambiguous {
		t.Fatal("NULL and non-NULL zero UUID became indistinguishable")
	}
	got, err = New(pool).Links(context.Background(), book, domain.Reach{})
	if err != nil || got == nil || len(got) != 0 || cap(got) != 0 {
		t.Fatalf("empty Links=%v (nil=%t, cap=%d), %v", got, got == nil, cap(got), err)
	}
}

// Each range and target is unique. Source changes at 512; NULL/non-NULL
// alternation continues across both block boundaries. Reverse insertion
// means the SQL's ORDER BY, rather than fixture order, determines the result.
func linksCollectorSeedMany(t *testing.T, pool *pgxpool.Pool, book uuid.UUID, n int) ([]app.Link, domain.Reach) {
	t.Helper()
	p1, p2, resolved := uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 11}
	want, data := make([]app.Link, n), make([][]any, n)
	for i := range n {
		source := p1
		if i >= 512 {
			source = p2
		}
		target, start, aliases := fmt.Sprintf("target/%04d/雪", i), int32(i*2), i%3 == 0
		var id any
		resolution := domain.Resolution{}
		if i%2 == 0 {
			id = resolved
			resolution = domain.Resolution{ID: resolved, Ambiguous: true, Asset: true}
		}
		data[i] = []any{source, start, start + 1, book, "wikilink", target, nil, nil, id,
			resolution.Ambiguous, resolution.Asset, aliases}
		want[i] = app.Link{SourceID: source, Start: int(start), Target: target, Resolution: resolution, Aliases: aliases}
	}
	slices.Reverse(data)
	linksCollectorCopy(t, pool, data)
	return want, domain.Reach{Sources: []uuid.UUID{p1, p2}}
}

func TestLinksReachedCollectorUsesBoundedBlocks(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	book := uuid.NewV7()
	for _, tc := range []struct{ n, full, tail, capacity int }{
		{0, 0, 0, 0}, {1, 0, 1, 8}, {8, 0, 8, 8}, {9, 0, 9, 16},
		{511, 0, 511, 512}, {512, 0, 512, 512}, {513, 1, 1, 512},
		{1024, 1, 512, 512}, {1025, 2, 1, 512},
	} {
		t.Run(fmt.Sprint(tc.n), func(t *testing.T) {
			if _, err := pool.Exec(context.Background(), "TRUNCATE page_links"); err != nil {
				t.Fatal(err)
			}
			want, reach := linksCollectorSeedMany(t, pool, book, tc.n)
			conn, err := pool.Acquire(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			rows := &linksCollectorRows{stable: true}
			q := &linksCollectorQuerier{Querier: conn, rows: rows}
			native, err := postgres.Planned(q).Query(context.Background(), gen.LinksReached, book, reach.Keys, reach.Targets, reach.Sources)
			if err != nil {
				t.Fatal(err)
			}
			var b linksReachedBuffer
			got, err := b.read(native)
			if q.calls != 1 {
				t.Fatalf("native Query calls=%d, want one", q.calls)
			}
			if err != nil || got == nil || !slices.Equal(got, want) || cap(got) != tc.n {
				t.Fatalf("read len/cap=%d/%d, nil=%t, err=%v; want %d exact values", len(got), cap(got), got == nil, err, tc.n)
			}
			if b.count != tc.n || len(b.full) != tc.full || len(b.tail) != tc.tail || cap(b.tail) != tc.capacity {
				t.Fatalf("buffer count/full/tail/cap=%d/%d/%d/%d, want %d/%d/%d/%d", b.count, len(b.full), len(b.tail), cap(b.tail), tc.n, tc.full, tc.tail, tc.capacity)
			}
			for _, block := range b.full {
				if len(block) != 512 || cap(block) != 512 {
					t.Fatalf("full block len/cap=%d/%d", len(block), cap(block))
				}
				block[0].Target = "changed after finish"
			}
			if len(b.tail) != 0 {
				b.tail[0].Target = "changed after finish"
			}
			if !slices.Equal(got, want) {
				t.Fatal("final output shares a block's backing storage")
			}
			rows.check(t, tc.n, 1, tc.n, 1)
			trace.checkLast(t, conn.Conn().PgConn().PID(), book, reach, nil)
			linksCollectorReusable(t, conn)
		})
	}
}

func TestLinksReachedCollectorActualStoreUsesOneScratch(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	book := uuid.NewV7()
	want, reach := linksCollectorSeedMany(t, pool, book, 513)
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	restore := linksCollectorObserveUUID(t, conn.Conn(), trace)
	defer restore()
	ctx := postgres.OnConn(context.Background(), conn)
	store := New(pool)
	check := func(ctx context.Context, want []app.Link) {
		t.Helper()
		before := trace.count()
		got, err := store.Links(ctx, book, reach)
		if trace.count() != before+1 {
			t.Fatalf("actual Store made %d generated queries, want one", trace.count()-before)
		}
		if err != nil || !slices.Equal(got, want) || cap(got) != len(want) {
			t.Fatalf("actual Store: len/cap=%d/%d, %v; want %d exact values", len(got), cap(got), err, len(want))
		}
		call := trace.checkLast(t, conn.Conn().PgConn().PID(), book, reach, nil)
		if call.sourceScans != len(want) || len(call.sourceDest) != 1 || len(call.resolvedDest) == 0 {
			t.Fatalf("actual Store source scans/destinations=%d/%d, resolved destinations=%d; want %d/1 and nonempty", call.sourceScans, len(call.sourceDest), len(call.resolvedDest), len(want))
		}
		for dst := range call.sourceDest {
			if call.resolvedDest[dst] {
				t.Fatal("SourceID and resolved UUID share a destination")
			}
		}
	}
	check(ctx, want)
	rollback := errors.New("collector test rollback")
	err = postgres.NewTxManager(pool, time.Second).WithinTx(ctx, func(txCtx context.Context) error {
		tx, ok := postgres.TxFrom(txCtx)
		if !ok || tx.Conn() != conn.Conn() {
			t.Fatal("callback does not carry the owned connection's actual transaction")
		}
		if _, err := tx.Exec(txCtx, `INSERT INTO page_links
			(source_id, range_start, range_end, notebook_id, kind, target, ambiguous)
			VALUES ($1, 3000, 3001, $2, 'link', 'only inside transaction', false)`, uuid.UUID{15: 2}, book); err != nil {
			return err
		}
		inside := append(slices.Clone(want), app.Link{SourceID: uuid.UUID{15: 2}, Start: 3000, Target: "only inside transaction"})
		check(txCtx, inside)
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction=%v, want original rollback cause", err)
	}
	check(ctx, want)
	linksCollectorReusable(t, conn)
}

func TestLinksReachedCollectorPreservesErrorsAndClose(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	book := uuid.NewV7()
	want, reach := linksCollectorSeedMany(t, pool, book, 1025)
	for _, mode := range []string{"scan_513", "err_after_512", "cancel_after_eof"} {
		t.Run(mode, func(t *testing.T) {
			conn, err := pool.Acquire(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rows := &linksCollectorRows{stable: true, mode: mode, cancel: cancel}
			q := &linksCollectorQuerier{Querier: conn, rows: rows}
			got, err := linksReached(ctx, postgres.Planned(q), book, reach)
			if q.calls != 1 {
				t.Fatalf("native Query calls=%d, want one", q.calls)
			}
			if ctx.Err() != context.Canceled {
				t.Fatal("the actual Query context was not canceled at the designated native boundary")
			}
			switch mode {
			case "scan_513":
				rows.check(t, 513, 0, 513, 0)
			case "err_after_512":
				rows.check(t, 512, 1, 512, 1)
			case "cancel_after_eof":
				rows.check(t, 1025, 1, 1025, 1)
				if err != nil || !slices.Equal(got, want) || cap(got) != 1025 {
					t.Fatalf("after native EOF: len=%d, err=%v; want complete successful result", len(got), err)
				}
			}
			if mode != "cancel_after_eof" {
				//nolint:errorlint // Scan and Rows.Err must expose the original native error object.
				if got != nil || rows.cause == nil || err != rows.cause || rows.Rows.Err() != rows.cause {
					t.Fatalf("native cause=%v, result nil=%t, err=%v; want original cause and no partial output", rows.cause, got == nil, err)
				}
				if errors.Is(err, context.Canceled) {
					t.Fatal("cancellation replaced the earlier native Scan error")
				}
			}
			trace.checkLast(t, conn.Conn().PgConn().PID(), book, reach, rows.cause)
			linksCollectorReusable(t, conn)
		})
	}
}

func TestLinksReachedCollectorQueryErrorAndStoreWrapping(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if err := conn.Conn().Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	book := uuid.NewV7()
	rows := &linksCollectorRows{stable: true}
	q := &linksCollectorQuerier{Querier: conn, rows: rows}
	got, err := linksReached(context.Background(), postgres.Planned(q), book, domain.Reach{})
	//nolint:errorlint // Query must return the original native error object.
	if got != nil || q.cause == nil || err != q.cause || q.calls != 1 {
		t.Fatalf("Query result=%v, error=%v, native cause=%v, calls=%d", got, err, q.cause, q.calls)
	}
	if rows.nextTrue != 0 || rows.nextFalse != 0 || rows.scans != 0 || rows.errs != 0 || rows.closes != 0 {
		t.Fatal("Query failure was followed by Rows operations")
	}
	// Pool acquisition has a stable native error identity. This is a real
	// public Store failure, not a substitute Store or injected product hook.
	closed, _ := newLinksCollectorPool(t)
	closed.Close()
	_, cause := closed.Query(context.Background(), gen.LinksReached, book, []string(nil), []uuid.UUID(nil), []uuid.UUID(nil))
	got, err = New(closed).Links(context.Background(), book, domain.Reach{})
	if got != nil || cause == nil || !errors.Is(err, cause) || err.Error() != fmt.Sprintf("links of %s: %s", book, cause) {
		t.Fatalf("Store result=%v error=%v; want links-of wrapper of %v", got, err, cause)
	}
}

type linksCollectorQuerier struct {
	postgres.Querier
	rows  *linksCollectorRows
	cause error
	calls int
}

func (q *linksCollectorQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.calls++
	rows, err := q.Querier.Query(ctx, sql, args...)
	q.cause = err
	if rows != nil {
		q.rows.Rows = rows
		return q.rows, err
	}
	return rows, err
}

type linksCollectorRows struct {
	pgx.Rows
	dest                       []any
	nextTrue, nextFalse, scans int
	errs, closes, columns      int
	stable                     bool
	mode                       string
	cancel                     context.CancelFunc
	cause                      error
}

func (r *linksCollectorRows) columnCount() int {
	if r.columns != 0 {
		return r.columns
	}
	return 7
}

func (r *linksCollectorRows) Next() bool {
	ok := r.Rows.Next() // Never invent EOF, limit rows, or replace native Err.
	if ok {
		r.nextTrue++
	} else {
		r.nextFalse++
		if r.mode == "cancel_after_eof" {
			r.cancel() // Native Next already closed/drained Rows and traced its end.
		}
	}
	return ok
}

func (r *linksCollectorRows) Scan(dest ...any) error {
	r.scans++
	if r.scans == 1 {
		r.dest = slices.Clone(dest)
	}
	r.stable = r.stable && slices.Equal(dest, r.dest) && len(dest) == r.columnCount()
	if r.mode == "scan_513" && r.scans == 513 {
		r.cause = r.Rows.Scan(dest[:r.columnCount()-1]...) // Native count mismatch calls fatal/Close.
		r.cancel()                                         // Only after the native cause and Close.
		return r.cause
	}
	err := r.Rows.Scan(dest...)
	if err == nil && r.mode == "err_after_512" && r.scans == 512 {
		// A second native Scan on this same row creates a real Rows.Err.
		// Return the first Scan's success; next native Next observes closed Rows.
		r.cause = r.Rows.Scan(dest[:r.columnCount()-1]...)
		r.cancel()
	}
	return err
}

func (r *linksCollectorRows) Err() error {
	r.errs++
	return r.Rows.Err()
}

func (r *linksCollectorRows) Close() {
	r.closes++
	r.Rows.Close()
}

func (r *linksCollectorRows) check(t *testing.T, nextTrue, nextFalse, scans, errs int) {
	t.Helper()
	if r.nextTrue != nextTrue || r.nextFalse != nextFalse || r.scans != scans || r.errs != errs || r.closes != 1 || !r.stable {
		t.Fatalf("native forwarding true/false/scan/err/close=%d/%d/%d/%d/%d stable=%t; want %d/%d/%d/%d/1 stable", r.nextTrue, r.nextFalse, r.scans, r.errs, r.closes, r.stable, nextTrue, nextFalse, scans, errs)
	}
	if scans != 0 {
		unique := map[any]bool{}
		for _, dst := range r.dest {
			unique[dst] = true
		}
		if len(unique) != r.columnCount() {
			t.Fatalf("the %d Scan fields do not have independent destinations", r.columnCount())
		}
	}
}

func linksCollectorReusable(t *testing.T, conn *pgxpool.Conn) {
	t.Helper()
	// Run only after read returned; never reenter SQL from a busy tracer.
	if conn.Conn().PgConn().IsBusy() {
		t.Fatal("Rows left the real connection busy")
	}
	if _, err := conn.Exec(context.Background(), "SELECT 1"); err != nil {
		t.Fatalf("reuse connection after Close: %v", err)
	}
}

type linksCollectorCall struct {
	pid                      uint32
	args                     []any
	prepares                 []string
	ends, sourceScans        int
	err                      error
	sourceDest, resolvedDest map[*uuid.UUID]bool
}

type linksCollectorTrace struct {
	sql    string // Empty selects LinksReached; retention tests set the other exact generated SQL.
	mu     sync.Mutex
	calls  []*linksCollectorCall
	active *linksCollectorCall
}

type linksCollectorTraceKey struct{}

func (tr *linksCollectorTrace) matches(sql string) bool {
	if tr.sql != "" {
		return sql == tr.sql
	}
	return sql == gen.LinksReached
}

func (tr *linksCollectorTrace) count() int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return len(tr.calls)
}

func (tr *linksCollectorTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !tr.matches(data.SQL) {
		return ctx
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	call := &linksCollectorCall{pid: conn.PgConn().PID(), args: slices.Clone(data.Args), sourceDest: map[*uuid.UUID]bool{}, resolvedDest: map[*uuid.UUID]bool{}}
	tr.calls = append(tr.calls, call)
	tr.active = call
	return context.WithValue(ctx, linksCollectorTraceKey{}, call)
}

func (tr *linksCollectorTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if call, ok := ctx.Value(linksCollectorTraceKey{}).(*linksCollectorCall); ok {
		tr.mu.Lock()
		defer tr.mu.Unlock()
		call.ends++
		call.err = data.Err
		tr.active = nil
	}
}

func (tr *linksCollectorTrace) TracePrepareStart(ctx context.Context, _ *pgx.Conn, data pgx.TracePrepareStartData) context.Context {
	if call, ok := ctx.Value(linksCollectorTraceKey{}).(*linksCollectorCall); ok && tr.matches(data.SQL) {
		tr.mu.Lock()
		call.prepares = append(call.prepares, data.Name)
		tr.mu.Unlock()
	}
	return ctx
}

func (*linksCollectorTrace) TracePrepareEnd(context.Context, *pgx.Conn, pgx.TracePrepareEndData) {}

func (tr *linksCollectorTrace) checkLast(t *testing.T, pid uint32, book uuid.UUID, reach domain.Reach, cause error) linksCollectorCall {
	t.Helper()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.calls) == 0 {
		t.Fatal("no actual gen.LinksReached Query observed")
	}
	call := tr.calls[len(tr.calls)-1]
	// pgx v5.11.0 traces before optionLoop, so the first argument is mode.
	want := []any{pgx.QueryExecModeDescribeExec, book, reach.Keys, reach.Targets, reach.Sources}
	//nolint:errorlint // The trace must contain the same native error object.
	if call.pid != pid || !reflect.DeepEqual(call.args, want) || !slices.Equal(call.prepares, []string{""}) || call.ends != 1 || call.err != cause || tr.active != nil {
		t.Fatalf("native SQL call pid=%d args=%#v prepare=%q ends=%d error=%v active=%t; want pid=%d, DescribeExec+four args, one unnamed prepare/end, cause=%v", call.pid, call.args, call.prepares, call.ends, call.err, tr.active != nil, pid, cause)
	}
	return *call
}

// Observe the platform's existing exact *uuid.UUID wrapper on the Store's
// actual connection. Its SetNext, Scan input and result are forwarded without
// changing the codec, next target or destination. Rows owns and clears Scan
// plans on Close in pgx v5.11.0; Map has no memoized Scan plans to invalidate.
func linksCollectorObserveUUID(t *testing.T, conn *pgx.Conn, trace *linksCollectorTrace) func() {
	t.Helper()
	m := conn.TypeMap()
	original := m.TryWrapScanPlanFuncs
	if len(original) == 0 {
		t.Fatal("platform UUID Scan wrapper missing")
	}
	plan, next, ok := original[0](new(uuid.UUID))
	if _, array := next.(*[16]byte); !ok || plan == nil || !array {
		t.Fatal("first platform wrapper is not the exact standard UUID wrapper")
	}
	wrappers := slices.Clone(original)
	wrappers[0] = func(target any) (pgtype.WrappedScanPlanNextSetter, any, bool) {
		plan, next, ok := original[0](target)
		if !ok {
			return plan, next, ok
		}
		return &linksCollectorUUIDPlan{WrappedScanPlanNextSetter: plan, trace: trace, pid: conn.PgConn().PID()}, next, true
	}
	m.TryWrapScanPlanFuncs = wrappers
	return func() { m.TryWrapScanPlanFuncs = original }
}

type linksCollectorUUIDPlan struct {
	pgtype.WrappedScanPlanNextSetter
	trace *linksCollectorTrace
	pid   uint32
}

func (p *linksCollectorUUIDPlan) Scan(src []byte, target any) error {
	err := p.WrappedScanPlanNextSetter.Scan(src, target)
	if dst, ok := target.(*uuid.UUID); err == nil && ok {
		p.trace.mu.Lock()
		defer p.trace.mu.Unlock()
		if call := p.trace.active; call != nil && call.pid == p.pid {
			switch *dst {
			case uuid.UUID{15: 1}, uuid.UUID{15: 2}:
				call.sourceScans++
				call.sourceDest[dst] = true // Retain pointers; GC cannot recycle them.
			case uuid.UUID{15: 11}:
				call.resolvedDest[dst] = true
			}
		}
	}
	return err
}
