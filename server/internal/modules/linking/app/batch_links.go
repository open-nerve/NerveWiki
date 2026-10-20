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
// merged node by node, the nodes those ops rename or move with
// update_links false, whose links it leaves (Moved.Frozen), and the nodes
// they create after a node's last rename or move, out of its subtree
// (Moved.CreatedAfter). The paths before are Before's:
// the moved nodes' former parents are not read. Reads, when set, keeps
// the subtrees read of the tree for the caller's next calls on it (M9/P3
// review B-N2).
type LinksMoved struct {
	LinksOn
	Before       LinksBefore
	Changes      []domain.Change
	Frozen       []uuid.UUID
	CreatedAfter map[uuid.UUID][]uuid.UUID
	Reads        *any
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

// Reached reports whether changes, the renames and moves of a batch's
// ops before a page's first content op, reach a link of the page id by the
// index at revision, on tree after them, the nodes gone by then none, the
// links that led to the nodes of frozen left, the nodes of createdAfter
// out of the subtrees of the nodes they key (Moved.CreatedAfter): by the
// rule the plan's Rewrite and the deferred participant follow them
// (ownLinks), on the index's rows of the page (M9/P3 review A-I1). The plan then reads the page whole, to
// compare a write with its content rewritten for them. False when the
// index is not of revision or of the current extractor, whose links the
// plan does not rewrite, as Indexed. The subtrees it reads of tree are
// kept in reads, when set, as Rewrite's.
func (b BatchLinks) Reached(ctx context.Context, notebookID, id uuid.UUID, revision int, changes []domain.Change,
	frozen []uuid.UUID, createdAfter map[uuid.UUID][]uuid.UUID, tree Pages, gone []uuid.UUID, reads *any,
) (bool, error) {
	if relocates, recased := domain.Relocation(changes); !relocates && len(recased) == 0 {
		return false, nil
	}
	x, ok, err := b.Store.View(ctx, id)
	if err != nil || !ok || x.Revision != revision || x.Extractor != domain.Extractor {
		return false, err
	}
	rows, err := b.Store.Links(ctx, notebookID, domain.Reach{Sources: []uuid.UUID{id}})
	if err != nil {
		return false, err
	}
	links := make([]domain.Link, 0, len(rows))
	for _, l := range rows {
		if l.SourceID == id {
			links = append(links, domain.Link{Start: l.Start, Target: l.Target, Aliases: l.Aliases})
		}
	}
	sub := b.subtreesOf(notebookID, tree, reads)
	reached, _, err := ownLinks(ctx, b.Store, tree, sub, notebookID, id, links, LinksBefore{Leads: x.Resolutions}, changes,
		setOf(frozen), setOf(gone), setsOf(createdAfter))
	return len(reached) > 0, err
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
	sub := b.subtreesOf(q.NotebookID, q.Tree, q.Reads)
	reached, recased, err := ownLinks(ctx, b.Store, q.Tree, sub, q.NotebookID, q.PageID, facts.Links, q.Before, q.Changes,
		setOf(q.Frozen), gone, setsOf(q.CreatedAfter))
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

// subtreesOf is the subtrees spread reads of tree, those reads keeps when
// set: made there by the first call on the tree.
func (b BatchLinks) subtreesOf(notebookID uuid.UUID, tree Pages, reads *any) *subtrees {
	if reads != nil {
		if sub, ok := (*reads).(*subtrees); ok {
			return sub
		}
	}
	sub := &subtrees{store: b.Store, pages: tree, notebookID: notebookID, read: map[uuid.UUID]subtree{}}
	if reads != nil {
		*reads = sub
	}
	return sub
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

// setsOf is ids, by node, as sets.
func setsOf(ids map[uuid.UUID][]uuid.UUID) map[uuid.UUID]map[uuid.UUID]bool {
	out := make(map[uuid.UUID]map[uuid.UUID]bool, len(ids))
	for id, of := range ids {
		out[id] = setOf(of)
	}
	return out
}

// setOf is ids as a set.
func setOf(ids []uuid.UUID) map[uuid.UUID]bool {
	out := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}
