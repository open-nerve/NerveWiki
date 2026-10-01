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

// team is what P2's use case tests start from: acme, whose admin is alice,
// bob a member and carol a guest; the fakes the use cases take, the
// decision's among them recording into the store's calls.
type team struct {
	store    *fakeStore
	auth     *fakeAuthorizer
	tx       *fakeTx
	profiles *fakeProfiles
	vetoer   *fakeVetoer
	sub      *fakeSubscriber
	clock    *tickingClock
	logs     *bytes.Buffer

	acme              domain.Workspace
	alice, bob, carol domain.Member
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
	tm.store = &fakeStore{
		workspaces: map[string]domain.Workspace{"acme": acme},
		active:     map[uuid.UUID]domain.Member{tm.alice.ID: tm.alice, tm.bob.ID: tm.bob, tm.carol.ID: tm.carol},
	}
	tm.auth = &fakeAuthorizer{roles: map[uuid.UUID]shared.WorkspaceRole{}, store: tm.store}
	tm.profiles = &fakeProfiles{store: tm.store, profiles: map[uuid.UUID]app.Profile{
		tm.alice.UserID: {DisplayName: "Alice", Email: "alice@corp.com"},
		tm.bob.UserID:   {DisplayName: "Bob", Email: "bob@corp.com"},
		tm.carol.UserID: {DisplayName: "Carol", Email: "carol@corp.com"},
	}}
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
	return app.MembershipEnder{Members: tm.store, Vetoers: []app.MembershipEndVetoer{tm.vetoer},
		Subscribers: []app.MembershipEndSubscriber{tm.sub}}
}

func (tm *team) update() *app.UpdateWorkspace {
	return app.NewUpdateWorkspace(app.UpdateWorkspaceDeps{Locker: tm.store, Workspaces: tm.store, Auth: tm.auth,
		Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

func (tm *team) delete() *app.DeleteWorkspace {
	return app.NewDeleteWorkspace(app.DeleteWorkspaceDeps{Locker: tm.store, Workspaces: tm.store, Members: tm.store,
		Subscribers: []app.WorkspaceDeletionSubscriber{tm.sub}, Auth: tm.auth, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
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
