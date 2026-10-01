package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// tokenOf is the fake tokens' token of inv.
func tokenOf(inv domain.Invitation) string { return "token of " + inv.ID.String() }

// invite adds a pending invitation of workspaceID to email with role.
func (tm *team) invite(workspaceID uuid.UUID, email string, role shared.WorkspaceRole) domain.Invitation {
	inv := domain.Invitation{ID: uuid.NewV7(), WorkspaceID: workspaceID, Email: email, Role: role, CreatedAt: now()}
	tm.store.invitations[inv.ID] = inv
	return inv
}

// assertNoAddressOrToken fails when the logs hold an address or a token.
func assertNoAddressOrToken(t *testing.T, logs string) {
	t.Helper()
	if strings.Contains(logs, "@corp.com") || strings.Contains(logs, "token of") {
		t.Errorf("logs hold an address or a token: %s", logs)
	}
}

func TestListInvitations(t *testing.T) {
	tm := newTeam()
	newer := tm.invite(tm.acme.ID, "erin@corp.com", shared.WorkspaceGuest)

	got, err := tm.listInvitations().Execute(tm.as(tm.alice), "acme")

	want := []app.ListedInvitation{{Invitation: newer, Token: tokenOf(newer)}, {Invitation: tm.invitation, Token: tokenOf(tm.invitation)}}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
	}
	wantCalls := []string{"FindWorkspaceBySlug acme", "Authorize workspace_invitation.list", "ListPendingInvitations"}
	if !slices.Equal(tm.store.calls, wantCalls) {
		t.Errorf("calls %q, want %q", tm.store.calls, wantCalls)
	}
}

func TestListInvitationsRefusals(t *testing.T) {
	for _, tt := range []struct {
		name, slug string
		caller     func(tm *team) context.Context
		want       error
		calls      []string
	}{
		{"a slug spelled as none", "\xff", func(tm *team) context.Context { return tm.as(tm.alice) }, domain.ErrNotFound, nil},
		{"no such workspace", "nowhere", func(tm *team) context.Context { return tm.as(tm.alice) }, domain.ErrNotFound,
			[]string{"FindWorkspaceBySlug nowhere"}},
		{"not a member", "acme", func(tm *team) context.Context { return tm.asStranger() }, domain.ErrNotFound,
			[]string{"FindWorkspaceBySlug acme", "Authorize workspace_invitation.list"}},
		{"a member", "acme", func(tm *team) context.Context { return tm.as(tm.bob) }, shared.Forbidden(),
			[]string{"FindWorkspaceBySlug acme", "Authorize workspace_invitation.list"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			ctx := tt.caller(tm)
			if errors.Is(tt.want, shared.Forbidden()) {
				tm.auth.err = shared.Forbidden()
			}

			_, err := tm.listInvitations().Execute(ctx, tt.slug)

			if !errors.Is(err, tt.want) || !slices.Equal(tm.store.calls, tt.calls) {
				t.Errorf("Execute() = %v after %q; want %v after %q", err, tm.store.calls, tt.want, tt.calls)
			}
		})
	}
}

// Under the share lock: the decision, the values, the address's account,
// then the write, at the clock's one read; the answer carries the token,
// the log neither the address nor the token.
func TestCreateInvitation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		email  string
		setUp  func(tm *team)
		lookup []string
	}{
		{"an address of no account", "  Erin@Corp.com ", nil, []string{"AccountIDByEmail erin@corp.com"}},
		{"an account of no membership", "dana@corp.com", func(tm *team) { delete(tm.store.invitations, tm.invitation.ID) },
			[]string{"AccountIDByEmail dana@corp.com", "FindMembership"}},
		{"an account whose membership ended", "bob@corp.com", func(tm *team) {
			tm.store.ended[tm.bob.ID] = tm.bob
			delete(tm.store.active, tm.bob.ID)
		}, []string{"AccountIDByEmail bob@corp.com", "FindMembership"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			if tt.setUp != nil {
				tt.setUp(tm)
			}
			email := shared.NormalizeEmail(tt.email)

			got, err := tm.createInvitation().Execute(tm.as(tm.alice), "acme", tt.email, "guest")

			want := app.ListedInvitation{Invitation: domain.Invitation{ID: got.ID, WorkspaceID: tm.acme.ID, Email: email,
				Role: shared.WorkspaceGuest, CreatedAt: firstTick()}}
			want.Token = tokenOf(want.Invitation)
			if err != nil || got != want {
				t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
			}
			wantCalls := inTxCalls(slices.Concat([]string{"ShareWorkspaceBySlug acme", "Authorize workspace_invitation.create"}, tt.lookup,
				[]string{"CreateInvitation " + email + " guest" + at(tm.alice)})...)
			if !slices.Equal(tm.store.calls, wantCalls) {
				t.Errorf("calls %q, want %q", tm.store.calls, wantCalls)
			}
			if !strings.Contains(tm.logs.String(), "workspace invitation created") {
				t.Errorf("logs = %s, want the invitation", tm.logs)
			}
			assertNoAddressOrToken(t, tm.logs.String())
		})
	}
}

