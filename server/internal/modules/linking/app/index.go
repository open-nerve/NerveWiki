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
	WorkspaceID  uuid.UUID
	NotebookID   uuid.UUID
	Changes      []domain.Change
	ChangesetID  uuid.UUID
	Kind         string
	Notebook     IngestNotebook
	BeforeIngest IngestBefore
}

// Index keeps the link index with the writes of the pages (M6/P3 design
// 3.4): the page module's observer, in each unit's transaction.
type Index struct {
	Store        Store
	Preparations SourcePreparations
	Pages        Pages
	Publisher    Publisher
	Ingest       *Ingest
}

// PagesChanged follows the unit e: under the notebook's lock of the index,
// it has the rows of the pages whose content e wrote hold their facts and
// drops those of the pages it deleted, resolves anew the links e reaches,
// sets those that resolve otherwise, and publishes the links event of
// what changed. It reaches what domain.Affected does, the pages and
// attachments under the nodes it renamed, the keys of every alias of those
// pages and of those it deleted, and the keys of the aliases a content it
// wrote drops or adds: those two change the notebook's aliases, which the
// event tells.
func (x Index) PagesChanged(ctx context.Context, e PagesChanged) error {
	reach, renamed, ok := domain.Affected(e.Changes)
	if !ok {
		return nil
	}
	if err := x.Store.Lock(ctx, e.NotebookID); err != nil {
		return err
	}
	ingest, err := x.beginIngest(ctx, e)
	if err != nil {
		return err
	}
	var written, deleted, dropped []uuid.UUID
	aliases := 0 // the keys of the aliases the unit dropped or added
	for _, c := range e.Changes {
		switch {
		case c.After != nil && c.Revision != 0:
			old, err := ingest.replace(ctx, x.Store, Page{ID: c.NodeID, NotebookID: e.NotebookID, Revision: c.Revision}, c.Facts)
			if err != nil {
				return err
			}
			written, dropped = append(written, c.NodeID), append(dropped, old.Targets...)
			changed := changedAliases(old.AliasKeys, c.Facts.Aliases)
			reach.Keys = append(reach.Keys, changed...)
			aliases += len(changed)
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
		aliases += len(old.AliasKeys)
	}
	reach, err = spread(ctx, x.Store, x.Pages, e.NotebookID, reach, renamed, nil)
	if err != nil {
		return err
	}
	links, err := ingest.links(ctx, x.Store, e.NotebookID, reach)
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
	if err := ingest.finish(ctx, written); err != nil {
		return err
	}
	return x.publish(ctx, e, links, changed, written, dropped, aliases > 0)
}

// spread has reach, a unit's, reach the pages and attachments under the
// nodes renamed, their links and those to them, and the keys of every
// alias of the pages it reaches: a page's path decides between the pages
// with an alias as between those with a name. The nodes of after, by the
// node renamed, which a batch created after its rename, are not reached
// (Moved.CreatedAfter); a unit's observers give none.
func spread(ctx context.Context, store Store, pages Pages, notebookID uuid.UUID, reach domain.Reach, renamed []uuid.UUID,
	after map[uuid.UUID]map[uuid.UUID]bool,
) (domain.Reach, error) {
	for _, id := range renamed {
		sub, err := pages.Subtree(ctx, notebookID, id)
		if err != nil {
			return domain.Reach{}, err
		}
		reach.Add(stepsNot(sub, after[id])...)
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
// otherwise: the pages whose links resolve to another node, but for
// written, which the pages event tells; and the nodes whose backlinks
// changed, those the links resolved to before or now, and those that
// written's links resolved to before. A link whose tie alone changed
// changes neither. aliases tells that the unit changed the notebook's
// aliases, which a link no page of the index holds may resolve by, a
// version's (M8 Codex review R3): an event of its own when nothing else
// changed. Nothing changed publishes nothing.
func (x Index) publish(ctx context.Context, e PagesChanged, links, changed []Link, written, dropped []uuid.UUID, aliases bool) error {
	nonzero := 0
	for _, l := range links {
		if l.Resolution.ID != uuid.Nil() {
			nonzero++
		}
	}
	before := make(map[linkKey]uuid.UUID)
	if len(changed) < nonzero {
		for _, l := range changed {
			before[keyOf(l)] = uuid.Nil()
		}
		for _, l := range links {
			key := keyOf(l)
			if _, needed := before[key]; needed {
				before[key] = l.Resolution.ID
			}
		}
	} else {
		for _, l := range links {
			if l.Resolution.ID == uuid.Nil() {
				delete(before, keyOf(l))
			} else {
				before[keyOf(l)] = l.Resolution.ID
			}
		}
	}
	writtenSet := make(map[uuid.UUID]struct{}, len(written))
	for _, id := range written {
		writtenSet[id] = struct{}{}
	}
	var pages, targets []uuid.UUID
	for _, id := range dropped {
		if id != uuid.Nil() {
			targets = appendEventPage(targets, id)
		}
	}
	for _, l := range changed {
		oldID := before[keyOf(l)]
		if oldID == l.Resolution.ID {
			continue
		}
		if _, ok := writtenSet[l.SourceID]; !ok {
			pages = appendEventPage(pages, l.SourceID)
		}
		if oldID != uuid.Nil() {
			targets = appendEventPage(targets, oldID)
		}
		if l.Resolution.ID != uuid.Nil() {
			targets = appendEventPage(targets, l.Resolution.ID)
		}
	}
	if len(pages) == 0 && len(targets) == 0 && !aliases {
		return nil
	}
	return x.Publisher.LinksChanged(ctx, LinksChanged{
		WorkspaceID: e.WorkspaceID, NotebookID: e.NotebookID, Pages: eventPages(pages), Targets: eventPages(targets), Aliases: aliases,
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

// appendEventPage keeps each ID once, plus one distinct ID past the event
// bound as an overflow witness. A nonempty overflow must still publish.
func appendEventPage(ids []uuid.UUID, id uuid.UUID) []uuid.UUID {
	if len(ids) > MaxEventPages || slices.Contains(ids, id) {
		return ids
	}
	return append(ids, id)
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
