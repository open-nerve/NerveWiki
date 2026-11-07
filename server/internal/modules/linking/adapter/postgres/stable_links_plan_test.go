//go:build !race

package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The observer delegates to the real Store. It samples only before/after that
// method returns, never by re-entering a busy pgx QueryTracer connection.
type stablePlanStore struct {
	app.Store
	before func(context.Context)
	after  func(context.Context, app.Dropped, error)
}

func (s stablePlanStore) ReplacePage(ctx context.Context, p app.Page, facts domain.Facts) (app.Dropped, error) {
	s.before(ctx)
	d, err := s.Store.ReplacePage(ctx, p, facts)
	s.after(ctx, d, err)
	return d, err
}

type stablePlanStoreEvidence struct {
	Case, Scope, State, Mode, Shape string
	PID                             uint32
	Settings                        map[string]string
	Work                            map[string]stablePlanWork
	BeforeChoice, AfterChoice       map[string]stablePlanCount
	Rows                            map[string]int64
	WallNS                          int64
	GoAlloc                         uint64
	Dropped                         app.Dropped
	Calls                           []stablePlanCall
}

func stablePlanNewCalls(calls []stablePlanCall) []stablePlanCall {
	var out []stablePlanCall
	for _, c := range calls {
		if c.Name == stablePlanInspect || c.Name == stablePlanDelete || c.Name == stablePlanWrite {
			out = append(out, c)
		}
	}
	return out
}

