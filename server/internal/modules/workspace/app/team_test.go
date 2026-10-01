package app_test

import (
	"bytes"
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// team is what P2's and P3's use case tests start from: acme, whose admin
// is alice, bob a member and carol a guest; dana, an account of no
// workspace, whom acme invites as a member; the fakes the use cases take,
// the decision's and the tokens' among them recording into the store's
// calls.
type team struct {
	store    *fakeStore
	auth     *fakeAuthorizer
	tx       *fakeTx
	profiles *fakeProfiles
	accounts *fakeAccounts
	finder   fakeAccountFinder
	tokens   fakeTokens
	vetoer   *fakeVetoer
	sub      *fakeSubscriber
	clock    *tickingClock
	logs     *bytes.Buffer

	acme              domain.Workspace
	alice, bob, carol domain.Member
	dana              uuid.UUID         // dana's account
	invitation        domain.Invitation // acme's to dana@corp.com
}

func newTeam() *team {
	acme := domain.Workspace{ID: uuid.NewV7(), Slug: "acme", Name: "Acme", CreatedAt: now(), UpdatedAt: now()}
	member := func(role shared.WorkspaceRole, joined time.Duration) domain.Member {
		return domain.Member{ID: uuid.NewV7(), WorkspaceID: acme.ID, UserID: uuid.NewV7(), Role: role, CreatedAt: now().Add(joined)}
	}
	tm := &team{
		tx: &fakeTx{}, clock: &tickingClock{}, logs: &bytes.Buffer{}, acme: acme,
		alice: member(shared.WorkspaceAdmin, 0), bob: member(shared.WorkspaceMember, time.Minute), carol: member(shared.WorkspaceGuest, time.Hour),
	}
	tm.dana = uuid.NewV7()
	tm.invitation = domain.Invitation{ID: uuid.NewV7(), WorkspaceID: acme.ID, Email: "dana@corp.com", Role: shared.WorkspaceMember,
		CreatedAt: now().Add(-time.Hour)}
	tm.store = &fakeStore{
		workspaces:  map[string]domain.Workspace{"acme": acme},
		active:      map[uuid.UUID]domain.Member{tm.alice.ID: tm.alice, tm.bob.ID: tm.bob, tm.carol.ID: tm.carol},
		ended:       map[uuid.UUID]domain.Member{},
		invitations: map[uuid.UUID]domain.Invitation{tm.invitation.ID: tm.invitation},
	}
	tm.auth = &fakeAuthorizer{roles: map[uuid.UUID]shared.WorkspaceRole{}, store: tm.store}
	tm.profiles = &fakeProfiles{store: tm.store, profiles: map[uuid.UUID]app.Profile{
		tm.alice.UserID: {DisplayName: "Alice", Email: "alice@corp.com"},
		tm.bob.UserID:   {DisplayName: "Bob", Email: "bob@corp.com"},
		tm.carol.UserID: {DisplayName: "Carol", Email: "carol@corp.com"},
	}}
	emails := map[uuid.UUID]string{tm.alice.UserID: "alice@corp.com", tm.bob.UserID: "bob@corp.com", tm.carol.UserID: "carol@corp.com",
		tm.dana: "dana@corp.com"}
	tm.accounts = &fakeAccounts{emails: emails, store: tm.store}
	tm.finder = fakeAccountFinder{store: tm.store, ids: map[string]uuid.UUID{}}
	for id, email := range emails {
		tm.finder.ids[email] = id
	}
	tm.tokens = fakeTokens{store: tm.store}
	tm.vetoer = &fakeVetoer{store: tm.store}
	tm.sub = &fakeSubscriber{store: tm.store}
	return tm
}

// as is a context whose caller is m's account, which the decision sees
// with m's role in acme.
func (tm *team) as(m domain.Member) context.Context {
	tm.auth.roles[tm.acme.ID] = m.Role
	return as(m.UserID)
}

// asStranger is a context whose caller is no member of acme.
func (tm *team) asStranger() context.Context {
	delete(tm.auth.roles, tm.acme.ID)
	return as(uuid.NewV7())
}

func (tm *team) logger() *slog.Logger { return slog.New(slog.NewTextHandler(tm.logs, nil)) }

func (tm *team) ender() app.MembershipEnder {
	return app.MembershipEnder{Members: tm.store, Invitations: tm.store, Profiles: tm.profiles,
		Vetoers: []app.MembershipEndVetoer{tm.vetoer}, Subscribers: []app.MembershipEndSubscriber{tm.sub}}
}

func (tm *team) update() *app.UpdateWorkspace {
	return app.NewUpdateWorkspace(app.UpdateWorkspaceDeps{Locker: tm.store, Workspaces: tm.store, Auth: tm.auth,
		Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

func (tm *team) delete() *app.DeleteWorkspace {
	return app.NewDeleteWorkspace(app.DeleteWorkspaceDeps{Locker: tm.store, Workspaces: tm.store, Members: tm.store,
		Invitations: tm.store, Subscribers: []app.WorkspaceDeletionSubscriber{tm.sub}, Auth: tm.auth, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

func (tm *team) listMembers() *app.ListMembers {
	return app.NewListMembers(app.ListMembersDeps{Workspaces: tm.store, Members: tm.store, Profiles: tm.profiles, Auth: tm.auth})
}

func (tm *team) updateMember() *app.UpdateMember {
	return app.NewUpdateMember(app.UpdateMemberDeps{Locker: tm.store, Finder: tm.store, Members: tm.store, Profiles: tm.profiles,
		Auth: tm.auth, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

func (tm *team) removeMember() *app.RemoveMember {
	return app.NewRemoveMember(app.RemoveMemberDeps{Locker: tm.store, Finder: tm.store, Ender: tm.ender(),
		Auth: tm.auth, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

func (tm *team) leave() *app.LeaveWorkspace {
	return app.NewLeaveWorkspace(app.LeaveWorkspaceDeps{Locker: tm.store, Finder: tm.store, Ender: tm.ender(),
		Auth: tm.auth, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

func (tm *team) listInvitations() *app.ListInvitations {
	return app.NewListInvitations(app.ListInvitationsDeps{Workspaces: tm.store, Invitations: tm.store, Tokens: tm.tokens, Auth: tm.auth})
}

func (tm *team) createInvitation() *app.CreateInvitation {
	return app.NewCreateInvitation(app.CreateInvitationDeps{Sharer: tm.store, Accounts: tm.finder, Members: tm.store,
		Invitations: tm.store, Tokens: tm.tokens, Auth: tm.auth, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

func (tm *team) deleteInvitation() *app.DeleteInvitation {
	return app.NewDeleteInvitation(app.DeleteInvitationDeps{Finder: tm.store, Sharer: tm.store, Invitations: tm.store,
		Auth: tm.auth, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

func (tm *team) previewInvitation() *app.PreviewInvitation {
	return app.NewPreviewInvitation(app.PreviewInvitationDeps{Tokens: tm.tokens, Invitations: tm.store, Workspaces: tm.store})
}

func (tm *team) acceptInvitation() *app.AcceptInvitation {
	return app.NewAcceptInvitation(app.AcceptInvitationDeps{Tokens: tm.tokens, Finder: tm.store, Accounts: tm.accounts,
		Locker: tm.store, Invitations: tm.store, Members: tm.store, Updater: tm.store,
		Subscribers: []app.MembershipRestoreSubscriber{tm.sub}, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

// inTx is calls, each made in the transaction.
func inTxCalls(calls ...string) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c + " in tx"
	}
	return out
}

// at is " by <id> at <the clock's first read>", as the fake store records
// a write: each use case reads the clock once.
func at(by domain.Member) string {
	return " by " + by.UserID.String() + " at " + firstTick().Format(time.RFC3339Nano)
}
