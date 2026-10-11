//go:build !race

package domain_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// Pages whose names cut alike are renamed each in time as long as the
// others: four times as many take about four times the allocations, as
// the whole plan does, not sixteen (Codex M7 review R3). The race detector
// changes the allocations: make test-go runs it without.
func TestPagesWhoseNamesCutAlikeAreRenamedInTimeAsLongAsThey(t *testing.T) {
	allocs := func(n int) float64 {
		nodes := alike(n)
		return testing.AllocsPerRun(1, func() {
			if _, err := domain.NewPlan(notebook(), nil, nodes, nothingLinked); err != nil {
				t.Fatal(err)
			}
		})
	}
	small, large := allocs(250), allocs(1000)
	if large > 6*small {
		t.Errorf("1000 pages take %.0f allocations, 250 %.0f: more than 6 times", large, small)
	}
}
