package domain

import (
	"slices"
	"uuid"
)

// The background restores and reverts (M8 design 4.10; M8/P4 design 3.3,
// 3.4): a trash item or a changeset past one unit is written in batches of
// steps, each batch a unit of the job's changeset, the notebook's row
// released between them.

// MaxJobItems bounds the changeset a background revert reverts by its
// items (M8 design 4.9; M8/P4 design 3.11): the request and the job's
// start each read them all, with their nodes, groups and the notebook's
// tree, and plan them, and the costliest step it cuts, a top's deletion or
// a group's first batch, grows with them. It is the largest size whose
// costliest step took 5 s at most (BenchmarkBackgroundSteps): the
// deletion of a top of an import of that many nodes, its pages of 2 KiB
// with ten links each, 4.7 s at 90,000, 5.1 s at 100,000, analyzed; at
// 90,000 the request takes 0.5 s, a group's first batch 1.4 s, a plan
// 14 MiB at most. The transfer module bounds an import's nodes by it, so
// that a background revert can undo any import (M8/P4 design 2.2 (a)).
const MaxJobItems = 90_000

// JobStep is a step of a background job (M8/P4 design 3.3), a write no
// batch splits: a restore's node; a revert's place of a node, deletion of
// a top's subtree, node of a group it restores or page it writes back.
// Nodes are the nodes it writes, a write back's 1; Bytes the content it
// parses.
type JobStep struct {
	Nodes int
	Bytes int64
}

// ShapeNode is a node of a trash item's group as a background restore's
// plan reads it, a level at a time: its id and its parent's, whether it is
// an attachment, and a page's content's bytes, zero when the purge took
// it.
type ShapeNode struct {
	ID       uuid.UUID
	ParentID uuid.UUID
	Asset    bool
	Bytes    int
}

// GroupNode is a node of a trash item's group as a background restore's
// plan keeps it, in the group's pre-order (GroupPreOrder): its id; its
// parent's position, -1 for the group's root; whether it is an
// attachment; a page's content's bytes; and, once GroupCounts counted
// them, the pages, attachments and levels of pages of its subtree within
// the group, itself included. No name: the plan of a group of hundreds of
// thousands of nodes keeps some 40 bytes of each, and each batch reads its
// own (M8/P4 design 3.4).
type GroupNode struct {
	ID            uuid.UUID
	Parent        int32
	Bytes         int32
	Pages, Assets int32
	Height        int16
	Asset         bool
}

// Size is how many nodes the node's subtree within the group holds, itself
// included: in the pre-order, its subtree is the nodes from it on, so
// many.
func (n GroupNode) Size() int {
	return int(n.Pages + n.Assets)
}

// GroupPreOrder is the trash item's group of root, whose descendants are
// below, in its depth-first pre-order (M8/P4 design 3.4): each parent
// before its children, which follow each other in below's order, by order
// and id as TrashShape reads them, each subtree whole before the next
// sibling. A background restore restores a stretch of it at a time: a
// node's parent is restored before it, and of two siblings named alike
// the first keeps its name, as a restore in one unit has it. A node whose
// parent is not in the group is left out: none is.
func GroupPreOrder(root ShapeNode, below []ShapeNode) []GroupNode {
	children := make(map[uuid.UUID][]int, len(below))
	for i, n := range below {
		children[n.ParentID] = append(children[n.ParentID], i)
	}
	out := make([]GroupNode, 0, len(below)+1)
	add := func(n ShapeNode, parent int) {
		out = append(out, GroupNode{ID: n.ID, Parent: int32(parent), Bytes: int32(n.Bytes), Asset: n.Asset})
	}
	add(root, -1)
	// A stack of the nodes to visit, each with its parent's position: a
	// node's children pushed last first, so that the first comes off next.
	type visit struct{ at, parent int }
	var stack []visit
	push := func(id uuid.UUID, parent int) {
		kids := children[id]
		for i := len(kids) - 1; i >= 0; i-- {
			stack = append(stack, visit{at: kids[i], parent: parent})
		}
	}
	push(root.ID, 0)
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		add(below[v.at], v.parent)
		push(below[v.at].ID, len(out)-1)
	}
	return out
}

// GroupCounts counts each node's subtree of the group in its pre-order,
// in place: its pages and attachments, itself included, and the levels of
// pages it spans, as Subtree.Height counts them, an attachment no level. A
// node of the group a batch leaves in the trash becomes a trash item with
// these counts (Frontier).
func GroupCounts(nodes []GroupNode) {
	for i := range nodes {
		n := &nodes[i]
		n.Pages, n.Assets, n.Height = 0, 0, 0
		if n.Asset {
			n.Assets = 1
		} else {
			n.Pages = 1
		}
	}
	// In reverse, a node's children are counted before it: its Height
	// holds their highest until it adds its own level.
	for i := len(nodes) - 1; i >= 0; i-- {
		n := &nodes[i]
		if n.Asset {
			n.Height = 0
		} else {
			n.Height++
		}
		if n.Parent >= 0 {
			p := &nodes[n.Parent]
			p.Pages, p.Assets, p.Height = p.Pages+n.Pages, p.Assets+n.Assets, max(p.Height, n.Height)
		}
	}
}

// Frontier are the nodes of the group in its pre-order that a batch,
// which restored the nodes from to to (not included) restored tells,
// leaves in the trash under a node it brought back (M8/P4 design 3.4):
// the children of those nodes from to on, each to become a trash item of
// its own, with its subtree's counts. In the pre-order they are the
// remaining children of the nodes on the batch's right edge, about levels
// times fan-out, never a whole level; a child of a node the batch did not
// restore (someone else did) is left alone. Ascending.
func Frontier(nodes []GroupNode, from, to int, restored func(i int) bool) []int {
	var out []int
	for i := from; i < to; i++ {
		if !restored(i) {
			continue
		}
		// The children follow each other, each after its subtree: those
		// before to the batch restored or left as it did their parent.
		for c, end := i+1, i+nodes[i].Size(); c < end; c += nodes[c].Size() {
			if c >= to {
				out = append(out, c)
			}
		}
	}
	slices.Sort(out)
	return out
}
