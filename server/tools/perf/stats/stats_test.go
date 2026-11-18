package stats

import (
	"testing"
	"time"
)

// nearestRank is BenchmarkSearch's (bootstrap/search_bench_test.go), copied:
// the quantile this package's must equal wherever p*n is not within its
// epsilon of an integer from above.
func nearestRank(sorted []time.Duration, p float64) time.Duration {
	i := int(p*float64(len(sorted))+0.999999) - 1
	return sorted[max(0, min(len(sorted)-1, i))]
}

// The nearest rank: a table written out, one sample and two, and the
// definition BenchmarkSearch's nearestRank takes (design 3.4.4, 7.2's 31).
func TestQuantileIsTheNearestRank(t *testing.T) {
	ms := func(ns ...int) []time.Duration {
		out := make([]time.Duration, len(ns))
		for i, n := range ns {
			out[i] = time.Duration(n) * time.Millisecond
		}
		return out
	}
	hundred := make([]time.Duration, 100)
	for i := range hundred {
		hundred[i] = time.Duration(100-i) * time.Millisecond // 100 down to 1
	}
	twenty := ms(9, 3, 14, 1, 20, 7, 12, 5, 18, 2, 16, 11, 4, 19, 8, 13, 6, 17, 10, 15)
	for _, c := range []struct {
		name     string
		xs       []time.Duration
		perMille int
		want     time.Duration
	}{
		{"none", nil, 500, 0},
		{"one, its median", ms(7), 500, 7 * time.Millisecond},
		{"one, its 99th", ms(7), 990, 7 * time.Millisecond},
		{"one, its 0th", ms(7), 0, 7 * time.Millisecond},
		{"two, the median the smaller", ms(9, 4), 500, 4 * time.Millisecond},
		{"two, the 99th the larger", ms(9, 4), 990, 9 * time.Millisecond},
		{"two, the 50.1st the larger", ms(9, 4), 501, 9 * time.Millisecond},
		{"three, the median the middle", ms(30, 10, 20), 500, 20 * time.Millisecond},
		{"a hundred, the median the 50th", hundred, 500, 50 * time.Millisecond},
		{"a hundred, the 99th the 99th", hundred, 990, 99 * time.Millisecond},
		{"a hundred, the 99.1st the largest", hundred, 991, 100 * time.Millisecond},
		{"a hundred, the largest", hundred, 1000, 100 * time.Millisecond},
		{"twenty, the median the 10th", twenty, 500, 10 * time.Millisecond},
		{"twenty, the 99th the largest", twenty, 990, 20 * time.Millisecond},
		{"twenty, the 95th the 19th", twenty, 950, 19 * time.Millisecond},
	} {
		if got := Quantile(c.xs, c.perMille); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if twenty[0] != 9*time.Millisecond {
		t.Error("Quantile sorted its argument")
	}
	for n := 1; n <= 120; n++ {
		xs := make([]time.Duration, n)
		for i := range xs {
			xs[i] = time.Duration(i+1) * time.Millisecond
		}
		for _, p := range []int{0, 1, 250, 500, 750, 900, 950, 990, 999, 1000} {
			if got, want := Quantile(xs, p), nearestRank(xs, float64(p)/1000); got != want {
				t.Errorf("n=%d, %d per mille: %v, nearestRank's %v", n, p, got, want)
			}
		}
	}
}

// Of summarizes; the worse round is the larger 99th percentile, then the
// larger median.
func TestTheWorseRoundIsTheLargerTail(t *testing.T) {
	ms := time.Millisecond
	s := Of([]time.Duration{5 * ms, 1 * ms, 3 * ms, 2 * ms, 4 * ms})
	if s != (Summary{N: 5, P50: 3 * ms, P99: 5 * ms, Max: 5 * ms}) {
		t.Errorf("Of = %+v", s)
	}
	a, b := Summary{N: 20, P50: 10 * ms, P99: 90 * ms}, Summary{N: 20, P50: 30 * ms, P99: 80 * ms}
	if Worse(a, b) != a || Worse(b, a) != a {
		t.Error("the worse round is not the one of the larger 99th percentile")
	}
	c := Summary{N: 20, P50: 40 * ms, P99: 90 * ms}
	if Worse(a, c) != c || Worse(c, a) != c {
		t.Error("of equal tails, the worse round is not the one of the larger median")
	}
}
