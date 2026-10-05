package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Backlinks is a page of the pages that link to a page, and the cursor of
// the next, "" after the last page.
type Backlinks struct {
	Pages      []Backlinking
	NextCursor string
}

// Backlinking is a page that links to another: how many of its links lead
// there, and the contexts of the first of them, one a line (M6/P5
// design 3).
type Backlinking struct {
	PageID   uuid.UUID
	Links    int
	Contexts []string
}

// ListBacklinks lists the pages that link to a page, a page of them at a
// time: GET /api/v0/pages/{page_id}/backlinks (M6/P5 design 3).
type ListBacklinks struct {
	Access   Access
	Reads    Reads
	Contents PageContents
}

// Execute returns the page of the pages that link to the page id that
// limit and cursor ask for, by id, from the first when cursor is nil. A
// cursor shared.DecodeCursor refuses is 400 bad_request, judged first;
// then the page and the decision; then the limit, 422 outside 1–100. One
// page more than limit is read to tell whether another follows. Each
// page's contexts are cut from its content, read one page at a time and
// let go; a content written since the index's rows, or gone, gives none,
// and the links event that follows has them read again. A read takes no
// lock and opens no transaction.
func (l ListBacklinks) Execute(ctx context.Context, id uuid.UUID, limit *int, cursor *string) (Backlinks, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Backlinks{}, err
	}
	var after uuid.UUID
	if cursor != nil {
		if err := shared.DecodeCursor(*cursor, &after); err != nil {
			return Backlinks{}, err
		}
	}
	if _, err := l.Access.page(ctx, actor, id, domain.ActionListBacklinks); err != nil {
		return Backlinks{}, err
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return Backlinks{}, err
	}
	rows, err := l.Reads.Backlinks(ctx, id, after, size+1, domain.MaxContexts)
	if err != nil {
		return Backlinks{}, err
	}
	var out Backlinks
	if len(rows) > size {
		rows = rows[:size]
		if out.NextCursor, err = shared.EncodeCursor(rows[size-1].SourceID); err != nil {
			return Backlinks{}, err
		}
	}
	out.Pages = make([]Backlinking, len(rows))
	for i, b := range rows {
		out.Pages[i] = Backlinking{PageID: b.SourceID, Links: b.Links, Contexts: []string{}}
		content, revision, ok, err := l.Contents.Content(ctx, b.SourceID)
		if err != nil {
			return Backlinks{}, err
		}
		if ok && revision == b.Revision {
			out.Pages[i].Contexts = domain.Contexts(content, b.Ranges)
		}
	}
	return out, nil
}
