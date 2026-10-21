package postgresadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
)

// GrepCandidates implements app.GrepReads: one statement on the pool, its
// error its SQLSTATE alone, as the search's (the keys are a content's
// pieces).
func (s *Store) GrepCandidates(ctx context.Context, notebookID uuid.UUID, keys []string) ([]uuid.UUID, error) {
	ids, err := s.queries(ctx).GrepCandidates(ctx, gen.GrepCandidatesParams{NotebookID: notebookID, Keys: keys})
	if err != nil {
		return nil, stateOnly("read grep's candidates", err)
	}
	return ids, nil
}

// GrepContents implements app.GrepReads: export's batch of contents.
func (s *Store) GrepContents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	return s.Contents(ctx, ids)
}
