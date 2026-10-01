package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The administrator's reactivate-member (M2/P4 design 3.3).

func (tm *team) reactivate() *app.ReactivateMember {
	return app.NewReactivateMember(app.ReactivateMemberDeps{Accounts: tm.accounts, Locker: tm.store, Members: tm.store, Updater: tm.store,
		Subscribers: []app.MembershipRestoreSubscriber{tm.sub}, Tx: tm.tx, Clock: tm.clock, Logger: tm.logger()})
}

// An ended membership comes back with its own role, after the account's
// share and under the workspace's lock; the restore's subscribers follow.
func TestReactivateMemberRestoresTheEndedMembership(t *testing.T) {
	tm := newTeam()
	delete(tm.store.active, tm.bob.ID)
	tm.store.ended[tm.bob.ID] = tm.bob

	got, err := tm.reactivate().Execute(as(tm.alice.UserID), "acme", "bob@corp.com")

	want := app.Reactivated{Slug: "acme", Role: shared.WorkspaceMember, EndedAt: now().Add(-time.Hour)}
	if err != nil || got != want {
		t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
	}
	wantCalls := inTxCalls("ShareActiveAccountByEmail bob@corp.com", "LockWorkspaceBySlug acme", "FindMembership",
		"CountActiveAdmins", "RestoreMember member"+at(tm.bob), "MembershipRestored")
	if !slices.Equal(tm.store.calls, wantCalls) {
		t.Errorf("calls = %q, want %q", tm.store.calls, wantCalls)
	}
	restored := app.MembershipRestore{WorkspaceID: tm.acme.ID, UserID: tm.bob.UserID, Role: shared.WorkspaceMember, By: tm.bob.UserID,
		At: firstTick()}
	if len(tm.sub.restored) != 1 || tm.sub.restored[0] != restored {
		t.Errorf("the subscriber saw %+v, want %+v", tm.sub.restored, restored)
	}
	logs := tm.logs.String()
	if !strings.Contains(logs, "workspace membership reactivated") || !strings.Contains(logs, "by=cli") || strings.Contains(logs, "bob@") {
		t.Errorf("logs = %s, want the reactivation by=cli, without the address", logs)
	}
}

// Rule three: once the only admin was deactivated alone in the workspace, a
// member's or a guest's membership does not come back before an admin's;
// the admin's does.
func TestReactivateMemberIntoAWorkspaceWithoutAdmin(t *testing.T) {
	tm := newTeam()
	for id, m := range tm.store.active {
		delete(tm.store.active, id)
		tm.store.ended[id] = m
	}

	for _, email := range []string{"bob@corp.com", "carol@corp.com"} {
		if _, err := tm.reactivate().Execute(as(tm.alice.UserID), "acme", email); !errors.Is(err, domain.ErrNoAdmin) {
			t.Errorf("Execute(%s) = %v, want %v", email, err, domain.ErrNoAdmin)
		}
	}
	if slices.ContainsFunc(tm.store.calls, isWrite) || len(tm.sub.restored) != 0 {
		t.Errorf("calls %q, restored %+v; want no write, no event", tm.store.calls, tm.sub.restored)
	}
	if _, err := tm.reactivate().Execute(as(tm.alice.UserID), "acme", "alice@corp.com"); err != nil {
		t.Fatalf("Execute(alice) = %v, want her admin membership back", err)
	}
	if _, err := tm.reactivate().Execute(as(tm.alice.UserID), "acme", "bob@corp.com"); err != nil {
		t.Errorf("Execute(bob) after alice = %v, want his membership back", err)
	}
}

// An active membership is left as it is: no write, no event, no log.
func TestReactivateMemberOfAnActiveMember(t *testing.T) {
	tm := newTeam()

	got, err := tm.reactivate().Execute(as(tm.alice.UserID), "acme", "carol@corp.com")

	if err != nil || got != (app.Reactivated{Slug: "acme", Role: shared.WorkspaceGuest, Already: true}) ||
		slices.ContainsFunc(tm.store.calls, isWrite) || len(tm.sub.restored) != 0 || tm.logs.Len() != 0 {
		t.Errorf("Execute() = %+v, %v after %q; want carol's guest membership already active, nothing done", got, err, tm.store.calls)
	}
}

func TestReactivateMemberRefusals(t *testing.T) {
	for _, tt := range []struct {
		name, slug, email string
		want              error
		calls             []string
	}{
		{"an invalid slug", "Not A Slug", "bob@corp.com", domain.ErrNotFound, nil},
		{"no such account", "acme", "erin@corp.com", errNoAccount(), inTxCalls("ShareActiveAccountByEmail erin@corp.com")},
		{"no such workspace", "gone", "bob@corp.com", domain.ErrNotFound,
			inTxCalls("ShareActiveAccountByEmail bob@corp.com", "LockWorkspaceBySlug gone")},
		{"never a member", "acme", "dana@corp.com", domain.ErrMemberNotFound,
			inTxCalls("ShareActiveAccountByEmail dana@corp.com", "LockWorkspaceBySlug acme", "FindMembership")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()

			_, err := tm.reactivate().Execute(as(tm.alice.UserID), tt.slug, tt.email)

			if !errors.Is(err, tt.want) || !slices.Equal(tm.store.calls, tt.calls) || tm.logs.Len() != 0 {
				t.Errorf("Execute() = %v after %q; want %v after %q", err, tm.store.calls, tt.want, tt.calls)
			}
		})
	}
	t.Run("the subscriber fails", func(t *testing.T) {
		tm := newTeam()
		delete(tm.store.active, tm.bob.ID)
		tm.store.ended[tm.bob.ID] = tm.bob
		tm.sub.err = errors.New("the subscriber failed")

		if _, err := tm.reactivate().Execute(as(tm.alice.UserID), "acme", "bob@corp.com"); !errors.Is(err, tm.sub.err) ||
			!tm.tx.rolledBack || tm.logs.Len() != 0 {
			t.Errorf("Execute() = %v, rolled back %v; want the subscriber's error, rolled back", err, tm.tx.rolledBack)
		}
	})
}
