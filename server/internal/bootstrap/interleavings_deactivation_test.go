package bootstrap

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The interleavings of deactivation (M2/P4 design 3.5), as P2's and P3's:
// the test holds the row both steps lock, sends them in turn, each once the
// one before waits for the row, and lets go. A deactivation locks the
// account's row first, then the workspaces of its memberships: a path that
// gives the account access waits for it on the account's row, one that
// ends a membership on the workspace's.

// deactivate is by's deactivation of its own account, through the API.
func deactivate(by string) step {
	return request(by, http.MethodPost, "/api/v0/me/deactivate", "")
}

// deactivateByCommand is the server administrator's deactivation of name's
// account, run in process; its configuration is built here, on the test's
// goroutine.
func (tm acmeTeam) deactivateByCommand(t *testing.T, name string) step {
	t.Helper()
	cfg := testConfig(t, tm.url, false)
	return step{by: "the administrator", command: func() error {
		return Users(context.Background(), cfg, io.Discard, io.Discard, DeactivateUser(name+"@example.com"))
	}}
}

// accountRow is name's account row.
func accountRow(name string) held {
	return held{"users", "SELECT 1 FROM users WHERE email = '" + name + "@example.com' FOR NO KEY UPDATE"}
}

// leave has name leave acme through the API, and returns when the
// membership ended.
func (tm acmeTeam) leave(t *testing.T, name string) time.Time {
	t.Helper()
	if status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/leave", tm.tokens[name], ""); status != http.StatusNoContent {
		t.Fatalf("%s leaves acme = %d %s", name, status, answer)
	}
	return tm.endedAt(t, name)
}

