package notebook

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Notebooks is what the modules inside a notebook read of it (M4/P1 design
// 3.3): the page module's port, which bootstrap wires to it. A page write
// reads the notebook's workspace unlocked, locks the workspace's row FOR
// SHARE, then the notebook's, then decides.
type Notebooks interface {
	// WorkspaceOf returns the workspace of the notebook not deleted with
	// id, unlocked, and whether there is one.
	WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
	// ShareByID locks the notebook not deleted with id FOR SHARE until the
	// transaction ctx carries ends, and reports whether there is one: a
	// deletion committed while it waited leaves none. Outside a
	// transaction it fails, since the lock would end with the statement.
	ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
	// LockByID is ShareByID FOR NO KEY UPDATE: a write that changes the
	// notebook's tree of pages.
	LockByID(ctx context.Context, id uuid.UUID) (bool, error)
}

// NewNotebooks returns Notebooks over pool alone: bootstrap builds it
// before the modules that read it.
func NewNotebooks(pool *pgxpool.Pool) Notebooks {
	return notebooks{store: postgresadapter.New(pool)}
}

type notebooks struct {
	store *postgresadapter.Store
}

func (n notebooks) WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	nb, err := n.store.FindNotebook(ctx, id)
	if ok, err := found(err); !ok || err != nil {
		return uuid.UUID{}, false, err
	}
	return nb.WorkspaceID, true, nil
}

func (n notebooks) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	if !postgres.InTx(ctx) {
		return false, errors.New("share the notebook row: not in a transaction, the lock would end with the statement")
	}
	ok, err := found(n.store.ShareNotebook(ctx, id))
	if err != nil {
		return false, fmt.Errorf("share the notebook row: %w", err)
	}
	return ok, nil
}

func (n notebooks) LockByID(ctx context.Context, id uuid.UUID) (bool, error) {
	if !postgres.InTx(ctx) {
		return false, errors.New("lock the notebook row: not in a transaction, the lock would end with the statement")
	}
	_, err := n.store.LockNotebook(ctx, id)
	ok, err := found(err)
	if err != nil {
		return false, fmt.Errorf("lock the notebook row: %w", err)
	}
	return ok, nil
}

// found is the store's answer as Notebooks gives it: app.ErrNotFound is no
// notebook, not an error.
func found(err error) (bool, error) {
	switch {
	case errors.Is(err, app.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}
