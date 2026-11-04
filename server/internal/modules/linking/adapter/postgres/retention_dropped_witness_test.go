package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Use the same real Store, native Rows observer and five-table rollback image
// as the complete collector tests. Only the old event targets differ.
func retentionWitnessSeed(t *testing.T, pool *pgxpool.Pool, n, repeat int) (app.Page, domain.Facts, []app.Link) {
	t.Helper()
	p, facts, want := retentionCollectorSeed(t, pool, n)
	for i := range want {
		want[i].Resolution.ID = uuid.UUID{15: byte((i/repeat)%22 + 1)}
	}
	if err := New(pool).SetResolutions(t.Context(), want); err != nil {
		t.Fatal(err)
	}
	return p, facts, want
}

func TestRetentionDroppedTargetsWitness(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	trace.sql = gen.InspectPageLinksForRetention
	for _, tc := range []struct {
		name                  string
		rows, repeat, targets int
		missing               bool
	}{
		{"twenty", 20, 1, 20, false}, {"twenty_one", 21, 1, 21, false}, {"twenty_two", 22, 1, 21, false}, {"duplicates", 44, 2, 21, false}, {"tail_missing", 23, 1, 21, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, facts, want := retentionWitnessSeed(t, pool, tc.rows, tc.repeat)
			var missing []int32
			if tc.missing {
				missing = []int32{int32(2 * (tc.rows - 1))}
				facts.Links = facts.Links[:tc.rows-1]
				want = want[:tc.rows-1]
			}
			// The final changed fact is beyond the witness boundary in the
			// 22-target cases. It must still reset and write its complete row.
			last := len(facts.Links) - 1
			facts.Links[last].Target = "Tail"
			want[last].Target, want[last].Resolution = "Tail", domain.Resolution{}
			original := slices.Clone(facts.Links)
			before := retentionCollectorImage(t, pool)
			conn, err := pool.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			rows := &linksCollectorRows{stable: true, columns: 13}
			w := &retentionCollectorWatch{rows: rows, pid: conn.Conn().PgConn().PID()}
			rollback := errors.New("witness rollback")
			err = postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
				store := New(pool)
				if err := store.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				p.Revision = 2
				dropped, err := store.ReplacePage(retentionCollectorContext{Context: ctx, watch: w}, p, facts)
				if err != nil {
					return err
				}
				ids := make([]uuid.UUID, tc.targets)
				for i := range ids {
					ids[i] = uuid.UUID{15: byte(i + 1)}
				}
				if !reflect.DeepEqual(dropped, app.Dropped{Targets: ids}) || cap(dropped.Targets) != tc.targets {
					t.Fatalf("DROPPED_WITNESS_BOUND: got=%+v capacity=%d want exactly%d distinct old event targets", dropped, cap(dropped.Targets), tc.targets)
				}
				rows.check(t, tc.rows, 1, tc.rows, 1)
				retentionCollectorCheckCalls(t, w, p, retentionCollectorStages())
				if !reflect.DeepEqual(w.deleteArgs, []any{p.ID, missing}) || len(w.writeArgs) != 13 || !reflect.DeepEqual(w.writeArgs[2], []int32{int32(2 * last)}) {
					t.Fatalf("DROPPED_WITNESS_TAIL: actual missing/changed arrays=%#v/%#v", w.deleteArgs, w.writeArgs)
				}
				got, err := store.Links(ctx, p.NotebookID, domain.Reach{Sources: []uuid.UUID{p.ID}})
				if err != nil || !slices.Equal(got, want) || !reflect.DeepEqual(facts.Links, original) {
					t.Fatalf("DROPPED_WITNESS_FACTS: complete rows=%+v want=%+v err=%v", got, want, err)
				}
				return rollback
			})
			if !errors.Is(err, rollback) || !slices.Equal(retentionCollectorImage(t, pool), before) {
				t.Fatalf("witness original rollback/image: %v", err)
			}
			retentionCollectorCheckTrace(t, trace, w.pid, p, nil)
			linksCollectorReusable(t, conn)
		})
	}
}

