//go:build !race

package app_test

import "testing"

// Names numbered whose names cut alike are numbered each in time as long
// as the others: four times as many take about four times the
// allocations, as the whole unit does, not sixteen (Codex M7 review R3).
// The race detector changes the allocations: make test-go runs it without.
func TestAnImportsUnitNumbersNamesThatCutAlikeInTimeAsLongAsThey(t *testing.T) {
	allocs := func(n int) float64 {
		return testing.AllocsPerRun(1, func() { importAlike(t, heldAlike(n), n) })
	}
	small, large := allocs(250), allocs(1000)
	if large > 6*small {
		t.Errorf("1000 names take %.0f allocations, 250 %.0f: more than 6 times", large, small)
	}
}
