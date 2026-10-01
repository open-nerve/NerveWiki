package workspace_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The module's part in a deactivation on a real database (M2/P4 design
// 3.1), called as identity calls it: in one transaction, after the
// account's row is locked, the vetoer, then the subscriber.

// deactivate deactivates who, whose address under the lock is email,
// through d.
func (f fixture) deactivate(d workspace.Deactivation, who uuid.UUID, email string) error {
	ctx := context.Background()
	return postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(ctx, func(ctx context.Context) error {
		if _, err := postgres.DB(ctx, f.pool).Exec(ctx, "SELECT 1 FROM users WHERE id = $1 FOR NO KEY UPDATE", who); err != nil {
			return err
		}
		x := workspace.Deactivated{UserID: who, Email: email, At: testNow()}
		if err := d.VetoDeactivation(ctx, x); err != nil {
			return err
		}
		// identity writes the account's row and revokes its sessions here.
		return d.AccountDeactivated(ctx, x)
	})
}

// newWorkspace inserts a workspace of slug whose only member is admin.
func (f fixture) newWorkspace(t *testing.T, slug string, admin uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	f.exec(t, "INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, $2, $3, $3, $4, $4)",
		id, slug, admin, testNow())
	f.exec(t, "INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) "+
		"VALUES ($1, $2, $3, 'admin', $3, $3, $4, $4)", uuid.NewV7(), id, admin, testNow())
	return id
}

// The deactivation ends every active membership of the account through the
// extension point: the vetoer under the workspaces' locks, before the
// write; the pending invitations of those workspaces to the address it was
// given deleted; the subscriber after, in the same transaction, with the
// workspaces by id, not by name as the account's list has them. Alone in a
// workspace, the account's admin membership ends too.
func TestADeactivationEndsTheMemberships(t *testing.T) {
	f := newFixture(t)
	abacus := f.newWorkspace(t, "abacus", f.bob) // after acme by id, before it by name
	gamma := f.newWorkspace(t, "gamma", f.alice)
	toBob, toBobElsewhere, toErin := f.invite(t, f.acme, "bob@corp.com", "admin"), f.invite(t, gamma, "bob@corp.com", "member"),
		f.invite(t, f.acme, "erin@corp.com", "member")
	v, s := &vetoer{t: t, f: f}, &subscriber{t: t, f: f}

	err := f.deactivate(workspace.NewDeactivation(f.pool, []workspace.MembershipEndVetoer{v}, []workspace.MembershipEndSubscriber{s}),
		f.bob, "bob@corp.com")

	if err != nil {
		t.Fatal(err)
	}
	if v.called != 1 || !v.inTx || !v.locked || v.endedAt != nil {
		t.Errorf("the vetoer: called %d, in tx %v, acme held %v, saw the end %v; want once, in it, held, before the end",
			v.called, v.inTx, v.locked, v.endedAt)
	}
	if len(s.ended) != 1 || s.endedAt == nil || !s.endedAt.Equal(testNow()) {
		t.Fatalf("the subscriber: %d calls, saw the end %v; want one, at %v", len(s.ended), s.endedAt, testNow())
	}
	if e := s.ended[0]; e.Cause != "deactivated" || e.UserID != f.bob || e.By != f.bob || !slices.Equal(e.WorkspaceIDs, []uuid.UUID{f.acme, abacus}) {
		t.Errorf("the end = %+v, want bob's of acme and abacus, by id, deactivated, by bob", e)
	}
	var active int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM workspace_members WHERE user_id = $1 AND ended_at IS NULL", f.bob).
		Scan(&active); err != nil || active != 0 {
		t.Errorf("bob has %d active memberships (%v), want none", active, err)
	}
	if at := f.invitationDeletedAt(t, toBob); at == nil || !at.Equal(testNow()) {
		t.Errorf("bob's invitation to acme deleted at %v, want %v", at, testNow())
	}
	for name, id := range map[string]uuid.UUID{"bob's to gamma, not his": toBobElsewhere, "erin's to acme": toErin} {
		if at := f.invitationDeletedAt(t, id); at != nil {
			t.Errorf("%s deleted at %v, want it pending", name, at)
		}
	}
}

// Rule two refuses the only admin of a workspace with other members, with
// workspace.sole_admin naming it, before the vetoer: nothing changes.
func TestADeactivationRefusedByRuleTwo(t *testing.T) {
	f := newFixture(t)
	f.newWorkspace(t, "solo", f.alice)
	v, s := &vetoer{t: t, f: f}, &subscriber{t: t, f: f}

	err := f.deactivate(workspace.NewDeactivation(f.pool, []workspace.MembershipEndVetoer{v}, []workspace.MembershipEndSubscriber{s}),
		f.alice, "alice@corp.com")

	var refusal interface{ ProblemCode() string }
	if !errors.As(err, &refusal) || refusal.ProblemCode() != "workspace.sole_admin" || !strings.Contains(err.Error(), "(acme)") {
		t.Errorf("deactivate alice = %v, want workspace.sole_admin naming acme alone", err)
	}
	var active int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM workspace_members WHERE ended_at IS NULL").Scan(&active); err != nil ||
		active != 3 || v.called != 0 || len(s.ended) != 0 {
		t.Errorf("%d active memberships, vetoer called %d, subscriber %d; want all 3 active, neither called", active, v.called, len(s.ended))
	}
}

// A membership end's vetoer refuses the deactivation with its code; a
// subscriber's failure rolls it back: nothing changes either way.
func TestADeactivationRollsBack(t *testing.T) {
	for _, tt := range []struct {
		name         string
		refuse, fail bool
		want         string
	}{{"vetoed", true, false, "The membership cannot end."}, {"the subscriber fails", false, true, "the subscriber failed"}} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			toBob := f.invite(t, f.acme, "bob@corp.com", "admin")
			v, s := &vetoer{t: t, f: f, refuse: tt.refuse}, &subscriber{t: t, f: f, fail: tt.fail}

			err := f.deactivate(workspace.NewDeactivation(f.pool, []workspace.MembershipEndVetoer{v}, []workspace.MembershipEndSubscriber{s}),
				f.bob, "bob@corp.com")

			if err == nil || err.Error() != tt.want {
				t.Errorf("deactivate bob = %v, want %q", err, tt.want)
			}
			if at, inv := f.endedAt(t, context.Background(), f.bobMembership), f.invitationDeletedAt(t, toBob); at != nil || inv != nil {
				t.Errorf("bob's membership ended at %v, his invitation deleted at %v; want both untouched", at, inv)
			}
		})
	}
}
