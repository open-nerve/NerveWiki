package postgresadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Backlinks implements app.Reads.
func (s *Store) Backlinks(ctx context.Context, target, after uuid.UUID, size, count, contexts int) ([]app.Backlink, error) {
	rows, err := s.queries(ctx).Backlinks(ctx, gen.BacklinksParams{
		Target: target, After: after, Size: int32(size), MaxCount: int32(count), Contexts: int32(contexts),
	})
	if err != nil {
		return nil, fmt.Errorf("the backlinks of %s: %w", target, err)
	}
	var out []app.Backlink
	for _, row := range rows {
		if len(out) == 0 || out[len(out)-1].SourceID != row.SourceID {
			out = append(out, app.Backlink{
				SourceID: row.SourceID, Revision: int(row.Revision), Extractor: int(row.Extractor), Links: int(row.Links),
			})
		}
		last := &out[len(out)-1]
		last.Ranges = append(last.Ranges, domain.Range{Start: int(row.RangeStart), End: int(row.RangeEnd)})
	}
	return out, nil
}

// Properties implements app.Reads.
func (s *Store) Properties(ctx context.Context, id uuid.UUID) (app.Properties, bool, error) {
	row, err := s.queries(ctx).PageProperties(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return app.Properties{}, false, nil
	case err != nil:
		return app.Properties{}, false, fmt.Errorf("the properties of %s: %w", id, err)
	}
	if len(row.PropertyValues) != len(row.Keys) || len(row.LinkIds) != len(row.LinkKeys) {
		return app.Properties{}, false, fmt.Errorf("the properties of %s: keys and values differ in number", id)
	}
	out := app.Properties{Valid: row.FrontmatterValid}
	for i, key := range row.Keys {
		out.Properties = append(out.Properties, app.Property{Key: key, Value: json.RawMessage(row.PropertyValues[i])})
	}
	for i, key := range row.LinkKeys {
		out.Links = append(out.Links, app.PropertyLink{Key: key, NodeID: row.LinkIds[i]})
	}
	return out, true, nil
}

// Tags implements app.Reads.
func (s *Store) Tags(ctx context.Context, notebookID uuid.UUID) ([]app.Tag, error) {
	rows, err := s.queries(ctx).Tags(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("the tags of %s: %w", notebookID, err)
	}
	out := make([]app.Tag, len(rows))
	for i, row := range rows {
		out[i] = app.Tag{Tag: row.Tag, Pages: int(row.Pages)}
	}
	return out, nil
}

// TagPages implements app.Reads.
func (s *Store) TagPages(ctx context.Context, notebookID uuid.UUID, key string) ([]uuid.UUID, error) {
	ids, err := s.queries(ctx).TagPages(ctx, gen.TagPagesParams{NotebookID: notebookID, Key: key})
	if err != nil {
		return nil, fmt.Errorf("the pages of a tag of %s: %w", notebookID, err)
	}
	return ids, nil
}

// NotebookAliases implements app.Reads.
func (s *Store) NotebookAliases(ctx context.Context, notebookID uuid.UUID) (map[uuid.UUID][]string, error) {
	rows, err := s.queries(ctx).NotebookAliases(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("the aliases of %s: %w", notebookID, err)
	}
	out := map[uuid.UUID][]string{}
	for _, row := range rows {
		out[row.SourceID] = append(out[row.SourceID], row.Alias)
	}
	return out, nil
}
