package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// TakeOverNotebookDeps are what TakeOverNotebook needs.
type TakeOverNotebookDeps struct {
	Workspaces  Workspaces
	Finder      NotebookFinder
	Notebooks   NotebookWriter
	Writer      MemberWriter
	Ownerless   OwnerlessWriter
	Audit       AuditRecorder
	Subscribers []VisibilitySubscriber
	Auth        shared.Authorizer
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// TakeOverNotebook makes a workspace admin the admin of an ownerless
// notebook: POST /api/v0/ownerless-notebooks/{notebook_id}/take-over
// (M3/P3 design 3.3).
type TakeOverNotebook struct {
	d TakeOverNotebookDeps
}

// NewTakeOverNotebook returns the use case.
func NewTakeOverNotebook(d TakeOverNotebookDeps) *TakeOverNotebook {
	return &TakeOverNotebook{d: d}
}

// Execute makes the caller the admin of ownerless notebook id, and returns
// the notebook. Under the locks and the decision, the caller's membership
// is added, restored or raised to admin; the notebook is owned again, its
// workspace access as it was; the take-over is recorded, and the
// visibility's subscribers are told of the caller. The workspace's row,
// held FOR SHARE, keeps the caller an active admin of the workspace until
// it commits, as an addition's does.
func (t *TakeOverNotebook) Execute(ctx context.Context, id uuid.UUID) (View, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return View{}, err
	}
	n, err := find(ctx, t.d.Finder, id)
	if err != nil {
		return View{}, err
	}
	var out View
	err = t.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		n, err := lockOwnerless(ctx, t.d.Workspaces, t.d.Notebooks, t.d.Auth, actor, domain.ActionTakeOver, n)
		if err != nil {
			return err
		}
		now := t.d.Clock.Now()
		m, err := t.d.Writer.FindMemberOf(ctx, n.ID, actor.UserID)
		switch {
		case errors.Is(err, ErrNotFound):
			m = domain.Member{ID: uuid.NewV7(), NotebookID: n.ID, UserID: actor.UserID, Role: shared.NotebookAdmin, CreatedAt: now}
			err = t.d.Writer.AddMember(ctx, m, actor.UserID)
		case err != nil:
			return err
		case m.Active():
			err = t.d.Writer.UpdateMemberRole(ctx, m.ID, shared.NotebookAdmin, actor.UserID, now)
		default:
			err = t.d.Writer.RestoreMember(ctx, m.ID, shared.NotebookAdmin, actor.UserID, now)
		}
		if err != nil {
			return err
		}
		if err := t.d.Ownerless.ClearOwnerless(ctx, []uuid.UUID{n.ID}); err != nil {
			return err
		}
		e := domain.AuditEvent{
			ID: uuid.NewV7(), WorkspaceID: n.WorkspaceID, NotebookID: n.ID, NotebookName: n.Name, Action: domain.AuditTakenOver,
			FormerOwnerID: n.Ownerless.FormerOwner, ActorID: actor.UserID, At: now,
		}
		if err := t.d.Audit.AddAuditEvent(ctx, e); err != nil {
			return err
		}
		v := VisibilityChange{WorkspaceID: n.WorkspaceID, UserIDs: []uuid.UUID{actor.UserID}, At: now}
		if err := publishVisibility(ctx, t.d.Subscribers, v); err != nil {
			return err
		}
		count, err := t.d.Notebooks.CountMembers(ctx, n.ID)
		if err != nil {
			return err
		}
		n.Ownerless = nil
		out = View{Notebook: n, Role: shared.NotebookAdmin, MemberCount: count}
		return nil
	})
	if err != nil {
		return View{}, err
	}
	t.d.Logger.InfoContext(ctx, "ownerless notebook taken over", slog.String("workspace_id", n.WorkspaceID.String()),
		slog.String("notebook_id", id.String()), slog.String("user_id", actor.UserID.String()))
	return out, nil
}
