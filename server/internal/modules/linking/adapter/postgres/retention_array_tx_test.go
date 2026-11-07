package postgresadapter_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type retentionArrayCall struct {
	name           string
	args           []any
	pid            uint32
	before, during [3]*pgtype.Type
	err            error
	done           bool
}
type retentionArrayTrace struct {
	mu         sync.Mutex
	before     [3]*pgtype.Type
	calls      []retentionArrayCall
	prepares   map[string]int
	panicCause any
	cancel     context.CancelFunc
}
type retentionArrayTraceKey struct{}
type retentionArrayTraceAt struct {
	trace *retentionArrayTrace
	index int
}

func retentionArraySQLName(sql string) string {
	header, _, _ := strings.Cut(sql, "\n")
	switch header {
	case "-- name: DeleteMissingPageLinks :exec":
		return "DeleteMissingPageLinks"
	case "-- name: WritePageLinksRetainingEqualPrefiltered :exec":
		return "WritePageLinksRetainingEqualPrefiltered"
	default:
		return ""
	}
}
func retentionArrayTypes(m *pgtype.Map) (out [3]*pgtype.Type) {
	for i, oid := range []uint32{pgtype.Int4ArrayOID, pgtype.TextArrayOID, pgtype.BoolArrayOID} {
		out[i], _ = m.TypeForOID(oid)
	}
	return
}
func (tr *retentionArrayTrace) TraceQueryStart(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	name := retentionArraySQLName(d.SQL)
	if name == "" {
		return ctx
	}
	tr.mu.Lock()
	at := len(tr.calls)
	tr.calls = append(tr.calls, retentionArrayCall{name: name, args: append([]any(nil), d.Args...), pid: c.PgConn().PID(), before: tr.before, during: retentionArrayTypes(c.TypeMap())})
	cause, cancel := tr.panicCause, tr.cancel
	tr.mu.Unlock()
	if name == "WritePageLinksRetainingEqualPrefiltered" {
		if cause != nil {
			panic(cause)
		}
		if cancel != nil {
			cancel()
		}
	}
	return context.WithValue(ctx, retentionArrayTraceKey{}, retentionArrayTraceAt{tr, at})
}
func (tr *retentionArrayTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	at, ok := ctx.Value(retentionArrayTraceKey{}).(retentionArrayTraceAt)
	if !ok || at.trace != tr {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.calls[at.index].err, tr.calls[at.index].done = d.Err, true
}
func (tr *retentionArrayTrace) TracePrepareStart(ctx context.Context, _ *pgx.Conn, d pgx.TracePrepareStartData) context.Context {
	name := retentionArraySQLName(d.SQL)
	if name != "" {
		tr.mu.Lock()
		if tr.prepares == nil {
			tr.prepares = map[string]int{}
		}
		tr.prepares[name]++
		tr.mu.Unlock()
	}
	return ctx
}
func (*retentionArrayTrace) TracePrepareEnd(context.Context, *pgx.Conn, pgx.TracePrepareEndData) {}

func retentionArrayPool(t *testing.T, f fixture, tr *retentionArrayTrace, mode pgx.QueryExecMode, statement, description int) *pgxpool.Pool {
	t.Helper()
	cfg := f.pool.Config()
	cfg.ConnConfig.DefaultQueryExecMode = mode
	cfg.ConnConfig.StatementCacheCapacity, cfg.ConnConfig.DescriptionCacheCapacity = statement, description
	cfg.ConnConfig.Tracer = tr
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
func retentionArrayFacts() domain.Facts {
	return domain.Facts{FrontmatterValid: true, Links: []domain.Link{
		{Kind: "wikilink", Target: "Straße.md", Start: 4, End: 12, Anchor: "節", Display: "a\"\\"},
		{Kind: "link", Target: "雪", Start: 20, End: 24, Property: "refs.0", Aliases: true},
	}}
}
func retentionArrayExpectedArgs(p app.Page, repeated bool) []any {
	if repeated {
		// The full input allocated owned arrays before unchanged rows were
		// removed. Empty after compaction is not a typed nil SQL NULL.
		return []any{p.ID, p.NotebookID, []int32{}, []int32{}, []string{}, []string{},
			[]bool{}, []string{}, []string{}, []string{}, []string{}, []string{}, []bool{}}
	}
	return []any{p.ID, p.NotebookID, []int32{4, 20}, []int32{12, 24}, []string{"wikilink", "link"}, []string{"", "refs.0"},
		[]bool{false, true}, []string{"Straße.md", "雪"}, []string{"節", ""}, []string{"a\"\\", ""},
		[]string{"strasse", "雪"}, []string{"strasse.md", ""}, []bool{false, true}}
}

// Calling through the real Store catches a wrapper attached to the wrong
// pool or query, extra pgx options in traced args, and lost statement caches.
func TestRetentionArrayEncodingActualTxKeepsTraceAndCaches(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct {
		name                             string
		mode                             pgx.QueryExecMode
		statement, description, prepares int
	}{
		{"statement", pgx.QueryExecModeCacheStatement, 64, 64, 1},
		{"statement_without_description", pgx.QueryExecModeCacheStatement, 64, 0, 1},
		{"description_without_statement", pgx.QueryExecModeCacheDescribe, 0, 64, 1},
		{"describe_without_caches", pgx.QueryExecModeDescribeExec, 0, 0, 2},
		{"statement_eviction", pgx.QueryExecModeCacheStatement, 1, 0, 2},
		{"description_eviction", pgx.QueryExecModeCacheDescribe, 0, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := &retentionArrayTrace{}
			actual := retentionArrayPool(t, f, tr, tc.mode, tc.statement, tc.description)
			conn, err := actual.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			tr.before = retentionArrayTypes(conn.Conn().TypeMap())
			other, err := f.pool.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer other.Release()
			otherBefore := retentionArrayTypes(other.Conn().TypeMap())
			p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
			f.replace(t, p, domain.Facts{Links: []domain.Link{{Kind: "link", Target: "old", Start: 4, End: 8}}})
			rollback := errors.New("keep the intermediate retained baseline private")
			err = postgres.NewTxManager(f.pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
				if err := f.s.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				for range 2 {
					p.Revision++
					if _, err := f.s.ReplacePage(ctx, p, retentionArrayFacts()); err != nil {
						return err
					}
					if retentionArrayTypes(conn.Conn().TypeMap()) != tr.before {
						t.Fatal("actual connection Type not restored")
					}
					if retentionArrayTypes(other.Conn().TypeMap()) != otherBefore {
						t.Fatal("constructor connection Type changed")
					}
				}
				var target, anchor, display string
				if err := postgres.DB(ctx, f.pool).QueryRow(ctx, "SELECT target, anchor, display FROM page_links WHERE source_id=$1 AND range_start=4", p.ID).Scan(&target, &anchor, &display); err != nil {
					return err
				}
				if target != "Straße.md" || anchor != "節" || display != "a\"\\" {
					t.Fatalf("stored literals=%q/%q/%q", target, anchor, display)
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatal(err)
			}
			if len(tr.calls) != 4 {
				t.Fatalf("target SQL count=%d", len(tr.calls))
			}
			for i, call := range tr.calls {
				want := []any{p.ID, []int32(nil)}
				if i%2 == 1 {
					want = retentionArrayExpectedArgs(p, i == 3)
				}
				if !call.done || call.err != nil || call.pid != conn.Conn().PgConn().PID() || !reflect.DeepEqual(call.args, want) {
					t.Fatalf("actual call %d=%+v; want args=%+v", i, call, want)
				}
				for j := range call.before {
					if call.during[j] == call.before[j] {
						t.Fatalf("target SQL %s was not scoped on the actual Tx", call.name)
					}
				}
			}
			for _, name := range []string{"DeleteMissingPageLinks", "WritePageLinksRetainingEqualPrefiltered"} {
				if tr.prepares[name] != tc.prepares {
					t.Fatalf("%s prepares=%d, want %d", name, tr.prepares[name], tc.prepares)
				}
			}
			cfg := conn.Conn().Config()
			if cfg.DefaultQueryExecMode != tc.mode || cfg.StatementCacheCapacity != tc.statement || cfg.DescriptionCacheCapacity != tc.description {
				t.Fatal("connection mode/capacity changed")
			}
		})
	}
}

// Removing defer restoration must fail for query errors, cancellation and a
// panic in the real pgx tracer. SQL errors must retain the actual pgx cause.
func TestRetentionArrayEncodingRestoresAfterFailedExec(t *testing.T) {
	f := newFixture(t)
	for _, kind := range []string{"sql_error", "canceled", "panic"} {
		t.Run(kind, func(t *testing.T) {
			tr := &retentionArrayTrace{}
			pool := retentionArrayPool(t, f, tr, pgx.QueryExecModeCacheStatement, 64, 64)
			conn, err := pool.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			m := conn.Conn().TypeMap()
			tr.before = retentionArrayTypes(m)
			p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
			f.replace(t, p, domain.Facts{Links: []domain.Link{{Kind: "link", Target: "old", Start: 4, End: 8}}})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cause := errors.New("panic from original pgx query tracer")
			if kind == "panic" {
				tr.panicCause = cause
			}
			if kind == "canceled" {
				tr.cancel = cancel
			}
			var panicked any
			var adapterErr error
			func() {
				defer func() { panicked = recover() }()
				err = postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(ctx, conn), func(ctx context.Context) error {
					if err := f.s.Lock(ctx, p.NotebookID); err != nil {
						return err
					}
					if kind == "sql_error" {
						if _, err := postgres.DB(ctx, pool).Exec(ctx, `CREATE FUNCTION retention_array_reject() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'retention-array-literal-failure' USING ERRCODE='P0001'; END$$`); err != nil {
							return err
						}
						if _, err := postgres.DB(ctx, pool).Exec(ctx, `CREATE TRIGGER retention_array_reject BEFORE INSERT OR UPDATE ON page_links FOR EACH ROW EXECUTE FUNCTION retention_array_reject()`); err != nil {
							return err
						}
					}
					p.Revision++
					_, adapterErr = f.s.ReplacePage(ctx, p, retentionArrayFacts())
					return adapterErr
				})
			}()
			if retentionArrayTypes(m) != tr.before {
				t.Fatal("failed Exec leaked scoped Type")
			}
			if kind == "panic" {
				//nolint:errorlint // The original panic value must survive adapter cleanup.
				if panicked != cause {
					t.Fatalf("panic=%v, want original %v", panicked, cause)
				}
			} else {
				if panicked != nil || err == nil || len(tr.calls) != 2 || !tr.calls[1].done || tr.calls[1].err == nil || !errors.Is(adapterErr, tr.calls[1].err) {
					t.Fatalf("cause changed: panic=%v return=%v calls=%+v", panicked, err, tr.calls)
				}
				if kind == "sql_error" {
					var pgErr *pgconn.PgError
					if !errors.As(adapterErr, &pgErr) || pgErr.Code != "P0001" || pgErr.Message != "retention-array-literal-failure" {
						t.Fatalf("SQL cause=%v", adapterErr)
					}
				}
				if kind == "canceled" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel cause=%v", err)
				}
			}
			// Cancellation may close pgx's connection. Never demand reuse of a
			// closed connection; on the other paths, prove the original map works.
			if !conn.Conn().IsClosed() {
				var n int
				if err := conn.QueryRow(t.Context(), "SELECT cardinality($1::integer[])", []int32{1, 2}).Scan(&n); err != nil || n != 2 {
					t.Fatalf("connection reuse=%d/%v", n, err)
				}
			}
		})
	}
}
