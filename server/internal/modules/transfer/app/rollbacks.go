package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Rollbacks start an upgrade's rollback's job in the caller's transaction
// (M11 design 4.10): the wiki module's abandonment and its continuation.
// It logs nothing: its caller logs once it commits.
type Rollbacks struct {
	d      StartDeps
	action shared.Action
}

// NewRollbacks returns the starts, which decide action, the notebook
// module's notebook.change_mode, for the caller.
func NewRollbacks(d StartDeps, action shared.Action) Rollbacks {
	return Rollbacks{d: d, action: action}
}

// Start writes the rollback of the notebook's upgrade upgradeID, queued,
// as its caller from client asks, and enqueues it, in the caller's
// transaction, which holds the notebook's row FOR NO KEY UPDATE and has
// set the upgrade rolling back; it answers the job's id: 503 server_busy
// when MaxQueued jobs wait or run, transfer.busy when a job writes the
// notebook's tree. The job is named "upgrade".
func (r Rollbacks) Start(ctx context.Context, notebookID, upgradeID uuid.UUID, client domain.Client) (uuid.UUID, error) {
	job, err := r.d.queueBackground(ctx, r.action, domain.Job{NotebookID: notebookID, Kind: domain.KindRollback, Name: "upgrade",
		Client: client, UpgradeID: &upgradeID}, r.d.Queue.Rollback)
	if err != nil {
		return uuid.UUID{}, err
	}
	return job.ID, nil
}
