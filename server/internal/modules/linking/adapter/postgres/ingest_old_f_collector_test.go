package postgresadapter

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Observe the actual manager's Tx and native Rows. No fabricated EOF/Err,
// substitute database, production hook or exported collector is needed.
type ingestOldBodyCollectorWatch struct {
	trace  *linksCollectorTrace
	rows   []*linksCollectorRows
	mode   string
	cancel context.CancelFunc
}
type ingestOldBodyCollectorContext struct {
	context.Context
	watch *ingestOldBodyCollectorWatch
}

func (c ingestOldBodyCollectorContext) Value(key any) any {
	v := c.Context.Value(key)
	if tx, ok := v.(pgx.Tx); ok {
		return ingestOldBodyCollectorTx{Tx: tx, watch: c.watch}
	}
	return v
}

type ingestOldBodyCollectorTx struct {
	pgx.Tx
	watch *ingestOldBodyCollectorWatch
}

func (tx ingestOldBodyCollectorTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if sql != gen.IngestOldBodyReferencesFirst && sql != gen.IngestOldBodyReferencesAfter {
		return tx.Tx.Query(ctx, sql, args...)
	}
	tx.watch.trace.sql = sql
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if rows == nil {
		return rows, err
	}
	observed := &linksCollectorRows{Rows: rows, stable: true, columns: 4, mode: tx.watch.mode, cancel: tx.watch.cancel}
	tx.watch.rows = append(tx.watch.rows, observed)
	return observed, err
}

