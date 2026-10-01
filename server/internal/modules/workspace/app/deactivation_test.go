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

// The module's part in a deactivation (M2/P4 design 3.1), as identity
// calls it: both phases in its transaction, the vetoer first.

func (tm *team) deactivation() app.Deactivation {
	ender := tm.ender()
	ender.Profiles = nil // a deactivation writes with the address it was given
	return app.Deactivation{Locker: tm.store, Standings: tm.store, Lister: tm.store, Ender: ender}
}

// deactivated is m's account being deactivated: the address it has under
// the account's lock is not the profile's, which the deactivation does not
// read.
func deactivated(m domain.Member) app.Deactivated {
	return app.Deactivated{UserID: m.UserID, Email: "renamed@corp.com", At: now()}
}

// deactivate runs the two phases in one transaction, as identity does.
func (tm *team) deactivate(x app.Deactivated) error {
	d := tm.deactivation()
	return tm.tx.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := d.VetoDeactivation(ctx, x); err != nil {
			return err
		}
		return d.AccountDeactivated(ctx, x)
	})
}

// listed makes the store list m's workspaces as its active memberships.
func (tm *team) listed(m domain.Member) {
	tm.store.memberships = []app.Membership{{Workspace: tm.acme, Role: m.Role}}
}

// The workspaces are locked before the memberships are read and judged;
// the vetoers run before any write; the end is written with the address
// read under the account's lock, then the subscribers follow.
func TestDeactivationEndsTheMemberships(t *testing.T) {
	tm := newTeam()
	tm.listed(tm.bob)

	err := tm.deactivate(deactivated(tm.bob))

	by := " by " + tm.bob.UserID.String() + " at " + now().Format(time.RFC3339Nano)
	wantCalls := inTxCalls("LockWorkspacesOf", "ListStandings", "VetoMembershipEnd",
		"ListWorkspacesOf "+tm.bob.UserID.String(), "DeleteInvitationsTo renamed@corp.com"+by, "EndMemberships"+by, "MembershipEnded")
	if err != nil || !slices.Equal(tm.store.calls, wantCalls) {
		t.Errorf("deactivate = %v after %q; want %q", err, tm.store.calls, wantCalls)
	}
	want := app.MembershipEnd{UserID: tm.bob.UserID, WorkspaceIDs: []uuid.UUID{tm.acme.ID}, Cause: app.EndDeactivated,
		By: tm.bob.UserID, At: now()}
	if len(tm.vetoer.seen) != 1 || !sameEnd(tm.vetoer.seen[0], want) || len(tm.sub.ended) != 1 || !sameEnd(tm.sub.ended[0], want) {
		t.Errorf("the vetoer saw %+v, the subscriber %+v; want %+v", tm.vetoer.seen, tm.sub.ended, want)
	}
}

