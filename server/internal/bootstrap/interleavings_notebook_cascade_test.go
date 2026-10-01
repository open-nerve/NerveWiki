package bootstrap

import (
	"net/http"
	"strings"
	"testing"
)

// The interleavings of a workspace membership's end and restore with the
// notebooks' writes, and of the ownerless notebooks' (M3/P3 design 3.8),
// harnessed as the others: an end or a restore holds its workspace's row,
// which a notebook's write shares, so the test holds acme's row and the
// two run one after the other, in the order they came; two take-overs
// both share it, and the test holds the notebook's. Each ends by checking
// the notebooks' invariant.

// creation is by's creation of a notebook named name in acme.
func creation(by, name string) step {
	return request(by, http.MethodPost, "/api/v0/workspaces/acme/notebooks", `{"name":"`+name+`"}`)
}

// addition is by's addition of name to notebook id as role.
func (tm acmeTeam) addition(t *testing.T, by, id, name, role string) step {
	t.Helper()
	return request(by, http.MethodPost, "/api/v0/notebooks/"+id+"/members",
		`{"user_id":"`+tm.userID(t, name).String()+`","role":"`+role+`"}`)
}

// promotion is by's making notebook membership id an admin.
func promotion(by, id string) step {
	return request(by, http.MethodPatch, "/api/v0/notebook-members/"+id, `{"role":"admin"}`)
}

// takeOver is by's take-over of ownerless notebook id.
func takeOver(by, id string) step {
	return request(by, http.MethodPost, "/api/v0/ownerless-notebooks/"+id+"/take-over", "")
}

// ownerlessDeletion is by's deletion of ownerless notebook id.
func ownerlessDeletion(by, id string) step {
	return request(by, http.MethodDelete, "/api/v0/ownerless-notebooks/"+id, "")
}

// notInWorkspace reports whether an addition was refused for an account no
// member of the notebook's workspace.
func (a answer) notInWorkspace() bool {
	return a.is(http.StatusUnprocessableEntity, "validation_failed") && strings.Contains(a.body, `{"field":"user_id","code":"not_allowed"`)
}

// idOf is the id a step's answer carries.
func idOf(t *testing.T, a answer) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	decodeAnswer(t, a.body, &v)
	return v.ID
}

// planTeam is acme with bob and carol members, and plans, bob's, with carol
// a reader: its id, and carol's membership of it.
func planTeam(t *testing.T) (tm acmeTeam, plans, carols string) {
	t.Helper()
	tm = newAcmeTeam(t, "member", "member")
	plans = tm.createNotebook(t, "bob", "Plans")
	return tm, plans, tm.addNotebookMember(t, "bob", plans, "carol", "reader")
}

// ownerlessTeam is acme with bob an admin, and plans, carol's, ownerless
// since alice removed her: its id.
func ownerlessTeam(t *testing.T) (acmeTeam, string) {
	t.Helper()
	tm := newAcmeTeam(t, "admin", "member")
	plans := tm.createNotebook(t, "carol", "Plans")
	tm.send(t, tm.removal("alice", "carol"), http.StatusNoContent)
	return tm, plans
}

