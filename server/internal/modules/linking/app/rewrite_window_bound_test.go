package app

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func TestRewriteWindowUpperUsesOnlyPositiveByteDeltas(t *testing.T) {
	for _, tc := range []struct {
		name       string
		old, limit int
		edits      []domain.Edit
		upper      int
		ok         bool
	}{
		{"unchanged", 10, 20, nil, 10, true},
		{"negative_does_not_pay_positive", 10, 20, []domain.Edit{{Start: 0, End: 5, Text: ""}, {Start: 6, End: 7, Text: "ABCDE"}}, 14, true},
		{"insert_utf8", 10, 20, []domain.Edit{{Start: 3, End: 3, Text: "雪"}}, 13, true},
		{"unsorted", 10, 20, []domain.Edit{{Start: 7, End: 8, Text: "abc"}, {Start: 1, End: 2, Text: "defg"}}, 15, true},
		{"saturates_legal_candidates", 10, 12, []domain.Edit{{Start: 0, End: 1, Text: "abcdef"}}, 12, true},
		{"negative_start", 10, 20, []domain.Edit{{Start: -1, End: 1}}, 0, false},
		{"reverse", 10, 20, []domain.Edit{{Start: 3, End: 2}}, 0, false},
		{"past_content", 10, 20, []domain.Edit{{Start: 9, End: 11}}, 0, false},
		{"overlap", 10, 20, []domain.Edit{{Start: 0, End: 5}, {Start: 4, End: 8}}, 0, false},
		{"old_oversize", 21, 20, nil, 0, false},
		{"invalid_after_saturation", 10, 12, []domain.Edit{{Start: 0, End: 1, Text: "abcdef"}, {Start: 9, End: 11}}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := rewriteUpper(tc.old, tc.limit, tc.edits)
			if got != tc.upper || ok != tc.ok {
				t.Fatalf("WINDOW_UPPER: %d/%t, want literal %d/%t", got, ok, tc.upper, tc.ok)
			}
		})
	}
}
