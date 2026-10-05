package bootstrap

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// The links written again as their pages are renamed or moved, through
// serve (M6/P4 design 4): pageParticipants hands the rewrite to the page
// module, and each test fails without it.

// A rename writes again the links to the page and to the pages under it,
// in the body and in the frontmatter, by their names, each as the rename's
// author, in its changeset; a link an alias led to that the new title
// takes is written by the alias's page with the alias shown.
func TestARenameWritesTheLinksToItAgain(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	child := tm.createPage(t, "alice", nb, a, "Child")
	p := tm.createPageWith(t, "alice", nb, "", "P", "---\naliases: [x]\n---\n")
	q := tm.createPage(t, "alice", nb, "", "Q")
	src := tm.createPageWith(t, "alice", nb, "", "Src", "---\nup: \"[[A]]\"\n---\n[[A]] [[A/Child]] [t](A.md) [[x]] [[Q]]\n")

	tm.send(t, nodeRename("bob", a, "B"), http.StatusOK)
	tm.wrote(t, src, "---\nup: \"[[B]]\"\n---\n[[B]] [[Child]] [t](B.md) [[x]] [[Q]]\n", 2)
	if n := count(t, tm.pool, `SELECT count(*) FROM page_revisions r JOIN changesets c ON c.id = r.changeset_id
		JOIN changeset_items i ON i.changeset_id = c.id JOIN users u ON u.id = c.created_by_id WHERE r.node_id = $1
		AND r.revision = 2 AND i.node_id = $2 AND i.after_name = 'B' AND u.email = 'bob@example.com'`, src, a); n != 1 {
		t.Errorf("the rewrite's revision in bob's rename's changeset: %d, want 1", n)
	}
	tm.send(t, nodeRename("alice", q, "x"), http.StatusOK)
	tm.wrote(t, src, "---\nup: \"[[B]]\"\n---\n[[B]] [[Child]] [t](B.md) [[P|x]] [[x]]\n", 3)
	tm.resolves(t, src, a, a, child, a, p, q)
	if c := tm.content(t, "alice", a); c.Revision != 1 || c.Content != "" {
		t.Errorf("the page renamed: %+v, want its content as it was", c)
	}
	checkLinks(t, tm.pool)
	checkPages(t, tm.pool)
}

// A move writes again the moved page's links that lead from its folder,
// and the links to it from the root; a link that still leads to its page
// is left.
func TestAMoveWritesTheLinksOfItAndToItAgain(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	here := tm.createPage(t, "alice", nb, "", "Here")
	out := tm.createPageWith(t, "alice", nb, "", "Out", "[h](./Here.md) [[Here]]\n")
	under := tm.createPage(t, "alice", nb, out, "Under")
	in := tm.createPageWith(t, "alice", nb, "", "In", "[[Out]] [[Out/Under]] [o](/Out.md)\n")
	box := tm.createPage(t, "alice", nb, "", "Box")

	tm.send(t, nodeMove("alice", out, box), http.StatusOK)
	tm.wrote(t, out, "[h](../Here.md) [[Here]]\n", 2)
	tm.wrote(t, in, "[[Out]] [[Out/Under]] [o](/Box/Out.md)\n", 2)
	tm.resolves(t, out, here, here)
	tm.resolves(t, in, out, under, out)
	checkLinks(t, tm.pool)
	checkPages(t, tm.pool)
}

// A rename or move whose rewrite reaches a page being edited is refused
// whole (M6/P4 design 4.1): 409 linking.pages_locked, its locks each such
// page by id with its editor, the caller's own among them, and nothing
// changed. A page being edited whose links need no rewrite moves, as the
// edit lock lets it; a page renamed while it is edited too. Once the locks
// are gone, the rename writes the links again.
func TestARewriteOfAPageBeingEditedIsRefused(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	src := tm.createPageWith(t, "alice", nb, "", "Src", "[[A]]\n")
	other := tm.createPageWith(t, "alice", nb, "", "Other", "[[A]]\n")
	tm.createPage(t, "alice", nb, "", "Here")
	box := tm.createPage(t, "alice", nb, "", "Box")
	out := tm.createPageWith(t, "alice", nb, "", "Out", "[h](./Here.md)\n")
	mine := tm.createPageWith(t, "alice", nb, "", "Mine", "[[Here]]\n")
	tm.openSession(t, "bob", src)
	tm.openSession(t, "alice", other)
	tm.openSession(t, "alice", out)
	tm.openSession(t, "alice", mine)
	tm.openSession(t, "bob", a)

	was := tm.snapshot(t, nb)
	tm.refused(t, nodeRename("alice", a, "B"), map[string]string{src: "bob", other: "alice"})
	tm.refused(t, nodeMove("alice", out, box), map[string]string{out: "alice"})
	if now := tm.snapshot(t, nb); now != was {
		t.Errorf("the refusals changed the notebook:\n%s\nwas\n%s", now, was)
	}

	tm.send(t, nodeMove("alice", mine, box), http.StatusOK)
	tm.send(t, unlock("alice", src), http.StatusNoContent)
	tm.send(t, unlock("alice", other), http.StatusNoContent)
	tm.send(t, nodeRename("alice", a, "B"), http.StatusOK)
	tm.wrote(t, src, "[[B]]\n", 2)
	tm.wrote(t, other, "[[B]]\n", 2)
	checkLinks(t, tm.pool)
	checkPages(t, tm.pool)
}

