package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Cancel cancels a job (M7/P5 design 3.11): a queued one at once, a
// running one at its next heartbeat. It writes the job's row alone, never
// the notebook's: a running job's steps and a cancel make no cycle.
type Cancel struct {
	tx    shared.TxManager
	rows  Rows
	views views
	clock Clock
}

// CancelDeps are what Cancel needs.
type CancelDeps struct {
	Tx         shared.TxManager
	Rows       Rows
	Authorizer shared.Authorizer
	Notebooks  Notebooks
	Names      Names
	Signer     Signer
	Clock      Clock
}

// NewCancel returns the use case.
func NewCancel(d CancelDeps) *Cancel {
	return &Cancel{tx: d.Tx, rows: d.Rows, clock: d.Clock,
		views: views{auth: d.Authorizer, notebooks: d.Notebooks, names: d.Names, signer: d.Signer, clock: d.Clock}}
}

// Run cancels the job id for its starter or the notebook's admin
// (transfer.cancel), and answers it as it is then: transfer.not_found as
// Reads.Get; transfer.not_cancellable for a job that has ended. A running
// job's cancel asked again keeps the first time.
func (c *Cancel) Run(ctx context.Context, id uuid.UUID) (JobView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return JobView{}, err
	}
	var j domain.Job
	err = c.tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := c.rows.LockJob(ctx, id)
		if errors.Is(err, ErrNoRow) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := c.views.sees(ctx, actor, domain.ActionCancel, locked); err != nil {
			return err
		}
		now := c.clock.Now()
		switch locked.State {
		case domain.StateQueued:
			if _, err := c.rows.CancelQueued(ctx, id, now, domain.Report{}); err != nil {
				return err
			}
		case domain.StateRunning:
			if _, err := c.rows.RequestCancel(ctx, id, now); err != nil {
				return err
			}
		default:
			return domain.ErrNotCancellable
		}
		j, err = c.rows.FindJob(ctx, id)
		return err
	})
	if err != nil {
		return JobView{}, err
	}
	got, err := c.views.of(ctx, []domain.Job{j})
	if err != nil {
		return JobView{}, err
	}
	return got[0], nil
}
