package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// AddMemberDeps are what AddMember needs.
type AddMemberDeps struct {
	Workspaces       Workspaces
	WorkspaceMembers WorkspaceMembers
	Finder           NotebookFinder
	Notebooks        NotebookWriter
	Writer           MemberWriter
	Profiles         MemberProfiles
	Subscribers      []VisibilitySubscriber
	Auth             shared.Authorizer
	Tx               shared.TxManager
	Clock            Clock
	Logger           *slog.Logger
}

// AddMember gives an account a role in a notebook:
// POST /api/v0/notebooks/{notebook_id}/members (M3/P2 design 3.3).
type AddMember struct {
	d AddMemberDeps
	m manager
}

// NewAddMember returns the use case.
func NewAddMember(d AddMemberDeps) *AddMember {
	return &AddMember{d: d, m: manager{workspaces: d.Workspaces, notebooks: d.Notebooks, auth: d.Auth}}
}

// Execute makes userID a member of notebook id with role, and returns the
// membership with the account's profile. Under the locks and the decision
// it checks the values: the account an active member of the workspace,
// read under the workspace's row, which every change of its memberships
// waits for; not a member of the notebook already. A membership that
// ended is restored, keeping when the account first joined; else one is
// added. The visibility's subscribers are told of the account.
func (a *AddMember) Execute(ctx context.Context, id, userID uuid.UUID, role string) (ListedMember, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return ListedMember{}, err
	}
	n, err := find(ctx, a.d.Finder, id)
	if err != nil {
		return ListedMember{}, err
	}
	var out ListedMember
	err = a.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		n, grant, err := a.m.lock(ctx, actor, domain.ActionAddMember, n, domain.ErrNotFound)
		if err != nil {
			return err
		}
		_, inWorkspace, err := a.d.WorkspaceMembers.RoleOf(ctx, n.WorkspaceID, userID)
		if err != nil {
			return err
		}
		var existing *domain.Member
		switch m, err := a.d.Writer.FindMemberOf(ctx, n.ID, userID); {
		case err == nil:
			existing = &m
		case !errors.Is(err, ErrNotFound):
			return err
		}
		r, err := domain.CheckAddition(role, inWorkspace, existing)
		if err != nil {
			return err
		}
		now := a.d.Clock.Now()
		m := domain.Member{ID: uuid.NewV7(), NotebookID: n.ID, UserID: userID, Role: r, CreatedAt: now}
		if existing != nil {
			m.ID, m.CreatedAt = existing.ID, existing.CreatedAt
			err = a.d.Writer.RestoreMember(ctx, m.ID, r, actor.UserID, now)
		} else {
			err = a.d.Writer.AddMember(ctx, m, actor.UserID)
		}
		if err != nil {
			return err
		}
		v := VisibilityChange{WorkspaceID: n.WorkspaceID, UserIDs: []uuid.UUID{userID}, At: now}
		if err := publishVisibility(ctx, a.d.Subscribers, v); err != nil {
			return err
		}
		// The profile is read here, after the write: nothing that can fail
		// comes after the commit (v0.1 design 13.1, item 19).
		list, err := withProfiles(ctx, a.d.Profiles, []domain.Member{m}, seesEmails(grant.WorkspaceRole))
		if err != nil {
			return err
		}
		out = list[0]
		return nil
	})
	if err != nil {
		return ListedMember{}, err
	}
	a.d.Logger.InfoContext(ctx, "notebook member added", slog.String("workspace_id", n.WorkspaceID.String()), slog.String("notebook_id", n.ID.String()),
		slog.String("member_id", out.ID.String()), slog.String("user_id", actor.UserID.String()))
	return out, nil
}
