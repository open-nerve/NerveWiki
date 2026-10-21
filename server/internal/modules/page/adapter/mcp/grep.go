package mcpadapter

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// grepIn is grep's arguments (M9 design 4.13; M9/P4 design 3.10.1).
type grepIn struct {
	Pattern    string  `json:"pattern" jsonschema:"The Go regular expression (RE2), at most 1024 bytes, matched against each line alone. It is read in NFC, as the lines are: write a combining mark as an escape, such as \\x{301}, to keep it apart from the character before it."`
	Glob       *string `json:"glob,omitempty" jsonschema:"Only the pages whose paths match this glob, as list's: \"Projects/**\", \"**/*.md\". Default: every page."`
	IgnoreCase *bool   `json:"ignore_case,omitempty" jsonschema:"Match without case, as (?i). Default false."`
	Limit      *int    `json:"limit,omitempty" jsonschema:"At most this many lines with a hit, 1 to 500. Default 100."`
	Cursor     *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

// grepAnswer is grep's answer: the hits and the partial lines, each in
// path and line order; next_cursor null when nothing follows.
type grepAnswer struct {
	Hits         []grepHit  `json:"hits"`
	PartialLines []grepLine `json:"partial_lines"`
	NextCursor   *string    `json:"next_cursor"`
	Building     bool       `json:"building"`
	Incomplete   bool       `json:"incomplete"`
}

// grepHit is a line with a hit: its text as stored, without its "\r" and
// line 1's byte order mark, or 1,000 bytes of it about the match, cut.
type grepHit struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Text   string `json:"text"`
	Column *int   `json:"column,omitempty"` // when the text does not start at the line's first byte
	Cut    bool   `json:"cut,omitempty"`
}

// grepLine is a line too long to match whole within a call.
type grepLine struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// errNoLineFits is a first line over the answer's bound, which the bounds
// of a path, a hit's text and a cursor do not let happen (M9/P4 design
// 3.10.1, review A-n6).
var errNoLineFits = errors.New("page: grep's first line does not fit in an answer")

// grep finds the lines of the notebook's pages a pattern matches: as many
// of them as the answer holds, hits and partial lines in their one order,
// next_cursor going on after the last of them. incomplete is the use
// case's only when every line fits: an answer cut at its bound is not
// (M9/P4 design 3.10.1, 2.1 item 7).
func (t tools) grep(ctx context.Context, c mcpserver.Call, in grepIn) (mcpserver.Result, error) {
	g, err := t.uc.Grep.Execute(ctx, c.Notebook, app.GrepRequest{Pattern: in.Pattern, Glob: in.Glob,
		IgnoreCase: in.IgnoreCase != nil && *in.IgnoreCase, Limit: in.Limit, Cursor: in.Cursor})
	if err != nil {
		return mcpserver.Result{}, toolErr(err, c, "")
	}
	n, cursors := len(g.Found), make([]string, len(g.Found))
	for i, f := range g.Found {
		cursors[i] = f.Cursor
	}
	res, k, err := mcpserver.Fit(n, func(k int) any {
		a := grepAnswer{Hits: []grepHit{}, PartialLines: []grepLine{}, Building: g.Building, Incomplete: g.Incomplete && k == n}
		for _, f := range g.Found[:k] {
			if f.Hit == nil {
				a.PartialLines = append(a.PartialLines, grepLine{Path: f.Path, Line: f.Line})
				continue
			}
			h := grepHit{Path: f.Path, Line: f.Line, Text: f.Hit.Text, Cut: f.Hit.Cut}
			if f.Hit.Column > 0 {
				h.Column = new(f.Hit.Column)
			}
			a.Hits = append(a.Hits, h)
		}
		a.NextCursor = nextCursor(k, n, cursors, g.Next)
		return a
	})
	switch {
	case err != nil:
		return mcpserver.Result{}, err
	case k == 0 && n > 0:
		return mcpserver.Result{}, errNoLineFits
	}
	return res, nil
}
