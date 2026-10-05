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
// it creates its node, deletes it, renames it to another key or moves it
// under another parent. A move among siblings does not.
func (c Change) relocates() bool {
	if c.Before == nil || c.After == nil {
		return c.Before != c.After
	}
	return c.renames() || !sameParent(c.Before.ParentID, c.After.ParentID)
}

// renames reports whether c gives its node another title key, which every
// node under it has on its path.
func (c Change) renames() bool {
	return c.Before != nil && c.After != nil && shared.TitleKey(c.Before.Name) != shared.TitleKey(c.After.Name)
}

func sameParent(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
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

// Affected is what a unit's changes reach, but for the pages under the
// nodes they rename, which the caller reads and adds: every node changed,
// by its keys before and after and by its id. A move lists the nodes under
// the one it moves, and a deletion those it deletes. It is false for a
// unit the index has nothing to do for: one that writes no content of a
// page it leaves, and relocates no node.
//
// The links a unit may resolve anew are those its changes may resolve
// otherwise (M6/P3 design 3.4): where a link resolves depends on the pages
// whose key is its target's last, their paths, its page's place and the
// aliases. A page appearing, going or renamed has its keys; a path changes
// only for a node renamed or moved, with the nodes under it, whose links
// to them and keys are reached; a page's place changes only when it moves,
// and its links are reached; aliases change only with a content, whose
// aliases' keys the caller adds.
func Affected(changes []Change) (r Reach, renamed []uuid.UUID, ok bool) {
	for _, c := range changes {
		ok = ok || c.Revision != 0 && c.After != nil || c.relocates()
		for _, p := range []*Place{c.Before, c.After} {
			if p != nil {
				r.Add(Step{ID: c.NodeID, Key: shared.TitleKey(p.Name)})
			}
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
