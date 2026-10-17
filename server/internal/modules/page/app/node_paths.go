package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// NodePaths finds a notebook's nodes by their paths (M9 design 4.1, 4.8),
// unlocked; it decides nothing: its callers decided the notebook. P4 adds
// Paths, the paths of nodes by id, with linking, its one consumer.
type NodePaths struct{ store PathStore }

// NewNodePaths returns the paths over store.
func NewNodePaths(store PathStore) NodePaths {
	return NodePaths{store: store}
}

// Found is a node a path named and its ancestors from the root.
type Found struct {
	Node      domain.Node
	Ancestors []domain.Node
}

// Find is the node path names in notebookID, with its ancestors from the
// root, a point lookup a segment: page.not_found, a *domain.PathNotFound,
// for none, for an attachment where the path has a page or the other way
// round, which its hint tells, and for a path through an attachment.
// Between the lookups a concurrent move may make the chain no path of any
// one moment: a read takes it, its answer's path written by the names
// found (M9/P1 design 3.9.2).
func (p NodePaths) Find(ctx context.Context, notebookID uuid.UUID, path domain.Path) (Found, error) {
	missing := &domain.PathNotFound{Path: path.Written}
	var ancestors []domain.Node
	for i, key := range path.Keys {
		var n domain.Node
		var ok bool
		var err error
		if i == 0 {
			n, ok, err = p.store.RootByKey(ctx, notebookID, key)
		} else {
			n, ok, err = p.store.ChildByKey(ctx, notebookID, ancestors[i-1].ID, key)
		}
		switch {
		case err != nil:
			return Found{}, err
		case !ok:
			return Found{}, missing
		case i < len(path.Keys)-1:
			if n.Kind != domain.KindPage {
				return Found{}, missing // an attachment has no children
			}
			ancestors = append(ancestors, n)
			continue
		case n.Kind != path.Kind && n.Kind == domain.KindPage:
			missing.Hint = domain.PageHint(domain.PathOf(ancestors, n))
			return Found{}, missing
		case n.Kind != path.Kind:
			missing.Hint = domain.AttachmentHint(domain.PathOf(ancestors, n))
			return Found{}, missing
		}
		return Found{Node: n, Ancestors: ancestors}, nil
	}
	return Found{}, missing
}
