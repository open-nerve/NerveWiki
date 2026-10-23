package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
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

// GrepContents implements app.GrepReads.
func (s *Store) GrepContents(ctx context.Context, ids []uuid.UUID, max int64) (app.ContentsWithin, error) {
	rows, err := s.queries(ctx).GrepContents(ctx, gen.GrepContentsParams{NodeIds: ids, MaxBytes: max})
	if err != nil {
		return app.ContentsWithin{}, fmt.Errorf("grep contents: %w", err)
	}
	out := app.ContentsWithin{Contents: make(map[uuid.UUID]string, len(rows))}
	for _, r := range rows {
		if !r.Within {
			out.Past, out.PastBytes = r.NodeID, int(r.ByteSize)
			continue
		}
		out.Contents[r.NodeID] = r.Content
	}
	return out, nil
}
