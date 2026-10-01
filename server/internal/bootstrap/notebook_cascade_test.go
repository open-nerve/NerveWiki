package bootstrap

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The notebook module's part in the end and the restore of a workspace
// membership reaches it through every path that ends or restores one (M2
// handoff to M3, item 1; M3/P3 design 3.2): the removal, leaving, the
// deactivation through the API and the command line, the accepted
// invitation and reactivate-member. Each runs the whole program.

// cascadeTeam is acme with bob and carol members: bob is the only admin of
// plans, private, with carol a reader, and of solo, his alone; alice is
// the admin of wiki, open to the workspace, with bob an editor.
type cascadeTeam struct {
	acmeTeam
	plans, solo, wiki string
}

func newCascadeTeam(t *testing.T) cascadeTeam {
	t.Helper()
	tm := cascadeTeam{acmeTeam: newAcmeTeam(t, "member", "member")}
	tm.plans, tm.solo, tm.wiki = tm.createNotebook(t, "bob", "Plans"), tm.createNotebook(t, "bob", "Solo"), tm.createNotebook(t, "alice", "Wiki")
	tm.addNotebookMember(t, "bob", tm.plans, "carol", "reader")
	tm.addNotebookMember(t, "alice", tm.wiki, "bob", "editor")
	if status, answer := ask(t, tm.contract, http.MethodPatch, tm.base+"/api/v0/notebooks/"+tm.wiki, tm.tokens["alice"],
		`{"workspace_access":"viewer"}`); status != http.StatusOK {
		t.Fatalf("open wiki = %d %s", status, answer)
	}
	return tm
}

// owner is who notebook id is ownerless of and since when, by name; "" when
// it is owned.
func (tm acmeTeam) owner(t *testing.T, id string) (formerOwner string, since time.Time) {
	t.Helper()
	var name *string
	var at *time.Time
	if err := tm.pool.QueryRow(context.Background(), `SELECT split_part(u.email, '@', 1), n.ownerless_since FROM notebooks n
		LEFT JOIN users u ON u.id = n.former_owner_id WHERE n.id = $1`, id).Scan(&name, &at); err != nil {
		t.Fatal(err)
	}
	if name == nil {
		return "", time.Time{}
	}
	return *name, *at
}

// notebookMembership is name's membership of notebook id: its role while
// active, or "ended by <name> at <time>".
func (tm acmeTeam) notebookMembership(t *testing.T, id, name string) string {
	t.Helper()
	var state string
	if err := tm.pool.QueryRow(context.Background(), `SELECT CASE WHEN m.ended_at IS NULL THEN m.role
		ELSE 'ended by ' || split_part(b.email, '@', 1) || ' at ' || to_char(m.ended_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US') END
		FROM notebook_members m JOIN users u ON u.id = m.user_id JOIN users b ON b.id = m.updated_by_id
		WHERE m.notebook_id = $1 AND u.email = $2`, id, name+"@example.com").Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

// endedState is how notebookMembership writes a membership ended by by at
// at.
func endedState(by string, at time.Time) string {
	return "ended by " + by + " at " + at.UTC().Format("2006-01-02T15:04:05.000000")
}

// A workspace admin removes bob: rule two does not refuse it; his notebook
// memberships end with his workspace membership, at its time, by her; the
// notebooks he was the only admin of become ownerless, alone or not, and
// carol still reads plans.
func TestRemovingTheOnlyAdminLeavesHisNotebooksOwnerless(t *testing.T) {
	tm := newCascadeTeam(t)

	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/workspace-members/"+tm.members["bob"].String(),
		tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("remove bob = %d %s", status, answer)
	}

	ended := tm.endedAt(t, "bob")
	for _, id := range []string{tm.plans, tm.solo} {
		if owner, since := tm.owner(t, id); owner != "bob" || !since.Equal(ended) {
			t.Errorf("%s ownerless of %q since %v, want bob's since %v", id, owner, since, ended)
		}
	}
	for _, id := range []string{tm.plans, tm.solo, tm.wiki} {
		if got := tm.notebookMembership(t, id, "bob"); got != endedState("alice", ended) {
			t.Errorf("bob's membership of %s %q, want %q", id, got, endedState("alice", ended))
		}
	}
	if owner, _ := tm.owner(t, tm.wiki); owner != "" || tm.notebookMembership(t, tm.plans, "carol") != "reader" {
		t.Errorf("wiki ownerless of %q, carol %q in plans; want wiki owned, carol a reader", owner, tm.notebookMembership(t, tm.plans, "carol"))
	}
	if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+tm.plans, tm.tokens["carol"], ""); status != http.StatusOK {
		t.Errorf("GET plans as carol = %d %s, want 200: an ownerless notebook is seen as before", status, answer)
	}
	checkNotebooks(t, tm.pool)
}

