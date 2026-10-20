package domain

import (
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Place is where a node is in its notebook's tree: its parent, nil at the
// root, and its name.
type Place struct {
	ParentID *uuid.UUID
	Name     string
}

// Change is what a page write unit did to a node, as the index reads it:
// where the node was before (nil: the unit created it) and after (nil: it
// deleted it), and the revision of the content the unit wrote with its
// facts (0: the content is untouched).
type Change struct {
	NodeID   uuid.UUID
	Before   *Place
	After    *Place
	Revision int
	Facts    Facts
}

// relocates reports whether c may change where links resolve by the tree:
// it creates its node, deletes it, renames it or moves it under another
// parent. A move among siblings does not.
func (c Change) relocates() bool {
	if c.Before == nil || c.After == nil {
		return c.Before != c.After
	}
	return c.renames() || !sameParent(c.Before.ParentID, c.After.ParentID)
}

// renames reports whether c gives its node another name where links
// resolve by it, which every node under it has on its path: another key, or
// the same key of another length (Node.length).
func (c Change) renames() bool {
	if c.Before == nil || c.After == nil {
		return false
	}
	before, after := c.Before.Name, c.After.Name
	return shared.TitleKey(before) != shared.TitleKey(after) || units(before) != units(after)
}

func sameParent(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// Recased is the pages a rename or a move gave a title that differs in
// case alone, its key the same, each by id with that title (M9/P3 design
// 3.7.2): none for none. A unit of one operation re-cases one at most; a
// batch's, merged, may re-case several.
type Recased map[uuid.UUID]string

// Relocation is what a rewrite of links follows of changes, an operation
// of a page write unit (M6/P4 design 2, 4.1): relocates when each keeps its
// node and writes no content, one renaming or moving it where links
// resolve by the tree, as a rename or a move does; and recased, each page
// a rename or a move changed the case of the title of only, under its
// parent or another (an MCP move's one step, M9/P2 design 3.5.3; a batch's
// several, M9/P3 design 3.7.2). Creating, deleting and writing a content
// rewrite nothing.
func Relocation(changes []Change) (relocates bool, recased Recased) {
	if len(changes) == 0 || slices.ContainsFunc(changes, func(c Change) bool {
		return c.Before == nil || c.After == nil || c.Revision != 0
	}) {
		return false, nil
	}
	for _, c := range changes {
		if c.Before.Name != c.After.Name && shared.TitleKey(c.Before.Name) == shared.TitleKey(c.After.Name) {
			if recased == nil {
				recased = Recased{}
			}
			recased[c.NodeID] = c.After.Name
		}
	}
	return slices.ContainsFunc(changes, Change.relocates), recased
}

// FormerParents are the parents the nodes changes moved had before, each
// once: a moved node's path before the move is its parent's then, which
// the move left as it was. The root is none.
func FormerParents(changes []Change) []uuid.UUID {
	var out []uuid.UUID
	for _, c := range changes {
		if c.Before != nil && c.After != nil && c.Before.ParentID != nil && !sameParent(c.Before.ParentID, c.After.ParentID) &&
			!slices.Contains(out, *c.Before.ParentID) {
			out = append(out, *c.Before.ParentID)
		}
	}
	return out
}

// PathBefore is n, at its path after changes, at its path before them
// (M9/P3 design 3.7.3): below the deepest node on its path that changes
// moved, the path parents gives that node's former parent, then its name
// before; each node under it, and each on a path no change moved, with
// its name before. parents are the former parents' paths as the unit
// began (FormerParents' nodes), read before any of its ops. A page off the
// nodes changes relocated has the path it has, a copy. It indexes changes
// for n alone: the paths of many nodes take BeforesOf's, built once.
func PathBefore(n Node, changes []Change, parents map[uuid.UUID][]Step) Node {
	return BeforesOf(changes).PathBefore(n, parents)
}

// Befores is changes by node, those that keep their node, for the paths
// before them of many nodes: a move of a folder has a change for each
// node under it, which an index built again for each link it reaches
// made a work of the links times the nodes (M9/P3 review B-I1).
type Befores map[uuid.UUID]Change

// BeforesOf is changes indexed by node for PathBefore, once.
func BeforesOf(changes []Change) Befores {
	byID := make(Befores, len(changes))
	for _, c := range changes {
		if c.Before != nil && c.After != nil {
			byID[c.NodeID] = c
		}
	}
	return byID
}

// PathBefore is n at its path before the changes byID indexes, as the
// package's PathBefore tells.
func (byID Befores) PathBefore(n Node, parents map[uuid.UUID][]Step) Node {
	from, path := 0, []Step(nil)
	for i := len(n.Path) - 1; i >= 0; i-- {
		if c, ok := byID[n.Path[i].ID]; ok && !sameParent(c.Before.ParentID, c.After.ParentID) {
			if c.Before.ParentID != nil {
				path = slices.Clone(parents[*c.Before.ParentID])
			}
			from = i
			break
		}
	}
	for _, s := range n.Path[from:] {
		if c, ok := byID[s.ID]; ok && c.Before.Name != c.After.Name {
			s = Step{ID: s.ID, Key: shared.TitleKey(c.Before.Name), Name: c.Before.Name}
		}
		path = append(path, s)
	}
	return Node{ID: n.ID, Path: path, Asset: n.Asset}
}

// Reach is the links a page write unit may resolve anew (M6/P3 design 3.4,
// steps 5 and 6): those whose target's keys meet Keys, those resolved to
// one of Targets, and those written in one of Sources.
type Reach struct {
	Keys    []string
	Targets []uuid.UUID
	Sources []uuid.UUID
}

// Add has r reach the nodes nodes too: their keys, the links to them and
// the links written in them.
func (r *Reach) Add(nodes ...Step) {
	for _, n := range nodes {
		r.Keys = append(r.Keys, n.Key)
		r.Targets = append(r.Targets, n.ID)
		r.Sources = append(r.Sources, n.ID)
	}
}

// Compact sorts r's keys and ids, each once.
func (r *Reach) Compact() {
	slices.Sort(r.Keys)
	r.Keys = slices.Compact(r.Keys)
	for _, ids := range []*[]uuid.UUID{&r.Targets, &r.Sources} {
		slices.SortFunc(*ids, uuid.UUID.Compare)
		*ids = slices.Compact(*ids)
	}
}

// Affected is what a unit's changes reach, but for the pages and
// attachments under the nodes they rename, which the caller reads and adds,
// and the aliases' keys, which the caller adds too. A unit that relocates a
// node reaches every node changed, by its keys before and after and by its
// id: a move lists the nodes under the one it moves, and a deletion those
// it deletes. One that relocates none reaches the pages whose content it
// writes, by their ids, as sources alone. It is false for a unit the index
// has nothing to do for: one that writes no content of a page it leaves,
// and relocates no node.
//
// The links a unit may resolve anew are those its changes may resolve
// otherwise (M6/P3 design 3.4; M7/P3 design 4.3): where a link resolves
// depends on the pages and attachments whose key is its target's last,
// their paths, its page's place and the aliases with their pages' paths. A
// node appearing, going or renamed has its keys; a path changes only for a
// node renamed or moved, with the nodes under it, whose links to them and
// keys are reached, and the keys of their aliases, which the caller adds; a
// page's place changes only when it moves, and its links are reached;
// aliases change only with a content, whose aliases' keys, those it drops
// and those it adds, the caller adds. A content written changes none of
// these but its own links and aliases.
func Affected(changes []Change) (r Reach, renamed []uuid.UUID, ok bool) {
	moves := slices.ContainsFunc(changes, Change.relocates)
	for _, c := range changes {
		written := c.Revision != 0 && c.After != nil
		ok = ok || written || moves
		switch {
		case moves:
			for _, p := range []*Place{c.Before, c.After} {
				if p != nil {
					r.Add(Step{ID: c.NodeID, Key: shared.TitleKey(p.Name)})
				}
			}
		case written:
			r.Sources = append(r.Sources, c.NodeID)
		}
		if c.renames() {
			renamed = append(renamed, c.NodeID)
		}
	}
	if !ok {
		return Reach{}, nil, false
	}
	r.Compact()
	return r, renamed, true
}