// Rule two refuses the only admin of a workspace with other members,
// naming every such workspace by slug, before the vetoers and any write.
// Alone in a workspace, the account passes.
func TestDeactivationRuleTwo(t *testing.T) {
	t.Run("the only admin, with others", func(t *testing.T) {
		tm := newTeam()
		zeta := domain.Workspace{ID: uuid.NewV7(), Slug: "zeta", Name: "Zeta"}
		tm.store.workspaces["zeta"] = zeta
		for _, m := range []domain.Member{
			{ID: uuid.NewV7(), WorkspaceID: zeta.ID, UserID: tm.alice.UserID, Role: shared.WorkspaceAdmin},
			{ID: uuid.NewV7(), WorkspaceID: zeta.ID, UserID: tm.bob.UserID, Role: shared.WorkspaceGuest},
		} {
			tm.store.active[m.ID] = m
		}

		err := tm.deactivate(deactivated(tm.alice))

		var se *shared.Error
		if !errors.Is(err, domain.ErrSoleAdmin) || !errors.As(err, &se) || !strings.Contains(se.Detail, "(acme, zeta)") ||
			!slices.Equal(tm.store.calls, inTxCalls("LockWorkspacesOf", "ListStandings")) || !tm.tx.rolledBack {
			t.Errorf("deactivate = %v after %q; want workspace.sole_admin naming acme, zeta, before the vetoers, rolled back",
				err, tm.store.calls)
		}
	})
	t.Run("the only admin, alone", func(t *testing.T) {
		tm := newTeam()
		for _, m := range []domain.Member{tm.bob, tm.carol} {
			delete(tm.store.active, m.ID)
		}
		tm.listed(tm.alice)

		if err := tm.deactivate(deactivated(tm.alice)); err != nil || len(tm.sub.ended) != 1 {
			t.Errorf("deactivate = %v, %d ends; want the membership ended", err, len(tm.sub.ended))
		}
	})
	t.Run("one of two admins", func(t *testing.T) {
		tm := newTeam()
		promoted := tm.bob
		promoted.Role = shared.WorkspaceAdmin
		tm.store.active[promoted.ID] = promoted
		tm.listed(tm.alice)

		if err := tm.deactivate(deactivated(tm.alice)); err != nil || len(tm.sub.ended) != 1 {
			t.Errorf("deactivate = %v, %d ends; want the membership ended", err, len(tm.sub.ended))
		}
	})
}

// A membership's vetoer refuses the whole deactivation, with its error,
// before any write.
func TestDeactivationVetoed(t *testing.T) {
	tm := newTeam()
	tm.vetoer.err = shared.NewError(shared.KindConflict, "test.vetoed", "Vetoed.")
	tm.listed(tm.bob)

	err := tm.deactivate(deactivated(tm.bob))

	if !errors.Is(err, tm.vetoer.err) || slices.ContainsFunc(tm.store.calls, isWrite) || len(tm.sub.ended) != 0 || !tm.tx.rolledBack {
		t.Errorf("deactivate = %v after %q; want the vetoer's error, no write, rolled back", err, tm.store.calls)
	}
}

// An account with no active membership passes untouched: nothing ends, so
// no registrant hears of it (v0.1 design 13.1, item 22). One whose last
// membership ended while its workspace was being locked is the same.
func TestDeactivationWithoutMemberships(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		tm := newTeam()

		err := tm.deactivate(app.Deactivated{UserID: tm.dana, Email: "dana@corp.com", At: now()})

		if err != nil || !slices.Equal(tm.store.calls, inTxCalls("LockWorkspacesOf", "ListWorkspacesOf "+tm.dana.String())) ||
			len(tm.vetoer.seen) != 0 || len(tm.sub.ended) != 0 {
			t.Errorf("deactivate = %v after %q; want the lock and the list alone", err, tm.store.calls)
		}
	})
	t.Run("ended while locking", func(t *testing.T) {
		tm := newTeam()
		tm.store.onLock = func() { delete(tm.store.active, tm.bob.ID) } // removed, committed before the lock
		d := tm.deactivation()

		err := tm.tx.WithinTx(context.Background(), func(ctx context.Context) error {
			return d.VetoDeactivation(ctx, deactivated(tm.bob))
		})

		if err != nil || !slices.Equal(tm.store.calls, inTxCalls("LockWorkspacesOf", "ListStandings")) || len(tm.vetoer.seen) != 0 {
			t.Errorf("veto = %v after %q; want no vetoer: the membership had ended", err, tm.store.calls)
		}
	})
}

// A subscriber's failure rolls the whole deactivation back.
func TestDeactivationSubscriberFails(t *testing.T) {
	tm := newTeam()
	tm.sub.err = errors.New("the subscriber failed")
	tm.listed(tm.bob)

	if err := tm.deactivate(deactivated(tm.bob)); !errors.Is(err, tm.sub.err) || !tm.tx.rolledBack {
		t.Errorf("deactivate = %v, rolled back %v; want the subscriber's error, rolled back", err, tm.tx.rolledBack)
	}
}
