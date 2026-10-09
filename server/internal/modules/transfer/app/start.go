package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// StartExport starts the export of a notebook, or of a page and its
// subtree (M7/P5 design 3.7): it writes the job's row, queued, and
// enqueues its River job in the same transaction.
type StartExport struct {
	d StartDeps
}

// StartDeps are what StartExport and StartImport need.
type StartDeps struct {
	Tx         shared.TxManager
	Authorizer shared.Authorizer
	Workspaces Workspaces
	Notebooks  Notebooks
	Nodes      Nodes
	Rows       Rows
	Archives   Archives
	Queue      Queue
	Names      Names
	Signer     Signer
	Clock      Clock
	Logger     *slog.Logger
	// Uploads are the imports' uploads under way, the same for the
	// exports and the imports.
	Uploads *Uploads
	// MaxQueued is transfer.max_queued, MinFree storage.min_free_bytes,
	// ImportMaxBytes transfer.import_max_bytes.
	MaxQueued      int
	MinFree        int64
	ImportMaxBytes int64
}

// NewStartExport returns the use case.
func NewStartExport(d StartDeps) *StartExport {
	return &StartExport{d: d}
}

// Run starts the export of the notebook notebookID, or of the page root
// and its subtree when root is set, for the caller from client, and
// answers its job. Under the
// workspace's row and the notebook's, FOR SHARE, it decides
// transfer.export (notebook.not_found); root is a page of the notebook not
// deleted (page.not_found); then, the jobs' creations one at a time, the
// jobs queued or running, with the imports' uploads under way, are fewer
// than MaxQueued (503 server_busy), the caller has no export queued or
// running in the notebook (transfer.busy), and the store has room, the
// uploads' declared bytes counted (507 storage_full).
func (s *StartExport) Run(ctx context.Context, notebookID uuid.UUID, root *uuid.UUID, client domain.Client) (JobView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return JobView{}, err
	}
	workspaceID, ok, err := s.d.Notebooks.WorkspaceOf(ctx, notebookID)
	switch {
	case err != nil:
		return JobView{}, err
	case !ok:
		return JobView{}, domain.ErrNotebookNotFound
	}
	var job domain.Job
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.d.lock(ctx, actor, workspaceID, notebookID, domain.ActionExport); err != nil {
			return err
		}
		name, err := s.nameOf(ctx, notebookID, root)
		if err != nil {
			return err
		}
		if err := s.admit(ctx, notebookID, actor.UserID); err != nil {
			return err
		}
		job = domain.Job{ID: uuid.NewV7(), NotebookID: notebookID, RootID: root, Kind: domain.KindExport, State: domain.StateQueued, Name: name,
			CreatedBy: actor.UserID, Client: client, CreatedAt: s.d.Clock.Now()}
		if err := s.d.Rows.CreateJob(ctx, job); err != nil {
			return err
		}
		return s.d.Queue.Export(ctx, job.ID)
	})
	if err != nil {
		return JobView{}, err
	}
	s.d.Logger.InfoContext(ctx, "export queued", slog.String("job_id", job.ID.String()), slog.String("notebook_id", notebookID.String()),
		slog.String("user_id", actor.UserID.String()), slog.String("client", string(client)))
	// A queued job's view has no address: no TTL is needed.
	got, err := views{names: s.d.Names, signer: s.d.Signer, clock: s.d.Clock}.of(ctx, []domain.Job{job})
	if err != nil {
		return JobView{}, err
	}
	return got[0], nil
}

// lock locks the workspace's row and the notebook's FOR SHARE, then
// decides action: a deletion committed meanwhile leaves none.
func (d StartDeps) lock(ctx context.Context, actor shared.Actor, workspaceID, notebookID uuid.UUID, action shared.Action) error {
	if ok, err := d.Workspaces.ShareByID(ctx, workspaceID); err != nil || !ok {
		return orNotFound(err)
	}
	if ok, err := d.Notebooks.ShareByID(ctx, notebookID); err != nil || !ok {
		return orNotFound(err)
	}
	return d.authorize(ctx, actor, workspaceID, notebookID, action)
}

// authorize decides action on the notebook: notebook.not_found for a
// caller who cannot see it.
func (d StartDeps) authorize(ctx context.Context, actor shared.Actor, workspaceID, notebookID uuid.UUID, action shared.Action) error {
	_, err := d.Authorizer.Authorize(ctx, actor, action, shared.Target{WorkspaceID: workspaceID, NotebookID: notebookID})
	if errors.Is(err, shared.ErrNotVisible) {
		return domain.ErrNotebookNotFound
	}
	return err
}

// orNotFound is err, or notebook.not_found when there is none.
func orNotFound(err error) error {
	if err != nil {
		return err
	}
	return domain.ErrNotebookNotFound
}

// nameOf is the name of what is exported: the page root's, or the
// notebook's.
func (s *StartExport) nameOf(ctx context.Context, notebookID uuid.UUID, root *uuid.UUID) (string, error) {
	if root != nil {
		name, ok, err := s.d.Nodes.Page(ctx, notebookID, *root)
		if err == nil && !ok {
			err = domain.ErrRootNotFound
		}
		return name, err
	}
	name, ok, err := s.d.Notebooks.NameOf(ctx, notebookID)
	if err == nil && !ok {
		err = domain.ErrNotebookNotFound
	}
	return name, err
}

// admit admits one more job, the jobs' creations one at a time: fewer
// than MaxQueued wait or run, none of the caller's exports of the
// notebook, and room in the store.
func (s *StartExport) admit(ctx context.Context, notebookID, userID uuid.UUID) error {
	if err := s.d.Rows.LockQueue(ctx); err != nil {
		return err
	}
	if err := s.d.room(ctx, uuid.Nil()); err != nil {
		return err
	}
	busy, err := s.d.Rows.Exporting(ctx, notebookID, userID)
	if err != nil {
		return err
	}
	if busy {
		return domain.ErrBusy
	}
	return s.d.free(ctx, 0, uuid.Nil())
}

// room is 503 server_busy when the jobs queued or running, and the
// uploads under way into notebooks but except, are MaxQueued.
func (d StartDeps) room(ctx context.Context, except uuid.UUID) error {
	active, err := d.Rows.CountActive(ctx)
	if err != nil {
		return err
	}
	if uploading, _ := d.Uploads.others(except); active+uploading >= d.MaxQueued {
		return domain.ErrQueueFull
	}
	return nil
}

// free is 507 storage_full when the store's disk would keep less than
// MinFree once need bytes, and those the uploads under way into notebooks
// but except declare, are written.
func (d StartDeps) free(ctx context.Context, need int64, except uuid.UUID) error {
	free, err := d.Archives.Free(ctx)
	if err != nil {
		return err
	}
	if _, uploading := d.Uploads.others(except); free-need-uploading < d.MinFree {
		return domain.ErrStorageFull
	}
	return nil
}
