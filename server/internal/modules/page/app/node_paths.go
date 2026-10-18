package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// NodePaths finds a notebook's nodes by their paths (M9 design 4.1, 4.8),
// and writes the paths of nodes by id (M9/P2 design 3.7), unlocked or in
// the transaction ctx carries; it decides nothing: its callers decided
// the notebook.
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
// round, which its hint tells, and for a path through an attachment. A
// page's path that names nothing looks its last segment up once more,
// ".md" and all: a page titled so is hinted.
// Between the lookups a concurrent move may make the chain no path of any
// one moment: a read takes it, its answer's path written by the names
// found (M9/P1 design 3.9.2).
func (p NodePaths) Find(ctx context.Context, notebookID uuid.UUID, path domain.Path) (Found, error) {
	missing := &domain.PathNotFound{Path: path.Written}
	var ancestors []domain.Node
	for i, key := range path.Keys {
		n, ok, err := p.child(ctx, notebookID, ancestors, key)
		switch {
		case err != nil:
			return Found{}, err
		case !ok && i == len(path.Keys)-1 && path.Kind == domain.KindPage:
			return Found{}, p.titled(ctx, notebookID, ancestors, key, missing)
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

// titled is missing, hinted when the parent of ancestors' end, the root
// for none, has a page titled key with ".md", the path's last segment as
// it was written.
func (p NodePaths) titled(ctx context.Context, notebookID uuid.UUID, ancestors []domain.Node, key string,
	missing *domain.PathNotFound,
) error {
	n, ok, err := p.child(ctx, notebookID, ancestors, key+".md")
	switch {
	case err != nil:
		return err
	case ok && n.Kind == domain.KindPage:
		missing.Hint = domain.TitleHint(domain.PathOf(ancestors, n))
	}
	return missing
}

// Walk looks keys up from the notebook's root, one segment after another
// as Find does, in the transaction ctx carries when it carries one: the
// nodes found, from the root, up to the first key that names nothing (all
// of them when each names one). A key before the last that names an
// attachment is an *AttachmentOnPath of its segment: an attachment holds
// no nodes; the nodes found end with it then. A write's use case walks in
// its unit, whose lock of the notebook's row keeps the chain as found
// (M9 design 4.8), so no hint is looked up.
func (p NodePaths) Walk(ctx context.Context, notebookID uuid.UUID, keys []string) ([]domain.Node, error) {
	var found []domain.Node
	for i, key := range keys {
		n, ok, err := p.child(ctx, notebookID, found, key)
		switch {
		case err != nil:
			return nil, err
		case !ok:
			return found, nil
		}
		found = append(found, n)
		if i < len(keys)-1 && n.Kind != domain.KindPage {
			return found, &domain.AttachmentOnPath{Segment: i + 1}
		}
	}
	return found, nil
}

// Paths is the paths of ids' nodes alive in notebookID, by id, as PathOf
// writes them: one statement whatever their number (PathStore.PathsOf). A
// node deleted, purged or of another notebook has none.
func (p NodePaths) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	return p.store.PathsOf(ctx, notebookID, ids)
}

// child is the node keyed key under the last of chain, the notebook's root
// for none: one point lookup.
func (p NodePaths) child(ctx context.Context, notebookID uuid.UUID, chain []domain.Node, key string) (domain.Node, bool, error) {
	if len(chain) == 0 {
		return p.store.RootByKey(ctx, notebookID, key)
	}
	return p.store.ChildByKey(ctx, notebookID, chain[len(chain)-1].ID, key)
}
