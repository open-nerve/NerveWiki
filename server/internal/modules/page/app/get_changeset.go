package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
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
	logged, err := g.r.logged(ctx, []domain.Changeset{c})
	if err != nil {
		return ChangesetDetail{}, err
	}
	moves, err := g.r.History.ChangesetMoves(ctx, c.ID)
	if err != nil {
		return ChangesetDetail{}, err
	}
	return ChangesetDetail{LoggedChangeset: logged[0], Moves: moves}, nil
}
