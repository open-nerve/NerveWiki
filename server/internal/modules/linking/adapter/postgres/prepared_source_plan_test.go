//go:build !race

package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Three statistics states x three actual plan_cache_mode settings. These are
// read-only adapter/kernel observations: Apply/fresh resolve/purge are covered
// by the separate prepared Store and bootstrap tests. All dense incoming facts
// equal stored facts, so the residual link relation is identical before Apply.
// Avoiding DML here preserves the fixture's literal pristine/rebuild ledger.
func TestPreparedSourceReadPlans(t *testing.T) {
	if testing.Short() {
		t.Skip("native PostgreSQL physical observation")
	}
	for _, state := range []string{"no_statistics", "analyzed", "app_rebuild_after_analyzed"} {
		t.Run(state, func(t *testing.T) {
			f := newStablePlanFixture(t, state, preparedPlanSpecs(), true)
			for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
				t.Run(mode, func(t *testing.T) {
					rollback := errors.New("prepared physical read observation")
					err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
						tx, _ := postgres.TxFrom(ctx)
						stablePlanSettings(t, ctx, tx, mode)
						if err := f.s.Lock(ctx, f.eng); err != nil {
							return err
						}
						preparedPlanHeads(t, ctx, f, f.specs[:3])
						unit := &app.SourcePreparationUnit{NotebookID: f.eng}
						var inspect []stablePlanCall
						for _, s := range f.specs[:3] {
							inspect = append(inspect, preparedPlanActualInspect(t, ctx, f, s, unit, mode))
						}
						// The exact source set still includes both dense sources on
						// entry. Actual adapter args must retain only global_source.
						reach := domain.Reach{Keys: []string{"d"}, Targets: []uuid.UUID{stablePlanID(104)},
							Sources: []uuid.UUID{f.specs[1].ID, f.specs[5].ID, f.specs[0].ID, f.specs[2].ID}}
						excluded := []uuid.UUID{f.specs[0].ID, f.specs[1].ID, f.specs[2].ID}
						residual := preparedPlanActualResidual(t, ctx, f, reach, excluded, mode)
						// Reuse exact captured SQL/args, never a hand-written query.
						// The second dense source has the same physical bound; the
						// separate empty-source plan proves the sentinel/CTE case.
						preparedPlanKernel(t, ctx, f, &f.specs[0], inspect[0], mode)
						preparedPlanKernel(t, ctx, f, &f.specs[2], inspect[2], mode)
						preparedPlanKernel(t, ctx, f, nil, residual, mode)
						return rollback
					})
					if !errors.Is(err, rollback) {
						t.Fatal(err)
					}
					if !slices.Equal(f.trace.terminal(), []string{"rollback"}) {
						t.Fatal("physical observation did not end with original rollback")
					}
				})
			}
			stablePlanAssertSnapshot(t, t.Context(), f, nil)
		})
	}
}

func preparedPlanSpecs() []stablePlanSpec {
	return []stablePlanSpec{
		stablePlanSpecOf("supplied_a", "stable", stablePlanID(10), 4096, false),
		stablePlanSpecOf("supplied_b", "stable", stablePlanID(11), 4096, false),
		stablePlanSpecOf("supplied_empty", "empty_to_empty", stablePlanID(12), 0, false),
		preparedPlanOne("global_key", "d", stablePlanID(20), stablePlanID(103)),
		preparedPlanOne("global_target", "e", stablePlanID(21), stablePlanID(104)),
		preparedPlanOne("global_source", "a", stablePlanID(22), stablePlanID(100)),
		stablePlanSpecOf("e", "target", stablePlanID(104), 0, false),
	}
}