// endedAt is when name's membership of acme ended.
func (tm acmeTeam) endedAt(t *testing.T, name string) time.Time {
	t.Helper()
	var at time.Time
	if err := tm.pool.QueryRow(context.Background(), `SELECT m.ended_at FROM workspace_members m JOIN users u ON u.id = m.user_id
		WHERE u.email = $1`, name+"@example.com").Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

// endedBy is whose account last wrote name's membership of acme: who ended
// it.
func (tm acmeTeam) endedBy(t *testing.T, name string) string {
	t.Helper()
	var by string
	if err := tm.pool.QueryRow(context.Background(), `SELECT split_part(b.email, '@', 1) FROM workspace_members m
		JOIN users u ON u.id = m.user_id JOIN users b ON b.id = m.updated_by_id WHERE u.email = $1`, name+"@example.com").Scan(&by); err != nil {
		t.Fatal(err)
	}
	return by
}

// Two admins are deactivated at once, one through the API and one by the
// administrator's command, with carol a member: the second, under acme's
// lock, counts itself its only admin beside carol, and rule two refuses it
// (interleaving 9).
func TestTwoAdminsDeactivatedAtOnceLeaveOne(t *testing.T) {
	orders(t, "alice", "bob", func(t *testing.T, first, _ string) {
		tm := newAcmeTeam(t, "admin", "member")
		byAPI, byCommand := deactivate("alice"), tm.deactivateByCommand(t, "bob")
		steps := [2]step{byAPI, byCommand}
		if first != "alice" {
			steps = [2]step{byCommand, byAPI}
		}

		a, b := tm.interleave(t, steps[0], steps[1])

		api, command := a, b
		if first != "alice" {
			api, command = b, a
		}
		deactivated, kept := "alice", "bob"
		apiWant, commandRefused := answer{status: http.StatusNoContent}, true
		if first != "alice" {
			deactivated, kept = "bob", "alice"
			apiWant, commandRefused = answer{status: http.StatusConflict, code: "workspace.sole_admin"}, false
		}
		refusedByRuleTwo := command.err != nil && strings.Contains(command.err.Error(), "(acme)")
		if !api.is(apiWant.status, apiWant.code) || refusedByRuleTwo != commandRefused || (!commandRefused && command.err != nil) {
			t.Errorf("alice's through the API = %d %s, bob's by the command = %v; want %d %s, the command refused by rule two %v",
				api.status, api.code, command.err, apiWant.status, apiWant.code, commandRefused)
		}
		if tm.isActive(t, deactivated+"@example.com") || !tm.isActive(t, kept+"@example.com") || tm.activeAdmins(t) != 1 ||
			tm.membership(t, kept) != "admin" || tm.membership(t, deactivated) != "ended" {
			t.Errorf("%s deactivated, %s kept as acme's admin: active %v and %v, %d admins", deactivated, kept,
				tm.isActive(t, deactivated+"@example.com"), tm.isActive(t, kept+"@example.com"), tm.activeAdmins(t))
		}
	})
}

// Dana deactivates while accepting an invitation, once to a new membership
// and once to the membership she left: the acceptance first, the
// deactivation ends what it gave; the deactivation first, the acceptance
// finds the account deactivated and the invitation stays (interleaving 10).
func TestDeactivatingWhileAccepting(t *testing.T) {
	for _, joining := range []string{"a new membership", "the ended one"} {
		t.Run(joining, func(t *testing.T) {
			orders(t, "the acceptance", "the deactivation", func(t *testing.T, first, _ string) {
				tm := newAcmeTeam(t, "member", "")
				before := "none"
				if joining == "the ended one" {
					tm.join(t, "dana", "guest")
					tm.leave(t, "dana")
					before = "ended"
				}
				inv := tm.invite(t, "dana", "member")
				steps := [2]step{accept("dana", inv), deactivate("dana")}
				if first != "the acceptance" {
					steps = [2]step{deactivate("dana"), accept("dana", inv)}
				}

				a, b := tm.interleaveOn(t, accountRow("dana"), steps[0], steps[1])

				acceptance, deactivation := a, b
				if first != "the acceptance" {
					acceptance, deactivation = b, a
				}
				acceptWant, member, state := answer{status: http.StatusOK}, "ended", "accepted"
				if first != "the acceptance" {
					acceptWant, member, state = answer{status: http.StatusForbidden, code: "identity.account_deactivated"}, before, "pending"
				}
				if !acceptance.is(acceptWant.status, acceptWant.code) || !deactivation.is(http.StatusNoContent, "") {
					t.Errorf("acceptance = %d %s, deactivation = %d %s; want %d %s, then 204", acceptance.status, acceptance.code,
						deactivation.status, deactivation.code, acceptWant.status, acceptWant.code)
				}
				if m, i := tm.membership(t, "dana"), tm.invitationState(t, inv); m != member || i != state || tm.isActive(t, "dana@example.com") {
					t.Errorf("dana's membership %s, the invitation %s; want %s, %s, dana deactivated", m, i, member, state)
				}
			})
		})
	}
}

// Dana deactivates while creating a workspace: the creation first, alone in
// it she may go, and her membership ends; the deactivation first, the
// creation finds the account deactivated (interleaving 11).
func TestDeactivatingWhileCreatingAWorkspace(t *testing.T) {
	orders(t, "the creation", "the deactivation", func(t *testing.T, first, _ string) {
		tm := newAcmeTeam(t, "", "")
		create := request("dana", http.MethodPost, "/api/v0/workspaces", `{"name":"Beta","slug":"beta"}`)
		steps := [2]step{create, deactivate("dana")}
		if first != "the creation" {
			steps = [2]step{deactivate("dana"), create}
		}

		a, b := tm.interleaveOn(t, accountRow("dana"), steps[0], steps[1])

		creation, deactivation := a, b
		if first != "the creation" {
			creation, deactivation = b, a
		}
		createWant, betas := answer{status: http.StatusCreated}, 1
		if first != "the creation" {
			createWant, betas = answer{status: http.StatusForbidden, code: "identity.account_deactivated"}, 0
		}
		if !creation.is(createWant.status, createWant.code) || !deactivation.is(http.StatusNoContent, "") {
			t.Errorf("creation = %d %s, deactivation = %d %s; want %d %s, then 204", creation.status, creation.code,
				deactivation.status, deactivation.code, createWant.status, createWant.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM workspaces WHERE slug = 'beta'"); n != betas {
			t.Errorf("%d workspaces beta, want %d", n, betas)
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM workspace_members m JOIN users u ON u.id = m.user_id
			WHERE u.email = 'dana@example.com' AND m.ended_at IS NULL`); n != 0 || tm.isActive(t, "dana@example.com") {
			t.Errorf("dana has %d active memberships; want none, her account deactivated", n)
		}
	})
}

// Dana, who left acme, deactivates while the administrator reactivates her
// membership: the reactivation first, the deactivation ends it again,
// later; the deactivation first, the command finds the account deactivated
// and the membership stays as it ended (interleaving 12).
func TestDeactivatingWhileReactivatingTheMembership(t *testing.T) {
	orders(t, "the reactivation", "the deactivation", func(t *testing.T, first, _ string) {
		tm := newAcmeTeam(t, "member", "")
		tm.join(t, "dana", "member")
		left := tm.leave(t, "dana")
		cfg := testConfig(t, tm.url, false)
		reactivate := step{by: "the administrator", command: func() error {
			return Workspaces(context.Background(), cfg, io.Discard, io.Discard, ReactivateMember("acme", "dana@example.com"))
		}}
		steps := [2]step{reactivate, deactivate("dana")}
		if first != "the reactivation" {
			steps = [2]step{deactivate("dana"), reactivate}
		}

		a, b := tm.interleaveOn(t, accountRow("dana"), steps[0], steps[1])

		reactivation, deactivation := a, b
		if first != "the reactivation" {
			reactivation, deactivation = b, a
		}
		refused := reactivation.err != nil && strings.Contains(reactivation.err.Error(), "This account is deactivated.")
		if (first == "the reactivation") == refused || !deactivation.is(http.StatusNoContent, "") {
			t.Errorf("reactivation = %v, deactivation = %d %s; want the reactivation through only when first, then 204",
				reactivation.err, deactivation.status, deactivation.code)
		}
		ended, endedAgain := tm.endedAt(t, "dana"), first == "the reactivation"
		if tm.membership(t, "dana") != "ended" || ended.After(left) != endedAgain || tm.isActive(t, "dana@example.com") {
			t.Errorf("dana's membership %s, ended at %v after leaving at %v; want ended, again later %v", tm.membership(t, "dana"),
				ended, left, endedAgain)
		}
	})
}

// Alice removes bob while he deactivates: the removal first, the
// deactivation finds under acme's lock no membership to end; the
// deactivation first, the removal finds none to remove (interleaving 13).
func TestDeactivatingWhileBeingRemoved(t *testing.T) {
	orders(t, "the removal", "the deactivation", func(t *testing.T, first, _ string) {
		tm := newAcmeTeam(t, "member", "")
		remove := request("alice", http.MethodDelete, "/api/v0/workspace-members/"+tm.members["bob"].String(), "")
		steps := [2]step{remove, deactivate("bob")}
		if first != "the removal" {
			steps = [2]step{deactivate("bob"), remove}
		}

		a, b := tm.interleave(t, steps[0], steps[1])

		removal, deactivation := a, b
		if first != "the removal" {
			removal, deactivation = b, a
		}
		removeWant, by := answer{status: http.StatusNoContent}, "alice"
		if first != "the removal" {
			removeWant, by = answer{status: http.StatusNotFound, code: "workspace.member_not_found"}, "bob"
		}
		if !removal.is(removeWant.status, removeWant.code) || !deactivation.is(http.StatusNoContent, "") {
			t.Errorf("removal = %d %s, deactivation = %d %s; want %d %s, and 204", removal.status, removal.code,
				deactivation.status, deactivation.code, removeWant.status, removeWant.code)
		}
		if m, endedBy := tm.membership(t, "bob"), tm.endedBy(t, "bob"); m != "ended" || endedBy != by || tm.isActive(t, "bob@example.com") {
			t.Errorf("bob's membership %s, ended by %s; want ended by %s, bob deactivated", m, endedBy, by)
		}
	})
}

// Alice, acme's only admin and alone in it, deactivates while dana joins:
// by an invitation as a guest, to a new membership; as a member, to the
// guest's membership she left; by the administrator's reactivate-member of
// it; and as an admin. The joining first, dana is in acme and rule two
// refuses alice; the deactivation first, acme has no admin, and rule three
// refuses dana. An admin joins either way, and alice goes (interleaving 14).
func TestSoleAdminDeactivatingWhileOneJoins(t *testing.T) {
	for _, tt := range []struct {
		name, role string
		left       bool // dana had joined acme and left
		byCommand  bool
	}{
		{"a guest, newly", "guest", false, false},
		{"a member, again", "member", true, false},
		{"by reactivate-member", "guest", true, true},
		{"an admin", "admin", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			orders(t, "the joining", "the deactivation", func(t *testing.T, first, _ string) {
				tm := newAcmeTeam(t, "", "")
				before := "none"
				if tt.left {
					tm.join(t, "dana", "guest")
					tm.leave(t, "dana")
					before = "ended"
				}
				var inv invitation
				var join step
				if tt.byCommand {
					join = tm.reactivateByCommand(t, "dana")
				} else {
					inv = tm.invite(t, "dana", tt.role)
					join = accept("dana", inv)
				}
				steps := [2]step{join, deactivate("alice")}
				if first != "the joining" {
					steps = [2]step{deactivate("alice"), join}
				}

				a, b := tm.interleave(t, steps[0], steps[1])

				joining, deactivation := a, b
				if first != "the joining" {
					joining, deactivation = b, a
				}
				joined := first == "the joining" || tt.role == "admin"
				deactivated := first != "the joining" || tt.role == "admin"
				joiningOK := joining.joined(tt.byCommand)
				if !joined {
					joiningOK = joining.refusedForNoAdmin(tt.byCommand)
				}
				deactivationOK := deactivation.is(http.StatusNoContent, "")
				if !deactivated {
					deactivationOK = deactivation.is(http.StatusConflict, "workspace.sole_admin")
				}
				if !joiningOK || !deactivationOK {
					t.Errorf("the joining = %d %s %v, the deactivation = %d %s; want dana in acme %v (else rule three), alice deactivated %v (else rule two)",
						joining.status, joining.code, joining.err, deactivation.status, deactivation.code, joined, deactivated)
				}
				member, state := before, "pending"
				if joined {
					member, state = tt.role, "accepted"
				}
				if m := tm.membership(t, "dana"); m != member || tm.isActive(t, "alice@example.com") == deactivated {
					t.Errorf("dana's membership %s, alice active %v; want %s, %v", m, tm.isActive(t, "alice@example.com"), member, !deactivated)
				}
				if !tt.byCommand && tm.invitationState(t, inv) != state {
					t.Errorf("the invitation %s, want %s", tm.invitationState(t, inv), state)
				}
				if n := tm.activeAdmins(t); joined != (n > 0) {
					t.Errorf("acme has %d active admins with dana in it %v; want one whenever anyone is", n, joined)
				}
			})
		})
	}
}

// reactivateByCommand is the server administrator's reactivate-member of
// name's membership of acme, run in process.
func (tm acmeTeam) reactivateByCommand(t *testing.T, name string) step {
	t.Helper()
	cfg := testConfig(t, tm.url, false)
	return step{by: "the administrator", command: func() error {
		return Workspaces(context.Background(), cfg, io.Discard, io.Discard, ReactivateMember("acme", name+"@example.com"))
	}}
}

// joined reports whether a joining went through: a command without error,
// an acceptance with 200.
func (a answer) joined(byCommand bool) bool {
	if byCommand {
		return a.err == nil
	}
	return a.is(http.StatusOK, "")
}

// refusedForNoAdmin reports whether rule three refused a joining.
func (a answer) refusedForNoAdmin(byCommand bool) bool {
	if byCommand {
		return a.err != nil && strings.Contains(a.err.Error(), "The workspace has no admin")
	}
	return a.is(http.StatusConflict, "workspace.no_admin")
}
