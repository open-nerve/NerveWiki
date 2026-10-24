package bootstrap

import (
	"net/http"
	"testing"
)

// The interleavings of an LLM Wiki's creation (M10/P1 design 3.15),
// harnessed as Interleaving 15 (interleavings_notebook_test.go): the
// creation's transaction takes acme's row FOR SHARE first, the template's
// unit takes it again within it, and every change of acme's memberships,
// its deletion too, holds it FOR NO KEY UPDATE, so the test holds acme's
// row and the two run one after the other, in the order they came. Each
// ends by checking the pages' invariant and the notebooks'.

// creatingAnLLMWiki is bob's creation of an LLM Wiki in acme.
func creatingAnLLMWiki() step {
	return request("bob", http.MethodPost, "/api/v0/workspaces/acme/notebooks",
		`{"name":"Field notes","mode":"llm_wiki","template_locale":"en"}`)
}

// wikiRows counts the notebooks, their members, nodes and changesets, kept
// or deleted.
func (tm acmeTeam) wikiRows(t *testing.T) (notebooks, members, nodes, changesets int) {
	t.Helper()
	return count(t, tm.pool, "SELECT count(*) FROM notebooks"), count(t, tm.pool, "SELECT count(*) FROM notebook_members"),
		count(t, tm.pool, "SELECT count(*) FROM nodes"), count(t, tm.pool, "SELECT count(*) FROM changesets")
}

// checkTheLLMWikisRows fails t unless the rows are one LLM Wiki's: the
// notebook, its admin, its two pages and its template's changeset.
func (tm acmeTeam) checkTheLLMWikisRows(t *testing.T) {
	t.Helper()
	if notebooks, members, nodes, changesets := tm.wikiRows(t); notebooks != 1 || members != 1 || nodes != 2 || changesets != 1 {
		t.Errorf("%d notebooks, %d members, %d nodes, %d changesets; want 1, 1, 2, 1", notebooks, members, nodes, changesets)
	}
}

