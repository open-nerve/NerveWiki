package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// UpdateMemberDeps are what UpdateMember needs.
type UpdateMemberDeps struct {
	Locker   WorkspaceLocker
	Finder   MemberFinder
	Members  MemberUpdater
	Profiles MemberProfiles
	Auth     shared.Authorizer
	Tx       shared.TxManager
	Clock    Clock
	Logger   *slog.Logger
}

// UpdateMember changes a member's role:
// PATCH /api/v0/workspace-members/{workspace_member_id} (M2/P2 design 3.2).
type UpdateMember struct {
	d UpdateMemberDeps
}

// NewUpdateMember returns the use case.
func NewUpdateMember(d UpdateMemberDeps) *UpdateMember {
	return &UpdateMember{d: d}
}

// Execute gives the membership id the role, and returns it with the
// account's profile. The membership names its workspace, whose row it
// locks before it reads the membership again, decides, checks the role
// and refuses the caller's own. Since the caller is an admin who cannot
// change their own role, the workspace keeps an admin.
func (u *UpdateMember) Execute(ctx context.Context, id uuid.UUID, role string) (ListedMember, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return ListedMember{}, err
	}
	m, err := u.d.Finder.FindActiveMember(ctx, id)
	if err != nil {
		return ListedMember{}, found(err, domain.ErrMemberNotFound)
	}
	var out ListedMember
	err = u.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if m, err = lockMember(ctx, u.d.Locker, u.d.Finder, m); err != nil {
			return err
		}
		if _, err := authorize(ctx, u.d.Auth, actor, domain.ActionUpdateMember, m.WorkspaceID, domain.ErrMemberNotFound); err != nil {
			return err
		}
		r, err := domain.CheckRole(role)
		if err != nil {
			return err
		}
		if m.UserID == actor.UserID {
			return domain.ErrOwnMembership
		}
		if err := u.d.Members.UpdateMemberRole(ctx, m.ID, r, actor.UserID, u.d.Clock.Now()); err != nil {
			return err
		}
		m.Role = r
		// The answer's profile is read here, after the write: nothing that
		// can fail comes after the commit (v0.1 design 13.1, item 19). The
		// caller is an admin, who sees emails.
		list, err := withProfiles(ctx, u.d.Profiles, []domain.Member{m}, true)
		if err != nil {
			return err
		}
		out = list[0]
		return nil
	})
	if err != nil {
		return ListedMember{}, err
	}
	u.d.Logger.InfoContext(ctx, "workspace member updated", slog.String("workspace_id", m.WorkspaceID.String()),
		slog.String("member_id", m.ID.String()), slog.String("user_id", actor.UserID.String()))
	return out, nil
}
