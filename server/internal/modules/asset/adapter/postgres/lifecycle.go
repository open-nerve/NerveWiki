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

// RestoreBlobsOfNodes implements app.Deletions, planned with nodeIDs: from
// a cached statement's sixth run, statistics taken while the rows were
// alive make its plan for any arguments read the rows of the time and
// compare each with the ids one by one, 110 ms at a restore's bound of
// nodes where the plan with them took 40 (M8/P2A fix check 1, I-1).
func (s *Store) RestoreBlobsOfNodes(ctx context.Context, nodeIDs []uuid.UUID, deletedAt time.Time) (int, error) {
	n, err := s.planned(ctx).RestoreBlobsOfNodes(ctx, gen.RestoreBlobsOfNodesParams{NodeIds: nodeIDs, DeletedAt: deletedAt})
	if err != nil {
		return 0, fmt.Errorf("restore the blobs of nodes: %w", err)
	}
	return int(n), nil
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

// AttachmentInfos is the type and size of the attachments of nodeIDs,
// deleted or not, by node.
func (s *Store) AttachmentInfos(ctx context.Context, nodeIDs []uuid.UUID) (map[uuid.UUID]app.AttachmentInfo, error) {
	rows, err := s.queries(ctx).AttachmentInfos(ctx, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("attachment infos: %w", err)
	}
	out := make(map[uuid.UUID]app.AttachmentInfo, len(rows))
	for _, r := range rows {
		out[r.NodeID] = app.AttachmentInfo{MIME: r.Mime, Bytes: r.ByteSize}
	}
	return out, nil
}
