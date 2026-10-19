package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LogQuery is what of a notebook's log a read asks for: one actor's
// changesets, one client's (mcp: every MCP client's), those that changed a
// node; a page of limit from cursor.
type LogQuery struct {
	Actor  *uuid.UUID
	Client *string
	PageID *uuid.UUID
	Limit  *int
	Cursor *string
}

// ChangesetList is a page of a notebook's log, and the cursor of the next,
// "" after the last page.
type ChangesetList struct {
	Changesets []LoggedChangeset
	NextCursor string
}

// ListChangesets lists a notebook's log, a page of it at a time: GET
// /api/v0/notebooks/{notebook_id}/changesets (M8/P3 design 3.4).
type ListChangesets struct {
	r HistoryReads
}

// NewListChangesets returns the use case.
func NewListChangesets(r HistoryReads) *ListChangesets {
	return &ListChangesets{r: r}
}

// Execute returns the page of the notebook id's log that q asks for, newer
// first by when each changeset was first written, then by id, from the
// newest when its cursor is nil. The codes come in listAssets' order: a
// cursor that does not decode is 400 bad_request, judged first; then the
// notebook and the decision (page.history: any role in it), a notebook
// that does not exist, is deleted, or that the caller has no role in
// notebook.not_found; then a node that is none of the notebook's, alive or
// in the trash, page.not_found; then the client and the limit, 422. One
// changeset more than limit is read to tell whether another follows; then
// their counts, reverts and first changes, and their actors' names, each
// read once. A read takes no lock and opens no transaction.
func (l *ListChangesets) Execute(ctx context.Context, notebookID uuid.UUID, q LogQuery) (ChangesetList, error) {
	below := domain.ChangesetFrom()
	if q.Cursor != nil {
		if err := shared.DecodeCursor(*q.Cursor, &below); err != nil {
			return ChangesetList{}, err
		}
	}
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return ChangesetList{}, err
	}
	workspaceID, ok, err := l.r.Notebooks.WorkspaceOf(ctx, notebookID)
	switch {
	case err != nil:
		return ChangesetList{}, err
	case !ok:
		return ChangesetList{}, domain.ErrNotebookNotFound
	}
	target := shared.Target{WorkspaceID: workspaceID, NotebookID: notebookID}
	if _, err := authorize(ctx, l.r.Auth, actor, domain.ActionHistory, target, domain.ErrNotebookNotFound); err != nil {
		return ChangesetList{}, err
	}
	f := domain.LogFilter{Actor: q.Actor, PageID: q.PageID}
	if q.PageID != nil {
		nodes, err := l.r.History.NodesNow(ctx, []uuid.UUID{*q.PageID}, uuid.UUID{}) // no changeset reverted
		if err != nil {
			return ChangesetList{}, err
		}
		if n, ok := nodes[*q.PageID]; !ok || n.NotebookID != notebookID {
			return ChangesetList{}, domain.ErrNotFound
		}
	}
	if q.Client != nil {
		if f.Client, f.AllMCP, err = domain.ParseClientFilter(*q.Client); err != nil {
			return ChangesetList{}, err
		}
	}
	size, err := shared.PageSize(q.Limit)
	if err != nil {
		return ChangesetList{}, err
	}
	return l.page(ctx, notebookID, f, below, size)
}

// PathLogQuery is log's arguments (M9 design 4.9): Path a node's path for
// LogQuery's page id, Actor an account's id or "me".
type PathLogQuery struct {
	Path, Actor, Client *string
	Limit               *int
	Cursor              *string
}

// PathLog is a page of the log as log answers it: the changesets, each
// with the cursor that goes on after it, and the next page's.
type PathLog struct {
	Changesets []LoggedChangeset
	Cursors    []string
	NextCursor string
}

// AtPath lists the notebook's log as Execute does (M9/P2 design 3.16.2),
// its node by its path, alive, page or attachment, and "me" the caller.
// Its order: the actor; the cursor, 400; the path (a trailing "/"
// refused), the actor, the client and the limit, 422 each; the notebook,
// notebook.not_found (page.history: no 403); the node, page.not_found,
// found unlocked; then Execute's reads.
func (l *ListChangesets) AtPath(ctx context.Context, notebookID uuid.UUID, q PathLogQuery) (PathLog, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return PathLog{}, err
	}
	below := domain.ChangesetFrom()
	if q.Cursor != nil {
		if err := shared.DecodeCursor(*q.Cursor, &below); err != nil {
			return PathLog{}, err
		}
	}
	var path domain.Path
	if q.Path != nil {
		if path, err = domain.ParsePath("path", *q.Path, false); err != nil {
			return PathLog{}, err
		}
	}
	var f domain.LogFilter
	if q.Actor != nil {
		id, err := domain.ParseActor(*q.Actor, actor.UserID)
		if err != nil {
			return PathLog{}, err
		}
		f.Actor = &id
	}
	if q.Client != nil {
		if f.Client, f.AllMCP, err = domain.ParseClientFilter(*q.Client); err != nil {
			return PathLog{}, err
		}
	}
	size, err := shared.PageSize(q.Limit)
	if err != nil {
		return PathLog{}, err
	}
	if err := decideNotebook(ctx, l.r.Notebooks, l.r.Auth, actor, domain.ActionHistory, notebookID); err != nil {
		return PathLog{}, err
	}
	if q.Path != nil {
		at, err := NewNodePaths(l.r.Paths).Find(ctx, notebookID, path)
		if err != nil {
			return PathLog{}, err
		}
		f.PageID = &at.Node.ID
	}
	list, err := l.page(ctx, notebookID, f, below, size)
	if err != nil {
		return PathLog{}, err
	}
	cursors, err := cursorsAfter(list.Changesets, func(c LoggedChangeset) any {
		return domain.ChangesetCursor{At: c.Changeset.CreatedAt, ID: c.Changeset.ID}
	})
	if err != nil {
		return PathLog{}, err
	}
	return PathLog{Changesets: list.Changesets, Cursors: cursors, NextCursor: list.NextCursor}, nil
}

// page is a page of the notebook's log that f keeps, newer first, below
// below, size at most: one changeset more is read to tell whether another
// follows; then their counts, reverts and first changes, and their
// actors' names, each read once.
func (l *ListChangesets) page(ctx context.Context, notebookID uuid.UUID, f domain.LogFilter, below domain.ChangesetCursor,
	size int,
) (ChangesetList, error) {
	changesets, err := l.r.History.ListChangesets(ctx, notebookID, f, below, size+1)
	if err != nil {
		return ChangesetList{}, err
	}
	var out ChangesetList
	if len(changesets) > size {
		changesets = changesets[:size]
		last := changesets[size-1]
		if out.NextCursor, err = shared.EncodeCursor(domain.ChangesetCursor{At: last.CreatedAt, ID: last.ID}); err != nil {
			return ChangesetList{}, err
		}
	}
	if out.Changesets, err = l.r.logged(ctx, changesets); err != nil {
		return ChangesetList{}, err
	}
	return out, nil
}
