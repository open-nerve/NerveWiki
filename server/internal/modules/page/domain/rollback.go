package domain

import (
	"bytes"
	"slices"
	"uuid"
)

// An upgrade's rollback (M11 design 4.10; M11/P1 design 3.18): each pass
// deletes the new subtrees that hold no baseline node, or, with none
// left, puts the rest back by a revert's plan made from the baseline, as
// if a changeset C had made the difference between the baseline and now.

// BaselineNode is a node as an upgrade's start found it (M11 design 4.4
// step 11), its baseline row: where it was, and a page's version, its
// content's hash and size.
type BaselineNode struct {
	NodeID    uuid.UUID
	Kind      Kind
	ParentID  *uuid.UUID
	Name      string
	SortOrder float64
	Revision  int
	Hash      []byte
	Bytes     int
}

// State is where the node was.
func (b BaselineNode) State() TreeState {
	return TreeState{ParentID: b.ParentID, Name: b.Name, SortOrder: b.SortOrder}
}

// RollbackLeft is what a rollback has left to do (M11 decision 226): the
// baseline nodes not in place (parent, name or order not the baseline's,
// or not alive), the baseline pages whose content is not the baseline's,
// and the live nodes with no baseline row.
type RollbackLeft struct {
	Nodes, Pages, Created int
}

// Zero reports whether nothing is left: the notebook is as the upgrade
// started.
func (l RollbackLeft) Zero() bool {
	return l == RollbackLeft{}
}

// Sum is the three counts together, which each pass makes smaller (M11/P1
// design 2.1 item 12).
func (l RollbackLeft) Sum() int {
	return l.Nodes + l.Pages + l.Created
}

// FreeNode is a new node alive that holds no baseline node, and its
// parent: none for one at the root.
type FreeNode struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
}

// PackFreeSubtrees packs the free nodes into the steps of a pass that
// deletes first (M11 design 4.10, decisions 231 and 243): a free node
// whose parent is not free roots a free subtree; each step is the roots of
// subtrees of pack nodes at most together, in the order of their roots'
// ids, each step as full as the next subtree lets it; a subtree past pack,
// of whole nodes at most, is a step alone, never split. A subtree past
// whole is not deleted whole this pass: the subtrees of whole nodes at
// most right under its largest part are, from the bottom up, and it waits
// for a later pass, smaller then. The roots of a pass are pairwise apart:
// none is under another, so each deletion takes a whole subtree.
func PackFreeSubtrees(forest []FreeNode, whole, pack int) [][]uuid.UUID {
	free := make(map[uuid.UUID]bool, len(forest))
	for _, n := range forest {
		free[n.ID] = true
	}
	children := map[uuid.UUID][]uuid.UUID{}
	var roots []uuid.UUID
	for _, n := range forest {
		if n.ParentID != nil && free[*n.ParentID] {
			children[*n.ParentID] = append(children[*n.ParentID], n.ID)
		} else {
			roots = append(roots, n.ID)
		}
	}
	// The sizes, from the bottom up: a post-order of each tree, iterative,
	// a tree a thousand levels deep being no defect of the input.
	size := make(map[uuid.UUID]int, len(forest))
	for _, root := range roots {
		type frame struct {
			id   uuid.UUID
			next int
		}
		stack := []frame{{id: root}}
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if kids := children[top.id]; top.next < len(kids) {
				top.next++
				stack = append(stack, frame{id: kids[top.next-1]})
				continue
			}
			n := 1
			for _, k := range children[top.id] {
				n += size[k]
			}
			size[top.id] = n
			stack = stack[:len(stack)-1]
		}
	}
	// The subtrees deleted this pass: each root of whole nodes at most, and
	// under a larger one each child that is, the larger ones gone down.
	var deleted []uuid.UUID
	pending := slices.Clone(roots)
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if size[id] <= whole {
			deleted = append(deleted, id)
			continue
		}
		pending = append(pending, children[id]...)
	}
	slices.SortFunc(deleted, func(a, b uuid.UUID) int { return a.Compare(b) })
	var steps [][]uuid.UUID
	var step []uuid.UUID
	n := 0
	for _, id := range deleted {
		if n+size[id] > pack && len(step) > 0 {
			steps, step, n = append(steps, step), nil, 0
		}
		step, n = append(step, id), n+size[id]
	}
	if len(step) > 0 {
		steps = append(steps, step)
	}
	return steps
}

// RollbackState is the state a rollback's plan reverts, of the changeset
// C that would have made now from the baseline (M11 design 4.10): an item
// for each baseline node alive whose parent, name or order is not the
// baseline's, from the baseline's place to its own; one for each new node
// alive holding baseline nodes, holders, from nowhere (a deletion when
// reverted); a version for each baseline page alive whose content is not
// the baseline's, from the baseline's version to its own. Every item's
// Revision is 0, none Restored: a moved page's content is no item's to
// compare (revert.go's left), and C restored nothing. A version rewrites
// nothing. Hashes holds each version's base's hash, the baseline's. Nodes
// and Contents are now's, which the caller read: a baseline node not
// alive (the guardrails keep any from being deleted, so a defect or a
// hand's SQL) is no item, which the rollback's left count tells. Past
// maxItems of the items, page.revert_too_large; the versions past what is
// left of maxItems, by their pages' ids, wait for a later pass.
func RollbackState(baseline []BaselineNode, holders []uuid.UUID, now map[uuid.UUID]NodeNow, contents map[uuid.UUID]ContentNow,
	maxItems int,
) (RevertState, error) {
	var items []ChangesetItem
	var versions []ChangesetVersion
	hashes := map[VersionRef][]byte{}
	for _, b := range baseline {
		n, ok := now[b.NodeID]
		if !ok || n.DeletedAt != nil {
			continue
		}
		if !n.State().Same(b.State()) {
			before, after := b.State(), n.State()
			items = append(items, ChangesetItem{NodeID: b.NodeID, Before: &before, After: &after})
		}
		if b.Kind != KindPage {
			continue
		}
		c, ok := contents[b.NodeID]
		if !ok || bytes.Equal(c.Hash, b.Hash) {
			continue
		}
		base := b.Revision
		versions = append(versions, ChangesetVersion{NodeID: b.NodeID, Base: &base, BaseBytes: b.Bytes, Revision: c.Revision, Hash: c.Hash})
		hashes[VersionRef{NodeID: b.NodeID, Revision: base}] = b.Hash
	}
	for _, id := range holders {
		n, ok := now[id]
		if !ok || n.DeletedAt != nil {
			continue
		}
		after := n.State()
		items = append(items, ChangesetItem{NodeID: id, After: &after})
	}
	if len(items) > maxItems {
		return RevertState{}, ErrRevertTooLarge
	}
	if room := maxItems - len(items); len(versions) > room {
		slices.SortFunc(versions, func(a, b ChangesetVersion) int { return a.NodeID.Compare(b.NodeID) })
		for _, v := range versions[room:] {
			delete(hashes, VersionRef{NodeID: v.NodeID, Revision: *v.Base})
		}
		versions = versions[:room]
	}
	slices.SortFunc(items, func(a, b ChangesetItem) int { return a.NodeID.Compare(b.NodeID) })
	return RevertState{Items: items, Versions: versions, Nodes: now, Contents: contents, Hashes: hashes,
		Groups: map[uuid.UUID][]TrashMember{}}, nil
}
