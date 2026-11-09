package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetUpgrade reads a notebook's upgrade under way, for its admins:
// GET /api/v0/notebooks/{notebook_id}/upgrade (M11/P1 design 3.14).
type GetUpgrade struct{ d UpgradeDeps }

// NewGetUpgrade returns the use case.
func NewGetUpgrade(d UpgradeDeps) *GetUpgrade { return &GetUpgrade{d: d} }

// Execute answers notebook id's upgrade under way: notebook.not_found for
// no notebook or one the caller cannot see, forbidden for one they are no
// admin of, wiki.upgrade_not_found for none under way. It takes no lock
// and no snapshot: each of its fields is of one row.
func (g *GetUpgrade) Execute(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return UpgradeView{}, err
	}
	workspace, ok, err := g.d.Notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return UpgradeView{}, err
	case !ok:
		return UpgradeView{}, domain.ErrNotebookNotFound
	}
	_, err = g.d.Authorizer.Authorize(ctx, actor, g.d.Action, shared.Target{WorkspaceID: workspace, NotebookID: id})
	if errors.Is(err, shared.ErrNotVisible) {
		return UpgradeView{}, domain.ErrNotebookNotFound
	}
	if err != nil {
		return UpgradeView{}, err
	}
	up, ok, err := g.d.Rows.Current(ctx, id)
	switch {
	case err != nil:
		return UpgradeView{}, err
	case !ok:
		return UpgradeView{}, domain.ErrUpgradeNotFound
	}
	ids := []uuid.UUID{up.CreatedBy}
	if up.SubmittedBy != nil {
		ids = append(ids, *up.SubmittedBy)
	}
	names, err := g.d.Names.DisplayNames(ctx, ids)
	if err != nil {
		return UpgradeView{}, err
	}
	view := UpgradeView{Upgrade: up, StartedByName: names[up.CreatedBy]}
	if up.SubmittedBy != nil {
		view.SubmittedByName = names[*up.SubmittedBy]
	}
	return view, nil
}
