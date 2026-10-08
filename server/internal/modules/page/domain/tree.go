package domain

import (
	"cmp"
	"slices"
	"uuid"
)

// PreOrder orders a notebook's nodes as its tree reads: each parent before
// its children, siblings by order and then id. A node whose parent is not
// among nodes is left out: it cannot be reached.
func PreOrder(nodes []Node) []Node {
	children := map[uuid.UUID][]Node{}
	var roots []Node
	for _, n := range nodes {
		if n.ParentID == nil {
			roots = append(roots, n)
		} else {
			children[*n.ParentID] = append(children[*n.ParentID], n)
		}
	}
	bySiblingOrder := func(a, b Node) int {
		return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), cmp.Compare(a.ID.String(), b.ID.String()))
	}
	out := make([]Node, 0, len(nodes))
	var walk func([]Node)
	walk = func(level []Node) {
		slices.SortFunc(level, bySiblingOrder)
		for _, n := range level {
			out = append(out, n)
			walk(children[n.ID])
		}
	}
	walk(roots)
	return out
}

// Depth is the depth of a node with these ancestors: a root's is 1.
func Depth(ancestors []Ancestor) int {
	return len(ancestors) + 1
}

// Subtree is a node not deleted and its descendants not deleted, the node
// first, each at its level in the subtree: the node's is 1.
type Subtree []SubtreeNode

// SubtreeNode is a node of a subtree and its level in it.
type SubtreeNode struct {
	Node  Node
	Level int
}

// Height is how many levels of pages the subtree spans: a lone page's is
// 1, a lone attachment's 0. An attachment is no level (M7/P2 design 3.3):
// a page as deep as pages go holds attachments, and moves with them.
func (s Subtree) Height() int {
	h := 0
	for _, n := range s {
		if n.Node.Kind == KindPage {
			h = max(h, n.Level)
		}
	}
	return h
}

// Holds reports whether id is the subtree's node or one of its
// descendants: a parent the node cannot move under.
func (s Subtree) Holds(id uuid.UUID) bool {
	return slices.ContainsFunc(s, func(n SubtreeNode) bool { return n.Node.ID == id })
}
