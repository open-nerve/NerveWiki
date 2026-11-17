// Package stats holds perf's statistics (M12/P4 design 3.4.4, 3.5.2): a
// quantile by the nearest rank, the definition BenchmarkSearch's
// nearestRank takes (bootstrap/search_bench_test.go), in whole per mille so
// that a rank is integer arithmetic, and two rounds' worse.
package stats

import (
	"fmt"
	"slices"
	"time"
)

// Quantile is the perMille-th per mille of xs by the nearest rank: the
// ceil(perMille*n/1000)-th smallest, the smallest for 0; 0 for none. xs is
// not changed. perMille is 0 to 1000.
func Quantile(xs []time.Duration, perMille int) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	if perMille < 0 || perMille > 1000 {
		panic(fmt.Sprintf("stats: a quantile of %d per mille", perMille))
	}
	s := slices.Sorted(slices.Values(xs))
	rank := (perMille*len(s) + 999) / 1000
	return s[max(0, rank-1)]
}

// Summary is a round's latencies: how many, the median and the 99th
// percentile by the nearest rank, and the largest.
type Summary struct {
	N             int
	P50, P99, Max time.Duration
}

// Of is xs's summary.
func Of(xs []time.Duration) Summary {
	return Summary{N: len(xs), P50: Quantile(xs, 500), P99: Quantile(xs, 990), Max: Quantile(xs, 1000)}
}

// Worse is the worse of two rounds, the one a budget is judged by (design
// 3.5.2): the larger 99th percentile, then the larger median, then a.
func Worse(a, b Summary) Summary {
	if b.P99 > a.P99 || b.P99 == a.P99 && b.P50 > a.P50 {
		return b
	}
	return a
}

// String is the summary in milliseconds.
func (s Summary) String() string {
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	return fmt.Sprintf("n=%d p50=%.1fms p99=%.1fms max=%.1fms", s.N, ms(s.P50), ms(s.P99), ms(s.Max))
}
