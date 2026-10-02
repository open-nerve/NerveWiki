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
