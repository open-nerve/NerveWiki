package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// PageView is a page as the API answers it: the node, its ancestors from
// the root down to its parent, and what it tells of its content.
type PageView struct {
	Node      domain.Node
	Ancestors []domain.Ancestor
	Content   ContentMeta
}

// readPage reads the page id not deleted: page.not_found for none, or for
// a node that is no page.
func readPage(ctx context.Context, nodes Nodes, id uuid.UUID) (PageView, error) {
	n, err := nodes.FindNode(ctx, id)
	switch {
	case err != nil:
		return PageView{}, found(err, domain.ErrNotFound)
	case n.Kind != domain.KindPage:
		return PageView{}, domain.ErrNotFound
	}
	return viewOf(ctx, nodes, n)
}

// viewOf is the page n with its ancestors and its content's meta.
func viewOf(ctx context.Context, nodes Nodes, n domain.Node) (PageView, error) {
	ancestors, err := nodes.Ancestors(ctx, n.ID)
	if err != nil {
		return PageView{}, err
	}
	meta, err := nodes.ContentMeta(ctx, n.ID)
	if err != nil {
		// Unlocked, the page may be deleted since its node was read: it is
		// as if it was then (M4–M5 Codex review R4).
		return PageView{}, found(err, domain.ErrNotFound)
	}
	return PageView{Node: n, Ancestors: ancestors, Content: meta}, nil
}
