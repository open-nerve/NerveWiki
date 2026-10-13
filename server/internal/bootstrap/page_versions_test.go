package bootstrap

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// A page's versions through serve (M8/P1 design 3.8).

// versionAnswer is a PageVersion answer, as much as the tests check.
type versionAnswer struct {
	ID           string `json:"id"`
	Revision     int    `json:"revision"`
	BaseRevision *int   `json:"base_revision"`
	Editing      bool   `json:"editing"`
	Content      string `json:"content"`
	Changeset    struct {
		Kind      string `json:"kind"`
		Client    string `json:"client"`
		CreatedBy struct {
			DisplayName string `json:"display_name"`
		} `json:"created_by"`
	} `json:"changeset"`
}

// versions reads the page id's versions as by, the first page of them.
func (tm acmeTeam) versions(t *testing.T, by, id string) []versionAnswer {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+id+"/versions", tm.tokens[by], "")
	if status != http.StatusOK {
		t.Fatalf("GET the versions of %s as %s = %d %s", id, by, status, body)
	}
	var l struct {
		Data []versionAnswer `json:"data"`
	}
	decodeAnswer(t, body, &l)
	return l.Data
}

// versionRestore is by's restore of the page id's version holding
// revision, on base.
func versionRestore(by, id string, revision, base int) step {
	return request(by, http.MethodPost, fmt.Sprintf("/api/v0/pages/%s/versions/%d/restore", id, revision),
		fmt.Sprintf(`{"base_revision":%d}`, base))
}

