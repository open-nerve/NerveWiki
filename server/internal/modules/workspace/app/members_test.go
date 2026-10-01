package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// email is a pointer to e.
func email(e string) *string { return &e }

func TestListMembersShowsEmailsToAdminsAndMembersOnly(t *testing.T) {
	for _, tt := range []struct {
		caller string
		emails bool
	}{{"admin", true}, {"member", true}, {"guest", false}} {
		t.Run(tt.caller, func(t *testing.T) {
			tm := newTeam()
			caller := map[string]domain.Member{"admin": tm.alice, "member": tm.bob, "guest": tm.carol}[tt.caller]

			got, err := tm.listMembers().Execute(tm.as(caller), "acme")

			want := []app.ListedMember{
				{Member: tm.alice, DisplayName: "Alice", Email: email("alice@corp.com")},
				{Member: tm.bob, DisplayName: "Bob", Email: email("bob@corp.com")},
				{Member: tm.carol, DisplayName: "Carol", Email: email("carol@corp.com")},
			}
			if !tt.emails {
				for i := range want {
					want[i].Email = nil
				}
			}
			if err != nil || !slices.EqualFunc(got, want, sameListed) {
				t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
			}
			// A read: no transaction, the decision before the list.
			wantCalls := []string{"FindWorkspaceBySlug acme", "Authorize workspace_member.list", "ListActiveMembers", "MemberProfiles"}
			if !slices.Equal(tm.store.calls, wantCalls) || len(tm.profiles.asked) != 1 {
				t.Errorf("calls = %q, profiles asked %d times; want %q and once", tm.store.calls, len(tm.profiles.asked), wantCalls)
			}
		})
	}
}

func sameListed(a, b app.ListedMember) bool {
	return a.Member == b.Member && a.DisplayName == b.DisplayName &&
		(a.Email == nil) == (b.Email == nil) && (a.Email == nil || *a.Email == *b.Email)
}

func TestListMembersRefusals(t *testing.T) {
	t.Run("a slug spelled as none", func(t *testing.T) {
		tm := newTeam()
		if _, err := tm.listMembers().Execute(tm.as(tm.alice), "Acme"); !errors.Is(err, domain.ErrNotFound) || len(tm.store.calls) != 0 {
			t.Errorf("Execute() = %v after %q; want not_found, nothing asked", err, tm.store.calls)
		}
	})
	t.Run("not a member", func(t *testing.T) {
		tm := newTeam()
		if _, err := tm.listMembers().Execute(tm.asStranger(), "acme"); !errors.Is(err, domain.ErrNotFound) || len(tm.profiles.asked) != 0 {
			t.Errorf("Execute() = %v; want not_found, no profile read", err)
		}
	})
	t.Run("an account without a profile", func(t *testing.T) {
		tm := newTeam()
		delete(tm.profiles.profiles, tm.bob.UserID)
		if _, err := tm.listMembers().Execute(tm.as(tm.alice), "acme"); err == nil || errors.As(err, new(*shared.Error)) {
			t.Errorf("Execute() = %v, want a fault", err)
		}
	})
}

// The membership is read, its workspace locked, the membership read again,
// then the decision, the role, the caller's own, and the write.
func TestUpdateMemberChangesTheRoleUnderTheLock(t *testing.T) {
	tm := newTeam()

	got, err := tm.updateMember().Execute(tm.as(tm.alice), tm.bob.ID, "guest")

	want := app.ListedMember{Member: tm.bob, DisplayName: "Bob", Email: email("bob@corp.com")}
	want.Role = shared.WorkspaceGuest
	if err != nil || !sameListed(got, want) {
		t.Errorf("Execute() = %+v, %v; want bob as a guest", got, err)
	}
	wantCalls := append([]string{"FindActiveMember"}, inTxCalls("LockWorkspaceByID", "FindActiveMember",
		"Authorize workspace_member.update", "UpdateMemberRole guest"+at(tm.alice), "MemberProfiles")...)
	if !slices.Equal(tm.store.calls, wantCalls) {
		t.Errorf("calls = %q, want %q", tm.store.calls, wantCalls)
	}
	if !strings.Contains(tm.logs.String(), "workspace member updated") {
		t.Errorf("logs = %s, want the update", tm.logs)
	}
}

// The answer's profile is read in the transaction: when it fails, the
// change is rolled back, not committed under a 500.
func TestUpdateMemberRollsBackWhenTheProfileCannotBeRead(t *testing.T) {
	tm := newTeam()
	failed := errors.New("connection reset")
	tm.profiles.err = failed

	_, err := tm.updateMember().Execute(tm.as(tm.alice), tm.bob.ID, "guest")

	if !errors.Is(err, failed) || !tm.tx.rolledBack || tm.logs.Len() != 0 {
		t.Errorf("Execute() = %v, rolled back %v, logs %s; want the failure, rolled back, nothing logged", err, tm.tx.rolledBack, tm.logs)
	}
}

