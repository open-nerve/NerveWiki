package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
)

// DeleteBlobsOfNodes implements app.Deletions.
func (s *Store) DeleteBlobsOfNodes(ctx context.Context, nodeIDs []uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).DeleteBlobsOfNodes(ctx, gen.DeleteBlobsOfNodesParams{At: at, NodeIds: nodeIDs}); err != nil {
		return fmt.Errorf("delete the blobs of nodes: %w", err)
	}
	return nil
}

// DeleteBlobsOfNotebooks implements app.Deletions.
func (s *Store) DeleteBlobsOfNotebooks(ctx context.Context, notebookIDs []uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).DeleteBlobsOfNotebooks(ctx, gen.DeleteBlobsOfNotebooksParams{At: at, NotebookIds: notebookIDs}); err != nil {
		return fmt.Errorf("delete the blobs of notebooks: %w", err)
	}
	return nil
}

// NotebookActivities implements app.Activities.
func (s *Store) NotebookActivities(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.Activity, error) {
	rows, err := s.queries(ctx).NotebookActivities(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("the attachments' activity: %w", err)
	}
	out := make(map[uuid.UUID]app.Activity, len(rows))
	for _, r := range rows {
		out[r.NotebookID] = app.Activity{Bytes: r.Bytes}
	}
	return out, nil
}

// KnownBlobs implements app.KnownRows.
func (s *Store) KnownBlobs(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	known, err := s.queries(ctx).KnownBlobs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("known blobs: %w", err)
	}
	return known, nil
}
