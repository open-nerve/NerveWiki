package domain

import (
	"time"
	"uuid"
)

// A changeset's revert (M8 design 4.8; M8/P3 design 3.5): every node it
// changed goes back to where it was before it, compared by state, not by
// time or revision, whole or not at all.

// ConflictReason is why a node keeps a revert from its changeset.
type ConflictReason string

// The reasons, in the order the conflicts are listed (M8/P3 design 3.5): a
// node is no longer as the changeset left it; a deletion's trash item is
// past the time it can be restored in; a node would go back under a
// parent no longer alive; a node another changeset put under one the
// revert deletes would go with it; a node would go back to a name another
// holds there; under itself; or with pages deeper than MaxDepth.
const (
	ConflictChanged       ConflictReason = "changed"
	ConflictExpired       ConflictReason = "expired"
	ConflictParentGone    ConflictReason = "parent_gone"
	ConflictChildrenAdded ConflictReason = "children_added"
	ConflictTitleTaken    ConflictReason = "title_taken"
	ConflictCycle         ConflictReason = "cycle"
	ConflictTooDeep       ConflictReason = "too_deep"
)

// MaxConflicts is how many of a revert's conflicts its problem names (M8
// design 4.14): the first, with how many there are.
const MaxConflicts = 100

// RevertConflict is a node that keeps a revert from its changeset: its
// name where the revert would put it (the name a title_taken contends
// for), its name now when the revert would leave it nowhere; why; and the
// node holding the name of a title_taken.
type RevertConflict struct {
	NodeID   uuid.UUID
	Name     string
	Reason   ConflictReason
	HolderID *uuid.UUID
}

// NodeNow is a node as it is now, alive or in the trash: DeletedAt is when
// it was deleted, nil while it is alive; a trash item's root holds its
// group's pages and attachments.
type NodeNow struct {
	Node
	DeletedAt               *time.Time
	TrashRoot               bool
	TrashPages, TrashAssets int
}

// ContentNow is a page's content now: its revision and hash.
type ContentNow struct {
	Revision int
	Hash     []byte
}
