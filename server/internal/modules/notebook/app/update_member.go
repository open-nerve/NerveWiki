package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// UpdateMemberDeps are what UpdateMember needs.
type UpdateMemberDeps struct {
	Workspaces Workspaces
	Finder     NotebookFinder
	Notebooks  NotebookWriter
	Members    MemberFinder
	Writer     MemberWriter
	Profiles   MemberProfiles
	Auth       shared.Authorizer
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
}

// UpdateMember changes a notebook member's role:
// PATCH /api/v0/notebook-members/{notebook_member_id} (M3/P2 design 3.3).
type UpdateMember struct {
	d UpdateMemberDeps
	m manager
}

// NewUpdateMember returns the use case.
func NewUpdateMember(d UpdateMemberDeps) *UpdateMember {
	return &UpdateMember{d: d, m: manager{workspaces: d.Workspaces, notebooks: d.Notebooks, auth: d.Auth}}
}

// Execute gives the membership id the role, and returns it with the
// account's profile. Under the notebook's lock and the decision it reads
// the membership again, checks the role and refuses the caller's own
// (rule one): since the caller is an admin who cannot change their own
// role, the notebook keeps an admin. A role in the notebook changes no
// one's visibility.
func (u *UpdateMember) Execute(ctx context.Context, id uuid.UUID, role string) (ListedMember, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return ListedMember{}, err
	}
	_, n, err := findMember(ctx, u.d.Members, u.d.Finder, id)
	if err != nil {
		return ListedMember{}, err
	}
	var out ListedMember
	err = u.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		m, grant, err := u.m.lockMember(ctx, actor, domain.ActionUpdateMember, n, u.d.Members, id)
		if err != nil {
			return err
		}
		r, err := domain.CheckRole(role)
		if err != nil {
			return err
		}
		if m.UserID == actor.UserID {
			return domain.ErrOwnMembership
		}
		if err := u.d.Writer.UpdateMemberRole(ctx, m.ID, r, actor.UserID, u.d.Clock.Now()); err != nil {
			return err
		}
		m.Role = r
		list, err := withProfiles(ctx, u.d.Profiles, []domain.Member{m}, seesEmails(grant.WorkspaceRole))
		if err != nil {
			return err
		}
		out = list[0]
		return nil
	})
	if err != nil {
		return ListedMember{}, err
	}
	u.d.Logger.InfoContext(ctx, "notebook member updated", slog.String("workspace_id", n.WorkspaceID.String()),
		slog.String("notebook_id", n.ID.String()), slog.String("member_id", id.String()), slog.String("user_id", actor.UserID.String()))
	return out, nil
}
