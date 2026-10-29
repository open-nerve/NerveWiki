package postgresadapter_test

import (
	"testing"
	"time"
	"uuid"
)

// searchRead is a page of a search: its query, the notebook it is limited
// to, if any, the page (1 or 3), whether it answers building, and its
// median of eight at each stage: without statistics, analyzed, planned
// for any arguments. The reference is the median of three runs on Linux
// ARM64 (Go 1.27.1, Docker 29.9.2, source 9e38a38f; M10 calibration).
// Each logged median was rounded to microseconds before the three-run
// median was rounded to 0.1 ms. Instantaneous load was not recorded;
// M8/P5 3.9 preserves the former M-series Mac reference.
type searchRead struct {
	q        string
	notebook *uuid.UUID
	page     int
	building bool
	medians  [3]time.Duration
}

// msOf are milliseconds.
func msOf(n ...float64) [3]time.Duration {
	return [3]time.Duration{time.Duration(n[0] * float64(time.Millisecond)), time.Duration(n[1] * float64(time.Millisecond)),
		time.Duration(n[2] * float64(time.Millisecond))}
}

// searchFloor is the least limit of a read (M8/P5A review C-M2). It dates
// to the former Mac reference, whose 2 to 20 ms reads varied by up to
// twice between runs. The trap the guard exists for, the scoped join
// without statistics filtering the candidates in a scan of nodes, took
// 97 to 143 ms at this size for "电 #", "the" and "的"; those reads then
// took 4 to 7 ms normally. Their recalibrated limits without statistics
// remain 40 ms, well under that historical trap.
const searchFloor = 40 * time.Millisecond

// searchTrap is the least time the trap took.
const searchTrap = 97 * time.Millisecond

// searchLimit is a read's limit: three times its median, searchFloor at
// least, the rule of 8c83ccb7 with a floor.
func searchLimit(median time.Duration) time.Duration {
	return max(3*median, searchFloor)
}

// searchReads are the guard's reads, eng the notebook of the most.
func searchReads(eng *uuid.UUID) []searchRead {
	// On this Linux ARM64 reference, "中国 database" and "theirs" have
	// medians of 34.4 to 42.7 ms, and eight common terms 33.8 to 34.5 ms.
	// The other reads range from 4.2 to 28.3 ms. These are the complete
	// 60-cell remeasurement, without scaling the former Mac table by a
	// common ratio; its historical measurements remain in M8/P5 3.9.
	return []searchRead{
		{q: "中国", notebook: eng, page: 1, medians: msOf(19.3, 19.4, 22.1)},
		{q: "中国", notebook: eng, page: 3, medians: msOf(14.4, 14.8, 16.6)},
		{q: "的是", notebook: eng, page: 1, medians: msOf(28.0, 25.8, 28.3)},
		{q: "的是", notebook: eng, page: 3, medians: msOf(16.4, 14.1, 15.1)},
		{q: "latency", notebook: eng, page: 1, medians: msOf(17.8, 16.9, 18.9)},
		{q: "latency", notebook: eng, page: 3, medians: msOf(10.4, 10.2, 11.9)},
		{q: "the", notebook: eng, page: 1, medians: msOf(10.5, 10.0, 9.8)},
		{q: "the", notebook: eng, page: 3, medians: msOf(7.7, 7.7, 7.5)},
		{q: "go", notebook: eng, page: 1, medians: msOf(20.1, 20.2, 20.9)},
		{q: "go", notebook: eng, page: 3, medians: msOf(14.4, 14.7, 15.9)},
		{q: "的", notebook: eng, page: 1, medians: msOf(10.0, 9.8, 9.4)},
		{q: "的", notebook: eng, page: 3, medians: msOf(7.3, 7.5, 7.3)},
		{q: "电", notebook: eng, page: 1, medians: msOf(4.2, 4.5, 4.5)},
		{q: "电 #", notebook: eng, page: 1, medians: msOf(6.4, 6.9, 8.9)},
		{q: "中国 database", notebook: eng, page: 1, medians: msOf(39.4, 40.4, 42.7)},
		{q: "theirs", notebook: eng, page: 1, medians: msOf(40.9, 40.8, 42.2)},
		{q: "theirs", notebook: eng, page: 3, medians: msOf(34.6, 34.4, 35.8)},
		{q: "the of and to in is that for", notebook: eng, page: 1, medians: msOf(33.8, 33.8, 34.5)},
		{q: "中国", page: 1, building: true, medians: msOf(22.1, 21.0, 23.3)},
		{q: "中国", page: 3, building: true, medians: msOf(15.2, 15.2, 16.6)},
	}
}

// The guard's limits keep its trap caught (M8/P5A review C-M2): each at
// least searchFloor, those of the reads the trap showed in, without
// statistics, under the time it took them.
func TestTheSearchGuardsLimitsKeepItsTrapCaught(t *testing.T) {
	for _, r := range searchReads(&uuid.UUID{}) {
		for k, median := range r.medians {
			limit := searchLimit(median)
			if limit < searchFloor || limit < 3*median {
				t.Errorf("%q page %d, stage %d: limit %s of the median %s, want three times it, %s at least", r.q, r.page, k, limit, median,
					searchFloor)
			}
			if trapped := k == 0 && (r.q == "电 #" || r.q == "the" || r.q == "的"); trapped && limit >= searchTrap {
				t.Errorf("%q page %d without statistics: limit %s, want under the trap's %s", r.q, r.page, limit, searchTrap)
			}
		}
	}
}