func TestIngestOldBodyCollectorActualAdapter(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	book, p1, p2, target := uuid.UUID{15: 50}, uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 11}
	for _, id := range []uuid.UUID{p1, p2} {
		if err := New(pool).AddPage(t.Context(), app.Page{ID: id, NotebookID: book, Revision: 1}, domain.Facts{FrontmatterValid: true}); err != nil {
			t.Fatal(err)
		}
	}
	want := make([]app.BodyReference, 16385)
	data := make([][]any, len(want))
	for i := range want {
		source := p1
		if i >= 10000 {
			source = p2
		}
		start := int32(i % 10000)
		want[i] = app.BodyReference{EdgePos: app.EdgePos{SourceID: source, Start: start}, ResolvedID: target, ResolvedAsset: i%2 == 0}
		data[i] = []any{source, start, start + 1, book, "link", "body", "body", nil, target, false, i%2 == 0, false}
	}
	slices.Reverse(data)
	linksCollectorCopy(t, pool, data)
	if _, err := pool.Exec(t.Context(), "CREATE TABLE old_f_collector_marker(n integer NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	restore := linksCollectorObserveUUID(t, conn.Conn(), trace)
	defer restore()
	stop := errors.New("old-F collector rollback")
	watch := &ingestOldBodyCollectorWatch{trace: trace}
	err = postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
		if err := New(pool).Lock(ctx, book); err != nil {
			return err
		}
		reader := NewIngestReads(pool, 5<<20)
		heads, err := reader.IndexedHeads(ctx, book, []app.IngestRevision{{ID: p1, Revision: 1}, {ID: p2, Revision: 1}})
		if err != nil || len(heads) != 2 {
			t.Fatalf("current source heads: %+v %v", heads, err)
		}
		for i, h := range heads {
			if h.ID != []uuid.UUID{p1, p2}[i] || !h.Present || h.PageRevision != 1 || h.IndexedRevision != 1 || h.Extractor != 2 {
				t.Fatalf("head proof: %+v", h)
			}
		}
		tx, _ := postgres.TxFrom(ctx)
		if _, err := tx.Exec(ctx, "INSERT INTO old_f_collector_marker VALUES(1)"); err != nil {
			return err
		}
		var after *app.EdgePos
		offset := 0
		for callIndex, n := range []int{16384, 1, 0} {
			got, err := reader.OldBodyReferences(ingestOldBodyCollectorContext{Context: ctx, watch: watch}, book, []uuid.UUID{p2, p1, p1}, after, 16384)
			if err != nil || !slices.Equal(got, want[offset:offset+n]) || len(watch.rows) != callIndex+1 {
				t.Fatalf("actual batch%d len%d want%d err%v", callIndex, len(got), n, err)
			}
			watch.rows[callIndex].check(t, n, 1, n, 1)
			trace.mu.Lock()
			c := trace.calls[len(trace.calls)-1]
			active := trace.active
			trace.mu.Unlock()
			mode := pgx.QueryExecModeCacheDescribe
			args := []any{mode, book, p1, p2, int32(16384), []uuid.UUID{p2, p1, p1}}
			if after != nil {
				args = []any{mode, book, p1, p2, after.SourceID, after.Start, int32(16384), []uuid.UUID{p2, p1, p1}}
			}
			if c.pid != conn.Conn().PgConn().PID() || c.ends != 1 || c.err != nil || active != nil || !reflect.DeepEqual(c.args, args) || c.sourceScans != n || len(c.sourceDest) != min(n, 1) {
				t.Fatalf("actual SQL/UUID wiring: %+v active%t", c, active != nil)
			}
			if n > 0 && len(c.resolvedDest) == 0 {
				t.Fatal("actual nullable UUID wrapper was not reached")
			}
			for dst := range c.sourceDest {
				if c.resolvedDest[dst] {
					t.Fatal("source and target share a Scan destination")
				}
			}
			offset += n
			if n > 0 {
				pos := want[offset-1].EdgePos
				after = &pos
			}
			if tx.Conn().PgConn().IsBusy() {
				t.Fatal("actual adapter returned before Close")
			}
		}
		if trace.count() != 3 || offset != 16385 {
			t.Fatal("missing full raw stream/empty EOF")
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
	var markers int
	if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM old_f_collector_marker").Scan(&markers); err != nil || markers != 0 {
		t.Fatalf("rollback marker=%d %v", markers, err)
	}
	linksCollectorReusable(t, conn)
	for _, mode := range []string{"scan_513", "err_after_512"} {
		t.Run("actual_"+mode, func(t *testing.T) {
			fault := &ingestOldBodyCollectorWatch{trace: trace, mode: mode}
			var returned error
			var got []app.BodyReference
			err := postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
				if err := New(pool).Lock(ctx, book); err != nil {
					return err
				}
				tx, _ := postgres.TxFrom(ctx)
				if _, err := tx.Exec(ctx, "INSERT INTO old_f_collector_marker VALUES(1)"); err != nil {
					return err
				}
				queryCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				fault.cancel = cancel
				got, returned = NewIngestReads(pool, 5<<20).OldBodyReferences(ingestOldBodyCollectorContext{Context: queryCtx, watch: fault}, book, []uuid.UUID{p1, p2}, nil, 16384)
				if queryCtx.Err() != context.Canceled {
					t.Fatal("native Scan/Err did not precede the injected cancellation")
				}
				return returned
			})
			if len(fault.rows) != 1 {
				t.Fatalf("actual failing query inventory %d", len(fault.rows))
			}
			r := fault.rows[0]
			if mode == "scan_513" {
				r.check(t, 513, 0, 513, 0)
			} else {
				r.check(t, 512, 1, 512, 1)
			}
			//nolint:errorlint // The adapter must return the exact native error object.
			if got != nil || r.cause == nil || returned != r.cause || !errors.Is(err, r.cause) || errors.Is(returned, context.Canceled) {
				t.Fatalf("actual adapter native cause: returned%v tx%v cause%v", returned, err, r.cause)
			}
			if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM old_f_collector_marker").Scan(&markers); err != nil || markers != 0 {
				t.Fatalf("actual fault rollback marker%d %v", markers, err)
			}
			linksCollectorReusable(t, conn)
		})
	}
}

