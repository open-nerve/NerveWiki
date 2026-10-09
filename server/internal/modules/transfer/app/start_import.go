package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// StartImport starts an import into a notebook, at its root or under a
// page (M7/P6 design 3.9), in three steps, as an attachment's upload
// does: Check before the file is read, Store as it is, Create once the
// request's body ended, which writes the job's row, queued, and enqueues
// its River job in one transaction.
type StartImport struct {
	d       StartDeps
	uploads *uploads
}

// NewStartImport returns the use case.
func NewStartImport(d StartDeps) *StartImport {
	return &StartImport{d: d, uploads: &uploads{notebooks: map[uuid.UUID]bool{}}}
}

// uploads are the notebooks an import's archive is being uploaded into,
// from Check to Release, in this process: v0.1 runs one. A notebook takes
// one at a time, and each counts as a job in the queue, so that uploads
// that will be refused are refused before they are sent.
type uploads struct {
	mu        sync.Mutex
	notebooks map[uuid.UUID]bool
}

// claim claims the notebook id: false when an upload holds it.
func (u *uploads) claim(id uuid.UUID) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.notebooks[id] {
		return false
	}
	u.notebooks[id] = true
	return true
}

func (u *uploads) release(id uuid.UUID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.notebooks, id)
}

// others is how many uploads are under way into notebooks but id.
func (u *uploads) others(id uuid.UUID) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.notebooks[id] {
		return len(u.notebooks) - 1
	}
	return len(u.notebooks)
}

// ImportRequest is what an import is started with: the notebook, the page
// it goes under (nil: the notebook's root), the uploaded file's name,
// where the request came from, and the bytes of its body as it declares
// them (-1: unknown).
type ImportRequest struct {
	NotebookID uuid.UUID
	ParentID   *uuid.UUID
	FileName   string
	Client     domain.Client
	Size       int64
}

// Stored is an import's archive Store wrote: its job's id, taken then, and
// its bytes.
type Stored struct {
	ID    uuid.UUID
	Bytes int64
}

// ReadError is a request's body that failed as Store read the file: the
// handler answers it 400.
type ReadError struct {
	Err error
}

func (e *ReadError) Error() string { return "transfer: read the file: " + e.Err.Error() }
func (e *ReadError) Unwrap() error { return e.Err }

// Check decides, before the file is read and unlocked, what Create
// decides again: a notebook the caller cannot see is notebook.not_found;
// a reader is forbidden; a parent that is no page of the notebook not
// deleted is page.not_found; an import of the notebook being uploaded,
// queued or running, anyone's, is transfer.busy; MaxQueued jobs waiting,
// running or being uploaded are 503 server_busy; a store that would keep
// less than MinFree once the body is written is 507 storage_full. Once it
// passes, the notebook is claimed for the upload until Release.
func (s *StartImport) Check(ctx context.Context, req ImportRequest) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	workspaceID, err := s.workspaceOf(ctx, req.NotebookID)
	if err != nil {
		return err
	}
	if err := s.d.authorize(ctx, actor, workspaceID, req.NotebookID, domain.ActionImport); err != nil {
		return err
	}
	if err := s.place(ctx, req); err != nil {
		return err
	}
	if !s.uploads.claim(req.NotebookID) {
		return domain.ErrImportBusy
	}
	if err := s.admit(ctx, req); err != nil {
		s.uploads.release(req.NotebookID)
		return err
	}
	return nil
}

// admit is Check's last: no import of the notebook, room in the queue and
// in the store for the body.
func (s *StartImport) admit(ctx context.Context, req ImportRequest) error {
	if err := s.busy(ctx, req.NotebookID); err != nil {
		return err
	}
	if err := s.d.room(ctx, s.uploads.others(req.NotebookID)); err != nil {
		return err
	}
	return s.d.free(ctx, max(req.Size, 0))
}

// Release releases the notebook Check claimed for req's upload: its
// request ended.
func (s *StartImport) Release(req ImportRequest) {
	s.uploads.release(req.NotebookID)
}

// workspaceOf is the notebook's workspace: notebook.not_found for none.
func (s *StartImport) workspaceOf(ctx context.Context, notebookID uuid.UUID) (uuid.UUID, error) {
	workspaceID, ok, err := s.d.Notebooks.WorkspaceOf(ctx, notebookID)
	if err == nil && !ok {
		err = domain.ErrNotebookNotFound
	}
	return workspaceID, err
}

// place is page.not_found when the import goes under a page that is no
// page of the notebook not deleted.
func (s *StartImport) place(ctx context.Context, req ImportRequest) error {
	if req.ParentID == nil {
		return nil
	}
	_, ok, err := s.d.Nodes.Page(ctx, req.NotebookID, *req.ParentID)
	if err == nil && !ok {
		err = domain.ErrRootNotFound
	}
	return err
}

