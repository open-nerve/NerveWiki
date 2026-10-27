package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The starts of the background restores and reverts (M8 design 4.10;
// M8/P4 design 3.6): the page module checks what the job will do, as its
// request would, unlocked; then the notebook is claimed in this process,
// as an import's upload claims it, until the job's row is written or the
// start fails, so that no import uploads its archive only to meet the
// new job; then, the jobs' creations one at a time, the row is written,
// queued, and its River job enqueued, in one transaction.

// BackgroundDeps are what the starts need besides StartDeps: the page
// module's tree, and its actions a restore and a revert decide on, which
// bootstrap hands from the page module.
type BackgroundDeps struct {
	Tree          Tree
	RestoreAction shared.Action
	RevertAction  shared.Action
	// Batch is the worker's batch configuration, also used by the
	// revert's nominal preflight. Zero takes the import's bounds.
	Batch Batch
}

// RestoreRequest is a background restore's: the trash item's root, in the
// notebook, and where the request came from.
type RestoreRequest struct {
	NotebookID uuid.UUID
	NodeID     uuid.UUID
	Client     domain.Client
}

// RevertRequest is a background revert's: the changeset, of the notebook,
// the revert's message (nil: none), and where the request came from.
type RevertRequest struct {
	NotebookID  uuid.UUID
	ChangesetID uuid.UUID
	Message     *string
	Client      domain.Client
}

// StartTrashRestore starts a background restore of a trash item: POST
// /api/v0/notebooks/{notebook_id}/trash-restores.
type StartTrashRestore struct {
	d StartDeps
	b BackgroundDeps
}

// NewStartTrashRestore returns the use case.
func NewStartTrashRestore(d StartDeps, b BackgroundDeps) *StartTrashRestore {
	return &StartTrashRestore{d: d, b: b}
}

// Execute starts the restore req asks for, and answers its job, in M8
// design 4.14's order: the page module's check (notebook.not_found,
// forbidden, page.not_found, page.not_in_trash, page.not_trash_root,
// page.restore_expired); then the notebook claimed, transfer.busy when an
// import's upload or another start holds it, or a job that writes its
// tree is queued or running; 503 server_busy when MaxQueued jobs wait or
// run; then the row, decided again. The job is named as the item.
func (s *StartTrashRestore) Execute(ctx context.Context, req RestoreRequest) (JobView, error) {
	name, err := s.b.Tree.CheckRestore(ctx, req.NotebookID, req.NodeID)
	if err != nil {
		return JobView{}, err
	}
	release, err := s.d.claim(ctx, req.NotebookID)
	if err != nil {
		return JobView{}, err
	}
	defer release()
	root := req.NodeID
	return s.d.startBackground(ctx, s.b.RestoreAction, domain.Job{NotebookID: req.NotebookID, RootID: &root, Kind: domain.KindTrashRestore,
		Name: name, Client: req.Client}, s.d.Queue.TrashRestore)
}

// StartRevert starts a background revert of a changeset: POST
// /api/v0/notebooks/{notebook_id}/reverts.
type StartRevert struct {
	d StartDeps
	b BackgroundDeps
}

// NewStartRevert returns the use case.
func NewStartRevert(d StartDeps, b BackgroundDeps) *StartRevert {
	return &StartRevert{d: d, b: b}
}

// Execute starts the revert req asks for, and answers its job, in M8
// design 4.14's order: the page module's check of the revert, unlocked,
// as revertChangeset's, which claims the notebook (transfer.busy, as
// StartTrashRestore) right after the changeset's items are counted, past
// MaxJobItems page.revert_too_large, before it reads anything else; its
// conflicts, the versions the purge took, and page.locked for a page it
// deletes or writes back that an edit session holds; then 503
// server_busy; then the row, decided again. The job is named as the
// changeset's kind.
func (s *StartRevert) Execute(ctx context.Context, req RevertRequest) (JobView, error) {
	release := func() {}
	defer func() { release() }()
	bounds := s.b.Batch.bounds()
	kind, err := s.b.Tree.CheckRevert(ctx, req.NotebookID, req.ChangesetID, req.Message, func(ctx context.Context) error {
		var err error
		release, err = s.d.claim(ctx, req.NotebookID)
		if err != nil {
			release = func() {}
		}
		return err
	}, func(steps []JobStep) [][2]int {
		return nominalBatches(steps, bounds)
	})
	if err != nil {
		return JobView{}, err
	}
	target := req.ChangesetID
	return s.d.startBackground(ctx, s.b.RevertAction, domain.Job{NotebookID: req.NotebookID, Kind: domain.KindRevert, Name: kind,
		Client: req.Client, TargetChangesetID: &target, Message: req.Message}, s.d.Queue.Revert)
}

// claim claims the notebook for a start in this process, as an import's
// Check does for its upload, but with no bytes and not admitted: it
// counts in no queue. transfer.busy when an upload or another start holds
// it, or the notebook has a job that writes its tree queued or running.
// It answers the claim's release.
func (d StartDeps) claim(ctx context.Context, notebookID uuid.UUID) (func(), error) {
	d.Uploads.checking.Lock()
	defer d.Uploads.checking.Unlock()
	if !d.Uploads.claim(notebookID, 0) {
		return nil, domain.ErrWriting
	}
	if err := d.writing(ctx, notebookID); err != nil {
		d.Uploads.release(notebookID)
		return nil, err
	}
	return func() { d.Uploads.release(notebookID) }, nil
}

// startBackground writes job's row, queued, and enqueues it, as its
// creator from its client asks: 503 server_busy when MaxQueued jobs wait
// or run; then, under the workspace's row and the notebook's, FOR SHARE,
// action decided again, and the jobs' creations one at a time, room in
// the queue and no job that writes the notebook's tree; the changeset it
// will write decided (M8/P4 design 3.2).
func (d StartDeps) startBackground(ctx context.Context, action shared.Action, job domain.Job,
	enqueue func(ctx context.Context, id uuid.UUID) error,
) (JobView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return JobView{}, err
	}
	if err := d.room(ctx, job.NotebookID, false); err != nil {
		return JobView{}, err
	}
	workspaceID, err := d.workspaceOf(ctx, job.NotebookID)
	if err != nil {
		return JobView{}, err
	}
	err = d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := d.lock(ctx, actor, workspaceID, job.NotebookID, action); err != nil {
			return err
		}
		if err := d.Rows.LockQueue(ctx); err != nil {
			return err
		}
		if err := d.room(ctx, job.NotebookID, true); err != nil {
			return err
		}
		if err := d.writing(ctx, job.NotebookID); err != nil {
			return err
		}
		changeset := uuid.NewV7()
		job.ID, job.State, job.CreatedBy, job.CreatedAt, job.ChangesetID = uuid.NewV7(), domain.StateQueued, actor.UserID, d.Clock.Now(),
			&changeset
		if err := d.Rows.CreateJob(ctx, job); err != nil {
			return err
		}
		return enqueue(ctx, job.ID)
	})
	if err != nil {
		return JobView{}, err
	}
	d.Logger.InfoContext(ctx, string(job.Kind)+" queued", logged(job)...)
	got, err := views{names: d.Names, signer: d.Signer, clock: d.Clock}.of(ctx, []domain.Job{job})
	if err != nil {
		return JobView{}, err
	}
	return got[0], nil
}

// logged are a background job's start's log attributes, with what it
// starts on: no name nor message (v0.1 design 13.1, item 10).
func logged(job domain.Job) []any {
	attrs := jobAttrs(job)
	if job.RootID != nil {
		attrs = append(attrs, slog.String("root_id", job.RootID.String()))
	}
	if job.TargetChangesetID != nil {
		attrs = append(attrs, slog.String("target_changeset_id", job.TargetChangesetID.String()))
	}
	return attrs
}
