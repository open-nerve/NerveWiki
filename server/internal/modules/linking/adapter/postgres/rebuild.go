package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

var _ app.RebuildWrites = (*Store)(nil)

// These methods are only the locked Rebuild protocol's write side. They do
// not change Store.AddPage or infer the maintenance lifecycle from InTx.
func (s *Store) rebuildQueries(ctx context.Context) (*gen.Queries, error) {
	if !postgres.InTx(ctx) {
		return nil, errors.New("rebuild writes require a transaction")
	}
	return s.queries(ctx), nil
}

func (s *Store) AddRebuildMetadata(ctx context.Context, p app.Page, f app.RebuildMetadata) error {
	q, err := s.rebuildQueries(ctx)
	if err == nil {
		err = q.InsertIndexedPage(ctx, gen.InsertIndexedPageParams{
			NodeID: p.ID, NotebookID: p.NotebookID, Revision: int32(p.Revision), Extractor: domain.Extractor,
			FrontmatterValid: f.FrontmatterValid,
		})
	}
	if err == nil {
		err = q.InsertTags(ctx, tagsParams(p, f.Tags))
	}
	if err == nil {
		err = q.InsertProperties(ctx, propertiesParams(p, f.Properties))
	}
	if err == nil {
		err = q.InsertAliases(ctx, aliasesParams(p, f.Aliases))
	}
	if err != nil {
		return fmt.Errorf("index page %s: %w", p.ID, err)
	}
	return nil
}

func (s *Store) AddRebuildUnresolvedLinks(ctx context.Context, p app.Page, links []domain.Link) error {
	q, err := s.rebuildQueries(ctx)
	if err == nil {
		err = q.InsertLinks(ctx, linksParams(p, links))
	}
	if err != nil {
		return fmt.Errorf("index page %s: %w", p.ID, err)
	}
	return nil
}

func (s *Store) AddRebuildResolvedLinks(ctx context.Context, p app.Page, links []domain.Link, resolved []domain.Resolution) error {
	if len(links) != len(resolved) {
		return fmt.Errorf("index page %s: %d rebuilt links have %d resolutions", p.ID, len(links), len(resolved))
	}
	q, err := s.rebuildQueries(ctx)
	if err != nil {
		return fmt.Errorf("index page %s: %w", p.ID, err)
	}
	args := linksParams(p, links)
	var ids []uuid.UUID
	var ambiguous, assets []bool
	if len(resolved) > 0 {
		ids = make([]uuid.UUID, len(resolved))
		ambiguous, assets = make([]bool, len(resolved)), make([]bool, len(resolved))
	}
	for i, result := range resolved {
		ids[i], ambiguous[i], assets[i] = result.ID, result.Ambiguous, result.Asset
	}
	err = q.InsertRebuiltLinks(ctx, gen.InsertRebuiltLinksParams{
		SourceID: args.SourceID, NotebookID: args.NotebookID,
		RangeStarts: args.RangeStarts, RangeEnds: args.RangeEnds, Kinds: args.Kinds,
		PropertyKeys: args.PropertyKeys, InFrontmatter: args.InFrontmatter, Targets: args.Targets,
		Anchors: args.Anchors, Displays: args.Displays, TargetKeys: args.TargetKeys, TargetAltKeys: args.TargetAltKeys,
		Aliases: args.Aliases, ResolvedIds: ids, Ambiguous: ambiguous, ResolvedAssets: assets,
	})
	if err != nil {
		return fmt.Errorf("index page %s: %w", p.ID, err)
	}
	return nil
}
