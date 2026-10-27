package app

import (
	"reflect"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestSourceRevertActionsFollowTheActualBatchGroups(t *testing.T) {
	r := func(g int32) revertStep { return revertStep{kind: domain.RevertRestore, group: g} }
	w := revertStep{kind: domain.RevertContent, group: -1}
	for _, tt := range []struct {
		name  string
		steps []revertStep
		want  []revertAction
	}{
		{"same_group", []revertStep{r(0), r(0), w, r(0)}, []revertAction{{index: 0, members: []int{0, 1, 3}}, {index: 2}}},
		{"first_cut", []revertStep{r(0), r(0), w}, []revertAction{{index: 0, members: []int{0, 1}}, {index: 2}}},
		{"continuation", []revertStep{r(0)}, []revertAction{{index: 0, members: []int{0}}}},
		{"independent_groups", []revertStep{r(0), w, r(1)}, []revertAction{{index: 0, members: []int{0}}, {index: 1}, {index: 2, members: []int{2}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := revertActions(tt.steps); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("actions=%+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSourceBatchCutsRejectGapsEmptyRangesAndJoinedSplits(t *testing.T) {
	steps := []domain.JobStep{{Nodes: 1, Joined: true}, {Nodes: 1}, {Nodes: 1}}
	for _, cuts := range [][][2]int{{{0, 1}, {1, 3}}, {{0, 2}}, {{0, 2}, {3, 3}}, {{0, 0}, {0, 3}}, {{0, 2}, {1, 3}}, {{0, 4}}} {
		if _, err := checkedBatches(steps, func([]domain.JobStep) [][2]int { return cuts }); err == nil {
			t.Fatalf("accepted malformed cuts %v", cuts)
		}
	}
	if _, err := checkedBatches(steps, func([]domain.JobStep) [][2]int { return [][2]int{{0, 2}, {2, 3}} }); err != nil {
		t.Fatal(err)
	}
	if _, err := checkedBatches(nil, func([]domain.JobStep) [][2]int { return nil }); err != nil {
		t.Fatal(err)
	}
}