// One-byte literal targets preserve the existing fixture's independently
// known range positions. The real parser/rebuild verifies the changed text.
func preparedPlanOne(name, target string, id, resolved uuid.UUID) stablePlanSpec {
	s := stablePlanSpecOf(name, "stable", id, 1, false)
	s.OldText = strings.Replace(s.OldText, "[[a]]\n", "[["+target+"]]\n", 1)
	s.NewText = strings.Replace(s.NewText, "[[a]]\n", "[["+target+"]]\n", 1)
	s.OldFacts.Links[0].Target, s.NewFacts.Links[0].Target = target, target
	for _, rows := range [][]stablePlanRow{s.Old, s.Deleted, s.Written, s.Fresh} {
		rows[0].Target, rows[0].Key = target, pgtype.Text{String: target, Valid: true}
		rows[0].Resolved = pgtype.UUID{Bytes: resolved, Valid: true}
	}
	return s
}

func preparedPlanHeads(t *testing.T, ctx context.Context, f *stablePlanFixture, specs []stablePlanSpec) {
	t.Helper()
	wanted := make([]app.IngestRevision, len(specs))
	for i, s := range specs {
		wanted[i] = app.IngestRevision{ID: s.ID, Revision: 1}
	}
	heads, err := postgresadapter.NewIngestReads(f.pool, 5<<20).IndexedHeads(ctx, f.eng, wanted)
	if err != nil || len(heads) != len(wanted) {
		t.Fatalf("physical fixture current heads: %v", err)
	}
	for i, h := range heads {
		if h.ID != wanted[i].ID || !h.Present || h.PageRevision != 1 || h.IndexedRevision != 1 || h.Extractor != 2 {
			t.Fatalf("physical fixture unqualified head: %+v", h)
		}
	}
}

func preparedPlanActualInspect(t *testing.T, ctx context.Context, f *stablePlanFixture, s stablePlanSpec, unit *app.SourcePreparationUnit, mode string) stablePlanCall {
	t.Helper()
	tx, _ := postgres.TxFrom(ctx)
	f.trace.begin()
	plan := f.s.NewSource(ctx, unit, app.Page{ID: s.ID, NotebookID: f.eng, Revision: 2}, s.NewFacts)
	if calls := f.trace.end(); plan == nil || len(calls) != 0 {
		t.Fatalf("actual plan creation issued SQL or declined canonical fixture: %+v", calls)
	}
	choice := stablePlanCounter(t, ctx, tx, gen.InspectPageLinksForPreparation)
	work := stablePlanRelationWork(t, ctx, tx)
	f.trace.begin()
	assets, err := plan.ReadBefore(ctx, unit)
	calls := f.trace.end()
	delta := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	after := stablePlanCounter(t, ctx, tx, gen.InspectPageLinksForPreparation)
	if err != nil || len(calls) != 1 {
		t.Fatalf("actual ReadBefore calls=%+v err=%v", calls, err)
	}
	c := calls[0]
	c.PreparedName = f.trace.prepareName(c.SQL)
	if c.Name != "InspectPageLinksForPreparation" || c.SQL != gen.InspectPageLinksForPreparation || c.PID != tx.Conn().PgConn().PID() || c.Error != "" || len(c.LeadingOptions) != 0 ||
		c.DefaultMode != pgx.QueryExecModeCacheStatement || c.StatementCapacity <= 0 || c.RowsAffected != int64(max(1, s.N)) || !reflect.DeepEqual(c.Args, []any{s.ID, f.eng, int32(2)}) {
		t.Fatalf("PREPARED_NATIVE_ARGS: actual Inspect=%+v", c)
	}
	if c.PreparedName != after.Name {
		t.Fatal("actual Inspect named statement identity changed")
	}
	stablePlanCountDelta(t, choice, after, mode)
	var wantAssets []uuid.UUID
	for _, r := range s.Old {
		if !r.Property.Valid && r.Resolved.Valid && r.Asset {
			wantAssets = append(wantAssets, uuid.UUID(r.Resolved.Bytes))
		}
	}
	if !slices.Equal(assets, wantAssets) {
		t.Fatal("actual complete F asset projection differs from independent fixture")
	}
	lintEvidence(t, "prepared-actual-inspect", map[string]any{"state": f.state, "mode": mode, "call": c,
		"before_choice": choice, "after_choice": after, "work": delta, "asset_rows": len(assets), "read_only_calibration": true})
	// Reuse the approved source-range work guard by its existing guard class;
	// evidence and executed SQL keep the new query's actual identity above.
	stablePlanCheckWork(t, s, stablePlanEvidence{Name: stablePlanInspect, State: f.state, ExecutionWork: delta})
	return c
}

