package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// IndexQuery is index's arguments (M9 design 4.9).
type IndexQuery struct {
	Type, Tag, Parent *string
	Limit             *int
	Cursor            *string
}

// IndexItem is a page of the index: its path, type, summary and tags, cut
// as Cut names them, when its content last changed, how many other pages
// link to it (to MaxInbound), its parent page's path (nil at the root), and
// the cursor after it.
type IndexItem struct {
	PageID     uuid.UUID
	Path       string
	Type       *string
	Summary    *string
	Tags       []string
	ChangedAt  time.Time
	Inbound    int
	ParentPath *string
	Cut        []string
	Cursor     string
}

// IndexPage is a page of index, and the cursor after it, "" after the last.
type IndexPage struct {
	Items []IndexItem
	Next  string
}

// ListIndex lists the pages of a notebook the link index holds, filtered
// by type, tag and parent page (M9 design 4.9; M9/P4 design 3.9.3).
type ListIndex struct {
	Access Access
	Reads  IndexReads
	Paths  NodePaths
	// TagKey is the key a tag's name is kept by; false for a name no
	// page's tag has, as GetTag's.
	TagKey func(name string) (string, bool)
}

// Execute is the page of q in notebookID: the cursor, 400; type (NUL,
// longer than MaxIndexType) and parent (NodePaths.Check, "/" the root),
// 422 at their fields; "" refused for type, tag and parent; the limit, 422, 1 to
// 100, 50 (shared.PageSize); the notebook, index.list, notebook.not_found;
// parent's page, page.not_found; then the index's pages read whole with
// the starts of their types (of type alone when it is given), filtered,
// ordered, the page of them after the cursor, and the page's types,
// summaries, tags, inbound counts, paths and last changes, a page deleted
// since left out. A tag loses one leading "#"; one no page can have (NUL
// among them) gives an empty page, as GetTag. A read takes no lock and
// opens no transaction.
func (l ListIndex) Execute(ctx context.Context, notebookID uuid.UUID, q IndexQuery) (IndexPage, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return IndexPage{}, err
	}
	var after *domain.IndexCursor
	if q.Cursor != nil {
		var c domain.IndexCursor
		if err := shared.DecodeCursor(*q.Cursor, &c); err != nil {
			return IndexPage{}, err
		}
		after = &c
	}
	if err := l.check(q); err != nil {
		return IndexPage{}, err
	}
	size, err := shared.PageSize(q.Limit)
	if err != nil {
		return IndexPage{}, err
	}
	if err := l.Access.notebook(ctx, actor, notebookID, domain.ActionListIndex); err != nil {
		return IndexPage{}, err
	}
	none := IndexPage{Items: []IndexItem{}}
	var children, tagged map[uuid.UUID]bool // nil: not filtered
	if q.Parent != nil {
		if children, err = l.childPages(ctx, notebookID, *q.Parent); err != nil || len(children) == 0 {
			return none, err
		}
	}
	if q.Tag != nil {
		key, ok := l.TagKey(strings.TrimPrefix(*q.Tag, "#"))
		if !ok {
			return none, nil
		}
		ids, err := l.Reads.TagPages(ctx, notebookID, key)
		if err != nil || len(ids) == 0 {
			return none, err
		}
		tagged = setOf(ids)
	}
	pages, err := l.Reads.IndexPages(ctx, notebookID, q.Type)
	if err != nil {
		return IndexPage{}, err
	}
	// One keyer for every type: its buffers are kept (M9/P4 design 3.9.3).
	keyer, order := domain.NewIndexKeyer(), make([]domain.IndexCursor, 0, len(pages))
	for _, p := range pages {
		if children != nil && !children[p.ID] || tagged != nil && !tagged[p.ID] {
			continue
		}
		c := domain.IndexCursor{Typed: p.Typed, ID: p.ID}
		if p.Typed {
			c.K = keyer.Key(p.TypeStart)
		}
		if after == nil || domain.CompareIndex(c, *after) > 0 {
			order = append(order, c)
		}
	}
	slices.SortFunc(order, domain.CompareIndex)
	out := none
	if len(order) > size {
		order = order[:size]
		// After the last read, which a page deleted since leaves out.
		if out.Next, err = shared.EncodeCursor(order[size-1]); err != nil {
			return IndexPage{}, err
		}
	}
	if len(order) == 0 {
		return out, nil
	}
	return l.items(ctx, notebookID, order, out)
}

