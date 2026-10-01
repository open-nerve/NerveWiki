package workspace

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Workspaces is what the modules inside a workspace read of it (M3/P1
// design 3.5): the notebook module's port, which bootstrap wires to it.
type Workspaces interface {
	// FindBySlug returns the id of the workspace not deleted with slug,
	// unlocked, and whether there is one. A slug no workspace could have
	// reaches no query.
	FindBySlug(ctx context.Context, slug string) (uuid.UUID, bool, error)
	// ShareByID locks the workspace not deleted with id FOR SHARE until the
	// transaction ctx carries ends, and reports whether there is one: a
	// deletion committed while it waited leaves none. Outside a transaction
	// it fails, since the lock would end with the statement.
	ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
	// Slugs returns the slugs of the workspaces not deleted among ids, by
	// id, unlocked: rule two names a refusal's workspaces (M3/P3 design
	// 3.2), which the refused change holds locked.
	Slugs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// NewWorkspaces returns Workspaces over pool alone: bootstrap builds it
// before the modules that read it.
func NewWorkspaces(pool *pgxpool.Pool) Workspaces {
	return workspaces{store: postgresadapter.New(pool)}
}

type workspaces struct {
	store *postgresadapter.Store
}

func (w workspaces) FindBySlug(ctx context.Context, slug string) (uuid.UUID, bool, error) {
	if !domain.ValidSlug(slug) {
		return uuid.UUID{}, false, nil
	}
	return idOf(w.store.FindWorkspaceBySlug(ctx, slug))
}

func (w workspaces) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	if !postgres.InTx(ctx) {
		return false, errors.New("share the workspace row: not in a transaction, the lock would end with the statement")
	}
	_, ok, err := idOf(w.store.ShareWorkspaceByID(ctx, id))
	if err != nil {
		return false, fmt.Errorf("share the workspace row: %w", err)
	}
	return ok, nil
}

func (w workspaces) Slugs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	return w.store.WorkspaceSlugs(ctx, ids)
}

// idOf is the store's answer as Workspaces gives it: app.ErrNotFound is no
// workspace, not an error.
func idOf(w domain.Workspace, err error) (uuid.UUID, bool, error) {
	switch {
	case errors.Is(err, app.ErrNotFound):
		return uuid.UUID{}, false, nil
	case err != nil:
		return uuid.UUID{}, false, err
	}
	return w.ID, true, nil
}