func TestRetentionDroppedTargetsWitnessTail(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	trace.sql = gen.InspectPageLinksForRetention
	for _, mode := range []string{"late_zero", "late_notebook", "scan_513", "err_after_512"} {
		t.Run(mode, func(t *testing.T) {
			p, facts, want := retentionWitnessSeed(t, pool, 513, 1)
			if mode == "late_zero" {
				if _, err := pool.Exec(t.Context(), "UPDATE page_links SET resolved_id=$3 WHERE source_id=$1 AND range_start=$2", p.ID, int32(1024), uuid.Nil()); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "late_notebook" {
				if _, err := pool.Exec(t.Context(), "UPDATE page_links SET notebook_id=$3 WHERE source_id=$1 AND range_start=$2", p.ID, int32(1024), uuid.UUID{15: 99}); err != nil {
					t.Fatal(err)
				}
			}
			before := retentionCollectorImage(t, pool)
			conn, err := pool.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			rows := &linksCollectorRows{stable: true, columns: 13}
			fault := mode == "scan_513" || mode == "err_after_512"
			if fault {
				rows.mode = mode
			}
			w := &retentionCollectorWatch{rows: rows, pid: conn.Conn().PgConn().PID()}
			rollback := errors.New("witness tail rollback")
			var returned error
			var dropped app.Dropped
			err = postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
				store := New(pool)
				if err := store.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				tx, _ := postgres.TxFrom(ctx)
				if _, err := tx.Exec(ctx, "UPDATE indexed_pages SET revision=99 WHERE node_id=$1", p.ID); err != nil {
					return err
				}
				queryCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				rows.cancel = cancel
				p.Revision = 2
				dropped, returned = store.ReplacePage(retentionCollectorContext{Context: queryCtx, watch: w}, p, facts)
				if returned != nil {
					return returned
				}
				if fault {
					return nil
				}
				rows.check(t, 513, 1, 513, 1)
				stages := retentionCollectorStages()
				stages[1] = "DeleteLinksOf"
				stages[7] = "InsertLinks"
				retentionCollectorCheckCalls(t, w, p, stages)
				ids := make([]uuid.UUID, 22)
				for i := range ids {
					ids[i] = uuid.UUID{15: byte(i + 1)}
				}
				if mode == "late_zero" {
					ids = append([]uuid.UUID{uuid.Nil()}, ids...)
				}
				if !reflect.DeepEqual(dropped, app.Dropped{Targets: ids}) {
					t.Fatalf("DROPPED_WITNESS_FALLBACK: got%+v want original complete legacy targets%v", dropped, ids)
				}
				for i := range want {
					want[i].Resolution = domain.Resolution{}
				}
				got, err := store.Links(ctx, p.NotebookID, domain.Reach{Sources: []uuid.UUID{p.ID}})
				if err != nil || !slices.Equal(got, want) {
					t.Fatalf("DROPPED_WITNESS_LEGACY: complete reset=%+v err%v", got, err)
				}
				return rollback
			})
			if fault {
				if rows.cause == nil || !errors.Is(returned, rows.cause) || !errors.Is(err, rows.cause) || returned.Error() != fmt.Sprintf("inspect the retained links of page %s: %s", p.ID, rows.cause) || !reflect.DeepEqual(dropped, app.Dropped{}) || errors.Is(returned, context.Canceled) {
					t.Fatalf("DROPPED_WITNESS_NATIVE_TAIL: returned%v manager%v native%v dropped%+v", returned, err, rows.cause, dropped)
				}
				if mode == "scan_513" {
					rows.check(t, 513, 0, 513, 0)
				} else {
					rows.check(t, 512, 1, 512, 1)
				}
				retentionCollectorCheckCalls(t, w, p, []string{"InspectPageLinksForRetention"})
				retentionCollectorCheckTrace(t, trace, w.pid, p, rows.cause)
			} else if !errors.Is(err, rollback) {
				t.Fatalf("witness tail original rollback: %v", err)
			}
			if !slices.Equal(retentionCollectorImage(t, pool), before) {
				t.Fatal("DROPPED_WITNESS_ROLLBACK: complete five-table image changed")
			}
			linksCollectorReusable(t, conn)
		})
	}
}