// The order of the answers: 404, 403, then the values (422), the address
// of an active member among them.
func TestCreateInvitationRefusals(t *testing.T) {
	invalid := func(err error) bool {
		var se *shared.Error
		return errors.As(err, &se) && se.Code == shared.CodeValidationFailed && len(se.Fields) == 2
	}
	is := func(want error) func(error) bool { return func(err error) bool { return errors.Is(err, want) } }
	for _, tt := range []struct {
		name              string
		slug, email, role string
		caller            func(tm *team) context.Context
		setUp             func(tm *team)
		want              func(error) bool
		asked             bool
	}{
		{"a slug spelled as none", "\xff", "erin@corp.com", "member", func(tm *team) context.Context { return tm.as(tm.alice) }, nil,
			is(domain.ErrNotFound), false},
		{"no such workspace", "nowhere", "erin@corp.com", "member", func(tm *team) context.Context { return tm.as(tm.alice) }, nil,
			is(domain.ErrNotFound), true},
		{"not a member, invalid values", "acme", "erin", "owner", func(tm *team) context.Context { return tm.asStranger() }, nil,
			is(domain.ErrNotFound), true},
		{"a member, invalid values", "acme", "erin", "owner", func(tm *team) context.Context { return tm.as(tm.bob) },
			func(tm *team) { tm.auth.err = shared.Forbidden() }, is(shared.Forbidden()), true},
		{"invalid values", "acme", "erin", "owner", func(tm *team) context.Context { return tm.as(tm.alice) }, nil, invalid, true},
		{"an active member's address", "acme", "Bob@corp.com", "member", func(tm *team) context.Context { return tm.as(tm.alice) }, nil,
			is(domain.ErrAlreadyMember), true},
		{"the admin's own address", "acme", "alice@corp.com", "member", func(tm *team) context.Context { return tm.as(tm.alice) }, nil,
			is(domain.ErrAlreadyMember), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			ctx := tt.caller(tm)
			if tt.setUp != nil {
				tt.setUp(tm)
			}

			_, err := tm.createInvitation().Execute(ctx, tt.slug, tt.email, tt.role)

			if !tt.want(err) || slices.ContainsFunc(tm.store.calls, isWrite) || tm.logs.Len() != 0 || (len(tm.store.calls) != 0) != tt.asked {
				t.Errorf("Execute() = %v after %q; want no write and nothing logged", err, tm.store.calls)
			}
		})
	}
	t.Run("a pending invitation's address", func(t *testing.T) {
		tm := newTeam()
		tm.store.createInvitationErr = domain.ErrAlreadyInvited

		_, err := tm.createInvitation().Execute(tm.as(tm.alice), "acme", "dana@corp.com", "member")

		if !errors.Is(err, domain.ErrAlreadyInvited) || !tm.tx.rolledBack || tm.logs.Len() != 0 {
			t.Errorf("Execute() = %v, rolled back %v; want the duplicate, rolled back, nothing logged", err, tm.tx.rolledBack)
		}
	})
}

// The invitation read first names its workspace; under the workspace's
// share lock it is read again, locked, before the decision and the write.
func TestDeleteInvitation(t *testing.T) {
	tm := newTeam()

	err := tm.deleteInvitation().Execute(tm.as(tm.alice), tm.invitation.ID)

	wantCalls := append([]string{"FindPendingInvitation"}, inTxCalls("ShareWorkspaceByID", "LockPendingInvitation",
		"Authorize workspace_invitation.delete", "DeleteInvitation"+at(tm.alice))...)
	if err != nil || !slices.Equal(tm.store.calls, wantCalls) {
		t.Errorf("Execute() = %v after %q; want %q", err, tm.store.calls, wantCalls)
	}
	if !strings.Contains(tm.logs.String(), "workspace invitation deleted") {
		t.Errorf("logs = %s, want the deletion", tm.logs)
	}
	assertNoAddressOrToken(t, tm.logs.String())
}

