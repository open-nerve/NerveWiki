package app

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// SweepAge is how old a file no row holds is when the orphan sweep deletes
// it: a day, far longer than a file waits between its commit and its
// row's, so that no upload under way loses its file.
const SweepAge = 24 * time.Hour

// sweepBatch is how many files one read of the rows asks about.
const sweepBatch = 500

// Sweep deletes the files no row holds (M7/P2 design 3.8): those an upload
// stored whose unit failed after the file, its outcome unknown, or whose
// server stopped in between. A row deleted still holds its file, which
// the purge deletes.
type Sweep struct {
	files  StoredFiles
	rows   KnownRows
	clock  Clock
	logger *slog.Logger
}

// NewSweep returns it.
func NewSweep(files StoredFiles, rows KnownRows, clock Clock, logger *slog.Logger) *Sweep {
	return &Sweep{files: files, rows: rows, clock: clock, logger: logger}
}

// Run deletes the attachments' files older than SweepAge that no row
// holds, sweepBatch files a read of the rows, and tells how many, logged
// when some were. A file of the area that is no blob's is left, logged as
// a warning; so is one it cannot delete, the run going on to the rest and
// failing at its end. Any other error stops it, the files deleted before
// gone.
func (s *Sweep) Run(ctx context.Context) (int, error) {
	deleted, failed := 0, 0
	var first error
	var batch []uuid.UUID
	flush := func() error {
		known, err := s.rows.KnownBlobs(ctx, batch)
		if err != nil {
			return err
		}
		held := make(map[uuid.UUID]bool, len(known))
		for _, id := range known {
			held[id] = true
		}
		for _, id := range batch {
			if held[id] {
				continue
			}
			if err := s.files.Delete(ctx, domain.Key(id)); err != nil {
				s.logger.WarnContext(ctx, "orphan attachment file not deleted", slog.String("blob_id", id.String()), slog.Any("error", err))
				failed++
				first = cmp.Or(first, err)
				continue
			}
			deleted++
		}
		batch = batch[:0]
		return nil
	}
	err := s.files.List(ctx, domain.Area, s.clock.Now().Add(-SweepAge), func(key string) error {
		id, ok := domain.IDOf(key)
		if !ok {
			s.logger.WarnContext(ctx, "a file of the attachments' area that is no attachment's", slog.String("key", key))
			return nil
		}
		if batch = append(batch, id); len(batch) == sweepBatch {
			return flush()
		}
		return nil
	})
	if err == nil && len(batch) > 0 {
		err = flush()
	}
	if deleted > 0 {
		s.logger.InfoContext(ctx, "orphan attachment files deleted", slog.Int("files", deleted))
	}
	if err == nil && failed > 0 {
		err = fmt.Errorf("%d orphan attachment files not deleted, the first: %w", failed, first)
	}
	return deleted, err
}
