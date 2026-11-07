package postgresadapter

import (
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Eligibility requires unique starts, not sorted or disjoint ranges. A
// reversal must not forget any earlier start or rewrite the caller's facts.
func TestRetainablePageFactsEligibility(t *testing.T) {
	p := app.Page{ID: uuid.UUID{15: 1}, NotebookID: uuid.UUID{15: 2}, Revision: 1}
	for _, tc := range []struct {
		name   string
		ranges [][2]int
		change func([]domain.Link)
		want   bool
	}{
		{name: "empty", want: true},
		{name: "zero_start", ranges: [][2]int{{0, 4}}, want: true},
		{name: "strict_growth", ranges: [][2]int{{0, 4}, {10, 14}, {20, 24}}, want: true},
		{name: "overlapping_unique", ranges: [][2]int{{0, 20}, {10, 15}}, want: true},
		{name: "unordered_unique", ranges: [][2]int{{10, 14}, {0, 4}, {20, 24}}, want: true},
		{name: "descending_unique", ranges: [][2]int{{20, 24}, {10, 14}, {0, 4}}, want: true},
		{name: "reversal_then_growth", ranges: [][2]int{{0, 4}, {10, 14}, {20, 24}, {5, 9}, {30, 34}}, want: true},
		{name: "duplicate_zero", ranges: [][2]int{{0, 4}, {0, 7}}, want: false},
		{name: "late_prefix_duplicate", ranges: [][2]int{{0, 4}, {10, 14}, {20, 24}, {5, 9}, {10, 17}}, want: false},
		{name: "late_zero_prefix_duplicate", ranges: [][2]int{{0, 4}, {10, 14}, {20, 24}, {5, 9}, {0, 7}}, want: false},
		{name: "duplicate_after_reversal", ranges: [][2]int{{0, 4}, {10, 14}, {20, 24}, {5, 9}, {5, 8}}, want: false},
		{name: "negative_start", ranges: [][2]int{{-1, 4}}, want: false},
		{name: "empty_range", ranges: [][2]int{{0, 0}}, want: false},
		{name: "unparseable_target", ranges: [][2]int{{0, 4}}, change: func(links []domain.Link) { links[0].Target = "a//b" }, want: true},
		{name: "empty_target", ranges: [][2]int{{0, 4}}, change: func(links []domain.Link) { links[0].Target = "" }, want: false},
		{name: "late_invalid_kind", ranges: [][2]int{{0, 4}, {10, 14}}, change: func(links []domain.Link) { links[1].Kind = "unknown" }, want: false},
		{name: "late_invalid_utf8", ranges: [][2]int{{0, 4}, {10, 14}}, change: func(links []domain.Link) { links[1].Display = string([]byte{0xff}) }, want: false},
		{name: "late_nul", ranges: [][2]int{{0, 4}, {10, 14}}, change: func(links []domain.Link) { links[1].Anchor = "a\x00b" }, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var facts domain.Facts
			for _, r := range tc.ranges {
				facts.Links = append(facts.Links, domain.Link{Kind: "link", Target: "Note", Start: r[0], End: r[1]})
			}
			if tc.change != nil {
				tc.change(facts.Links)
			}
			original, page := slices.Clone(facts.Links), p
			if got := retainablePageFacts(p, facts); got != tc.want {
				t.Errorf("retainable=%t, want %t for literal ranges %v", got, tc.want, tc.ranges)
			}
			if !slices.Equal(facts.Links, original) || p != page {
				t.Fatal("eligibility changed the caller's page or link facts")
			}
		})
	}
}

func TestRetainablePageFactsOrderedAllocations(t *testing.T) {
	p := app.Page{ID: uuid.UUID{15: 1}, NotebookID: uuid.UUID{15: 2}, Revision: 1}
	facts := domain.Facts{Links: make([]domain.Link, 10_000)}
	for i := range facts.Links {
		facts.Links[i] = domain.Link{Kind: "link", Target: "Note", Start: 10 * i, End: 10*i + 4}
	}
	original := slices.Clone(facts.Links)
	accepted := true
	allocs := testing.AllocsPerRun(100, func() {
		accepted = retainablePageFacts(p, facts) && accepted
	})
	if !accepted || !slices.Equal(facts.Links, original) {
		t.Fatal("ordered 10000-link facts were rejected or changed")
	}
	t.Logf("ordered 10000 links: allocations per call=%.0f", allocs)
	if allocs != 0 {
		t.Fatalf("ordered 10000 links allocated %.0f objects per call, want zero", allocs)
	}
}
