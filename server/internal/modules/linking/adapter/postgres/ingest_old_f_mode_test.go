package postgresadapter_test

import (
	"context"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func TestIngestOldBodyQueryModes(t *testing.T) {
	f, p, want := lintBodyModeSeed(t)
	for _, tc := range []struct {
		name, route                               string
		statement, description, readerDescription int
		mode                                      pgx.QueryExecMode
	}{
		{"tx-positive", "tx", 8, 8, 8, pgx.QueryExecModeCacheDescribe},
		{"tx-description-zero", "tx", 8, 0, 0, pgx.QueryExecModeDescribeExec},
		{"tx-statement-zero", "tx", 0, 8, 8, pgx.QueryExecModeCacheDescribe},
		{"tx-both-zero", "tx", 0, 0, 0, pgx.QueryExecModeDescribeExec},
		{"tx-reader-zero-actual-positive", "tx", 8, 8, 0, pgx.QueryExecModeCacheDescribe},
		{"tx-reader-positive-actual-zero", "tx", 8, 0, 8, pgx.QueryExecModeDescribeExec},
		{"pool-positive-rejected", "pool", 8, 8, 8, 0},
		{"pool-zero-rejected", "pool", 0, 0, 0, 0},
		{"onconn-positive-rejected", "onconn", 8, 8, 8, 0},
		{"onconn-zero-rejected", "onconn", 0, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := &ingestOldBodyTrace{}
			actual := lintBodyModePool(t, f, tr, tc.statement, tc.description)
			readerPool := actual
			readerTrace := tr
			if tc.readerDescription != tc.description {
				readerTrace = &ingestOldBodyTrace{}
				readerPool = lintBodyModePool(t, f, readerTrace, 8, tc.readerDescription)
			}
			conn, err := actual.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			pid := conn.Conn().PgConn().PID()
			r := postgresadapter.NewIngestReads(readerPool, 5<<20)
			bound := postgres.OnConn(t.Context(), conn)
			if tc.route != "tx" {
				ctx := bound
				if tc.route == "pool" {
					conn.Release()
					ctx = t.Context()
				}
				for _, ids := range [][]uuid.UUID{{p}, nil} {
					for _, after := range []*app.EdgePos{nil, &want[0].EdgePos} {
						rows, err := r.OldBodyReferences(ctx, f.eng, ids, after, 512)
						if err == nil || err.Error() != "ingest old bodies: outside a transaction" || rows != nil {
							t.Fatalf("non-Tx old-F accepted: rows=%v err=%v", rows, err)
						}
					}
				}
				if len(tr.observations()) != 0 || len(tr.prepareObservations()) != 0 {
					t.Fatalf("rejected old-F emitted SQL: %+v / %+v", tr.observations(), tr.prepareObservations())
				}
				return
			}
			// An actual OnConn transaction may come from a different pool than
			// the constructor; the executor's capacities must choose the mode.
			err = lintBodyTxManager(readerPool).WithinTx(bound, func(ctx context.Context) error {
				tx, ok := postgres.TxFrom(ctx)
				if !ok || tx.Conn().PgConn().PID() != pid {
					t.Fatal("wrong actual Tx")
				}
				ingestOldBodyFixtureHeads(t, ctx, f, r, []uuid.UUID{p})
				for range 2 {
					first, err := r.OldBodyReferences(ctx, f.eng, []uuid.UUID{p}, nil, 1)
					if err != nil || !slices.Equal(first, want[:1]) {
						t.Fatalf("First literal DTO: %+v %v", first, err)
					}
					after, err := r.OldBodyReferences(ctx, f.eng, []uuid.UUID{p}, &want[0].EdgePos, 512)
					if err != nil || !slices.Equal(after, want[1:]) {
						t.Fatalf("nonempty After literal DTO: %+v %v", after, err)
					}
				}
				got := tr.observations()
				if len(got) != 4 {
					t.Fatalf("private old-F calls=%d want4", len(got))
				}
				for i, call := range got {
					name := ingestOldBodyFirst
					if i%2 != 0 {
						name = ingestOldBodyAfter
					}
					lintAssertMode(t, call, name, tc.mode, pid, tc.statement, tc.description)
					// Bound to native sqlc v1.31.1 ingest_old_f.sql.go, not
					// inferred from the old Body statement's parameter order.
					args := []any{tc.mode, f.eng, p, p, int32(1), []uuid.UUID{p}}
					if i%2 != 0 {
						args = []any{tc.mode, f.eng, p, p, p, int32(10), int32(512), []uuid.UUID{p}}
					}
					if call.Err != nil || !reflect.DeepEqual(call.Args, args) {
						t.Fatalf("actual old-F args/result: %+v %v want%+v", call.Args, call.Err, args)
					}
				}
				prepares := tr.prepareObservations()
				expectedPrepares := 2
				if tc.description == 0 {
					expectedPrepares = 4
				}
				if len(prepares) != expectedPrepares {
					t.Fatalf("actual descriptions=%+v want%d", prepares, expectedPrepares)
				}
				for i, prepare := range prepares {
					name := ingestOldBodyFirst
					if i%2 != 0 {
						name = ingestOldBodyAfter
					}
					lintAssertPrepares(t, []lintPrepareObservation{prepare}, name, pid, 1)
				}
				if readerTrace != tr && len(readerTrace.observations()) != 0 {
					t.Fatal("constructor pool executed old-F instead of ctx Tx")
				}
				// After mode/cache assertions, describe each actual SQL directly.
				// This is server type evidence, not wire-format observation and
				// not one of the measured driver Prepare callbacks above.
				for i, call := range got[:2] {
					desc, err := tx.Conn().PgConn().Prepare(ctx, "", call.SQL, nil)
					if err != nil {
						t.Fatal(err)
					}
					wantOIDs := []uint32{2950, 2950, 2950, 23, 2951}
					if i == 1 {
						wantOIDs = []uint32{2950, 2950, 2950, 2950, 23, 23, 2951}
					}
					if !slices.Equal(desc.ParamOIDs, wantOIDs) {
						t.Fatalf("actual old-F server parameter OIDs: %v want%v", desc.ParamOIDs, wantOIDs)
					}
					var fields []uint32
					for _, field := range desc.Fields {
						fields = append(fields, field.DataTypeOID)
					}
					if !slices.Equal(fields, []uint32{2950, 23, 2950, 16}) {
						t.Fatalf("actual old-F server result OIDs: %v", fields)
					}
					t.Logf("old-F actual server description %s params=%v results=%v; wire formats not observed", call.Name, desc.ParamOIDs, fields)
				}
				ingestOldBodyNoNamed(t, ctx, readerPool)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIngestOldBodyDescriptionReuse(t *testing.T) {
	f, p, want := lintBodyModeSeed(t)
	for _, capacity := range []int{8, 1} {
		t.Run("capacity="+strconv.Itoa(capacity), func(t *testing.T) {
			tr := &ingestOldBodyTrace{}
			pool := lintBodyModePool(t, f, tr, 0, capacity)
			conn, err := pool.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			pid := conn.Conn().PgConn().PID()
			r := postgresadapter.NewIngestReads(pool, 5<<20)
			err = lintBodyTxManager(pool).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
				ingestOldBodyFixtureHeads(t, ctx, f, r, []uuid.UUID{p})
				invoke := func(after *app.EdgePos, expected []app.BodyReference, descriptions int) {
					t.Helper()
					name := ingestOldBodyFirst
					if after != nil {
						name = ingestOldBodyAfter
					}
					before, prepared := len(tr.observations()), len(tr.prepareObservations())
					rows, err := r.OldBodyReferences(ctx, f.eng, []uuid.UUID{p}, after, 512)
					if err != nil || !slices.Equal(rows, expected) {
						t.Fatalf("cold/hot old-F literal DTO: %+v %v", rows, err)
					}
					calls := tr.observations()
					if len(calls) != before+1 {
						t.Fatalf("old-F retried or skipped: %+v", calls[before:])
					}
					lintAssertMode(t, calls[before], name, pgx.QueryExecModeCacheDescribe, pid, 0, capacity)
					lintAssertPrepares(t, tr.prepareObservations()[prepared:], name, pid, descriptions)
				}
				invoke(nil, want, 1)
				invoke(nil, want, 0)
				// The unrelated legacy query uses DescribeExec, not this cache.
				before := len(tr.observations())
				heads, err := postgresadapter.NewLintReads(pool, 5<<20).Heads(ctx, f.eng)
				if err != nil || len(heads) != 1 {
					t.Fatalf("unrelated Heads: %+v %v", heads, err)
				}
				if len(tr.observations()) != before {
					t.Fatal("new cohort included legacy SQL")
				}
				invoke(nil, want, 0)
				invoke(&want[0].EdgePos, want[1:], 1)
				invoke(&want[0].EdgePos, want[1:], 0)
				evicted := 0
				if capacity == 1 {
					evicted = 1
				}
				invoke(nil, want, evicted)
				invoke(&want[0].EdgePos, want[1:], evicted)
				ingestOldBodyNoNamed(t, ctx, pool)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
