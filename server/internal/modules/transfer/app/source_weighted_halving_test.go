package app_test

import (
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// The page test TestSourceJobWeightedHalvingChangesDuplicateOutcome builds
// these weights from an actual nine-node deletion and a connected restore
// group, then executes both handwritten paths through real JobWrites. Here
// the lower history/refusal port is fake; Background.Run must use the real
// step-index halver, retry 503 at the same cut and re-greedy each new from.
func TestSourceBackgroundHalvesByStepIndexBeforeDuplicate(t *testing.T) {
	w := newBackgroundWorld()
	w.batch = app.Batch{Nodes: 17}
	w.tree.steps = []app.JobStep{{Nodes: 9}, {Nodes: 1}, {Nodes: 1, Bytes: 1, Joined: true},
		{Nodes: 1, Bytes: 1}, {Nodes: 1, Bytes: 1}, {Nodes: 1}, {Nodes: 1}, {Nodes: 1}, {Nodes: 1}}
	w.tree.history = &jobHistory{rows: map[int]int{1: 2, 2: 3, 4: 2, 5: 2, 6: 2, 7: 2, 8: 2},
		node: map[int]int{3: 2}, bound: 7}
	w.tree.planBatches = &jobBatchProbe{steps: slices.Clone(w.tree.steps)}
	w.tree.busy = 1
	a, b := uuid.NewV7(), uuid.NewV7()
	w.tree.fail[4] = &app.Refused{Code: "page.source_duplicate", NodeID: b, DuplicateOf: &a,
		Err: errors.New("B duplicates the A committed by the preceding prefix")}
	got := w.run(t, w.queued(domain.KindRevert))
	want := [][2]int{{0, 9}, {0, 9}, {0, 4}, {4, 9}, {4, 6}}
	if cuts := w.tree.written(); !slices.Equal(cuts, want) {
		t.Errorf("actual worker cuts=%v, want step-index cuts %v", cuts, want)
	}
	if committed := w.tree.succeeded(); !slices.Equal(committed, [][2]int{{0, 4}}) {
		t.Errorf("committed cuts=%v, want only N/F/A prefix [0,4)", committed)
	}
	if !w.tree.planBatches.present || w.tree.planBatches.calls != 1 ||
		!slices.Equal(w.tree.planBatches.cuts, [][2]int{{0, 9}}) {
		t.Errorf("nominal plan=%+v, want one unsplit [0,9) callback", w.tree.planBatches)
	}
	if got.State != domain.StateFailed || got.Report == nil || got.Report.Failure != domain.FailureRefused ||
		got.Progress != (domain.Progress{Done: 12, Total: 17}) {
		t.Fatalf("job=%s progress=%+v report=%+v, want refused with twelve committed node operations", got.State, got.Progress, got.Report)
	}
	r := got.Report.Refusal
	if r == nil || r.Code != "page.source_duplicate" || r.NodeID != b || r.DuplicateOf == nil || *r.DuplicateOf != a {
		t.Fatalf("refusal=%+v, want B's duplicate target A", r)
	}
}