// A membership that is not there, or that the caller cannot see, is
// member_not_found; the role is checked after the decision, then the
// caller's own membership; none of them writes.
func TestUpdateMemberRefusals(t *testing.T) {
	tests := []struct {
		name   string
		caller func(tm *team) context.Context
		target func(tm *team) uuid.UUID
		role   string
		meant  func(tm *team) // what another transaction committed before the lock
		want   error
	}{
		{"no such membership", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(*team) uuid.UUID { return uuid.NewV7() }, "guest", nil, domain.ErrMemberNotFound},
		{"ended before the lock", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(tm *team) uuid.UUID { return tm.bob.ID }, "guest", func(tm *team) { delete(tm.store.active, tm.bob.ID) }, domain.ErrMemberNotFound},
		{"its workspace deleted before the lock", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(tm *team) uuid.UUID { return tm.bob.ID }, "guest", func(tm *team) { delete(tm.store.workspaces, "acme") }, domain.ErrMemberNotFound},
		{"not a member, a bad role", func(tm *team) context.Context { return tm.asStranger() },
			func(tm *team) uuid.UUID { return tm.bob.ID }, "owner", nil, domain.ErrMemberNotFound},
		{"a member", func(tm *team) context.Context { return tm.as(tm.bob) },
			func(tm *team) uuid.UUID { return tm.carol.ID }, "member", nil, shared.Forbidden()},
		{"the admin, a bad role", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(tm *team) uuid.UUID { return tm.bob.ID }, "owner", nil, shared.Invalid()},
		{"the admin's own", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(tm *team) uuid.UUID { return tm.alice.ID }, "member", nil, domain.ErrOwnMembership},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			ctx := tt.caller(tm)
			if errors.Is(tt.want, shared.Forbidden()) {
				tm.auth.err = shared.Forbidden()
			}
			if tt.meant != nil {
				tm.store.onLock = func() { tt.meant(tm) }
			}

			_, err := tm.updateMember().Execute(ctx, tt.target(tm), tt.role)

			if !errors.Is(err, tt.want) || slices.ContainsFunc(tm.store.calls, isWrite) || tm.logs.Len() != 0 {
				t.Errorf("Execute() = %v after %q; want %v, no write", err, tm.store.calls, tt.want)
			}
		})
	}
}

// isWrite reports a call of the fake store that writes.
func isWrite(call string) bool {
	for _, w := range []string{"UpdateMemberRole", "EndMemberships", "DeleteMembersOf", "DeleteWorkspace", "RenameWorkspace"} {
		if strings.HasPrefix(call, w) {
			return true
		}
	}
	return false
}

// The membership ends through the extension point: the vetoers, the
// write, the subscribers, under the lock and after the decision.
func TestRemoveMemberEndsTheMembership(t *testing.T) {
	tm := newTeam()

	err := tm.removeMember().Execute(tm.as(tm.alice), tm.bob.ID)

	wantCalls := append([]string{"FindActiveMember"}, inTxCalls("LockWorkspaceByID", "FindActiveMember",
		"Authorize workspace_member.remove", "VetoMembershipEnd", "EndMemberships"+at(tm.alice), "MembershipEnded")...)
	if err != nil || !slices.Equal(tm.store.calls, wantCalls) {
		t.Errorf("Execute() = %v after %q; want %q", err, tm.store.calls, wantCalls)
	}
	want := app.MembershipEnd{UserID: tm.bob.UserID, WorkspaceIDs: []uuid.UUID{tm.acme.ID}, Cause: app.EndRemoved,
		By: tm.alice.UserID, At: firstTick()}
	if len(tm.vetoer.seen) != 1 || !sameEnd(tm.vetoer.seen[0], want) || len(tm.sub.ended) != 1 || !sameEnd(tm.sub.ended[0], want) {
		t.Errorf("the vetoer saw %+v, the subscriber %+v; want %+v", tm.vetoer.seen, tm.sub.ended, want)
	}
	if !strings.Contains(tm.logs.String(), "workspace member removed") {
		t.Errorf("logs = %s, want the removal", tm.logs)
	}
}

func sameEnd(a, b app.MembershipEnd) bool {
	return a.UserID == b.UserID && slices.Equal(a.WorkspaceIDs, b.WorkspaceIDs) && a.Cause == b.Cause && a.By == b.By && a.At.Equal(b.At)
}

