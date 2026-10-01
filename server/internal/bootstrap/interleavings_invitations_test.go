package bootstrap

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The interleavings of the invitations (M2/P3 design 3.9), as P2's: the
// test holds the row both steps lock, sends them in turn, each once the one
// before waits for the row, and lets go. Interleaving 7 holds no row: its
// two invitations share the workspace's row FOR SHARE, and the later waits
// for the earlier's key in the unique index.

// invitation is an invitation's id and its link's token, as its creation
// answers them.
type invitation struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

// invite has alice invite name's address to acme as role, through the API.
func (tm acmeTeam) invite(t *testing.T, name, role string) invitation {
	t.Helper()
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/invitations", tm.tokens["alice"],
		`{"email":"`+name+`@example.com","role":"`+role+`"}`)
	var inv invitation
	if decodeAnswer(t, answer, &inv); status != http.StatusCreated {
		t.Fatalf("invite %s = %d %s", name, status, answer)
	}
	return inv
}

// accept is by's acceptance of inv.
func accept(by string, inv invitation) step {
	return request(by, http.MethodPost, "/api/v0/workspace-invitations/"+inv.ID+"/accept", `{"token":"`+inv.Token+`"}`)
}

// invitationState is what became of an invitation: pending, accepted, or
// deleted otherwise.
func (tm acmeTeam) invitationState(t *testing.T, inv invitation) string {
	t.Helper()
	var state string
	if err := tm.pool.QueryRow(context.Background(), `SELECT CASE WHEN deleted_at IS NULL THEN 'pending'
		WHEN accepted_at IS NOT NULL THEN 'accepted' ELSE 'deleted' END FROM workspace_invitations WHERE id = $1`, inv.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

// membership is what name's membership of acme is: active with its role,
// ended, deleted, or none.
func (tm acmeTeam) membership(t *testing.T, name string) string {
	t.Helper()
	var state string
	err := tm.pool.QueryRow(context.Background(), `SELECT coalesce((SELECT CASE WHEN m.deleted_at IS NOT NULL THEN 'deleted'
		WHEN m.ended_at IS NOT NULL THEN 'ended' ELSE m.role END
		FROM workspace_members m JOIN users u ON u.id = m.user_id JOIN workspaces w ON w.id = m.workspace_id
		WHERE w.slug = 'acme' AND split_part(u.email, '@', 1) = $1), 'none')`, name).Scan(&state)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// Dana accepts while alice deletes acme: whichever commits first, the other
// sees it. The deletion after the acceptance takes dana's new membership
// with it; the acceptance after the deletion finds no workspace
// (interleaving 4).
func TestAcceptingWhileTheWorkspaceIsDeleted(t *testing.T) {
	del := request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "")
	t.Run("the acceptance first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "guest")
		inv := tm.invite(t, "dana", "member")

		a, b := tm.interleave(t, accept("dana", inv), del)

		if !a.is(http.StatusOK, "") || !b.is(http.StatusNoContent, "") {
			t.Errorf("acceptance: %d %s; deletion: %d %s; want 200, then 204", a.status, a.code, b.status, b.code)
		}
		if m, i := tm.membership(t, "dana"), tm.invitationState(t, inv); m != "deleted" || i != "accepted" {
			t.Errorf("dana's membership %s, the invitation %s; want deleted with acme, accepted", m, i)
		}
	})
	t.Run("the deletion first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "guest")
		inv := tm.invite(t, "dana", "member")

		a, b := tm.interleave(t, del, accept("dana", inv))

		if !a.is(http.StatusNoContent, "") || !b.is(http.StatusNotFound, "workspace.invitation_not_found") {
			t.Errorf("deletion: %d %s; acceptance: %d %s; want 204, then 404 workspace.invitation_not_found", a.status, a.code, b.status, b.code)
		}
		if m, i := tm.membership(t, "dana"), tm.invitationState(t, inv); m != "none" || i != "deleted" {
			t.Errorf("dana's membership %s, the invitation %s; want none, deleted with acme", m, i)
		}
	})
}

