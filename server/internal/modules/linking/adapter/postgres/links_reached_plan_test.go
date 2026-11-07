//go:build !race

package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The three disjoint selected rows have the same physical bound as the
// existing residual fixture. Ordinary Links has no excluded-source input:
// neither dense supplied source belongs in this reach. Full raw observations,
// rollback and the stored fixture comparison precede every work guard.
func TestLinksReachedReadPlans(t *testing.T) {
	if testing.Short() {
		t.Skip("native PostgreSQL LinksReached physical observation")
	}
	for _, state := range []string{"no_statistics", "analyzed", "app_rebuild_after_analyzed"} {
		t.Run(state, func(t *testing.T) {
			f := newStablePlanFixture(t, state, preparedPlanSpecs(), true)
			for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
				t.Run(mode, func(t *testing.T) {
					want := []app.Link{
						{SourceID: stablePlanID(20), Start: 141, Target: "d", Resolution: domain.Resolution{ID: stablePlanID(103)}},
						{SourceID: stablePlanID(21), Start: 153, Target: "e", Resolution: domain.Resolution{ID: stablePlanID(104)}},
						{SourceID: stablePlanID(22), Start: 153, Target: "a", Resolution: domain.Resolution{ID: stablePlanID(100)}},
					}
					reach := domain.Reach{Keys: []string{"d"}, Targets: []uuid.UUID{stablePlanID(104)}, Sources: []uuid.UUID{stablePlanID(22)}}
					var nativeWork []map[string]stablePlanWork
					var replay stablePlanEvidence
					var readDWork map[string]stablePlanWork
					rollback := errors.New("LinksReached physical observation must roll back")
					err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
						tx, ok := postgres.TxFrom(ctx)
						if !ok || postgres.DB(ctx, f.pool) != tx {
							t.Fatal("LinksReached observation lost its original transaction")
						}
						settings := stablePlanSettings(t, ctx, tx, mode)
						if err := f.s.Lock(ctx, f.eng); err != nil {
							return err
						}
						var xid string
						if err := tx.QueryRow(ctx, "SELECT pg_current_xact_id()::text").Scan(&xid); err != nil {
							return err
						}
						var captured stablePlanCall
						for pass := range 2 {
							var work map[string]stablePlanWork
							captured, work = linksReachedPlanActual(t, ctx, f, reach, want, mode, xid, pass)
							nativeWork = append(nativeWork, work)
						}
						replay, readDWork = linksReachedPlanKernel(t, ctx, f, captured, want, mode, settings)
						return rollback
					})
					terminal := f.trace.terminal()
					lintEvidence(t, "links-reached-rollback", map[string]any{"state": state, "mode": mode,
						"terminal": terminal, "original_cause": errors.Is(err, rollback)})
					if !errors.Is(err, rollback) || !slices.Equal(terminal, []string{"rollback"}) {
						t.Fatalf("LinksReached physical rollback: %v terminal=%v", err, terminal)
					}
					stablePlanAssertSnapshot(t, t.Context(), f, nil)
					lintEvidence(t, "links-reached-snapshot", map[string]any{"state": state, "mode": mode,
						"all_five_tables_equal": true, "selected_rows": want,
						"same_notebook_noise_rows": 10000, "foreign_noise_rows": 200000, "dense_supplied_rows": 8192})
					// This exact guard is shared without changing its original
					// three-row/version numbers. Native work is judged first.
					for _, work := range append(nativeWork, readDWork, replay.ExecutionWork) {
						for relation, w := range work {
							if w.Inserted != 0 || w.Updated != 0 || w.Deleted != 0 {
								t.Fatalf("LinksReached read performed DML on %s: %+v", relation, w)
							}
						}
						preparedPlanResidualWork(t, work, state)
					}
					linksReachedPlanTree(t, replay.Plan)
				})
			}
		})
	}
}

