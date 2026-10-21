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
// there, up to domain.MaxCount, and the contexts of the first of them, one
// a line (M6/P5 design 3).
type Backlinking struct {
	PageID   uuid.UUID
	Links    int
	Contexts []string
}

// BacklinksAt is a page of the pages that link to the page at a path:
// each with its path and the cursor after it, so that the adapter can stop
// at the answer's bound; NextCursor after the page, "" after the last.
type BacklinksAt struct {
	Pages      []BacklinkingAt
	NextCursor string
}

// BacklinkingAt is a page that links there, at its path now.
type BacklinkingAt struct {
	Backlinking
	Path   string
	Cursor string
}

// ListBacklinks lists the pages that link to a page, a page of them at a
// time: GET /api/v0/pages/{page_id}/backlinks (M6/P5 design 3), and MCP's
// backlinks by the page's path (M9/P4 design 3.9.2), which Paths finds and
// writes.
type ListBacklinks struct {
	Access   Access
	Reads    Reads
	Contents PageContents
	Paths    NodePaths
}

// Execute returns the page of the pages that link to the page id that
// limit and cursor ask for, by id, from the first when cursor is nil. A
// cursor shared.DecodeCursor refuses is 400 bad_request, judged first;
// then the page and the decision; then the limit, 422 outside 1–100. One
// page more than limit is read to tell whether another follows. Each
// page's contexts are cut from its content, read one page at a time and
// let go; a content written since the index's rows, or gone, gives none,
// and the links event that follows has them read again. So does a page
// whose rows are of another extractor, before nervewiki reindex, whose
// ranges may mean other bytes; and every page after domain.MaxContentRead
// bytes of contents are read. A read takes no lock and opens no
// transaction.
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
	return l.read(ctx, id, after, size)
}

// AtPath is Execute for the page at path in notebookID, as an MCP call
// asks for it (M9 design 4.9; M9/P4 design 3.9.2): the cursor, 400; the
// path, 422 (NodePaths.Check); the limit, 422 (shared.PageSize); the
// notebook, backlink.list, notebook.not_found; the page at the path,
// page.not_found, an attachment's too; then Execute's reads, and the
// source pages' paths in one statement, a source deleted since left out.
func (l ListBacklinks) AtPath(ctx context.Context, notebookID uuid.UUID, path string, limit *int, cursor *string) (BacklinksAt, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return BacklinksAt{}, err
	}
	var after uuid.UUID
	if cursor != nil {
		if err := shared.DecodeCursor(*cursor, &after); err != nil {
			return BacklinksAt{}, err
		}
	}
	if err := l.Paths.Check("path", path); err != nil {
		return BacklinksAt{}, err
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return BacklinksAt{}, err
	}
	if err := l.Access.notebook(ctx, actor, notebookID, domain.ActionListBacklinks); err != nil {
		return BacklinksAt{}, err
	}
	id, err := l.Paths.FindPage(ctx, notebookID, "path", path)
	if err != nil {
		return BacklinksAt{}, err
	}
	b, err := l.read(ctx, id, after, size)
	if err != nil {
		return BacklinksAt{}, err
	}
	ids := make([]uuid.UUID, len(b.Pages))
	for i, p := range b.Pages {
		ids[i] = p.PageID
	}
	paths, err := l.Paths.Paths(ctx, notebookID, ids)
	if err != nil {
		return BacklinksAt{}, err
	}
	out := BacklinksAt{Pages: make([]BacklinkingAt, 0, len(b.Pages)), NextCursor: b.NextCursor}
	for _, p := range b.Pages {
		// A source put in the trash since the rows were read has no path:
		// it is left out, and a cursor after a later page goes past it.
		at, ok := paths[p.PageID]
		if !ok {
			continue
		}
		c, err := shared.EncodeCursor(p.PageID)
		if err != nil {
			return BacklinksAt{}, err
		}
		out.Pages = append(out.Pages, BacklinkingAt{Backlinking: p, Path: at, Cursor: c})
	}
	return out, nil
}

// read is the page of size of the pages that link to the page id, after
// the page after: one more is read to tell whether another follows, and
// each page's contexts are cut from its content, read one page at a time.
func (l ListBacklinks) read(ctx context.Context, id, after uuid.UUID, size int) (Backlinks, error) {
	rows, err := l.Reads.Backlinks(ctx, id, after, size+1, domain.MaxCount, domain.MaxContexts)
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
	read := 0
	for i, b := range rows {
		out.Pages[i] = Backlinking{PageID: b.SourceID, Links: b.Links, Contexts: []string{}}
		if b.Extractor != domain.Extractor || read >= domain.MaxContentRead {
			continue
		}
		content, revision, ok, err := l.Contents.Content(ctx, b.SourceID)
		if err != nil {
			return Backlinks{}, err
		}
		read += len(content)
		if ok && revision == b.Revision {
			out.Pages[i].Contexts = domain.Contexts(content, b.Ranges)
		}
	}
	return out, nil
}
