package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// linksReadCall is a statement sqlc wrote as a connection ran it: its
// name, its connection and the execution modes before its arguments.
type linksReadCall struct {
	name  string
	pid   uint32
	modes []pgx.QueryExecMode
}

// linksReadTrace records every statement of sqlc's a pool runs, by its
// "-- name:" line.
type linksReadTrace struct {
	mu    sync.Mutex
	calls []linksReadCall
}

func (tr *linksReadTrace) TraceQueryStart(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	fields := strings.Fields(strings.SplitN(d.SQL, "\n", 2)[0])
	if len(fields) != 4 || fields[0] != "--" || fields[1] != "name:" {
		return ctx
	}
	call := linksReadCall{name: fields[2], pid: c.PgConn().PID()}
	for _, arg := range d.Args {
		mode, ok := arg.(pgx.QueryExecMode)
		if !ok {
			break
		}
		call.modes = append(call.modes, mode)
	}
	tr.mu.Lock()
	tr.calls = append(tr.calls, call)
	tr.mu.Unlock()
	return ctx
}

func (*linksReadTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// take is the calls since the last take.
func (tr *linksReadTrace) take() []linksReadCall {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	out := tr.calls
	tr.calls = nil
	return out
}

// newLinksReadPool is a pool of two connections over url with the trace:
// two, so that a read the transaction should make but makes on the pool
// reads another connection's settings, rather than waiting for the one the
// transaction holds. generic sets plan_cache_mode = force_generic_plan as a
// connection's parameter (as bootstrap/grep_cost_test.go does).
func newLinksReadPool(t *testing.T, url string, trace *linksReadTrace, generic bool) *pgxpool.Pool {
	t.Helper()
	base, err := postgres.NewPool(t.Context(), config.DatabaseConfig{URL: url, MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	cfg := base.Config()
	base.Close()
	cfg.ConnConfig.Tracer = trace // Keep the platform's real AfterConnect.
	if generic {
		cfg.ConnConfig.RuntimeParams["plan_cache_mode"] = "force_generic_plan"
	}
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// A reach's links are read one index search a key below 4,000 keys and
// targets together (LinksReached), and in one pass over the rows they meet
// from there (M12/P4/S7 design 3.1, 4.1 item 3): LinksReachedWide, or,
// with plan_cache_mode force_generic_plan in the reading transaction's
// session, set there or as the connection's parameter,
// LinksReachedWideGeneric; each planned with its arguments (DescribeExec).
// The cells' sizes are the literal 3,999 and 4,000 of the design's cut
// (1.3 item 2), not the constant: a threshold moved either way turns one
// of them red. 2,000 keys and 1,999 targets with one source stay below a
// threshold of all three; exactly 4,000 with ten sources, in keys and
// targets, keys alone and targets alone, are past one of keys and targets
// alone, of keys alone, of targets alone or of sources.
func TestLinksReadWideFromItsThreshold(t *testing.T) {
	url := pgtest.NewDatabase(t)
	trace := &linksReadTrace{}
	pool := newLinksReadPool(t, url, trace, false)
	genericTrace := &linksReadTrace{}
	genericPool := newLinksReadPool(t, url, genericTrace, true)
	book, foreign := uuid.UUID{15: 200}, uuid.UUID{15: 201}
	target := func(i int) uuid.UUID { return uuid.UUID{0: 0x7a, 14: byte(i >> 8), 15: byte(i)} }
	keys := func(from, n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("k%04d", from+i)
		}
		return out
	}
	targets := func(from, n int) []uuid.UUID {
		out := make([]uuid.UUID, n)
		for i := range out {
			out[i] = target(from + i)
		}
		return out
	}
	sources := make([]uuid.UUID, 10)
	for i := range sources {
		sources[i] = uuid.UUID{0: 0x50, 15: byte(i)}
	}
	s1, s2, s3 := uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 3}
	linksCollectorCopy(t, pool, [][]any{
		{s1, int32(0), int32(1), book, "link", "K5", "k0005", nil, target(5), false, false, false},
		{s2, int32(0), int32(1), book, "link", "alt", "zz", "k0500", nil, false, false, true},
		{s3, int32(0), int32(1), book, "link", "by-id", "yy", nil, target(2500), true, false, false},
		{sources[0], int32(4), int32(5), book, "link", "own", "k0001", nil, nil, false, false, false},
		{uuid.UUID{15: 9}, int32(0), int32(1), foreign, "link", "foreign", "k0005", "k0500", target(2500), false, false, false},
	})
	a := app.Link{SourceID: s1, Start: 0, Target: "K5", Resolution: domain.Resolution{ID: target(5)}}
	b := app.Link{SourceID: s2, Start: 0, Target: "alt", Aliases: true}
	c := app.Link{SourceID: s3, Start: 0, Target: "by-id", Resolution: domain.Resolution{ID: target(2500), Ambiguous: true}}
	d := app.Link{SourceID: sources[0], Start: 4, Target: "own"}
	store, generic := New(pool), New(genericPool)
	for _, tc := range []struct {
		name  string
		reach domain.Reach
		want  []app.Link
		wide  bool
	}{
		{"3999_keys_and_targets_one_source", domain.Reach{Keys: keys(0, 2000), Targets: targets(0, 1999), Sources: sources[:1]}, []app.Link{a, b, d}, false},
		{"4000_keys_and_targets", domain.Reach{Keys: keys(0, 1000), Targets: targets(0, 3000), Sources: sources}, []app.Link{a, b, c, d}, true},
		{"4000_keys", domain.Reach{Keys: keys(0, 4000), Sources: sources}, []app.Link{a, b, d}, true},
		{"4000_targets", domain.Reach{Targets: targets(0, 4000), Sources: sources}, []app.Link{a, c, d}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if n := len(tc.reach.Keys) + len(tc.reach.Targets); n != 3999 && n != 4000 {
				t.Fatalf("a cell of %d keys and targets", n)
			}
			for _, setting := range []struct {
				name, local string
				generic     bool
			}{
				{"auto_on_the_pool", "", false},
				{"auto_set_local", "auto", false},
				{"force_custom_plan_set_local", "force_custom_plan", false},
				{"force_generic_plan_set_local", "force_generic_plan", true},
				{"force_generic_plan_connection_parameter", "", true},
			} {
				t.Run(setting.name, func(t *testing.T) {
					s, tr, p := store, trace, pool
					if setting.name == "force_generic_plan_connection_parameter" {
						s, tr, p = generic, genericTrace, genericPool
					}
					var got []app.Link
					read := func(ctx context.Context) error {
						tr.take()
						var err error
						got, err = s.Links(ctx, book, tc.reach)
						return err
					}
					var err error
					if setting.local == "" {
						err = read(t.Context())
					} else {
						err = postgres.NewTxManager(p, 10*time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
							tx, _ := postgres.TxFrom(ctx)
							if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode = "+setting.local); err != nil {
								return err
							}
							return read(ctx)
						})
					}
					calls := tr.take()
					if err != nil || !slices.Equal(got, tc.want) {
						t.Fatalf("Links=%+v err=%v; want %+v", got, err, tc.want)
					}
					var names []string
					for _, c := range calls {
						names = append(names, c.name)
					}
					want := []string{"LinksReached"}
					if tc.wide {
						want = []string{"PlanCacheMode", "LinksReachedWide"}
						if setting.generic {
							want[1] = "LinksReachedWideGeneric"
						}
					}
					if !slices.Equal(names, want) {
						t.Fatalf("read %v; want %v", names, want)
					}
					last := calls[len(calls)-1]
					if !slices.Equal(last.modes, []pgx.QueryExecMode{pgx.QueryExecModeDescribeExec}) {
						t.Fatalf("%s was read with the modes %v; want planned with its arguments (DescribeExec)", last.name, last.modes)
					}
					// In a transaction both are its connection's; on the pool
					// they may be two (design 3.1: none reads them there).
					if tc.wide && setting.local != "" && calls[0].pid != last.pid {
						t.Fatalf("the setting was read on the connection %d, the links on %d", calls[0].pid, last.pid)
					}
				})
			}
		})
	}
	t.Run("a_failed_setting_read_is_the_links_error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := store.Links(ctx, book, domain.Reach{Keys: keys(0, 4000)})
		if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "links of "+book.String()+": ") {
			t.Fatalf("Links with an ended context: %v", err)
		}
	})
}
