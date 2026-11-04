//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// This independent replay observes actual native DataRow values/bytes. It is
// not a second collector implementation or an allocation oracle for Store.
type preparedPlanD struct {
	stablePlanD
	ErrChecks int
}

func preparedPlanReadD(t *testing.T, ctx context.Context, tx pgx.Tx, c stablePlanCall, s stablePlanSpec) (out preparedPlanD) {
	t.Helper()
	rows, err := tx.Query(ctx, c.SQL, c.Args...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		rows.Close()
		out.CloseCalls++
		out.Closed = !tx.Conn().PgConn().IsBusy()
		if !out.Closed {
			t.Fatal("prepared complete ReadD did not release its native connection")
		}
	}()
	wantOIDs := []uint32{pgtype.BoolOID, pgtype.Int4OID, pgtype.Int4OID, pgtype.UUIDOID,
		pgtype.TextOID, pgtype.TextOID, pgtype.TextOID, pgtype.TextOID, pgtype.TextOID,
		pgtype.TextOID, pgtype.TextOID, pgtype.BoolOID, pgtype.UUIDOID, pgtype.BoolOID, pgtype.BoolOID}
	for _, field := range rows.FieldDescriptions() {
		out.OIDs = append(out.OIDs, field.DataTypeOID)
		out.Formats = append(out.Formats, field.Format)
		if field.Format != tx.Conn().TypeMap().FormatCodeForOID(field.DataTypeOID) {
			t.Fatalf("prepared ReadD actual field format %s=%d", field.Name, field.Format)
		}
	}
	if !slices.Equal(out.OIDs, wantOIDs) {
		t.Fatalf("prepared actual fifteen result OIDs=%v want=%v", out.OIDs, wantOIDs)
	}
	out.CanRetain = true
	out.FieldBytes, out.FieldNulls = make([]int64, 15), make([]int, 15)
	seen := map[uuid.UUID]bool{}
	var allowed bool
	var start pgtype.Int4
	var row stablePlanRow
	for {
		if !rows.Next() {
			out.EOFs++
			break
		}
		raw := rows.RawValues()
		if len(raw) != 15 {
			t.Fatalf("prepared ReadD field count=%d", len(raw))
		}
		out.Rows++
		out.DataRowBytes += 7 + 4*int64(len(raw))
		for i, value := range raw {
			out.FieldBytes[i] += int64(len(value))
			out.DataRowBytes += int64(len(value))
			if value == nil {
				out.FieldNulls[i]++
			}
		}
		row = stablePlanRow{Source: s.ID}
		if err := rows.Scan(&allowed, &start, &row.End, &row.Notebook, &row.Kind,
			&row.Property, &row.Target, &row.Anchor, &row.Display, &row.Key, &row.Alt,
			&row.Aliases, &row.Resolved, &row.Ambiguous, &row.Asset); err != nil {
			t.Fatal(err)
		}
		out.CanRetain = out.CanRetain && allowed
		if !start.Valid {
			out.Sentinels++
			if len(s.Old) != 0 || !allowed || row != (stablePlanRow{Source: s.ID}) {
				t.Fatalf("prepared empty sentinel=%+v allowed=%t", row, allowed)
			}
			continue
		}
		row.Start = start.Int32
		if out.OldRows >= len(s.Old) || !allowed || row != s.Old[out.OldRows] {
			t.Fatalf("prepared complete literal row %d mismatch: %+v allowed=%t", out.OldRows, row, allowed)
		}
		out.OldRows++
		if row.Resolved.Valid {
			seen[uuid.UUID(row.Resolved.Bytes)] = true
		}
	}
	out.ErrChecks++
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantSentinels := 0
	if s.N == 0 {
		wantSentinels = 1
	}
	if out.Rows != max(1, s.N) || out.OldRows != s.N || out.Sentinels != wantSentinels || out.EOFs != 1 {
		t.Fatalf("prepared complete ReadD rows/old/sentinel/EOF=%d/%d/%d/%d", out.Rows, out.OldRows, out.Sentinels, out.EOFs)
	}
	for id := range seen {
		out.Targets = append(out.Targets, id)
	}
	slices.SortFunc(out.Targets, uuid.UUID.Compare)
	return out
}

