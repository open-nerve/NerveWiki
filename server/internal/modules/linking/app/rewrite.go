package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
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
// case alone. It finds the links of the index the operation reaches that
// it writes again (domain.Rewrites tells each), on the pages the index has
// with the current extractor; refuses with linking.pages_locked when one
// of these pages is being edited, the caller's own too; then writes each
// again, one page after another by id, its contents parsed with the budget
// taken now (server_busy). A page whose content was written since its
// index, and a link no writing leads back, are logged and left.
func (r Rewrite) Participate(ctx context.Context, m Moved, u Appender) error {
	relocates, recased := domain.Relocation(m.Changes)
	if !m.UpdateLinks || !relocates && recased.ID == (uuid.UUID{}) {
		return nil
	}
	reached, targets, err := r.reached(ctx, m, relocates, recased)
	if err != nil || len(reached) == 0 {
		return err
	}
	ids := slices.SortedFunc(maps.Keys(reached), uuid.UUID.Compare)
	indexed, err := r.Store.IndexedOf(ctx, ids)
	if err != nil {
		return err
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
		return nil
	}
	if err := r.refuseLocked(ctx, pages, m); err != nil {
		return err
	}
	tree, from, err := r.tree(ctx, m, targets, pages)
	if err != nil {
		return err
	}
	for _, id := range pages {
		err := r.rewrite(ctx, id, indexed[id].Revision, reached[id], from[id], tree, recased, u)
		if errors.Is(err, ErrGuardLocked) {
			return r.guardLocked(ctx, pages, m, err)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// reached is the links of the index m reaches that it writes again, as
// domain.Rewrites tells from the index's rows, by their page and where
// their target starts; with the pages they resolved to. recased is the
// page whose title's case alone m changed.
func (r Rewrite) reached(ctx context.Context, m Moved, relocates bool, recased domain.Recased) (
	map[uuid.UUID]map[int]rewritten, []uuid.UUID, error,
) {
	reach := domain.Reach{Targets: []uuid.UUID{recased.ID}}
	if relocates {
		var renamed []uuid.UUID
		reach, renamed, _ = domain.Affected(m.Changes)
		var err error
		if reach, err = spread(ctx, r.Store, r.Pages, m.NotebookID, reach, renamed); err != nil {
			return nil, nil, err
		}
	}
	links, err := r.Store.Links(ctx, m.NotebookID, reach)
	if err != nil {
		return nil, nil, err
	}
	after, missing, err := resolutions(ctx, r.Store, r.Pages, m.NotebookID, links)
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

// refuseLocked is linking.pages_locked naming the edit locks of pages,
// alive at m's time, if one is.
func (r Rewrite) refuseLocked(ctx context.Context, pages []uuid.UUID, m Moved) error {
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

// tree is what the rewrite reads of the pages: those targets are, at their
// paths after m and before it, and the pages with the keys a writing of
// them reads; and the paths after m of sources, the pages written.
func (r Rewrite) tree(ctx context.Context, m Moved, targets, sources []uuid.UUID) (
	domain.Tree, map[uuid.UUID][]domain.Step, error,
) {
	parents := domain.FormerParents(m.Changes)
	ids := slices.Concat(targets, sources, parents)
	slices.SortFunc(ids, uuid.UUID.Compare)
	nodes, err := r.Pages.Paths(ctx, m.NotebookID, slices.Compact(ids))
	if err != nil {
		return domain.Tree{}, nil, err
	}
	byID := make(map[uuid.UUID]domain.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	before := make(map[uuid.UUID][]domain.Step, len(parents))
	for _, id := range parents {
		before[id] = byID[id].Path
	}
	tree := domain.Tree{Before: map[uuid.UUID]domain.Node{}, After: map[uuid.UUID]domain.Node{}, Named: map[string][]domain.Node{}}
	var keys []string
	for _, id := range targets {
		if n, ok := byID[id]; ok {
			tree.After[id], tree.Before[id] = n, domain.PathBefore(n, m.Changes, before)
			keys = append(keys, domain.WrittenKeys(n)...)
		}
	}
	slices.Sort(keys)
	named, err := r.Pages.ByKeys(ctx, m.NotebookID, slices.Compact(keys))
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
// not read back as the content's.
func (r Rewrite) rewrite(ctx context.Context, id uuid.UUID, revision int, reached map[int]rewritten, from []domain.Step,
	tree domain.Tree, recased domain.Recased, u Appender,
) error {
	content, current, err := r.Contents.Content(ctx, id)
	if err != nil {
		return err
	}
	if current != revision {
		r.Logger.LogAttrs(ctx, slog.LevelWarn, "the links of a page are not rewritten: its content is not the index's",
			slog.String("page_id", id.String()), slog.Int("revision", current), slog.Int("indexed", revision))
		return nil
	}
	was, err := r.Parser.ParseNow(ctx, content)
	if err != nil {
		return err
	}
	var links []domain.Resolved
	for _, l := range was.Facts.Links {
		if x, ok := reached[l.Start]; ok {
			links = append(links, domain.Resolved{Link: l, Before: x.before, After: x.after})
		}
	}
	// The content's share goes back before a writing's parse takes its own,
	// its links and aliases kept for Written uncounted until then, within
	// some tenth of what that parse holds (markdown.Hold.KeepFacts); the
	// rest of its facts go. Held, the two would ask more than the smallest
	// budget, a page's largest content, for a page at its largest, which no
	// wait would free (M6/P4 fix check c1, c5-6).
	was.Release()
	facts := domain.Facts{Links: was.Facts.Links, Aliases: was.Facts.Aliases} // what a writing is read back against
	rewriting := domain.Rewrite(content, from, links, tree, recased)
	for _, l := range rewriting.Left {
		r.Logger.LogAttrs(ctx, slog.LevelError, "a link is not rewritten: no writing leads where it led",
			slog.String("page_id", id.String()), slog.Int("start", l.Start), slog.String("target", l.Target))
	}
	if len(rewriting.Edits) == 0 {
		return nil
	}
	var now Parsed // the last writing's parse
	var large int  // the bytes of a writing past MaxContent, if one was
	parsed := 0    // the writings parsed, to read back
	written, left, kept, err := rewriting.Written(content, facts, from, tree, func(writing string) (domain.Facts, error) {
		now.release()
		now = Parsed{}
		if len(writing) > r.MaxContent {
			large = len(writing)
			return domain.Facts{}, domain.ErrTooLarge
		}
		var err error
		now, err = r.Parser.ParseNow(ctx, writing)
		parsed++
		return now.Facts, err
	})
	if err != nil {
		return err
	}
	var sizes []slog.Attr
	if large > 0 {
		sizes = []slog.Attr{slog.Int("bytes", len(content)), slog.Int("written", large)}
	}
	if !kept {
		now.release()
		level, msg := slog.LevelError, "the links of a page are not rewritten: no writing reads back as its links and aliases"
		if parsed == 0 {
			level, msg = slog.LevelWarn, "the links of a page are not rewritten: it would hold more than a page may"
		}
		r.Logger.LogAttrs(ctx, level, msg, append([]slog.Attr{slog.String("page_id", id.String())}, sizes...)...)
		return nil
	}
	// Left are the property links of a writing of the body alone, the one
	// writing parsed when those with them would all hold too much.
	level, msg := slog.LevelError, "a link is not rewritten: writing the frontmatter again would change more than its links"
	if parsed == 1 {
		level, msg = slog.LevelWarn, "a link is not rewritten: writing the frontmatter again would hold more than a page may"
	}
	for _, l := range left {
		r.Logger.LogAttrs(ctx, level, msg, append([]slog.Attr{
			slog.String("page_id", id.String()), slog.Int("start", l.Start), slog.String("target", l.Target),
		}, sizes...)...)
	}
	u.Defer(now.Release)
	return u.WriteContent(ctx, Rewritten{PageID: id, Base: revision, Content: written, Facts: now.Written})
}

// release gives back what p holds of the budget, if it holds any.
func (p Parsed) release() {
	if p.Release != nil {
		p.Release()
	}
}
