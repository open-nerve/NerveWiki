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
// root, a point lookup a segment, as domain.FindPath finds it:
// page.not_found, a *domain.PathNotFound, for none, for an attachment
// where the path has a page or the other way round, which its hint tells,
// and for a path through an attachment.
// Between the lookups a concurrent move may make the chain no path of any
// one moment: a read takes it, its answer's path written by the names
// found (M9/P1 design 3.9.2).
func (p NodePaths) Find(ctx context.Context, notebookID uuid.UUID, path domain.Path) (Found, error) {
	n, ancestors, err := domain.FindPath(path, p.lookup(ctx, notebookID))
	if err != nil {
		return Found{}, err
	}
	return Found{Node: n, Ancestors: ancestors}, nil
}

// Walk looks keys up from the notebook's root, as domain.WalkPath does, in
// the transaction ctx carries when it carries one: the nodes found, from
// the root, up to the first key that names nothing; an *AttachmentOnPath
// for a key before the last that names an attachment. A write's use case
// walks in its unit, whose lock of the notebook's row keeps the chain as
// found (M9 design 4.8).
func (p NodePaths) Walk(ctx context.Context, notebookID uuid.UUID, keys []string) ([]domain.Node, error) {
	return domain.WalkPath(keys, p.lookup(ctx, notebookID))
}

// Paths is the paths of ids' nodes alive in notebookID, by id, as PathOf
// writes them: one statement whatever their number (PathStore.PathsOf). A
// node deleted, purged or of another notebook has none.
func (p NodePaths) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	return p.store.PathsOf(ctx, notebookID, ids)
}

// lookup is the store's point lookup in notebookID: the node keyed key
// under the last of chain, the notebook's root for none.
func (p NodePaths) lookup(ctx context.Context, notebookID uuid.UUID) domain.Lookup {
	return func(chain []domain.Node, key string) (domain.Node, bool, error) {
		if len(chain) == 0 {
			return p.store.RootByKey(ctx, notebookID, key)
		}
		return p.store.ChildByKey(ctx, notebookID, chain[len(chain)-1].ID, key)
	}
}
