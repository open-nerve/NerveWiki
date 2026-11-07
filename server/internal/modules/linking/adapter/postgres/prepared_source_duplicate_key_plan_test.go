//go:build !race

package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// One additional physical case: repeated input keys still select the original
// three literal rows, within the same work/loop/filter/temp guards. The Store
// receives the duplicates; neither a test helper nor the adapter removes them.
// The named replay is separate evidence, not the native call's attached plan.
func TestPreparedSourceResidualDuplicateKeyPlan(t *testing.T) {
	if testing.Short() {
		t.Skip("native PostgreSQL duplicate-key physical observation")
	}
	const mode = "force_generic_plan"
	f := newStablePlanFixture(t, "analyzed", preparedPlanSpecs(), true)
	rollback := errors.New("duplicate-key physical observation must roll back")
	err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
		tx, _ := postgres.TxFrom(ctx)
		stablePlanSettings(t, ctx, tx, mode)
		if err := f.s.Lock(ctx, f.eng); err != nil {
			return err
		}
		reach := domain.Reach{
			Keys:    []string{"d", "d", "d", "d", "d", "d", "d", "d"},
			Targets: []uuid.UUID{stablePlanID(104)},
			Sources: []uuid.UUID{f.specs[1].ID, f.specs[5].ID, f.specs[0].ID, f.specs[2].ID},
		}
		keys, sources := slices.Clone(reach.Keys), slices.Clone(reach.Sources)
		excluded := []uuid.UUID{f.specs[0].ID, f.specs[1].ID, f.specs[2].ID}
		want := []app.Link{
			{SourceID: stablePlanID(20), Start: int(f.specs[3].Old[0].Start), Target: "d", Resolution: domain.Resolution{ID: stablePlanID(103)}},
			{SourceID: stablePlanID(21), Start: int(f.specs[4].Old[0].Start), Target: "e", Resolution: domain.Resolution{ID: stablePlanID(104)}},
			{SourceID: stablePlanID(22), Start: int(f.specs[5].Old[0].Start), Target: "a", Resolution: domain.Resolution{ID: stablePlanID(100)}},
		}
		var captured stablePlanCall
		var nativeWork []map[string]stablePlanWork
		for pass := range 2 {
			work := stablePlanRelationWork(t, ctx, tx)
			f.trace.begin()
			got, err := f.s.LinksExcept(ctx, f.eng, reach, excluded)
			calls := f.trace.end()
			delta := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
			if err != nil || len(calls) != 1 || !slices.Equal(got, want) {
				t.Fatalf("DUPLICATE_KEY_VALUES: actual calls%+v rows%+v err%v", calls, got, err)
			}
			c := calls[0]
			wantArgs := []any{excluded, keys, f.eng, []uuid.UUID{stablePlanID(104)}, []uuid.UUID{stablePlanID(22)}}
			if c.Name != "LinksReachedExcept" || c.SQL != gen.LinksReachedExcept || c.PID != tx.Conn().PgConn().PID() ||
				c.Error != "" || c.RowsAffected != 3 || !reflect.DeepEqual(c.Args, wantArgs) ||
				!slices.Equal(c.LeadingOptions, []string{fmt.Sprintf("leading=true mode=%v", pgx.QueryExecModeDescribeExec)}) ||
				!slices.Equal(reach.Keys, keys) || !slices.Equal(reach.Sources, sources) {
				t.Fatalf("DUPLICATE_KEY_NATIVE_ARGS: actual SQL/duplicates/source ownership=%+v", c)
			}
			if count := stablePlanCounter(t, ctx, tx, c.SQL); count.Name != "" {
				t.Fatalf("duplicate-key native path retained named SQL: %+v", count)
			}
			lintEvidence(t, "prepared-duplicate-key-actual", map[string]any{
				"state": f.state, "mode": mode, "pass": pass, "call": c, "work": delta,
				"input_keys": 8, "distinct_keys": 1, "residual_rows": 3,
				"scope": "actual Store LinksExcept DescribeExec; input still contains eight literal d keys",
			})
			nativeWork = append(nativeWork, delta)
			captured = c
		}
		// Emit the complete named plan/counters before the original guards
		// judge it, then judge both actual native calls by the same work bound.
		preparedPlanKernel(t, ctx, f, nil, captured, mode)
		for _, work := range nativeWork {
			preparedPlanResidualWork(t, work, f.state)
		}
		return rollback
	})
	if !errors.Is(err, rollback) || !slices.Equal(f.trace.terminal(), []string{"rollback"}) {
		t.Fatalf("duplicate-key physical rollback=%v", err)
	}
	stablePlanAssertSnapshot(t, t.Context(), f, nil)
}
