package app

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The module's extension points (M3 design 8), and its registrants of the
// workspace module's deletion, addition and role change. The registrants
// are built from the pool alone and composed in bootstrap's
// registrants.go; their statements reach the transaction through the
// context.

// NotebookCreation is a notebook being created (M10 design 8): its id and
// workspace, its mode, its source root (the zero id for a standard one),
// its template's language, who created it, from where, at the creation's
// one time.
type NotebookCreation struct {
	NotebookID   uuid.UUID
	WorkspaceID  uuid.UUID
	Mode         domain.Mode
	SourceRootID uuid.UUID
	Locale       domain.Locale
	By           uuid.UUID
	Client       string
	At           time.Time
}

// Seeder writes what a notebook is created with, after the notebook and its
// admin were inserted and before the visibility's event, in the
// creation's transaction: it answers the source root it wrote, the zero id
// for none, and any error rolls the creation back. Exactly one seeder
// answers an LLM Wiki's source root, the one the creation decided; none
// answers one for a standard notebook. M10's is the wiki module's template.
type Seeder interface {
	NotebookCreated(ctx context.Context, c NotebookCreation) (uuid.UUID, error)
}

// seed calls the seeders in order, the first error stopping it, and
// answers the source roots they wrote, the zero ids left out. A seeder's
// problem (a *shared.Error, a guard's refusal among them) is a defect: the
// template is the server's own, and createNotebook declares no code of
// another module's (M10/P1 design 3.4). It is answered as an error
// without a kind, the problem's code in its text, which the 500's log
// carries; any other error passes as it is, ctx's cancellation and
// deadline among them (no *shared.Error wraps one).
func seed(ctx context.Context, seeders []Seeder, c NotebookCreation) ([]uuid.UUID, error) {
	var roots []uuid.UUID
	for i, s := range seeders {
		root, err := s.NotebookCreated(ctx, c)
		var problem *shared.Error
		switch {
		case err == nil:
		case errors.As(err, &problem):
			return nil, fmt.Errorf("notebook: seeder %d refused notebook %s with %s, a defect: the template is the server's own",
				i, c.NotebookID, problem.Code)
		default:
			return nil, err
		}
		if root != (uuid.UUID{}) {
			roots = append(roots, root)
		}
	}
	return roots, nil
}

// ModeChange is a notebook's mode changing (M10 design 8): from and to,
// who changed it, at the change's one time.
type ModeChange struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	From, To    domain.Mode
	By          uuid.UUID
	At          time.Time
}

// ModeSubscriber follows a mode change, after the notebook's row was
// written and in its transaction: an error rolls everything back. M10's is
// the event stream's mode; M11's upgrade changes the mode through it too.
type ModeSubscriber interface {
	ModeChanged(ctx context.Context, m ModeChange) error
}

// publishMode calls the subscribers in order; the first error stops it.
func publishMode(ctx context.Context, subscribers []ModeSubscriber, m ModeChange) error {
	for _, s := range subscribers {
		if err := s.ModeChanged(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

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
// and tells the notebook deletion's subscribers once, with every id. It
// runs in the deletion's transaction, which holds the workspace's row FOR
// NO KEY UPDATE: no notebook management write of the workspace runs beside
// it, as each takes the row FOR SHARE; the notebooks are locked by id, the
// order of every write that holds more than one.
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

// NotebookActivity is what a module holds of a notebook (M3 design 4, 8):
// its bytes, and when it last wrote them, nil if it never did.
type NotebookActivity struct {
	Bytes       int64
	LastWriteAt *time.Time
}

// NotebookActivitySource tells the activity of notebooks: M4's pages and
// M7's attachments register one; M3 has none. It runs in its caller's
// read, unlocked, and answers the notebooks it holds nothing of with
// nothing.
type NotebookActivitySource interface {
	NotebookActivities(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]NotebookActivity, error)
}

// activityOf is the notebooks' activity across sources: the bytes summed,
// the latest write, which updated, a notebook's own row's last update,
// starts from.
func activityOf(ctx context.Context, sources []NotebookActivitySource, updated map[uuid.UUID]time.Time,
) (map[uuid.UUID]NotebookActivity, error) {
	out := make(map[uuid.UUID]NotebookActivity, len(updated))
	ids := make([]uuid.UUID, 0, len(updated))
	for id, at := range updated {
		out[id] = NotebookActivity{LastWriteAt: &at}
		ids = append(ids, id)
	}
	for _, s := range sources {
		got, err := s.NotebookActivities(ctx, ids)
		if err != nil {
			return nil, err
		}
		for id, a := range got {
			sum, ok := out[id]
			if !ok {
				continue
			}
			sum.Bytes += a.Bytes
			if a.LastWriteAt != nil && a.LastWriteAt.After(*sum.LastWriteAt) {
				sum.LastWriteAt = a.LastWriteAt
			}
			out[id] = sum
		}
	}
	return out, nil
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