// A workspace's deletion, the end of the creator's membership, and a change
// of its role, each before and after an LLM Wiki's creation: the creation
// first, the change takes the notebook with its template as any; the
// change first, the creation is refused as a standard one would be, and
// writes nothing.
func TestWorkspaceChangesAndCreatingAnLLMWiki(t *testing.T) {
	t.Run("the workspace's deletion", func(t *testing.T) {
		deletion := request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "")
		t.Run("the deletion first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			deleted, created := tm.interleave(t, deletion, creatingAnLLMWiki())
			if !deleted.is(http.StatusNoContent, "") || !created.is(http.StatusNotFound, "workspace.not_found") {
				t.Errorf("DELETE acme = %d, then POST an LLM Wiki = %d %s; want 204, then 404 workspace.not_found",
					deleted.status, created.status, created.code)
			}
			if notebooks, members, nodes, changesets := tm.wikiRows(t); notebooks+members+nodes+changesets != 0 {
				t.Errorf("%d notebooks, %d members, %d nodes, %d changesets; want none", notebooks, members, nodes, changesets)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
		t.Run("the creation first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			created, deleted := tm.interleave(t, creatingAnLLMWiki(), deletion)
			if !created.is(http.StatusCreated, "") || !deleted.is(http.StatusNoContent, "") {
				t.Fatalf("POST an LLM Wiki = %d %s, then DELETE acme = %d; want 201, then 204", created.status, created.code, deleted.status)
			}
			id := idOf(t, created)
			tm.checkTheLLMWikisRows(t)
			if notebooks, members, _ := tm.deletedWithAcme(t); notebooks != 1 || members != 1 ||
				count(t, tm.pool, `SELECT count(*) FROM nodes n JOIN workspaces w ON w.slug = 'acme'
					WHERE n.notebook_id = $1 AND n.deleted_at = w.deleted_at`, id) != 2 {
				t.Errorf("%d notebooks and %d members deleted with acme; want the LLM Wiki, its admin and its two pages", notebooks, members)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	})

	t.Run("the creator's membership's end", func(t *testing.T) {
		t.Run("the end first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			removed, created := tm.interleave(t, tm.removal("alice", "bob"), creatingAnLLMWiki())
			if !removed.is(http.StatusNoContent, "") || !created.is(http.StatusNotFound, "workspace.not_found") {
				t.Errorf("DELETE bob = %d, then POST an LLM Wiki = %d %s; want 204, then 404 workspace.not_found",
					removed.status, created.status, created.code)
			}
			if notebooks, members, nodes, changesets := tm.wikiRows(t); notebooks+members+nodes+changesets != 0 {
				t.Errorf("%d notebooks, %d members, %d nodes, %d changesets; want none", notebooks, members, nodes, changesets)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
		t.Run("the creation first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			created, removed := tm.interleave(t, creatingAnLLMWiki(), tm.removal("alice", "bob"))
			if !created.is(http.StatusCreated, "") || !removed.is(http.StatusNoContent, "") {
				t.Fatalf("POST an LLM Wiki = %d %s, then DELETE bob = %d; want 201, then 204", created.status, created.code, removed.status)
			}
			id := idOf(t, created)
			tm.checkTheLLMWikisRows(t)
			// Bob's membership of the notebook ends with his workspace
			// membership: the LLM Wiki is ownerless, its template kept.
			if count(t, tm.pool, `SELECT count(*) FROM notebooks n JOIN users u ON u.id = n.former_owner_id
					WHERE n.id = $1 AND n.ownerless_since IS NOT NULL AND u.email = 'bob@example.com'`, id) != 1 ||
				count(t, tm.pool, "SELECT count(*) FROM notebook_members WHERE notebook_id = $1 AND ended_at IS NULL", id) != 0 ||
				len(tm.rootOf(t, id)) != 2 {
				t.Errorf("the LLM Wiki after bob's removal: want it ownerless of bob, no active member, its two pages kept")
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	})

	t.Run("the creator's role's change", func(t *testing.T) {
		demotion := func(tm acmeTeam) step {
			return request("alice", http.MethodPatch, "/api/v0/workspace-members/"+tm.members["bob"].String(), `{"role":"guest"}`)
		}
		t.Run("the change first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			changed, created := tm.interleave(t, demotion(tm), creatingAnLLMWiki())
			if !changed.is(http.StatusOK, "") || !created.is(http.StatusForbidden, "forbidden") {
				t.Errorf("PATCH bob a guest = %d, then POST an LLM Wiki = %d %s; want 200, then 403 forbidden",
					changed.status, created.status, created.code)
			}
			if notebooks, members, nodes, changesets := tm.wikiRows(t); notebooks+members+nodes+changesets != 0 {
				t.Errorf("%d notebooks, %d members, %d nodes, %d changesets; want none", notebooks, members, nodes, changesets)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
		t.Run("the creation first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			created, changed := tm.interleave(t, creatingAnLLMWiki(), demotion(tm))
			if !created.is(http.StatusCreated, "") || !changed.is(http.StatusOK, "") {
				t.Fatalf("POST an LLM Wiki = %d %s, then PATCH bob a guest = %d; want 201, then 200", created.status, created.code, changed.status)
			}
			id := idOf(t, created)
			tm.checkTheLLMWikisRows(t)
			// A notebook's membership does not follow the workspace's role.
			if count(t, tm.pool, `SELECT count(*) FROM notebook_members m JOIN users u ON u.id = m.user_id
					WHERE m.notebook_id = $1 AND m.role = 'admin' AND m.ended_at IS NULL AND u.email = 'bob@example.com'`, id) != 1 ||
				len(tm.rootOf(t, id)) != 2 {
				t.Errorf("the LLM Wiki after bob's change: want bob its admin still, its two pages kept")
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	})
}

// An import's start and an LLM Wiki's downgrade (M10/P1 design 3.16): the
// start shares the notebook's row, the downgrade locks it FOR NO KEY
// UPDATE, so the test holds the row and they run in the order they came,
// the imports' one worker held so the job stays queued. The start first:
// the downgrade finds its job and waits for it (transfer.busy). The
// downgrade first: the import starts in a standard notebook and runs to
// its end as in any.
func TestImportsAndDowngradingAnLLMWiki(t *testing.T) {
	archive := func(t *testing.T) []byte { return vaultZip(t, vaultFile{"a.md", "a"}) }
	downgrading := func(nb string) step {
		return request("alice", http.MethodPost, "/api/v0/notebooks/"+nb+"/downgrade", "")
	}
	t.Run("the start first", func(t *testing.T) {
		tm := newAcmeTeam(t, "", "")
		nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Field notes","mode":"llm_wiki"}`).ID
		release := tm.holdImportWorker(t)
		started, downgraded := tm.interleaveOn(t, notebookRow(nb), importStep(t, "alice", nb, "", archive(t)), downgrading(nb))
		if !started.is(http.StatusAccepted, "") || !downgraded.is(http.StatusConflict, "transfer.busy") {
			t.Fatalf("the import = %d, then the downgrade = %d %s; want 202, then 409 transfer.busy", started.status,
				downgraded.status, downgraded.code)
		}
		release()
		var j transferJob
		decodeAnswer(t, started.body, &j)
		if ended := tm.endedJob(t, "alice", j.ID); ended.State != "succeeded" ||
			count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE id = $1 AND mode = 'llm_wiki'", nb) != 1 {
			t.Errorf("the import = %+v; want it succeeded, the notebook an LLM Wiki still", ended)
		}
		checkImports(t, tm)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the downgrade first", func(t *testing.T) {
		tm := newAcmeTeam(t, "", "")
		nb := tm.createWith(t, tm.tokens["alice"], `{"name":"Field notes","mode":"llm_wiki"}`).ID
		release := tm.holdImportWorker(t)
		downgraded, started := tm.interleaveOn(t, notebookRow(nb), downgrading(nb), importStep(t, "alice", nb, "", archive(t)))
		if !downgraded.is(http.StatusOK, "") || !started.is(http.StatusAccepted, "") {
			t.Fatalf("the downgrade = %d %s, then the import = %d; want 200, then 202", downgraded.status, downgraded.code, started.status)
		}
		release()
		var j transferJob
		decodeAnswer(t, started.body, &j)
		if ended := tm.endedJob(t, "alice", j.ID); ended.State != "succeeded" ||
			count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name = 'a' AND deleted_at IS NULL", nb) != 1 {
			t.Errorf("the import = %+v; want it succeeded, its page a written", ended)
		}
		checkImports(t, tm)
		checkNotebooks(t, tm.pool)
	})
}
