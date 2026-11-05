package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type setWritePrepareKey struct{}

func (tr *setWriteTrace) TracePrepareStart(ctx context.Context, conn *pgx.Conn, d pgx.TracePrepareStartData) context.Context {
	query := setWriteQuery(d.SQL)
	if query == "" {
		return ctx
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	at := len(tr.prepared)
	tr.prepared = append(tr.prepared, lintPrepareObservation{Query: query, Name: d.Name, PID: conn.PgConn().PID()})
	return context.WithValue(ctx, setWritePrepareKey{}, setWriteTraceAt{tr, at})
}

func (tr *setWriteTrace) TracePrepareEnd(ctx context.Context, _ *pgx.Conn, d pgx.TracePrepareEndData) {
	at, ok := ctx.Value(setWritePrepareKey{}).(setWriteTraceAt)
	if !ok || at.trace != tr {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.prepared[at.index].Ended, tr.prepared[at.index].Err = true, d.Err
}

func (tr *setWriteTrace) prepareObservations() []lintPrepareObservation {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.prepared)
}

// Only completed observations are cleared; the actual connection and both
// pgx caches remain untouched between the cold and hot production calls.
func (tr *setWriteTrace) reset() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.calls, tr.prepared = nil, nil
}

// The original 1537-row inputs retain all actual connection-capacity directions.
// Three additional 24577-row cases execute six new statements plus one old tail.
// Disabling descriptions, using the statement cache/capacity, or consulting
// the constructor pool instead of the caller's connection fails these cases.
func TestSetResolutionsBatchDescriptionCache(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		constructorDescription int
		statement, description int
		mode                   pgx.QueryExecMode
		prepares               [2]int
		plan                   string
		longRuns               bool
		tailPrepares           [2]int
	}{
		{"description_positive", 8, 8, 8, pgx.QueryExecModeCacheDescribe, [2]int{1, 0}, "auto", false, [2]int{}},
		{"description_zero", 0, 8, 0, pgx.QueryExecModeDescribeExec, [2]int{1, 1}, "auto", false, [2]int{}},
		{"statement_zero", 8, 0, 8, pgx.QueryExecModeCacheDescribe, [2]int{1, 0}, "auto", false, [2]int{}},
		{"constructor_zero_actual_positive", 0, 8, 8, pgx.QueryExecModeCacheDescribe, [2]int{1, 0}, "auto", false, [2]int{}},
		{"constructor_positive_actual_zero", 8, 8, 0, pgx.QueryExecModeDescribeExec, [2]int{1, 1}, "auto", false, [2]int{}},
		{"force_generic", 8, 8, 8, pgx.QueryExecModeCacheDescribe, [2]int{1, 0}, "force_generic_plan", false, [2]int{}},
		{"long_description_positive", 8, 8, 8, pgx.QueryExecModeCacheDescribe, [2]int{1, 0}, "auto", true, [2]int{1, 0}},
		{"long_description_zero", 0, 8, 0, pgx.QueryExecModeDescribeExec, [2]int{6, 6}, "auto", true, [2]int{1, 1}},
		{"long_force_generic", 8, 8, 8, pgx.QueryExecModeCacheDescribe, [2]int{1, 0}, "force_generic_plan", true, [2]int{1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var f setWriteFixture
			lengths, queries := []int{1537}, []string{"SetOrderedSourceResolutions"}
			if tc.longRuns {
				f = newSetWriteRuns(t, []int{8192, 8192, 8193})
				lengths = []int{4096, 4096, 4096, 4096, 4096, 4096, 1}
				queries = []string{"SetOrderedSourceResolutions", "SetOrderedSourceResolutions", "SetOrderedSourceResolutions", "SetOrderedSourceResolutions", "SetOrderedSourceResolutions", "SetOrderedSourceResolutions", "SetResolutions"}
			} else {
				f = newSetWriteFixture(t, 1537)
			}
			cfg := f.pool.Config()
			cfg.MaxConns = 1
			cfg.ConnConfig.StatementCacheCapacity = tc.statement
			cfg.ConnConfig.DescriptionCacheCapacity = tc.description
			cfg.ConnConfig.Tracer = f.trace
			if tc.statement == 0 {
				// LockNotebook is an ordinary generated query. A connection with
				// no statement cache needs a usable default for that setup SQL;
				// Set must still explicitly select CacheDescribe itself.
				cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
			}
			actual, err := pgxpool.NewWithConfig(t.Context(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(actual.Close)
			constructorTrace := &setWriteTrace{}
			constructor := lintBodyModePool(t, f.fixture, constructorTrace, 8, tc.constructorDescription)
			f.s, f.tx = postgresadapter.New(constructor), postgres.NewTxManager(constructor, time.Second)
			conn, err := actual.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			pid := conn.Conn().PgConn().PID()
			ctx := postgres.OnConn(t.Context(), conn)
			err = f.tx.WithinTx(ctx, func(ctx context.Context) error {
				tx, ok := postgres.TxFrom(ctx)
				if !ok || tx.Conn() != conn.Conn() {
					t.Fatal("Set fixture lost the actual caller transaction")
				}
				if err := f.s.Lock(ctx, f.eng); err != nil {
					return err
				}
				// These are test-owned session settings, never product overrides.
				setting := "SET LOCAL plan_cache_mode='auto'"
				if tc.plan == "force_generic_plan" {
					setting = "SET LOCAL plan_cache_mode='force_generic_plan'"
				}
				if _, err := postgres.Planned(tx).Exec(ctx, setting); err != nil {
					return err
				}
				for stage, wantPrepares := range tc.prepares {
					var plan string
					if err := postgres.Planned(tx).QueryRow(ctx, "SHOW plan_cache_mode").Scan(&plan); err != nil || plan != tc.plan {
						t.Fatalf("actual plan_cache_mode=%q want=%q err=%v", plan, tc.plan, err)
					}
					f.trace.reset()
					if err := f.s.SetResolutions(ctx, f.desired); err != nil {
						return err
					}
					prepared := f.trace.prepareObservations()
					setWriteAssertPrepares(t, prepared, pid, wantPrepares, tc.tailPrepares[stage])
					t.Logf("SETWRITE descriptions: stage=%d PID=%d actual=%+v", stage, pid, prepared)
					calls := f.assertCalls(t, f.desired, lengths, pid, tc.mode, queries...)
					for i, c := range calls {
						if c.Err != nil || c.Rows != int64(c.Lengths[0]) || c.StatementCapacity != tc.statement || c.DescriptionCapacity != tc.description {
							t.Fatalf("SET_WRITE_CACHE_CONNECTION: stage=%d statement=%d actual=%+v want capacities=%d/%d", stage, i, c, tc.statement, tc.description)
						}
					}
					var named int
					if err := postgres.Planned(tx).QueryRow(ctx, "SELECT count(*) FROM pg_prepared_statements WHERE statement LIKE '-- name: SetResolutions %' OR statement LIKE '-- name: SetOrderedSourceResolutions %'").Scan(&named); err != nil || named != 0 {
						t.Fatalf("SET_WRITE_NAMED: count=%d err=%v", named, err)
					}
					if stage == 0 {
						// Replace the server's unnamed statement without modifying the
						// cached description; the next real Store call must still work.
						var one int
						if err := postgres.Planned(tx).QueryRow(ctx, "SELECT $1::integer", 29).Scan(&one); err != nil || one != 29 {
							t.Fatalf("intervening unnamed query: %d %v", one, err)
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls := constructorTrace.snapshot(); len(calls) != 0 {
				t.Fatalf("Set escaped caller connection to constructor pool: %+v", calls)
			}
			f.assertRows(t, f.desired)
		})
	}
}

// A large canonical input can fail in its first statement. One observed call
// still belongs to the cached batch route, and its native cause must survive
// rollback; a later call on that same connection must describe again.
func TestSetResolutionsBatchCacheFirstFailure(t *testing.T) {
	f := newSetWriteFixture(t, 513)
	conn, err := f.writer.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	ctx := postgres.OnConn(t.Context(), conn)
	pid := conn.Conn().PgConn().PID()
	bad := slices.Clone(f.desired)
	bad[0].Resolution = domain.Resolution{Asset: true}
	var callback error
	err = f.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := f.s.Lock(ctx, f.eng); err != nil {
			return err
		}
		callback = f.s.SetResolutions(ctx, bad)
		return callback
	})
	calls := f.assertCalls(t, bad, []int{513}, pid, pgx.QueryExecModeCacheDescribe, "SetOrderedSourceResolutions")
	var pg *pgconn.PgError
	if callback == nil || !errors.Is(err, callback) || !errors.As(callback, &pg) || pg.Code != "23514" || pg.ConstraintName != "page_links_resolved_asset_check" || !strings.HasPrefix(callback.Error(), "set 513 links' resolutions: ") || calls[0].Err == nil || !errors.Is(callback, calls[0].Err) || calls[0].Rows != 0 {
		t.Fatalf("first native CHECK/cause: callback=%v manager=%v actual=%+v", callback, err, calls)
	}
	if conn.Conn().PgConn().TxStatus() != 'I' {
		t.Fatal("failed Set did not finish its rollback")
	}
	lintAssertPrepares(t, f.trace.prepareObservations(), "SetOrderedSourceResolutions", pid, 1)
	f.assertRows(t, f.before)
	f.trace.reset()
	gotPID, err := f.set(ctx, f.desired)
	if err != nil || gotPID != pid {
		t.Fatalf("same-connection recovery: PID=%d want=%d err=%v", gotPID, pid, err)
	}
	for _, c := range f.assertCalls(t, f.desired, []int{513}, pid, pgx.QueryExecModeCacheDescribe, "SetOrderedSourceResolutions") {
		if c.Err != nil || c.Rows != int64(c.Lengths[0]) {
			t.Fatalf("recovery actual completed statement=%+v", c)
		}
	}
	lintAssertPrepares(t, f.trace.prepareObservations(), "SetOrderedSourceResolutions", pid, 1)
	f.assertRows(t, f.desired)
}

// Count descriptions separately for the two exact generated SQL identities.
func setWriteAssertPrepares(t *testing.T, got []lintPrepareObservation, pid uint32, ordered, original int) {
	t.Helper()
	var counts [2]int
	for _, p := range got {
		if p.Name != "" || p.PID != pid || !p.Ended || p.Err != nil {
			t.Fatalf("SET_WRITE_UNNAMED_DESCRIPTION: %+v", p)
		}
		switch p.Query {
		case "SetOrderedSourceResolutions":
			counts[0]++
		case "SetResolutions":
			counts[1]++
		default:
			t.Fatalf("unexpected description identity: %+v", p)
		}
	}
	if counts != [2]int{ordered, original} {
		t.Fatalf("SET_WRITE_DESCRIPTIONS: actual%v want new%d old%d observations%+v", counts, ordered, original, got)
	}
}
