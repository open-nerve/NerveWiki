//go:build !race

package domain_test

import (
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// Pages whose names cut alike are renamed each in time as long as the
// others: four times as many take about four times the allocations, as
// the whole plan does, not sixteen (Codex M7 review R3). The race detector
// changes the allocations: make test-go runs it without.
func TestPagesWhoseNamesCutAlikeAreRenamedInTimeAsLongAsThey(t *testing.T) {
	allocs := func(n int) float64 {
		// Each spelled its own way, alike by their keys: "ß" or "SS", two bytes either, by each of its number's bits
		// (Codex M7 review fix checks B-M2, B2-N1).
		nodes := alike(n)
		for i := range nodes {
			var spelled strings.Builder
			for b := range 10 {
				spelled.WriteString(map[bool]string{true: "ß", false: "SS"}[i>>b&1 == 1])
			}
			nodes[i].Name = spelled.String() + nodes[i].Name[spelled.Len():]
		}
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