func stablePlanRunStore(t *testing.T, f *stablePlanFixture, s stablePlanSpec, mode string, templates []stablePlanCall, fullSnapshot, exactArguments bool, fixed *pgxpool.Conn) []stablePlanCall {
	t.Helper()
	conn := fixed
	var err error
	if conn == nil {
		conn, err = f.pool.Acquire(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
	}
	ctx := postgres.OnConn(t.Context(), conn)
	rollback := errors.New("stable physical observation must roll back")
	e := stablePlanStoreEvidence{Case: t.Name(), Scope: "whole_real_Store_ReplacePage_with_capture_observer", State: f.state, Mode: mode, Shape: s.Shape, BeforeChoice: map[string]stablePlanCount{}, AfterChoice: map[string]stablePlanCount{}, Rows: map[string]int64{}}
	var calls []stablePlanCall
	err = f.tx.WithinTx(ctx, func(ctx context.Context) error {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			t.Fatal("stable actual Store lacks original Tx")
		}
		e.PID = tx.Conn().PgConn().PID()
		e.Settings = stablePlanSettings(t, ctx, tx, mode)
		var work map[string]stablePlanWork
		var mem runtime.MemStats
		var start time.Time
		observer := stablePlanStore{Store: f.s}
		observer.before = func(ctx context.Context) {
			for _, c := range stablePlanNewCalls(templates) {
				e.BeforeChoice[c.Name] = stablePlanCounter(t, ctx, tx, c.SQL)
			}
			work = stablePlanRelationWork(t, ctx, tx)
			runtime.ReadMemStats(&mem)
			start = time.Now()
			f.trace.begin()
		}
		observer.after = func(ctx context.Context, d app.Dropped, callErr error) {
			e.WallNS = time.Since(start).Nanoseconds()
			var end runtime.MemStats
			runtime.ReadMemStats(&end)
			e.GoAlloc = end.TotalAlloc - mem.TotalAlloc
			calls = f.trace.end()
			e.Work = stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
			lintEvidence(t, "stable-actual-replace-capture", calls)
			if callErr != nil {
				t.Fatalf("actual stable ReplacePage: %v", callErr)
			}
			stablePlanAssertCapture(t, calls, e.PID)
			for i, c := range calls {
				stablePlanAssertParameters(t, c, s)
				calls[i].PreparedName = f.trace.prepareName(c.SQL)
				e.Rows[c.Name] = c.RowsAffected
				if c.Name == stablePlanInspect || c.Name == stablePlanDelete || c.Name == stablePlanWrite {
					after := stablePlanCounter(t, ctx, tx, c.SQL)
					e.AfterChoice[c.Name] = after
					before := e.BeforeChoice[c.Name]
					if len(templates) == 0 && after.Generic+after.Custom != 1 {
						t.Fatalf("stable capture was not a first real statement selection: %+v", after)
					}
					stablePlanCountDelta(t, before, after, mode)
					if calls[i].PreparedName != after.Name {
						t.Fatalf("stable actual Prepare/statement identity differs: %q/%q", calls[i].PreparedName, after.Name)
					}
				}
			}
			if templates != nil {
				if len(calls) != len(templates) {
					t.Fatal("stable actual sequence changed since independent capture")
				}
				for i, c := range calls {
					if c.SQL != templates[i].SQL || (exactArguments && !reflect.DeepEqual(c.Args, templates[i].Args)) {
						t.Fatalf("stable actual SQL/complete arguments differ at %s", c.Name)
					}
				}
			}
			e.Calls = calls
			e.Dropped = d
			lintEvidence(t, "stable-store-after-raw", e)
			wantD := stablePlanWantD(s)
			stableAssertDroppedWitness(t, d.Targets, wantD)
			aliases := make([]string, len(s.OldFacts.Aliases))
			for i, a := range s.OldFacts.Aliases {
				aliases[i] = a.Key
			}
			gotAliases := slices.Clone(d.AliasKeys)
			slices.Sort(gotAliases)
			slices.Sort(aliases)
			if !slices.Equal(gotAliases, aliases) {
				t.Fatalf("stable complete dropped aliases=%v want=%v", gotAliases, aliases)
			}
			if e.Rows[stablePlanInspect] != int64(max(1, s.N)) {
				t.Fatalf("stable actual Inspect complete row count=%d want=%d", e.Rows[stablePlanInspect], max(1, s.N))
			}
			if e.Rows[stablePlanDelete] != int64(s.Z) || e.Rows[stablePlanWrite] != int64(s.C+s.I) {
				t.Fatalf("stable actual DML tags=%v; tags alone do not prove reads", e.Rows)
			}
			stablePlanAssertSource(t, ctx, f, s, s.Written, 2)
			if fullSnapshot {
				stablePlanAssertSnapshot(t, ctx, f, &s.ID)
			}
			e.Dropped = d
		}
		index := app.Index{Store: observer, Pages: f.inputs, Publisher: f.inputs}
		place := domain.Place{Name: s.Name}
		if err := index.PagesChanged(ctx, app.PagesChanged{WorkspaceID: stablePlanID(900003), NotebookID: f.eng, Changes: []domain.Change{{NodeID: s.ID, Before: &place, After: &place, Revision: 2, Facts: s.NewFacts}}}); err != nil {
			return err
		}
		stablePlanAssertSource(t, ctx, f, s, s.Fresh, 2)
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("stable actual Store rollback identity: %v", err)
	}
	if !slices.Equal(f.trace.terminal(), []string{"rollback"}) {
		t.Fatalf("stable first/final transaction end=%v", f.trace.terminal())
	}
	stablePlanAssertSource(t, ctx, f, s, s.Old, 1)
	if fullSnapshot {
		stablePlanAssertSnapshot(t, ctx, f, nil)
	}
	e.Calls = calls
	lintEvidence(t, "stable-store-total", e)
	return calls
}

func stablePlanReplayOther(t *testing.T, ctx context.Context, tx pgx.Tx, c stablePlanCall) {
	t.Helper()
	if c.Kind == ":exec" {
		if _, err := tx.Exec(ctx, c.SQL, c.Args...); err != nil {
			t.Fatal(err)
		}
		return
	}
	if c.Kind != ":many" && c.Kind != ":one" {
		t.Fatalf("stable uncaptured sqlc result kind %q", c.Kind)
	}
	rows, err := tx.Query(ctx, c.SQL, c.Args...)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		if _, err := rows.Values(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func stablePlanRunKernel(t *testing.T, f *stablePlanFixture, s stablePlanSpec, mode, operation string, calls []stablePlanCall, inspectOnly, history bool, fixed *pgxpool.Conn) {
	t.Helper()
	conn := fixed
	var err error
	if conn == nil {
		conn, err = f.pool.Acquire(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
	}
	ctx := postgres.OnConn(t.Context(), conn)
	rollback := errors.New("stable original SQL kernel observation must roll back")
	f.trace.begin()
	f.trace.end() // Reset end observation without capturing helper SQL.
	err = f.tx.WithinTx(ctx, func(ctx context.Context) error {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			t.Fatal("stable kernel outside Tx")
		}
		if err := f.s.Lock(ctx, f.eng); err != nil {
			return err
		}
		stablePlanSettings(t, ctx, tx, mode)
		stablePlanAssertSource(t, ctx, f, s, s.Old, 1)
		for _, c := range calls {
			if inspectOnly && c.Name != stablePlanInspect {
				break
			}
			switch c.Name {
			case stablePlanInspect, stablePlanDelete, stablePlanWrite:
				name := c.PreparedName
				if history {
					count := stablePlanCounter(t, ctx, tx, c.SQL)
					if count.Name == "" || count.Generic+count.Custom != 12 {
						t.Fatalf("stable actual history before observation=%+v; require exactly twelve completed product selections", count)
					}
					name = count.Name
				}
				e := stablePlanObserve(t, ctx, f, s, c, mode, operation, name, !history)
				if history && e.AfterChoice.Generic+e.AfterChoice.Custom != 13 {
					t.Fatalf("stable actual thirteenth choice=%+v", e.AfterChoice)
				}
				switch c.Name {
				case stablePlanInspect:
					stablePlanAssertLinks(t, ctx, f, s.ID, s.Old)
				case stablePlanDelete:
					stablePlanAssertLinks(t, ctx, f, s.ID, s.Deleted)
				case stablePlanWrite:
					stablePlanAssertLinks(t, ctx, f, s.ID, s.Written)
				}
			default:
				stablePlanReplayOther(t, ctx, tx, c)
			}
		}
		if inspectOnly {
			stablePlanAssertSource(t, ctx, f, s, s.Old, 1)
		} else {
			stablePlanAssertSource(t, ctx, f, s, s.Written, 2)
		}
		stablePlanAssertSnapshot(t, ctx, f, &s.ID)
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("stable kernel rollback identity: %v", err)
	}
	if !slices.Equal(f.trace.terminal(), []string{"rollback"}) {
		t.Fatalf("stable kernel transaction end=%v", f.trace.terminal())
	}
	stablePlanAssertSnapshot(t, ctx, f, nil)
	var one int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("stable connection recovery %d %v", one, err)
	}
}

func stablePlanMainSpecs() []stablePlanSpec {
	return []stablePlanSpec{stablePlanSpecOf("pstable", "stable", stablePlanID(1), 10000, false), stablePlanSpecOf("pmixed", "mixed", stablePlanID(2), 10000, false), stablePlanSpecOf("pchanged", "changed", stablePlanID(3), 10000, false)}
}

func stablePlanCapture(t *testing.T, specs []stablePlanSpec) map[string][]stablePlanCall {
	t.Helper()
	out := map[string][]stablePlanCall{}
	t.Run("capture", func(t *testing.T) {
		// The first product operation on each source occurs only here. These
		// databases are never reused as pristine execution/EXPLAIN fixtures.
		for _, s := range specs {
			t.Run(s.Name, func(t *testing.T) {
				f := newStablePlanFixture(t, "no_statistics", []stablePlanSpec{s}, false)
				out[s.Name] = stablePlanRunStore(t, f, f.specs[0], "auto", nil, true, true, nil)
			})
		}
	})
	if len(out) != len(specs) {
		t.Fatal("stable real capture incomplete; selected physical branches must include their capture leaves")
	}
	return out
}

func stablePlanToastPresent(t *testing.T, f *stablePlanFixture) {
	t.Helper()
	for _, table := range []string{"page_links", "page_properties"} {
		var name string
		err := f.pool.QueryRow(t.Context(), `SELECT toast.relname FROM pg_class base JOIN pg_class toast ON toast.oid=base.reltoastrelid WHERE base.oid=$1::regclass`, table).Scan(&name)
		if err != nil {
			t.Fatal(err)
		}
		var chunks int
		if err := f.pool.QueryRow(t.Context(), "SELECT count(*) FROM "+pgx.Identifier{"pg_toast", name}.Sanitize()).Scan(&chunks); err != nil || chunks == 0 {
			t.Fatalf("stable real external TOAST %s chunks=%d err=%v", table, chunks, err)
		}
		lintEvidence(t, "stable-toast-storage", map[string]any{"table": table, "actual_toast_relation": name, "chunks": chunks})
	}
}

// The main 54 leaves are 3 states × 3 modes × 3 shapes × 2 observation
// operations. execute has a separately seeded real-Store total and SQL-kernel
// fixture; explain has its own third fixture. No rolled-back seed is pristine.
func TestStableLinkRetentionPlans(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL physical observation; native cost recipe owns it")
	}
	specs := stablePlanMainSpecs()
	captured := stablePlanCapture(t, specs)
	t.Run("matrix", func(t *testing.T) {
		for _, state := range []string{"no_statistics", "analyzed", "app_rebuild_after_analyzed"} {
			t.Run(state, func(t *testing.T) {
				for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
					t.Run(mode, func(t *testing.T) {
						for _, operation := range []string{"execute", "explain"} {
							t.Run(operation, func(t *testing.T) {
								var real *stablePlanFixture
								if operation == "execute" {
									real = newStablePlanFixture(t, state, specs, true)
								}
								kernel := newStablePlanFixture(t, state, specs, true)
								for i, s := range specs {
									t.Run(s.Shape, func(t *testing.T) {
										if real != nil {
											stablePlanRunStore(t, real, real.specs[i], mode, captured[s.Name], true, true, nil)
										}
										stablePlanRunKernel(t, kernel, kernel.specs[i], mode, operation, captured[s.Name], false, false, nil)
									})
								}
							})
						}
					})
				}
			})
		}
	})
	t.Run("boundaries", func(t *testing.T) {
		endpoints := []stablePlanSpec{stablePlanSpecOf("empty_empty", "empty_to_empty", stablePlanID(10), 0, false), stablePlanSpecOf("empty_one", "empty_to_one", stablePlanID(11), 0, false), stablePlanSpecOf("one_empty", "one_to_empty", stablePlanID(12), 1, false)}
		cap := stablePlanCapture(t, endpoints)
		for _, operation := range []string{"execute", "explain"} {
			t.Run(operation, func(t *testing.T) {
				f := newStablePlanFixture(t, "no_statistics", endpoints, true)
				for i, s := range endpoints {
					t.Run(s.Shape, func(t *testing.T) {
						stablePlanRunKernel(t, f, f.specs[i], "auto", operation, cap[s.Name], false, false, nil)
					})
				}
			})
		}
	})
	t.Run("distinct_d", func(t *testing.T) {
		s := stablePlanSpecOf("distinct_d", "distinct_d", stablePlanID(30), 10000, false)
		cap := stablePlanCapture(t, []stablePlanSpec{s})
		for _, state := range []string{"no_statistics", "analyzed"} {
			t.Run(state, func(t *testing.T) {
				for _, operation := range []string{"execute", "explain"} {
					t.Run(operation, func(t *testing.T) {
						f := newStablePlanFixture(t, state, []stablePlanSpec{s}, true)
						if len(stablePlanWantD(f.specs[0])) != 10000 {
							t.Fatal("stable full D fixture lost an actual target")
						}
						stablePlanRunKernel(t, f, f.specs[0], "auto", operation, cap[s.Name], true, false, nil)
					})
				}
			})
		}
	})
	t.Run("toast", func(t *testing.T) {
		specs := []stablePlanSpec{stablePlanSpecOf("toast_stable", "stable", stablePlanID(40), 10000, true), stablePlanSpecOf("toast_mixed", "mixed", stablePlanID(41), 10000, true)}
		cap := stablePlanCapture(t, specs)
		for _, operation := range []string{"execute", "explain"} {
			t.Run(operation, func(t *testing.T) {
				f := newStablePlanFixture(t, "no_statistics", specs, true)
				stablePlanToastPresent(t, f)
				for i, s := range specs {
					t.Run(s.Shape, func(t *testing.T) {
						stablePlanRunKernel(t, f, f.specs[i], "auto", operation, cap[s.Name], false, false, nil)
					})
				}
			})
		}
	})
	t.Run("named_history", func(t *testing.T) { stablePlanHistory(t, captured[specs[0].Name]) })
}

func stablePlanHistory(t *testing.T, template []stablePlanCall) {
	t.Helper()
	var specs []stablePlanSpec
	for i := 0; i < 13; i++ {
		n := 1
		if i%2 == 1 {
			n = 10000
		}
		specs = append(specs, stablePlanSpecOf(fmt.Sprintf("h%04d", i), "stable", stablePlanID(uint64(200+i)), n, false))
	}
	for _, state := range []string{"no_statistics", "analyzed", "app_rebuild_after_analyzed"} {
		t.Run(state, func(t *testing.T) {
			var observed []stablePlanCall
			for _, leg := range []string{"actual_store", "execute", "explain"} {
				t.Run(leg, func(t *testing.T) {
					f := newStablePlanFixture(t, state, specs, true)
					// MaxConns=1, and each transaction checks the actual PID. Retain
					// the session explicitly so no pool handoff can fabricate history.
					conn, err := f.pool.Acquire(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					pid := conn.Conn().PgConn().PID()
					defer conn.Release()
					var last map[string]stablePlanCount
					for i := 0; i < 12; i++ {
						// Only SQL identity is reused here: source/array parameters are
						// captured anew from this real public Index/Store call.
						calls := stablePlanRunHistoryStore(t, f, f.specs[i], template, pid, conn)
						last = stablePlanHistoryCounts(t, f, calls, pid, i+1, conn)
						lintEvidence(t, "stable-actual-named-history", map[string]any{"state": state, "leg": leg, "ordinal": i + 1, "size": f.specs[i].M, "pid": pid, "counts": last})
					}
					stablePlanAssertSnapshot(t, postgres.OnConn(t.Context(), conn), f, nil)
					if leg == "actual_store" {
						observed = stablePlanRunHistoryStore(t, f, f.specs[12], template, pid, conn)
						stablePlanHistoryCounts(t, f, observed, pid, 13, conn)
						stablePlanAssertSnapshot(t, postgres.OnConn(t.Context(), conn), f, nil)
						return
					}
					if len(observed) == 0 {
						t.Fatal("stable history lacks independently captured thirteenth actual input")
					}
					stablePlanRunKernel(t, f, f.specs[12], "auto", leg, observed, false, true, conn)
				})
			}
		})
	}
}

// Replace the template's parameters only by an independent real call, never
// by guessing generated ordinal fields. runStore's exact argument comparison
// is intentionally disabled; the actual counts below bind this SQL history.
func stablePlanRunHistoryStore(t *testing.T, f *stablePlanFixture, s stablePlanSpec, template []stablePlanCall, pid uint32, conn *pgxpool.Conn) []stablePlanCall {
	t.Helper()
	// Seeded histories use the same three SQL texts; before counts must be
	// taken using them even though this page's parameters differ.
	calls := stablePlanRunStore(t, f, s, "auto", template, false, false, conn)
	for _, c := range calls {
		if c.PID != pid {
			t.Fatalf("stable history Store changed actual PID: %d/%d", c.PID, pid)
		}
	}
	return calls
}

func stablePlanHistoryCounts(t *testing.T, f *stablePlanFixture, calls []stablePlanCall, pid uint32, want int, conn *pgxpool.Conn) map[string]stablePlanCount {
	t.Helper()
	if conn.Conn().PgConn().PID() != pid {
		t.Fatal("stable named history changed actual PID")
	}
	out := map[string]stablePlanCount{}
	err := f.tx.WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
		tx, _ := postgres.TxFrom(ctx)
		for _, c := range stablePlanNewCalls(calls) {
			count := stablePlanCounter(t, ctx, tx, c.SQL)
			if count.Name != c.PreparedName || count.Generic+count.Custom != int64(want) {
				t.Fatalf("stable actual completed selections ordinal=%d count=%+v prepared=%s", want, count, c.PreparedName)
			}
			out[c.Name] = count
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
