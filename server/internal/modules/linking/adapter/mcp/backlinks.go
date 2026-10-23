package mcpadapter

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// backlinksIn is backlinks' arguments (M9 design 4.9; M9/P4 design
// 3.10.3).
type backlinksIn struct {
	Path   string  `json:"path" jsonschema:"The page's path from the notebook's root, ending with .md."`
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many pages, 1 to 100. Default 50."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

// backlinksAnswer is backlinks' answer: the pages that link there, by id;
// next_cursor null when nothing follows.
type backlinksAnswer struct {
	Pages      []backlinkPage `json:"pages"`
	NextCursor *string        `json:"next_cursor"`
}

// backlinkPage is a page that links there: its path now, as changeset's
// node.path; how many of its links lead there, to linking's MaxCount; the
// lines of its first MaxContexts links, each MaxContext bytes at most.
type backlinkPage struct {
	Path     string   `json:"path"`
	Links    int      `json:"links"`
	Contexts []string `json:"contexts"`
}

// errNoBacklinkFits is a first page over the answer's bound, which a
// path's bound and the contexts' do not let happen: one is about 33 KB at
// most (M9/P4 design 3.9.2).
var errNoBacklinkFits = errors.New("linking: backlinks' first page does not fit in an answer")

// backlinks lists the pages that link to the page at the path: as many as
// the answer holds, next_cursor going on after the last of them (M9/P4
// design 3.9.2, 3.10.3).
func (t tools) backlinks(ctx context.Context, c mcpserver.Call, in backlinksIn) (mcpserver.Result, error) {
	b, err := t.uc.Backlinks.AtPath(ctx, c.Notebook, in.Path, in.Limit, in.Cursor)
	if err != nil {
		return mcpserver.Result{}, err
	}
	pages, cursors := make([]backlinkPage, len(b.Pages)), make([]string, len(b.Pages))
	for i, p := range b.Pages {
		pages[i], cursors[i] = backlinkPage{Path: p.Path, Links: p.Links, Contexts: p.Contexts}, p.Cursor
	}
	n := len(pages)
	res, k, err := mcpserver.Fit(n, func(k int) any {
		return backlinksAnswer{Pages: pages[:k], NextCursor: nextCursor(k, n, cursors, b.NextCursor)}
	})
	switch {
	case err != nil:
		return mcpserver.Result{}, err
	case k == 0 && n > 0:
		return mcpserver.Result{}, errNoBacklinkFits
	}
	return res, nil
}
