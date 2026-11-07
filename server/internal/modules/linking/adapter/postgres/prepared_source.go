package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

var _ app.SourcePreparations = (*Store)(nil)

// NewSource performs no SQL. The private app path already proved the source's
// Before/current head under the notebook/index locks. Borrowed facts belong
// to that unit and remain read-only through Apply; no AST or old text is kept.
func (s *Store) NewSource(ctx context.Context, unit *app.SourcePreparationUnit, page app.Page, facts domain.Facts) app.PreparedSource {
	tx, ok := postgres.TxFrom(ctx)
	if !ok || ctx.Err() != nil || unit == nil || unit.NotebookID != page.NotebookID || !retainablePageFacts(page, facts) {
		return nil
	}
	return &preparedSource{store: s, unit: unit, tx: tx, page: page, facts: facts}
}

type preparedSource struct {
	store *Store
	unit  *app.SourcePreparationUnit
	tx    pgx.Tx
	page  app.Page
	facts domain.Facts
	state uint8 // 0 new, 1 reading/failed, 2 complete (eligible or negative), 3 consumed
	read  preparedLinkRead
}

func (p *preparedSource) bound(ctx context.Context, unit *app.SourcePreparationUnit) bool {
	tx, ok := postgres.TxFrom(ctx)
	return ok && unit == p.unit && unit.NotebookID == p.page.NotebookID && tx == p.tx
}

func (p *preparedSource) ReadBefore(ctx context.Context, unit *app.SourcePreparationUnit) ([]uuid.UUID, error) {
	if p.state != 0 || !p.bound(ctx, unit) {
		return nil, errors.New("prepared source: invalid read scope or state")
	}
	p.state = 1
	// Compute every original Keys result before the native query; the other
	// incoming values remain borrowed until the complete read selects a plan.
	links := newPreparedLinkFacts(p.page, p.facts.Links)
	read, err := inspectPageLinksForPreparation(ctx, p.tx, gen.InspectPageLinksForPreparationParams{
		SourceID: p.page.ID, NotebookID: p.page.NotebookID, Extractor: domain.Extractor}, &links)
	if err != nil {
		return nil, fmt.Errorf("inspect the retained links of page %s: %w", p.page.ID, err)
	}
	assets := read.assets
	read.assets = nil // transfer to the original app metadata qualification
	p.read, p.state = read, 2
	if read.canRetain {
		p.facts.Links = nil
	} // Apply needs side facts, not a second incoming link slice.
	return assets, nil
}

func (p *preparedSource) Apply(ctx context.Context, unit *app.SourcePreparationUnit) (app.Dropped, *app.SourceLinks, error) {
	if p.state != 2 || !p.bound(ctx, unit) {
		return app.Dropped{}, nil, errors.New("prepared source: invalid apply scope or state")
	}
	p.state = 3 // Even a failed write cannot be retried through this handle.
	defer func() { p.read = preparedLinkRead{}; p.facts = domain.Facts{} }()
	if !p.read.canRetain {
		old, err := p.store.DeletePages(ctx, []uuid.UUID{p.page.ID})
		if err != nil {
			return app.Dropped{}, nil, err
		}
		if err := p.store.AddPage(ctx, p.page, p.facts); err != nil {
			return app.Dropped{}, nil, err
		}
		return old, nil, nil
	}
	old, err := p.store.replacePageRetaining(ctx, p.page, p.facts, p.read.links, p.read.targets, p.read.missing)
	if err != nil {
		return app.Dropped{}, nil, err
	}
	return old, &app.SourceLinks{SourceID: p.page.ID, Links: p.read.inputs}, nil
}

func (s *Store) LinksExcept(ctx context.Context, notebookID uuid.UUID, r domain.Reach, excluded []uuid.UUID) ([]app.Link, error) {
	// Remove the supplied source arm before planning, while SQL still excludes
	// those sources outside the complete OR to prevent key/target re-entry.
	set := make(map[uuid.UUID]bool, len(excluded))
	for _, id := range excluded {
		set[id] = true
	}
	sources := slices.DeleteFunc(slices.Clone(r.Sources), func(id uuid.UUID) bool { return set[id] })
	out, err := linksReachedExcept(ctx, postgres.Planned(postgres.DB(ctx, s.pool)), gen.LinksReachedExceptParams{
		NotebookID: notebookID, Keys: r.Keys, Targets: r.Targets, Sources: sources, ExcludedSources: excluded})
	if err != nil {
		return nil, fmt.Errorf("links of %s: %w", notebookID, err)
	}
	return out, nil
}

func sortPreparedInputs(inputs []app.Link) {
	byStart := func(a, b app.Link) int { return a.Start - b.Start }
	if !slices.IsSortedFunc(inputs, byStart) {
		slices.SortFunc(inputs, byStart)
	}
}
