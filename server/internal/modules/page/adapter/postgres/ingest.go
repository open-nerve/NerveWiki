package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

const maxIngestSources = 10000

// Add implements app.IngestMarks. The one statement locks the owned live
// edit, appends after its old count, and updates the count atomically.
func (s *Store) Add(ctx context.Context, notebookID, changesetID uuid.UUID, sourceIDs []uuid.UUID) (app.IngestMarkResult, error) {
	for i := 1; i < len(sourceIDs); i++ {
		if sourceIDs[i-1].Compare(sourceIDs[i]) >= 0 {
			return app.IngestMarkResult{}, fmt.Errorf("ingest marks: source IDs must be sorted and unique")
		}
	}
	r, err := s.queries(ctx).AddIngestMarks(ctx, gen.AddIngestMarksParams{
		NotebookID: notebookID, ChangesetID: changesetID, SourceIds: sourceIDs, MaxSources: maxIngestSources,
	})
	if err != nil {
		return app.IngestMarkResult{}, fmt.Errorf("append ingest marks: %w", err)
	}
	return app.IngestMarkResult{Added: int(r.Added), Count: int(r.IngestCount)}, nil
}

// FirstIngestSources reads the first n historical positions, with a null
// for each source that is no longer alive in the changeset's notebook.
func (s *Store) FirstIngestSources(ctx context.Context, ids []uuid.UUID, n int) (map[uuid.UUID][]*uuid.UUID, error) {
	if n != 3 && n != 5 && n != 20 {
		return nil, fmt.Errorf("ingest preview: unsupported count %d", n)
	}
	out := make(map[uuid.UUID][]*uuid.UUID, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	for _, id := range ids {
		out[id] = []*uuid.UUID{}
	}
	rows, err := s.queries(ctx).FirstIngestSources(ctx, gen.FirstIngestSourcesParams{ChangesetIds: ids, PreviewCount: int32(n)})
	if err != nil {
		return nil, fmt.Errorf("first ingest sources: %w", err)
	}
	for _, row := range rows {
		out[row.ChangesetID] = append(out[row.ChangesetID], row.NodeID)
	}
	return out, nil
}
