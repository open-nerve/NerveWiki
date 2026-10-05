package app

import (
	"context"
	"fmt"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// resolve resolves links, of notebookID, anew (M6/P3 design 3.4, step 7),
// and returns those that resolve otherwise, each with its new resolution.
// Under the notebook's lock, a link or an alias of a page the notebook
// does not have is a defect: an error.
func (x Index) resolve(ctx context.Context, notebookID uuid.UUID, links []Link) ([]Link, error) {
	rs, missing, err := resolutions(ctx, x.Store, x.Pages, notebookID, links)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("linking: the pages %v of links or aliases are not pages of %s", missing, notebookID)
	}
	var changed []Link
	for i, l := range links {
		if rs[i] != l.Resolution {
			l.Resolution = rs[i]
			changed = append(changed, l)
		}
	}
	return changed, nil
}

// resolutions is where links, of notebookID, resolve (M6/P3 design 3.4,
// step 7), in links' order: it reads the candidates of all of them at
// once, the pages with their last keys, the pages with an alias of them,
// and the paths of those and of the links' pages. A link whose page is not
// one of the notebook's resolves to none, and an alias whose page is not
// is none: missing has those pages, each once. A target that is no path
// resolves to none.
func resolutions(ctx context.Context, store Store, pages Pages, notebookID uuid.UUID, links []Link) (
	[]domain.Resolution, []uuid.UUID, error,
) {
	if len(links) == 0 {
		return nil, nil, nil
	}
	targets := make([]domain.Target, len(links))
	parsed := make([]bool, len(links))
	var keys []string
	ids := make([]uuid.UUID, 0, len(links))
	for i, l := range links {
		if targets[i], parsed[i] = domain.ParseTarget(l.Target); parsed[i] {
			keys = append(keys, targets[i].LastKeys()...)
		}
		ids = append(ids, l.SourceID)
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	candidates, err := pages.ByKeys(ctx, notebookID, keys)
	if err != nil {
		return nil, nil, err
	}
	byKey := make(map[string][]domain.Node)
	for _, n := range candidates {
		key := n.Path[len(n.Path)-1].Key
		byKey[key] = append(byKey[key], n)
	}
	aliases, err := store.Aliases(ctx, notebookID, keys)
	if err != nil {
		return nil, nil, err
	}
	aliasedBy := make(map[string][]uuid.UUID)
	for _, a := range aliases {
		aliasedBy[a.Key] = append(aliasedBy[a.Key], a.PageID)
		ids = append(ids, a.PageID)
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	nodes, err := pages.Paths(ctx, notebookID, slices.Compact(ids))
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[uuid.UUID]domain.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	var missing []uuid.UUID
	out := make([]domain.Resolution, len(links))
	for i, l := range links {
		if !parsed[i] {
			continue
		}
		from, ok := byID[l.SourceID]
		if !ok {
			missing = append(missing, l.SourceID)
			continue
		}
		var named []domain.Node
		aliased := make(map[string][]domain.Node)
		for _, key := range targets[i].LastKeys() {
			named = append(named, byKey[key]...)
			for _, id := range aliasedBy[key] {
				if n, ok := byID[id]; ok {
					aliased[key] = append(aliased[key], n)
				} else {
					missing = append(missing, id)
				}
			}
		}
		out[i] = domain.Resolve(targets[i], from.Path, named, aliased)
	}
	slices.SortFunc(missing, uuid.UUID.Compare)
	return out, slices.Compact(missing), nil
}
