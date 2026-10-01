package bootstrap

import (
	"net/http"
	"strings"
	"testing"
)

// The workspace module's part in a deactivation reaches both of identity's
// (M1 handoff to M2, item 1; v0.1 design 13.1, item 21): the caller's own
// through the API, and the administrator's through the command line. The
// composition check proves only that serve and the command line reach the
// registrants; these prove they hand them over.

// isActive reports whether the account of email is active.
func (tm acmeTeam) isActive(t *testing.T, email string) bool {
	t.Helper()
	return count(t, tm.pool, "SELECT count(*) FROM users WHERE email = $1 AND is_active", email) == 1
}

// activeMembers counts acme's active members.
func (tm acmeTeam) activeMembers(t *testing.T) int {
	t.Helper()
	return count(t, tm.pool, `SELECT count(*) FROM workspace_members m JOIN workspaces w ON w.id = m.workspace_id
		WHERE w.slug = 'acme' AND m.ended_at IS NULL AND m.deleted_at IS NULL`)
}

// Rule two refuses the only admin of a workspace with other members, both
// ways, with the workspace named: the account stays active and signed in.
func TestRuleTwoRefusesBothDeactivations(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")

	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/me/deactivate", tm.tokens["alice"], "")
	if status != http.StatusConflict || problemCode(t, answer) != "workspace.sole_admin" || !strings.Contains(answer, "(acme)") {
		t.Errorf("POST /me/deactivate = %d %s, want 409 workspace.sole_admin naming acme", status, answer)
	}
	out, _, err := runUsers(t, tm.url, DeactivateUser("alice@example.com"))
	if err == nil || !strings.Contains(err.Error(), "the only admin of workspaces that have other members (acme)") || out != "" {
		t.Errorf("users deactivate = %q, %v; want the refusal naming acme, no output", out, err)
	}

	if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/me", tm.tokens["alice"], ""); status != http.StatusOK {
		t.Errorf("GET /me after both refusals = %d %s, want alice still signed in", status, answer)
	}
	if !tm.isActive(t, "alice@example.com") || tm.activeMembers(t) != 2 {
		t.Errorf("alice active %v, %d active members; want her active, both members", tm.isActive(t, "alice@example.com"), tm.activeMembers(t))
	}
}

// A deactivation rule two lets through ends the account's memberships: a
// member's through the API; then the only member's, alone in acme, through
// the command line.
func TestDeactivationsEndTheMemberships(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")

	if status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/me/deactivate", tm.tokens["bob"], ""); status != http.StatusNoContent {
		t.Fatalf("POST /me/deactivate as bob = %d %s, want 204", status, answer)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM workspace_members m JOIN users u ON u.id = m.user_id
		WHERE u.email = 'bob@example.com' AND m.ended_at IS NOT NULL AND m.updated_by_id = u.id`); n != 1 {
		t.Errorf("%d of bob's memberships ended by him, want his one", n)
	}
	if _, _, err := runUsers(t, tm.url, DeactivateUser("alice@example.com")); err != nil {
		t.Fatalf("users deactivate alice, alone in acme = %v", err)
	}
	if n := tm.activeMembers(t); n != 0 || tm.isActive(t, "alice@example.com") {
		t.Errorf("%d active members, alice active %v; want none, her deactivated", n, tm.isActive(t, "alice@example.com"))
	}
}