func TestDeleteInvitationRefusals(t *testing.T) {
	for _, tt := range []struct {
		name   string
		caller func(tm *team) context.Context
		target func(tm *team) uuid.UUID
		setUp  func(tm *team)
		want   error
	}{
		{"no such invitation", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(*team) uuid.UUID { return uuid.NewV7() }, nil, domain.ErrInvitationNotFound},
		{"not a member", func(tm *team) context.Context { return tm.asStranger() },
			func(tm *team) uuid.UUID { return tm.invitation.ID }, nil, domain.ErrInvitationNotFound},
		{"a member", func(tm *team) context.Context { return tm.as(tm.bob) },
			func(tm *team) uuid.UUID { return tm.invitation.ID }, func(tm *team) { tm.auth.err = shared.Forbidden() }, shared.Forbidden()},
		{"accepted while it waited", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(tm *team) uuid.UUID { return tm.invitation.ID },
			func(tm *team) { tm.store.onLock = func() { delete(tm.store.invitations, tm.invitation.ID) } }, domain.ErrInvitationNotFound},
		{"its workspace deleted while it waited", func(tm *team) context.Context { return tm.as(tm.alice) },
			func(tm *team) uuid.UUID { return tm.invitation.ID },
			func(tm *team) { tm.store.onLock = func() { delete(tm.store.workspaces, "acme") } }, domain.ErrInvitationNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			ctx := tt.caller(tm)
			if tt.setUp != nil {
				tt.setUp(tm)
			}

			err := tm.deleteInvitation().Execute(ctx, tt.target(tm))

			if !errors.Is(err, tt.want) || slices.ContainsFunc(tm.store.calls, isWrite) || tm.logs.Len() != 0 {
				t.Errorf("Execute() = %v after %q; want %v, no write", err, tm.store.calls, tt.want)
			}
		})
	}
}

// Anyone with the link sees the workspace and the role; the token is
// checked before any read.
func TestPreviewInvitation(t *testing.T) {
	tm := newTeam()

	got, err := tm.previewInvitation().Execute(context.Background(), tm.invitation.ID, tokenOf(tm.invitation))

	want := app.InvitationPreview{Workspace: tm.acme, Role: shared.WorkspaceMember}
	wantCalls := []string{"Valid", "FindPendingInvitation", "FindWorkspaceByID"}
	if err != nil || got != want || !slices.Equal(tm.store.calls, wantCalls) {
		t.Errorf("Execute() = %+v, %v after %q; want %+v after %q", got, err, tm.store.calls, want, wantCalls)
	}
}

func TestPreviewInvitationRefusals(t *testing.T) {
	for _, tt := range []struct {
		name  string
		token func(tm *team) string
		setUp func(tm *team)
		calls []string
	}{
		{"a wrong token", func(tm *team) string { return "token of " + uuid.NewV7().String() }, nil, []string{"Valid"}},
		{"no longer pending", func(tm *team) string { return tokenOf(tm.invitation) },
			func(tm *team) { delete(tm.store.invitations, tm.invitation.ID) }, []string{"Valid", "FindPendingInvitation"}},
		{"its workspace deleted", func(tm *team) string { return tokenOf(tm.invitation) },
			func(tm *team) { delete(tm.store.workspaces, "acme") }, []string{"Valid", "FindPendingInvitation", "FindWorkspaceByID"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			if tt.setUp != nil {
				tt.setUp(tm)
			}

			_, err := tm.previewInvitation().Execute(context.Background(), tm.invitation.ID, tt.token(tm))

			if !errors.Is(err, domain.ErrInvitationNotFound) || !slices.Equal(tm.store.calls, tt.calls) {
				t.Errorf("Execute() = %v after %q; want workspace.invitation_not_found after %q", err, tm.store.calls, tt.calls)
			}
		})
	}
}

