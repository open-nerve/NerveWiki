package domain

// minGap is the narrowest gap between two siblings' orders a node is put
// into: below it the siblings are renumbered, long before doubles run out
// of precision.
const minGap = 1e-9

// Place returns the order that puts a node after the after-th of siblings,
// the orders of a parent's children in order: -1 puts it first,
// len(siblings)-1 last. A parent without children puts it at 0. When no
// gap is left, renumbered holds the siblings' new orders, 0, 1, 2… in
// their order with the new node's slot skipped, and value is that slot's.
func Place(siblings []float64, after int) (value float64, renumbered []float64) {
	n := len(siblings)
	switch {
	case n == 0:
		return 0, nil
	case after < 0:
		if v := siblings[0] - 1; v < siblings[0] {
			return v, nil
		}
	case after >= n-1:
		if v := siblings[n-1] + 1; v > siblings[n-1] {
			return v, nil
		}
	default:
		lo, hi := siblings[after], siblings[after+1]
		if v := lo + (hi-lo)/2; hi-lo >= minGap && v > lo && v < hi {
			return v, nil
		}
	}
	slot := min(max(after+1, 0), n)
	renumbered = make([]float64, n)
	for i := range siblings {
		renumbered[i] = float64(i)
		if i >= slot {
			renumbered[i] = float64(i + 1)
		}
	}
	return float64(slot), renumbered
}