// Rule two refuses bob, plans' only admin beside carol, on every path he
// takes himself, counting plans in acme; then, carol gone from plans, he
// leaves, and plans and solo, his alone, become ownerless.
func TestRuleTwoRefusesTheOnlyAdminLeavingHimself(t *testing.T) {
	tm := newCascadeTeam(t)
	reason := "The account is the only admin of notebooks with other members (1 in acme)"

	for _, path := range []string{"/api/v0/workspaces/acme/leave", "/api/v0/me/deactivate"} {
		if status, answer := ask(t, tm.contract, http.MethodPost, tm.base+path, tm.tokens["bob"], ""); status != http.StatusConflict ||
			problemCode(t, answer) != "notebook.sole_admin" || !strings.Contains(answer, reason) {
			t.Errorf("POST %s as bob = %d %s, want 409 notebook.sole_admin counting plans", path, status, answer)
		}
	}
	if out, _, err := runUsers(t, tm.url, DeactivateUser("bob@example.com")); err == nil || !strings.Contains(err.Error(), reason) || out != "" {
		t.Errorf("users deactivate bob = %q, %v; want the refusal counting plans", out, err)
	}
	if tm.membership(t, "bob") != "member" || !tm.isActive(t, "bob@example.com") || tm.notebookMembership(t, tm.plans, "bob") != "admin" {
		t.Fatalf("bob %s of acme, active %v, %s of plans; want all as before", tm.membership(t, "bob"), tm.isActive(t, "bob@example.com"),
			tm.notebookMembership(t, tm.plans, "bob"))
	}

	if status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+tm.plans+"/leave", tm.tokens["carol"], ""); status != http.StatusNoContent {
		t.Fatalf("carol leaves plans = %d %s", status, answer)
	}
	ended := tm.leave(t, "bob")

	for _, id := range []string{tm.plans, tm.solo} {
		if owner, since := tm.owner(t, id); owner != "bob" || !since.Equal(ended) || tm.notebookMembership(t, id, "bob") != endedState("bob", ended) {
			t.Errorf("%s ownerless of %q since %v, bob %q; want bob's since %v, his membership ended by him", id, owner, since,
				tm.notebookMembership(t, id, "bob"), ended)
		}
	}
	checkNotebooks(t, tm.pool)
}

// A deactivation rule two lets through ends the account's notebook
// memberships, by the account: carol's own through the API, alone in her
// notebook; dana's by the administrator's command.
func TestDeactivationsLeaveTheirNotebooksOwnerless(t *testing.T) {
	tm := newCascadeTeam(t)
	tm.join(t, "dana", "member")
	carols, danas := tm.createNotebook(t, "carol", "Carol's"), tm.createNotebook(t, "dana", "Dana's")

	if status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/me/deactivate", tm.tokens["carol"], ""); status != http.StatusNoContent {
		t.Fatalf("POST /me/deactivate as carol = %d %s", status, answer)
	}
	if _, _, err := runUsers(t, tm.url, DeactivateUser("dana@example.com")); err != nil {
		t.Fatalf("users deactivate dana: %v", err)
	}

	for _, c := range []struct{ name, id string }{{"carol", carols}, {"dana", danas}} {
		ended := tm.endedAt(t, c.name)
		if owner, since := tm.owner(t, c.id); owner != c.name || !since.Equal(ended) || tm.notebookMembership(t, c.id, c.name) != endedState(c.name, ended) {
			t.Errorf("%s's notebook ownerless of %q since %v, %q; want hers since %v, ended by her", c.name, owner, since,
				tm.notebookMembership(t, c.id, c.name), ended)
		}
	}
	if got := tm.notebookMembership(t, tm.plans, "carol"); got != endedState("carol", tm.endedAt(t, "carol")) {
		t.Errorf("carol's membership of plans %q, want it ended with hers of acme", got)
	}
	checkNotebooks(t, tm.pool)
}

