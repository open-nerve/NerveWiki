package postgresadapter_test

import (
	"testing"
	"time"
	"uuid"
)

// searchRead is a page of a search: its query, the notebook it is limited
// to, if any, the page (1 or 3), whether it answers building, and its
// median of eight on an M-series Mac at each stage: without statistics,
// analyzed, planned for any arguments (M8/P5A fix round 1: the median of
// three runs' medians, at a load of 5 to 7).
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

// searchFloor is the least limit of a read (M8/P5A review C-M2). Three
// times this machine's median leaves no room for the reads of 2 to 20 ms:
// their round trips' fixed costs swing them up to twice between runs
// here, and CI's runners are about twice as slow. The trap the guard
// exists for, the scoped join without statistics filtering the candidates
// in a scan of nodes, took 97 to 143 ms at this size for "电 #", "the" and
// "的", which take 4 to 7 ms: the floor stays well under it.
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
	// The slowest reads take 34 to 61 ms, most of it their body tier's: for
	// eight common terms and "的是" nearly every page is a candidate, joined
	// to the notebook's nodes and to its content, and sorted for the walk;
	// "theirs" and "中国 database" walk their bound of 500 pages, reading
	// and checking each content, read as its NFC (M8/P5A review A-M4); a
	// first page whose walk stopped at its first passes samples on for the
	// count (C-M1). The rest take 3 to 26 ms. Without statistics the scoped
	// join hashes the candidates with the notebook's nodes, about as fast
	// as analyzed; planned for any arguments, its hits taken for 100,000, a
	// read takes up to 1.8 times as long.
	return []searchRead{
		{q: "中国", notebook: eng, page: 1, medians: msOf(16, 16.1, 19.2)},
		{q: "中国", notebook: eng, page: 3, medians: msOf(12.1, 12.4, 13.6)},
		{q: "的是", notebook: eng, page: 1, medians: msOf(20.9, 21.1, 23)},
		{q: "的是", notebook: eng, page: 3, medians: msOf(11.9, 11.8, 12)},
		{q: "latency", notebook: eng, page: 1, medians: msOf(21.4, 22.3, 26.5)},
		{q: "latency", notebook: eng, page: 3, medians: msOf(9.8, 10.1, 10.8)},
		{q: "the", notebook: eng, page: 1, medians: msOf(6, 6.4, 7.2)},
		{q: "the", notebook: eng, page: 3, medians: msOf(4.7, 5.3, 5.4)},
		{q: "go", notebook: eng, page: 1, medians: msOf(22.4, 25.1, 25.7)},
		{q: "go", notebook: eng, page: 3, medians: msOf(12.1, 15.8, 12.7)},
		{q: "的", notebook: eng, page: 1, medians: msOf(7.4, 7, 6.9)},
		{q: "的", notebook: eng, page: 3, medians: msOf(5.1, 5.1, 5.2)},
		{q: "电", notebook: eng, page: 1, medians: msOf(3, 3.2, 3.4)},
		{q: "电 #", notebook: eng, page: 1, medians: msOf(5, 5.6, 7)},
		{q: "中国 database", notebook: eng, page: 1, medians: msOf(36.6, 39, 43)},
		{q: "theirs", notebook: eng, page: 1, medians: msOf(33.8, 39.5, 38.7)},
		{q: "theirs", notebook: eng, page: 3, medians: msOf(28.1, 31.4, 30.1)},
		{q: "the of and to in is that for", notebook: eng, page: 1, medians: msOf(53.4, 57.4, 60.7)},
		{q: "中国", page: 1, building: true, medians: msOf(17.2, 17.8, 18.9)},
		{q: "中国", page: 3, building: true, medians: msOf(12.5, 12.6, 13.2)},
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