func preparedPlanKernel(t *testing.T, ctx context.Context, f *stablePlanFixture, s *stablePlanSpec, c stablePlanCall, mode string) {
	t.Helper()
	tx, _ := postgres.TxFrom(ctx)
	desc, err := tx.Conn().PgConn().Prepare(ctx, "", c.SQL, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantOIDs := []uint32{pgtype.UUIDOID, pgtype.UUIDOID, pgtype.Int4OID}
	if s == nil {
		wantOIDs = []uint32{pgtype.UUIDArrayOID, pgtype.TextArrayOID, pgtype.UUIDOID, pgtype.UUIDArrayOID, pgtype.UUIDArrayOID}
	}
	if len(c.Args) != len(wantOIDs) || !slices.Equal(desc.ParamOIDs, wantOIDs) {
		t.Fatalf("prepared exact captured parameter OIDs=%v want=%v", desc.ParamOIDs, wantOIDs)
	}
	// Inspect reuses its actual cached statement. Residual is natively
	// DescribeExec; its named custom/generic plan below is diagnostic only.
	name := c.PreparedName
	if s == nil {
		name = "prepared_source_residual_observation"
		defer func() {
			if err := tx.Conn().Deallocate(ctx, name); err != nil {
				t.Fatal(err)
			}
		}()
	}
	if name == "" {
		t.Fatal("prepared Inspect actual named identity missing")
	}
	if _, err := tx.Conn().Prepare(ctx, name, c.SQL); err != nil {
		t.Fatal(err)
	}
	execute := stablePlanExecute(t, name, c.Args)
	beforeChoice := stablePlanCounter(t, ctx, tx, c.SQL)
	work := stablePlanRelationWork(t, ctx, tx)
	var planned string
	if err := tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+execute).Scan(&planned); err != nil {
		t.Fatal(err)
	}
	planningWork := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	afterChoice := stablePlanCounter(t, ctx, tx, c.SQL)
	lintEvidence(t, "prepared-plan-only", map[string]any{"state": f.state, "mode": mode, "call": c,
		"plan": json.RawMessage(planned), "work": planningWork, "before_choice": beforeChoice,
		"after_choice": afterChoice, "parameter_oids": desc.ParamOIDs, "input_wire_formats": "NOT_OBSERVED"})
	stablePlanCountDelta(t, beforeChoice, afterChoice, mode)
	if s != nil {
		beforeChoice, work = stablePlanCounter(t, ctx, tx, c.SQL), stablePlanRelationWork(t, ctx, tx)
		d := preparedPlanReadD(t, ctx, tx, c, *s)
		delta := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
		afterChoice = stablePlanCounter(t, ctx, tx, c.SQL)
		lintEvidence(t, "prepared-ReadD-replay", map[string]any{"state": f.state, "mode": mode, "call": c,
			"D": d, "work": delta, "before_choice": beforeChoice, "after_choice": afterChoice})
		stablePlanCountDelta(t, beforeChoice, afterChoice, mode)
		if !d.CanRetain || d.ErrChecks != 1 || d.CloseCalls != 1 || !d.Closed || !slices.Equal(d.Targets, stablePlanWantD(*s)) {
			t.Fatalf("prepared complete native ReadD=%+v", d)
		}
		stablePlanCheckWork(t, *s, stablePlanEvidence{Name: stablePlanInspect, State: f.state, ExecutionWork: delta})
	}
	beforeChoice, work = stablePlanCounter(t, ctx, tx, c.SQL), stablePlanRelationWork(t, ctx, tx)
	var raw string
	if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+execute).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	delta := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	afterChoice = stablePlanCounter(t, ctx, tx, c.SQL)
	// Keep all raw node/buffer/filter/temp evidence before a work guard fails.
	lintEvidence(t, "prepared-physical-replay", map[string]any{"state": f.state, "mode": mode, "call": c,
		"plan": json.RawMessage(raw), "work": delta, "before_choice": beforeChoice, "after_choice": afterChoice,
		"scope": "exact captured SQL/args kernel; residual named replay is not native DescribeExec"})
	stablePlanCountDelta(t, beforeChoice, afterChoice, mode)
	root := stablePlanTree(t, json.RawMessage(raw))
	if s != nil {
		guard := stablePlanEvidence{Name: stablePlanInspect, State: f.state, ExecutionWork: delta}
		stablePlanCheckWork(t, *s, guard)
		stablePlanCheckTree(t, *s, guard, root)
		preparedPlanProducers(t, *s, root)
	} else {
		preparedPlanResidualWork(t, delta, f.state)
		if root.Rows != 3 || root.Loops != 1 {
			t.Fatalf("prepared residual result rows/loops=%v/%v", root.Rows, root.Loops)
		}
		for _, node := range root.all() {
			if !node.RowsObserved || !node.LoopsObserved || node.Loops > 1 {
				t.Fatalf("prepared residual row/loop observation=%+v", node)
			}
			if node.Relation != "" && node.Relation != "page_links" {
				t.Fatalf("prepared residual unexpected relation=%+v", node)
			}
			if node.Relation == "page_links" && node.Rows+node.Removed+node.Rechecked > 3 || node.JoinRemoved != 0 {
				t.Fatalf("prepared residual visited/filtered more than three literal rows: %+v", node)
			}
		}
	}
	// This small fixture has no TOAST values and at most 4096 narrow output
	// rows. As in the existing old-F observer, no temp spill is permitted.
	var document []struct {
		Plan struct {
			Read    int64 `json:"Temp Read Blocks"`
			Written int64 `json:"Temp Written Blocks"`
		}
	}
	if err := json.Unmarshal([]byte(raw), &document); err != nil || len(document) != 1 {
		t.Fatalf("prepared temp plan: %v", err)
	}
	if document[0].Plan.Read != 0 || document[0].Plan.Written != 0 {
		t.Fatalf("prepared small physical plan spilled: %+v", document[0].Plan)
	}
}

