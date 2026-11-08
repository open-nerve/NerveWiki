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

// UpgradeStates writes a notebook's upgrade's columns and tells the mode's
// subscribers of each change, in the transaction ctx carries (M11 design
// 4.1): the wiki module's upgrade calls it, under its own transaction.
// Each method fails outside a transaction: a lock would end with its
// statement, and a write's subscribers would run in none.
type UpgradeStates struct {
	store       *postgresadapter.Store
	subscribers []ModeSubscriber
}

// NewUpgradeStates is over pool and the mode's subscribers, the ones
// bootstrap hands the downgrade (v0.1 design 12.1 item 6's exception).
func NewUpgradeStates(pool *pgxpool.Pool, subscribers []ModeSubscriber) *UpgradeStates {
	return &UpgradeStates{store: postgresadapter.New(pool), subscribers: subscribers}
}

// Lock locks the notebook not deleted id FOR NO KEY UPDATE until the
// transaction ends and answers its state; false for none. Its caller has
// locked the workspace's row FOR SHARE first.
func (u *UpgradeStates) Lock(ctx context.Context, id uuid.UUID) (State, bool, error) {
	if !postgres.InTx(ctx) {
		return State{}, false, errors.New("lock the notebook row for its upgrade: not in a transaction")
	}
	nb, err := u.store.LockNotebook(ctx, id)
	ok, err := found(err)
	if err != nil {
		return State{}, false, fmt.Errorf("lock the notebook row for its upgrade: %w", err)
	}
	return stateOf(nb), ok, nil
}

// Start sets the notebook's upgrade to up, which is upgrading, by up.By
// at up.At; the notebook is locked, standard, and has none. An id of
// zero, another state, or a notebook that changed no row is a defect:
// the zero id is no upgrade everywhere else (M11/P1 S1 review S1B-N1),
// and the caller judged the rest under the lock.
func (u *UpgradeStates) Start(ctx context.Context, id uuid.UUID, up Upgrade) error {
	if !postgres.InTx(ctx) {
		return errors.New("start the notebook's upgrade: not in a transaction")
	}
	if up.ID == (uuid.UUID{}) || up.State != UpgradeUpgrading {
		return fmt.Errorf("notebook: an upgrade's start of %+v, a defect: it starts upgrading, with an id", up)
	}
	n, err := u.store.StartUpgrade(ctx, id, up.ID, up.By, up.At)
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("notebook: the upgrade's start changed %d rows of notebook %s, a defect: its caller judged it standard and without one", n, id)
	}
	return nil
}

// Publish tells the mode's subscribers of c in their order, as the
// downgrade does: the change of an upgrade's state is one with From ==
// To.
func (u *UpgradeStates) Publish(ctx context.Context, c ModeChange) error {
	if !postgres.InTx(ctx) {
		return errors.New("publish the notebook's mode: not in a transaction")
	}
	return app.PublishMode(ctx, u.subscribers, c)
}
