package domain

import (
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Rekey is nodes, a notebook's not deleted, with their title keys taken
// anew from their names by the current Unicode data (shared.TitleKey; a
// new Unicode version may change them, v0.1 design 3.5): those whose key
// changes, with their new keys. When siblings would share a key, it
// changes none and returns the groups of them, each group the siblings of
// one key, as nodes' order has them.
func Rekey(nodes []Node) (changed []Node, clashes [][]Node) {
	type sibling struct {
		parent uuid.UUID // the root's is zero
		key    string
	}
	groups := map[sibling][]Node{}
	var order []sibling
	for _, n := range nodes {
		s := sibling{key: shared.TitleKey(n.Name)}
		if n.ParentID != nil {
			s.parent = *n.ParentID
		}
		if _, ok := groups[s]; !ok {
			order = append(order, s)
		}
		groups[s] = append(groups[s], n)
		if s.key != n.NameKey {
			n.NameKey = s.key
			changed = append(changed, n)
		}
	}
	for _, s := range order {
		if len(groups[s]) > 1 {
			clashes = append(clashes, slices.Clone(groups[s]))
		}
	}
	if len(clashes) > 0 {
		return nil, clashes
	}
	return changed, nil
}
