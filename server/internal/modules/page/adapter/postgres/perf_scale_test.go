package postgresadapter_test

import (
	"fmt"
	"strconv"
	"testing"
)

// changesetsOf is the long log's changesets (history_cost_test.go): 100,000
// unless NW_SCALE_CHANGESETS gives another number by hand (M12/P4 design
// 3.3), a multiple of 5 of 100,000 at least, with true; a number that is
// not, or none, is an error. The switch is there to measure larger logs,
// and the log's reads hold their pages at 100,000 and past it: the rare
// client's changesets, one in a thousand, fill a page of 101 from 101,000
// and are 100 below it, which the page's read allows; a fifth of the
// changesets without items are the deletions of the unit's 10,000 roots
// that TestTheRevertsArrayReadsInTimeAtTheUnitsBound reads.
func changesetsOf(getenv func(string) string) (int, bool, error) {
	s := getenv("NW_SCALE_CHANGESETS")
	if s == "" {
		return 100_000, false, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 100_000 || n%5 != 0 {
		return 0, false, fmt.Errorf("NW_SCALE_CHANGESETS=%q is no multiple of 5 of 100,000 at least", s)
	}
	return n, true, nil
}

// The long log is 100,000 changesets unless a number is given by hand, a
// multiple of 5 of 100,000 at least (M12/P4 design 3.3, 7.2's 20).
func TestALongLogIsAHundredThousandChangesetsUnlessGiven(t *testing.T) {
	for _, c := range []struct {
		value  string
		n      int
		byHand bool
	}{{"", 100_000, false}, {"1000000", 1_000_000, true}, {"100000", 100_000, true}, {"100005", 100_005, true},
		{"250005", 250_005, true}} {
		n, byHand, err := changesetsOf(func(k string) string {
			if k == "NW_SCALE_CHANGESETS" {
				return c.value
			}
			return ""
		})
		if err != nil || n != c.n || byHand != c.byHand {
			t.Errorf("NW_SCALE_CHANGESETS=%q: %d, %v, %v; want %d, %v", c.value, n, byHand, err, c.n, c.byHand)
		}
	}
	for _, value := range []string{"many", "0", "50000", "99995", "100001", "-100000", "1e6"} {
		if n, _, err := changesetsOf(func(string) string { return value }); err == nil {
			t.Errorf("NW_SCALE_CHANGESETS=%q: %d, want an error", value, n)
		}
	}
}
