package notebook

import (
	"context"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// VisibleNotebooks lists the notebooks an account sees: the event stream's
// port (M5 design 4.10), which bootstrap wires to it.
type VisibleNotebooks interface {
	// VisibleIn lists the ids of the notebooks not deleted of workspaceID
	// that userID, of role there, has a role in, unlocked: the notebook
	// list's rule, shared.EffectiveNotebookRole, which the access module
	// decides by too.
	VisibleIn(ctx context.Context, workspaceID, userID uuid.UUID, role shared.WorkspaceRole) ([]uuid.UUID, error)
}

// NewVisibleNotebooks returns VisibleNotebooks over pool alone.
func NewVisibleNotebooks(pool *pgxpool.Pool) VisibleNotebooks {
	return visible{store: postgresadapter.New(pool)}
}

type visible struct {
	store *postgresadapter.Store
}

func (v visible) VisibleIn(ctx context.Context, workspaceID, userID uuid.UUID, role shared.WorkspaceRole) ([]uuid.UUID, error) {
	listed, err := v.store.ListNotebooks(ctx, workspaceID, userID, shared.ReachedByAccess(role))
	if err != nil {
		return nil, fmt.Errorf("notebook: the notebooks %s sees in %s: %w", userID, workspaceID, err)
	}
	ids := make([]uuid.UUID, len(listed))
	for i, l := range listed {
		ids[i] = l.Notebook.ID
	}
	return ids, nil
}