func TestRemoveMemberRefusals(t *testing.T) {
	vetoed := shared.NewError(shared.KindConflict, "test.vetoed", "Vetoed.")
	failed := errors.New("the subscriber failed")
	tests := []struct {
		name   string
		caller func(tm *team) context.Context
		target func(tm *team) uuid.UUID
		setUp  func(tm *team)
		want   error
	}{
		{"no such membership", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(*team) uuid.UUID { return uuid.NewV7() }, nil, domain.ErrMemberNotFound},
		{"not a member", func(tm *team) context.Context { return tm.asStranger() },
			func(tm *team) uuid.UUID { return tm.bob.ID }, nil, domain.ErrMemberNotFound},
		{"a guest", func(tm *team) context.Context { return tm.as(tm.carol) },
			func(tm *team) uuid.UUID { return tm.bob.ID }, func(tm *team) { tm.auth.err = shared.Forbidden() }, shared.Forbidden()},
		{"the admin's own", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(tm *team) uuid.UUID { return tm.alice.ID }, nil, domain.ErrOwnMembership},
		{"vetoed", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(tm *team) uuid.UUID { return tm.bob.ID }, func(tm *team) { tm.vetoer.err = vetoed }, vetoed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			ctx := tt.caller(tm)
			if tt.setUp != nil {
				tt.setUp(tm)
			}

			err := tm.removeMember().Execute(ctx, tt.target(tm))

			if !errors.Is(err, tt.want) || slices.ContainsFunc(tm.store.calls, isWrite) || len(tm.sub.ended) != 0 || tm.logs.Len() != 0 {
				t.Errorf("Execute() = %v after %q; want %v, no write", err, tm.store.calls, tt.want)
			}
		})
	}
	t.Run("the subscriber fails", func(t *testing.T) {
		tm := newTeam()
		tm.sub.err = failed
		if err := tm.removeMember().Execute(tm.as(tm.alice), tm.bob.ID); !errors.Is(err, failed) || !tm.tx.rolledBack {
			t.Errorf("Execute() = %v, rolled back %v; want the subscriber's error, rolled back", err, tm.tx.rolledBack)
		}
	})
}

func TestLeaveWorkspace(t *testing.T) {
	t.Run("a member", func(t *testing.T) {
		tm := newTeam()

		err := tm.leave().Execute(tm.as(tm.bob), "acme")

		wantCalls := inTxCalls("LockWorkspaceBySlug acme", "Authorize workspace.leave",
			"VetoMembershipEnd", "EndMemberships"+at(tm.bob), "MembershipEnded")
		if err != nil || !slices.Equal(tm.store.calls, wantCalls) {
			t.Errorf("Execute() = %v after %q; want %q", err, tm.store.calls, wantCalls)
		}
		want := app.MembershipEnd{UserID: tm.bob.UserID, WorkspaceIDs: []uuid.UUID{tm.acme.ID}, Cause: app.EndLeft, By: tm.bob.UserID, At: firstTick()}
		if len(tm.sub.ended) != 1 || !sameEnd(tm.sub.ended[0], want) {
			t.Errorf("the subscriber saw %+v, want %+v", tm.sub.ended, want)
		}
		if !strings.Contains(tm.logs.String(), "workspace left") {
			t.Errorf("logs = %s, want the leaving", tm.logs)
		}
	})
	t.Run("an admin beside another", func(t *testing.T) {
		tm := newTeam()
		promoted := tm.bob
		promoted.Role = shared.WorkspaceAdmin
		tm.store.active[promoted.ID] = promoted

		err := tm.leave().Execute(tm.as(tm.alice), "acme")

		wantCalls := inTxCalls("LockWorkspaceBySlug acme", "Authorize workspace.leave", "CountActiveAdmins",
			"VetoMembershipEnd", "EndMemberships"+at(tm.alice), "MembershipEnded")
		if err != nil || !slices.Equal(tm.store.calls, wantCalls) {
			t.Errorf("Execute() = %v after %q; want %q", err, tm.store.calls, wantCalls)
		}
	})
}

// The only admin cannot leave, alone or not; nor can a caller who cannot
// see the workspace. Neither asks the extension point.
func TestLeaveWorkspaceRefusals(t *testing.T) {
	tests := []struct {
		name   string
		slug   string
		caller func(tm *team) context.Context
		setUp  func(tm *team)
		want   error
	}{
		{"a slug spelled as none", "\x00", func(tm *team) context.Context { return tm.as(tm.bob) }, nil, domain.ErrNotFound},
		{"not a member", "acme", func(tm *team) context.Context { return tm.asStranger() }, nil, domain.ErrNotFound},
		{"the only admin", "acme", func(tm *team) context.Context { return tm.as(tm.alice) }, nil, domain.ErrSoleAdmin},
		{"the only admin, alone", "acme", func(tm *team) context.Context { return tm.as(tm.alice) }, func(tm *team) {
			delete(tm.store.active, tm.bob.ID)
			delete(tm.store.active, tm.carol.ID)
		}, domain.ErrSoleAdmin},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			ctx := tt.caller(tm)
			if tt.setUp != nil {
				tt.setUp(tm)
			}

			err := tm.leave().Execute(ctx, tt.slug)

			if !errors.Is(err, tt.want) || slices.ContainsFunc(tm.store.calls, isWrite) || len(tm.vetoer.seen) != 0 {
				t.Errorf("Execute() = %v after %q; want %v, the extension point not asked", err, tm.store.calls, tt.want)
			}
			if tt.slug != "acme" && len(tm.store.calls) != 0 {
				t.Errorf("Execute(%q) asked %q, want nothing asked", tt.slug, tm.store.calls)
			}
		})
	}
}