func preparedPlanProducers(t *testing.T, s stablePlanSpec, root stablePlanNode) {
	t.Helper()
	want := map[string]int{"old_head": 1, "old_tags": len(s.OldFacts.Tags),
		"old_properties": len(s.OldFacts.Properties), "old_aliases": len(s.OldFacts.Aliases)}
	seen := map[string]bool{}
	for _, node := range root.all() {
		name, producer := strings.CutPrefix(node.Subplan, "CTE ")
		if rows, ok := want[name]; ok && producer {
			if seen[name] || node.Rows != float64(rows) || node.Loops != 1 {
				t.Fatalf("PREPARED_SIDE_ONCE: producer %s=%+v want=%d/1", name, node, rows)
			}
			seen[name] = true
		}
		if rows, ok := want[node.CTE]; ok && (node.Loops > 1 || node.Rows+node.Removed > float64(rows)) {
			t.Fatalf("PREPARED_SIDE_ONCE: repeated side CTE=%+v bound=%d", node, rows)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("PREPARED_SIDE_ONCE: producers=%v want=%v", seen, want)
	}
}

func preparedPlanResidualWork(t *testing.T, work map[string]stablePlanWork, state string) {
	t.Helper()
	// Exactly three disjoint global arms each select one resolved row. The
	// real rebuild creates at most old+insert+resolution-update versions.
	s := stablePlanSpec{N: 3, Old: make([]stablePlanRow, 3)}
	for i := range s.Old {
		s.Old[i].Resolved.Valid = true
	}
	stablePlanCheckWork(t, s, stablePlanEvidence{Name: stablePlanInspect, State: state, ExecutionWork: work})
}
