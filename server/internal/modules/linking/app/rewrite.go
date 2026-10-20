package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Rewrite writes again the links a rename or a move would lead elsewhere,
// so that each leads where it did (M6/P4 design 4.1): the page module's
// participant, in the unit's transaction, which holds its notebook's row
// FOR NO KEY UPDATE. It takes no lock of the index: the row keeps out
// every other unit of the notebook, which the index's writers all are.
type Rewrite struct {
	Store    Store
	Pages    Pages
	Contents PageContents
	Locks    Locks
	Parser   RewriteParser
	Logger   *slog.Logger
	// MaxContent is the most bytes a page's content holds: the page module's.
	MaxContent int
}

// rewritten is a link of the index to write again, where it resolved
// before the operation and where it resolves after.
type rewritten struct {
	before, after domain.Resolution
}

// Participate follows m, an operation of a unit that rewrites links: a
// rename or a move of nodes that relocates one, or a rename of a title's
// case alone. It finds the links of the index the operation reaches that it
// writes again (domain.Rewrites tells each), on the pages the index has with
// the current extractor; and of a batch's (M9/P3 design 3.7.2), the links of
// the pages it wrote that their own changes meet, which it follows from
// their contents. It counts the pages it would write against m.MaxPages
// before it reads the tree they are written on, refuses or parses any
// (ErrTooManyRewrites; M9/P3 review B-I1), a page the batch wrote with such
// a link as m.WrittenWeight (review B-M3), one with none not at all (review
// B1-M2), and their links against m.MaxLinks (review B1-M3: the unit's time
// follows the links); refuses with linking.pages_locked when one of these
// pages is being edited, the caller's own too; then writes each again, one
// page after another, the batch's first, its contents parsed with the budget
// taken now (server_busy). A page whose content was written since its index,
// a link whose page led to is gone, and a link no writing leads back, are
// logged and left.
func (r Rewrite) Participate(ctx context.Context, m Moved, u Appender) error {
	if !m.UpdateLinks {
		return nil
	}
	gone := setOf(m.Gone)
	index, err := r.followed(ctx, m, gone)
	if err != nil {
		return err
	}
	met, err := r.ownMet(ctx, m, gone)
	if err != nil {
		return err
	}
	written := met.counted(gone)
	weight := max(m.WrittenWeight, 1)
	if counted := index.counted(gone) + weight*len(written); m.MaxPages > 0 && counted > m.MaxPages {
		tooMany := &TooManyRewrites{Pages: counted, Max: m.MaxPages}
		if len(written) > 0 && weight > 1 {
			tooMany.Weight = weight
		}
		return tooMany
	}
	if linked := index.linked(gone) + met.linked(gone); m.MaxLinks > 0 && linked > m.MaxLinks {
		return &TooManyRewrites{Links: linked, Max: m.MaxLinks}
	}
	if err := r.readTree(ctx, m, &index, gone); err != nil {
		return err
	}
	all := slices.Concat(index.pages, written)
	if len(all)+len(met.pages) == 0 {
		return nil
	}
	if err := r.refuseLocked(ctx, all, m); err != nil {
		return err
	}
	// The pages the batch wrote first: their read backs, of contents up to
	// a page's largest, are taken while the budget holds none of the
	// shares the facts of the writes before keep to the unit's end.
	own, err := r.ownWritten(ctx, m, met, gone)
	if err != nil {
		return err
	}
	trees, err := readWritten(ctx, r.Pages, m.NotebookID, own)
	if err != nil {
		return err
	}
	for _, id := range met.pages {
		err := r.followWritten(ctx, id, m.Written[id], own[id], trees, gone, u)
		if errors.Is(err, ErrGuardLocked) {
			return r.guardLocked(ctx, all, m, err)
		}
		if err != nil {
			return err
		}
	}
	for _, id := range index.pages {
		err := r.rewrite(ctx, id, index.indexed[id].Revision, nil, index.reached[id], index.from[id], index.tree, index.recased, u)
		if errors.Is(err, ErrGuardLocked) {
			return r.guardLocked(ctx, all, m, err)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// followedPages is what a rewrite follows of the index: the pages to
// write, those of reached with a current index, by id, with their indexed
// revisions, their links to write, by where their targets start, the
// pages those led to, and the pages whose titles' case alone the
// operation changed; once readTree read it, the tree they are written on
// and their paths after the operation.
type followedPages struct {
	pages   []uuid.UUID
	indexed map[uuid.UUID]Indexed
	reached map[uuid.UUID]map[int]rewritten
	targets []uuid.UUID
	recased domain.Recased
	from    map[uuid.UUID][]domain.Step
	tree    domain.Tree
}

// counted is how many of f's pages hold a link whose target a batch did
// not delete (gone): the pages the rewrite writes, as the bound counts
// them before the tree is read; one whose every link led to a page
// another deleted since the plan is counted, and left by readTree.
func (f followedPages) counted(gone map[uuid.UUID]bool) int {
	n := 0
	for _, id := range f.pages {
		for _, x := range f.reached[id] {
			if !gone[x.before.ID] {
				n++
				break
			}
		}
	}
	return n
}

// linked is how many links of f's pages lead to a node a batch did not
// delete (gone): the links the rewrite writes, as MaxLinks counts them
// before the tree is read (M9/P3 review B1-M3).
func (f followedPages) linked(gone map[uuid.UUID]bool) int {
	n := 0
	for _, id := range f.pages {
		for _, x := range f.reached[id] {
			if !gone[x.before.ID] {
				n++
			}
		}
	}
	return n
}

// readTree reads the tree f's pages are written on (treeOf), and drops
// the links whose target before is gone (dropDead), and the pages left
// with none.
func (r Rewrite) readTree(ctx context.Context, m Moved, f *followedPages, gone map[uuid.UUID]bool) error {
	if len(f.pages) == 0 {
		return nil
	}
	tree, from, err := treeOf(ctx, r.Pages, m, f.targets, f.pages)
	if err != nil {
		return err
	}
	f.pages = slices.DeleteFunc(f.pages, func(id uuid.UUID) bool { return dropDead(ctx, r.Logger, id, f.reached[id], tree, gone) })
	f.tree, f.from = tree, from
	return nil
}

// followed is the pages of the index m's changes reach whose links it
// writes again, before the tree they are written on is read (readTree):
// none when they relocate and re-case nothing. The rows of the pages a
// batch deleted (gone) or wrote are not followed from the index: the first
// are gone, the second followed from their contents.
func (r Rewrite) followed(ctx context.Context, m Moved, gone map[uuid.UUID]bool) (followedPages, error) {
	relocates, recased := domain.Relocation(m.Changes)
	if !relocates && len(recased) == 0 {
		return followedPages{}, nil
	}
	reached, targets, err := r.reached(ctx, m, relocates, recased, gone)
	if err != nil || len(reached) == 0 {
		return followedPages{}, err
	}
	ids := slices.SortedFunc(maps.Keys(reached), uuid.UUID.Compare)
	indexed, err := r.Store.IndexedOf(ctx, ids)
	if err != nil {
		return followedPages{}, err
	}
	pages := slices.DeleteFunc(ids, func(id uuid.UUID) bool {
		if x, ok := indexed[id]; ok && x.Extractor == domain.Extractor {
			return false
		}
		r.Logger.LogAttrs(ctx, slog.LevelWarn, "the links of a page are not rewritten: the index of it is not current",
			slog.String("page_id", id.String()), slog.String("notebook_id", m.NotebookID.String()))
		return true
	})
	if len(pages) == 0 {
		return followedPages{}, nil
	}
	return followedPages{pages: pages, indexed: indexed, reached: reached, targets: targets, recased: recased}, nil
}

// reached is the links of the index m reaches that it writes again, as
// domain.Rewrites tells from the index's rows, by their page and where
// their target starts; with the pages they resolved to. recased is the
// pages whose titles' case alone m changed; the rows of the pages gone,
// or written by a batch, are not among them, nor those that led to a node
// a batch froze (Moved.Frozen), and the aliases of those gone are none.
// The nodes a batch created after a node's rename are not spread under it
// (Moved.CreatedAfter); those a move carried are among its changes as the
// move left them.
func (r Rewrite) reached(ctx context.Context, m Moved, relocates bool, recased domain.Recased, gone map[uuid.UUID]bool) (
	map[uuid.UUID]map[int]rewritten, []uuid.UUID, error,
) {
	reach := domain.Reach{Targets: slices.SortedFunc(maps.Keys(recased), uuid.UUID.Compare)}
	if relocates {
		var renamed []uuid.UUID
		reach, renamed, _ = domain.Affected(m.Changes)
		var err error
		if reach, err = spread(ctx, r.Store, r.Pages, m.NotebookID, reach, renamed, setsOf(m.CreatedAfter)); err != nil {
			return nil, nil, err
		}
	}
	links, err := r.Store.Links(ctx, m.NotebookID, reach)
	if err != nil {
		return nil, nil, err
	}
	frozen := setOf(m.Frozen)
	links = slices.DeleteFunc(links, func(l Link) bool {
		_, written := m.Written[l.SourceID]
		return written || gone[l.SourceID] || frozen[l.Resolution.ID]
	})
	after, missing, err := resolutions(ctx, r.Store, r.Pages, m.NotebookID, links, nil, gone)
	if err != nil {
		return nil, nil, err
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("linking: the pages %v of links or aliases are not pages of %s", missing, m.NotebookID)
	}
	out := make(map[uuid.UUID]map[int]rewritten)
	var targets []uuid.UUID
	for i, l := range links {
		before := l.Resolution
		if !domain.Rewrites(domain.Link{Target: l.Target, Aliases: l.Aliases}, before, after[i], recased) {
			continue
		}
		if out[l.SourceID] == nil {
			out[l.SourceID] = make(map[int]rewritten)
		}
		out[l.SourceID][l.Start] = rewritten{before: before, after: after[i]}
		targets = append(targets, before.ID)
	}
	return out, targets, nil
}

// ownReach is the links of a page a batch wrote that its own changes
// reach and write again (ownLinks), with the pages they re-cased.
type ownReach struct {
	reached map[int]rewritten
	recased domain.Recased
}

// writtenMet is, of the pages a batch wrote, alive after it, those with a
// link their own changes meet, in id order, each with those links
// (metLinks): what the bounds count of them before the tree is read
// (M9/P3 review B1-M2), and the rewrite resolves after it.
type writtenMet struct {
	pages []uuid.UUID
	met   map[uuid.UUID]metRows
}

// metRows is the links of a page that changes meet (metLinks), with where
// each led, and the pages the changes re-cased.
type metRows struct {
	rows    []Link
	befores []domain.Resolution
	recased domain.Recased
}

// counted is the pages of w with a link whose target a batch did not
// delete (gone): those the rewrite writes, as the bound counts them, as
// followedPages.counted counts the index's; one whose every link led to a
// page the batch deleted is followed, its links left by dropDead.
func (w writtenMet) counted(gone map[uuid.UUID]bool) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range w.pages {
		if slices.ContainsFunc(w.met[id].befores, func(b domain.Resolution) bool { return !gone[b.ID] }) {
			out = append(out, id)
		}
	}
	return out
}

// linked is how many links of w's pages lead to a node a batch did not
// delete (gone): the links the rewrite writes of them, as MaxLinks counts
// them before they are resolved, more than domain.Rewrites may tell.
func (w writtenMet) linked(gone map[uuid.UUID]bool) int {
	n := 0
	for _, id := range w.pages {
		for _, b := range w.met[id].befores {
			if !gone[b.ID] {
				n++
			}
		}
	}
	return n
}

// ownMet is writtenMet of m's pages, the subtrees spread reads read once
// for the batch, each reach made once for the same changes (subtrees).
func (r Rewrite) ownMet(ctx context.Context, m Moved, gone map[uuid.UUID]bool) (writtenMet, error) {
	sub := &subtrees{store: r.Store, pages: r.Pages, notebookID: m.NotebookID, read: map[uuid.UUID]subtree{}}
	out := writtenMet{met: map[uuid.UUID]metRows{}}
	for _, id := range slices.SortedFunc(maps.Keys(m.Written), uuid.UUID.Compare) {
		if gone[id] {
			continue
		}
		w := m.Written[id]
		met, err := metLinks(ctx, sub, id, w.Facts.Links, w.Before, w.Changes, setOf(w.Frozen), setsOf(w.CreatedAfter))
		if err != nil {
			return writtenMet{}, err
		}
		if len(met.rows) > 0 {
			out.pages = append(out.pages, id)
			out.met[id] = met
		}
	}
	return out, nil
}

// ownWritten is, of the pages of met, the links each one's own changes
// reach and write again, resolved after (rewrittenOf); the pages with none
// are not in it.
func (r Rewrite) ownWritten(ctx context.Context, m Moved, met writtenMet, gone map[uuid.UUID]bool) (map[uuid.UUID]ownReach, error) {
	out := make(map[uuid.UUID]ownReach, len(met.pages))
	for _, id := range met.pages {
		x := met.met[id]
		reached, err := rewrittenOf(ctx, r.Store, r.Pages, m.NotebookID, x, gone)
		if err != nil {
			return nil, err
		}
		if len(reached) > 0 {
			out[id] = ownReach{reached: reached, recased: x.recased}
		}
	}
	return out, nil
}

// followWritten writes again the links of the page id, which a batch
// wrote (M9/P3 design 3.7.2): those its own changes, w.Changes, reach and
// move (own), from its final content, the unit's, its links taken from
// w.Facts unparsed (a); whatever its index's revision (b), on its current
// one (c); each matched by where its target starts to where it led at the
// page's last content op, its target's path then its was (d), on the tree
// read once for all the pages the batch wrote (trees).
func (r Rewrite) followWritten(ctx context.Context, id uuid.UUID, w WrittenPage, own ownReach, trees writtenTrees,
	gone map[uuid.UUID]bool, u Appender,
) error {
	if len(own.reached) == 0 {
		return nil
	}
	tree, from := trees.of(id, own.reached, w.Before)
	if dropDead(ctx, r.Logger, id, own.reached, tree, gone) {
		return nil
	}
	return r.rewrite(ctx, id, 0, &w.Facts, own.reached, from, tree, own.recased, u)
}

// ownLinks is the links, of the page id's links, that changes, a page a
// batch wrote's own, reach and write again (M9/P3 design 3.7.2 (a), (e)):
// those they meet (metLinks), resolved after on pages, the aliases of gone
// none, that domain.Rewrites tells (rewrittenOf); with the pages changes
// re-cased.
func ownLinks(ctx context.Context, store Store, pages Pages, sub *subtrees, notebookID, id uuid.UUID, links []domain.Link,
	before LinksBefore, changes []domain.Change, frozen, gone map[uuid.UUID]bool, unspread map[uuid.UUID]map[uuid.UUID]bool,
) (map[int]rewritten, domain.Recased, error) {
	met, err := metLinks(ctx, sub, id, links, before, changes, frozen, unspread)
	if err != nil || len(met.rows) == 0 {
		return nil, met.recased, err
	}
	out, err := rewrittenOf(ctx, store, pages, notebookID, met, gone)
	return out, met.recased, err
}

// metLinks is the links, of the page id's links, that changes meet,
// matched by where their targets start to before's leads: reached as the
// index's rows are, by Affected and spread, which reads the subtree of
// each node changes rename or move, once for the batch (sub), but those
// that led to a node of frozen, which are left (Moved.Frozen), the nodes
// of unspread out of the subtrees of the nodes they key
// (Moved.CreatedAfter); and those that led to a node, not by an alias's
// value, the rest of which domain.Rewrites writes none of. A page of no
// links meets none, and spreads nothing for it (M9/P3 review A's N1).
func metLinks(ctx context.Context, sub *subtrees, id uuid.UUID, links []domain.Link, before LinksBefore,
	changes []domain.Change, frozen map[uuid.UUID]bool, unspread map[uuid.UUID]map[uuid.UUID]bool,
) (metRows, error) {
	if len(links) == 0 {
		return metRows{}, nil
	}
	relocates, recased := domain.Relocation(changes)
	if !relocates && len(recased) == 0 {
		return metRows{}, nil
	}
	reach := domain.Reach{Targets: slices.SortedFunc(maps.Keys(recased), uuid.UUID.Compare)}
	if relocates {
		var err error
		if reach, err = sub.reach(ctx, changes, unspread); err != nil {
			return metRows{}, err
		}
	}
	out := metRows{recased: recased}
	for _, l := range links {
		if lead, ok := before.Leads[l.Start]; ok && lead.ID != (uuid.UUID{}) && !l.Aliases && !frozen[lead.ID] &&
			reachedBy(reach, id, l, lead) {
			out.rows = append(out.rows, Link{SourceID: id, Start: l.Start, Target: l.Target, Aliases: l.Aliases})
			out.befores = append(out.befores, lead)
		}
	}
	return out, nil
}

// rewrittenOf is, of met's links, those domain.Rewrites tells, resolved
// after on pages, the aliases of gone none, by where their targets start.
func rewrittenOf(ctx context.Context, store Store, pages Pages, notebookID uuid.UUID, met metRows, gone map[uuid.UUID]bool) (
	map[int]rewritten, error,
) {
	after, missing, err := resolutions(ctx, store, pages, notebookID, met.rows, nil, gone)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("linking: the pages %v of links or aliases are not pages of %s", missing, notebookID)
	}
	out := map[int]rewritten{}
	for i, l := range met.rows {
		if domain.Rewrites(domain.Link{Target: l.Target, Aliases: l.Aliases}, met.befores[i], after[i], met.recased) {
			out[l.Start] = rewritten{before: met.befores[i], after: after[i]}
		}
	}
	return out, nil
}

// reachedBy reports whether reach, compacted (sorted), meets l, a link of
// the page id that led to before, as Store.Links tells of an index's row:
// by its target's keys, by the node it led to, or by its page; each looked
// up in its sorted keys or ids, not walked (a folder's subtree spread may
// hold a hundred thousand).
func reachedBy(reach domain.Reach, id uuid.UUID, l domain.Link, before domain.Resolution) bool {
	_, byTarget := slices.BinarySearchFunc(reach.Targets, before.ID, uuid.UUID.Compare)
	_, bySource := slices.BinarySearchFunc(reach.Sources, id, uuid.UUID.Compare)
	return byTarget || bySource || slices.ContainsFunc(l.Keys(), func(k string) bool {
		_, ok := slices.BinarySearch(reach.Keys, k)
		return ok
	})
}

// stepsNot is steps, a node's subtree as spread reads it after a batch's
// renames and moves, but the nodes of after, which the batch created after
// the node's last rename or move (Moved.CreatedAfter): the subtree as that
// op left it, of the nodes it holds still. Its rewrite reached none of
// them, and their creation only resolves links again (M9 design 4.11), so
// their keys, the shorter paths of which may take links that led
// elsewhere, would have those written back (M9/P3 review B1-M1). steps
// itself for none.
func stepsNot(steps []domain.Step, after map[uuid.UUID]bool) []domain.Step {
	if len(after) == 0 {
		return steps
	}
	return slices.DeleteFunc(slices.Clone(steps), func(s domain.Step) bool { return after[s.ID] })
}

// movedOf is the nodes changes move under another parent.
func movedOf(changes []domain.Change) []uuid.UUID {
	var out []uuid.UUID
	for _, c := range changes {
		if c.Before != nil && c.After != nil && !sameParent(c.Before.ParentID, c.After.ParentID) {
			out = append(out, c.NodeID)
		}
	}
	return out
}

// sameParent reports whether a and b are one parent, the root for nil.
func sameParent(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// subtrees is the subtrees spread reads for the pages a batch wrote, each
// node's once for the batch, with the keys of its nodes' aliases (M9/P3
// design 3.7.2 (e)): a moved folder's nodes held once, not once a page;
// and the reaches made of them, each once for the changes it was made of
// and the nodes left out (M9/P3 review A's N1): the content ops after a
// rename of a folder, on one tree, spread its subtree once, not once each.
type subtrees struct {
	store      Store
	pages      Pages
	notebookID uuid.UUID
	read       map[uuid.UUID]subtree
	reaches    map[string]domain.Reach
}

// subtree is a node's subtree as spread reads it, and its aliases' keys.
type subtree struct {
	steps []domain.Step
	keys  []string
}

// reach is what changes, relocating, reach on the tree s reads, as ownLinks
// follows them: Affected's, with the subtrees of the nodes they rename or
// move spread, the nodes of unspread out of them, compacted; made once for
// the same changes and unspread on the tree, then shared, read alone.
func (s *subtrees) reach(ctx context.Context, changes []domain.Change, unspread map[uuid.UUID]map[uuid.UUID]bool) (
	domain.Reach, error,
) {
	key := reachKey(changes, unspread)
	if r, ok := s.reaches[key]; ok {
		return r, nil
	}
	reach, renamed, _ := domain.Affected(changes)
	reach, err := s.spread(ctx, reach, append(renamed, movedOf(changes)...), unspread)
	if err != nil {
		return domain.Reach{}, err
	}
	if s.reaches == nil {
		s.reaches = map[string]domain.Reach{}
	}
	s.reaches[key] = reach
	return reach, nil
}

// reachKey is changes, and the nodes unspread leaves out under each of
// theirs, written out whole, each name quoted: the key of their reach on a
// tree.
func reachKey(changes []domain.Change, unspread map[uuid.UUID]map[uuid.UUID]bool) string {
	var b strings.Builder
	place := func(p *domain.Place) {
		switch {
		case p == nil:
			b.WriteString(" -")
		case p.ParentID == nil:
			fmt.Fprintf(&b, " root %q", p.Name)
		default:
			fmt.Fprintf(&b, " %s %q", p.ParentID, p.Name)
		}
	}
	for _, c := range changes {
		b.WriteString(c.NodeID.String())
		place(c.Before)
		place(c.After)
		for _, id := range slices.SortedFunc(maps.Keys(unspread[c.NodeID]), uuid.UUID.Compare) {
			b.WriteString(" " + id.String())
		}
		b.WriteByte(';')
	}
	return b.String()
}

// spread is the package's spread for a page a batch wrote: reach with the
// subtrees of nodes, each read once for the batch, as their last rename or
// move left them (stepsNot), and the aliases' keys of reach's own targets
// and of the subtrees' nodes, which a node the batch created has none of.
func (s *subtrees) spread(ctx context.Context, reach domain.Reach, nodes []uuid.UUID, after map[uuid.UUID]map[uuid.UUID]bool) (
	domain.Reach, error,
) {
	own, err := s.store.AliasKeys(ctx, reach.Targets)
	if err != nil {
		return domain.Reach{}, err
	}
	for _, id := range nodes {
		sub, ok := s.read[id]
		if !ok {
			steps, err := s.pages.Subtree(ctx, s.notebookID, id)
			if err != nil {
				return domain.Reach{}, err
			}
			ids := make([]uuid.UUID, len(steps))
			for i, st := range steps {
				ids[i] = st.ID
			}
			keys, err := s.store.AliasKeys(ctx, ids)
			if err != nil {
				return domain.Reach{}, err
			}
			sub = subtree{steps: steps, keys: keys}
			s.read[id] = sub
		}
		reach.Add(stepsNot(sub.steps, after[id])...)
		reach.Keys = append(reach.Keys, sub.keys...)
	}
	reach.Keys = append(reach.Keys, own...)
	reach.Compact()
	return reach, nil
}

// writtenTree is what the rewrite of a page a batch wrote reads of the
// tree on pages (M9/P3 design 3.7.2 (d)): the targets reached led to,
// after, and before, as its last content op left them, their paths then;
// and its own path.
func writtenTree(ctx context.Context, pages Pages, notebookID, id uuid.UUID, reached map[int]rewritten, before LinksBefore) (
	domain.Tree, []domain.Step, error,
) {
	trees, err := readWritten(ctx, pages, notebookID, map[uuid.UUID]ownReach{id: {reached: reached}})
	if err != nil {
		return domain.Tree{}, nil, err
	}
	tree, from := trees.of(id, reached, before)
	return tree, from, nil
}

// writtenTrees is what the rewrites of the pages a batch wrote read of the
// tree, once for all of them (M9/P3 review B-M3): their reached links'
// targets at their paths after, the pages with the keys a writing of them
// reads, and the pages' own paths.
type writtenTrees struct {
	tree domain.Tree
	from map[uuid.UUID][]domain.Step
}

// readWritten reads the tree the pages of own are written on: one read of
// the paths of their targets and of the pages, one of the keys.
func readWritten(ctx context.Context, pages Pages, notebookID uuid.UUID, own map[uuid.UUID]ownReach) (writtenTrees, error) {
	if len(own) == 0 {
		return writtenTrees{}, nil
	}
	var targets []uuid.UUID
	sources := slices.SortedFunc(maps.Keys(own), uuid.UUID.Compare)
	for _, id := range sources {
		for _, x := range own[id].reached {
			targets = append(targets, x.before.ID)
		}
	}
	tree, from, err := treeOf(ctx, pages, Moved{NotebookID: notebookID}, targets, sources)
	return writtenTrees{tree: tree, from: from}, err
}

// of is the tree the page id is written on, its links reached: the
// targets after and the pages by their keys as read for all, the targets
// before at the paths before gives them, as its last content op left them,
// a target before does not name left out; and its own path.
func (w writtenTrees) of(id uuid.UUID, reached map[int]rewritten, before LinksBefore) (domain.Tree, []domain.Step) {
	tree := domain.Tree{Before: make(map[uuid.UUID]domain.Node, len(reached)), After: w.tree.After, Named: w.tree.Named}
	for _, x := range reached {
		t := x.before.ID
		n, alive := w.tree.After[t]
		if path, ok := before.Targets[t]; ok && alive {
			tree.Before[t] = domain.Node{ID: t, Path: path, Asset: n.Asset}
		}
	}
	return tree, w.from[id]
}

// dropDead drops, of links, the page id's, those whose target before is
// not on tree now (M9/P3 design 3.7.2): deleted by the batch, gone, at
// info; by another since the batch's plan read it, at warn. It tells
// whether none is left.
func dropDead(ctx context.Context, logger *slog.Logger, id uuid.UUID, links map[int]rewritten, tree domain.Tree,
	gone map[uuid.UUID]bool,
) bool {
	for _, start := range slices.Sorted(maps.Keys(links)) {
		target := links[start].before.ID
		if _, alive := tree.After[target]; alive {
			continue
		}
		level := slog.LevelWarn
		if gone[target] {
			level = slog.LevelInfo
		}
		logger.LogAttrs(ctx, level, "a link is not rewritten: where it led is gone",
			slog.String("page_id", id.String()), slog.Int("start", start))
		delete(links, start)
	}
	return len(links) == 0
}

// refuseLocked is linking.pages_locked naming the edit locks of pages,
// alive at m's time, if one is; none of none read.
func (r Rewrite) refuseLocked(ctx context.Context, pages []uuid.UUID, m Moved) error {
	if len(pages) == 0 {
		return nil
	}
	locks, err := r.Locks.Of(ctx, pages, m.At)
	switch {
	case err != nil:
		return err
	case len(locks) > 0:
		return domain.PagesLocked(locks)
	}
	return nil
}

// guardLocked is linking.pages_locked for err, a write the edit lock
// refused: naming the edit locks alive at m's time, read again; or, when
// none is by then, its session ended since, the lock the guard named
// (M6/P4 review r2-4). Another module's code would be the operation's
// undeclared answer.
func (r Rewrite) guardLocked(ctx context.Context, pages []uuid.UUID, m Moved, err error) error {
	if locked := r.refuseLocked(ctx, pages, m); locked != nil {
		return locked
	}
	var refused *shared.Error
	if errors.As(err, &refused) && refused.Lock != nil {
		return domain.PagesLocked([]shared.LockHolder{*refused.Lock})
	}
	return err
}

// treeOf is what a rewrite reads of the pages: those targets are, at
// their paths after m and before it, and the pages with the keys a writing
// of them reads; and the paths after m of sources, the pages written. The
// paths the moved nodes' former parents had are m's Parents, as the unit
// began, or read now, which a unit of one operation left as they were.
// Each target, one for each link reached, is looked at once, on m's
// changes indexed once (M9/P3 review B-I1).
func treeOf(ctx context.Context, pages Pages, m Moved, targets, sources []uuid.UUID) (
	domain.Tree, map[uuid.UUID][]domain.Step, error,
) {
	targets = slices.Clone(targets)
	slices.SortFunc(targets, uuid.UUID.Compare)
	targets = slices.Compact(targets)
	parents := domain.FormerParents(m.Changes)
	ids := slices.Concat(targets, sources)
	if m.Parents == nil {
		ids = append(ids, parents...)
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	nodes, err := pages.Paths(ctx, m.NotebookID, slices.Compact(ids))
	if err != nil {
		return domain.Tree{}, nil, err
	}
	byID := make(map[uuid.UUID]domain.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	before := make(map[uuid.UUID][]domain.Step, len(parents))
	for _, id := range parents {
		if m.Parents != nil {
			before[id] = m.Parents[id]
		} else {
			before[id] = byID[id].Path
		}
	}
	tree := domain.Tree{Before: map[uuid.UUID]domain.Node{}, After: map[uuid.UUID]domain.Node{}, Named: map[string][]domain.Node{}}
	var keys []string
	befores := domain.BeforesOf(m.Changes)
	for _, id := range targets {
		if n, ok := byID[id]; ok {
			tree.After[id], tree.Before[id] = n, befores.PathBefore(n, before)
			keys = append(keys, domain.WrittenKeys(n)...)
		}
	}
	slices.Sort(keys)
	named, err := pages.ByKeys(ctx, m.NotebookID, slices.Compact(keys))
	if err != nil {
		return domain.Tree{}, nil, err
	}
	for _, n := range named {
		key := n.Path[len(n.Path)-1].Key
		tree.Named[key] = append(tree.Named[key], n)
	}
	from := make(map[uuid.UUID][]domain.Step, len(sources))
	for _, id := range sources {
		from[id] = byID[id].Path
	}
	return tree, from, nil
}

// rewrite writes the page id's links of reached again, through u: its
// content parsed, its links matched to the index's by where their target
// starts, written by domain.Rewrite and read back (Written), and appended
// on revision, the index's, with the facts of the writing kept, whose
// share of the budget the unit's end gives back. A content written since,
// which the unit's lock keeps out, is logged and left; so is a link no
// writing leads back, a writing past MaxContent, and one whose links do
// not read back as the content's. A page a batch wrote has its content's
// facts given, which no parse takes again, and is written on its current
// revision, newer than its index's (M9/P3 design 3.7.2 (a)-(c)).
func (r Rewrite) rewrite(ctx context.Context, id uuid.UUID, revision int, given *domain.Facts, reached map[int]rewritten,
	from []domain.Step, tree domain.Tree, recased domain.Recased, u Appender,
) error {
	content, current, ok, err := r.Contents.Content(ctx, id)
	switch {
	case err != nil:
		return err
	case !ok:
		// The unit's lock of the notebook keeps its pages.
		return fmt.Errorf("the content of page %s, which the rewrite reads, is gone", id)
	}
	var facts domain.Facts // what a writing is read back against
	if given != nil {
		facts, revision = domain.Facts{Links: given.Links, Aliases: given.Aliases}, current
	} else {
		if current != revision {
			r.Logger.LogAttrs(ctx, slog.LevelWarn, "the links of a page are not rewritten: its content is not the index's",
				slog.String("page_id", id.String()), slog.Int("revision", current), slog.Int("indexed", revision))
			return nil
		}
		was, err := r.Parser.ParseNow(ctx, content)
		if err != nil {
			return err
		}
		defer was.release() // given back below; again, on a panic before, nothing held stays (M6 closeout A-M4)
		// The content's share goes back before a writing's parse takes its
		// own, its links and aliases kept for Written uncounted until then,
		// within some tenth of what that parse holds
		// (markdown.Hold.KeepFacts); the rest of its facts go. Held, the two
		// would ask more than the smallest budget, a page's largest content,
		// for a page at its largest, which no wait would free (M6/P4 fix
		// check c1, c5-6).
		was.Release()
		facts = domain.Facts{Links: was.Facts.Links, Aliases: was.Facts.Aliases}
	}
	var links []domain.Resolved
	for _, l := range facts.Links {
		if x, ok := reached[l.Start]; ok {
			links = append(links, domain.Resolved{Link: l, Before: x.before, After: x.after})
		}
	}
	rewriting := domain.Rewrite(content, from, links, tree, recased)
	for _, l := range rewriting.Left {
		r.left(ctx, "no writing leads where it led", id, l)
	}
	if len(rewriting.Edits) == 0 {
		return nil
	}
	var now Parsed   // the last writing's parse
	var large int    // the bytes of a writing past MaxContent, if one was
	read := false    // whether a writing was parsed, to read back
	written := false // whether now is the unit's, given back as it ends
	defer func() {
		if !written {
			now.release() // a writing not written, or a panic, holds none of the budget
		}
	}()
	writing, left, kept, err := rewriting.Written(content, facts, from, tree, func(writing string) (domain.Facts, error) {
		now.release()
		now = Parsed{}
		if len(writing) > r.MaxContent {
			large = len(writing)
			return domain.Facts{}, domain.ErrTooLarge
		}
		var err error
		now, err = r.Parser.ParseNow(ctx, writing)
		read = true
		return now.Facts, err
	})
	if err != nil {
		return err
	}
	if !kept {
		level, msg := slog.LevelError, "the links of a page are not rewritten: no writing reads back as its links and aliases"
		if !read {
			level, msg = slog.LevelWarn, "the links of a page are not rewritten: it would hold more than a page may"
		}
		attrs := []slog.Attr{slog.String("page_id", id.String())}
		if large > 0 {
			attrs = append(attrs, slog.Int("bytes", len(content)), slog.Int("written", large))
		}
		r.Logger.LogAttrs(ctx, level, msg, attrs...)
		return nil
	}
	// The links a writing of the body alone left: with the sizes when a
	// writing with them was too large; whether the others changed more than
	// the links, or failed for the body's edits, the writings do not tell.
	// An error, too large alone or not, where a page left whole for its size
	// is a warning: the page is written, its links lead elsewhere unseen
	// (M6/P4 fix check c10-1).
	var sizes []slog.Attr
	if left.TooLarge > 0 {
		sizes = []slog.Attr{slog.Int("bytes", len(content)), slog.Int("written", left.TooLarge)}
	}
	for _, l := range left.Links {
		r.left(ctx, "no writing of the page with it was kept", id, l, sizes...)
	}
	u.Defer(now.Release)
	written = true
	return u.WriteContent(ctx, Rewritten{PageID: id, Base: revision, Content: writing, Facts: now.Written})
}

// left logs l, a link of the page id not rewritten, why, and attrs: by
// where it starts, as its target holds a title, which no log holds (v0.1
// design 13.1 rule 10; Codex review R2).
func (r Rewrite) left(ctx context.Context, why string, id uuid.UUID, l domain.Link, attrs ...slog.Attr) {
	r.Logger.LogAttrs(ctx, slog.LevelError, "a link is not rewritten: "+why,
		append([]slog.Attr{slog.String("page_id", id.String()), slog.Int("start", l.Start)}, attrs...)...)
}

// release gives back what p holds of the budget, if it holds any.
func (p Parsed) release() {
	if p.Release != nil {
		p.Release()
	}
}
