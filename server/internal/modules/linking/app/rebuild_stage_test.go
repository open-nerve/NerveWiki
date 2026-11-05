package app

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func TestRebuildStageOwnsAllStringsIncludingEmpty(t *testing.T) {
	backing := strings.Repeat("q", 4096) + "targetpropertyanchordisplaylink"
	start := 4096
	empty := backing[start:start]
	canonicalEmpty := strings.Clone("")
	if rebuildStringSlot(&empty) == rebuildStringSlot(&canonicalEmpty) {
		t.Fatal("ownership fixture did not retain its nonempty backing")
	}
	input := []domain.Link{
		{Kind: backing[start+27:], Target: backing[start : start+6], Property: backing[start+6 : start+14],
			Anchor: backing[start+14 : start+20], Display: backing[start+20 : start+27], Start: 2, End: 9, Quote: '\'', InTable: true, Aliases: true},
		{Kind: empty, Target: empty, Property: empty, Anchor: empty, Display: empty, Start: 12, End: 13},
		{Kind: backing[1:5], Target: "z", Start: 20, End: 21},
	}
	want := slices.Clone(input)
	s := newRebuildStage(1, 0)
	if ok, err := s.add(t.Context(), 0, input); err != nil || !ok {
		t.Fatalf("stage = %t, %v", ok, err)
	}
	got := s.links[0]
	if !slices.Equal(got, want) || len(got) != cap(got) {
		t.Fatalf("owned complete values/capacity = %+v %d/%d", got, len(got), cap(got))
	}
	for i, before := range want {
		after := got[i]
		oldStrings := []string{before.Kind, before.Target, before.Property, before.Anchor, before.Display}
		newStrings := []string{after.Kind, after.Target, after.Property, after.Anchor, after.Display}
		for j, value := range newStrings {
			ptr := rebuildStringSlot(&newStrings[j])
			if value == "" {
				if ptr != rebuildStringSlot(&canonicalEmpty) {
					t.Fatalf("row%d field%d retained empty substring backing", i, j)
				}
			} else if ptr == rebuildStringSlot(&oldStrings[j]) {
				t.Fatalf("row%d field%d still aliases the parser string", i, j)
			}
		}
	}
	clear(input)
	if !slices.Equal(got, want) {
		t.Fatal("later parser slice reuse changed staged links")
	}
	slots := s.links
	s.release()
	if s.links != nil || s.order != nil || s.rows != 0 || s.charged != 0 || slots[0] != nil {
		t.Fatal("release left staged rows reachable through the slot array")
	}
}

func TestRebuildStageChargeAndAdmissionBoundaries(t *testing.T) {
	linkBytes := uint64(reflect.TypeFor[domain.Link]().Size())
	slotBytes := uint64(reflect.TypeFor[[]domain.Link]().Size() + reflect.TypeFor[int]().Size())
	t.Logf("actual layouts: domain.Link=%d app.Link=%d Resolution=%d", linkBytes,
		reflect.TypeFor[Link]().Size(), reflect.TypeFor[domain.Resolution]().Size())
	if reflect.TypeFor[int]().Size() == 8 && (linkBytes != 104 || reflect.TypeFor[Link]().Size() != 64 || reflect.TypeFor[domain.Resolution]().Size() != 18) {
		t.Fatal("review the changed native layout and resource estimate")
	}
	row := []domain.Link{{Kind: "link", Target: "aa.a", Start: 1, End: 5}}
	limit := 2*slotBytes + linkBytes + 16
	s := newRebuildStage(2, limit)
	if s == nil || s.charged != 2*slotBytes || cap(s.links) != 2 {
		t.Fatal("exact slot reservation failed")
	}
	if ok, err := s.add(t.Context(), 0, row); err != nil || !ok || s.charged != limit {
		t.Fatalf("exact fit = %t, %v, charge=%d", ok, err, s.charged)
	}
	if ok, err := s.add(t.Context(), 1, row); err != nil || ok || s.links[1] != nil || s.rows != 1 || s.charged != limit {
		t.Fatalf("overflow changed admission state: %t %v %+v", ok, err, s)
	}
	if ok, err := s.add(t.Context(), 1, []domain.Link{}); err != nil || !ok || s.links[1] == nil {
		t.Fatal("a nonnil empty slice lost its shape or acquired a payload charge")
	}
	oneLess := newRebuildStage(1, slotBytes+linkBytes+15)
	if ok, err := oneLess.add(t.Context(), 0, row); err != nil || ok || oneLess.links[0] != nil {
		t.Fatal("one byte below the whole-page charge admitted a snapshot")
	}
	if newRebuildStage(1, slotBytes-1) != nil || newRebuildStage(-1, 0) != nil || newRebuildStage(int(^uint(0)>>1), 0) != nil {
		t.Fatal("slot overflow must select streaming before allocation")
	}
	if newRebuildStage(0, 0).limit != 512<<20 || newRebuildStage(0, ^uint64(0)).limit != 512<<20 {
		t.Fatal("zero/default or positive override raised the hard bound")
	}
	for _, n := range []uint64{0, 1, 4, 8, 9, 1000} {
		want := map[uint64]uint64{0: 0, 1: 16, 4: 16, 8: 16, 9: 18, 1000: 2000}[n]
		if got, ok := rebuildStringBytes(n); !ok || got != want {
			t.Fatalf("string charge %d = %d/%t want%d", n, got, ok, want)
		}
	}
	if _, ok := rebuildStringBytes(^uint64(0)); ok {
		t.Fatal("overflowed string charge")
	}
	if _, ok := rebuildMul(^uint64(0), 2); ok {
		t.Fatal("overflowed slice charge")
	}
	if _, ok := rebuildAdd(^uint64(0), 1); ok {
		t.Fatal("overflowed aggregate charge")
	}
	// Resource-policy arithmetic only, not four million allocations or a timing gate.
	if uint64(4_000_000)*(linkBytes+16)+1100*slotBytes >= 512<<20 {
		t.Fatal("the approved short-link resource estimate no longer fits")
	}
}