// The three memberships an acceptance meets: none, which it adds; an
// ended one, which it restores with the invitation's role, its subscribers
// told; an active one, which it keeps. Each uses the invitation up, at the
// membership's time.
func TestAcceptInvitation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		setUp   func(tm *team) (domain.Invitation, uuid.UUID)
		role    shared.WorkspaceRole
		joining func(by string) []string // the membership's calls, by " by <caller> at <time>"
		how     string
	}{
		{"a new member", func(tm *team) (domain.Invitation, uuid.UUID) { return tm.invitation, tm.dana }, shared.WorkspaceMember,
			func(by string) []string {
				return []string{"CountActiveAdmins", "AddMember member" + by, "MembershipAdded"}
			}, "added"},
		{"a member whose membership ended", func(tm *team) (domain.Invitation, uuid.UUID) {
			tm.store.ended[tm.bob.ID] = tm.bob
			delete(tm.store.active, tm.bob.ID)
			return tm.invite(tm.acme.ID, "bob@corp.com", shared.WorkspaceAdmin), tm.bob.UserID
		}, shared.WorkspaceAdmin, func(by string) []string {
			return []string{"CountActiveAdmins", "RestoreMember admin" + by, "MembershipRestored"}
		}, "restored"},
		{"an active member", func(tm *team) (domain.Invitation, uuid.UUID) {
			return tm.invite(tm.acme.ID, "carol@corp.com", shared.WorkspaceAdmin), tm.carol.UserID
		}, shared.WorkspaceGuest, func(string) []string { return nil }, "kept"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			inv, caller := tt.setUp(tm)
			by := " by " + caller.String() + " at " + firstTick().Format(time.RFC3339Nano)

			got, err := tm.acceptInvitation().Execute(as(caller), inv.ID, tokenOf(inv))

			if want := (app.Membership{Workspace: tm.acme, Role: tt.role}); err != nil || got != want {
				t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
			}
			wantCalls := append([]string{"Valid", "FindPendingInvitation"}, inTxCalls(slices.Concat(
				[]string{"ShareActiveAccount", "LockWorkspaceByID", "LockPendingInvitation", "FindMembership"}, tt.joining(by),
				[]string{"AcceptInvitation" + by})...)...)
			if !slices.Equal(tm.store.calls, wantCalls) {
				t.Errorf("calls %q, want %q", tm.store.calls, wantCalls)
			}
			if _, pending := tm.store.invitations[inv.ID]; pending {
				t.Error("the invitation is still pending")
			}
			if !strings.Contains(tm.logs.String(), "workspace invitation accepted") || !strings.Contains(tm.logs.String(), "membership="+tt.how) {
				t.Errorf("logs = %s, want the acceptance, membership=%s", tm.logs, tt.how)
			}
			assertNoAddressOrToken(t, tm.logs.String())
		})
	}
}

// Rule three: a workspace whose only admin was deactivated alone in it has
// no active member. A member or a guest cannot join it, by a new membership
// or an ended one: nothing is written, and the invitation stays pending. An
// admin can, and the workspace has an admin again.
func TestAcceptInvitationIntoAWorkspaceWithoutAdmin(t *testing.T) {
	for _, tt := range []struct {
		name  string
		role  shared.WorkspaceRole
		ended bool // the invitee is bob, whose membership ended; else dana, never a member
		err   error
	}{
		{"a new member", shared.WorkspaceMember, false, domain.ErrNoAdmin},
		{"a new guest", shared.WorkspaceGuest, false, domain.ErrNoAdmin},
		{"a member whose membership ended", shared.WorkspaceMember, true, domain.ErrNoAdmin},
		{"a new admin", shared.WorkspaceAdmin, false, nil},
		{"an admin whose membership ended", shared.WorkspaceAdmin, true, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			for id, m := range tm.store.active {
				delete(tm.store.active, id)
				tm.store.ended[id] = m
			}
			caller, email := tm.dana, "dana@corp.com"
			if tt.ended {
				caller, email = tm.bob.UserID, "bob@corp.com"
			}
			inv := tm.invite(tm.acme.ID, email, tt.role)

			got, err := tm.acceptInvitation().Execute(as(caller), inv.ID, tokenOf(inv))

			if !errors.Is(err, tt.err) {
				t.Fatalf("Execute() = %+v, %v; want %v", got, err, tt.err)
			}
			_, pending := tm.store.invitations[inv.ID]
			if tt.err != nil && (slices.ContainsFunc(tm.store.calls, isWrite) || !pending) {
				t.Errorf("calls %q, invitation pending %v; want no write, the invitation still pending", tm.store.calls, pending)
			}
			if tt.err == nil && got.Role != shared.WorkspaceAdmin {
				t.Errorf("Execute() = %+v, want the admin's membership", got)
			}
		})
	}
}

