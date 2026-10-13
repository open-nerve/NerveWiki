package domain

import (
	"encoding/json"
	"time"
	"uuid"
)

// The trash (M8 design 4.5, 4.6, 4.9, 4.11; M8/P2 design 3.4, 3.8, 3.11):
// a deletion's root is a trash item, which holds the nodes deleted with it,
// at its time, its group. It is listed, read and restored whole, within a
// time the purge keeps clear of.

// TrashMargin is how long before the purge may take a trash item it can no
// longer be restored (M8 design 4.11): the purge takes what was deleted
// before now less the retention, a restore what was deleted after now less
// the retention and the margin, so that the two never reach one group and
// lock nothing against each other.
const TrashMargin = time.Hour

// RestorableUntil is when a trash item deleted at deletedAt can no longer be
// restored, under retention, the purge's.
func RestorableUntil(deletedAt time.Time, retention time.Duration) time.Time {
	return deletedAt.Add(retention - TrashMargin)
}

// Restorable reports whether a trash item deleted at deletedAt can be
// restored at now: before RestorableUntil, not at it.
func Restorable(deletedAt, now time.Time, retention time.Duration) bool {
	return now.Before(RestorableUntil(deletedAt, retention))
}

// TrashSince is the time the trash lists the items deleted after at now:
// those Restorable.
func TrashSince(now time.Time, retention time.Duration) time.Time {
	return now.Add(TrashMargin - retention)
}

// A unit's bounds (M8 design 4.9): what one request restores, or reverts
// (M8/P3). The pages it parses, their content's bytes and their links are
// those of an import's batch (M7/P6 design 3.3), but a lone page, which one
// request wrote, is never past them. The nodes of the tree it writes, pages
// and attachments, are bounded by what its statements write within the
// request's time.
const (
	MaxUnitPages = 100
	MaxUnitBytes = 8 << 20
	MaxUnitLinks = 20_000
	// MaxUnitNodes is measured (M8/P2 design 3.11).
	MaxUnitNodes = 2_000
)

// UnitTooLarge reports whether a unit's pages, their content's bytes or
// their links are past its bounds: a lone page's never are.
func UnitTooLarge(pages, bytes, links int) bool {
	return pages > 1 && (pages > MaxUnitPages || bytes > MaxUnitBytes || links > MaxUnitLinks)
}

// TrashRoot is a trash item: its root node, which UpdatedBy deleted at
// DeletedAt; how many pages and attachments its group holds, root
// included, and how many levels of pages it spans, as the deletion counted
// them; and a page's content's revision, zero for an attachment, or a page
// whose content the purge took.
type TrashRoot struct {
	Node          Node
	DeletedAt     time.Time
	Pages, Assets int
	Height        int
	Revision      int
}

// TrashAncestor is an ancestor of a trash item's root, deleted or not: its
// name is part of where the item was, and DeletedAt when it was deleted,
// nil while it is alive.
type TrashAncestor struct {
	ID        uuid.UUID
	Name      string
	DeletedAt *time.Time
}

// TrashMember is a node of a trash item's group at its level, the root's
// 1; a page's with its content's revision and bytes, zero when the purge
// took its content.
type TrashMember struct {
	Node     Node
	Level    int
	Revision int
	Bytes    int
}

// TrashCursor is a page of a notebook's trash's cursor: the next page lists
// the items deleted before At, or at At with an id below ID: the last one
// listed.
type TrashCursor struct {
	At time.Time `json:"at"`
	ID uuid.UUID `json:"id"`
}

// MarshalJSON writes the cursor with At in UTC, so that each position has
// one spelling: shared.DecodeCursor refuses another.
func (c TrashCursor) MarshalJSON() ([]byte, error) {
	type plain TrashCursor
	return json.Marshal(plain{At: c.At.UTC(), ID: c.ID})
}

// TrashFrom is the cursor above every item: the first page's. A cursor at
// no time would read as one at the first time (pgx sends no infinity).
func TrashFrom() TrashCursor {
	return TrashCursor{At: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), ID: uuid.Max()}
}

// PlacementReason is why a trash item goes back where it does.
type PlacementReason string

// The reasons (M8 design 4.6): its parent is alive and it fits under it
// again, or it is restored at the notebook's root, its parent in the trash
// or its pages too deep under it. Its parent is never gone: the purge
// deletes leaves first, and a node's parent row stays while it does.
const (
	PlacedOriginal      PlacementReason = "original"
	PlacedParentInTrash PlacementReason = "parent_in_trash"
	PlacedTooDeep       PlacementReason = "too_deep"
)

// Placement is where a trash item goes back: under ParentID, nil for the
// notebook's root, and why.
type Placement struct {
	ParentID *uuid.UUID
	Reason   PlacementReason
}

// PlacementOf is where a trash item goes back, whose root's line is line,
// its ancestors from the notebook's root down to its parent, and whose
// group spans height levels of pages: under its parent when the parent is
// alive and the group fits under it within MaxDepth (as a move checks:
// Depth(line) + height - 1), or at the root, as it was when line is empty.
func PlacementOf(line []TrashAncestor, height int) Placement {
	if len(line) == 0 {
		return Placement{Reason: PlacedOriginal}
	}
	parent := line[len(line)-1]
	switch {
	case parent.DeletedAt != nil:
		return Placement{Reason: PlacedParentInTrash}
	case len(line)+height > MaxDepth:
		return Placement{Reason: PlacedTooDeep}
	}
	return Placement{ParentID: &parent.ID, Reason: PlacedOriginal}
}
