package app_test

import (
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// These cuts are handwritten from the public weights and Joined markers,
// independently of the production batch helper or the runner's results.
func nominalBatchCases() []struct {
	name  string
	batch app.Batch
	steps []app.JobStep
	want  [][2]int
} {
	return []struct {
		name  string
		batch app.Batch
		steps []app.JobStep
		want  [][2]int
	}{
		{"empty", app.Batch{}, nil, nil},
		{"default nodes", app.Batch{}, []app.JobStep{{Nodes: 60}, {Nodes: 40}, {Nodes: 1}},
			[][2]int{{0, 2}, {2, 3}}},
		{"default bytes", app.Batch{}, []app.JobStep{{Nodes: 1, Bytes: 5 << 20}, {Nodes: 1, Bytes: 3 << 20}, {Nodes: 1, Bytes: 1}},
			[][2]int{{0, 2}, {2, 3}}},
		{"node weights and Joined; links do not cut", app.Batch{Nodes: 5, Bytes: 1000, Links: 1},
			[]app.JobStep{{Nodes: 2, Bytes: 7}, {Nodes: 2, Bytes: 11, Joined: true}, {Nodes: 1, Bytes: 13},
				{Nodes: 1, Bytes: 17}, {Nodes: 4, Bytes: 19}}, [][2]int{{0, 3}, {3, 5}}},
		{"bytes and an overweight Joined segment", app.Batch{Nodes: 100, Bytes: 10},
			[]app.JobStep{{Nodes: 1, Bytes: 4}, {Nodes: 1, Bytes: 7, Joined: true}, {Nodes: 1, Bytes: 8},
				{Nodes: 1, Bytes: 3}, {Nodes: 1, Bytes: 7}}, [][2]int{{0, 1}, {1, 3}, {3, 5}}},
		{"restore F A content A B; one batch", app.Batch{Nodes: 4},
			[]app.JobStep{{Nodes: 1}, {Nodes: 1, Joined: true}, {Nodes: 1}, {Nodes: 1}}, [][2]int{{0, 4}}},
		{"restore F A content A B; continuation", app.Batch{Nodes: 3},
			[]app.JobStep{{Nodes: 1}, {Nodes: 1, Joined: true}, {Nodes: 1}, {Nodes: 1}}, [][2]int{{0, 3}, {3, 4}}},
		{"first Joined segment exceeds nodes", app.Batch{Nodes: 1},
			[]app.JobStep{{Nodes: 1}, {Nodes: 1, Joined: true}, {Nodes: 1}, {Nodes: 1}}, [][2]int{{0, 1}, {1, 3}, {3, 4}}},
	}
}

func TestARevertStartReceivesNominalBatches(t *testing.T) {
	for _, tt := range nominalBatchCases() {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			q, tree := &queue{}, newJobTree(w.rec)
			probe := &jobBatchProbe{steps: tt.steps}
			tree.checkBatches = probe
			if _, err := start(w, w.startsWithBatch(q, tree, 20, tt.batch), domain.KindRevert); err != nil {
				t.Fatal(err)
			}
			if !probe.present || probe.calls != 1 || !slices.Equal(probe.cuts, tt.want) {
				t.Errorf("CheckRevert callback present=%t, calls=%d, cuts=%v; want present, one call, %v",
					probe.present, probe.calls, probe.cuts, tt.want)
			}
		})
	}
}

func TestARevertWorkerReceivesNominalBatches(t *testing.T) {
	for _, tt := range nominalBatchCases() {
		t.Run(tt.name, func(t *testing.T) {
			w := newBackgroundWorld()
			w.batch, w.tree.steps = tt.batch, tt.steps
			probe := &jobBatchProbe{steps: tt.steps}
			w.tree.planBatches = probe
			if got := w.run(t, w.queued(domain.KindRevert)); got.State != domain.StateSucceeded {
				t.Fatalf("job %s, report %+v; want succeeded", got.State, got.Report)
			}
			if !probe.present || probe.calls != 1 || !slices.Equal(probe.cuts, tt.want) {
				t.Errorf("PlanRevert callback present=%t, calls=%d, cuts=%v; want present, one call, %v",
					probe.present, probe.calls, probe.cuts, tt.want)
			}
		})
	}
}

// The same handwritten oracle also checks the existing runtime entry
// before the greedy loop is extracted; it may already pass at baseline.
func TestABackgroundBatchesMatchHandwrittenCuts(t *testing.T) {
	for _, tt := range nominalBatchCases() {
		t.Run(tt.name, func(t *testing.T) {
			w := newBackgroundWorld()
			w.batch, w.tree.steps = tt.batch, tt.steps
			if got := w.run(t, w.queued(domain.KindRevert)); got.State != domain.StateSucceeded {
				t.Fatalf("job %s, report %+v; want succeeded", got.State, got.Report)
			}
			if got := w.tree.written(); !slices.Equal(got, tt.want) {
				t.Errorf("runtime cuts %v, want %v", got, tt.want)
			}
		})
	}
}

// A fake unit refusal exercises the runner's response, not a real page
// history bound: halve by step index, then greedily cut from the new from.
func TestABackgroundRevertRegreediesAfterJoinedHalving(t *testing.T) {
	w := newBackgroundWorld()
	w.batch = app.Batch{Nodes: 4}
	w.tree.steps = []app.JobStep{{Nodes: 1}, {Nodes: 1, Joined: true}, {Nodes: 1}, {Nodes: 1}}
	w.tree.busy, w.tree.halving = 1, 2
	if got := w.run(t, w.queued(domain.KindRevert)); got.State != domain.StateSucceeded || got.Progress.Done != 4 {
		t.Fatalf("job %s, progress %+v, report %+v; want succeeded", got.State, got.Progress, got.Report)
	}
	want := [][2]int{{0, 4}, {0, 4}, {0, 1}, {1, 4}, {1, 3}, {3, 4}}
	if got := w.tree.written(); !slices.Equal(got, want) {
		t.Errorf("runtime attempts %v, want %v", got, want)
	}
	if got, want := w.tree.succeeded(), [][2]int{{0, 1}, {1, 3}, {3, 4}}; !slices.Equal(got, want) {
		t.Errorf("committed cuts %v, want %v", got, want)
	}
}
