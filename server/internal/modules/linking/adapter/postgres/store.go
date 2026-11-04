// Package postgresadapter is the linking module's repository adapter: the
// index's tables (M6/P3 design 3.2) through sqlc queries (queries/,
// generated into gen/) over the transaction that the context carries.
package postgresadapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"unicode/utf8"
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
// (postgres.Planned): for those a plan for any arguments makes much slower.
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

// ReplacePage implements app.Store. Retaining a comparison baseline is only
// valid in the caller's index-maintenance transaction, under its notebook/index
// locks, after any before-capture and before full fresh resolution. InTx alone
// does not prove that lifecycle; the internal Store contract requires it.
func (s *Store) ReplacePage(ctx context.Context, p app.Page, f domain.Facts) (app.Dropped, error) {
	if postgres.InTx(ctx) && ctx.Err() == nil && retainablePageFacts(p, f) {
		// All targets still go through the original per-link Keys computation.
		links := linksParams(p, f.Links)
		old, err := inspectPageLinksForRetention(ctx, postgres.DB(ctx, s.pool), gen.InspectPageLinksForRetentionParams{
			SourceID: p.ID, NotebookID: p.NotebookID, Extractor: domain.Extractor,
		}, &links)
		if err != nil {
			return app.Dropped{}, fmt.Errorf("inspect the retained links of page %s: %w", p.ID, err)
		}
		if old.canRetain {
			return s.replacePageRetaining(ctx, p, f, links, old.targets, old.missing)
		}
	}
	dropped, err := s.DeletePages(ctx, []uuid.UUID{p.ID})
	if err != nil {
		return app.Dropped{}, err
	}
	if err := s.AddPage(ctx, p, f); err != nil {
		return app.Dropped{}, err
	}
	return dropped, nil
}

// retainablePageFacts chooses a path, never validates or rewrites the caller's
// input. Noncanonical input keeps the original Delete/Add and its SQL errors.
// Nonempty targets that cannot parse are legal and must still be written.
func retainablePageFacts(p app.Page, f domain.Facts) bool {
	if p.ID == uuid.Nil() || p.NotebookID == uuid.Nil() || p.Revision < 1 || int(int32(p.Revision)) != p.Revision {
		return false
	}
	starts := make(map[int32]struct{}, len(f.Links))
	for _, l := range f.Links {
		start, end := int32(l.Start), int32(l.End)
		if start < 0 || end <= start || int(start) != l.Start || int(end) != l.End || l.Target == "" {
			return false
		}
		if _, duplicate := starts[start]; duplicate {
			return false
		}
		starts[start] = struct{}{}
		switch l.Kind {
		case "wikilink", "embed", "link", "image":
		default:
			return false
		}
		if !retentionText(l.Kind, l.Property, l.Target, l.Anchor, l.Display) {
			return false
		}
	}
	tags := make(map[string]struct{}, len(f.Tags))
	for _, t := range f.Tags {
		if t.Key == "" || t.Name == "" || t.Count < 1 || int(int32(t.Count)) != t.Count || !retentionText(t.Key, t.Name) {
			return false
		}
		if _, duplicate := tags[t.Key]; duplicate {
			return false
		}
		tags[t.Key] = struct{}{}
	}
	aliases := make(map[string]struct{}, len(f.Aliases))
	for _, a := range f.Aliases {
		if a.Key == "" || a.Name == "" || !retentionText(a.Key, a.Name) {
			return false
		}
		if _, duplicate := aliases[a.Key]; duplicate {
			return false
		}
		aliases[a.Key] = struct{}{}
	}
	for i, p := range f.Properties {
		if int(int32(i)) != i || !retentionText(p.Key) || bytes.IndexByte(p.Value, 0) >= 0 || !utf8.Valid(p.Value) {
			return false
		}
		// JSON syntax, numeric and escaped-Unicode validity stay with the original
		// InsertProperties value::jsonb at its original position, not Go json.Valid.
	}
	return true
}

