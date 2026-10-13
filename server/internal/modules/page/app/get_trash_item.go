package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// GetTrashItem reads a trash item: GET /api/v0/nodes/{node_id}/trash-item
// (M8/P2 design 3.5), which the trash's preview reads.
type GetTrashItem struct {
	r TrashReads
}

// NewGetTrashItem returns the use case.
func NewGetTrashItem(r TrashReads) *GetTrashItem {
	return &GetTrashItem{r: r}
}

// Execute reads the trash item of the root id, as the trash lists it. The
// codes come in M8 design 4.14's order: a node that does not exist, or
// whose notebook is deleted or the caller has no role in (page.history:
// any role), is page.not_found; then a node alive page.not_in_trash, one
// in another root's item page.not_trash_root with that root, and an item
// past the time it could be restored in page.restore_expired. A read takes
// no lock and opens no transaction.
func (g *GetTrashItem) Execute(ctx context.Context, id uuid.UUID) (TrashItem, error) {
	n, err := readableNode(ctx, g.r.Notebooks, g.r.Nodes, g.r.Auth, id, domain.ActionHistory)
	if err != nil {
		return TrashItem{}, err
	}
	root, err := g.r.trashRoot(ctx, n, g.r.Clock.Now())
	if err != nil {
		return TrashItem{}, err
	}
	items, err := g.r.trashItems(ctx, []domain.TrashRoot{root})
	if err != nil {
		return TrashItem{}, err
	}
	return items[0], nil
}
