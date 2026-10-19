package mcpadapter

import (
	"context"
	"errors"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// listIn is list's arguments (M9/P1 design 3.10.5).
type listIn struct {
	Pattern *string `json:"pattern,omitempty" jsonschema:"\"\" or \"/\" for the root's children, \"A/B/\" for the children of A/B.md, or a glob. Default: the root's children."`
	Limit   *int    `json:"limit,omitempty" jsonschema:"At most this many items, 1 to 1000. Default 200."`
	Cursor  *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

// listAnswer is list's answer (M9/P1 design 3.10.2): next_cursor null
// when nothing may follow.
type listAnswer struct {
	Items      []listItem `json:"items"`
	NextCursor *string    `json:"next_cursor"`
	Hint       string     `json:"hint,omitempty"`
}

// listItem is an item of list: a page's version, size and last change, or
// an attachment's type and size.
type listItem struct {
	Path        string     `json:"path"`
	Kind        string     `json:"kind"`
	HasChildren bool       `json:"has_children"`
	Version     string     `json:"version,omitempty"`
	MIME        string     `json:"mime,omitempty"`
	Bytes       int64      `json:"byte_size"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// errNoItemFits is a first item over the answer's bound, which no path's
// bound lets happen (M9/P1 design 3.10.5).
var errNoItemFits = errors.New("page: list's first item does not fit in an answer")

// list lists the nodes the pattern names: as many items as the answer
// holds, next_cursor going on after the last of them.
func (t tools) list(ctx context.Context, c mcpserver.Call, in listIn) (mcpserver.Result, error) {
	var pattern string
	if in.Pattern != nil {
		pattern = *in.Pattern
	}
	l, err := t.uc.List.Execute(ctx, c.Notebook, pattern, in.Limit, in.Cursor)
	if err != nil {
		return mcpserver.Result{}, toolErr(err, c, pattern)
	}
	items := make([]listItem, len(l.Items))
	for i, it := range l.Items {
		items[i] = itemOf(it)
	}
	n := len(items)
	res, k, err := mcpserver.Fit(n, func(k int) any {
		a := listAnswer{Items: items[:k], Hint: l.Hint}
		next := l.Next
		if k < n {
			next = ""
			if k > 0 {
				next = l.Items[k-1].Cursor
			}
		}
		if next != "" {
			a.NextCursor = &next
		}
		return a
	})
	switch {
	case err != nil:
		return mcpserver.Result{}, err
	case k == 0 && n > 0:
		return mcpserver.Result{}, errNoItemFits
	}
	return res, nil
}

// itemOf is it as list answers it: kind "attachment", not the store's
// "asset".
func itemOf(it app.Listed) listItem {
	item := listItem{Path: it.Path, Kind: "attachment", HasChildren: it.HasChildren}
	switch {
	case it.Page != nil:
		updated := it.Page.UpdatedAt.UTC()
		item.Kind, item.Bytes, item.UpdatedAt = "page", int64(it.Page.Bytes), &updated
		item.Version = domain.VersionToken{Node: it.Node.ID, Revision: it.Page.Revision}.String()
	case it.Attachment != nil:
		item.MIME, item.Bytes = it.Attachment.MIME, it.Attachment.Bytes
	}
	return item
}
