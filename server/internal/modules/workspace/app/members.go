package app

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
)

// What the member use cases share: a membership read again under its
// workspace's lock, and memberships with their accounts' profiles.

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

// ListedMember is a membership with the account's profile, as the caller
// may see it.
type ListedMember struct {
	domain.Member
	DisplayName string
	Email       *string // nil: the caller is a guest, who sees no member's email
}

// withProfiles is members with their accounts' profiles, the emails only
// when showEmails. A membership's account always exists (a foreign key), so
// a missing profile is a fault.
func withProfiles(ctx context.Context, profiles MemberProfiles, members []domain.Member, showEmails bool) ([]ListedMember, error) {
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	byID, err := profiles.MemberProfiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	list := make([]ListedMember, len(members))
	for i, m := range members {
		p, ok := byID[m.UserID]
		if !ok {
			return nil, fmt.Errorf("no profile of account %s, member %s", m.UserID, m.ID)
		}
		list[i] = ListedMember{Member: m, DisplayName: p.DisplayName}
		if showEmails {
			list[i].Email = &p.Email
		}
	}
	return list, nil
}
