package domain

import (
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ErrNotFound is a job that does not exist, is deleted, is of a notebook
// the caller cannot see, or is another's and the caller is not the
// notebook's admin.
var ErrNotFound = shared.NewError(shared.KindNotFound, "transfer.not_found", "No such job.")

// ErrNotebookNotFound is a notebook that does not exist, is deleted, or
// that the caller has no role in.
var ErrNotebookNotFound = shared.NewError(shared.KindNotFound, "notebook.not_found", "No such notebook.")

// ErrRootNotFound is a page to export that is no page of the notebook not
// deleted: missing, deleted, or an attachment.
var ErrRootNotFound = shared.NewError(shared.KindNotFound, "page.not_found", "No such page.")

// ErrBusy is an export asked of a reader who has one queued or running in
// the notebook (M7 design 4.9).
var ErrBusy = shared.NewError(shared.KindConflict, "transfer.busy",
	"An export of yours in this notebook is queued or running: wait for it, or cancel it.")

// ErrNotCancellable is a cancel of a job that has ended.
var ErrNotCancellable = shared.NewError(shared.KindConflict, "transfer.not_cancellable", "The job has ended.")

// ErrDownloadNotFound is an address of an archive that is not one the
// server signed, has expired, or whose job or file is gone: the platform's
// not_found, alike for each.
var ErrDownloadNotFound = shared.NewError(shared.KindNotFound, "not_found", "No such archive.")

// QueueRetry is the Retry-After of a job refused because transfer.max_queued
// jobs wait: a few minutes, as jobs take.
const QueueRetry = 5 * time.Minute

// ErrQueueFull is a job refused because transfer.max_queued jobs are queued
// or running: 503 server_busy.
var ErrQueueFull = shared.ServerBusy(QueueRetry)

// ErrStorageFull is an archive the store has no room for: 507 storage_full.
var ErrStorageFull = shared.StorageFull()
