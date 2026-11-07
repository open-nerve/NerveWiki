package app

import (
	"context"
	"reflect"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type positionEventPublisher struct {
	events []LinksChanged
}

func (p *positionEventPublisher) LinksChanged(_ context.Context, e LinksChanged) error {
	p.events = append(p.events, e)
	return nil
}

// Existing index and stream tests cover aliases and the 20/21 event boundary.
// These small inputs distinguish last-write-wins for (source,start), and the
// zero UUID's membership in pages from its exclusion in targets.
func TestPublishedLinksUseLastPositionAndSeparateZero(t *testing.T) {
	source := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	a := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	b := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	c := uuid.MustParse("00000000-0000-0000-0000-000000000004")
	zero := uuid.Nil()
	link := func(source uuid.UUID, start int, target uuid.UUID) Link {
		return Link{SourceID: source, Start: start, Resolution: domain.Resolution{ID: target}}
	}
	for _, tc := range []struct {
		name             string
		links, changed   []Link
		written, dropped []uuid.UUID
		want             *LinksChanged
	}{
		{
			name:    "last_zero_no_change",
			links:   []Link{link(source, 0, a), link(source, 0, zero)},
			changed: []Link{link(source, 0, zero)},
			dropped: []uuid.UUID{zero},
		},
		{
			name:    "last_zero_keeps_positions_separate",
			links:   []Link{link(source, 0, a), link(source, 0, zero), link(source, 10, b)},
			changed: []Link{link(source, 0, c), link(source, 10, b)},
			want:    &LinksChanged{Pages: []uuid.UUID{source}, Targets: []uuid.UUID{c}},
		},
		{
			name:    "later_nonzero_wins",
			links:   []Link{link(source, 0, zero), link(source, 0, a), link(source, 0, b)},
			changed: []Link{link(source, 0, c)},
			want:    &LinksChanged{Pages: []uuid.UUID{source}, Targets: []uuid.UUID{b, c}},
		},
		{
			name:    "zero_source_is_a_page",
			changed: []Link{link(zero, 0, a)},
			dropped: []uuid.UUID{zero, a, a},
			want:    &LinksChanged{Pages: []uuid.UUID{zero}, Targets: []uuid.UUID{a}},
		},
		{
			name:    "written_zero_leaves_non_nil_empty_pages",
			changed: []Link{link(zero, 0, a)},
			written: []uuid.UUID{source, zero, source},
			dropped: []uuid.UUID{zero},
			want:    &LinksChanged{Pages: []uuid.UUID{}, Targets: []uuid.UUID{a}},
		},
		{
			name:    "needed_last_zero_ignores_unrelated",
			links:   []Link{link(source, 0, a), link(source, 0, zero), link(source, 10, b)},
			changed: []Link{link(source, 0, zero)},
			want:    nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &positionEventPublisher{}
			x := Index{Publisher: p}
			if err := x.publish(context.Background(), PagesChanged{}, tc.links, tc.changed, tc.written, tc.dropped, false); err != nil {
				t.Fatal(err)
			}
			if tc.want == nil {
				if len(p.events) != 0 {
					t.Fatalf("last zero must not publish: %+v", p.events)
				}
				return
			}
			if !reflect.DeepEqual(p.events, []LinksChanged{*tc.want}) {
				t.Fatalf("published events=%+v want=%+v (including nil versus empty slices)", p.events, []LinksChanged{*tc.want})
			}
		})
	}
}
