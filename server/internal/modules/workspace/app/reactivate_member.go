package app

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ReactivateMemberDeps are what ReactivateMember needs.
type ReactivateMemberDeps struct {
	Accounts    AccountsByEmail
	Locker      WorkspaceLocker
	Members     MemberFinder
	Updater     MemberUpdater
	Subscribers []MembershipRestoreSubscriber
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// ReactivateMember makes an account's ended membership active again, for
// the server's administrator: nervewiki workspaces reactivate-member (M2/P4
// design 3.3), after users activate. Any ended membership: a deactivation's
// end, or a removal or a leaving; the result tells when it ended.
type ReactivateMember struct {
	d ReactivateMemberDeps
}

// NewReactivateMember returns the use case.
func NewReactivateMember(d ReactivateMemberDeps) *ReactivateMember {
	return &ReactivateMember{d: d}
}

// Reactivated is the membership the command reactivated: its workspace's
// slug, its role, and when it had ended; or, Already, the active one it
// found, and nothing was done.
type Reactivated struct {
	Slug    string
	Role    shared.WorkspaceRole
	EndedAt time.Time // zero when Already
	Already bool
}

// Execute reactivates the membership of the account of email in the
// workspace of slug. The transaction shares the account's row first, as
// every path that gives an account access does (v0.1 design 13.1, item
// 18), then locks the workspace and reads the membership under the lock.
// The ended row comes back with its role and when it first joined, and the
// restore's subscribers follow. The account itself is recorded as the one
// who restored it: the command line has no account of its own.
func (r *ReactivateMember) Execute(ctx context.Context, slug, email string) (Reactivated, error) {
	if !domain.ValidSlug(slug) {
		return Reactivated{}, domain.ErrNotFound
	}
	now := r.d.Clock.Now()
	var got Reactivated
	var m domain.Member
	err := r.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		userID, err := r.d.Accounts.ShareActiveAccountByEmail(ctx, email)
		if err != nil {
			return err
		}
		w, err := r.d.Locker.LockWorkspaceBySlug(ctx, slug)
		if err != nil {
			return found(err, domain.ErrNotFound)
		}
		if m, err = r.d.Members.FindMembership(ctx, w.ID, userID); err != nil {
			return found(err, domain.ErrMemberNotFound)
		}
		got = Reactivated{Slug: w.Slug, Role: m.Role}
		if m.Active() {
			got.Already = true
			return nil
		}
		got.EndedAt = *m.EndedAt
		return restore(ctx, r.d.Updater, r.d.Subscribers, m, m.Role, userID, now)
	})
	if err != nil {
		return Reactivated{}, err
	}
	if !got.Already {
		r.d.Logger.InfoContext(ctx, "workspace membership reactivated", slog.String("workspace_id", m.WorkspaceID.String()),
			slog.String("user_id", m.UserID.String()), slog.String("role", string(m.Role)), slog.String("by", byCLI))
	}
	return got, nil
}

// restore makes the ended membership m active again with role, by by at
// now, and tells the subscribers (M2/P3 design 3.5).
func restore(ctx context.Context, updater MemberUpdater, subscribers []MembershipRestoreSubscriber, m domain.Member,
	role shared.WorkspaceRole, by uuid.UUID, now time.Time,
) error {
	if err := updater.RestoreMember(ctx, m.ID, role, by, now); err != nil {
		return err
	}
	restored := MembershipRestore{WorkspaceID: m.WorkspaceID, UserID: m.UserID, Role: role, By: by, At: now}
	for _, s := range subscribers {
		if err := s.MembershipRestored(ctx, restored); err != nil {
			return err
		}
	}
	return nil
}
