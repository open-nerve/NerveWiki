package mcpadapter

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// indexIn is index's arguments (M9 design 4.9; M9/P4 design 3.10.4).
type indexIn struct {
	Type   *string `json:"type,omitempty" jsonschema:"Only the pages whose frontmatter's type is exactly this."`
	Tag    *string `json:"tag,omitempty" jsonschema:"Only the pages with this tag or one under it, with or without its leading #."`
	Parent *string `json:"parent,omitempty" jsonschema:"Only the child pages of this page, by its path ending with .md; \"/\" for the root's."`
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many pages, 1 to 100. Default 50."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

// indexAnswer is index's answer: the pages, those with a type first;
// next_cursor null when nothing follows.
type indexAnswer struct {
	Pages      []indexPage `json:"pages"`
	NextCursor *string     `json:"next_cursor"`
}

// indexPage is a page of the index: its path; its type and summary, null
// for none; its tags; its content's last change; how many other pages link
// to it, to MaxInbound; its parent page's path, null at the root; and what
// was cut: "type", "summary", "tags[i]", and "tags" for more than 20.
type indexPage struct {
	Path         string    `json:"path"`
	Type         *string   `json:"type"`
	Summary      *string   `json:"summary"`
	Tags         []string  `json:"tags"`
	ChangedAt    time.Time `json:"changed_at"`
	InboundLinks int       `json:"inbound_links"`
	Parent       *string   `json:"parent"`
	Cut          []string  `json:"cut"`
}

// errNoIndexPageFits is a first page over the answer's bound without its
// tags, which the bounds of a path, a type and a summary do not let happen:
// one is about 33 KB at most then (M9/P4 design 3.10.4, review B-M8).
var errNoIndexPageFits = errors.New("linking: index's first page does not fit in an answer without its tags")

// index lists the pages of the notebook's index: as many as the answer
// holds, next_cursor going on after the last of them. A first page the
// answer cannot hold comes alone without its tags, "tags" in its cut,
// next_cursor going on after it: each call goes on by a page at least
// (M9/P4 design 3.10.4).
func (t tools) index(ctx context.Context, c mcpserver.Call, in indexIn) (mcpserver.Result, error) {
	p, err := t.uc.Index.Execute(ctx, c.Notebook, app.IndexQuery{Type: in.Type, Tag: in.Tag, Parent: in.Parent, Limit: in.Limit,
		Cursor: in.Cursor})
	if err != nil {
		return mcpserver.Result{}, err
	}
	pages, cursors := make([]indexPage, len(p.Items)), make([]string, len(p.Items))
	for i, it := range p.Items {
		pages[i], cursors[i] = indexPageOf(it), it.Cursor
	}
	n := len(pages)
	res, k, err := mcpserver.Fit(n, func(k int) any {
		return indexAnswer{Pages: pages[:k], NextCursor: nextCursor(k, n, cursors, p.Next)}
	})
	if err != nil || k > 0 || n == 0 {
		return res, err
	}
	next := p.Next // the one page: the next page's
	if n > 1 {
		next = cursors[0]
	}
	first := pages[0]
	first.Tags = []string{}
	first.Cut = append(slices.DeleteFunc(slices.Clone(first.Cut), func(c string) bool { return strings.HasPrefix(c, "tags") }), "tags")
	res, k, err = mcpserver.Fit(1, func(k int) any {
		return indexAnswer{Pages: []indexPage{first}[:k], NextCursor: cursorOf(next)}
	})
	switch {
	case err != nil:
		return mcpserver.Result{}, err
	case k == 0:
		return mcpserver.Result{}, errNoIndexPageFits
	}
	return res, nil
}

// indexPageOf is the item it as index answers it, the time in UTC.
func indexPageOf(it app.IndexItem) indexPage {
	return indexPage{Path: it.Path, Type: it.Type, Summary: it.Summary, Tags: it.Tags, ChangedAt: it.ChangedAt.UTC(),
		InboundLinks: it.Inbound, Parent: it.ParentPath, Cut: it.Cut}
}