// A rewrite that finds no budget for a page's parse is refused whole, 503
// server_busy, and changes nothing: under a budget of one take's least,
// which the configuration would not let, the first page's facts hold it
// all to the unit's end. A rewrite of one page fits, again and again: the
// page's contents parsed give their share back.
func TestARewriteWithoutTheBudgetIsBusy(t *testing.T) {
	tm := newAcmeTeamWith(t, "member", "", func(c *config.Config) { c.Page.ParseBudgetBytes = 4 << 10 })
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.createPage(t, "alice", nb, "", "A")
	tm.createPageWith(t, "alice", nb, "", "One", "[[A]]\n")
	tm.createPageWith(t, "alice", nb, "", "Two", "[[A]]\n")
	c := tm.createPage(t, "alice", nb, "", "C")
	three := tm.createPageWith(t, "alice", nb, "", "Three", "[[C]]\n")

	tm.send(t, nodeRename("alice", c, "D"), http.StatusOK)
	tm.send(t, nodeRename("alice", c, "E"), http.StatusOK)
	tm.wrote(t, three, "[[E]]\n", 3)
	was := tm.snapshot(t, nb)
	rename := nodeRename("alice", a, "B")
	if status, body := ask(t, tm.contract, rename.method, tm.base+rename.path, tm.tokens["alice"], rename.body); status != http.StatusServiceUnavailable ||
		!strings.Contains(body, `"code":"server_busy"`) {
		t.Errorf("the rename = %d %s, want 503 server_busy", status, body)
	}
	if now := tm.snapshot(t, nb); now != was {
		t.Errorf("the busy rename changed the notebook:\n%s\nwas\n%s", now, was)
	}
	checkLinks(t, tm.pool)
}

// wrote fails t unless the page id holds content at revision.
func (tm acmeTeam) wrote(t *testing.T, id, content string, revision int) {
	t.Helper()
	if c := tm.content(t, "alice", id); c.Content != content || c.Revision != revision {
		t.Errorf("the page %s holds %q at %d, want %q at %d", id, c.Content, c.Revision, content, revision)
	}
}

// refused fails t unless c is 409 linking.pages_locked with the locks of
// want.
func (tm acmeTeam) refused(t *testing.T, c step, want map[string]string) {
	t.Helper()
	status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.body)
	pagesLocked(t, c.name(), answer{status: status, body: body}, want)
}

// pagesLocked fails t unless a, what's answer, is 409
// linking.pages_locked, its locks those of want, page id to its editor's
// name, in the pages' order.
func pagesLocked(t *testing.T, what string, a answer, want map[string]string) {
	t.Helper()
	var p struct {
		Code  string `json:"code"`
		Locks []struct {
			PageID      string `json:"page_id"`
			DisplayName string `json:"display_name"`
		} `json:"locks"`
	}
	decodeAnswer(t, a.body, &p)
	var got, wanted []string
	for _, l := range p.Locks {
		got = append(got, l.PageID+" "+l.DisplayName)
	}
	for id, name := range want {
		wanted = append(wanted, id+" "+name)
	}
	slices.Sort(wanted)
	if a.status != http.StatusConflict || p.Code != "linking.pages_locked" || !slices.Equal(got, wanted) {
		t.Errorf("%s = %d %s, want 409 linking.pages_locked with %q", what, a.status, a.body, wanted)
	}
}

// snapshot is the notebook's pages as a rewrite would change them, each
// one's parent, name, revision and content, and its changesets' count.
func (tm acmeTeam) snapshot(t *testing.T, nb string) string {
	t.Helper()
	var s string
	if err := tm.pool.QueryRow(t.Context(), `SELECT coalesce(string_agg(concat_ws(' ', n.id, n.parent_id, n.name, c.revision, c.content),
		E'\n' ORDER BY n.id), '') || E'\n' || (SELECT count(*) FROM changesets WHERE notebook_id = $1)
		FROM nodes n JOIN page_contents c ON c.node_id = n.id WHERE n.notebook_id = $1`, nb).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}
