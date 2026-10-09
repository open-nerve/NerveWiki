package linking

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
)

// LinkedPages tells an export which of its pages without content links
// lead to (M7/P5 design 3.8), in the caller's transaction: the export's
// snapshot. Built on the pool alone.
type LinkedPages struct {
	store *postgresadapter.Store
}

// NewLinkedPages returns LinkedPages over pool.
func NewLinkedPages(pool *pgxpool.Pool) LinkedPages {
	return LinkedPages{store: postgresadapter.New(pool)}
}

// Linked is those of targets, pages of the notebook, that a link of a page
// of sources resolves to: the index's links, a page's first 10,000 (v0.1
// design 13.1, item 31), its properties' among them.
func (l LinkedPages) Linked(ctx context.Context, sources, targets []uuid.UUID) ([]uuid.UUID, error) {
	return l.store.LinkedPages(ctx, sources, targets)
}