// Dana accepts while alice withdraws the invitation: the second finds it no
// longer pending, under the locks (interleaving 5).
func TestAcceptingWhileTheInvitationIsWithdrawn(t *testing.T) {
	t.Run("the acceptance first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "guest")
		inv := tm.invite(t, "dana", "member")

		a, b := tm.interleave(t, accept("dana", inv), request("alice", http.MethodDelete, "/api/v0/workspace-invitations/"+inv.ID, ""))

		if !a.is(http.StatusOK, "") || !b.is(http.StatusNotFound, "workspace.invitation_not_found") {
			t.Errorf("acceptance: %d %s; withdrawal: %d %s; want 200, then 404 workspace.invitation_not_found", a.status, a.code, b.status, b.code)
		}
		if m, i := tm.membership(t, "dana"), tm.invitationState(t, inv); m != "member" || i != "accepted" {
			t.Errorf("dana's membership %s, the invitation %s; want member, accepted", m, i)
		}
	})
	t.Run("the withdrawal first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "guest")
		inv := tm.invite(t, "dana", "member")

		a, b := tm.interleave(t, request("alice", http.MethodDelete, "/api/v0/workspace-invitations/"+inv.ID, ""), accept("dana", inv))

		if !a.is(http.StatusNoContent, "") || !b.is(http.StatusNotFound, "workspace.invitation_not_found") {
			t.Errorf("withdrawal: %d %s; acceptance: %d %s; want 204, then 404 workspace.invitation_not_found", a.status, a.code, b.status, b.code)
		}
		if m, i := tm.membership(t, "dana"), tm.invitationState(t, inv); m != "none" || i != "deleted" {
			t.Errorf("dana's membership %s, the invitation %s; want none, deleted", m, i)
		}
	})
}

// Bob, a member with an invitation still pending to his address, accepts
// it while alice removes him. After the acceptance, his role is the one he
// had and the removal ends it; after the removal, the invitation went with
// his membership and cannot bring him back (interleaving 6).
func TestAcceptingWhileBeingRemoved(t *testing.T) {
	// bob is invited before he is a member: then he joins by another path
	// (an address changed to the one invited, or M2/P4's reactivation).
	setUp := func(t *testing.T) (acmeTeam, invitation, step) {
		tm := newAcmeTeam(t, "", "guest")
		inv := tm.invite(t, "bob", "admin")
		tm.join(t, "bob", "member")
		return tm, inv, request("alice", http.MethodDelete, "/api/v0/workspace-members/"+tm.members["bob"].String(), "")
	}
	t.Run("the acceptance first", func(t *testing.T) {
		tm, inv, removal := setUp(t)

		a, b := tm.interleave(t, accept("bob", inv), removal)

		if !a.is(http.StatusOK, "") || !b.is(http.StatusNoContent, "") {
			t.Errorf("acceptance: %d %s; removal: %d %s; want 200, then 204", a.status, a.code, b.status, b.code)
		}
		var joined struct{ Role string }
		if decodeAnswer(t, a.body, &joined); joined.Role != "member" {
			t.Errorf("the acceptance answered bob as %s, want the member he was", joined.Role)
		}
		if m, i := tm.membership(t, "bob"), tm.invitationState(t, inv); m != "ended" || i != "accepted" {
			t.Errorf("bob's membership %s, the invitation %s; want ended, accepted", m, i)
		}
	})
	t.Run("the removal first", func(t *testing.T) {
		tm, inv, removal := setUp(t)

		a, b := tm.interleave(t, removal, accept("bob", inv))

		if !a.is(http.StatusNoContent, "") || !b.is(http.StatusNotFound, "workspace.invitation_not_found") {
			t.Errorf("removal: %d %s; acceptance: %d %s; want 204, then 404 workspace.invitation_not_found", a.status, a.code, b.status, b.code)
		}
		if m, i := tm.membership(t, "bob"), tm.invitationState(t, inv); m != "ended" || i != "deleted" {
			t.Errorf("bob's membership %s, the invitation %s; want ended, deleted with it", m, i)
		}
	})
}

