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
// admin of, wiki.upgrade_not_found for none under way. One rolling back
// whose job is neither queued nor running tells what its rollback has
// left, read now (M11/P1 design 3.20). It takes no lock; the upgrade's
// row, whether a job writes and what is left are read in one snapshot,
// so that a rollback's end committed between them is in all three or in
// none: never an upgrade rolling back whose baseline is gone, every node
// alive told as created (M11/P1 S3 review A-M2).
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
	var up domain.Upgrade
	if err := g.d.Snapshots.WithinSnapshot(ctx, func(ctx context.Context) error {
		var ok bool
		var err error
		up, ok, err = g.d.Rows.Current(ctx, id)
		switch {
		case err != nil:
			return err
		case !ok:
			return domain.ErrUpgradeNotFound
		case up.State != domain.UpgradeRollingBack:
			return nil
		}
		writing, err := g.d.Writing.Writing(ctx, id)
		if err != nil || writing {
			return err
		}
		left, err := g.d.Rollback.Left(ctx, up.ID, id)
		if err != nil {
			return err
		}
		up.RollbackLeft = &left
		return nil
	}); err != nil {
		return UpgradeView{}, err
	}
	return g.d.named(ctx, up)
}
