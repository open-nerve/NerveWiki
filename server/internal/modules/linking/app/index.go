package app

import (
	"context"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// PagesChanged is a page write unit's changes in a notebook, merged by
// node, as the page module's observers get them once per unit.
type PagesChanged struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	Changes     []domain.Change
}

// Index keeps the link index with the writes of the pages (M6/P3 design
// 3.4): the page module's observer, in each unit's transaction.
type Index struct {
	Store     Store
	Pages     Pages
	Publisher Publisher
}

// PagesChanged follows the unit e: under the notebook's lock of the index,
// it has the rows of the pages whose content e wrote hold their facts and
// drops those of the pages it deleted, resolves anew the links e reaches,
// sets those that resolve otherwise, and publishes the links event of
// what changed. It reaches what domain.Affected does, the pages under the
// nodes it renamed, the keys of every alias of those pages and of those it
// deleted, and the keys of the aliases a content it wrote drops or adds.
func (x Index) PagesChanged(ctx context.Context, e PagesChanged) error {
	reach, renamed, ok := domain.Affected(e.Changes)
	if !ok {
		return nil
	}
	if err := x.Store.Lock(ctx, e.NotebookID); err != nil {
		return err
	}
	var written, deleted, dropped []uuid.UUID
	for _, c := range e.Changes {
		switch {
		case c.After != nil && c.Revision != 0:
			old, err := x.Store.ReplacePage(ctx, Page{ID: c.NodeID, NotebookID: e.NotebookID, Revision: c.Revision}, c.Facts)
			if err != nil {
				return err
			}
			written, dropped = append(written, c.NodeID), append(dropped, old.Targets...)
			reach.Keys = append(reach.Keys, changedAliases(old.AliasKeys, c.Facts.Aliases)...)
		case c.After == nil && c.Before != nil:
			deleted = append(deleted, c.NodeID)
		}
	}
	if len(deleted) > 0 {
		old, err := x.Store.DeletePages(ctx, deleted)
		if err != nil {
			return err
		}
		dropped = append(dropped, old.Targets...)
		reach.Keys = append(reach.Keys, old.AliasKeys...)
	}
	reach, err := spread(ctx, x.Store, x.Pages, e.NotebookID, reach, renamed)
	if err != nil {
		return err
	}
	links, err := x.Store.Links(ctx, e.NotebookID, reach)
	if err != nil {
		return err
	}
	changed, err := x.resolve(ctx, e.NotebookID, links)
	if err != nil {
		return err
	}
	if err := x.Store.SetResolutions(ctx, changed); err != nil {
		return err
	}
	return x.publish(ctx, e, links, changed, written, dropped)
}

// spread has reach, a unit's, reach the pages under the nodes renamed,
// their links and those to them, and the keys of every alias of the pages
// it reaches: a page's path decides between the pages with an alias as
// between those with a name.
func spread(ctx context.Context, store Store, pages Pages, notebookID uuid.UUID, reach domain.Reach, renamed []uuid.UUID) (
	domain.Reach, error,
) {
	for _, id := range renamed {
		sub, err := pages.Subtree(ctx, notebookID, id)
		if err != nil {
			return domain.Reach{}, err
		}
		reach.Add(sub...)
	}
	aliasKeys, err := store.AliasKeys(ctx, reach.Targets)
	if err != nil {
		return domain.Reach{}, err
	}
	reach.Keys = append(reach.Keys, aliasKeys...)
	reach.Compact()
	return reach, nil
}

// publish publishes the links event of the unit e, which wrote the content
// of written, dropped links to dropped and resolved changed, of links,
// otherwise: the pages whose links resolve to another page, but for
// written, which the pages event tells; and the pages whose backlinks
// changed, those the links resolved to before or now, and those that
// written's links resolved to before. A link whose tie alone changed
// changes neither. Nothing changed publishes nothing.
func (x Index) publish(ctx context.Context, e PagesChanged, links, changed []Link, written, dropped []uuid.UUID) error {
	before := make(map[linkKey]domain.Resolution, len(links))
	for _, l := range links {
		before[keyOf(l)] = l.Resolution
	}
	var pages []uuid.UUID
	targets := slices.Clone(dropped)
	for _, l := range changed {
		old := before[keyOf(l)]
		if old.ID == l.Resolution.ID {
			continue
		}
		if !slices.Contains(written, l.SourceID) {
			pages = append(pages, l.SourceID)
		}
		targets = append(targets, old.ID, l.Resolution.ID)
	}
	targets = slices.DeleteFunc(targets, func(id uuid.UUID) bool { return id == uuid.Nil() })
	if len(pages) == 0 && len(targets) == 0 {
		return nil
	}
	return x.Publisher.LinksChanged(ctx, LinksChanged{
		WorkspaceID: e.WorkspaceID, NotebookID: e.NotebookID, Pages: eventPages(pages), Targets: eventPages(targets),
	})
}

// changedAliases is the keys of the aliases a page had, before, or has,
// after, but not both.
func changedAliases(before []string, after []domain.Alias) []string {
	now := make(map[string]bool, len(after))
	for _, a := range after {
		now[a.Key] = true
	}
	var out []string
	for _, k := range before {
		if !now[k] {
			out = append(out, k)
		}
		delete(now, k)
	}
	for _, a := range after {
		if now[a.Key] {
			out = append(out, a.Key)
		}
	}
	return out
}

// linkKey is a link's key: its page and where its target is written.
type linkKey struct {
	source uuid.UUID
	start  int
}

func keyOf(l Link) linkKey {
	return linkKey{source: l.SourceID, start: l.Start}
}

// eventPages is ids as a links event lists them: sorted, each once, empty
// for none and nil past MaxEventPages.
func eventPages(ids []uuid.UUID) []uuid.UUID {
	slices.SortFunc(ids, uuid.UUID.Compare)
	ids = slices.Compact(ids)
	if len(ids) > MaxEventPages {
		return nil
	}
	return append([]uuid.UUID{}, ids...)
}