// Interleaving 20: bob adds carol to his notebook while alice removes her
// from acme. The removal first: the addition, under acme's lock, finds her
// no member of acme: 422, and no row. The addition first: the removal ends
// her new membership with hers of acme, by alice.
func TestAddingANotebookMemberAndRemovingThemFromTheWorkspace(t *testing.T) {
	t.Run("the removal first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "member")
		id := tm.createNotebook(t, "bob", "Notes")

		removed, added := tm.interleave(t, tm.removal("alice", "carol"), tm.addition(t, "bob", id, "carol", "reader"))

		if !removed.is(http.StatusNoContent, "") || !added.notInWorkspace() {
			t.Errorf("remove carol = %d %s, then add her = %d %s; want 204, then 422 user_id not_allowed", removed.status, removed.code,
				added.status, added.body)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM notebook_members WHERE user_id = $1", tm.userID(t, "carol")); n != 0 {
			t.Errorf("%d notebook memberships of carol's, want none", n)
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the addition first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "member")
		id := tm.createNotebook(t, "bob", "Notes")

		added, removed := tm.interleave(t, tm.addition(t, "bob", id, "carol", "reader"), tm.removal("alice", "carol"))

		if !added.is(http.StatusCreated, "") || !removed.is(http.StatusNoContent, "") {
			t.Errorf("add carol = %d %s, then remove her = %d %s; want 201, then 204", added.status, added.code, removed.status, removed.code)
		}
		if got, want := tm.notebookMembership(t, id, "carol"), endedState("alice", tm.endedAt(t, "carol")); got != want {
			t.Errorf("carol's membership of the notebook %q, want %q", got, want)
		}
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 21: bob deactivates his account while he creates a
// notebook; the deactivation holds his account's row, and waits for acme's
// as the creation does. The deactivation first: the creation finds him no
// member of acme: 404, and no notebook. The creation first: rule two lets
// the deactivation through, bob the new notebook's only member; it ends
// his membership of it, and the notebook is ownerless.
func TestDeactivatingAndCreatingANotebook(t *testing.T) {
	t.Run("the deactivation first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")

		deactivated, created := tm.interleave(t, deactivate("bob"), creation("bob", "Late"))

		if !deactivated.is(http.StatusNoContent, "") || !created.is(http.StatusNotFound, "workspace.not_found") {
			t.Errorf("deactivate bob = %d %s, then create = %d %s; want 204, then 404 workspace.not_found", deactivated.status,
				deactivated.code, created.status, created.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM notebooks"); n != 0 {
			t.Errorf("%d notebooks, want none", n)
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the creation first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")

		created, deactivated := tm.interleave(t, creation("bob", "Late"), deactivate("bob"))

		if !created.is(http.StatusCreated, "") || !deactivated.is(http.StatusNoContent, "") {
			t.Fatalf("create = %d %s, then deactivate bob = %d %s; want 201, then 204", created.status, created.code, deactivated.status,
				deactivated.code)
		}
		id, ended := idOf(t, created), tm.endedAt(t, "bob")
		if owner, since := tm.owner(t, id); owner != "bob" || !since.Equal(ended) || tm.notebookMembership(t, id, "bob") != endedState("bob", ended) {
			t.Errorf("the notebook ownerless of %q since %v, bob %q; want bob's since %v, his membership ended by him", owner, since,
				tm.notebookMembership(t, id, "bob"), ended)
		}
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 22: the administrator deactivates carol while bob adds her
// to his notebook. The deactivation first: carol is no member of acme:
// 422, and no row. The addition first: the deactivation ends her new
// membership, by her.
func TestDeactivatingAndBeingAddedToANotebook(t *testing.T) {
	t.Run("the deactivation first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "member")
		id := tm.createNotebook(t, "bob", "Notes")

		deactivated, added := tm.interleave(t, tm.deactivateByCommand(t, "carol"), tm.addition(t, "bob", id, "carol", "reader"))

		if deactivated.err != nil || !added.notInWorkspace() {
			t.Errorf("deactivate carol: %v, then add her = %d %s; want done, then 422 user_id not_allowed", deactivated.err, added.status,
				added.body)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM notebook_members WHERE user_id = $1", tm.userID(t, "carol")); n != 0 {
			t.Errorf("%d notebook memberships of carol's, want none", n)
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the addition first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "member")
		id := tm.createNotebook(t, "bob", "Notes")

		added, deactivated := tm.interleave(t, tm.addition(t, "bob", id, "carol", "reader"), tm.deactivateByCommand(t, "carol"))

		if !added.is(http.StatusCreated, "") || deactivated.err != nil {
			t.Errorf("add carol = %d %s, then deactivate her: %v; want 201, then done", added.status, added.code, deactivated.err)
		}
		if got, want := tm.notebookMembership(t, id, "carol"), endedState("carol", tm.endedAt(t, "carol")); got != want {
			t.Errorf("carol's membership of the notebook %q, want %q", got, want)
		}
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 23: bob, plans' only admin, leaves acme while he makes
// carol, its reader, an admin. The leaving first: rule two, under acme's
// lock, refuses it, and the promotion goes through. The promotion first:
// bob is no longer plans' only admin, and leaves; plans is carol's.
func TestTheOnlyAdminLeavingWhilePromotingAnother(t *testing.T) {
	leave := request("bob", http.MethodPost, "/api/v0/workspaces/acme/leave", "")

	t.Run("the leaving first", func(t *testing.T) {
		tm, plans, carols := planTeam(t)

		left, promoted := tm.interleave(t, leave, promotion("bob", carols))

		if !left.is(http.StatusConflict, "notebook.sole_admin") || !promoted.is(http.StatusOK, "") {
			t.Errorf("bob leaves = %d %s, then promotes carol = %d %s; want 409 notebook.sole_admin, then 200", left.status, left.code,
				promoted.status, promoted.code)
		}
		if tm.membership(t, "bob") != "member" || adminsOf(t, tm, plans) != 2 {
			t.Errorf("bob %s of acme, plans with %d admins; want him a member, plans with him and carol", tm.membership(t, "bob"),
				adminsOf(t, tm, plans))
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the promotion first", func(t *testing.T) {
		tm, plans, carols := planTeam(t)

		promoted, left := tm.interleave(t, promotion("bob", carols), leave)

		if !promoted.is(http.StatusOK, "") || !left.is(http.StatusNoContent, "") {
			t.Errorf("bob promotes carol = %d %s, then leaves = %d %s; want 200, then 204", promoted.status, promoted.code, left.status, left.code)
		}
		if owner, _ := tm.owner(t, plans); owner != "" || tm.notebookMembership(t, plans, "carol") != "admin" ||
			!strings.HasPrefix(tm.notebookMembership(t, plans, "bob"), "ended") {
			t.Errorf("plans ownerless of %q, carol %q, bob %q; want carol's, bob's membership ended", owner,
				tm.notebookMembership(t, plans, "carol"), tm.notebookMembership(t, plans, "bob"))
		}
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 24: alice takes over plans, ownerless since she removed bob,
// while the administrator reactivates his membership. The take-over first:
// plans is alice's, and the restore returns solo alone. The restore first:
// both are bob's again, and the take-over finds plans owned: 404.
func TestTakingOverAndReturning(t *testing.T) {
	t.Run("the take-over first", func(t *testing.T) {
		tm := newCascadeTeam(t)
		tm.send(t, tm.removal("alice", "bob"), http.StatusNoContent)

		took, returned := tm.interleave(t, takeOver("alice", tm.plans), tm.reactivateByCommand(t, "bob"))

		if !took.is(http.StatusOK, "") || returned.err != nil {
			t.Errorf("alice takes plans over = %d %s, then bob is reactivated: %v; want 200, then done", took.status, took.code, returned.err)
		}
		if owner, _ := tm.owner(t, tm.plans); owner != "" || tm.notebookMembership(t, tm.plans, "alice") != "admin" ||
			!strings.HasPrefix(tm.notebookMembership(t, tm.plans, "bob"), "ended") {
			t.Errorf("plans ownerless of %q, alice %q, bob %q; want alice's", owner, tm.notebookMembership(t, tm.plans, "alice"),
				tm.notebookMembership(t, tm.plans, "bob"))
		}
		if got := tm.returnedEvents(t, "bob"); strings.Join(got, ", ") != "Solo by bob" {
			t.Errorf("returned events %q, want solo alone", got)
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the restore first", func(t *testing.T) {
		tm := newCascadeTeam(t)
		tm.send(t, tm.removal("alice", "bob"), http.StatusNoContent)

		returned, took := tm.interleave(t, tm.reactivateByCommand(t, "bob"), takeOver("alice", tm.plans))

		if returned.err != nil || !took.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("bob is reactivated: %v, then alice takes plans over = %d %s; want done, then 404 notebook.not_found", returned.err,
				took.status, took.code)
		}
		if owner, _ := tm.owner(t, tm.plans); owner != "" || tm.notebookMembership(t, tm.plans, "bob") != "admin" {
			t.Errorf("plans ownerless of %q, bob %q; want it his again", owner, tm.notebookMembership(t, tm.plans, "bob"))
		}
		if got := tm.auditEvents(t); strings.Join(got, ", ") != "returned Plans by bob, returned Solo by bob" {
			t.Errorf("audit events %q, want plans and solo returned, no take-over", got)
		}
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 25: alice and bob, acme's admins, take plans over at once.
// Both share acme's row, so the test holds plans': the first takes it
// over; the second finds it owned under its lock: 404. Plans has one
// admin, and one take-over is recorded.
func TestTwoAdminsTakingOverAtOnce(t *testing.T) {
	orders(t, "alice", "bob", func(t *testing.T, first, second string) {
		tm, plans := ownerlessTeam(t)

		took, refused := tm.interleaveOn(t, notebookRow(plans), takeOver(first, plans), takeOver(second, plans))

		if !took.is(http.StatusOK, "") || !refused.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("%s takes plans over = %d %s, then %s = %d %s; want 200, then 404 notebook.not_found", first, took.status, took.code,
				second, refused.status, refused.code)
		}
		if adminsOf(t, tm, plans) != 1 || tm.notebookMembership(t, plans, first) != "admin" {
			t.Errorf("plans with %d admins, %s %q; want %s alone", adminsOf(t, tm, plans), first, tm.notebookMembership(t, plans, first), first)
		}
		if got := tm.auditEvents(t); strings.Join(got, ", ") != "taken_over Plans by "+first {
			t.Errorf("audit events %q, want the take-over by %s", got, first)
		}
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 26: alice removes bob while he creates a notebook. The
// removal first: the creation finds him no member of acme: 404, and no
// notebook. The creation first: the removal ends his membership of it, and
// the notebook is ownerless, his to come back to.
func TestRemovingAMemberAndTheirCreatingANotebook(t *testing.T) {
	t.Run("the removal first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")

		removed, created := tm.interleave(t, tm.removal("alice", "bob"), creation("bob", "Late"))

		if !removed.is(http.StatusNoContent, "") || !created.is(http.StatusNotFound, "workspace.not_found") {
			t.Errorf("remove bob = %d %s, then create = %d %s; want 204, then 404 workspace.not_found", removed.status, removed.code,
				created.status, created.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM notebooks"); n != 0 {
			t.Errorf("%d notebooks, want none", n)
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the creation first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")

		created, removed := tm.interleave(t, creation("bob", "Late"), tm.removal("alice", "bob"))

		if !created.is(http.StatusCreated, "") || !removed.is(http.StatusNoContent, "") {
			t.Fatalf("create = %d %s, then remove bob = %d %s; want 201, then 204", created.status, created.code, removed.status, removed.code)
		}
		id, ended := idOf(t, created), tm.endedAt(t, "bob")
		if owner, since := tm.owner(t, id); owner != "bob" || !since.Equal(ended) || tm.notebookMembership(t, id, "bob") != endedState("alice", ended) {
			t.Errorf("the notebook ownerless of %q since %v, bob %q; want bob's since %v, his membership ended by alice", owner, since,
				tm.notebookMembership(t, id, "bob"), ended)
		}
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 27: bob, plans' only admin with carol its reader, makes her
// an admin while alice removes him, or the administrator deactivates him.
// The removal first: plans is ownerless, and the promotion finds bob gone:
// 404. The deactivation first: rule two refuses it, and the promotion goes
// through. The promotion first: bob is no longer plans' only admin; the
// removal, and the deactivation, which rule two lets through, leave plans
// carol's.
func TestTheOnlyAdminRemovedOrDeactivatedWhilePromotingAnother(t *testing.T) {
	t.Run("the removal first", func(t *testing.T) {
		tm, plans, carols := planTeam(t)

		removed, promoted := tm.interleave(t, tm.removal("alice", "bob"), promotion("bob", carols))

		if !removed.is(http.StatusNoContent, "") || !promoted.is(http.StatusNotFound, "notebook.member_not_found") {
			t.Errorf("remove bob = %d %s, then he promotes carol = %d %s; want 204, then 404 notebook.member_not_found", removed.status,
				removed.code, promoted.status, promoted.code)
		}
		if owner, _ := tm.owner(t, plans); owner != "bob" || tm.notebookMembership(t, plans, "carol") != "reader" {
			t.Errorf("plans ownerless of %q, carol %q; want it bob's ownerless, carol a reader", owner, tm.notebookMembership(t, plans, "carol"))
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the deactivation first", func(t *testing.T) {
		tm, plans, carols := planTeam(t)

		deactivated, promoted := tm.interleave(t, tm.deactivateByCommand(t, "bob"), promotion("bob", carols))

		if deactivated.err == nil || !strings.Contains(deactivated.err.Error(), "only admin of notebooks with other members (1 in acme)") ||
			!promoted.is(http.StatusOK, "") {
			t.Errorf("deactivate bob: %v, then he promotes carol = %d %s; want rule two's refusal, then 200", deactivated.err, promoted.status,
				promoted.code)
		}
		if !tm.isActive(t, "bob@example.com") || adminsOf(t, tm, plans) != 2 {
			t.Errorf("bob active %v, plans with %d admins; want him active, plans with him and carol", tm.isActive(t, "bob@example.com"),
				adminsOf(t, tm, plans))
		}
		checkNotebooks(t, tm.pool)
	})

	for _, end := range []struct {
		name string
		step func(t *testing.T, tm acmeTeam) step
		by   string // who ends bob's notebook memberships
	}{
		{"removal", func(_ *testing.T, tm acmeTeam) step { return tm.removal("alice", "bob") }, "alice"},
		{"deactivation", func(t *testing.T, tm acmeTeam) step { return tm.deactivateByCommand(t, "bob") }, "bob"},
	} {
		t.Run("the promotion first, then the "+end.name, func(t *testing.T) {
			tm, plans, carols := planTeam(t)

			promoted, ended := tm.interleave(t, promotion("bob", carols), end.step(t, tm))

			if !promoted.is(http.StatusOK, "") || ended.err != nil || (ended.res != nil && ended.status != http.StatusNoContent) {
				t.Errorf("bob promotes carol = %d %s, then the %s = %d %s %v; want 200, then done", promoted.status, promoted.code, end.name,
					ended.status, ended.code, ended.err)
			}
			if owner, _ := tm.owner(t, plans); owner != "" || tm.notebookMembership(t, plans, "carol") != "admin" ||
				tm.notebookMembership(t, plans, "bob") != endedState(end.by, tm.endedAt(t, "bob")) {
				t.Errorf("plans ownerless of %q, carol %q, bob %q; want carol's, bob's membership ended by %s", owner,
					tm.notebookMembership(t, plans, "carol"), tm.notebookMembership(t, plans, "bob"), end.by)
			}
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 28: alice deletes plans, ownerless since she removed bob,
// while bob accepts an invitation back. The deletion first: the restore
// returns solo alone. The restore first: plans is bob's again, and the
// deletion finds it owned: 404.
func TestDeletingAnOwnerlessNotebookAndReturning(t *testing.T) {
	t.Run("the deletion first", func(t *testing.T) {
		tm := newCascadeTeam(t)
		tm.send(t, tm.removal("alice", "bob"), http.StatusNoContent)
		inv := tm.invite(t, "bob", "guest")

		deleted, accepted := tm.interleave(t, ownerlessDeletion("alice", tm.plans), accept("bob", inv))

		if !deleted.is(http.StatusNoContent, "") || !accepted.is(http.StatusOK, "") {
			t.Errorf("alice deletes plans = %d %s, then bob accepts = %d %s; want 204, then 200", deleted.status, deleted.code,
				accepted.status, accepted.code)
		}
		if got := tm.auditEvents(t); strings.Join(got, ", ") != "deleted Plans by alice, returned Solo by bob" {
			t.Errorf("audit events %q, want plans deleted, solo alone returned", got)
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the restore first", func(t *testing.T) {
		tm := newCascadeTeam(t)
		tm.send(t, tm.removal("alice", "bob"), http.StatusNoContent)
		inv := tm.invite(t, "bob", "guest")

		accepted, deleted := tm.interleave(t, accept("bob", inv), ownerlessDeletion("alice", tm.plans))

		if !accepted.is(http.StatusOK, "") || !deleted.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("bob accepts = %d %s, then alice deletes plans = %d %s; want 200, then 404 notebook.not_found", accepted.status,
				accepted.code, deleted.status, deleted.code)
		}
		if owner, _ := tm.owner(t, tm.plans); owner != "" || tm.notebookMembership(t, tm.plans, "bob") != "admin" {
			t.Errorf("plans ownerless of %q, bob %q; want it his again", owner, tm.notebookMembership(t, tm.plans, "bob"))
		}
		if got := tm.auditEvents(t); strings.Join(got, ", ") != "returned Plans by bob, returned Solo by bob" {
			t.Errorf("audit events %q, want plans and solo returned, no deletion", got)
		}
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 29: alice takes plans over while bob, acme's other admin,
// makes her a member, or removes her. The take-over first: the demotion
// leaves her plans' admin; the removal ends her membership of it, and
// plans is ownerless again, hers to come back to. The change first: alice
// is no admin of acme when she decides, or no member: 404, and plans stays
// carol's ownerless.
func TestTakingOverAndTheTakerDemotedOrRemoved(t *testing.T) {
	for _, change := range []struct {
		name   string
		step   func(tm acmeTeam) step
		status int
	}{
		{"demotion", func(tm acmeTeam) step {
			return request("bob", http.MethodPatch, "/api/v0/workspace-members/"+tm.members["alice"].String(), `{"role":"member"}`)
		}, http.StatusOK},
		{"removal", func(tm acmeTeam) step { return tm.removal("bob", "alice") }, http.StatusNoContent},
	} {
		t.Run("the take-over first, then the "+change.name, func(t *testing.T) {
			tm, plans := ownerlessTeam(t)

			took, changed := tm.interleave(t, takeOver("alice", plans), change.step(tm))

			if !took.is(http.StatusOK, "") || !changed.is(change.status, "") {
				t.Errorf("alice takes plans over = %d %s, then the %s = %d %s; want 200, then %d", took.status, took.code, change.name,
					changed.status, changed.code, change.status)
			}
			owner, since := tm.owner(t, plans)
			switch change.name {
			case "demotion":
				if owner != "" || tm.notebookMembership(t, plans, "alice") != "admin" {
					t.Errorf("plans ownerless of %q, alice %q; want alice's", owner, tm.notebookMembership(t, plans, "alice"))
				}
			case "removal":
				if ended := tm.endedAt(t, "alice"); owner != "alice" || !since.Equal(ended) ||
					tm.notebookMembership(t, plans, "alice") != endedState("bob", ended) {
					t.Errorf("plans ownerless of %q since %v, alice %q; want alice's since %v, her membership ended by bob", owner, since,
						tm.notebookMembership(t, plans, "alice"), ended)
				}
			}
			checkNotebooks(t, tm.pool)
		})

		t.Run("the "+change.name+" first", func(t *testing.T) {
			tm, plans := ownerlessTeam(t)

			changed, took := tm.interleave(t, change.step(tm), takeOver("alice", plans))

			if !changed.is(change.status, "") || !took.is(http.StatusNotFound, "notebook.not_found") {
				t.Errorf("the %s = %d %s, then alice takes plans over = %d %s; want %d, then 404 notebook.not_found", change.name,
					changed.status, changed.code, took.status, took.code, change.status)
			}
			if owner, _ := tm.owner(t, plans); owner != "carol" || adminsOf(t, tm, plans) != 0 {
				t.Errorf("plans ownerless of %q with %d admins; want carol's ownerless still", owner, adminsOf(t, tm, plans))
			}
			checkNotebooks(t, tm.pool)
		})
	}
}
