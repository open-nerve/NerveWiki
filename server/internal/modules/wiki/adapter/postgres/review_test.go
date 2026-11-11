package postgresadapter_test

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// A review's rows (M11/P3 design 3.5), each conjunct and disjunct of
// their two statements refused by a row that meets every other: Approve
// sets an upgrade approved, only its submission's, in review with its
// report, alive; its submission's three columns and its report kept, who
// and when the approver's. SendBack sets one upgrading again, its
// submission and report cleared: in review with a report, or without one
// submitted at takenOver or before (ReviewOf's boundary); none upgrading,
// rolling back or deleted. A refusal changes no column.
func TestTheReviewsRows(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	at := time.Date(2026, 11, 12, 9, 0, 0, 123000, time.UTC)
	must := func(t *testing.T, what string, ok bool, err error, want bool) {
		t.Helper()
		if err != nil || ok != want {
			t.Fatalf("%s = %v, %v; want %v", what, ok, err, want)
		}
	}
	report := domain.UpgradeReport{ComputedAt: at, Passed: true, Rules: []domain.ReportRule{}}
	// reviewing is an upgrade of notebook in review, submitted by bob at
	// submitted, with a report when reported; its submission's id.
	reviewing := func(t *testing.T, notebook uuid.UUID, submitted time.Time, reported bool) (uuid.UUID, uuid.UUID) {
		t.Helper()
		u := started(notebook, f.alice, at)
		if err := f.store.Insert(ctx, u, f.alice, at); err != nil {
			t.Fatal(err)
		}
		submission := uuid.NewV7()
		ok, err := f.store.Submit(ctx, u.ID, submission, f.bob, submitted, submitted)
		must(t, "Submit", ok, err, true)
		if reported {
			ok, err = f.store.KeepReport(ctx, u.ID, submission, report, f.bob, submitted)
			must(t, "KeepReport", ok, err, true)
		}
		return u.ID, submission
	}
	// moment is at as to_jsonb writes it.
	moment := func(t *testing.T, at time.Time) string {
		t.Helper()
		var raw []byte
		if err := f.pool.QueryRow(ctx, "SELECT to_jsonb($1::timestamptz)", at).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	exec := func(t *testing.T, sql string, args ...any) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// refused runs act, which must answer false, and fails t unless the
	// upgrade id's row is as it was.
	refused := func(t *testing.T, what string, id uuid.UUID, act func() (bool, error)) {
		t.Helper()
		before := reviewRow(t, f, id)
		ok, err := act()
		must(t, what, ok, err, false)
		if after := reviewRow(t, f, id); !maps.Equal(after, before) {
			t.Errorf("%s changed the row from %v to %v", what, before, after)
		}
	}
	// changed checks the upgrade id's row after act, which must answer
	// true, against before: the columns want named set to their values,
	// every other as it was.
	changed := func(t *testing.T, what string, id uuid.UUID, act func() (bool, error), want map[string]string) {
		t.Helper()
		before := reviewRow(t, f, id)
		ok, err := act()
		must(t, what, ok, err, true)
		after := reviewRow(t, f, id)
		for _, column := range slices.Sorted(maps.Keys(after)) {
			value, set := want[column]
			if !set {
				value = before[column]
			}
			if after[column] != value {
				t.Errorf("%s: %s = %s, want %s", what, column, after[column], value)
			}
		}
	}

	t.Run("Approve", func(t *testing.T) {
		id, submission := reviewing(t, f.eng, at, true)
		approve := func(id, submission uuid.UUID) func() (bool, error) {
			return func() (bool, error) { return f.store.Approve(ctx, id, submission, f.alice, at.Add(time.Minute)) }
		}
		refused(t, "Approve(another upgrade)", id, approve(uuid.NewV7(), submission))
		refused(t, "Approve(another submission)", id, approve(id, uuid.NewV7()))
		exec(t, "UPDATE notebook_upgrades SET deleted_at = $2 WHERE id = $1", id, at)
		refused(t, "Approve(deleted)", id, approve(id, submission))
		exec(t, "UPDATE notebook_upgrades SET deleted_at = NULL WHERE id = $1", id)
		changed(t, "Approve", id, approve(id, submission), map[string]string{"state": `"approved"`,
			"updated_by_id": `"` + f.alice.String() + `"`, "updated_at": moment(t, at.Add(time.Minute))})
		refused(t, "Approve(approved)", id, approve(id, submission))

		other, _ := reviewing(t, f.ops, at, false)
		submissionOf := func() uuid.UUID {
			var s uuid.UUID
			if err := f.pool.QueryRow(ctx, "SELECT submission_id FROM notebook_upgrades WHERE id = $1", other).Scan(&s); err != nil {
				t.Fatal(err)
			}
			return s
		}
		refused(t, "Approve(no report)", other, approve(other, submissionOf()))
		ok, err := f.store.SendBack(ctx, other, f.bob, at, at)
		must(t, "SendBack(the one without a report)", ok, err, true)
		refused(t, "Approve(upgrading)", other, approve(other, uuid.NewV7()))
		exec(t, "UPDATE notebook_upgrades SET deleted_at = $2 WHERE id = $1", other, at)
	})

	t.Run("SendBack", func(t *testing.T) {
		submitted := at.Add(time.Hour)
		id, _ := reviewing(t, f.ops, submitted, false)
		back := func(id uuid.UUID, takenOver time.Time) func() (bool, error) {
			return func() (bool, error) { return f.store.SendBack(ctx, id, f.bob, submitted.Add(time.Minute), takenOver) }
		}
		cleared := map[string]string{"state": `"upgrading"`, "submission_id": "null", "submitted_by_id": "null",
			"submitted_at": "null", "report": "null", "updated_by_id": `"` + f.bob.String() + `"`,
			"updated_at": moment(t, submitted.Add(time.Minute))}
		refused(t, "SendBack(no report, submitted a microsecond after takenOver)", id, back(id, submitted.Add(-time.Microsecond)))
		changed(t, "SendBack(no report, submitted at takenOver)", id, back(id, submitted), cleared)
		refused(t, "SendBack(upgrading)", id, back(id, submitted.Add(time.Hour)))
		exec(t, "UPDATE notebook_upgrades SET deleted_at = $2 WHERE id = $1", id, at)

		id, _ = reviewing(t, f.ops, submitted, true)
		refused(t, "SendBack(another upgrade)", id, back(uuid.NewV7(), submitted.Add(-time.Hour)))
		exec(t, "UPDATE notebook_upgrades SET deleted_at = $2 WHERE id = $1", id, at)
		refused(t, "SendBack(deleted)", id, back(id, submitted.Add(-time.Hour)))
		exec(t, "UPDATE notebook_upgrades SET deleted_at = NULL WHERE id = $1", id)
		exec(t, "UPDATE notebook_upgrades SET state = 'rolling_back', ending = 'rejected', rollback_job_id = $2 WHERE id = $1", id,
			uuid.NewV7())
		refused(t, "SendBack(rolling back)", id, back(id, submitted.Add(-time.Hour)))
		exec(t, "UPDATE notebook_upgrades SET state = 'reviewing', ending = NULL, rollback_job_id = NULL WHERE id = $1", id)
		changed(t, "SendBack(a report, submitted after takenOver)", id, back(id, submitted.Add(-time.Hour)), cleared)
	})
}

// reviewRow is the upgrade id's row, each column's value as JSON writes
// it.
func reviewRow(t *testing.T, f fixture, id uuid.UUID) map[string]string {
	t.Helper()
	var raw []byte
	if err := f.pool.QueryRow(context.Background(), "SELECT to_jsonb(u) FROM notebook_upgrades u WHERE id = $1", id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var columns map[string]json.RawMessage
	if err := json.Unmarshal(raw, &columns); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string, len(columns))
	for k, v := range columns {
		out[k] = string(v)
	}
	return out
}
