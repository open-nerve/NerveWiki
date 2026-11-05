package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Observe the real generated statement without retaining its reusable arrays.
// A test may gate/cancel only after the original Exec has actually finished.
type setWriteKey struct {
	Source uuid.UUID
	Start  int32
}
type setWriteValue struct {
	Key        setWriteKey
	Resolution domain.Resolution
}
type setWriteCall struct {
	Query                                  string
	Values                                 []setWriteValue
	PID                                    uint32
	Mode                                   pgx.QueryExecMode
	StatementCapacity, DescriptionCapacity int
	Lengths                                [5]int
	First, Last                            setWriteKey
	Rows                                   int64
	Err                                    error
	Done                                   bool
	BadArgs                                string
}
type setWriteTrace struct {
	mu       sync.Mutex
	calls    []setWriteCall
	prepared []lintPrepareObservation
	after    func(context.Context, int, setWriteCall)
}
type setWriteTraceKey struct{}
type setWriteTraceAt struct {
	trace *setWriteTrace
	index int
}

// Only these two generated identities belong to this private write protocol.
func setWriteQuery(sql string) string {
	for _, name := range []string{"SetResolutions", "SetOrderedSourceResolutions"} {
		if strings.HasPrefix(sql, "-- name: "+name+" ") {
			return name
		}
	}
	return ""
}

func (tr *setWriteTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	query := setWriteQuery(d.SQL)
	if query == "" {
		return ctx
	}
	cfg := conn.Config()
	call := setWriteCall{Query: query, PID: conn.PgConn().PID(), StatementCapacity: cfg.StatementCacheCapacity, DescriptionCapacity: cfg.DescriptionCacheCapacity}
	args := d.Args
	if len(args) > 0 {
		if mode, ok := args[0].(pgx.QueryExecMode); ok {
			call.Mode, args = mode, args[1:]
		}
	}
	if len(args) != 5 {
		call.BadArgs = fmt.Sprintf("got %d parameters, want five", len(args))
	} else {
		sources, sOK := args[0].([]uuid.UUID)
		starts, pOK := args[1].([]int32)
		targets, tOK := args[2].([]uuid.UUID)
		ambiguous, aOK := args[3].([]bool)
		assets, kOK := args[4].([]bool)
		if !sOK || !pOK || !tOK || !aOK || !kOK {
			call.BadArgs = fmt.Sprintf("parameter types %T %T %T %T %T", args[0], args[1], args[2], args[3], args[4])
		} else {
			call.Lengths = [5]int{len(sources), len(starts), len(targets), len(ambiguous), len(assets)}
			if len(starts) == len(sources) && len(targets) == len(sources) && len(ambiguous) == len(sources) && len(assets) == len(sources) {
				call.Values = make([]setWriteValue, len(sources))
				for i := range sources {
					call.Values[i] = setWriteValue{setWriteKey{sources[i], starts[i]}, domain.Resolution{ID: targets[i], Ambiguous: ambiguous[i], Asset: assets[i]}}
				}
			}
			if len(sources) > 0 && len(starts) == len(sources) {
				call.First = setWriteKey{sources[0], starts[0]}
				call.Last = setWriteKey{sources[len(sources)-1], starts[len(starts)-1]}
			}
		}
	}
	tr.mu.Lock()
	at := len(tr.calls)
	tr.calls = append(tr.calls, call)
	tr.mu.Unlock()
	return context.WithValue(ctx, setWriteTraceKey{}, setWriteTraceAt{tr, at})
}
func (tr *setWriteTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	at, ok := ctx.Value(setWriteTraceKey{}).(setWriteTraceAt)
	if !ok || at.trace != tr {
		return
	}
	tr.mu.Lock()
	call := &tr.calls[at.index]
	call.Rows, call.Err, call.Done = d.CommandTag.RowsAffected(), d.Err, true
	finished := *call
	tr.mu.Unlock()
	if tr.after != nil {
		tr.after(ctx, at.index, finished)
	}
}
func (tr *setWriteTrace) snapshot() []setWriteCall {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.calls)
}

// The original real repository fixture supplies migrations, manager and pools.
// Literal zero is a source, and the 4096th position crosses to source 1.
type setWriteFixture struct {
	fixture
	writer          *pgxpool.Pool
	trace           *setWriteTrace
	before, desired []app.Link
	sources         []uuid.UUID
}

