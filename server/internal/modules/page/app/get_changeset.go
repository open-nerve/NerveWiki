package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ChangesetDetail is a changeset as the log lists it, and whether its
// first items rename or move a node.
type ChangesetDetail struct {
	LoggedChangeset
	Moves domain.ChangesetMoves
}

// GetChangeset reads a changeset: GET /api/v0/changesets/{changeset_id}
// (M8/P3 design 3.4), which a job's "see it in the history" and a revert's
// "reverted …" read.
type GetChangeset struct {
	r HistoryReads
}

// NewGetChangeset returns the use case.
func NewGetChangeset(r HistoryReads) *GetChangeset {
	return &GetChangeset{r: r}
}

// Execute reads the changeset id as the log lists it, with whether its
// first items rename or move a node, which the log's lines do not read: a
// changeset that does not exist, whose notebook is deleted, or whose
// notebook the caller has no role in (page.history: any role) is
// page.changeset_not_found. A read takes no lock and opens no transaction.
func (g *GetChangeset) Execute(ctx context.Context, id uuid.UUID) (ChangesetDetail, error) {
	c, err := g.r.readableChangeset(ctx, id)
	if err != nil {
		return ChangesetDetail{}, err
	}
	return g.r.detail(ctx, c)
}

// ChangesetPage is changeset's answer (M9 design 4.9): the changeset as
// GetChangeset reads it, on the first page only (nil with a cursor); a
// page of its changes by node id, each with the cursor that goes on after
// it; the paths of their nodes alive now, by node id; the next page's
// cursor; and Start, the cursor of the first page's changes, for a first
// page whose changeset leaves its first change no room.
type ChangesetPage struct {
	Detail     *ChangesetDetail
	Changes    []domain.ChangeRow
	Cursors    []string
	Paths      map[uuid.UUID]string
	NextCursor string
	Start      string
}

// InNotebook reads the changeset id of the notebook notebookID (M9/P2
// design 3.16.3): a changeset of another notebook, one the caller could
// read there among them, is page.changeset_not_found, as the entry is the
// notebook's. Its order: the actor; the cursor, 400; the limit, 422; the
// notebook, notebook.not_found (page.history, which every role may: no
// 403); the changeset, page.changeset_not_found, the zero id unread; then
// GetChangeset's reads without a cursor, ListChanges' reads, and the
// paths of the nodes of the page's changes alive now, read once. A read
// takes no lock and opens no transaction.
func (g *GetChangeset) InNotebook(ctx context.Context, notebookID, id uuid.UUID, limit *int, cursor *string) (ChangesetPage, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return ChangesetPage{}, err
	}
	var after domain.ChangeCursor
	if cursor != nil {
		if err := shared.DecodeCursor(*cursor, &after); err != nil {
			return ChangesetPage{}, err
		}
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return ChangesetPage{}, err
	}
	if err := decideNotebook(ctx, g.r.Notebooks, g.r.Auth, actor, domain.ActionHistory, notebookID); err != nil {
		return ChangesetPage{}, err
	}
	if id == (uuid.UUID{}) {
		return ChangesetPage{}, domain.ErrChangesetNotFound
	}
	c, err := g.r.History.FindChangeset(ctx, id)
	switch {
	case err != nil:
		return ChangesetPage{}, found(err, domain.ErrChangesetNotFound)
	case c.NotebookID != notebookID:
		return ChangesetPage{}, domain.ErrChangesetNotFound
	}
	var out ChangesetPage
	if cursor == nil {
		d, err := g.r.detail(ctx, c)
		if err != nil {
			return ChangesetPage{}, err
		}
		out.Detail = &d
	}
	l, err := g.r.changes(ctx, c.ID, after.After, size)
	if err != nil {
		return ChangesetPage{}, err
	}
	out.Changes, out.NextCursor = l.Changes, l.NextCursor
	if out.Cursors, err = cursorsAfter(l.Changes, func(r domain.ChangeRow) any { return domain.ChangeCursor{After: r.NodeID} }); err != nil {
		return ChangesetPage{}, err
	}
	if out.Paths, err = g.r.alivePaths(ctx, notebookID, l.Changes); err != nil {
		return ChangesetPage{}, err
	}
	if out.Start, err = shared.EncodeCursor(domain.ChangeCursor{}); err != nil { // after no id: from the first
		return ChangesetPage{}, err
	}
	return out, nil
}
