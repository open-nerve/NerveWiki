package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Purge deletes the attachments' rows deleted longer than the retention
// ago, their files first (M7/P2 design 3.8): a row outlives its file, never
// the other way, so no file of the store is left without a row but those
// the orphan sweep finds.
type Purge struct {
	tx     shared.TxManager
	rows   ExpiredRows
	files  Files
	logger *slog.Logger
}

// NewPurge returns the purge.
func NewPurge(tx shared.TxManager, rows ExpiredRows, files Files, logger *slog.Logger) *Purge {
	return &Purge{tx: tx, rows: rows, files: files, logger: logger}
}

// Batch purges up to batch rows deleted before before, in one transaction
// that holds them: their files, a file already gone deleted all the same,
// then the rows. A file that is not deleted fails the batch, logged with
// its blob, and keeps the rows. A commit that fails leaves the rows of
// files already gone, for the next run.
func (p *Purge) Batch(ctx context.Context, before time.Time, batch int) (int, error) {
	deleted := 0
	err := p.tx.WithinTx(ctx, func(ctx context.Context) error {
		ids, err := p.rows.ExpiredBlobs(ctx, before, batch)
		if err != nil || len(ids) == 0 {
			return err
		}
		for _, id := range ids {
			if err := p.files.Delete(ctx, domain.Key(id)); err != nil {
				p.logger.ErrorContext(ctx, "attachment file not purged", slog.String("blob_id", id.String()), slog.Any("error", err))
				return fmt.Errorf("delete the file of blob %s: %w", id, err)
			}
		}
		deleted, err = p.rows.DeleteBlobs(ctx, ids)
		return err
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}
