package app

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// BatchLinks resolves and rewrites links for a batch's plan, on the tree
// the plan simulates (M9 design 4.1, 4.11; M9/P3 design 3.7.4): by the
// deferred participant's rules for a page the batch wrote (ownLinks,
// writtenTree, dropDead, domain.Rewrite and Written), one rule written
// once, over the index's rows and aliases as the batch began, read on the
// pool, and the tree its caller gives. It writes nothing.
type BatchLinks struct {
	Store  Store
	Logger *slog.Logger
	// MaxContent is the most bytes a page's content holds: the page module's.
	MaxContent int
}

// LinksOn is a content of a page of a batch's plan to resolve its links
// on: the notebook and the page; the plan's tree at the moment, as Pages
// reads it, and the nodes the batch deleted by then, whose aliases are
// none; and the plan's parse, which admits a content within the plan's
// bounds and keeps its facts by page and hash.
type LinksOn struct {
	NotebookID uuid.UUID
	PageID     uuid.UUID
	Content    string
	Tree       Pages
	Gone       []uuid.UUID
	Parse      func(ctx context.Context, content string) (domain.Facts, error)
}

// LinksMoved is a content of a page of a batch's plan to rewrite, on the
// tree after the op it is rewritten for (LinksOn): where its links led at
// the moment its content was read (Before), the renames and moves since,
// merged node by node, and the paths their former parents had then.
type LinksMoved struct {
	LinksOn
	Before  LinksBefore
	Changes []domain.Change
	Parents map[uuid.UUID][]domain.Step
}

// Indexed is where the links of the page id of notebookID led by the
// index, its revision revision, with their targets at their paths on tree:
// false when the index is not of revision or of the current extractor,
// whose links the plan does not rewrite (M6's skips).
func (b BatchLinks) Indexed(ctx context.Context, notebookID, id uuid.UUID, revision int, tree Pages) (LinksBefore, bool, error) {
	x, ok, err := b.Store.View(ctx, id)
	if err != nil || !ok || x.Revision != revision || x.Extractor != domain.Extractor {
		return LinksBefore{}, false, err
	}
	before, err := withTargets(ctx, tree, notebookID, x.Resolutions)
	return before, err == nil, err
}

// Resolve is where each link of q's content leads on q's tree, by where
// its target starts, with the targets at their paths there: the before of
// the page's next rewrite, and of the deferred one.
func (b BatchLinks) Resolve(ctx context.Context, q LinksOn) (LinksBefore, error) {
	facts, err := q.Parse(ctx, q.Content)
	if err != nil {
		return LinksBefore{}, err
	}
	rows := make([]Link, len(facts.Links))
	for i, l := range facts.Links {
		rows[i] = Link{SourceID: q.PageID, Start: l.Start, Target: l.Target, Aliases: l.Aliases}
	}
	rs, missing, err := resolutions(ctx, b.Store, q.Tree, q.NotebookID, rows, nil, setOf(q.Gone))
	if err != nil {
		return LinksBefore{}, err
	}
	if len(missing) > 0 {
		return LinksBefore{}, fmt.Errorf("linking: the pages %v of links or aliases are not pages of %s", missing, q.NotebookID)
	}
	leads := make(map[int]domain.Resolution, len(rows))
	for i, l := range rows {
		leads[l.Start] = rs[i]
	}
	return withTargets(ctx, q.Tree, q.NotebookID, leads)
}

// Rewrite is q's content with the links q's changes reach written again
// to lead where they led, q.Before, on q's tree: as the deferred
// participant writes a page the batch wrote. The content as it is when
// none is reached, or no writing reads back as its links.
func (b BatchLinks) Rewrite(ctx context.Context, q LinksMoved) (string, error) {
	if relocates, recased := domain.Relocation(q.Changes); !relocates && len(recased) == 0 {
		return q.Content, nil
	}
	facts, err := q.Parse(ctx, q.Content)
	if err != nil {
		return "", err
	}
	gone := setOf(q.Gone)
	sub := &subtrees{store: b.Store, pages: q.Tree, notebookID: q.NotebookID, read: map[uuid.UUID]subtree{}}
	reached, recased, err := ownLinks(ctx, b.Store, q.Tree, sub, q.NotebookID, q.PageID, facts.Links, q.Before, q.Changes, gone)
	if err != nil || len(reached) == 0 {
		return q.Content, err
	}
	tree, from, err := writtenTree(ctx, q.Tree, q.NotebookID, q.PageID, reached, q.Before)
	if err != nil {
		return "", err
	}
	if dropDead(ctx, b.Logger, q.PageID, reached, tree, gone) {
		return q.Content, nil
	}
	var links []domain.Resolved
	for _, l := range facts.Links {
		if x, ok := reached[l.Start]; ok {
			links = append(links, domain.Resolved{Link: l, Before: x.before, After: x.after})
		}
	}
	rewriting := domain.Rewrite(q.Content, from, links, tree, recased)
	if len(rewriting.Edits) == 0 {
		return q.Content, nil
	}
	was := domain.Facts{Links: facts.Links, Aliases: facts.Aliases}
	written, _, kept, err := rewriting.Written(q.Content, was, from, tree, func(writing string) (domain.Facts, error) {
		if len(writing) > b.MaxContent {
			return domain.Facts{}, domain.ErrTooLarge
		}
		return q.Parse(ctx, writing)
	})
	if err != nil || !kept {
		return q.Content, err
	}
	return written, nil
}

// withTargets is leads with the nodes they lead to at their paths on tree.
func withTargets(ctx context.Context, tree Pages, notebookID uuid.UUID, leads map[int]domain.Resolution) (LinksBefore, error) {
	var ids []uuid.UUID
	for _, r := range leads {
		if r.ID != (uuid.UUID{}) {
			ids = append(ids, r.ID)
		}
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	nodes, err := tree.Paths(ctx, notebookID, slices.Compact(ids))
	if err != nil {
		return LinksBefore{}, err
	}
	out := LinksBefore{Leads: leads, Targets: make(map[uuid.UUID][]domain.Step, len(nodes))}
	for _, n := range nodes {
		out.Targets[n.ID] = n.Path
	}
	return out, nil
}

// setOf is ids as a set.
func setOf(ids []uuid.UUID) map[uuid.UUID]bool {
	out := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}