// check is q's 422s, each at its field, before any read: "" for any filter,
// a type with NUL or longer than MaxIndexType, a parent that is no page's
// path but "/" (M9/P4 design 2.1, item 13; review B-M7).
func (l ListIndex) check(q IndexQuery) error {
	if t := q.Type; t != nil {
		switch {
		case *t == "":
			return indexField("type", shared.FieldTooShort, "must not be empty: omit it for the pages of every type")
		case len(*t) > domain.MaxIndexType:
			return indexField("type", shared.FieldTooLong, fmt.Sprintf("must be at most %d bytes", domain.MaxIndexType))
		case strings.ContainsRune(*t, 0):
			return indexField("type", shared.FieldInvalidFormat, "must not hold U+0000")
		}
	}
	if q.Tag != nil && *q.Tag == "" {
		return indexField("tag", shared.FieldTooShort, "must not be empty: omit it for the pages of every tag")
	}
	if p := q.Parent; p != nil {
		switch {
		case *p == "":
			return indexField("parent", shared.FieldTooShort, `must not be empty: omit it for every page, or use "/" for the root's`)
		case *p != "/":
			return l.Paths.Check("parent", *p)
		}
	}
	return nil
}

// childPages is the child pages of the page at parent, the root's for "/":
// page.not_found for none.
func (l ListIndex) childPages(ctx context.Context, notebookID uuid.UUID, parent string) (map[uuid.UUID]bool, error) {
	var id uuid.UUID
	if parent != "/" {
		var err error
		if id, err = l.Paths.FindPage(ctx, notebookID, "parent", parent); err != nil {
			return nil, err
		}
	}
	ids, err := l.Paths.ChildPages(ctx, notebookID, id)
	if err != nil {
		return nil, err
	}
	return setOf(ids), nil
}

// items is out with the items of the pages order names, in its order: a
// page whose path or index rows are gone is left out.
func (l ListIndex) items(ctx context.Context, notebookID uuid.UUID, order []domain.IndexCursor, out IndexPage) (IndexPage, error) {
	ids := make([]uuid.UUID, len(order))
	for i, c := range order {
		ids[i] = c.ID
	}
	facts, err := l.Reads.IndexItems(ctx, ids)
	if err != nil {
		return IndexPage{}, err
	}
	inbound, err := l.Reads.InboundCounts(ctx, ids, domain.MaxInbound)
	if err != nil {
		return IndexPage{}, err
	}
	paths, err := l.Paths.Pages(ctx, notebookID, ids)
	if err != nil {
		return IndexPage{}, err
	}
	for _, c := range order {
		p, alive := paths[c.ID]
		f, indexed := facts[c.ID]
		if !alive || !indexed {
			continue
		}
		cursor, err := shared.EncodeCursor(c)
		if err != nil {
			return IndexPage{}, err
		}
		out.Items = append(out.Items, indexItemOf(c.ID, p, f, inbound[c.ID], cursor))
	}
	return out, nil
}

// indexItemOf is the item of the page id at p with the facts f: its type,
// summary and tags cut to MaxIndexString bytes, at most MaxIndexTags tags,
// cut naming each cut and "tags" for more; its parent's path that of its
// path's last segment's parent, a page's.
func indexItemOf(id uuid.UUID, p PagePath, f IndexFacts, inbound int, cursor string) IndexItem {
	it := IndexItem{PageID: id, Path: p.Path, Tags: []string{}, ChangedAt: p.ChangedAt, Inbound: inbound, Cut: []string{}, Cursor: cursor}
	if i := strings.LastIndex(p.Path, "/"); i >= 0 {
		parent := p.Path[:i] + ".md"
		it.ParentPath = &parent
	}
	cut := func(s *string, name string) *string {
		if s == nil {
			return nil
		}
		v, cut := domain.CutIndexString(*s)
		if cut {
			it.Cut = append(it.Cut, name)
		}
		return &v
	}
	it.Type, it.Summary = cut(f.Type, "type"), cut(f.Summary, "summary")
	tags := f.Tags
	if len(tags) > domain.MaxIndexTags {
		tags = tags[:domain.MaxIndexTags]
	}
	for i, tag := range tags {
		it.Tags = append(it.Tags, *cut(&tag, fmt.Sprintf("tags[%d]", i)))
	}
	if len(f.Tags) > domain.MaxIndexTags {
		it.Cut = append(it.Cut, "tags")
	}
	return it
}

// indexField is 422 at field with code and message.
func indexField(field, code, message string) error {
	return shared.Invalid(shared.FieldError{Field: field, Code: code, Message: message})
}
