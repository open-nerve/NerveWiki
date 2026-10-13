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
// A page in the trash, which the index does not have, resolves them all
// anew from its path through its ancestors, deleted or not, against the
// pages alive: what it would lead to were it where it was (M8/P2 design
// 3.7); none when the page is gone meanwhile.
func (v Views) Resolve(ctx context.Context, p Page, links []Link) (map[int]domain.Resolution, error) {
	if len(links) == 0 {
		return nil, nil
	}
	if p.Deleted {
		return v.resolveDeleted(ctx, p, links)
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
	return v.resolved(ctx, p, anew, nil, out)
}

// resolveDeleted resolves the links of the page in the trash p anew, from
// its path.
func (v Views) resolveDeleted(ctx context.Context, p Page, links []Link) (map[int]domain.Resolution, error) {
	source, ok, err := v.Pages.SourcePath(ctx, p.NotebookID, p.ID)
	if err != nil {
		return nil, err
	}
	var known []domain.Node
	if ok {
		known = []domain.Node{source}
	}
	return v.resolved(ctx, p, links, known, make(map[int]domain.Resolution, len(links)))
}

// resolved adds to out where links of p resolve anew, their sources'
// paths known, by where each starts.
func (v Views) resolved(ctx context.Context, p Page, links []Link, known []domain.Node, out map[int]domain.Resolution) (
	map[int]domain.Resolution, error,
) {
	rs, _, err := resolutions(ctx, v.Store, v.Pages, p.NotebookID, links, known)
	if err != nil {
		return nil, err
	}
	for i, l := range links {
		out[l.Start] = rs[i]
	}
	return out, nil
}
