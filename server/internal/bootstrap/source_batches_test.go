package bootstrap

import (
	"reflect"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

func TestSourceBatchAdapterPreservesNilWeightsAndCuts(t *testing.T) {
	if pageJobBatches(nil) != nil {
		t.Fatal("nil callback was wrapped")
	}
	steps := []page.JobStep{{Nodes: 2, Bytes: 9, Joined: true}, {Nodes: 1, Bytes: 5}, {Nodes: 3, Bytes: 1}}
	want := []transfer.JobStep{{Nodes: 2, Bytes: 9, Joined: true}, {Nodes: 1, Bytes: 5}, {Nodes: 3, Bytes: 1}}
	cuts := [][2]int{{0, 2}, {2, 3}}
	calls := 0
	got := pageJobBatches(func(actual []transfer.JobStep) [][2]int {
		calls++
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("weights=%v, want %v", actual, want)
		}
		return cuts
	})(steps)
	if calls != 1 || !reflect.DeepEqual(got, cuts) || &got[0] != &cuts[0] {
		t.Fatalf("calls=%d cuts=%v, want same callback result %v", calls, got, cuts)
	}
}