func linksReachedPlanActual(t *testing.T, ctx context.Context, f *stablePlanFixture, reach domain.Reach, want []app.Link, mode, xid string, pass int) (stablePlanCall, map[string]stablePlanWork) {
	t.Helper()
	tx, _ := postgres.TxFrom(ctx)
	original := domain.Reach{Keys: slices.Clone(reach.Keys), Targets: slices.Clone(reach.Targets), Sources: slices.Clone(reach.Sources)}
	beforeChoice := stablePlanCounter(t, ctx, tx, gen.LinksReached)
	work := stablePlanRelationWork(t, ctx, tx)
	f.trace.begin()
	f.trace.mu.Lock()
	delete(f.trace.prepares, gen.LinksReached)
	f.trace.mu.Unlock()
	got, err := f.s.Links(ctx, f.eng, reach)
	calls := f.trace.end()
	if tx.Conn().PgConn().IsBusy() {
		t.Fatalf("actual LinksReached left native Rows busy: calls=%+v err=%v", calls, err)
	}
	delta := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	afterChoice := stablePlanCounter(t, ctx, tx, gen.LinksReached)
	var afterXID string
	if err := tx.QueryRow(ctx, "SELECT pg_current_xact_id()::text").Scan(&afterXID); err != nil {
		t.Fatal(err)
	}
	f.trace.mu.Lock()
	prepareName, prepared := f.trace.prepares[gen.LinksReached]
	f.trace.mu.Unlock()
	lintEvidence(t, "links-reached-actual", map[string]any{"state": f.state, "mode": mode, "pass": pass,
		"calls": calls, "sql_sha": fmt.Sprintf("%x", sha256.Sum256([]byte(gen.LinksReached))), "work": delta,
		"rows": got, "want": want, "error": fmt.Sprint(err), "tx_identity": fmt.Sprintf("%p", tx),
		"xid_before": xid, "xid_after": afterXID, "tx_status": string(tx.Conn().PgConn().TxStatus()),
		"before_choice": beforeChoice, "after_choice": afterChoice,
		"prepare_observed": prepared, "prepare_name": prepareName, "native_rows_closed": true,
		"scope": "actual Store.Links DescribeExec; no attached execution plan claimed"})
	if err != nil || len(calls) != 1 || got == nil || !slices.Equal(got, want) || cap(got) != len(want) {
		t.Fatalf("LINKS_REACHED_VALUES: actual calls=%+v rows=%+v err=%v", calls, got, err)
	}
	c := calls[0]
	wantArgs := []any{f.eng, original.Keys, original.Targets, original.Sources}
	cfg := f.pool.Config().ConnConfig
	if c.Name != "LinksReached" || c.SQL != gen.LinksReached || c.Kind != ":many" || c.Error != "" ||
		c.PID != tx.Conn().PgConn().PID() || c.CommandTag != "SELECT 3" || c.RowsAffected != 3 ||
		!slices.Equal(c.LeadingOptions, []string{fmt.Sprintf("leading=true mode=%v", pgx.QueryExecModeDescribeExec)}) ||
		!slices.Equal(c.ArgumentTypes, []string{"uuid.UUID", "[]string", "[]uuid.UUID", "[]uuid.UUID"}) ||
		!reflect.DeepEqual(c.Args, wantArgs) || !reflect.DeepEqual(reach, original) ||
		c.DefaultMode != cfg.DefaultQueryExecMode || c.StatementCapacity != cfg.StatementCacheCapacity ||
		c.DescriptionCapacity != cfg.DescriptionCacheCapacity || !prepared || prepareName != "" ||
		beforeChoice.Name != "" || afterChoice.Name != "" || afterXID != xid || tx.Conn().PgConn().TxStatus() != 'T' {
		t.Fatalf("LINKS_REACHED_NATIVE_ARGS: actual identity/binding/ownership=%+v", c)
	}
	return c, delta
}

