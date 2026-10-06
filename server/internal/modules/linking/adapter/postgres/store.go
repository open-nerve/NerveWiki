// Package postgresadapter is the linking module's repository adapter: the
// index's tables (M6/P3 design 3.2) through sqlc queries (queries/,
// generated into gen/) over the transaction that the context carries.
package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// lockSpace is the first key of the index's locks, the second the
// notebook's hash: two notebooks that hash alike share a lock, which
// costs them only waiting.
const lockSpace = 0x6c696e6b // "link"

// Store implements app.Store.
type Store struct {
	pool *pgxpool.Pool
}

// New returns the store over pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// queries runs in the context's transaction when there is one.
func (s *Store) queries(ctx context.Context) *gen.Queries {
	return gen.New(postgres.DB(ctx, s.pool))
}

// planned is queries with each statement planned with its arguments
// (postgres.Planned): for those whose arrays grow with the data.
func (s *Store) planned(ctx context.Context) *gen.Queries {
	return gen.New(postgres.Planned(postgres.DB(ctx, s.pool)))
}

// Lock implements app.Store. On the pool the lock would end with its
// statement: it is refused there.
func (s *Store) Lock(ctx context.Context, notebookID uuid.UUID) error {
	if !postgres.InTx(ctx) {
		return errors.New("the index's lock is taken outside a transaction")
	}
	h := fnv.New32a()
	_, _ = h.Write(notebookID[:])
	if err := s.queries(ctx).LockNotebook(ctx, gen.LockNotebookParams{Space: lockSpace, Key: int32(h.Sum32())}); err != nil {
		return fmt.Errorf("lock the index of %s: %w", notebookID, err)
	}
	return nil
}

// ReplacePage implements app.Store.
func (s *Store) ReplacePage(ctx context.Context, p app.Page, f domain.Facts) (app.Dropped, error) {
	dropped, err := s.DeletePages(ctx, []uuid.UUID{p.ID})
	if err != nil {
		return app.Dropped{}, err
	}
	if err := s.AddPage(ctx, p, f); err != nil {
		return app.Dropped{}, err
	}
	return dropped, nil
}