// The restore event names the workspace, the account, the new role, who
// restored it and when, at the membership's time.
func TestAcceptInvitationTellsTheRestoreSubscribers(t *testing.T) {
	tm := newTeam()
	tm.store.ended[tm.bob.ID] = tm.bob
	delete(tm.store.active, tm.bob.ID)
	inv := tm.invite(tm.acme.ID, "bob@corp.com", shared.WorkspaceGuest)

	if _, err := tm.acceptInvitation().Execute(as(tm.bob.UserID), inv.ID, tokenOf(inv)); err != nil {
		t.Fatal(err)
	}

	want := []app.MembershipRestore{{WorkspaceID: tm.acme.ID, UserID: tm.bob.UserID, Role: shared.WorkspaceGuest, By: tm.bob.UserID, At: firstTick()}}
	if !slices.Equal(tm.sub.restored, want) {
		t.Errorf("the subscriber saw %+v, want %+v", tm.sub.restored, want)
	}
	if m := tm.store.active[tm.bob.ID]; m.Role != shared.WorkspaceGuest || !m.CreatedAt.Equal(tm.bob.CreatedAt) {
		t.Errorf("bob's membership = %+v, want a guest who joined when he first did", m)
	}
}

// The addition event names the workspace, the account, its role, the
// account as who added it, and the membership's time; a restore and a kept
// membership are no addition (TestAcceptInvitation's calls).
func TestAcceptInvitationTellsTheAdditionSubscribers(t *testing.T) {
	tm := newTeam()

	if _, err := tm.acceptInvitation().Execute(as(tm.dana), tm.invitation.ID, tokenOf(tm.invitation)); err != nil {
		t.Fatal(err)
	}

	want := []app.MembershipAddition{{WorkspaceID: tm.acme.ID, UserID: tm.dana, Role: shared.WorkspaceMember, By: tm.dana, At: firstTick()}}
	if !slices.Equal(tm.sub.added, want) || len(tm.sub.restored) != 0 {
		t.Errorf("the subscriber saw %+v and restores %+v, want %+v alone", tm.sub.added, tm.sub.restored, want)
	}
}

func TestAcceptInvitationRefusals(t *testing.T) {
	deactivated := shared.NewError(shared.KindForbidden, "identity.account_deactivated", "Deactivated.")
	for _, tt := range []struct {
		name   string
		caller func(tm *team) uuid.UUID
		token  func(tm *team) string
		setUp  func(tm *team)
		want   error
		calls  int // how many calls were made, the last the refusal's
	}{
		{"a wrong token", func(tm *team) uuid.UUID { return tm.dana }, func(tm *team) string { return "token of " + uuid.NewV7().String() },
			nil, domain.ErrInvitationNotFound, 1},
		{"no longer pending", func(tm *team) uuid.UUID { return tm.dana }, func(tm *team) string { return tokenOf(tm.invitation) },
			func(tm *team) { delete(tm.store.invitations, tm.invitation.ID) }, domain.ErrInvitationNotFound, 2},
		{"a deactivated account", func(tm *team) uuid.UUID { return tm.dana }, func(tm *team) string { return tokenOf(tm.invitation) },
			func(tm *team) { tm.accounts.err = deactivated }, deactivated, 3},
		{"its workspace deleted while it waited", func(tm *team) uuid.UUID { return tm.dana },
			func(tm *team) string { return tokenOf(tm.invitation) },
			func(tm *team) { tm.store.onLock = func() { delete(tm.store.workspaces, "acme") } }, domain.ErrInvitationNotFound, 4},
		{"withdrawn while it waited", func(tm *team) uuid.UUID { return tm.dana }, func(tm *team) string { return tokenOf(tm.invitation) },
			func(tm *team) { tm.store.onLock = func() { delete(tm.store.invitations, tm.invitation.ID) } }, domain.ErrInvitationNotFound, 5},
		{"another account's address", func(tm *team) uuid.UUID { return tm.alice.UserID },
			func(tm *team) string { return tokenOf(tm.invitation) }, nil, domain.ErrInvitationEmailMismatch, 5},
		{"an address changed before the lock", func(tm *team) uuid.UUID { return tm.dana },
			func(tm *team) string { return tokenOf(tm.invitation) }, func(tm *team) { tm.accounts.emails[tm.dana] = "dana@elsewhere.com" },
			domain.ErrInvitationEmailMismatch, 5},
		// ſ (U+017F, long s) folds to s: another normalized address,
		// though strings.EqualFold takes it for the same.
		{"an address equal to it but for case folding", func(tm *team) uuid.UUID { return tm.dana },
			func(tm *team) string { return tokenOf(tm.invitation) }, func(tm *team) {
				tm.invitation.Email = "dana.s@corp.com"
				tm.store.invitations[tm.invitation.ID] = tm.invitation
				tm.accounts.emails[tm.dana] = "dana.ſ@corp.com"
			}, domain.ErrInvitationEmailMismatch, 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTeam()
			if tt.setUp != nil {
				tt.setUp(tm)
			}

			_, err := tm.acceptInvitation().Execute(as(tt.caller(tm)), tm.invitation.ID, tt.token(tm))

			if !errors.Is(err, tt.want) || len(tm.store.calls) != tt.calls || slices.ContainsFunc(tm.store.calls, isWrite) || tm.logs.Len() != 0 {
				t.Errorf("Execute() = %v after %q; want %v after %d calls, no write", err, tm.store.calls, tt.want, tt.calls)
			}
		})
	}
	t.Run("the restore subscriber fails", func(t *testing.T) {
		tm := newTeam()
		failed := errors.New("the subscriber failed")
		tm.sub.err = failed
		tm.store.ended[tm.bob.ID] = tm.bob
		delete(tm.store.active, tm.bob.ID)
		inv := tm.invite(tm.acme.ID, "bob@corp.com", shared.WorkspaceMember)

		_, err := tm.acceptInvitation().Execute(as(tm.bob.UserID), inv.ID, tokenOf(inv))

		if !errors.Is(err, failed) || !tm.tx.rolledBack || tm.logs.Len() != 0 || slices.ContainsFunc(tm.store.calls, func(c string) bool { return strings.HasPrefix(c, "AcceptInvitation") }) {
			t.Errorf("Execute() = %v, rolled back %v after %q; want the subscriber's error, rolled back", err, tm.tx.rolledBack, tm.store.calls)
		}
	})
}

