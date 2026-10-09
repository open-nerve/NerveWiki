package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
)

// LinkedPages is those of targets that a link of a page of sources
// resolves to, read in the caller's transaction (M7/P5 design 3.8).
func (s *Store) LinkedPages(ctx context.Context, sources, targets []uuid.UUID) ([]uuid.UUID, error) {
	if len(targets) == 0 || len(sources) == 0 {
		return nil, nil
	}
	ids, err := s.planned(ctx).LinkedPages(ctx, gen.LinkedPagesParams{Targets: targets, Sources: sources})
	if err != nil {
		return nil, fmt.Errorf("linked pages: %w", err)
	}
	return ids, nil
}
