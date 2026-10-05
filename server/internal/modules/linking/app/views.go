package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Views tells a page's reading view where its links lead (M6 design 4.7,
// "阅读视图的一致性"; M6/P3 design 6.5), without a lock or a transaction.
type Views struct {
	Store Store
	Pages Pages
}

// Resolve is where links, those of p's content at p.Revision, resolve, by
// where each starts: the index's when its rows are of that revision and
// of this extractor, and anew for the others, or for every one when they
// are not. Resolving anew reads the candidates, the aliases and the paths
// in statements of their own, outside a transaction: a page moved or
// renamed between them may have a link resolve as before the move, and
// one gone meanwhile resolves none of its links, an alias of it none,
// until the view is read again: with the event of the change, for a page
// the index has, or by a later read (M6/P3 design 6.1, P3B review L1, L2).
func (v Views) Resolve(ctx context.Context, p Page, links []Link) (map[int]domain.Resolution, error) {
	if len(links) == 0 {
		return nil, nil
	}
	indexed, ok, err := v.Store.View(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := make(map[int]domain.Resolution, len(links))
	anew := links
	if ok && indexed.Revision == p.Revision && indexed.Extractor == domain.Extractor {
		anew = nil
		for _, l := range links {
			if r, ok := indexed.Resolutions[l.Start]; ok {
				out[l.Start] = r
			} else {
				anew = append(anew, l)
			}
		}
	}
	rs, _, err := resolutions(ctx, v.Store, v.Pages, p.NotebookID, anew)
	if err != nil {
		return nil, err
	}
	for i, l := range anew {
		out[l.Start] = rs[i]
	}
	return out, nil
}
