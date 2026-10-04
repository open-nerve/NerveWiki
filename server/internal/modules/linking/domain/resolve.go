package domain

import (
	"slices"
	"uuid"
)

// Step is a page on a path from a notebook's root: its id and title key.
type Step struct {
	ID  uuid.UUID
	Key string
}

// Node is a page a link may resolve to, with its path from the root, itself
// last.
type Node struct {
	ID   uuid.UUID
	Path []Step
}

// Resolution is where a link resolves to: a page, or none (a nil ID); and
// whether pages alike in every preference tied, the least id winning.
type Resolution struct {
	ID        uuid.UUID
	Ambiguous bool
}

// Resolve resolves t, written in the page whose path from the root is from
// (itself last), among candidates, the pages whose title key is one of t's
// LastKeys, and aliased, the pages with an alias whose key is one of them
// (M6/P3 design 2). The first step that finds a page decides:
//
//  1. a relative target: the page whose path is the source folder's, up
//     t.Up (no further than the root), then t's segments; or none;
//  2. a target from the root, or one that is exactly a page's path from the
//     root, a name alone too;
//  3. a page whose path ends with t's segments, whole: those in the source
//     folder's subtree first, then the fewest levels, then the least id;
//  4. for a name alone, a page with it as an alias, preferred as in 3.
//
// The source folder is the source page's parent, the root for a page at
// the root. Each step tries the target without its ".md", then with it.
func Resolve(t Target, from []Step, candidates, aliased []Node) Resolution {
	folder := from[:max(len(from)-1, 0)]
	if t.Relative {
		base := keysOf(folder[:max(len(folder)-t.Up, 0)])
		for _, form := range t.forms() {
			if id, ok := exactly(append(slices.Clone(base), form...), candidates); ok {
				return Resolution{ID: id}
			}
		}
		return Resolution{}
	}
	for _, form := range t.forms() {
		if id, ok := exactly(form, candidates); ok {
			return Resolution{ID: id}
		}
	}
	if t.Rooted {
		return Resolution{}
	}
	for _, form := range t.forms() {
		var ends []Node
		for _, c := range candidates {
			if len(c.Path) > len(form) && slices.Equal(keysOf(c.Path[len(c.Path)-len(form):]), form) {
				ends = append(ends, c)
			}
		}
		if len(ends) > 0 {
			return preferred(ends, folder)
		}
	}
	if len(t.Keys) == 1 && len(aliased) > 0 {
		return preferred(aliased, folder)
	}
	return Resolution{}
}

// exactly is the candidate whose path is keys, if one is.
func exactly(keys []string, candidates []Node) (uuid.UUID, bool) {
	for _, c := range candidates {
		if slices.Equal(keysOf(c.Path), keys) {
			return c.ID, true
		}
	}
	return uuid.UUID{}, false
}

// preferred is the one of nodes, which are not none, in folder's subtree if
// one is, then with the fewest levels, then with the least id: ambiguous
// when the id decides.
func preferred(nodes []Node, folder []Step) Resolution {
	in := nodes
	if len(folder) > 0 {
		var inside []Node
		for _, n := range nodes {
			if under(n, folder) {
				inside = append(inside, n)
			}
		}
		if len(inside) > 0 {
			in = inside
		}
	}
	least := slices.MinFunc(in, func(a, b Node) int { return len(a.Path) - len(b.Path) })
	var tied []Node
	for _, n := range in {
		if len(n.Path) == len(least.Path) {
			tied = append(tied, n)
		}
	}
	first := slices.MinFunc(tied, func(a, b Node) int { return a.ID.Compare(b.ID) })
	return Resolution{ID: first.ID, Ambiguous: len(tied) > 1}
}

// under tells whether n is in the subtree of folder's last page, below it.
func under(n Node, folder []Step) bool {
	i := len(folder) - 1
	return len(n.Path) > len(folder) && n.Path[i].ID == folder[i].ID
}

func keysOf(steps []Step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Key
	}
	return out
}
