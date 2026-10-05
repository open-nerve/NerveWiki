package app

import (
	"context"
	"fmt"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// resolve resolves links, of notebookID, anew (M6/P3 design 3.4, step 7),
// and returns those that resolve otherwise, each with its new resolution:
// it reads the candidates of all of them at once, the pages with their
// last keys, the pages with an alias of them, and the paths of those and
// of the links' pages.
func (x Index) resolve(ctx context.Context, notebookID uuid.UUID, links []Link) ([]Link, error) {
	if len(links) == 0 {
		return nil, nil
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
	candidates, err := x.Pages.ByKeys(ctx, notebookID, keys)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string][]domain.Node)
	for _, n := range candidates {
		key := n.Path[len(n.Path)-1].Key
		byKey[key] = append(byKey[key], n)
	}
	aliases, err := x.Store.Aliases(ctx, notebookID, keys)
	if err != nil {
		return nil, err
	}
	aliasedBy := make(map[string][]uuid.UUID)
	for _, a := range aliases {
		aliasedBy[a.Key] = append(aliasedBy[a.Key], a.PageID)
		ids = append(ids, a.PageID)
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	nodes, err := x.Pages.Paths(ctx, notebookID, slices.Compact(ids))
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]domain.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	var changed []Link
	for i, l := range links {
		var r domain.Resolution
		if parsed[i] {
			from, ok := byID[l.SourceID]
			if !ok {
				return nil, fmt.Errorf("linking: the page %s of a link is not a page of %s", l.SourceID, notebookID)
			}
			var named, aliased []domain.Node
			for _, key := range targets[i].LastKeys() {
				named = append(named, byKey[key]...)
				for _, id := range aliasedBy[key] {
					n, ok := byID[id]
					if !ok {
						return nil, fmt.Errorf("linking: the page %s of an alias is not a page of %s", id, notebookID)
					}
					if !slices.ContainsFunc(aliased, func(a domain.Node) bool { return a.ID == id }) {
						aliased = append(aliased, n)
					}
				}
			}
			r = domain.Resolve(targets[i], from.Path, named, aliased)
		}
		if r != l.Resolution {
			l.Resolution = r
			changed = append(changed, l)
		}
	}
	return changed, nil
}