// busy is transfer.busy when the notebook has an import queued or
// running.
func (s *StartImport) busy(ctx context.Context, notebookID uuid.UUID) error {
	busy, err := s.d.Rows.Importing(ctx, notebookID)
	if err == nil && busy {
		err = domain.ErrImportBusy
	}
	return err
}

// Store writes the file r brings as an import's archive, its job's id
// taken now: ErrTooLarge past ImportMaxBytes, domain.ErrStorageFull, a
// *ReadError when r fails; none leaves a file. A commit that fails may
// leave it all the same, which the sweep deletes.
func (s *StartImport) Store(ctx context.Context, r io.Reader) (Stored, error) {
	id := uuid.NewV7()
	up, err := s.d.Archives.Upload(ctx, id)
	if err != nil {
		return Stored{}, err
	}
	n, err := copyAtMost(up, r, s.d.ImportMaxBytes)
	if err != nil {
		if abortErr := up.Abort(); abortErr != nil {
			s.d.Logger.WarnContext(ctx, "import archive not dropped", slog.String("job_id", id.String()), slog.Any("error", abortErr))
		}
		return Stored{}, err
	}
	if err := up.Commit(); err != nil {
		return Stored{}, err
	}
	return Stored{ID: id, Bytes: n}, nil
}

// copyAtMost copies r to w: ErrTooLarge past most bytes, a *ReadError when
// r fails, w's error when it does.
func copyAtMost(w io.Writer, r io.Reader, most int64) (int64, error) {
	buf := make([]byte, 32<<10)
	var n int64
	for {
		k, rerr := r.Read(buf)
		if k > 0 {
			if n += int64(k); n > most {
				return n, ErrTooLarge
			}
			if _, err := w.Write(buf[:k]); err != nil {
				return n, err
			}
		}
		if errors.Is(rerr, io.EOF) {
			return n, nil
		}
		if rerr != nil {
			return n, &ReadError{Err: rerr}
		}
	}
}

// Discard deletes a stored archive no job will read: its request failed
// after Store. The sweep deletes one it cannot.
func (s *StartImport) Discard(ctx context.Context, stored Stored) {
	if err := s.d.Archives.Delete(ctx, domain.KindImport, stored.ID); err != nil {
		s.d.Logger.WarnContext(ctx, "import archive not deleted", slog.String("job_id", stored.ID.String()), slog.Any("error", err))
	}
}

// Create starts the import of stored as req asks, and answers its job:
// under the workspace's row and the notebook's, FOR SHARE, it decides
// transfer.import and checks the parent again; then, the jobs' creations
// one at a time, there is room in the queue and no import of the
// notebook. The job, named as the file, goes under the parent. The
// archive is deleted on any failure before the commit; one whose commit
// failed is left to the sweep.
func (s *StartImport) Create(ctx context.Context, req ImportRequest, stored Stored) (JobView, error) {
	job, written, err := s.write(ctx, req, stored)
	if err != nil {
		if !written {
			s.Discard(context.WithoutCancel(ctx), stored)
		}
		return JobView{}, err
	}
	s.d.Logger.InfoContext(ctx, "import queued", slog.String("job_id", job.ID.String()), slog.String("notebook_id", job.NotebookID.String()),
		slog.String("user_id", job.CreatedBy.String()), slog.String("client", string(job.Client)), slog.Int64("bytes", stored.Bytes))
	got, err := views{names: s.d.Names, signer: s.d.Signer, clock: s.d.Clock}.of(ctx, []domain.Job{job})
	if err != nil {
		return JobView{}, err
	}
	return got[0], nil
}

// write writes the job's row and enqueues it; written tells both were,
// the transaction then committing.
func (s *StartImport) write(ctx context.Context, req ImportRequest, stored Stored) (job domain.Job, written bool, err error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return job, false, err
	}
	workspaceID, err := s.workspaceOf(ctx, req.NotebookID)
	if err != nil {
		return job, false, err
	}
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.d.lock(ctx, actor, workspaceID, req.NotebookID, domain.ActionImport); err != nil {
			return err
		}
		if err := s.place(ctx, req); err != nil {
			return err
		}
		if err := s.d.Rows.LockQueue(ctx); err != nil {
			return err
		}
		if err := s.d.room(ctx, s.uploads.others(req.NotebookID)); err != nil {
			return err
		}
		if err := s.busy(ctx, req.NotebookID); err != nil {
			return err
		}
		job = domain.Job{ID: stored.ID, NotebookID: req.NotebookID, RootID: req.ParentID, Kind: domain.KindImport, State: domain.StateQueued,
			Name: domain.ImportName(req.FileName), CreatedBy: actor.UserID, Client: req.Client, CreatedAt: s.d.Clock.Now()}
		if err := s.d.Rows.CreateJob(ctx, job); err != nil {
			return err
		}
		if err := s.d.Queue.Import(ctx, job.ID); err != nil {
			return err
		}
		written = true
		return nil
	})
	return job, written, err
}