// Bob, an editor, lists a page's versions newer-first, each with who made
// it, and restores the first, which linked to Target: the content, a new
// version on the page's revision in a version_restore changeset of his, and
// the index follow, and the streams see the page's write and its links.
// While alice's session holds the page, his restore is page.locked by her.
// A restore of the content the page holds writes nothing: no event.
func TestRestoringAVersionThroughServe(t *testing.T) {
	tm, nb, marker, s := eventsTeam(t)
	target := tm.createPage(t, "alice", nb, "", "Target")
	s.tree(t, nb, map[string]int{target: 1})
	id := tm.createPageWith(t, "alice", nb, "", "Notes", "see [[Target]]")
	s.tree(t, nb, map[string]int{id: 1})
	s.links(t, nb, []string{}, []string{target})
	tm.send(t, contentWrite("alice", id, "nothing", 1, ""), http.StatusOK)
	s.pages(t, nb)
	s.links(t, nb, []string{}, []string{target})

	v := tm.versions(t, "bob", id)
	if len(v) != 2 || v[0].Revision != 2 || *v[0].BaseRevision != 1 || v[1].Revision != 1 || v[1].BaseRevision != nil ||
		v[0].Changeset.Kind != "edit" || v[0].Changeset.Client != "web" || v[0].Changeset.CreatedBy.DisplayName != "alice" {
		t.Fatalf("versions = %+v, want 2 on 1, then 1, alice's edits from the web", v)
	}

	session := tm.openSession(t, "alice", id)
	s.lock(t, id, session)
	c := versionRestore("bob", id, 1, 2)
	status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], c.body)
	if status != http.StatusConflict || !strings.Contains(body, `"code":"page.locked"`) || lockHolder(answer{body: body}) != "alice" {
		t.Errorf("bob's restore while alice edits = %d %s, want 409 page.locked by alice", status, body)
	}
	tm.send(t, request("alice", http.MethodDelete, "/api/v0/edit-sessions/"+session, ""), http.StatusNoContent)
	s.lock(t, id, session)

	status, body = ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], c.body)
	var r struct {
		Revision    int     `json:"revision"`
		ChangesetID *string `json:"changeset_id"`
		Changed     bool    `json:"changed"`
	}
	decodeAnswer(t, body, &r)
	if status != http.StatusOK || r.Revision != 3 || r.ChangesetID == nil || !r.Changed {
		t.Fatalf("bob's restore = %d %s, want revision 3, changed, in a changeset", status, body)
	}
	if p := s.pages(t, nb); p.Tree || fmt.Sprint(p.written()) != fmt.Sprint(map[string]int{id: 3}) {
		t.Errorf("the restore: tree %v, written %v; want %s at 3 alone", p.Tree, p.written(), id)
	}
	s.links(t, nb, []string{}, []string{target})
	tm.resolves(t, id, target)
	if got := tm.content(t, "alice", id); got.Content != "see [[Target]]" || got.Revision != 3 {
		t.Errorf("the content = %+v, want version 1's at revision 3", got)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM changesets c JOIN page_revisions r ON r.changeset_id = c.id
		JOIN users u ON u.id = c.created_by_id WHERE c.id = $1 AND c.kind = 'version_restore' AND c.reverts_id IS NULL
		AND u.email = 'bob@example.com' AND r.node_id = $2 AND r.base_revision = 2 AND r.revision = 3`, *r.ChangesetID, id); n != 1 {
		t.Errorf("%d version_restore changesets of bob with the version 3 on 2, want 1", n)
	}

	tm.send(t, versionRestore("bob", id, 1, 3), http.StatusOK)
	tm.quiet(t, nb, marker, s)
	checkPages(t, tm.pool)
}

// An edit session's saves are one version, being edited while the session
// is alive, which any revision they wrote finds, answered with its own; its
// reading view's links lead where they lead now. A page in the trash keeps
// its history, read as an alive page's (its links resolve as M6 left a
// deleted page's until P2); a restore is page.not_found until the page
// leaves the trash. A version no revision of the page holds is
// page.version_not_found.
func TestAPageInTheTrashKeepsItsHistory(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	target := tm.createPage(t, "alice", nb, "", "Target")
	id := tm.createPageWith(t, "alice", nb, "", "Notes", "one")
	session := tm.openSession(t, "bob", id)
	tm.send(t, contentWrite("bob", id, "two [[Target]]", 1, session), http.StatusOK)
	tm.send(t, contentWrite("bob", id, "three [[Target]]", 2, session), http.StatusOK)
	if v := tm.versions(t, "bob", id); len(v) != 2 || v[0].Revision != 3 || *v[0].BaseRevision != 1 || !v[0].Editing || v[1].Editing {
		t.Fatalf("versions = %+v, want the session's, 3 on 1, being edited, then 1", v)
	}
	tm.send(t, request("bob", http.MethodDelete, "/api/v0/edit-sessions/"+session, ""), http.StatusNoContent)
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+id+"/versions/3/view", tm.tokens["bob"], "")
	if status != http.StatusOK || !strings.Contains(body, `data-nw-node=\"`+target+`\"`) || !strings.Contains(body, `"revision":3`) {
		t.Errorf("version 3's view = %d %s, want it rendered at 3, its link to Target", status, body)
	}
	tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)

	if v := tm.versions(t, "bob", id); len(v) != 2 || v[0].Editing {
		t.Errorf("versions in the trash = %+v, want both, none being edited", v)
	}
	status, body = ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+id+"/versions/2", tm.tokens["bob"], "")
	var got versionAnswer
	decodeAnswer(t, body, &got)
	if status != http.StatusOK || got.Revision != 3 || got.Content != "three [[Target]]" {
		t.Errorf("version 2 = %d %s, want the session's version 3", status, body)
	}
	status, body = ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+id+"/versions/3/view", tm.tokens["bob"], "")
	if status != http.StatusOK || !strings.Contains(body, `"revision":3`) {
		t.Errorf("version 3's view in the trash = %d %s, want it rendered at 3", status, body)
	}
	for _, c := range []struct {
		step step
		want cell
	}{
		{request("bob", http.MethodGet, "/api/v0/pages/"+id+"/versions/4", ""), cell{http.StatusNotFound, "page.version_not_found"}},
		{request("bob", http.MethodGet, "/api/v0/pages/"+id+"/versions/0/view", ""), cell{http.StatusNotFound, "page.version_not_found"}},
		{request("bob", http.MethodGet, "/api/v0/pages/"+id+"/versions/x", ""), cell{http.StatusBadRequest, "bad_request"}},
		{versionRestore("bob", id, 1, 3), cell{http.StatusNotFound, "page.not_found"}},
		{request("carol", http.MethodGet, "/api/v0/pages/"+id+"/versions", ""), cell{http.StatusNotFound, "page.not_found"}},
	} {
		status, answer := ask(t, tm.contract, c.step.method, tm.base+c.step.path, tm.tokens[c.step.by], c.step.body)
		if status != c.want.status || problemCode(t, answer) != c.want.code {
			t.Errorf("%s = %d %s, want %s", c.step.name(), status, answer, c.want)
		}
	}
	checkPages(t, tm.pool)
}