func preparedPlanActualResidual(t *testing.T, ctx context.Context, f *stablePlanFixture, reach domain.Reach, excluded []uuid.UUID, mode string) stablePlanCall {
	t.Helper()
	tx, _ := postgres.TxFrom(ctx)
	original := slices.Clone(reach.Sources)
	want := []app.Link{
		{SourceID: stablePlanID(20), Start: int(f.specs[3].Old[0].Start), Target: "d", Resolution: domain.Resolution{ID: stablePlanID(103)}},
		{SourceID: stablePlanID(21), Start: int(f.specs[4].Old[0].Start), Target: "e", Resolution: domain.Resolution{ID: stablePlanID(104)}},
		{SourceID: stablePlanID(22), Start: int(f.specs[5].Old[0].Start), Target: "a", Resolution: domain.Resolution{ID: stablePlanID(100)}},
	}
	var captured stablePlanCall
	for pass := range 2 {
		work := stablePlanRelationWork(t, ctx, tx)
		f.trace.begin()
		got, err := f.s.LinksExcept(ctx, f.eng, reach, excluded)
		calls := f.trace.end()
		delta := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
		if err != nil || len(calls) != 1 || !slices.Equal(got, want) {
			t.Fatalf("actual residual calls%+v rows%+v err%v", calls, got, err)
		}
		c := calls[0]
		wantArgs := []any{excluded, []string{"d"}, f.eng, []uuid.UUID{stablePlanID(104)}, []uuid.UUID{stablePlanID(22)}}
		if c.Name != "LinksReachedExcept" || c.SQL != gen.LinksReachedExcept || c.Error != "" ||
			c.PID != tx.Conn().PgConn().PID() || c.RowsAffected != 3 ||
			!slices.Equal(c.LeadingOptions, []string{fmt.Sprintf("leading=true mode=%v", pgx.QueryExecModeDescribeExec)}) ||
			!reflect.DeepEqual(c.Args, wantArgs) || !slices.Equal(reach.Sources, original) {
			t.Fatalf("PREPARED_RESIDUAL_SOURCE_ARM: actual args/mode/source ownership=%+v", c)
		}
		// The native path is DescribeExec; named generic replay below is a
		// separate diagnostic and must not be mislabeled the native route.
		if count := stablePlanCounter(t, ctx, tx, c.SQL); count.Name != "" {
			t.Fatalf("native residual unexpectedly used/retained named SQL: %+v", count)
		}
		lintEvidence(t, "prepared-actual-residual", map[string]any{"state": f.state, "mode": mode, "pass": pass,
			"call": c, "work": delta, "dense_supplied_rows": 8192, "residual_rows": 3})
		preparedPlanResidualWork(t, delta, f.state)
		captured = c
	}
	return captured
}

