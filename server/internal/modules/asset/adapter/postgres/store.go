// Package postgresadapter keeps the asset module's rows in PostgreSQL,
// through the queries sqlc generates from queries/ into gen.
package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Store is the asset module's repository.
type Store struct {
	pool *pgxpool.Pool
}

// New returns the store over pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// queries runs in the context's transaction when there is one.
func (s *Store) queries(ctx context.Context) *gen.Queries {
	return gen.New(postgres.DB(ctx, s.pool))
}

// CreateBlob implements app.Rows.
func (s *Store) CreateBlob(ctx context.Context, b domain.Blob) error {
	err := s.queries(ctx).CreateBlob(ctx, gen.CreateBlobParams{
		ID: b.ID, NodeID: b.NodeID, NotebookID: b.NotebookID, Mime: b.MIME, ByteSize: b.Bytes, Sha256: b.SHA256,
		Width: side(b.Width), Height: side(b.Height), CreatedByID: b.CreatedBy, CreatedAt: b.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("create blob: %w", err)
	}
	return nil
}

// BlobOfNode implements app.Rows.
func (s *Store) BlobOfNode(ctx context.Context, nodeID uuid.UUID) (domain.Blob, error) {
	r, err := s.queries(ctx).BlobOfNode(ctx, nodeID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Blob{}, app.ErrNoRow
	case err != nil:
		return domain.Blob{}, fmt.Errorf("blob of node: %w", err)
	}
	return blobOf(r), nil
}

// BlobsOfNodes implements app.Rows.
func (s *Store) BlobsOfNodes(ctx context.Context, nodeIDs []uuid.UUID) (map[uuid.UUID]domain.Blob, error) {
	rows, err := s.queries(ctx).BlobsOfNodes(ctx, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("blobs of nodes: %w", err)
	}
	out := make(map[uuid.UUID]domain.Blob, len(rows))
	for _, r := range rows {
		out[r.NodeID] = blobOf(gen.BlobOfNodeRow(r))
	}
	return out, nil
}

// side is an image's width or height as its column holds it: NULL for 0,
// unknown.
func side(n int) *int32 {
	if n == 0 {
		return nil
	}
	v := int32(n) //nolint:gosec // at most domain.MaxSide
	return &v
}

// blobOf is a row of the queries that read a blob's columns: they read
// the same, so their rows convert to this one.
func blobOf(r gen.BlobOfNodeRow) domain.Blob {
	b := domain.Blob{ID: r.ID, NodeID: r.NodeID, NotebookID: r.NotebookID, MIME: r.Mime, Bytes: r.ByteSize, SHA256: r.Sha256,
		CreatedBy: r.CreatedByID, CreatedAt: r.CreatedAt}
	if r.Width != nil && r.Height != nil {
		b.Width, b.Height = int(*r.Width), int(*r.Height)
	}
	return b
}

// Analyze updates the statistics of the module's tables an import writes
// (M7/P6 design 3.6).
func (s *Store) Analyze(ctx context.Context) error {
	if err := s.queries(ctx).Analyze(ctx); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	return nil
}