// The kernel reuses the captured SQL and four original arguments. Its named
// plans are separate diagnostics; the DataRow replay retains DescribeExec.
// The existing prepared kernel has five/fifteen-slot residual/Inspect shapes
// and is deliberately not used for this four-parameter, seven-column query.
func linksReachedPlanKernel(t *testing.T, ctx context.Context, f *stablePlanFixture, c stablePlanCall, want []app.Link, mode string, settings map[string]string) (stablePlanEvidence, map[string]stablePlanWork) {
	t.Helper()
	tx, _ := postgres.TxFrom(ctx)
	desc, err := tx.Conn().PgConn().Prepare(ctx, "", c.SQL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Args) != 4 || !slices.Equal(desc.ParamOIDs, []uint32{pgtype.UUIDOID, pgtype.TextArrayOID, pgtype.UUIDArrayOID, pgtype.UUIDArrayOID}) {
		t.Fatalf("LinksReached four ordered parameter OIDs=%v", desc.ParamOIDs)
	}
	work := stablePlanRelationWork(t, ctx, tx)
	d := linksReachedPlanReadD(t, ctx, tx, c, want)
	readDWork := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	lintEvidence(t, "links-reached-ReadD-replay", map[string]any{"state": f.state, "mode": mode,
		"call": c, "D": d, "work": readDWork, "scope": "independent seven-column DescribeExec replay"})
	if !d.Closed || d.CloseCalls != 1 {
		t.Fatalf("LinksReached DataRow replay did not release its native connection: %+v", d)
	}
	const name = "links_reached_physical_observation"
	if _, err := tx.Conn().Prepare(ctx, name, c.SQL); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Conn().Deallocate(ctx, name); err != nil {
			t.Fatal(err)
		}
	}()
	e := stablePlanEvidence{Case: t.Name(), Scope: "independent named captured-SQL replay; not native Store.Links attached plan",
		State: f.state, Mode: mode, Name: c.Name, SQL: c.SQL, SQLSHA: fmt.Sprintf("%x", sha256.Sum256([]byte(c.SQL))),
		ArgumentSHA: c.ArgumentSHA, PID: tx.Conn().PgConn().PID(), Statistics: f.statistics, Settings: settings,
		Encoding: stablePlanEncoding{OIDs: slices.Clone(desc.ParamOIDs), InputWireFormats: "NOT_OBSERVED"}, D: &d,
		PlanningObservation: "separate named EXPLAIN without ANALYZE before named execution"}
	execute := stablePlanExecute(t, name, c.Args)
	beforeChoice := stablePlanCounter(t, ctx, tx, c.SQL)
	work = stablePlanRelationWork(t, ctx, tx)
	var planOnly string
	if err := tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+execute).Scan(&planOnly); err != nil {
		t.Fatal(err)
	}
	e.PlanningPlan = json.RawMessage(planOnly)
	e.PlanningWork = stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	afterChoice := stablePlanCounter(t, ctx, tx, c.SQL)
	lintEvidence(t, "links-reached-plan-only", map[string]any{"state": f.state, "mode": mode, "call": c,
		"plan": e.PlanningPlan, "work": e.PlanningWork, "parameter_oids": desc.ParamOIDs,
		"before_choice": beforeChoice, "after_choice": afterChoice})
	e.BeforeChoice = afterChoice
	work = stablePlanRelationWork(t, ctx, tx)
	var raw string
	if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+execute).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	e.Plan = json.RawMessage(raw)
	e.ExecutionWork = stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	e.AfterChoice = stablePlanCounter(t, ctx, tx, c.SQL)
	lintEvidence(t, "links-reached-physical-replay", e)
	stablePlanCountDelta(t, beforeChoice, afterChoice, mode)
	stablePlanCountDelta(t, e.BeforeChoice, e.AfterChoice, mode)
	return e, readDWork
}

