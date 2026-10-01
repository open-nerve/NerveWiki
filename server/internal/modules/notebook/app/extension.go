package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The module's extension points (M3 design 8), and its registrants of the
// workspace module's deletion, addition and role change. The registrants
// are built from the pool alone and composed in bootstrap's
// registrants.go; their statements reach the transaction through the
// context.

// NotebookDeletion is notebooks being deleted: one by deleteNotebook, every
// one of a workspace by the workspace's deletion. A subscriber deletes its
// own rows at At, so the cleanup removes them with the notebooks.
type NotebookDeletion struct {
	WorkspaceID uuid.UUID
	NotebookIDs []uuid.UUID
	By          uuid.UUID
	At          time.Time
}

// NotebookDeletionSubscriber follows a deletion, after the notebooks and
// their members were deleted and in their transaction: an error rolls
// everything back. A deletion has no vetoer: it is an admin's explicit,
// destructive act.
type NotebookDeletionSubscriber interface {
	NotebookDeleted(ctx context.Context, d NotebookDeletion) error
}

// publishDeletion calls the subscribers in order; the first error stops it.
func publishDeletion(ctx context.Context, subscribers []NotebookDeletionSubscriber, d NotebookDeletion) error {
	for _, s := range subscribers {
		if err := s.NotebookDeleted(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

// WorkspaceDeleted is a workspace being deleted, field by field as the
// workspace module's deletion tells it: bootstrap converts
// workspace.WorkspaceDeletion.
type WorkspaceDeleted struct {
	WorkspaceID uuid.UUID
	By          uuid.UUID
	At          time.Time
}

// WorkspaceDeletion is the module's registrant of the workspace module's
// deletion (M3/P1 design 3.8): it deletes the workspace's notebooks, their
// members and its audit events (M3/P3 design 3.4) at the deletion's time,
// and tells the notebook deletion's subscribers once, with every id. It runs in the deletion's transaction,
// which holds the workspace's row FOR NO KEY UPDATE: no notebook
// management write of the workspace runs beside it, as each takes the row
// FOR SHARE; the notebooks are locked by id, the order of every write that
// holds more than one.
type WorkspaceDeletion struct {
	Notebooks   NotebooksDeleter
	Subscribers []NotebookDeletionSubscriber
}

// WorkspaceDeleted follows the deletion of d.WorkspaceID.
func (w WorkspaceDeletion) WorkspaceDeleted(ctx context.Context, d WorkspaceDeleted) error {
	ids, err := w.Notebooks.DeleteNotebooksOf(ctx, d.WorkspaceID, d.By, d.At)
	if err != nil {
		return err
	}
	// A workspace whose notebooks are all gone may still have events.
	if err := w.Notebooks.DeleteAuditEventsOf(ctx, d.WorkspaceID, d.By, d.At); err != nil || len(ids) == 0 {
		return err
	}
	return publishDeletion(ctx, w.Subscribers, NotebookDeletion{WorkspaceID: d.WorkspaceID, NotebookIDs: ids, By: d.By, At: d.At})
}

// VisibilityChange is accounts whose notebooks seen in a workspace may have
// changed (M3 design 4, 8): those of UserIDs, and, when Reached, every
// active admin and member of the workspace as the writing transaction sees
// them, whom a workspace access crossing none reaches. A deletion is told
// by NotebookDeletion instead.
type VisibilityChange struct {
	WorkspaceID uuid.UUID
	UserIDs     []uuid.UUID
	Reached     bool
	At          time.Time
}

// VisibilitySubscriber follows a visibility change, after the write that
// caused it and in its transaction: an error rolls everything back. M5
// closes the accounts' event streams.
type VisibilitySubscriber interface {
	VisibilityChanged(ctx context.Context, v VisibilityChange) error
}

// publishVisibility calls the subscribers in order; the first error stops
// it.
func publishVisibility(ctx context.Context, subscribers []VisibilitySubscriber, v VisibilityChange) error {
	for _, s := range subscribers {
		if err := s.VisibilityChanged(ctx, v); err != nil {
			return err
		}
	}
	return nil
}

// WorkspaceMemberAdded is a workspace's new membership, field by field as
// the workspace module's addition tells it: bootstrap converts
// workspace.MembershipAddition.
type WorkspaceMemberAdded struct {
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	Role        shared.WorkspaceRole
	By          uuid.UUID
	At          time.Time
}

// WorkspaceRoleChanged is a workspace member's role changed, field by field
// as the workspace module's role change tells it: bootstrap converts
// workspace.MemberRoleChange.
type WorkspaceRoleChanged struct {
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	From, To    shared.WorkspaceRole
	By          uuid.UUID
	At          time.Time
}

// WorkspaceMemberEvents is the module's registrant of the workspace
// module's addition and role change (M3/P2 design 3.6): it tells the
// visibility's subscribers of an account the change gave or took a default
// role, which a workspace access gives the workspace's admins and members,
// not its guests. It reads nothing.
type WorkspaceMemberEvents struct {
	Subscribers []VisibilitySubscriber
}

// MembershipAdded follows an addition: a new admin or member is reached by
// the open notebooks, a new guest by none.
func (w WorkspaceMemberEvents) MembershipAdded(ctx context.Context, a WorkspaceMemberAdded) error {
	if !shared.ReachedByAccess(a.Role) {
		return nil
	}
	return publishVisibility(ctx, w.Subscribers, VisibilityChange{WorkspaceID: a.WorkspaceID, UserIDs: []uuid.UUID{a.UserID}, At: a.At})
}

// MemberRoleChanged follows a role change that crosses guest: between an
// admin and a member, the open notebooks reach the account alike.
func (w WorkspaceMemberEvents) MemberRoleChanged(ctx context.Context, c WorkspaceRoleChanged) error {
	if shared.ReachedByAccess(c.From) == shared.ReachedByAccess(c.To) {
		return nil
	}
	return publishVisibility(ctx, w.Subscribers, VisibilityChange{WorkspaceID: c.WorkspaceID, UserIDs: []uuid.UUID{c.UserID}, At: c.At})
}
