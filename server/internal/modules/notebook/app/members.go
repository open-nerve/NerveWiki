package app

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// What the member use cases share: a membership read again under its
// notebook's lock, and memberships with their accounts' profiles.

// ListedMember is a membership with the account's profile, as the caller
// may see it.
type ListedMember struct {
	domain.Member
	DisplayName string
	Email       *string // nil: the caller is a guest of the workspace, who sees no member's email
}

// seesEmails reports whether a caller of workspace role sees the members'
// emails: the workspace's admins and members do, its guests do not, as in
// the workspace's member list.
func seesEmails(role shared.WorkspaceRole) bool {
	return role != shared.WorkspaceGuest
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
			return nil, fmt.Errorf("no profile of account %s, notebook member %s", m.UserID, m.ID)
		}
		list[i] = ListedMember{Member: m, DisplayName: p.DisplayName}
		if showEmails {
			list[i].Email = &p.Email
		}
	}
	return list, nil
}

// findMember reads the active membership id and its notebook, unlocked,
// for the notebook's workspace: the first lock is the workspace's.
// notebook.member_not_found when either is gone.
func findMember(ctx context.Context, members MemberFinder, notebooks NotebookFinder, id uuid.UUID) (domain.Member, domain.Notebook, error) {
	m, err := members.FindActiveMember(ctx, id)
	if err != nil {
		return domain.Member{}, domain.Notebook{}, found(err, domain.ErrMemberNotFound)
	}
	n, err := notebooks.FindNotebook(ctx, m.NotebookID)
	if err != nil {
		return domain.Member{}, domain.Notebook{}, found(err, domain.ErrMemberNotFound)
	}
	return m, n, nil
}

// lockMember locks n, the notebook of membership id read before the
// transaction ctx carries, for actor to do action, then reads the
// membership again under the lock, where no other member write of the
// notebook runs. notebook.member_not_found for a notebook gone or hidden,
// and for a membership ended meanwhile.
func (m manager) lockMember(ctx context.Context, actor shared.Actor, action shared.Action, n domain.Notebook, members MemberFinder,
	id uuid.UUID,
) (domain.Member, shared.Grant, error) {
	_, grant, err := m.lock(ctx, actor, action, n, domain.ErrMemberNotFound)
	if err != nil {
		return domain.Member{}, shared.Grant{}, err
	}
	member, err := members.FindActiveMember(ctx, id)
	if err != nil {
		return domain.Member{}, shared.Grant{}, found(err, domain.ErrMemberNotFound)
	}
	return member, grant, nil
}
