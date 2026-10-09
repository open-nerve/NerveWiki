// Package app holds the transfer module's use cases and the ports they
// need (M7/P5 design 3.7–3.12).
package app

import (
	"context"
	"errors"
	"io"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// Workspaces locks a notebook's workspace: bootstrap hands it the
// workspace module's.
type Workspaces interface {
	// ShareByID locks the workspace not deleted id FOR SHARE until the
	// transaction ends, and reports whether there is one.
	ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
}

// Notebooks reads and locks the notebooks: bootstrap hands it the notebook
// module's.
type Notebooks interface {
	// WorkspaceOf is the workspace of the notebook not deleted id,
	// unlocked; false for none.
	WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
	// ShareByID locks the notebook not deleted id FOR SHARE until the
	// transaction ends, and reports whether there is one.
	ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
	// NameOf is the name of the notebook not deleted id, in the caller's
	// transaction; false for none.
	NameOf(ctx context.Context, id uuid.UUID) (string, bool, error)
}

// Names reads the accounts' display names: identity's directory.
type Names interface {
	DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// Nodes reads an export's scope, in the caller's snapshot: bootstrap
// adapts the page module's ExportNodes.
type Nodes interface {
	// Page is the name of the page not deleted id of notebookID; false for
	// none, or for an attachment.
	Page(ctx context.Context, notebookID, id uuid.UUID) (string, bool, error)
	// Scope is the nodes not deleted of notebookID, or of the page root and
	// its subtree when root is set.
	Scope(ctx context.Context, notebookID uuid.UUID, root *uuid.UUID) ([]domain.Node, error)
	// Contents is the content of each page of ids not deleted, by id.
	Contents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// Linked tells which pages links lead to, in the caller's snapshot:
// bootstrap hands it the linking module's.
type Linked interface {
	// Linked is those of targets that a link of a page of sources resolves
	// to: pages of one notebook, the export's.
	Linked(ctx context.Context, sources, targets []uuid.UUID) ([]uuid.UUID, error)
}

// Blobs reads the attachments' files: bootstrap hands it the asset
// module's.
type Blobs interface {
	// Of is the blob of each attachment of nodeIDs not deleted, of
	// notebookID, by node, in the caller's snapshot.
	Of(ctx context.Context, notebookID uuid.UUID, nodeIDs []uuid.UUID) (map[uuid.UUID]Blob, error)
	// Open opens the blob's file, or answers ErrFileMissing.
	Open(ctx context.Context, blobID uuid.UUID) (io.ReadCloser, error)
}

// Blob is an attachment's file: its id, and when it was written, its
// entry's time in the archive.
type Blob struct {
	ID      uuid.UUID
	Created time.Time
}

// ErrFileMissing is a file that is not in the store.
var ErrFileMissing = errors.New("transfer: the file is not in the store")

// Archives keeps the jobs' archives in the store: adapter/archive.
type Archives interface {
	// Create starts the archive of the export id; domain.ErrStorageFull
	// when the store keeps no room for it.
	Create(ctx context.Context, id uuid.UUID) (Archive, error)
	// Open opens the export id's archive, or answers ErrFileMissing.
	Open(ctx context.Context, id uuid.UUID) (ArchiveFile, error)
	// Delete removes the archive of the job id of kind; there being none
	// is success.
	Delete(ctx context.Context, kind domain.Kind, id uuid.UUID) error
	// List calls each with the id of every archive of kind's jobs last
	// modified before before; a file of the area that is no archive's is
	// left, and logged.
	List(ctx context.Context, kind domain.Kind, before time.Time, each func(id uuid.UUID) error) error
	// Free tells the free bytes of the store's disk.
	Free(ctx context.Context) (int64, error)
}

// Archive is an export's archive as it is written: a zip file, visible
// once it commits. A write that runs out of room is
// domain.ErrStorageFull.
type Archive interface {
	// Add writes the file at path, modified then, as it is when stored,
	// deflated otherwise; a folder's entry when r is nil, path ending in
	// "/".
	Add(path string, modified time.Time, stored bool, r io.Reader) error
	// Commit finishes the archive and keeps it, and tells its bytes. On
	// failure the file may be there all the same: an orphan, which the
	// sweep finds.
	Commit() (int64, error)
	// Abort drops it.
	Abort() error
}

// ArchiveFile is an export's archive open for reading.
type ArchiveFile interface {
	io.ReadSeekCloser
	Size() int64
	ModTime() time.Time
}

// Queue enqueues the jobs in the caller's transaction: adapter/river.
type Queue interface {
	Export(ctx context.Context, id uuid.UUID) error
}

// Held tells which jobs River still holds: adapter/river.
type Held interface {
	// Held is the ids of the exports River has not finished: to work,
	// working, or to try again.
	Held(ctx context.Context) ([]uuid.UUID, error)
}

// Signer signs the addresses of the exports' archives: adapter/mac.
type Signer interface {
	// Sign signs the archive of the export id as of now.
	Sign(now time.Time, id uuid.UUID) Signed
	// Valid reports whether sig signs the archive of the export id,
	// expiring at e, and e is later than now.
	Valid(now time.Time, id uuid.UUID, e int64, sig string) bool
}

// Signed is an archive's signed address: when it expires, and its
// signature.
type Signed struct {
	Expires   time.Time
	Signature string
}

// Beat is what a running job's heartbeat reads back.
type Beat struct {
	// CancelRequested tells a cancel was asked.
	CancelRequested bool
	// Deleted tells the job was deleted, with its notebook.
	Deleted bool
}

// Cursor is where a list of jobs goes on: after the job of this time and
// id.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Ended is a job's end as it is written: its state, report, name and
// progress, and an export's archive's bytes.
type Ended struct {
	State       domain.State
	At          time.Time
	Report      domain.Report
	Name        string
	Progress    domain.Progress
	ResultBytes *int64
}

// Interrupted is a job the rescue failed.
type Interrupted struct {
	ID, NotebookID, CreatedBy uuid.UUID
	Client                    domain.Client
}

// Rows keeps the jobs' rows: adapter/postgres, in the caller's
// transaction when it has one. A job not found is ErrNoRow.
type Rows interface {
	CreateJob(ctx context.Context, j domain.Job) error
	// LockQueue serializes the jobs' creations until the transaction ends.
	LockQueue(ctx context.Context) error
	// CountActive is how many jobs are queued or running.
	CountActive(ctx context.Context) (int, error)
	// Exporting reports whether userID has an export queued or running in
	// notebookID.
	Exporting(ctx context.Context, notebookID, userID uuid.UUID) (bool, error)
	FindJob(ctx context.Context, id uuid.UUID) (domain.Job, error)
	// LockJob is FindJob locked until the transaction ends.
	LockJob(ctx context.Context, id uuid.UUID) (domain.Job, error)
	// ListJobs is notebookID's jobs, newest first, by's alone when it is
	// set, after the cursor's, at most limit.
	ListJobs(ctx context.Context, notebookID uuid.UUID, by *uuid.UUID, after *Cursor, limit int) ([]domain.Job, error)
	// StartJob moves the queued job id to running at at; ErrNoRow when it
	// is not queued, or deleted.
	StartJob(ctx context.Context, id uuid.UUID, at time.Time) (domain.Job, error)
	// BeatJob writes the running job's heartbeat and progress; ErrNoRow
	// when it no longer runs.
	BeatJob(ctx context.Context, id uuid.UUID, at time.Time, p domain.Progress) (Beat, error)
	// FinishJob ends the running job id; false when it does not run, or
	// is deleted.
	FinishJob(ctx context.Context, id uuid.UUID, e Ended) (bool, error)
	// ExpireOthers expires userID's other exports of notebookID that
	// succeeded, and tells which.
	ExpireOthers(ctx context.Context, notebookID, userID, id uuid.UUID) ([]uuid.UUID, error)
	// CancelQueued cancels the queued job id at at; false when it is not
	// queued.
	CancelQueued(ctx context.Context, id uuid.UUID, at time.Time, r domain.Report) (bool, error)
	// RequestCancel asks the running job id to stop; false when it does
	// not run.
	RequestCancel(ctx context.Context, id uuid.UUID, at time.Time) (bool, error)
}

// ErrNoRow is a job's row that is not there.
var ErrNoRow = errors.New("transfer: no such job's row")

// MaintainedRows are the rows the background jobs keep: adapter/postgres.
type MaintainedRows interface {
	// ExpireExports expires up to batch exports that succeeded before
	// before, skipping those locked, and tells which.
	ExpireExports(ctx context.Context, before time.Time, batch int) ([]uuid.UUID, error)
	// InterruptJobs fails the running jobs whose heartbeat is older than
	// beatBefore, or all of them when it is nil, with r; those another
	// transaction holds are skipped.
	InterruptJobs(ctx context.Context, beatBefore *time.Time, at time.Time, r domain.Report) ([]Interrupted, error)
	// QueuedJobs is the ids of the queued jobs.
	QueuedJobs(ctx context.Context) ([]uuid.UUID, error)
	// FailQueued fails those of ids still queued with r; those another
	// transaction holds are skipped.
	FailQueued(ctx context.Context, ids []uuid.UUID, at time.Time, r domain.Report) ([]Interrupted, error)
	// LiveArchives is those of ids whose archives are kept.
	LiveArchives(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
	// DeleteJobsOfNotebooks deletes the notebooks' jobs at at.
	DeleteJobsOfNotebooks(ctx context.Context, notebookIDs []uuid.UUID, at time.Time) error
	// ExpiredJobs is up to batch jobs deleted before before, locked until
	// the transaction ends, with their kinds.
	ExpiredJobs(ctx context.Context, before time.Time, batch int) (map[uuid.UUID]domain.Kind, error)
	DeleteJobs(ctx context.Context, ids []uuid.UUID) (int, error)
}
