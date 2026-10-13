package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ChangeList is a page of a changeset's changes, and the cursor of the
// next, "" after the last page.
type ChangeList struct {
	Changes    []domain.ChangeRow
	NextCursor string
}

// ListChanges lists a changeset's changes, a page of them at a time: GET
// /api/v0/changesets/{changeset_id}/changes (M8/P3 design 3.4).
type ListChanges struct {
	r HistoryReads
}

// NewListChanges returns the use case.
func NewListChanges(r HistoryReads) *ListChanges {
	return &ListChanges{r: r}
}

// Execute returns the page of the changeset id's changes that limit and
// cursor ask for, by node id, from the first when cursor is nil. A cursor
// that does not decode is 400 bad_request, judged first; then the
// changeset as GetChangeset finds it, page.changeset_not_found; then the
// limit, 422 outside 1–100. One change more than limit is read to tell
// whether another follows. A read takes no lock and opens no transaction.
func (l *ListChanges) Execute(ctx context.Context, id uuid.UUID, limit *int, cursor *string) (ChangeList, error) {
	var after domain.ChangeCursor
	if cursor != nil {
		if err := shared.DecodeCursor(*cursor, &after); err != nil {
			return ChangeList{}, err
		}
	}
	c, err := l.r.readableChangeset(ctx, id)
	if err != nil {
		return ChangeList{}, err
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return ChangeList{}, err
	}
	changes, err := l.r.History.ListChanges(ctx, c.ID, after.After, size+1)
	if err != nil {
		return ChangeList{}, err
	}
	var out ChangeList
	if len(changes) > size {
		changes = changes[:size]
		if out.NextCursor, err = shared.EncodeCursor(domain.ChangeCursor{After: changes[size-1].NodeID}); err != nil {
			return ChangeList{}, err
		}
	}
	out.Changes = changes
	return out, nil
}