func retentionText(values ...string) bool {
	for _, value := range values {
		if strings.IndexByte(value, 0) >= 0 || !utf8.ValidString(value) {
			return false
		}
	}
	return true
}

// Once Inspect selected this branch, every SQL failure returns its cause to the
// original transaction. Never retry it through legacy after a partial write.
func (s *Store) replacePageRetaining(ctx context.Context, p app.Page, f domain.Facts, links gen.InsertLinksParams, targets []uuid.UUID, missing []int32) (app.Dropped, error) {
	q := gen.New(retentionArrayQuerier{Querier: postgres.DB(ctx, s.pool)})
	if err := q.DeleteMissingPageLinks(ctx, gen.DeleteMissingPageLinksParams{SourceID: p.ID, MissingRangeStarts: missing}); err != nil {
		return app.Dropped{}, fmt.Errorf("delete the links of %d pages: %w", 1, err)
	}
	ids := []uuid.UUID{p.ID}
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
		return app.Dropped{}, fmt.Errorf("delete the index of %d pages: %w", 1, err)
	}
	err = q.InsertIndexedPage(ctx, gen.InsertIndexedPageParams{
		NodeID: p.ID, NotebookID: p.NotebookID, Revision: int32(p.Revision), Extractor: domain.Extractor,
		FrontmatterValid: f.FrontmatterValid,
	})
	if err == nil {
		err = q.WritePageLinksRetainingEqualPrefiltered(ctx, gen.WritePageLinksRetainingEqualPrefilteredParams(links))
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
		return app.Dropped{}, fmt.Errorf("index page %s: %w", p.ID, err)
	}
	if len(targets) == 0 {
		targets = nil
	}
	return app.Dropped{Targets: targets, AliasKeys: keys}, nil
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
	if n := len(links); n > 0 {
		out.RangeStarts = make([]int32, n)
		out.RangeEnds = make([]int32, n)
		out.Kinds = make([]string, n)
		out.PropertyKeys = make([]string, n)
		out.InFrontmatter = make([]bool, n)
		out.Targets = make([]string, n)
		out.Anchors = make([]string, n)
		out.Displays = make([]string, n)
		out.TargetKeys = make([]string, n)
		out.TargetAltKeys = make([]string, n)
		out.Aliases = make([]bool, n)
	}
	for i, l := range links {
		var key, alt string
		if keys := l.Keys(); len(keys) > 0 {
			key = keys[0]
			if len(keys) > 1 {
				alt = keys[1]
			}
		}
		out.RangeStarts[i] = int32(l.Start)
		out.RangeEnds[i] = int32(l.End)
		out.Kinds[i] = l.Kind
		out.PropertyKeys[i] = l.Property
		out.InFrontmatter[i] = l.InFrontmatter()
		out.Targets[i] = l.Target
		out.Anchors[i] = l.Anchor
		out.Displays[i] = l.Display
		out.TargetKeys[i] = key
		out.TargetAltKeys[i] = alt
		out.Aliases[i] = l.Aliases
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
// sources, in each unit that writes: a plan for any compared each link with
// them one by one, 75–83 ms of a rename where theirs took 16–26 ms (M6
// closeout FA5-Q1).
func (s *Store) Links(ctx context.Context, notebookID uuid.UUID, r domain.Reach) ([]app.Link, error) {
	out, err := linksReached(ctx, postgres.Planned(postgres.DB(ctx, s.pool)), notebookID, r)
	if err != nil {
		return nil, fmt.Errorf("links of %s: %w", notebookID, err)
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
			r = domain.Resolution{ID: *row.ResolvedID, Ambiguous: *row.Ambiguous, Asset: *row.ResolvedAsset}
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

// SetResolutions implements app.Store, planned with links: a plan for any
// links, which the server may make a cached statement's from its sixth
// run, joins them to the index's rows by comparing each with each when the
// statistics count none, as after the deletion of a notebook's pages
// before their restore; 20,000 links took 53 s where the plan for them
// took 0.1 s (M8/P2 design 3.11's measurement). A link it does not find,
// or finds twice, is a defect: the maintenance read each once, in the
// same transaction.
func (s *Store) SetResolutions(ctx context.Context, links []app.Link) error {
	if len(links) > resolutionWriteRows && postgres.InTx(ctx) && orderedResolutionKeys(links) {
		return s.setOrderedResolutions(ctx, links)
	}
	var p gen.SetResolutionsParams
	for _, l := range links {
		p.SourceIds = append(p.SourceIds, l.SourceID)
		p.RangeStarts = append(p.RangeStarts, int32(l.Start))
		p.ResolvedIds = append(p.ResolvedIds, l.Resolution.ID)
		p.Ambiguous = append(p.Ambiguous, l.Resolution.Ambiguous)
		p.ResolvedAssets = append(p.ResolvedAssets, l.Resolution.Asset)
	}
	n, err := s.planned(ctx).SetResolutions(ctx, p)
	if err != nil {
		return fmt.Errorf("set %d links' resolutions: %w", len(links), err)
	}
	if int(n) != len(links) {
		return fmt.Errorf("set %d links' resolutions, of %d", n, len(links))
	}
	return nil
}

// resolutionWriteRows bounds one statement's parameter arrays, never the
// complete input. Every eligible row is still written in the caller's transaction.
const resolutionWriteRows = 256

// orderedResolutionKeys admits only a globally strict SQL-key order. Checking
// every key before any write prevents duplicates across statement boundaries.
// Noncanonical callers keep the original single statement and its error behavior.
func orderedResolutionKeys(links []app.Link) bool {
	for i, l := range links {
		start := int32(l.Start)
		if start < 0 || int(start) != l.Start {
			return false
		}
		if i > 0 {
			previous := links[i-1]
			order := previous.SourceID.Compare(l.SourceID)
			if order > 0 || order == 0 && int32(previous.Start) >= start {
				return false
			}
		}
	}
	return true
}

func (s *Store) setOrderedResolutions(ctx context.Context, links []app.Link) error {
	p := gen.SetResolutionsParams{
		SourceIds:      make([]uuid.UUID, resolutionWriteRows),
		RangeStarts:    make([]int32, resolutionWriteRows),
		ResolvedIds:    make([]uuid.UUID, resolutionWriteRows),
		Ambiguous:      make([]bool, resolutionWriteRows),
		ResolvedAssets: make([]bool, resolutionWriteRows),
	}
	q := s.planned(ctx)
	var total int64
	for from := 0; from < len(links); from += resolutionWriteRows {
		batch := links[from:min(from+resolutionWriteRows, len(links))]
		p.SourceIds, p.RangeStarts = p.SourceIds[:len(batch)], p.RangeStarts[:len(batch)]
		p.ResolvedIds, p.Ambiguous = p.ResolvedIds[:len(batch)], p.Ambiguous[:len(batch)]
		p.ResolvedAssets = p.ResolvedAssets[:len(batch)]
		for i, l := range batch {
			p.SourceIds[i], p.RangeStarts[i] = l.SourceID, int32(l.Start)
			p.ResolvedIds[i] = l.Resolution.ID
			p.Ambiguous[i], p.ResolvedAssets[i] = l.Resolution.Ambiguous, l.Resolution.Asset
		}
		n, err := q.SetResolutions(ctx, p)
		if err != nil {
			return fmt.Errorf("set %d links' resolutions: %w", len(links), err)
		}
		total += n
	}
	// A later database/context error must win over an earlier missing row,
	// just as the original statement fails before its affected-row check.
	if int(total) != len(links) {
		return fmt.Errorf("set %d links' resolutions, of %d", total, len(links))
	}
	return nil
}

// Analyze updates the statistics of the module's tables an import writes
// (M7/P6 design 3.6).
func (s *Store) Analyze(ctx context.Context) error {
	if err := s.queries(ctx).Analyze(ctx); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	return nil
}
