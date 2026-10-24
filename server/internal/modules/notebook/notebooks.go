package notebook

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Mode is a notebook's mode (M10 design 4.3), which bootstrap converts to
// the modules' own, field by field.
type Mode = domain.Mode

// The two modes, for bootstrap's conversions: an alias carries no
// constants.
const (
	ModeStandard = domain.ModeStandard
	ModeLLMWiki  = domain.ModeLLMWiki
)

// State is a notebook's mode and source root, as the modules inside it
// read them (M10 design 4.3, 4.6): the zero id for none.
type State struct {
	Mode         Mode
	SourceRootID uuid.UUID
}

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
	// NameOf is the name of the notebook not deleted with id, unlocked, in
	// the caller's transaction when it has one, and whether there is one:
	// an export names its archive after it (M7/P5 design 3.9).
	NameOf(ctx context.Context, id uuid.UUID) (string, bool, error)
	// StateOf is the state of the notebook not deleted with id, unlocked,
	// in the caller's transaction when it has one, and whether there is
	// one: the reads that hold no lock (MCP's open_notebook; M10's list,
	// read, instructions, lint) see one side of a downgrade or the other.
	StateOf(ctx context.Context, id uuid.UUID) (State, bool, error)
	// ShareState is ShareByID with the state read under the lock, and
	// LockState LockByID's: a unit's mode does not change while it runs
	// (M10 design 4.6).
	ShareState(ctx context.Context, id uuid.UUID) (State, bool, error)
	LockState(ctx context.Context, id uuid.UUID) (State, bool, error)
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

func (n notebooks) NameOf(ctx context.Context, id uuid.UUID) (string, bool, error) {
	nb, err := n.store.FindNotebook(ctx, id)
	if ok, err := found(err); !ok || err != nil {
		return "", false, err
	}
	return nb.Name, true, nil
}

func (n notebooks) StateOf(ctx context.Context, id uuid.UUID) (State, bool, error) {
	nb, err := n.store.FindNotebook(ctx, id)
	if ok, err := found(err); !ok || err != nil {
		return State{}, false, err
	}
	return State{Mode: nb.Mode, SourceRootID: nb.SourceRootID}, true, nil
}

func (n notebooks) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	_, ok, err := n.ShareState(ctx, id)
	return ok, err
}

func (n notebooks) LockByID(ctx context.Context, id uuid.UUID) (bool, error) {
	_, ok, err := n.LockState(ctx, id)
	return ok, err
}

func (n notebooks) ShareState(ctx context.Context, id uuid.UUID) (State, bool, error) {
	if !postgres.InTx(ctx) {
		return State{}, false, errors.New("share the notebook row: not in a transaction, the lock would end with the statement")
	}
	mode, root, err := n.store.ShareNotebook(ctx, id)
	ok, err := found(err)
	if err != nil {
		return State{}, false, fmt.Errorf("share the notebook row: %w", err)
	}
	return State{Mode: mode, SourceRootID: root}, ok, nil
}

func (n notebooks) LockState(ctx context.Context, id uuid.UUID) (State, bool, error) {
	if !postgres.InTx(ctx) {
		return State{}, false, errors.New("lock the notebook row: not in a transaction, the lock would end with the statement")
	}
	nb, err := n.store.LockNotebook(ctx, id)
	ok, err := found(err)
	if err != nil {
		return State{}, false, fmt.Errorf("lock the notebook row: %w", err)
	}
	return State{Mode: nb.Mode, SourceRootID: nb.SourceRootID}, ok, nil
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
