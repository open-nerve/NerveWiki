package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// RemoveMemberDeps are what RemoveMember needs.
type RemoveMemberDeps struct {
	Workspaces  Workspaces
	Finder      NotebookFinder
	Notebooks   NotebookWriter
	Members     MemberFinder
	Writer      MemberWriter
	Subscribers []VisibilitySubscriber
	Auth        shared.Authorizer
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// RemoveMember ends a notebook member's membership:
// DELETE /api/v0/notebook-members/{notebook_member_id} (M3/P2 design 3.3).
type RemoveMember struct {
	d RemoveMemberDeps
	m manager
}

// NewRemoveMember returns the use case.
func NewRemoveMember(d RemoveMemberDeps) *RemoveMember {
	return &RemoveMember{d: d, m: manager{workspaces: d.Workspaces, notebooks: d.Notebooks, auth: d.Auth}}
}

// Execute ends the membership id. Under the notebook's lock and the
// decision it reads the membership again and refuses the caller's own
// (rule one: an admin leaves instead). The visibility's subscribers are
// told of the account, even one the workspace access still lets see the
// notebook: one told too many costs nothing.
func (r *RemoveMember) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	_, n, err := findMember(ctx, r.d.Members, r.d.Finder, id)
	if err != nil {
		return err
	}
	err = r.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		m, _, err := r.m.lockMember(ctx, actor, domain.ActionRemoveMember, n, r.d.Members, id)
		if err != nil {
			return err
		}
		if m.UserID == actor.UserID {
			return domain.ErrOwnMembership
		}
		now := r.d.Clock.Now()
		if err := r.d.Writer.EndMember(ctx, m.ID, actor.UserID, now); err != nil {
			return err
		}
		return publishVisibility(ctx, r.d.Subscribers, VisibilityChange{WorkspaceID: n.WorkspaceID, UserIDs: []uuid.UUID{m.UserID}, At: now})
	})
	if err != nil {
		return err
	}
	r.d.Logger.InfoContext(ctx, "notebook member removed", slog.String("workspace_id", n.WorkspaceID.String()),
		slog.String("notebook_id", n.ID.String()), slog.String("member_id", id.String()), slog.String("user_id", actor.UserID.String()))
	return nil
}
