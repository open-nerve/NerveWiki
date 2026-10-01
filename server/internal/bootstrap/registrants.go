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
	ext := workspaceRegistrants(pool, nil)
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
// deletion, those that follow a membership's restore (M2/P3 design 3.5),
// and those that follow a membership's addition and a member's role change
// (M3 design 8).
type workspaceExtensions struct {
	endVetoers            []workspace.MembershipEndVetoer
	endSubscribers        []workspace.MembershipEndSubscriber
	deletionSubscribers   []workspace.WorkspaceDeletionSubscriber
	restoreSubscribers    []workspace.MembershipRestoreSubscriber
	additionSubscribers   []workspace.MembershipAdditionSubscriber
	roleChangeSubscribers []workspace.MemberRoleChangeSubscriber
}

// workspaceRegistrants are the modules that take part in the workspace
// module's membership ends, restores, deletions, additions and role
// changes: the notebook module follows a deletion (M3/P1), an addition and
// a role change (M3/P2), and vetoes and follows an end and follows a
// restore (M3/P3). returned, when set, counts the notebooks the restores
// return: the command line prints it; serve hands nil.
func workspaceRegistrants(pool *pgxpool.Pool, returned *int) workspaceExtensions {
	return workspaceRegistrantsWith(pool, notebookRegistrants(), returned)
}

// workspaceRegistrantsWith is workspaceRegistrants with nb, the notebook
// module's registrants, whom the notebook module's parts call in turn: a
// test hands its own.
func workspaceRegistrantsWith(pool *pgxpool.Pool, nb notebookExtensions, returned *int) workspaceExtensions {
	events := workspaceMemberEvents{notebook.NewWorkspaceMemberEvents(nb.visibilitySubscribers)}
	end := membershipEnd{notebook.NewMembershipEnd(pool, workspace.NewWorkspaces(pool), nb.visibilitySubscribers)}
	return workspaceExtensions{
		endVetoers:     []workspace.MembershipEndVetoer{end},
		endSubscribers: []workspace.MembershipEndSubscriber{end},
		deletionSubscribers: []workspace.WorkspaceDeletionSubscriber{
			workspaceDeletion{notebook.NewWorkspaceDeletion(pool, nb.deletionSubscribers)},
		},
		restoreSubscribers: []workspace.MembershipRestoreSubscriber{
			membershipRestore{notebook.NewMembershipRestore(pool, nb.visibilitySubscribers), returned},
		},
		additionSubscribers:   []workspace.MembershipAdditionSubscriber{events},
		roleChangeSubscribers: []workspace.MemberRoleChangeSubscriber{events},
	}
}

// membershipEnd is the notebook module's part in a membership end as the
// workspace module calls it: its rule two refuses an account that leaves
// itself, by leaving or by being deactivated, which the cause tells; a
// removal is not refused (M3 design 4).
type membershipEnd struct {
	notebook notebook.MembershipEnd
}

func (m membershipEnd) VetoMembershipEnd(ctx context.Context, e workspace.MembershipEnd) error {
	return m.notebook.VetoMembershipEnd(ctx, membershipEnded(e))
}

func (m membershipEnd) MembershipEnded(ctx context.Context, e workspace.MembershipEnd) error {
	return m.notebook.MembershipEnded(ctx, membershipEnded(e))
}

// membershipEnded is e as the notebook module reads it.
func membershipEnded(e workspace.MembershipEnd) notebook.WorkspaceMembershipEnd {
	return notebook.WorkspaceMembershipEnd{
		UserID: e.UserID, WorkspaceIDs: e.WorkspaceIDs, Voluntary: e.Cause == workspace.EndLeft || e.Cause == workspace.EndDeactivated,
		By: e.By, At: e.At,
	}
}

// membershipRestore is the notebook module's part in a membership restore
// as the workspace module calls it, adding the notebooks it returns to
// returned when set.
type membershipRestore struct {
	notebook notebook.MembershipRestore
	returned *int
}

func (r membershipRestore) MembershipRestored(ctx context.Context, x workspace.MembershipRestore) error {
	n, err := r.notebook.MembershipRestored(ctx, notebook.WorkspaceMembershipRestore(x))
	if err == nil && r.returned != nil {
		*r.returned += n
	}
	return err
}

// workspaceDeletion is the notebook module's part in a workspace's
// deletion as the workspace module calls it: the two modules do not
// import each other, so their values, alike field by field, meet here.
type workspaceDeletion struct {
	notebook notebook.WorkspaceDeletion
}

func (d workspaceDeletion) WorkspaceDeleted(ctx context.Context, x workspace.WorkspaceDeletion) error {
	return d.notebook.WorkspaceDeleted(ctx, notebook.WorkspaceDeleted(x))
}

// workspaceMemberEvents is the notebook module's part in a workspace's
// addition and role change, as the workspace module calls it.
type workspaceMemberEvents struct {
	notebook notebook.WorkspaceMemberEvents
}

func (e workspaceMemberEvents) MembershipAdded(ctx context.Context, a workspace.MembershipAddition) error {
	return e.notebook.MembershipAdded(ctx, notebook.WorkspaceMemberAdded(a))
}

func (e workspaceMemberEvents) MemberRoleChanged(ctx context.Context, c workspace.MemberRoleChange) error {
	return e.notebook.MemberRoleChanged(ctx, notebook.WorkspaceRoleChanged(c))
}

// notebookExtensions are the registrants of the notebook module's
// extension points (M3 design 8): those that follow a notebook's deletion,
// and those that follow a change of what accounts see.
type notebookExtensions struct {
	deletionSubscribers   []notebook.NotebookDeletionSubscriber
	visibilitySubscribers []notebook.VisibilitySubscriber
}

// notebookRegistrants are the modules that take part in a notebook's
// deletion and in a visibility change: none in M3; M4's pages and M7's
// attachments follow a deletion, M5's event streams both. The module's
// use cases and its parts in the workspace module's events all take them
// from here.
func notebookRegistrants() notebookExtensions {
	return notebookExtensions{}
}

// purgers are the modules' purgers of the soft-deleted rows, leaf to root
// (M2 design 8, M2/P4 design 3.4): a module whose tables reference
// another's comes before it, the notebooks before the workspaces. The
// database test of the purge checks the order against the foreign keys,
// and that every table with deleted_at has its purger.
func purgers(pool *pgxpool.Pool) []jobs.Purger {
	return slices.Concat(notebook.Purgers(pool), workspace.Purgers(pool))
}
