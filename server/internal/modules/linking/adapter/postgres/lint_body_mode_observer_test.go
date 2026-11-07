package postgresadapter_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// These are actual Query/Prepare observations, not wire packet observations.
// Existing calls/modes remain available to the unchanged rule13/presence7 guards.
type lintModeObservation struct {
	Name, SQL                              string
	Mode                                   pgx.QueryExecMode
	Modes                                  int
	Leading                                bool
	PID                                    uint32
	StatementCapacity, DescriptionCapacity int
	Args                                   []any
	Ended                                  bool
	Err                                    error
}

type lintPrepareObservation struct {
	Query, Name string
	PID         uint32
	Ended       bool
	Err         error
}

type lintModeQueryKey struct{}
type lintModePrepareKey struct{}

func lintObservedMode(conn *pgx.Conn, data pgx.TraceQueryStartData) lintModeObservation {
	cfg := conn.Config()
	o := lintModeObservation{Name: strings.Fields(data.SQL)[2], SQL: data.SQL, PID: conn.PgConn().PID(),
		StatementCapacity: cfg.StatementCacheCapacity, DescriptionCapacity: cfg.DescriptionCacheCapacity,
		Args: append([]any(nil), data.Args...)}
	for i, arg := range data.Args {
		if mode, ok := arg.(pgx.QueryExecMode); ok {
			o.Modes++
			if i == 0 {
				o.Leading = true
				o.Mode = mode
			}
		}
	}
	return o
}

func (tr *lintTrace) observations() []lintModeObservation {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.observed)
}

func (tr *lintTrace) prepareObservations() []lintPrepareObservation {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.prepared)
}

func (tr *lintTrace) TracePrepareStart(ctx context.Context, conn *pgx.Conn, data pgx.TracePrepareStartData) context.Context {
	if !strings.HasPrefix(data.SQL, "-- name: Lint") {
		return ctx
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	i := len(tr.prepared)
	tr.prepared = append(tr.prepared, lintPrepareObservation{Query: strings.Fields(data.SQL)[2], Name: data.Name, PID: conn.PgConn().PID()})
	return context.WithValue(ctx, lintModePrepareKey{}, i)
}

func (tr *lintTrace) TracePrepareEnd(ctx context.Context, _ *pgx.Conn, data pgx.TracePrepareEndData) {
	i, ok := ctx.Value(lintModePrepareKey{}).(int)
	if !ok {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.prepared[i].Ended, tr.prepared[i].Err = true, data.Err
}

func lintAssertMode(t *testing.T, got lintModeObservation, name string, mode pgx.QueryExecMode, pid uint32, statement, description int) {
	t.Helper()
	if got.Name != name || got.Modes != 1 || !got.Leading || got.Mode != mode || !got.Ended ||
		got.PID != pid || got.StatementCapacity != statement || got.DescriptionCapacity != description {
		t.Fatalf("exact query mode: got %+v; want name=%s mode=%v one leading option PID=%d capacities=%d/%d", got, name, mode, pid, statement, description)
	}
}

func lintAssertPrepares(t *testing.T, got []lintPrepareObservation, name string, pid uint32, count int) {
	t.Helper()
	if len(got) != count {
		t.Fatalf("actual unnamed description count for %s: got %+v, want %d", name, got, count)
	}
	for _, p := range got {
		if p.Query != name || p.Name != "" || p.PID != pid || !p.Ended || p.Err != nil {
			t.Fatalf("actual unnamed description: %+v", p)
		}
	}
}

func lintBodyModePool(t *testing.T, f fixture, tracer pgx.QueryTracer, statement, description int) *pgxpool.Pool {
	t.Helper()
	cfg := f.pool.Config()
	cfg.MaxConns = 1
	cfg.ConnConfig.StatementCacheCapacity = statement
	cfg.ConnConfig.DescriptionCacheCapacity = description
	cfg.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Diagnostics keep working with both pgx caches disabled. They do not create
// Lint observations and do not change DefaultQueryExecMode or the tested route.
func lintNoNamedStatements(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var count int
	err := postgres.Planned(postgres.DB(ctx, pool)).QueryRow(ctx,
		`SELECT count(*) FROM pg_prepared_statements WHERE statement LIKE '-- name: Lint%'`).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("named Lint statements: count=%d err=%v", count, err)
	}
}

func lintBodyTxManager(pool *pgxpool.Pool) *postgres.TxManager {
	return postgres.NewTxManager(pool, time.Second)
}