func newSetWriteFixture(t *testing.T, n int) setWriteFixture {
	t.Helper()
	f := newFixture(t)
	for _, id := range []uuid.UUID{{}, lintID(1)} {
		f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
	}
	lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,resolved_id,ambiguous,resolved_asset)
 SELECT CASE WHEN k<4096 THEN $1::uuid ELSE $2::uuid END,
 CASE WHEN k<4096 THEN k ELSE k-4096 END,CASE WHEN k<4096 THEN k+1 ELSE k-4095 END,
 $3,'wikilink','x','x',$4,false,false FROM generate_series(0,$5::integer-1) k`, uuid.UUID{}, lintID(1), f.eng, lintID(240), n)
	tr := &setWriteTrace{}
	pool := lintPool(t, f, tr, 2)
	f.s, f.tx = postgresadapter.New(pool), postgres.NewTxManager(pool, time.Second)
	before, err := f.s.Links(t.Context(), f.eng, domain.Reach{Sources: []uuid.UUID{{}, lintID(1)}})
	if err != nil || len(before) != n {
		t.Fatalf("actual LinksReached seed: %d, %v; want %d", len(before), err, n)
	}
	if n > 0 && (before[0].SourceID != (uuid.UUID{}) || before[0].Start != 0) {
		t.Fatal("fixture lost literal zero source/start")
	}
	desired := slices.Clone(before)
	for i := range desired {
		switch i % 4 {
		case 0:
			desired[i].Resolution = domain.Resolution{}
		case 1:
			desired[i].Resolution = domain.Resolution{ID: lintID(250)}
		case 2:
			desired[i].Resolution = domain.Resolution{ID: lintID(250), Ambiguous: true}
		case 3:
			desired[i].Resolution = domain.Resolution{ID: lintID(251), Asset: true}
		}
	}
	return setWriteFixture{fixture: f, writer: pool, trace: tr, before: before, desired: desired, sources: []uuid.UUID{{}, lintID(1)}}
}
func (f setWriteFixture) assertRows(t *testing.T, want []app.Link) {
	t.Helper()
	sources := f.sources
	if sources == nil {
		sources = []uuid.UUID{{}, lintID(1)}
	}
	got, err := f.s.Links(t.Context(), f.eng, domain.Reach{Sources: sources})
	if err != nil || !slices.Equal(got, want) {
		at := 0
		for at < min(len(got), len(want)) && got[at] == want[at] {
			at++
		}
		t.Fatalf("persisted resolutions: got %d rows, want %d, first difference %d, err %v", len(got), len(want), at, err)
	}
}
func (f setWriteFixture) set(ctx context.Context, links []app.Link) (uint32, error) {
	var pid uint32
	err := f.tx.WithinTx(ctx, func(ctx context.Context) error {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			return errors.New("fixture has no original Tx")
		}
		pid = tx.Conn().PgConn().PID()
		if err := f.s.Lock(ctx, f.eng); err != nil {
			return err
		}
		return f.s.SetResolutions(ctx, links)
	})
	return pid, err
}
func (f setWriteFixture) assertCalls(t *testing.T, links []app.Link, lengths []int, pid uint32, wantMode pgx.QueryExecMode, queries ...string) []setWriteCall {
	t.Helper()
	calls := f.trace.snapshot()
	brief := slices.Clone(calls)
	for i := range brief {
		brief[i].Values = nil
	}
	t.Logf("SETWRITE actual: %+v", brief)
	if len(calls) != len(lengths) {
		t.Fatalf("SET_WRITE_BATCHES: got %d actual SQL calls, want lengths %v; calls=%+v", len(calls), lengths, brief)
	}
	if len(queries) != 0 && len(queries) != len(lengths) {
		t.Fatal("test query identities do not match expected lengths")
	}
	at := 0
	for i, c := range calls {
		n := lengths[i]
		query := "SetResolutions"
		if len(queries) > 0 {
			query = queries[i]
		}
		if c.Query != query {
			t.Fatalf("SET_WRITE_QUERY: statement%d=%q want%q", i, c.Query, query)
		}
		if c.BadArgs != "" || !c.Done || c.PID == 0 || pid != 0 && c.PID != pid || c.Mode != wantMode || c.Lengths != [5]int{n, n, n, n, n} {
			t.Fatalf("SET_WRITE_PARAMETERS: call %d=%+v, want five arrays of %d on writer %d through %v", i, c, n, pid, wantMode)
		}
		if n > 0 {
			if len(c.Values) != n {
				t.Fatalf("SET_WRITE_VALUES: statement%d observed%d want%d", i, len(c.Values), n)
			}
			for j, v := range c.Values {
				l := links[at+j]
				if v != (setWriteValue{setWriteKey{l.SourceID, int32(l.Start)}, l.Resolution}) {
					t.Fatalf("SET_WRITE_VALUES: statement%d row%d actual%+v want%+v", i, j, v, l)
				}
			}
			first, last := links[at], links[at+n-1]
			if c.First != (setWriteKey{first.SourceID, int32(first.Start)}) || c.Last != (setWriteKey{last.SourceID, int32(last.Start)}) {
				t.Fatalf("SET_WRITE_TAIL: call %d endpoints=%+v/%+v, want %+v/%+v", i, c.First, c.Last, first, last)
			}
		}
		at += n
	}
	return calls
}

// Expected statement counts are explicit fixture facts, independent of the
// private production strategy. full is a literal count of complete 256-row
// statements; tail is the literal final length, if any.
func setWrite256Lengths(full, tail int) []int {
	lengths := make([]int, full, full+1)
	for i := range lengths {
		lengths[i] = 256
	}
	if tail > 0 {
		lengths = append(lengths, tail)
	}
	return lengths
}

// Disabling bounded dispatch, skipping the tail or mixing any parameter vector
// fails against actual SQL calls and committed rows, not a private predicate.
func TestSetResolutionsUsesCompleteBoundedStatements(t *testing.T) {
	for _, tc := range []struct {
		name    string
		n       int
		lengths []int
		groups  [4]int
		mode    pgx.QueryExecMode
		queries []string
	}{
		{"nil", 0, []int{0}, [4]int{}, pgx.QueryExecModeDescribeExec, nil},
		{"empty", 0, []int{0}, [4]int{}, pgx.QueryExecModeDescribeExec, nil},
		{"255", 255, []int{255}, [4]int{64, 64, 64, 63}, pgx.QueryExecModeDescribeExec, nil},
		{"256", 256, []int{256}, [4]int{64, 64, 64, 64}, pgx.QueryExecModeDescribeExec, nil},
		{"257", 257, []int{257}, [4]int{65, 64, 64, 64}, pgx.QueryExecModeCacheDescribe, []string{"SetOrderedSourceResolutions"}},
		{"512", 512, []int{512}, [4]int{128, 128, 128, 128}, pgx.QueryExecModeCacheDescribe, []string{"SetOrderedSourceResolutions"}},
		{"513", 513, []int{513}, [4]int{129, 128, 128, 128}, pgx.QueryExecModeCacheDescribe, []string{"SetOrderedSourceResolutions"}},
		{"4095", 4095, []int{4095}, [4]int{1024, 1024, 1024, 1023}, pgx.QueryExecModeCacheDescribe, []string{"SetOrderedSourceResolutions"}},
		{"4096", 4096, []int{4096}, [4]int{1024, 1024, 1024, 1024}, pgx.QueryExecModeCacheDescribe, []string{"SetOrderedSourceResolutions"}},
		{"4097", 4097, []int{4096, 1}, [4]int{1025, 1024, 1024, 1024}, pgx.QueryExecModeCacheDescribe, []string{"SetOrderedSourceResolutions", "SetResolutions"}},
		{"8192", 8192, []int{4096, 4096}, [4]int{2048, 2048, 2048, 2048}, pgx.QueryExecModeCacheDescribe, []string{"SetOrderedSourceResolutions", "SetOrderedSourceResolutions"}},
		{"8193", 8193, []int{4096, 4096, 1}, [4]int{2049, 2048, 2048, 2048}, pgx.QueryExecModeCacheDescribe, []string{"SetOrderedSourceResolutions", "SetOrderedSourceResolutions", "SetResolutions"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSetWriteFixture(t, tc.n)
			links := f.desired
			if tc.name == "nil" {
				links = nil
			}
			if tc.name == "empty" {
				links = []app.Link{}
			}
			pid, err := f.set(t.Context(), links)
			if err != nil {
				t.Fatal(err)
			}
			f.assertRows(t, f.desired)
			var groups [4]int
			err = f.pool.QueryRow(t.Context(), `SELECT
 count(*) FILTER(WHERE resolved_id IS NULL AND NOT ambiguous AND NOT resolved_asset),
 count(*) FILTER(WHERE resolved_id=$1 AND NOT ambiguous AND NOT resolved_asset),
 count(*) FILTER(WHERE resolved_id=$1 AND ambiguous AND NOT resolved_asset),
 count(*) FILTER(WHERE resolved_id=$2 AND NOT ambiguous AND resolved_asset) FROM page_links`, lintID(250), lintID(251)).Scan(&groups[0], &groups[1], &groups[2], &groups[3])
			if err != nil || groups != tc.groups {
				t.Fatalf("independent groups=%v want%v: %v", groups, tc.groups, err)
			}
			for i, c := range f.assertCalls(t, links, tc.lengths, pid, tc.mode, tc.queries...) {
				if c.Err != nil || c.Rows != int64(tc.lengths[i]) {
					t.Fatalf("actual completed write %d: %+v", i, c)
				}
			}
		})
	}
	t.Run("source_transition_inside_statement", func(t *testing.T) {
		// Keep all 8193 rows and the original source switch at position 4096.
		// This actual subset crosses that switch inside its first 256 parameters.
		f := newSetWriteFixture(t, 8193)
		links := slices.Clone(f.desired[4095:4352])
		if len(links) != 257 || links[0].SourceID != (uuid.UUID{}) || links[0].Start != 4095 || links[1].SourceID != lintID(1) || links[1].Start != 0 || links[255].SourceID != lintID(1) || links[255].Start != 254 || links[256].SourceID != lintID(1) || links[256].Start != 255 {
			t.Fatal("fixture lost the literal mixed-source statement and final key")
		}
		pid, err := f.set(t.Context(), links)
		if err != nil {
			t.Fatal(err)
		}
		want := slices.Clone(f.before)
		copy(want[4095:4352], links)
		f.assertRows(t, want)
		for i, c := range f.assertCalls(t, links, []int{256, 1}, pid, pgx.QueryExecModeCacheDescribe) {
			if c.Err != nil || c.Rows != int64([]int{256, 1}[i]) {
				t.Fatalf("mixed-source actual completed statement %d: %+v", i, c)
			}
		}
	})
}

// Inputs outside the strict transaction/order domain retain one original SQL,
// including duplicates that straddle a prospective chunk and int32 aliases.
func TestSetResolutionsRetainsSingleStatementFallback(t *testing.T) {
	for _, kind := range []string{"descending", "interior_inversion", "cross_chunk_duplicate", "cross_new_boundary_duplicate", "int32_alias", "int32_conversion", "negative", "pool", "connection"} {
		t.Run(kind, func(t *testing.T) {
			n := 4097
			if kind == "cross_chunk_duplicate" || kind == "cross_new_boundary_duplicate" || kind == "int32_alias" {
				n = 8193
			}
			f := newSetWriteFixture(t, n)
			links := slices.Clone(f.desired)
			missing := false
			switch kind {
			case "descending":
				slices.Reverse(links)
			case "interior_inversion":
				links[1000], links[1001] = links[1001], links[1000]
			case "cross_chunk_duplicate":
				links[4096] = links[0]
				missing = true
			case "cross_new_boundary_duplicate":
				links[256] = links[0]
				missing = true
			case "int32_alias":
				if strconv.IntSize != 64 {
					t.Fatal("int32 alias fixture requires the supported 64-bit host")
				}
				links[n-1] = links[4096]
				wide := int64(1) << 32
				links[n-1].Start = int(wide)
				missing = true
			case "int32_conversion":
				if strconv.IntSize != 64 {
					t.Fatal("int32 conversion fixture requires the supported 64-bit host")
				}
				wide := int64(1) << 32
				links[n-1].Start = int(wide)
			case "negative":
				links[n-1].Start = -1
				missing = true
			}
			var pid uint32
			var err error
			switch kind {
			case "pool":
				err = f.s.SetResolutions(t.Context(), links)
			case "connection":
				conn, e := f.writer.Acquire(t.Context())
				if e != nil {
					t.Fatal(e)
				}
				pid = conn.Conn().PgConn().PID()
				ctx := postgres.OnConn(t.Context(), conn)
				if postgres.InTx(ctx) {
					conn.Release()
					t.Fatal("OnConn-only fixture unexpectedly has a Tx")
				}
				err = f.s.SetResolutions(ctx, links)
				conn.Release()
			default:
				pid, err = f.set(t.Context(), links)
			}
			if missing {
				want := fmt.Sprintf("set %d links' resolutions, of %d", n-1, n)
				if err == nil || err.Error() != want {
					t.Fatalf("original whole-statement count error=%v, want %q", err, want)
				}
				f.assertRows(t, f.before)
			} else {
				if err != nil {
					t.Fatal(err)
				}
				f.assertRows(t, f.desired)
			}
			calls := f.assertCalls(t, links, []int{n}, pid, pgx.QueryExecModeDescribeExec)
			wantRows := n
			if missing {
				wantRows--
			}
			if calls[0].Err != nil || calls[0].Rows != int64(wantRows) {
				t.Fatalf("fallback actual command=%+v", calls[0])
			}
		})
	}
}

// Count mismatch is checked only after every successful chunk. A genuine later
// CHECK violation wins over an earlier missing key and stops subsequent writes.
func TestSetResolutionsPreservesFailureAndRollsBack(t *testing.T) {
	for _, kind := range []string{"missing_first_and_tail", "later_check", "missing_then_check"} {
		t.Run(kind, func(t *testing.T) {
			f := newSetWriteFixture(t, 8193)
			if kind != "later_check" {
				lintExec(t, f.fixture, "DELETE FROM page_links WHERE source_id=$1 AND range_start=0", uuid.UUID{})
				f.before = slices.DeleteFunc(f.before, func(l app.Link) bool { return l.SourceID == (uuid.UUID{}) && l.Start == 0 })
			}
			if kind == "missing_first_and_tail" {
				lintExec(t, f.fixture, "DELETE FROM page_links WHERE source_id=$1 AND range_start=4096", lintID(1))
				f.before = slices.DeleteFunc(f.before, func(l app.Link) bool { return l.SourceID == lintID(1) && l.Start == 4096 })
			} else {
				// Preserve the original bad key at 4096: the second new statement fails
				// after all 4096 preceding keys succeeded in the same transaction.
				f.desired[4096].Resolution = domain.Resolution{Asset: true}
			}
			var callback error
			var pid uint32
			err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
				tx, _ := postgres.TxFrom(ctx)
				pid = tx.Conn().PgConn().PID()
				if err := f.s.Lock(ctx, f.eng); err != nil {
					return err
				}
				callback = f.s.SetResolutions(ctx, f.desired)
				return callback
			})
			lengths := []int{4096, 4096}
			queries := []string{"SetOrderedSourceResolutions", "SetOrderedSourceResolutions"}
			if kind == "missing_first_and_tail" {
				lengths = []int{4096, 4096, 1}
				queries = append(queries, "SetResolutions")
			}
			calls := f.assertCalls(t, f.desired, lengths, pid, pgx.QueryExecModeCacheDescribe, queries...)
			if err == nil || callback == nil || !errors.Is(err, callback) {
				t.Fatalf("callback/manager lost original error: %v / %v", callback, err)
			}
			if kind == "missing_first_and_tail" {
				if callback.Error() != "set 8191 links' resolutions, of 8193" {
					t.Fatalf("aggregate missing rows: %v", callback)
				}
				for i, c := range calls {
					want := int64(4096)
					switch i {
					case 0:
						want = 4095
					case 2:
						want = 0
					}
					if c.Err != nil || c.Rows != want {
						t.Fatalf("complete missing-key statement %d: %+v wantRows%d", i, c, want)
					}
				}
			} else {
				var pg *pgconn.PgError
				if !errors.As(callback, &pg) || pg.Code != "23514" || pg.ConstraintName != "page_links_resolved_asset_check" || !strings.HasPrefix(callback.Error(), "set 8193 links' resolutions: ") || !errors.Is(callback, calls[1].Err) {
					t.Fatalf("later real CHECK lost precedence/identity: callback=%v calls=%+v", callback, calls)
				}
				for i, c := range calls[:1] {
					want := int64(4096)
					if i == 0 && kind == "missing_then_check" {
						want--
					}
					if c.Err != nil || c.Rows != want {
						t.Fatalf("real success before later CHECK: statement%d=%+v wantRows%d", i, c, want)
					}
				}
				if calls[1].Err == nil || calls[1].Rows != 0 {
					t.Fatalf("real statement2 CHECK boundary: %+v", calls[1])
				}
			}
			f.assertRows(t, f.before)
		})
	}
	for _, tc := range []struct {
		name    string
		after   int
		lengths []int
		queries []string
	}{
		{"cancel_between_successful_statements", 0, []int{4096, 4096}, []string{"SetOrderedSourceResolutions", "SetOrderedSourceResolutions"}},
		{"cancel_at_new_boundary", 0, setWrite256Lengths(2, 0), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var f setWriteFixture
			if tc.name == "cancel_at_new_boundary" {
				// Same 8193-row request, with 33 short sources: after256 remains
				// a real successful original-SQL boundary before cancellation.
				runs := make([]int, 33)
				for i := range runs {
					runs[i] = 256
				}
				runs[32] = 1
				f = newSetWriteRuns(t, runs)
			} else {
				f = newSetWriteFixture(t, 8193)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			f.trace.after = func(_ context.Context, i int, c setWriteCall) {
				if i == tc.after && c.Err == nil {
					cancel()
				}
			}
			var callback error
			var pid uint32
			err := f.tx.WithinTx(ctx, func(ctx context.Context) error {
				tx, _ := postgres.TxFrom(ctx)
				pid = tx.Conn().PgConn().PID()
				if err := f.s.Lock(ctx, f.eng); err != nil {
					return err
				}
				callback = f.s.SetResolutions(ctx, f.desired)
				return callback
			})
			calls := f.assertCalls(t, f.desired, tc.lengths, pid, pgx.QueryExecModeCacheDescribe, tc.queries...)
			for i, c := range calls[:len(calls)-1] {
				if c.Err != nil || c.Rows != int64(tc.lengths[i]) {
					t.Fatalf("real successful prefix before cancellation: statement%d=%+v", i, c)
				}
			}
			last := calls[len(calls)-1]
			if !errors.Is(last.Err, context.Canceled) || last.Rows != 0 || !errors.Is(callback, context.Canceled) || !errors.Is(err, context.Canceled) || !strings.HasPrefix(callback.Error(), "set 8193 links' resolutions: ") {
				t.Fatalf("original cancellation lost: ctx=%v callback=%v manager=%v actual=%+v", ctx.Err(), callback, err, calls)
			}
			f.assertRows(t, f.before)
			var one int
			if e := f.writer.QueryRow(t.Context(), "SELECT 1").Scan(&one); e != nil || one != 1 {
				t.Fatalf("pool after cancellation: %d %v", one, e)
			}
		})
	}
	t.Run("cancel_after_last_statement", func(t *testing.T) {
		f := newSetWriteFixture(t, 4097)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		f.trace.after = func(_ context.Context, i int, c setWriteCall) {
			if i == 1 && c.Err == nil {
				cancel()
			}
		}
		pid, err := f.set(ctx, f.desired)
		lengths := []int{4096, 1}
		calls := f.assertCalls(t, f.desired, lengths, pid, pgx.QueryExecModeCacheDescribe, "SetOrderedSourceResolutions", "SetResolutions")
		if err != nil || !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("completed statements must keep original detached commit: ctx=%v manager=%v actual=%+v", ctx.Err(), err, calls)
		}
		for i, c := range calls {
			if c.Err != nil || c.Rows != int64(lengths[i]) {
				t.Fatalf("last-statement cancellation lost a completed write: statement%d=%+v", i, c)
			}
		}
		f.assertRows(t, f.desired)
	})
	for _, kind := range []string{"nil", "empty"} {
		t.Run("cancel_"+kind, func(t *testing.T) {
			f := newSetWriteFixture(t, 1)
			var links []app.Link
			if kind == "empty" {
				links = []app.Link{}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var callback error
			var pid uint32
			err := f.tx.WithinTx(ctx, func(ctx context.Context) error {
				tx, _ := postgres.TxFrom(ctx)
				pid = tx.Conn().PgConn().PID()
				cancel()
				callback = f.s.SetResolutions(ctx, links)
				return callback
			})
			calls := f.assertCalls(t, links, []int{0}, pid, pgx.QueryExecModeDescribeExec)
			if !errors.Is(callback, context.Canceled) || !errors.Is(err, context.Canceled) || !errors.Is(calls[0].Err, context.Canceled) {
				t.Fatalf("empty SQL/cause suppressed: %v / %v / %+v", callback, err, calls)
			}
			f.assertRows(t, f.before)
		})
	}
}
