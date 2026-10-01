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
		return nil
	})
	if err != nil {
		return ListedMember{}, err
	}
	u.d.Logger.InfoContext(ctx, "workspace member updated", slog.String("workspace_id", m.WorkspaceID.String()),
		slog.String("member_id", m.ID.String()), slog.String("user_id", actor.UserID.String()))
	// The caller is an admin, who sees emails.
	list, err := withProfiles(ctx, u.d.Profiles, []domain.Member{m}, true)
	if err != nil {
		return ListedMember{}, err
	}
	return list[0], nil
}

// lockMember locks the workspace of m, a membership read before the
// transaction, and reads m again under the lock, where it can no longer
// change: domain.ErrMemberNotFound when the workspace was deleted or the
// membership ended meanwhile.
func lockMember(ctx context.Context, locker WorkspaceLocker, finder MemberFinder, m domain.Member) (domain.Member, error) {
	if _, err := locker.LockWorkspaceByID(ctx, m.WorkspaceID); err != nil {
		return domain.Member{}, found(err, domain.ErrMemberNotFound)
	}
	m, err := finder.FindActiveMember(ctx, m.ID)
	if err != nil {
		return domain.Member{}, found(err, domain.ErrMemberNotFound)
	}
	return m, nil
}