func TestRebuildStageOrdersOnlyOwnedRowsAndKeepsResolutionPositions(t *testing.T) {
	ids := []uuid.UUID{{15: 2}, {}, {15: 1}}
	rows := [][]domain.Link{
		{{Kind: "link", Start: 30, End: 31, Target: "source-two"}},
		{{Kind: "link", Start: 7, End: 8, Target: "zero"}},
		{{Kind: "image", Start: 20, End: 21, Target: "last"}, {Kind: "link", Start: 5, End: 6, Target: "first", Aliases: true}, {Kind: "embed", Start: 5, End: 9, Target: "tie", Quote: '"'}},
	}
	s := newRebuildStage(len(ids), 0)
	for i := range rows {
		if ok, err := s.add(t.Context(), i, rows[i]); err != nil || !ok {
			t.Fatal(ok, err)
		}
	}
	got, err := s.inputs(t.Context(), ids)
	want := []Link{
		{SourceID: ids[1], Start: 7, Target: "zero"},
		{SourceID: ids[2], Start: 5, Target: "first", Aliases: true},
		{SourceID: ids[2], Start: 5, Target: "tie"},
		{SourceID: ids[2], Start: 20, Target: "last"},
		{SourceID: ids[0], Start: 30, Target: "source-two"},
	}
	if err != nil || !slices.Equal(got, want) || cap(got) != len(got) || !slices.Equal(s.order, []int{1, 2, 0}) {
		t.Fatalf("ordered inputs=%+v order=%v err=%v", got, s.order, err)
	}
	if rows[2][0].Target != "last" || s.links[2][0].Target != "first" || s.links[2][1].Quote != '"' || s.links[2][1].End != 9 {
		t.Fatal("sort mutated parser rows, lost stable ties or detached persisted fields")
	}
	// Final row/resolution slices use the same source order and contiguous offsets.
	at := 0
	for _, slot := range s.order {
		for _, l := range s.links[slot] {
			if got[at].SourceID != ids[slot] || got[at].Start != int(int32(l.Start)) || got[at].Target != l.Target {
				t.Fatal("resolution position no longer identifies its complete stored row")
			}
			at++
		}
	}
}

func TestRebuildStageCancellationAndNilRelease(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	s := newRebuildStage(1, 0)
	if ok, err := s.add(ctx, 0, []domain.Link{{Kind: "link", Target: "x"}}); ok || !errors.Is(err, context.Canceled) || s.links[0] != nil || s.rows != 0 {
		t.Fatalf("canceled snapshot=%t/%v rows=%d", ok, err, s.rows)
	}
	if _, err := s.inputs(ctx, []uuid.UUID{{15: 1}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled input construction=%v", err)
	}
	s.release()
	var absent *rebuildStage
	absent.release()
}

// Observe the actual string slot: string-to-any boxing canonicalizes empty
// strings before reflect.ValueOf can inspect their original backing.
func rebuildStringSlot(value *string) uintptr {
	return reflect.ValueOf(value).Elem().Pointer()
}
