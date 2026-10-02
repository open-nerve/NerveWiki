package domain_test

import (
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestPlace(t *testing.T) {
	for _, tt := range []struct {
		name     string
		siblings []float64
		after    int
		want     float64
	}{
		{"no siblings", nil, -1, 0},
		{"no siblings, last", nil, 0, 0},
		{"first", []float64{0, 1, 2}, -1, -1},
		{"last", []float64{0, 1, 2}, 2, 3},
		{"past the last is last", []float64{0, 1, 2}, 7, 3},
		{"between", []float64{0, 1, 2}, 0, 0.5},
		{"between negatives", []float64{-3, -1}, 0, -2},
	} {
		got, renumbered := domain.Place(tt.siblings, tt.after)
		if got != tt.want || renumbered != nil {
			t.Errorf("%s: Place(%v, %d) = %v, %v; want %v and no renumbering", tt.name, tt.siblings, tt.after, got, renumbered, tt.want)
		}
	}
}

// Putting a node in the same gap again and again halves the gap each time:
// the siblings are renumbered before two orders meet, and the order stays
// strict throughout.
func TestPlaceKeepsTheOrderStrict(t *testing.T) {
	orders := []float64{0, 1}
	renumberings := 0
	for i := range 200 {
		v, renumbered := domain.Place(orders, 0)
		if renumbered != nil {
			renumberings++
			orders = renumbered
		}
		orders = slices.Insert(orders, 1, v)
		for j := 1; j < len(orders); j++ {
			if orders[j-1] >= orders[j] {
				t.Fatalf("after %d insertions the orders are %v, want them strictly increasing", i+1, orders)
			}
		}
	}
	if renumberings == 0 {
		t.Error("200 insertions into one gap never renumbered the siblings")
	}
}

func TestPlaceRenumbersWhenNoGapIsLeft(t *testing.T) {
	siblings := []float64{0, 1e-10, 1}
	got, renumbered := domain.Place(siblings, 0)
	if got != 1 || !slices.Equal(renumbered, []float64{0, 2, 3}) {
		t.Errorf("Place(%v, 0) = %v, %v; want 1 between the renumbered 0, 2 and 3", siblings, got, renumbered)
	}
	first, renumbered := domain.Place(siblings, -1)
	if first != -1 || renumbered != nil {
		t.Errorf("Place(%v, -1) = %v, %v; want -1: the ends always have room", siblings, first, renumbered)
	}
}
