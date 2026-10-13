package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// TrashList is a page of a notebook's trash, and the cursor of the next,
// "" after the last page.
type TrashList struct {
	Items      []TrashItem
	NextCursor string
}

// ListTrash lists a notebook's trash, a page of it at a time: GET
// /api/v0/notebooks/{notebook_id}/trash (M8/P2 design 3.5).
type ListTrash struct {
	r TrashReads
}

// NewListTrash returns the use case.
func NewListTrash(r TrashReads) *ListTrash {
	return &ListTrash{r: r}
}

// Execute returns the page of the notebook id's trash items that limit and
// cursor ask for, newer-first by when they were deleted, then by id, from
// the newest when cursor is nil; only those that can be restored still. A
// cursor that does not decode is 400 bad_request, judged first; then the
// notebook and the decision (page.history: any role in it), a notebook
// that does not exist, is deleted, or that the caller has no role in
// notebook.not_found; then the limit, 422 outside 1–100. One item more than
// limit is read to tell whether another follows; then the items' lines,
// their deleters' names and their attachments' infos, each read once. A
// read takes no lock and opens no transaction.
func (l *ListTrash) Execute(ctx context.Context, id uuid.UUID, limit *int, cursor *string) (TrashList, error) {
	below := domain.TrashFrom()
	if cursor != nil {
		if err := shared.DecodeCursor(*cursor, &below); err != nil {
			return TrashList{}, err
		}
	}
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return TrashList{}, err
	}
	workspaceID, ok, err := l.r.Notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return TrashList{}, err
	case !ok:
		return TrashList{}, domain.ErrNotebookNotFound
	}
	target := shared.Target{WorkspaceID: workspaceID, NotebookID: id}
	if _, err := authorize(ctx, l.r.Auth, actor, domain.ActionHistory, target, domain.ErrNotebookNotFound); err != nil {
		return TrashList{}, err
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return TrashList{}, err
	}
	roots, err := l.r.Trash.ListTrash(ctx, id, domain.TrashSince(l.r.Clock.Now(), l.r.Retention), below, size+1)
	if err != nil {
		return TrashList{}, err
	}
	var out TrashList
	if len(roots) > size {
		roots = roots[:size]
		last := roots[size-1]
		if out.NextCursor, err = shared.EncodeCursor(domain.TrashCursor{At: last.DeletedAt, ID: last.Node.ID}); err != nil {
			return TrashList{}, err
		}
	}
	if out.Items, err = l.r.trashItems(ctx, roots); err != nil {
		return TrashList{}, err
	}
	return out, nil
}
