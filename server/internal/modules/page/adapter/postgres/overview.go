package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// NotebookCounts implements app.OverviewStore.
func (s *Store) NotebookCounts(ctx context.Context, notebookID uuid.UUID) (pages, attachments int64, err error) {
	row, err := s.queries(ctx).NotebookCounts(ctx, notebookID)
	if err != nil {
		return 0, 0, fmt.Errorf("count a notebook's nodes: %w", err)
	}
	return row.Pages, row.Attachments, nil
}

// NotebookLastChange implements app.OverviewStore: the statement answers
// no row for a notebook without a changeset, which is nil.
func (s *Store) NotebookLastChange(ctx context.Context, notebookID uuid.UUID) (*time.Time, error) {
	at, err := s.queries(ctx).NotebookLastChange(ctx, notebookID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("read a notebook's last change: %w", err)
	}
	return &at, nil
}

// FirstChildren implements app.OverviewStore: the root's alone.
func (s *Store) FirstChildren(ctx context.Context, notebookID uuid.UUID, n int) ([]domain.Node, error) {
	rows, err := s.queries(ctx).FirstChildren(ctx, gen.FirstChildrenParams{NotebookID: notebookID, N: int32(n)})
	if err != nil {
		return nil, fmt.Errorf("list the root's first children: %w", err)
	}
	out := make([]domain.Node, len(rows))
	for i, r := range rows {
		out[i] = nodeOf(gen.FindNodeRow(r))
	}
	return out, nil
}
