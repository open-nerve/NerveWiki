package domain

import (
	"time"
	"uuid"
)

// TreeState is where a node is in the tree: its parent, name and order.
type TreeState struct {
	ParentID  *uuid.UUID
	Name      string
	SortOrder float64
}

// Same reports whether s and o are one place in the tree.
func (s TreeState) Same(o TreeState) bool {
	return s.Name == o.Name && s.SortOrder == o.SortOrder && SameParent(s.ParentID, o.ParentID)
}

// SameParent reports whether a and b are one parent: the same page, or
// both the notebook's root.
func SameParent(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// Change is what a write did to a node: where it was before (nil: the
// write created it) and after (nil: the write deleted it), the revision it
// left the content at (0: the content is untouched), and that content's
// facts. A change of the content alone has the same before and after.
type Change struct {
	NodeID   uuid.UUID
	Before   *TreeState
	After    *TreeState
	Revision int
	// Facts are the content's facts when Revision is set: opaque here, the
	// app's Facts, taken before the transaction (M4 design 4, "parse
	// timing"; M6 design 4.7), without the tree; the guards, the
	// participants and the observers read them. They need not be
	// comparable: two Changes are not compared with == (M6/P2 review L3).
	Facts any
	// Restored is set on a change that brings its node back from the trash
	// (M8/P2 design 3.6): with no before, it tells a restore from a
	// creation.
	Restored *Restored
}

// Restored is a restore from the trash: when the node was deleted, which
// its attachment's file was too, and whether it is an attachment, as
// LinkNode and ExportNode tell one: an observer that follows attachments
// restores their files of that time, and counts them.
type Restored struct {
	DeletedAt time.Time
	Asset     bool
}

// Moves reports whether the change moves the node in the tree: it is
// created, deleted, renamed, moved or reordered. A change that does is a
// changeset's item.
func (c Change) Moves() bool {
	return c.Before == nil || c.After == nil || !c.Before.Same(*c.After)
}

// Then is c followed by a later change of the same node, as one change:
// the first before and the last after, and the last revision written with
// its facts. A restore is the first change's, as its before is: no unit of
// M8 deletes a node and restores it in one changeset.
func (c Change) Then(later Change) Change {
	out := Change{NodeID: c.NodeID, Before: c.Before, After: later.After, Revision: c.Revision, Facts: c.Facts, Restored: c.Restored}
	if later.Revision != 0 {
		out.Revision, out.Facts = later.Revision, later.Facts
	}
	return out
}

// Operation is what a write does, for the write guards.
type Operation string

// The operations of M4.
const (
	OpCreate Operation = "create"
	OpRename Operation = "rename"
	OpMove   Operation = "move"
	OpDelete Operation = "delete"
	// OpContent writes a page's content.
	OpContent Operation = "content"
	// OpRestore brings a trash item back (M8/P2 design 3.5).
	OpRestore Operation = "restore"
)