// A separate tiny value replay distinguishes every new flag, range zero,
// nullable/empty text and UUID NULL. Its deliberate updates are not included
// in the pristine/rebuild physical-work ledger of the nine leaves above.
func TestPreparedSourceReadDValues(t *testing.T) {
	if testing.Short() {
		t.Skip("native PostgreSQL DataRow observation")
	}
	f := newStablePlanFixture(t, "no_statistics", []stablePlanSpec{
		stablePlanSpecOf("value_source", "stable", stablePlanID(10), 3, false),
	}, false)
	rollback := errors.New("prepared value ReadD observation")
	err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
		tx, _ := postgres.TxFrom(ctx)
		if err := f.s.Lock(ctx, f.eng); err != nil {
			return err
		}
		preparedPlanHeads(t, ctx, f, f.specs)
		s := f.specs[0]
		s.Old = slices.Clone(s.Old)
		s.Old[0].Start, s.Old[0].End = 0, 1
		s.Old[0].Anchor = pgtype.Text{String: "anchor", Valid: true}
		s.Old[0].Display = pgtype.Text{String: "first", Valid: true}
		s.Old[0].Ambiguous = true
		s.Old[1].Property, s.Old[1].Anchor = pgtype.Text{Valid: true}, pgtype.Text{Valid: true}
		s.Old[1].Key, s.Old[1].Aliases = pgtype.Text{Valid: true}, true
		s.Old[1].Resolved = pgtype.UUID{}
		s.Old[2].Display = pgtype.Text{Valid: true}
		s.Old[2].Alt = pgtype.Text{String: "\tΩ", Valid: true}
		s.Old[2].Asset = true
		for i, row := range s.Old {
			tag, err := tx.Exec(ctx, `UPDATE page_links SET range_start=$3,range_end=$4,
property_key=$5,anchor=$6,display=$7,target_key=$8,target_alt_key=$9,
aliases=$10,resolved_id=$11,ambiguous=$12,resolved_asset=$13
WHERE source_id=$1 AND range_start=$2`, s.ID, f.specs[0].Old[i].Start,
				row.Start, row.End, row.Property, row.Anchor, row.Display, row.Key, row.Alt,
				row.Aliases, row.Resolved, row.Ambiguous, row.Asset)
			if err != nil || tag.RowsAffected() != 1 {
				t.Fatalf("prepared literal value setup row%d: %v", i, err)
			}
		}
		unit := &app.SourcePreparationUnit{NotebookID: f.eng}
		f.trace.begin()
		plan := f.s.NewSource(ctx, unit, app.Page{ID: s.ID, NotebookID: f.eng, Revision: 2}, s.NewFacts)
		if plan == nil {
			t.Fatal("canonical value fixture declined preparation")
		}
		assets, err := plan.ReadBefore(ctx, unit)
		calls := f.trace.end()
		if err != nil || !slices.Equal(assets, []uuid.UUID{stablePlanID(102)}) || len(calls) != 1 {
			t.Fatalf("actual three-row prepared projection: assets%v calls%v err%v", assets, calls, err)
		}
		c := calls[0]
		if c.SQL != gen.InspectPageLinksForPreparation || c.PID != tx.Conn().PgConn().PID() ||
			c.RowsAffected != 3 || !reflect.DeepEqual(c.Args, []any{s.ID, f.eng, int32(2)}) {
			t.Fatalf("actual three-row ReadD query: %+v", c)
		}
		d := preparedPlanReadD(t, ctx, tx, c, s)
		lintEvidence(t, "prepared-ReadD-literal-values", map[string]any{"call": c, "D": d})
		if !d.CanRetain || d.ErrChecks != 1 || d.CloseCalls != 1 || !d.Closed ||
			!slices.Equal(d.Targets, []uuid.UUID{stablePlanID(100), stablePlanID(102)}) ||
			!slices.Equal(d.FieldBytes, []int64{3, 12, 12, 48, 24, 0, 3, 6, 5, 2, 3, 3, 32, 3, 3}) ||
			!slices.Equal(d.FieldNulls, []int{0, 0, 0, 0, 0, 2, 0, 1, 1, 0, 2, 0, 1, 0, 0}) || d.DataRowBytes != 360 {
			t.Fatalf("prepared literal binary/text DataRow bytes/NULLs differ: %+v", d)
		}
		return rollback
	})
	if !errors.Is(err, rollback) || !slices.Equal(f.trace.terminal(), []string{"rollback"}) {
		t.Fatalf("prepared ReadD value rollback: %v", err)
	}
	stablePlanAssertSnapshot(t, t.Context(), f, nil)
}
