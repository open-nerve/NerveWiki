package mcpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// logIn is log's arguments (M9/P2 design 3.16.2).
type logIn struct {
	Path   *string `json:"path,omitempty" jsonschema:"Only the changesets that changed the page or attachment at this path, from the notebook's root."`
	Actor  *string `json:"actor,omitempty" jsonschema:"Only the changesets by this account: its id, or \"me\" for yours."`
	Client *string `json:"client,omitempty" jsonschema:"Only the changesets of this client: \"web\", \"api\", \"cli\", \"mcp:<name>\" for an agent, or \"mcp\" for every agent."`
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many changesets, 1 to 100. Default 50."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

// logAnswer is log's answer (M9/P2 design 3.16.2): the changesets, newest
// first; next_cursor null when nothing follows.
type logAnswer struct {
	Changesets []changesetRow `json:"changesets"`
	NextCursor *string        `json:"next_cursor"`
}

// log lists the notebook's changesets: as many rows as the answer holds,
// next_cursor going on after the last of them. A first row the answer
// cannot hold whole comes alone, with as many of its changes as fit and
// changes_cut, next_cursor going on after it: each call goes on by a row
// at least (M9/P2 design 3.16.2, 3.16.4).
func (t tools) log(ctx context.Context, c mcpserver.Call, in logIn) (mcpserver.Result, error) {
	var path string
	if in.Path != nil {
		path = *in.Path
	}
	l, err := t.uc.Log.AtPath(ctx, c.Notebook, app.PathLogQuery{Path: in.Path, Actor: in.Actor, Client: in.Client, Limit: in.Limit,
		Cursor: in.Cursor})
	if err != nil {
		return mcpserver.Result{}, toolErr(err, c, path)
	}
	rows := make([]changesetRow, len(l.Changesets))
	for i, lc := range l.Changesets {
		rows[i] = changesetRowOf(lc)
	}
	n := len(rows)
	res, k, err := mcpserver.Fit(n, func(k int) any {
		return logAnswer{Changesets: rows[:k], NextCursor: nextCursor(k, n, l.Cursors, l.NextCursor)}
	})
	if err != nil || k > 0 || n == 0 {
		return res, err
	}
	next := l.NextCursor // the one row: the next page's
	if n > 1 {
		next = l.Cursors[0]
	}
	first := rows[0]
	res, _, err = mcpserver.Fit(len(first.Changes), func(j int) any {
		row := first
		row.Changes, row.ChangesCut = first.Changes[:j], true
		return logAnswer{Changesets: []changesetRow{row}, NextCursor: cursorOf(next)}
	})
	return res, err
}