// The production SQL excludes NULL targets. Real native rows here isolate the
// collector's defensive NULL logic and preserve its original error priority.
func TestIngestOldBodyCollectorNativeErrorPriority(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	if _, err := pool.Exec(t.Context(), "CREATE TABLE old_f_collector_marker(n integer NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, mode string
		n, take    int
		nullFirst  bool
	}{
		{"literal_zero", "", 3, 3, false},
		{"null_then_eof", "", 513, 513, true},
		{"null_then_scan", "scan_513", 513, 513, true},
		{"null_then_rows_error", "err_after_512", 513, 513, true},
		{"oversize_then_eof", "", 3, 2, false},
		{"null_and_oversize_then_scan", "scan_513", 513, 1, true},
		{"native_eof_then_cancel", "cancel_after_eof", 3, 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := pool.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			rows := &linksCollectorRows{stable: true, columns: 4, mode: tc.mode}
			var got []app.BodyReference
			var returned error
			var queryCtx context.Context
			stop := errors.New("healthy native collector rollback")
			err = postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
				tx, _ := postgres.TxFrom(ctx)
				if _, err := tx.Exec(ctx, "INSERT INTO old_f_collector_marker VALUES(1)"); err != nil {
					return err
				}
				var cancel context.CancelFunc
				queryCtx, cancel = context.WithCancel(ctx)
				defer cancel()
				rows.cancel = cancel
				native, err := tx.Query(queryCtx, `SELECT $1::uuid AS source_id,k::integer AS range_start,
 CASE WHEN $4::boolean AND k=0 THEN NULL WHEN k%2=0 THEN '00000000-0000-0000-0000-000000000000'::uuid ELSE $2::uuid END AS resolved_id,
 (k%2=0) AS resolved_asset FROM generate_series(0,$3::integer-1) k ORDER BY k`, pgx.QueryExecModeDescribeExec, uuid.UUID{15: 1}, uuid.UUID{15: 11}, int32(tc.n), tc.nullFirst)
				if err != nil {
					return err
				}
				rows.Rows = native
				got, returned = collectIngestOldBodyReferences(rows, tc.take)
				if tc.mode != "" && queryCtx.Err() != context.Canceled {
					t.Fatal("native boundary did not cancel the actual read context")
				}
				if returned != nil {
					return returned
				}
				return stop
			})
			switch tc.mode {
			case "scan_513":
				rows.check(t, 513, 0, 513, 0)
			case "err_after_512":
				rows.check(t, 512, 1, 512, 1)
			default:
				rows.check(t, tc.n, 1, tc.n, 1)
			}
			if rows.cause != nil {
				//nolint:errorlint // The collector must return the exact native error object.
				if got != nil || returned != rows.cause || !errors.Is(err, rows.cause) || errors.Is(returned, context.Canceled) || queryCtx.Err() != context.Canceled {
					t.Fatalf("native cause masked: got%v returned%v tx%v native%v ctx%v", got, returned, err, rows.cause, queryCtx.Err())
				}
			} else if tc.nullFirst || tc.n > tc.take {
				message := "ingest old bodies: query exceeds requested batch"
				if tc.nullFirst {
					message = "lint reads: body reference has no target ID"
				}
				if got != nil || returned == nil || returned.Error() != message || !errors.Is(err, returned) {
					t.Fatalf("logical error before/after full native EOF: got%v returned%v tx%v", got, returned, err)
				}
			} else {
				want := []app.BodyReference{
					{EdgePos: app.EdgePos{SourceID: uuid.UUID{15: 1}, Start: 0}, ResolvedAsset: true},
					{EdgePos: app.EdgePos{SourceID: uuid.UUID{15: 1}, Start: 1}, ResolvedID: uuid.UUID{15: 11}},
					{EdgePos: app.EdgePos{SourceID: uuid.UUID{15: 1}, Start: 2}, ResolvedAsset: true},
				}
				if returned != nil || !errors.Is(err, stop) || !slices.Equal(got, want) {
					t.Fatalf("literal/native EOF result=%+v returned%v tx%v", got, returned, err)
				}
			}
			var markers int
			if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM old_f_collector_marker").Scan(&markers); err != nil || markers != 0 {
				t.Fatalf("native failure rollback marker=%d %v", markers, err)
			}
			linksCollectorReusable(t, conn)
		})
	}
}

var _ pgx.Tx = ingestOldBodyCollectorTx{}
var _ context.Context = ingestOldBodyCollectorContext{}