// A membership's end deletes the pending invitations of its workspaces to
// the account's address, and no other.
func TestMembershipEndDeletesTheInvitationsToTheAddress(t *testing.T) {
	tm := newTeam()
	toBob := tm.invite(tm.acme.ID, "bob@corp.com", shared.WorkspaceAdmin)
	elsewhere := tm.invite(uuid.NewV7(), "bob@corp.com", shared.WorkspaceMember)

	if err := tm.removeMember().Execute(tm.as(tm.alice), tm.bob.ID); err != nil {
		t.Fatal(err)
	}

	_, toBobPending := tm.store.invitations[toBob.ID]
	_, elsewherePending := tm.store.invitations[elsewhere.ID]
	_, danaPending := tm.store.invitations[tm.invitation.ID]
	if toBobPending || !elsewherePending || !danaPending {
		t.Errorf("pending: bob's to acme %v, bob's elsewhere %v, dana's %v; want only bob's to acme deleted", toBobPending, elsewherePending, danaPending)
	}
}

// Without the account's address the membership cannot end: its profile's
// failure, or its absence, stops it before any write.
func TestMembershipEndNeedsTheAddress(t *testing.T) {
	for name, setUp := range map[string]func(tm *team){
		"the read fails": func(tm *team) { tm.profiles.err = errors.New("directory down") },
		"no profile":     func(tm *team) { delete(tm.profiles.profiles, tm.bob.UserID) },
	} {
		t.Run(name, func(t *testing.T) {
			tm := newTeam()
			setUp(tm)

			err := tm.removeMember().Execute(tm.as(tm.alice), tm.bob.ID)

			if err == nil || !tm.tx.rolledBack || slices.ContainsFunc(tm.store.calls, isWrite) {
				t.Errorf("Execute() = %v after %q; want an error before any write, rolled back", err, tm.store.calls)
			}
		})
	}
}

// A workspace's deletion takes its pending invitations with it.
func TestDeleteWorkspaceDeletesItsInvitations(t *testing.T) {
	tm := newTeam()
	elsewhere := tm.invite(uuid.NewV7(), "dana@corp.com", shared.WorkspaceMember)

	if err := tm.delete().Execute(tm.as(tm.alice), "acme"); err != nil {
		t.Fatal(err)
	}

	if _, ok := tm.store.invitations[tm.invitation.ID]; ok || len(tm.store.invitations) != 1 || tm.store.invitations[elsewhere.ID] != elsewhere {
		t.Errorf("pending invitations %+v, want only the other workspace's", tm.store.invitations)
	}
}