func linksReachedPlanReadD(t *testing.T, ctx context.Context, tx pgx.Tx, c stablePlanCall, want []app.Link) (out stablePlanD) {
	t.Helper()
	rows, err := postgres.Planned(tx).Query(ctx, c.SQL, c.Args...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		rows.Close()
		out.CloseCalls++
		out.Closed = !tx.Conn().PgConn().IsBusy()
	}()
	for _, field := range rows.FieldDescriptions() {
		out.OIDs = append(out.OIDs, field.DataTypeOID)
		out.Formats = append(out.Formats, field.Format)
		if field.Format != tx.Conn().TypeMap().FormatCodeForOID(field.DataTypeOID) {
			t.Fatalf("LinksReached native field format %s=%d", field.Name, field.Format)
		}
	}
	if !slices.Equal(out.OIDs, []uint32{pgtype.UUIDOID, pgtype.Int4OID, pgtype.TextOID, pgtype.UUIDOID, pgtype.BoolOID, pgtype.BoolOID, pgtype.BoolOID}) {
		t.Fatalf("LinksReached seven result OIDs=%v", out.OIDs)
	}
	out.FieldBytes, out.FieldNulls = make([]int64, 7), make([]int, 7)
	for rows.Next() {
		raw := rows.RawValues()
		if len(raw) != 7 {
			t.Fatalf("LinksReached DataRow fields=%d", len(raw))
		}
		out.DataRowBytes += 7 + 4*int64(len(raw))
		for i, value := range raw {
			out.FieldBytes[i] += int64(len(value))
			out.DataRowBytes += int64(len(value))
			if value == nil {
				out.FieldNulls[i]++
			}
		}
		var source uuid.UUID
		var start int32
		var target string
		var resolved pgtype.UUID
		var ambiguous, asset, aliases bool
		if err := rows.Scan(&source, &start, &target, &resolved, &ambiguous, &asset, &aliases); err != nil {
			t.Fatal(err)
		}
		if out.Rows >= len(want) {
			t.Fatal("LinksReached DataRow replay returned an extra row")
		}
		w := want[out.Rows]
		if source != w.SourceID || start != int32(w.Start) || target != w.Target ||
			resolved != (pgtype.UUID{Bytes: w.Resolution.ID, Valid: true}) ||
			ambiguous != w.Resolution.Ambiguous || asset != w.Resolution.Asset || aliases != w.Aliases {
			t.Fatalf("LinksReached complete DataRow %d differs from literal %+v", out.Rows, w)
		}
		out.Rows++
	}
	out.EOFs++
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if out.Rows != 3 || out.EOFs != 1 ||
		!slices.Equal(out.FieldBytes, []int64{48, 12, 3, 48, 3, 3, 3}) ||
		!slices.Equal(out.FieldNulls, []int{0, 0, 0, 0, 0, 0, 0}) || out.DataRowBytes != 225 {
		t.Fatalf("LinksReached complete seven-column native DataRow=%+v", out)
	}
	return out
}

func linksReachedPlanTree(t *testing.T, raw json.RawMessage) {
	t.Helper()
	root := stablePlanTree(t, raw)
	if root.Rows != 3 || root.Loops != 1 {
		t.Fatalf("LinksReached result rows/loops=%v/%v", root.Rows, root.Loops)
	}
	for _, node := range root.all() {
		if !node.RowsObserved || !node.LoopsObserved || node.Loops > 1 {
			t.Fatalf("LinksReached row/loop observation=%+v", node)
		}
		if node.Relation != "" && node.Relation != "page_links" {
			t.Fatalf("LinksReached unexpected relation=%+v", node)
		}
		if node.Relation == "page_links" && node.Rows+node.Removed+node.Rechecked > 3 || node.JoinRemoved != 0 {
			t.Fatalf("LinksReached visited/filtered more than three disjoint literal rows: %+v", node)
		}
	}
	var document []struct {
		Plan struct {
			Read    int64 `json:"Temp Read Blocks"`
			Written int64 `json:"Temp Written Blocks"`
		}
	}
	if err := json.Unmarshal(raw, &document); err != nil || len(document) != 1 {
		t.Fatalf("LinksReached temp plan: %v", err)
	}
	if document[0].Plan.Read != 0 || document[0].Plan.Written != 0 {
		t.Fatalf("LinksReached three narrow rows spilled: %+v", document[0].Plan)
	}
}
