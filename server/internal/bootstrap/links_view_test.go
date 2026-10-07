package bootstrap

import (
	"net/http"
	"strings"
	"testing"
)

// A reading view's links through serve (M6/P3 design 6.8): serve's
// Markdown resolves them through the linking module (markdownExtensions,
// parsing). They lead where the index has them resolve when its rows are
// of the content rendered, and resolve anew when they are of another
// revision, of another extractor, or not there.
func TestAReadingViewsLinksLeadWhereTheyResolve(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	target := tm.createPage(t, "alice", nb, "", "Target")
	other := tm.createPage(t, "alice", nb, "", "Other")
	src := tm.createPage(t, "alice", nb, "", "Source")
	tm.send(t, contentWrite("alice", src, "[[Target#Part Two]] [[Missing]] [t](Target.md)\n", 1, ""), http.StatusOK)

	view := func() string {
		t.Helper()
		status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+src+"/view", tm.tokens["bob"], "")
		if status != http.StatusOK {
			t.Fatalf("bob's reading view = %d %s", status, body)
		}
		var v struct {
			HTML string `json:"html"`
		}
		decodeAnswer(t, body, &v)
		return v.HTML
	}
	leads := func(what, to string) {
		t.Helper()
		html := view()
		for _, want := range []string{
			`<a class="nw-wikilink" data-nw-node="` + to + `" data-nw-anchor="nw-part-two">Target &gt; Part Two</a>`,
			`<a class="nw-wikilink nw-unresolved" data-nw-target="Missing">Missing</a>`,
			`<a data-nw-node="` + target + `">t</a>`,
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s: the reading view %q holds no %s", what, html, want)
			}
		}
	}
	exec := func(sql string) {
		t.Helper()
		if _, err := tm.pool.Exec(t.Context(), sql, src); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	leads("the index", target)
	// The index read is the one the view shows: its link misdirected.
	if _, err := tm.pool.Exec(t.Context(), `UPDATE page_links SET resolved_id = $1 WHERE source_id = $2 AND range_start = 2`,
		other, src); err != nil {
		t.Fatal(err)
	}
	leads("the index misdirected", other)
	exec(`UPDATE indexed_pages SET revision = revision + 1 WHERE node_id = $1`)
	leads("another revision", target)
	exec(`UPDATE indexed_pages SET revision = revision - 1, extractor = extractor + 1 WHERE node_id = $1`)
	leads("another extractor", target)
	exec(`UPDATE indexed_pages SET extractor = extractor - 1 WHERE node_id = $1`)
	leads("the index again", other)
	exec(`DELETE FROM indexed_pages WHERE node_id = $1`)
	leads("no index", target)
}

// A reading view's property links through serve (M6/P6 design 4): the
// property table writes them as the body writes its links, leading where
// they resolve, or carrying their targets.
func TestAReadingViewsPropertyLinksLeadWhereTheyResolve(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	target := tm.createPage(t, "alice", nb, "", "Target")
	src := tm.createPageWith(t, "alice", nb, "", "Source", "---\nup: \"[[Target#Part Two|top]]\"\nsee: [\"[x](Missing.md)\"]\n---\n")
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+src+"/view", tm.tokens["bob"], "")
	if status != http.StatusOK {
		t.Fatalf("bob's reading view = %d %s", status, body)
	}
	var v struct {
		HTML string `json:"html"`
	}
	decodeAnswer(t, body, &v)
	for _, want := range []string{
		`<td><a class="nw-wikilink" data-nw-node="` + target + `" data-nw-anchor="nw-part-two">top</a></td>`,
		`<td><ul><li><a class="nw-unresolved" data-nw-target="Missing.md">x</a></li></ul></td>`,
	} {
		if !strings.Contains(v.HTML, want) {
			t.Errorf("the reading view %q holds no %s", v.HTML, want)
		}
	}
}

// A page's links past the first 10,000 (linking/domain.MaxLinks; M6
// closeout A-I1) are not in the index: no page has its backlink, and the
// reading view resolves them anew, leading where they resolve still.
func TestAPagesLinksPastTheIndexsBoundLeadWhereTheyResolve(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	tm.createPage(t, "alice", nb, "", "Target")
	last := tm.createPage(t, "alice", nb, "", "Last")
	src := tm.createPage(t, "alice", nb, "", "Source")
	tm.send(t, contentWrite("alice", src, strings.Repeat("[[Target]] ", 10_000)+"[[Last]]\n", 1, ""), http.StatusOK)

	var rows, toLast int
	if err := tm.pool.QueryRow(t.Context(), `SELECT count(*), count(*) FILTER (WHERE resolved_id = $2)
		FROM page_links WHERE source_id = $1`, src, last).Scan(&rows, &toLast); err != nil {
		t.Fatal(err)
	}
	if rows != 10_000 || toLast != 0 {
		t.Errorf("the index has %d links of the page, %d to Last; want 10,000, none", rows, toLast)
	}
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+src+"/view", tm.tokens["bob"], "")
	if status != http.StatusOK {
		t.Fatalf("bob's reading view = %d %s", status, body)
	}
	var v struct {
		HTML string `json:"html"`
	}
	decodeAnswer(t, body, &v)
	if want := `<a class="nw-wikilink" data-nw-node="` + last + `">Last</a>`; !strings.HasSuffix(strings.TrimSpace(v.HTML), want+"</p>") {
		t.Errorf("bob's reading view ends %q, want %s", v.HTML[max(0, len(v.HTML)-120):], want)
	}
	status, body = ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+last+"/backlinks", tm.tokens["bob"], "")
	if status != http.StatusOK || !strings.Contains(body, `"data":[]`) {
		t.Errorf("Last's backlinks = %d %s, want none", status, body)
	}
}
