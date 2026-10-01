package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LeaveNotebookDeps are what LeaveNotebook needs.
type LeaveNotebookDeps struct {
	Workspaces  Workspaces
	Finder      NotebookFinder
	Notebooks   NotebookWriter
	Writer      MemberWriter
	Subscribers []VisibilitySubscriber
	Auth        shared.Authorizer
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// LeaveNotebook ends the caller's membership of a notebook:
// POST /api/v0/notebooks/{notebook_id}/leave (M3/P2 design 3.3).
type LeaveNotebook struct {
	d LeaveNotebookDeps
	m manager
}

// NewLeaveNotebook returns the use case.
func NewLeaveNotebook(d LeaveNotebookDeps) *LeaveNotebook {
	return &LeaveNotebook{d: d, m: manager{workspaces: d.Workspaces, notebooks: d.Notebooks, auth: d.Auth}}
}

// Execute ends the caller's membership of notebook id. Any role in it may
// leave; one that uses it by its workspace access alone has no membership
// to end (notebook.member_not_found). Its only admin cannot, even alone
// (rule one): the admins are counted under the notebook's lock, so two
// admins leaving at once leave one. The visibility's subscribers are told
// of the caller.
func (l *LeaveNotebook) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	n, err := find(ctx, l.d.Finder, id)
	if err != nil {
		return err
	}
	err = l.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, _, err := l.m.lock(ctx, actor, domain.ActionLeave, n, domain.ErrNotFound); err != nil {
			return err
		}
		m, err := l.d.Writer.FindMemberOf(ctx, n.ID, actor.UserID)
		switch {
		case errors.Is(err, ErrNotFound) || err == nil && !m.Active():
			return domain.ErrMemberNotFound
		case err != nil:
			return err
		}
		admins, err := l.d.Writer.CountAdmins(ctx, n.ID)
		if err != nil {
			return err
		}
		if err := domain.CheckLeave(m, admins); err != nil {
			return err
		}
		now := l.d.Clock.Now()
		if err := l.d.Writer.EndMember(ctx, m.ID, actor.UserID, now); err != nil {
			return err
		}
		return publishVisibility(ctx, l.d.Subscribers, VisibilityChange{WorkspaceID: n.WorkspaceID, UserIDs: []uuid.UUID{actor.UserID}, At: now})
	})
	if err != nil {
		return err
	}
	l.d.Logger.InfoContext(ctx, "notebook left", slog.String("workspace_id", n.WorkspaceID.String()),
		slog.String("notebook_id", n.ID.String()), slog.String("user_id", actor.UserID.String()))
	return nil
}
