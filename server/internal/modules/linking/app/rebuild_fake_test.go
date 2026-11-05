package app_test

import (
	"context"
	"fmt"
	"slices"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// These are the same in-memory tables used by the original app suite, with
// explicit Rebuild-only writes. They do not route through SetResolutions.
func (s *store) AddRebuildMetadata(ctx context.Context, p app.Page, f app.RebuildMetadata) error {
	return s.AddPage(ctx, p, domain.Facts{FrontmatterValid: f.FrontmatterValid, Tags: f.Tags, Properties: f.Properties, Aliases: f.Aliases})
}

func (s *store) AddRebuildUnresolvedLinks(_ context.Context, p app.Page, links []domain.Link) error {
	f, ok := s.facts[p.ID]
	if !ok {
		return fmt.Errorf("the staged page %s has no metadata", p.ID)
	}
	if len(f.Links) > 0 {
		return fmt.Errorf("the staged page %s already has links", p.ID)
	}
	f.Links = slices.Clone(links)
	s.facts[p.ID] = f
	for _, l := range links {
		s.links = append(s.links, app.Link{SourceID: p.ID, Start: int(int32(l.Start)), Target: l.Target, Aliases: l.Aliases})
	}
	return nil
}

func (s *store) AddRebuildResolvedLinks(ctx context.Context, p app.Page, links []domain.Link, resolved []domain.Resolution) error {
	if len(links) != len(resolved) {
		return fmt.Errorf("rebuilt row/result lengths differ")
	}
	first := len(s.links)
	if err := s.AddRebuildUnresolvedLinks(ctx, p, links); err != nil {
		return err
	}
	for i, result := range resolved {
		s.links[first+i].Resolution = result
	}
	return nil
}
