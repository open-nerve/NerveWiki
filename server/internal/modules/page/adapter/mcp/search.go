package mcpadapter

import (
	"context"
	"errors"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// searchIn is search's arguments (M9 design 4.13; M9/P4 design 3.10.2).
type searchIn struct {
	Q      string  `json:"q" jsonschema:"The words to find, parted by spaces: at most 256 bytes and 8 words."`
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many results, 1 to 50. Default 20."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

// searchAnswer is search's answer: the results, best first; the first
// page's count, null on the others; next_cursor null when nothing follows.
type searchAnswer struct {
	Results    []searchResult `json:"results"`
	Count      *searchCount   `json:"count"`
	NextCursor *string        `json:"next_cursor"`
	Building   bool           `json:"building"`
}

// searchResult is a node found: its path, its kind, a page's excerpt
// about its first hit in the content, plain text with "…" where it is
// cut, null without one, and when it last changed.
type searchResult struct {
	Path      string    `json:"path"`
	Kind      string    `json:"kind"`
	Excerpt   *string   `json:"excerpt"`
	ChangedAt time.Time `json:"changed_at"`
}

// searchCount is the first page's count, as M8's search tells it: exact,
// about or at_least.
type searchCount struct {
	Value int    `json:"value"`
	Kind  string `json:"kind"`
}

// errNoResultFits is a first result over the answer's bound, which a
// path's bound and an excerpt's do not let happen (M9/P4 design 3.10.2).
var errNoResultFits = errors.New("page: search's first result does not fit in an answer")

// search searches the notebook's words, its titles first, then its
// contents: as many results as the answer holds, next_cursor going on
// after the last of them, by its tier (M9/P4 design 3.8, 3.10.2).
func (t tools) search(ctx context.Context, c mcpserver.Call, in searchIn) (mcpserver.Result, error) {
	q := in.Q
	p, err := t.uc.Search.InNotebook(ctx, c.Notebook, app.SearchRequest{Q: &q, Limit: in.Limit, Cursor: in.Cursor})
	if err != nil {
		return mcpserver.Result{}, toolErr(err, c, "")
	}
	results, cursors := make([]searchResult, len(p.Hits)), make([]string, len(p.Hits))
	for i, h := range p.Hits {
		results[i] = searchResultOf(h)
		if cursors[i], err = h.Cursor(); err != nil {
			return mcpserver.Result{}, err
		}
	}
	var count *searchCount
	if p.Count != nil {
		count = &searchCount{Value: p.Count.Value, Kind: string(p.Count.Kind)}
	}
	n := len(results)
	res, k, err := mcpserver.Fit(n, func(k int) any {
		return searchAnswer{Results: results[:k], Count: count, NextCursor: nextCursor(k, n, cursors, p.NextCursor),
			Building: p.Building}
	})
	switch {
	case err != nil:
		return mcpserver.Result{}, err
	case k == 0 && n > 0:
		return mcpserver.Result{}, errNoResultFits
	}
	return res, nil
}

// searchResultOf is the hit h as search answers it: its path from its
// ancestors' names, kind "attachment" for an asset, the excerpt's text
// with "…" at each end it is cut at, the time in UTC.
func searchResultOf(h app.SearchHit) searchResult {
	names := make([]string, 0, len(h.Ancestors)+1)
	for _, a := range h.Ancestors {
		names = append(names, a.Name)
	}
	r := searchResult{Path: domain.PathOfNames(append(names, h.Node.Name), h.Node.Kind), Kind: "attachment",
		ChangedAt: h.ChangedAt.UTC()}
	if h.Node.Kind == domain.KindPage {
		r.Kind = "page"
	}
	if e := h.Excerpt; e != nil {
		text := e.Text
		if e.CutStart {
			text = "…" + text
		}
		if e.CutEnd {
			text += "…"
		}
		r.Excerpt = &text
	}
	return r
}
