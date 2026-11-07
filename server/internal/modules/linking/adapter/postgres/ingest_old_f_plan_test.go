//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The caller has proved every source head before using this private reader.
// Public body_heads still exercises its separate current-head predicate.
func testIngestOldBodyPlans(t *testing.T) {
	for _, state := range []string{"no_statistics", "analyzed/pristine", "analyzed/app_rebuild"} {
		t.Run(state, func(t *testing.T) {
			f, q, _ := novelRangeFixture(t, state)
			ids := make([]uuid.UUID, 0, len(q.CurrentSources))
			for _, v := range q.CurrentSources {
				ids = append(ids, v.ID)
			}
			// Literal Markdown in novelRangeFixture: one S->A, then 400
			// sources with 128 [[junk.bin]] lines after a 23-byte header.
			want := []app.BodyReference{{EdgePos: app.EdgePos{SourceID: novelRangeID(4), Start: 25}, ResolvedID: novelRangeID(1), ResolvedAsset: true}}
			for i := range 400 {
				for j := range 128 {
					want = append(want, app.BodyReference{EdgePos: app.EdgePos{SourceID: novelRangeID(uint64(1000 + i)), Start: int32(25 + 13*j)}, ResolvedID: novelRangeID(6), ResolvedAsset: true})
				}
			}
			for _, part := range []struct {
				name  string
				after *app.EdgePos
				take  int
				want  []app.BodyReference
				live  int64
			}{
				{"first_sparse", nil, 512, want[:512], 512 + 515 + 1 + 2},
				{"after_dense", &want[511].EdgePos, 512, want[512:1024], 512 + 515 + 1 + 2},
				{"last", &want[len(want)-2].EdgePos, 512, want[len(want)-1:], 512 + 515 + 1 + 2},
				{"eof", &want[len(want)-1].EdgePos, 512, nil, 512 + 515 + 1 + 2},
				{"first_16384", nil, 16384, want[:16384], 16384 + 515 + 1 + 2},
				{"after_16384_cross_source", &want[16383].EdgePos, 16384, want[16384:32768], 16384 + 2},
			} {
				t.Run(part.name, func(t *testing.T) {
					// First crosses the complete unrequested Qf (515) and Qb (1).
					// After16384 begins at source1127's final row, then crosses
					// sources1128..1255; those two unrequested ranges are behind it.
					// Keep the old512 ceilings. Two endpoint probes are planning.
					versions := int64(1)
					if strings.HasSuffix(state, "/app_rebuild") {
						versions = 3
					}
					ingestOldBodyPhysicalCalls(t, f, ids, part.after, part.take, part.want, part.live, versions)
				})
			}
		})
	}
	// Keep the complete 100k parameter relation independently of the dense
	// fixture. The additional 2,000 emitters have three interleaved rows each.
	t.Run("complete_100k", func(t *testing.T) {
		f := lintPlanFixtureKind(t, "00000000", 50000, "complete")
		// These additional eligible heads have no body. Their presence avoids
		// pretending unqualified source IDs satisfy the private precondition.
		lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ('00000001-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,true FROM generate_series(1,100000) k`, f.eng)
		ids := make([]uuid.UUID, 0, 102000)
		for k := uint64(1); k <= 100000; k++ {
			id := lintID(k)
			id[3] = 1
			ids = append(ids, id)
		}
		for k := uint64(1); k <= 2000; k++ {
			ids = append(ids, lintReaderID(k))
		}
		var want []app.BodyReference
		for k := uint64(1001); k <= 1512; k++ {
			want = append(want, app.BodyReference{EdgePos: app.EdgePos{SourceID: lintReaderID(k)}, ResolvedID: lintReaderID(2000)})
		}
		for _, state := range []string{"no_statistics", "analyzed"} {
			if state == "analyzed" {
				lintAnalyze(t, f)
			}
			t.Run(state, func(t *testing.T) {
				ingestOldBodyPhysicalCalls(t, f, ids, &app.EdgePos{SourceID: lintReaderID(1000)}, 512, want, 3*512+2, 1)
			})
		}
	})
}

// Capture actual adapter SQL/parameters on the real ctx Tx. Measure its actual
// cache route separately from the explicit PREPARE/EXECUTE generic comparison.
func ingestOldBodyPhysicalCalls(t *testing.T, f fixture, ids []uuid.UUID, after *app.EdgePos, take int, want []app.BodyReference, live, versions int64) {
	t.Helper()
	original := slices.Clone(ids)
	ordered := slices.Clone(ids)
	slices.SortFunc(ordered, uuid.UUID.Compare)
	var captured statement
	for _, capacity := range []int{8, 0} {
		t.Run(fmt.Sprintf("actual_description_%d", capacity), func(t *testing.T) {
			tr := &ingestPublicCaptureTrace{}
			cfg := f.pool.Config()
			cfg.MaxConns = 1
			cfg.ConnConfig.DescriptionCacheCapacity = capacity
			cfg.ConnConfig.StatementCacheCapacity = 8
			cfg.ConnConfig.Tracer = tr
			pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			err = lintBodyTxManager(pool).WithinTx(t.Context(), func(ctx context.Context) error {
				tx, ok := postgres.TxFrom(ctx)
				if !ok {
					t.Fatal("private plan call is not in its actual Tx")
				}
				if err := postgresadapter.New(pool).Lock(ctx, f.eng); err != nil {
					return err
				}
				reader := postgresadapter.NewIngestReads(pool, 5<<20)
				for pass := range 2 {
					before := lintRelationWork(t, tx)
					got, err := reader.OldBodyReferences(ctx, f.eng, ids, after, take)
					work := lintWorkDelta(before, lintRelationWork(t, tx))
					if err != nil || !slices.Equal(got, want) {
						t.Fatalf("actual full body DTOs: got%v want%v err=%v", got, want, err)
					}
					calls := tr.observations()
					if len(calls) != pass+1 {
						t.Fatalf("old-F adapter skipped/retried SQL: %+v", calls)
					}
					call := calls[pass]
					name, mode := "IngestOldBodyReferencesFirst", pgx.QueryExecModeCacheDescribe
					if after != nil {
						name = "IngestOldBodyReferencesAfter"
					}
					if capacity == 0 {
						mode = pgx.QueryExecModeDescribeExec
					}
					lintAssertMode(t, call, name, mode, tx.Conn().PgConn().PID(), 8, capacity)
					args := []any{mode, f.eng, ordered[0], ordered[len(ordered)-1], int32(take), ids}
					if after != nil {
						args = []any{mode, f.eng, ordered[0], ordered[len(ordered)-1], after.SourceID, after.Start, int32(take), ids}
					}
					if call.Err != nil || !reflect.DeepEqual(call.Args, args) || !slices.Equal(ids, original) {
						t.Fatalf("private old-F actual arguments/input changed: got%+v want%+v err=%v", call.Args, args, call.Err)
					}
					captured = statement{sql: call.SQL, args: call.Args[1:]}
					lintEvidence(t, "private-old-f-actual", map[string]any{"pass": pass, "mode": mode.String(), "rows": len(got), "take": take, "live_ceiling": live, "versions": versions, "work": work, "complete_emitters": len(ids)})
					ingestOldBodyWork(t, work, live, versions)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	if captured.sql == "" {
		t.Fatal("missing actual adapter SQL capture")
	}
	for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
		t.Run(mode, func(t *testing.T) {
			var got []app.BodyReference
			plan, ev := lintExecutedMode(t, f, captured, mode, func(rows pgx.Rows, _ int) {
				var r app.BodyReference
				if err := rows.Scan(&r.SourceID, &r.Start, &r.ResolvedID, &r.ResolvedAsset); err != nil {
					t.Fatal(err)
				}
				got = append(got, r)
			})
			if !slices.Equal(got, want) || ev.Rows != len(want) {
				t.Fatalf("complete prepared body DTOs: got%v want%v", got, want)
			}
			lintAssertRangeRows(t, plan, ev, after != nil, true, live*versions)
			ingestOldBodyWork(t, ev.ExecutionWork, live, versions)
			if len(want) > 0 {
				lintAssertMembership(t, ev, "emitters", len(ids))
			}
			if len(plan.all(readsOf("indexed_pages"))) != 0 {
				t.Fatal("private old-F repeated qualified head work")
			}
			for _, n := range stablePlanTree(t, ev.Plan).all() {
				if strings.Contains(n.Type, "Sort") {
					t.Fatalf("old-F range ordered only after materialization: %+v", n)
				}
				if n.Relation == "page_links" {
					visited := n.Rows + n.Removed + n.Rechecked
					if n.Loops > 1 || visited > float64(live) {
						t.Fatalf("old-F physical range exceeded live ledger: %+v", n)
					}
					// These two new legs have a literal complete live range:
					// First16900 including 516 rejected rows; After16384.
					if take == 16384 && (n.Loops != 1 || visited != float64(live-2)) {
						t.Fatalf("old-F16384 range/filter ledger differs: %+v want%d", n, live-2)
					}
				}
			}
			if take == 16384 {
				// This fixture has only 403 UUID emitters and no sort/materialized
				// result stream. Its new legs must not spill. The unchanged100k
				// relation retains its original separate raw-evidence review.
				var document []struct {
					Plan struct {
						Read    int64 `json:"Temp Read Blocks"`
						Written int64 `json:"Temp Written Blocks"`
					}
				}
				if err := json.Unmarshal(ev.Plan, &document); err != nil || len(document) != 1 {
					t.Fatalf("old-F temp plan: %v", err)
				}
				if document[0].Plan.Read != 0 || document[0].Plan.Written != 0 {
					t.Fatalf("old-F403-emitter streaming plan spilled: %+v", document[0].Plan)
				}
			}
			// Full JSON retains filters, loops, buffers and temp. EXPLAIN work
			// is separate from actual desc8/desc0 adapter execution above.
		})
	}
}

func ingestOldBodyWork(t *testing.T, work map[string]lintWork, live, versions int64) {
	t.Helper()
	for name, w := range work {
		if strings.HasPrefix(name, "indexed_pages") && w != (lintWork{}) {
			t.Errorf("private old-F repeated head work %s: %+v", name, w)
		}
		if name == "page_links" && (w.Scans != 0 || w.Returned > live || w.Fetched > live) {
			t.Errorf("private old-F heap exceeds range %d: %+v", live, w)
		}
		if strings.HasPrefix(name, "page_links_") && w.Returned > live*versions {
			t.Errorf("private old-F index exceeds %d versions of %d live rows: %s %+v", versions, live, name, w)
		}
	}
}
