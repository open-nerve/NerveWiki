package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
)

// IndexPages implements app.IndexReads.
func (s *Store) IndexPages(ctx context.Context, notebookID uuid.UUID, typeIs *string) ([]app.IndexedPage, error) {
	rows, err := s.queries(ctx).IndexPages(ctx, gen.IndexPagesParams{NotebookID: notebookID, TypeIs: typeIs})
	if err != nil {
		return nil, fmt.Errorf("the index's pages of %s: %w", notebookID, err)
	}
	out := make([]app.IndexedPage, len(rows))
	for i, r := range rows {
		out[i] = app.IndexedPage{ID: r.NodeID, TypeStart: r.TypeStart, Typed: r.Typed}
	}
	return out, nil
}

// IndexItems implements app.IndexReads.
func (s *Store) IndexItems(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.IndexFacts, error) {
	rows, err := s.queries(ctx).IndexItems(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("the index's items: %w", err)
	}
	out := make(map[uuid.UUID]app.IndexFacts, len(rows))
	for _, r := range rows {
		f := app.IndexFacts{Tags: r.Tags}
		if r.Typed {
			f.Type = &r.TypeText
		}
		if r.Summarized {
			f.Summary = &r.SummaryStart
		}
		out[r.NodeID] = f
	}
	return out, nil
}

// InboundCounts implements app.IndexReads.
func (s *Store) InboundCounts(ctx context.Context, ids []uuid.UUID, most int) (map[uuid.UUID]int, error) {
	rows, err := s.queries(ctx).InboundCounts(ctx, gen.InboundCountsParams{Ids: ids, MaxCount: int32(most)})
	if err != nil {
		return nil, fmt.Errorf("the pages' inbound links: %w", err)
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		out[r.PageID] = int(r.Inbound)
	}
	return out, nil
}
