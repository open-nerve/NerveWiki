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
	return resolve(t, from, nodeList(candidates), aliased)
}

// Resolve is Resolve among the pages s holds, those whose title key is one
// of t's LastKeys, or more: the others are none of t's.
func (s Suffixes) Resolve(t Target, from []Step, aliased map[string][]Node) Resolution {
	return resolve(t, from, s, aliased)
}

// resolve is Resolve among candidates.
func resolve(t Target, from []Step, candidates candidateSet, aliased map[string][]Node) Resolution {
	folder := from[:max(len(from)-1, 0)]
	form := t.form(candidates)
	if t.Relative {
		up := folder[:max(len(folder)-t.Up, 0)]
		if c, ok := candidates.exactly(up, form); ok {
			return Resolution{ID: c}
		}
		return Resolution{}
	}
	if c, ok := candidates.exactly(nil, form); ok {
		return Resolution{ID: c}
	}
	if t.Rooted {
		return Resolution{}
	}
	if ends := candidates.ending(form); len(ends) > 0 {
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

// A candidateSet is the pages a target may resolve to, as Resolve reads
// them: a list, or a list read once into Suffixes.
type candidateSet interface {
	// has tells whether a page's title key is key.
	has(key string) bool
	// exactly is the first page whose path is the steps of base, then keys,
	// if one is.
	exactly(base []Step, keys []string) (uuid.UUID, bool)
	// ending is the pages whose path is longer than keys and ends with them,
	// in their order.
	ending(keys []string) []Node
}

// nodeList is a candidateSet read as it is, page by page.
type nodeList []Node

func (ns nodeList) has(key string) bool {
	return slices.ContainsFunc(ns, func(c Node) bool { return c.key() == key })
}

func (ns nodeList) exactly(base []Step, keys []string) (uuid.UUID, bool) {
	for _, c := range ns {
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

func (ns nodeList) ending(keys []string) []Node {
	var ends []Node
	for _, c := range ns {
		if len(c.Path) > len(keys) && endsWith(c.Path, keys) {
			ends = append(ends, c)
		}
	}
	return ends
}

// Suffixes are pages read once by the ends of their paths, for the targets
// of a read to look up: each finds the pages whose path ends as it does,
// not every page of its title. Read page by page, a page of distinct paths
// to pages of one title cost the square of their number: 10,000 pages
// named x and a page of [[g1/x]] … [[g10000/x]] took seconds each time it
// was read (M6 closeout FA-I1).
type Suffixes struct {
	root *suffix
}

// suffix is the pages whose paths end with the keys from the root to it,
// the last key first: exact, whose path is just these keys, and longer,
// whose path has more; each in the order read.
type suffix struct {
	next   map[string]*suffix
	exact  []Node
	longer []Node
}

// NewSuffixes reads candidates by the ends of their paths. Each page is in
// as many suffixes as its path has steps.
func NewSuffixes(candidates []Node) Suffixes {
	root := &suffix{}
	for _, c := range candidates {
		at := root
		for i := len(c.Path) - 1; i >= 0; i-- {
			at = at.child(c.Path[i].Key)
			if i == 0 {
				at.exact = append(at.exact, c)
			} else {
				at.longer = append(at.longer, c)
			}
		}
	}
	return Suffixes{root: root}
}

// child is the suffix of s and key before it, made if it was not.
func (s *suffix) child(key string) *suffix {
	if s.next == nil {
		s.next = make(map[string]*suffix)
	}
	c, ok := s.next[key]
	if !ok {
		c = &suffix{}
		s.next[key] = c
	}
	return c
}

// at is the suffix of the steps of base, then keys; nil for none.
func (s Suffixes) at(base []Step, keys []string) *suffix {
	at := s.root
	for i := len(keys) - 1; i >= 0 && at != nil; i-- {
		at = at.next[keys[i]]
	}
	for i := len(base) - 1; i >= 0 && at != nil; i-- {
		at = at.next[base[i].Key]
	}
	return at
}

func (s Suffixes) has(key string) bool {
	_, ok := s.root.next[key]
	return ok
}

func (s Suffixes) exactly(base []Step, keys []string) (uuid.UUID, bool) {
	if at := s.at(base, keys); at != nil && len(at.exact) > 0 {
		return at.exact[0].ID, true
	}
	return uuid.UUID{}, false
}

func (s Suffixes) ending(keys []string) []Node {
	if at := s.at(nil, keys); at != nil {
		return at.longer
	}
	return nil
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
