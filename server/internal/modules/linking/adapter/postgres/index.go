package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// IndexPages implements app.IndexReads.
func (s *Store) IndexPages(ctx context.Context, notebookID uuid.UUID, typeIs *string) ([]app.IndexedPage, error) {
	rows, err := s.queries(ctx).IndexPages(ctx, gen.IndexPagesParams{NotebookID: notebookID, TypeIs: typeIs, TrimChars: lintTrimChars})
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
	rows, err := s.queries(ctx).IndexItems(ctx, gen.IndexItemsParams{Ids: ids, TrimChars: lintTrimChars})
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

// CatalogueHeads reads each requested ID once, in the same sorted order.
func (s *Store) CatalogueHeads(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]app.IndexCatalogueHead, error) {
	if len(ids) > 512 {
		return nil, fmt.Errorf("index: catalogue batch exceeds 512 IDs")
	}
	for i, id := range ids {
		if i > 0 && ids[i-1].Compare(id) >= 0 {
			return nil, fmt.Errorf("index: catalogue IDs must be sorted and unique")
		}
	}
	rows, err := s.queries(ctx).IndexCatalogueHeads(ctx, gen.IndexCatalogueHeadsParams{NotebookID: notebookID, Ids: ids, TrimChars: lintTrimChars})
	if err != nil {
		return nil, fmt.Errorf("read index catalogue: %w", err)
	}
	if len(rows) != len(ids) {
		return nil, fmt.Errorf("index: incomplete catalogue batch")
	}
	out := make([]app.IndexCatalogueHead, len(rows))
	for i, r := range rows {
		if r.ID != ids[i] || !r.Typed && r.FilterType != "" || len(r.FilterType) > domain.MaxIndexType {
			return nil, fmt.Errorf("index: invalid catalogue row")
		}
		out[i] = app.IndexCatalogueHead{ID: r.ID, Typed: r.Typed}
		if r.FilterType != "" {
			out[i].FilterType = &r.FilterType
		}
	}
	return out, nil
}