// returnedEvents are acme's audit events of notebooks returned to name, by
// name, as "<notebook> by <actor>", oldest first.
func (tm acmeTeam) returnedEvents(t *testing.T, name string) []string {
	t.Helper()
	rows, err := tm.pool.Query(context.Background(), `SELECT e.notebook_name || ' by ' || split_part(a.email, '@', 1) FROM notebook_audit_events e
		JOIN users o ON o.id = e.former_owner_id JOIN users a ON a.id = e.created_by_id
		WHERE e.action = 'returned' AND o.email = $1 ORDER BY e.created_at, e.notebook_name`, name+"@example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var events []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

// Bob, removed, comes back by an invitation, as a guest: plans and solo
// return to him, recorded as returned by him; wiki, which has its admin,
// does not take him back.
func TestAcceptingAnInvitationReturnsTheNotebooks(t *testing.T) {
	tm := newCascadeTeam(t)
	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/workspace-members/"+tm.members["bob"].String(),
		tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("remove bob = %d %s", status, answer)
	}
	inv := tm.invite(t, "bob", "guest")

	if status, answer := ask(t, tm.contract, accept("bob", inv).method, tm.base+accept("bob", inv).path, tm.tokens["bob"],
		accept("bob", inv).body); status != http.StatusOK {
		t.Fatalf("bob accepts = %d %s", status, answer)
	}

	for _, id := range []string{tm.plans, tm.solo} {
		if owner, _ := tm.owner(t, id); owner != "" || tm.notebookMembership(t, id, "bob") != "admin" {
			t.Errorf("%s ownerless of %q, bob %q; want it his again", id, owner, tm.notebookMembership(t, id, "bob"))
		}
	}
	if got := tm.notebookMembership(t, tm.wiki, "bob"); !strings.HasPrefix(got, "ended") {
		t.Errorf("bob's membership of wiki %q, want it still ended", got)
	}
	if got := tm.returnedEvents(t, "bob"); strings.Join(got, ", ") != "Plans by bob, Solo by bob" {
		t.Errorf("returned events %q, want plans and solo by bob", got)
	}
	checkNotebooks(t, tm.pool)
}

// The administrator's reactivate-member returns them too, and prints how
// many.
func TestReactivateMemberReturnsTheNotebooks(t *testing.T) {
	tm := newCascadeTeam(t)
	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/workspace-members/"+tm.members["bob"].String(),
		tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("remove bob = %d %s", status, answer)
	}

	out, _, err := runWorkspaces(t, tm.url, ReactivateMember("acme", "bob@example.com"))

	if err != nil || !strings.HasSuffix(out, "; ownerless notebooks returned: 2\n") {
		t.Errorf("reactivate-member = %q, %v; want 2 notebooks returned", out, err)
	}
	if owner, _ := tm.owner(t, tm.plans); owner != "" || tm.notebookMembership(t, tm.plans, "bob") != "admin" {
		t.Errorf("plans ownerless of %q, bob %q; want it his again", owner, tm.notebookMembership(t, tm.plans, "bob"))
	}
	if got := tm.returnedEvents(t, "bob"); strings.Join(got, ", ") != "Plans by bob, Solo by bob" {
		t.Errorf("returned events %q, want plans and solo by bob, whom the command acts as", got)
	}
	checkNotebooks(t, tm.pool)
}

// A notebook a workspace admin took over or deleted while bob was away is
// not his to come back to: plans stays alice's, solo stays deleted; the
// audit events tell the three in order.
func TestATakenOverOrDeletedNotebookIsNotReturned(t *testing.T) {
	tm := newCascadeTeam(t)
	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/workspace-members/"+tm.members["bob"].String(),
		tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("remove bob = %d %s", status, answer)
	}
	if status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/ownerless-notebooks/"+tm.plans+"/take-over",
		tm.tokens["alice"], ""); status != http.StatusOK {
		t.Fatalf("alice takes plans over = %d %s", status, answer)
	}
	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/ownerless-notebooks/"+tm.solo,
		tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("alice deletes solo = %d %s", status, answer)
	}

	out, _, err := runWorkspaces(t, tm.url, ReactivateMember("acme", "bob@example.com"))

	if err != nil || !strings.HasSuffix(out, "; ownerless notebooks returned: 0\n") {
		t.Errorf("reactivate-member = %q, %v; want none returned", out, err)
	}
	if owner, _ := tm.owner(t, tm.plans); owner != "" || tm.notebookMembership(t, tm.plans, "alice") != "admin" ||
		!strings.HasPrefix(tm.notebookMembership(t, tm.plans, "bob"), "ended") {
		t.Errorf("plans ownerless of %q, alice %q, bob %q; want alice's", owner, tm.notebookMembership(t, tm.plans, "alice"),
			tm.notebookMembership(t, tm.plans, "bob"))
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE id = $1 AND deleted_at IS NOT NULL", tm.solo); n != 1 {
		t.Errorf("solo deleted %d, want it still deleted", n)
	}
	var events []string
	rows, err := tm.pool.Query(context.Background(), `SELECT e.action || ' ' || e.notebook_name || ' by ' || split_part(a.email, '@', 1)
		FROM notebook_audit_events e JOIN users a ON a.id = e.created_by_id ORDER BY e.created_at, e.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(events, ", ") != "taken_over Plans by alice, deleted Solo by alice" {
		t.Errorf("audit events %q, want the take-over and the deletion by alice", events)
	}
	checkNotebooks(t, tm.pool)
}