// Alice and bob, both admins, invite erin at once. Their invitations hold
// acme's row FOR SHARE together. A trigger of the test's database keeps the
// earlier's transaction open once its row is in, waiting for a row of a
// table of its own that the test holds; the later waits for the earlier's
// key in the unique index, and once the earlier commits is refused 422
// duplicate on email (interleaving 7).
func TestTwoAdminsInvitingOneAddressAtOnce(t *testing.T) {
	orders(t, "alice", "bob", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "admin", "guest")
		ctx := context.Background()
		if _, err := tm.pool.Exec(ctx, `CREATE TABLE interleaving_pause (id int PRIMARY KEY);
			INSERT INTO interleaving_pause VALUES (1);
			CREATE FUNCTION pause_after_invitation() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS
			$$ BEGIN PERFORM 1 FROM interleaving_pause WHERE id = 1 FOR SHARE; RETURN NEW; END $$;
			CREATE TRIGGER pause_after_invitation AFTER INSERT ON workspace_invitations
			FOR EACH ROW EXECUTE FUNCTION pause_after_invitation()`); err != nil {
			t.Fatal(err)
		}
		holder, err := tm.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = holder.Rollback(ctx) }()
		if _, err := holder.Exec(ctx, "SELECT 1 FROM interleaving_pause FOR UPDATE"); err != nil {
			t.Fatal(err)
		}
		invite := func(by string) func() answer {
			return tm.sender(t, request(by, http.MethodPost, "/api/v0/workspaces/acme/invitations", `{"email":"erin@example.com","role":"member"}`))
		}
		earlier, later := make(chan answer, 1), make(chan answer, 1)
		send := invite(first)
		go func() { earlier <- send() }()
		pgtest.WaitForLockWaitsOn(t, tm.pool, "interleaving_pause", 1, interleavingWait)
		send = invite(second)
		go func() { later <- send() }()
		pgtest.WaitForKeyWaitOn(t, tm.pool, "workspace_invitations", 1, interleavingWait)
		if err := holder.Rollback(ctx); err != nil {
			t.Fatal(err)
		}

		var a, b answer
		for _, got := range []struct {
			ch <-chan answer
			to *answer
		}{{earlier, &a}, {later, &b}} {
			select {
			case *got.to = <-got.ch:
			case <-time.After(interleavingWait):
				t.Fatal("an invitation did not answer")
			}
			if got.to.res == nil {
				t.Fatalf("an invitation failed: %v", got.to.err)
			}
			tm.contract.CheckResponse(t, got.to.req, got.to.res)
		}
		var problem struct {
			Errors []struct{ Field, Code string }
		}
		decodeAnswer(t, b.body, &problem)
		if !a.is(http.StatusCreated, "") || !b.is(http.StatusUnprocessableEntity, "validation_failed") ||
			len(problem.Errors) != 1 || problem.Errors[0].Field != "email" || problem.Errors[0].Code != "duplicate" {
			t.Errorf("%s invited: %d %s; %s: %d %s %+v; want 201, then 422 duplicate on email", first, a.status, a.code, second, b.status, b.code, problem.Errors)
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM workspace_invitations i JOIN users u ON u.id = i.created_by_id
			WHERE i.email = 'erin@example.com' AND i.deleted_at IS NULL AND u.email = $1`, first+"@example.com"); n != 1 {
			t.Errorf("%d pending invitations of erin by %s, want 1", n, first)
		}
	})
}

// Dana accepts while the server's administrator changes her address
// (nervewiki users set-email, in process). Both queue on her account's row:
// after the change the acceptance reads the new address under its lock,
// not the one invited; before it, she joins and the change follows
// (interleaving 8).
func TestAcceptingWhileTheAddressChanges(t *testing.T) {
	orders(t, "dana", "the administrator", func(t *testing.T, first, _ string) {
		tm := newAcmeTeam(t, "member", "guest")
		inv := tm.invite(t, "dana", "member")
		setEmail := step{by: "the administrator", command: func() error {
			return Users(context.Background(), testConfig(t, tm.url, false), io.Discard, io.Discard, SetEmail("dana@example.com", "dana@elsewhere.com"))
		}}
		danasRow := held{"users", "SELECT 1 FROM users WHERE email = 'dana@example.com' FOR NO KEY UPDATE"}

		steps := [2]step{accept("dana", inv), setEmail}
		if first != "dana" {
			steps = [2]step{setEmail, accept("dana", inv)}
		}

		a, b := tm.interleaveOn(t, danasRow, steps[0], steps[1])

		acceptance, changed := a, b.err
		if first != "dana" {
			acceptance, changed = b, a.err
		}

		if changed != nil {
			t.Errorf("set-email = %v", changed)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM users WHERE email = 'dana@elsewhere.com'"); n != 1 {
			t.Errorf("%d accounts of dana@elsewhere.com, want dana's", n)
		}
		member, state, want := "member", "accepted", answer{status: http.StatusOK}
		if first != "dana" {
			member, state, want = "none", "pending", answer{status: http.StatusForbidden, code: "workspace.invitation_email_mismatch"}
		}
		if !acceptance.is(want.status, want.code) {
			t.Errorf("acceptance = %d %s, want %d %s", acceptance.status, acceptance.code, want.status, want.code)
		}
		if m, i := tm.membership(t, "dana"), tm.invitationState(t, inv); m != member || i != state {
			t.Errorf("dana's membership %s, the invitation %s; want %s, %s", m, i, member, state)
		}
	})
}
