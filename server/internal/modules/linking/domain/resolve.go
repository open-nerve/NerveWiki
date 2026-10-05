package domain

import (
	"slices"
	"unicode/utf16"
	"uuid"
)

// Step is a page on a path from a notebook's root: its id, title key and
// name.
type Step struct {
	ID   uuid.UUID
	Key  string
	Name string
}

// Node is a page a link may resolve to, with its path from the root, itself
// last.
type Node struct {
	ID   uuid.UUID
	Path []Step
}

// key is n's title key.
func (n Node) key() string {
	return n.Path[len(n.Path)-1].Key
}

// length is the length of n's path as Obsidian measures it in an export,
// the names and the '/' between them in UTF-16 code units (JavaScript's
// String.length): the ".md" every page's ends with left out.
func (n Node) length() int {
	l := len(n.Path) - 1
	for _, s := range n.Path {
		l += units(s.Name)
	}
	return l
}

// units is the length of s in UTF-16 code units.
func units(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// Resolution is where a link resolves to: a page, or none (a nil ID); and
// whether pages alike in every preference tied, the least id winning.
type Resolution struct {
	ID        uuid.UUID
	Ambiguous bool
}

// Resolve resolves t, written in the page whose path from the root is from
// (itself last), among candidates, the pages whose title key is one of t's
// LastKeys, and aliased, the pages with an alias, by its key, whose key is
// one of them (M6/P3 design 2). It reads t in one form (Target.form), and
// the first step that finds a page decides:
//
//  1. a relative target: the page whose path is the source folder's, up
//     t.Up (no further than the root), then t's segments; or none;
//  2. a target from the root, or one that is exactly a page's path from the
//     root, a name alone too;
//  3. a page whose path ends with t's segments, whole: those in the source
//     folder's subtree, its own page included, first, then the shortest
//     path (Node.length), then the least id;
//  4. for a name alone, a page with it as an alias, without the ".md" and
//     then with it, preferred as in 3.
//
// The source folder is the source page's parent, the root for a page at
// the root.
func Resolve(t Target, from []Step, candidates []Node, aliased map[string][]Node) Resolution {
	folder := from[:max(len(from)-1, 0)]
	form := t.form(candidates)
	if t.Relative {
		up := folder[:max(len(folder)-t.Up, 0)]
		if c, ok := exactly(up, form, candidates); ok {
			return Resolution{ID: c}
		}
		return Resolution{}
	}
	if c, ok := exactly(nil, form, candidates); ok {
		return Resolution{ID: c}
	}
	if t.Rooted {
		return Resolution{}
	}
	var ends []Node
	for _, c := range candidates {
		if len(c.Path) > len(form) && endsWith(c.Path, form) {
			ends = append(ends, c)
		}
	}
	if len(ends) > 0 {
		return preferred(ends, folder)
	}
	if t.ByAlias() {
		for _, key := range t.LastKeys() {
			if nodes := aliased[key]; len(nodes) > 0 {
				return preferred(nodes, folder)
			}
		}
	}
	return Resolution{}
}

// exactly is the candidate whose path is the steps of base, then keys, if
// one is.
func exactly(base []Step, keys []string, candidates []Node) (uuid.UUID, bool) {
	for _, c := range candidates {
		if len(c.Path) != len(base)+len(keys) || !endsWith(c.Path, keys) {
			continue
		}
		if !slices.EqualFunc(c.Path[:len(base)], base, func(a, b Step) bool { return a.Key == b.Key }) {
			continue
		}
		return c.ID, true
	}
	return uuid.UUID{}, false
}

// endsWith tells whether path ends with steps of keys, which is no longer.
func endsWith(path []Step, keys []string) bool {
	tail := path[len(path)-len(keys):]
	for i, k := range keys {
		if tail[i].Key != k {
			return false
		}
	}
	return true
}

// preferred is the one of nodes, which are not none, in folder's subtree if
// one is, folder's own page included, then with the shortest path, then
// with the least id: ambiguous when the id decides.
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
	shortest := slices.MinFunc(in, func(a, b Node) int { return a.length() - b.length() }).length()
	var tied []Node
	for _, n := range in {
		if n.length() == shortest {
			tied = append(tied, n)
		}
	}
	first := slices.MinFunc(tied, func(a, b Node) int { return a.ID.Compare(b.ID) })
	return Resolution{ID: first.ID, Ambiguous: len(tied) > 1}
}

// under tells whether n is folder's last page or below it. A page is its
// folder in an export (A.md beside A/), and Obsidian, which compares paths
// as strings, counts A.md in A too: from under A, [[A]] is A, not a deeper
// page named so, nor the source page itself.
func under(n Node, folder []Step) bool {
	i := len(folder) - 1
	return len(n.Path) >= len(folder) && n.Path[i].ID == folder[i].ID
}
