package domain_test

import (
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A trash item is restorable until the retention less the margin after its
// deletion, not at that time; the trash lists what is restorable.
func TestRestorable(t *testing.T) {
	deleted := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	const retention = 24 * time.Hour
	until := deleted.Add(23 * time.Hour)
	if got := domain.RestorableUntil(deleted, retention); !got.Equal(until) {
		t.Fatalf("RestorableUntil() = %s, want %s", got, until)
	}
	for _, tt := range []struct {
		name string
		now  time.Time
		want bool
	}{
		{"at its deletion", deleted, true},
		{"a microsecond before", until.Add(-time.Microsecond), true},
		{"at the time", until, false},
		{"after", until.Add(time.Microsecond), false},
	} {
		if got := domain.Restorable(deleted, tt.now, retention); got != tt.want {
			t.Errorf("%s: Restorable() = %v, want %v", tt.name, got, tt.want)
		}
		listed := deleted.After(domain.TrashSince(tt.now, retention))
		if listed != tt.want {
			t.Errorf("%s: listed after TrashSince() = %v, want %v as restorable", tt.name, listed, tt.want)
		}
	}
}

// A unit is too large past any of its bounds, but for a lone page, which
// one request wrote.
func TestUnitTooLarge(t *testing.T) {
	for _, tt := range []struct {
		name                string
		pages, bytes, links int
		want                bool
	}{
		{"none", 0, 0, 0, false},
		{"at every bound", domain.MaxUnitPages, domain.MaxUnitBytes, domain.MaxUnitLinks, false},
		{"a page more", domain.MaxUnitPages + 1, 0, 0, true},
		{"a byte more", 2, domain.MaxUnitBytes + 1, 0, true},
		{"a link more", 2, 0, domain.MaxUnitLinks + 1, true},
		{"a lone page past the bytes and links", 1, domain.MaxUnitBytes + 1, domain.MaxUnitLinks + 1, false},
	} {
		if got := domain.UnitTooLarge(tt.pages, tt.bytes, tt.links); got != tt.want {
			t.Errorf("%s: UnitTooLarge() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// The first page's cursor is above any item.
func TestTrashFrom(t *testing.T) {
	from := domain.TrashFrom()
	if from.ID != uuid.Max() || !from.At.Equal(time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)) {
		t.Errorf("TrashFrom() = %+v, want the last second and the largest id", from)
	}
}

// An item goes back under its parent alive when it fits there, as it was
// at the root, and at the root when its parent is in the trash or it would
// be too deep.
func TestPlacementOf(t *testing.T) {
	deleted := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	line := func(n int, parentDeleted bool) []domain.TrashAncestor {
		out := make([]domain.TrashAncestor, n)
		for i := range out {
			out[i] = domain.TrashAncestor{ID: uuid.NewV7(), Name: "a"}
		}
		if parentDeleted {
			out[n-1].DeletedAt = &deleted
		}
		return out
	}
	deep := line(domain.MaxDepth-3, false)
	for _, tt := range []struct {
		name   string
		line   []domain.TrashAncestor
		height int
		parent bool
		want   domain.PlacementReason
	}{
		{"at the root", nil, domain.MaxDepth, false, domain.PlacedOriginal},
		{"under its parent", line(2, false), 1, true, domain.PlacedOriginal},
		{"an attachment as deep as pages go", line(domain.MaxDepth, false), 0, true, domain.PlacedOriginal},
		{"fitting at the bound", deep, 3, true, domain.PlacedOriginal},
		{"a level too deep", deep, 4, false, domain.PlacedTooDeep},
		{"its parent in the trash", line(1, true), 1, false, domain.PlacedParentInTrash},
		{"its parent in the trash, deep", line(domain.MaxDepth, true), 9, false, domain.PlacedParentInTrash},
	} {
		got := domain.PlacementOf(tt.line, tt.height)
		switch {
		case got.Reason != tt.want:
			t.Errorf("%s: PlacementOf() = %s, want %s", tt.name, got.Reason, tt.want)
		case tt.parent && (got.ParentID == nil || *got.ParentID != tt.line[len(tt.line)-1].ID):
			t.Errorf("%s: PlacementOf() under %v, want its parent", tt.name, got.ParentID)
		case !tt.parent && got.ParentID != nil:
			t.Errorf("%s: PlacementOf() under %s, want the root", tt.name, got.ParentID)
		}
	}
}
