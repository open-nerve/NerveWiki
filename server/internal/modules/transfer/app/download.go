package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// Download serves an export's archive at its signed address (M7/P5 design
// 3.11): the signature is the grant, given only by reads that decided the
// caller reads the notebook.
type Download struct {
	rows     Rows
	archives Archives
	signer   Signer
	clock    Clock
	logger   *slog.Logger
}

// NewDownload returns the use case.
func NewDownload(rows Rows, archives Archives, signer Signer, clock Clock, logger *slog.Logger) *Download {
	return &Download{rows: rows, archives: archives, signer: signer, clock: clock, logger: logger}
}

// Address is a download's address as it was read: the job, the time it
// expires at, in Unix seconds, and its signature.
type Address struct {
	JobID     uuid.UUID
	Expires   int64
	Signature string
}

// Opened is an archive opened: its file, and the name it is downloaded by.
type Opened struct {
	File ArchiveFile
	Name string
}

// Open opens the archive a's signature signs, unexpired, of an export not
// deleted that succeeded, its file in the store; anything else is the
// platform's not_found, alike for each. The signature is checked before
// any read. A file missing is logged.
func (d *Download) Open(ctx context.Context, a Address) (Opened, error) {
	if !d.signer.Valid(d.clock.Now(), a.JobID, a.Expires, a.Signature) {
		return Opened{}, domain.ErrDownloadNotFound
	}
	j, err := d.rows.FindJob(ctx, a.JobID)
	if errors.Is(err, ErrNoRow) {
		return Opened{}, domain.ErrDownloadNotFound
	}
	if err != nil {
		return Opened{}, err
	}
	if j.Kind != domain.KindExport || j.State != domain.StateSucceeded {
		return Opened{}, domain.ErrDownloadNotFound
	}
	f, err := d.archives.Open(ctx, j.ID)
	if errors.Is(err, ErrFileMissing) {
		d.logger.WarnContext(ctx, "export archive missing", slog.String("job_id", j.ID.String()), slog.String("notebook_id", j.NotebookID.String()))
		return Opened{}, domain.ErrDownloadNotFound
	}
	if err != nil {
		return Opened{}, err
	}
	return Opened{File: f, Name: j.Name + ".zip"}, nil
}
