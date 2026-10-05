package notebook

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
)

// Catalog lists the notebooks for the command line (nervewiki reindex,
// M6/P3 design 3.6).
type Catalog interface {
	// IDs is the notebooks not deleted, by id.
	IDs(ctx context.Context) ([]uuid.UUID, error)
}

// NewCatalog returns Catalog over pool alone.
func NewCatalog(pool *pgxpool.Pool) Catalog {
	return catalog{store: postgresadapter.New(pool)}
}

type catalog struct {
	store *postgresadapter.Store
}

func (c catalog) IDs(ctx context.Context) ([]uuid.UUID, error) {
	return c.store.NotebookIDs(ctx)
}