// AddPage implements app.Store.
func (s *Store) AddPage(ctx context.Context, p app.Page, f domain.Facts) error {
	q := s.queries(ctx)
	err := q.InsertIndexedPage(ctx, gen.InsertIndexedPageParams{
		NodeID: p.ID, NotebookID: p.NotebookID, Revision: int32(p.Revision), Extractor: domain.Extractor,
		FrontmatterValid: f.FrontmatterValid,
	})
	if err == nil {
		err = q.InsertLinks(ctx, linksParams(p, f.Links))
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

func linksParams(p app.Page, links []domain.Link) gen.InsertLinksParams {
	out := gen.InsertLinksParams{SourceID: p.ID, NotebookID: p.NotebookID}
	for _, l := range links {
		var key, alt string
		if keys := l.Keys(); len(keys) > 0 {
			key = keys[0]
			if len(keys) > 1 {
				alt = keys[1]
			}
		}
		out.RangeStarts = append(out.RangeStarts, int32(l.Start))
		out.RangeEnds = append(out.RangeEnds, int32(l.End))
		out.Kinds = append(out.Kinds, l.Kind)
		out.PropertyKeys = append(out.PropertyKeys, l.Property)
		out.Targets = append(out.Targets, l.Target)
		out.Anchors = append(out.Anchors, l.Anchor)
		out.Displays = append(out.Displays, l.Display)
		out.TargetKeys = append(out.TargetKeys, key)
		out.TargetAltKeys = append(out.TargetAltKeys, alt)
		out.Aliases = append(out.Aliases, l.Aliases)
	}
	return out
}

func tagsParams(p app.Page, tags []domain.Tag) gen.InsertTagsParams {
	out := gen.InsertTagsParams{SourceID: p.ID, NotebookID: p.NotebookID}
	for _, t := range tags {
		out.TagKeys = append(out.TagKeys, t.Key)
		out.Tags = append(out.Tags, t.Name)
		out.Counts = append(out.Counts, int32(t.Count))
	}
	return out
}

func propertiesParams(p app.Page, props []domain.Property) gen.InsertPropertiesParams {
	out := gen.InsertPropertiesParams{SourceID: p.ID, NotebookID: p.NotebookID}
	for i, prop := range props {
		out.Positions = append(out.Positions, int32(i))
		out.Keys = append(out.Keys, prop.Key)
		out.Values = append(out.Values, string(prop.Value))
	}
	return out
}

func aliasesParams(p app.Page, aliases []domain.Alias) gen.InsertAliasesParams {
	out := gen.InsertAliasesParams{SourceID: p.ID, NotebookID: p.NotebookID}
	for _, a := range aliases {
		out.AliasKeys = append(out.AliasKeys, a.Key)
		out.Aliases = append(out.Aliases, a.Name)
	}
	return out
}

// DeletePages implements app.Store.
func (s *Store) DeletePages(ctx context.Context, ids []uuid.UUID) (app.Dropped, error) {
	q := s.queries(ctx)
	targets, err := q.DeleteLinksOf(ctx, ids)
	if err != nil {
		return app.Dropped{}, fmt.Errorf("delete the links of %d pages: %w", len(ids), err)
	}
	keys, err := q.DeleteAliasesOf(ctx, ids)
	if err == nil {
		err = q.DeleteTagsOf(ctx, ids)
	}
	if err == nil {
		err = q.DeletePropertiesOf(ctx, ids)
	}
	if err == nil {
		err = q.DeleteIndexedPages(ctx, ids)
	}
	if err != nil {
		return app.Dropped{}, fmt.Errorf("delete the index of %d pages: %w", len(ids), err)
	}
	return app.Dropped{Targets: targets, AliasKeys: keys}, nil
}

// DeleteNotebooks implements app.Store.
func (s *Store) DeleteNotebooks(ctx context.Context, ids []uuid.UUID) error {
	q := s.queries(ctx)
	for _, del := range []func(context.Context, []uuid.UUID) error{
		q.DeleteNotebooksLinks, q.DeleteNotebooksTags, q.DeleteNotebooksProperties, q.DeleteNotebooksAliases,
		q.DeleteNotebooksIndexedPages,
	} {
		if err := del(ctx, ids); err != nil {
			return fmt.Errorf("delete the index of %d notebooks: %w", len(ids), err)
		}
	}
	return nil
}

// Links implements app.Store, read planned with r's keys, targets and
// sources: a plan for any compared each link with them one by one, 75–83 ms
// of a rename where theirs took 16–26 ms (M6 closeout FA5-Q1).
func (s *Store) Links(ctx context.Context, notebookID uuid.UUID, r domain.Reach) ([]app.Link, error) {
	rows, err := s.planned(ctx).LinksReached(ctx, gen.LinksReachedParams{
		NotebookID: notebookID, Keys: r.Keys, Targets: r.Targets, Sources: r.Sources,
	})
	if err != nil {
		return nil, fmt.Errorf("links of %s: %w", notebookID, err)
	}
	out := make([]app.Link, len(rows))
	for i, row := range rows {
		out[i] = app.Link{SourceID: row.SourceID, Start: int(row.RangeStart), Target: row.Target, Aliases: row.Aliases}
		if row.ResolvedID != nil {
			out[i].Resolution = domain.Resolution{ID: *row.ResolvedID, Ambiguous: row.Ambiguous}
		}
	}
	return out, nil
}

// View implements app.Store.
func (s *Store) View(ctx context.Context, id uuid.UUID) (app.Indexed, bool, error) {
	rows, err := s.queries(ctx).PageView(ctx, id)
	if err != nil {
		return app.Indexed{}, false, fmt.Errorf("the index of %s: %w", id, err)
	}
	if len(rows) == 0 {
		return app.Indexed{}, false, nil
	}
	out := app.Indexed{
		Revision: int(rows[0].Revision), Extractor: int(rows[0].Extractor),
		Resolutions: make(map[int]domain.Resolution, len(rows)),
	}
	for _, row := range rows {
		if row.RangeStart == nil {
			continue // no link
		}
		var r domain.Resolution
		if row.ResolvedID != nil {
			r = domain.Resolution{ID: *row.ResolvedID, Ambiguous: *row.Ambiguous}
		}
		out.Resolutions[int(*row.RangeStart)] = r
	}
	return out, true, nil
}

// IndexedOf implements app.Store.
func (s *Store) IndexedOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.Indexed, error) {
	rows, err := s.queries(ctx).IndexedPagesOf(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("the index of %d pages: %w", len(ids), err)
	}
	out := make(map[uuid.UUID]app.Indexed, len(rows))
	for _, row := range rows {
		out[row.NodeID] = app.Indexed{Revision: int(row.Revision), Extractor: int(row.Extractor)}
	}
	return out, nil
}

// Aliases implements app.Store.
func (s *Store) Aliases(ctx context.Context, notebookID uuid.UUID, keys []string) ([]app.Alias, error) {
	rows, err := s.queries(ctx).AliasesByKeys(ctx, gen.AliasesByKeysParams{NotebookID: notebookID, Keys: keys})
	if err != nil {
		return nil, fmt.Errorf("aliases of %s: %w", notebookID, err)
	}
	out := make([]app.Alias, len(rows))
	for i, row := range rows {
		out[i] = app.Alias{PageID: row.SourceID, Key: row.AliasKey}
	}
	return out, nil
}

// AliasKeys implements app.Store.
func (s *Store) AliasKeys(ctx context.Context, ids []uuid.UUID) ([]string, error) {
	keys, err := s.queries(ctx).AliasKeysOf(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("the alias keys of %d pages: %w", len(ids), err)
	}
	return keys, nil
}

// SetResolutions implements app.Store. A link it does not find, or finds
// twice, is a defect: the maintenance read each once, in the same
// transaction.
func (s *Store) SetResolutions(ctx context.Context, links []app.Link) error {
	var p gen.SetResolutionsParams
	for _, l := range links {
		p.SourceIds = append(p.SourceIds, l.SourceID)
		p.RangeStarts = append(p.RangeStarts, int32(l.Start))
		p.ResolvedIds = append(p.ResolvedIds, l.Resolution.ID)
		p.Ambiguous = append(p.Ambiguous, l.Resolution.Ambiguous)
	}
	n, err := s.queries(ctx).SetResolutions(ctx, p)
	if err != nil {
		return fmt.Errorf("set %d links' resolutions: %w", len(links), err)
	}
	if int(n) != len(links) {
		return fmt.Errorf("set %d links' resolutions, of %d", n, len(links))
	}
	return nil
}
