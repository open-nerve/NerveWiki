package bootstrap

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// deactivationRegistrants are the modules that take part in an account's
// deactivation: the vetoers that may refuse it and the subscribers that
// follow it (M1 design 8). serve and the command line both take them from
// here, so that a registrant a module adds reaches the self-service
// deactivation and the administrator's alike (M1/P4 design 3.6). M2's is
// the workspace module's: rule two, and the end of the account's
// memberships through the membership end's registrants (M2/P4 design 3.1).
func deactivationRegistrants(pool *pgxpool.Pool) ([]identity.DeactivationVetoer, []identity.DeactivationSubscriber) {
	ext := workspaceRegistrants()
	ws := deactivation{workspace.NewDeactivation(pool, ext.endVetoers, ext.endSubscribers)}
	return []identity.DeactivationVetoer{ws}, []identity.DeactivationSubscriber{ws}
}

// deactivation is the workspace module's part in a deactivation as
// identity calls it: the two modules do not import each other, so their
// values, alike field by field, meet here.
type deactivation struct {
	workspace workspace.Deactivation
}

func (d deactivation) VetoDeactivation(ctx context.Context, x identity.Deactivation) error {
	return d.workspace.VetoDeactivation(ctx, workspace.Deactivated(x))
}

func (d deactivation) AccountDeactivated(ctx context.Context, x identity.Deactivation) error {
	return d.workspace.AccountDeactivated(ctx, workspace.Deactivated(x))
}

// workspaceExtensions are the registrants of the workspace module's
// extension points (M2 design 8): the vetoers that may refuse a membership
// end, the subscribers that follow one, those that follow a workspace's
// deletion, and those that follow a membership's restore (M2/P3 design
// 3.5).
type workspaceExtensions struct {
	endVetoers          []workspace.MembershipEndVetoer
	endSubscribers      []workspace.MembershipEndSubscriber
	deletionSubscribers []workspace.WorkspaceDeletionSubscriber
	restoreSubscribers  []workspace.MembershipRestoreSubscriber
}

// workspaceRegistrants are the modules that take part in the workspace
// module's membership ends, restores and deletions. M2 has none: the
// notebooks' come with M3.
func workspaceRegistrants() workspaceExtensions {
	return workspaceExtensions{}
}

// purgers are the modules' purgers of the soft-deleted rows, leaf to root
// (M2 design 8, M2/P4 design 3.4): a module whose tables reference
// another's comes before it, the notebooks before the workspaces. The
// database test of the purge checks the order against the foreign keys,
// and that every table with deleted_at has its purger.
func purgers(pool *pgxpool.Pool) []jobs.Purger {
	return slices.Concat(notebook.Purgers(pool), workspace.Purgers(pool))
}
