package bootstrap

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
)

// The link index through serve (M6/P3 design 3.4, 3.5; M6 design 4.8).

// checkLinks fails t unless the index is its pages' (M6/P3 design 5): each
// page not deleted is indexed at its content's revision, by the current
// extractor, in its notebook; no row is of another page or notebook; and a
// link resolves only to a page not deleted of its notebook.
func checkLinks(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for what, query := range map[string]string{
		"not indexed at its content's revision": fmt.Sprintf(`SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id = n.id
			WHERE n.deleted_at IS NULL AND n.kind = 'page' AND c.deleted_at IS NULL AND NOT EXISTS (
				SELECT 1 FROM indexed_pages i WHERE i.node_id = n.id AND i.notebook_id = n.notebook_id AND i.revision = c.revision
					AND i.extractor = %d)`, linking.Extractor),
		"indexed, not a page or deleted": `SELECT count(*) FROM (
				SELECT node_id AS id, notebook_id FROM indexed_pages UNION ALL SELECT source_id, notebook_id FROM page_links
				UNION ALL SELECT source_id, notebook_id FROM page_tags UNION ALL SELECT source_id, notebook_id FROM page_properties
				UNION ALL SELECT source_id, notebook_id FROM page_aliases
			) r WHERE NOT EXISTS (SELECT 1 FROM nodes n WHERE n.id = r.id AND n.notebook_id = r.notebook_id
				AND n.kind = 'page' AND n.deleted_at IS NULL)`,
		"with a link to a page deleted or elsewhere": `SELECT count(*) FROM page_links l WHERE l.resolved_id IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM nodes n WHERE n.id = l.resolved_id AND n.notebook_id = l.notebook_id
				AND n.kind = 'page' AND n.deleted_at IS NULL)`,
	} {
		if n := count(t, pool, query); n != 0 {
			t.Errorf("%d pages %s, want none", n, what)
		}
	}
}

// createPageWith creates a page titled title with content in the notebook
// as by, under parent ("" for the root), and returns its id.
func (tm acmeTeam) createPageWith(t *testing.T, by, notebookID, parent, title, content string) string {
	t.Helper()
	body := map[string]any{"parent_id": nil, "title": title, "content": content}
	if parent != "" {
		body["parent_id"] = parent
	}
	b, _ := json.Marshal(body)
	status, got := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+notebookID+"/pages", tm.tokens[by], string(b))
	if status != http.StatusCreated {
		t.Fatalf("create %s as %s = %d %s", title, by, status, got)
	}
	checkPages(t, tm.pool)
	return idOf(t, answer{body: got})
}

// resolvedFrom is where the links of the page id resolve, in the order
// written, "" for none.
func (tm acmeTeam) resolvedFrom(t *testing.T, id string) []string {
	t.Helper()
	rows, err := tm.pool.Query(t.Context(), `SELECT coalesce(resolved_id::text, '') FROM page_links WHERE source_id = $1
		ORDER BY range_start`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// resolves fails t unless the links of the page id resolve to want.
func (tm acmeTeam) resolves(t *testing.T, id string, want ...string) {
	t.Helper()
	if got := tm.resolvedFrom(t, id); !slices.Equal(got, want) {
		t.Errorf("the links of %s resolve to %q, want %q", id, got, want)
	}
}

// links fails t unless the stream's next frame is a links event of the
// notebook nb listing pages and targets, in any order; nil is null.
func (s *eventStream) links(t *testing.T, nb string, pages, targets []string) {
	t.Helper()
	f := s.expect(t, "links")
	var got struct {
		NotebookID string    `json:"notebook_id"`
		Pages      *[]string `json:"pages"`
		Targets    *[]string `json:"targets"`
	}
	if err := json.Unmarshal([]byte(f.data), &got); err != nil || got.NotebookID != nb {
		t.Fatalf("%s's stream: links %s, %v; want one of %s", s.name, f.data, err, nb)
	}
	if !sameSet(got.Pages, pages) || !sameSet(got.Targets, targets) {
		t.Fatalf("%s's stream: links %s; want pages %q, targets %q", s.name, f.data, pages, targets)
	}
}

func sameSet(got *[]string, want []string) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	a, b := slices.Clone(*got), slices.Clone(want)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// Each write of the pages keeps the index through serve, and its links
// event reaches a stream that sees the notebook, after the unit's pages
// event: a page created takes the links to its name; a rename takes the
// links to the pages under it by their new paths, and writes again those
// that led to them, whose page's links the index takes (M6/P4); a move the
// links to it by its new path; a content
// written its own links, and the links to its aliases; a task toggled
// writes the content again; a subtree deleted the links to its pages. A
// write that changes no link's resolution publishes no links event.
func TestEveryPageWriteKeepsTheIndex(t *testing.T) {
	tm, nb, marker, s := eventsTeam(t)

	src := tm.createPageWith(t, "alice", nb, "", "Src", "[[Target]] [[Target/Sub]] [[Renamed/Sub]] [[Box/Renamed]] [[Nick]]")
	s.tree(t, nb, map[string]int{src: 1})
	tm.resolves(t, src, "", "", "", "", "")
	target := tm.createPage(t, "alice", nb, "", "Target")
	s.tree(t, nb, map[string]int{target: 1})
	s.links(t, nb, []string{src}, []string{target})
	sub := tm.createPage(t, "alice", nb, target, "Sub")
	s.tree(t, nb, map[string]int{sub: 1})
	s.links(t, nb, []string{src}, []string{sub})
	tm.resolves(t, src, target, sub, "", "", "")

	tm.send(t, nodeRename("alice", target, "Renamed"), http.StatusOK)
	s.tree(t, nb, map[string]int{src: 2})
	s.links(t, nb, []string{}, []string{target, sub})
	tm.wrote(t, src, "[[Renamed]] [[Sub]] [[Renamed/Sub]] [[Box/Renamed]] [[Nick]]", 2)
	tm.resolves(t, src, target, sub, sub, "", "")

	box := tm.createPage(t, "alice", nb, "", "Box")
	s.tree(t, nb, map[string]int{box: 1})
	tm.send(t, nodeMove("alice", target, box), http.StatusOK)
	s.tree(t, nb, map[string]int{})
	s.links(t, nb, []string{src}, []string{target})
	tm.resolves(t, src, target, sub, sub, target, "")

	tm.send(t, contentWrite("alice", target, "---\naliases: [Nick]\n---\n", 1, ""), http.StatusOK)
	s.pages(t, nb)
	s.links(t, nb, []string{src}, []string{target})
	tm.resolves(t, src, target, sub, sub, target, target)

	tasks := tm.createPageWith(t, "alice", nb, "", "Tasks", "- [ ] [[Box]]\n")
	s.tree(t, nb, map[string]int{tasks: 1})
	s.links(t, nb, []string{}, []string{box})
	tm.send(t, taskToggle("alice", tasks, 1, 3, true), http.StatusOK)
	s.pages(t, nb)
	s.links(t, nb, []string{}, []string{box})

	tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
	s.tree(t, nb, map[string]int{})
	s.links(t, nb, []string{src, tasks}, []string{box, target, sub})
	tm.resolves(t, src, "", "", "", "", "")
	tm.resolves(t, tasks, "")

	tm.send(t, contentWrite("alice", src, "[[Tasks]]", 2, ""), http.StatusOK)
	s.pages(t, nb)
	s.links(t, nb, []string{}, []string{tasks})
	tm.quiet(t, nb, marker, s)
	checkPages(t, tm.pool)
}

// A unit's links event lists no more than 20 pages a set: more are null.
func TestALinksEventOfManyPagesListsNone(t *testing.T) {
	tm, nb, _, s := eventsTeam(t)
	var srcs []string
	for i := range 21 {
		srcs = append(srcs, tm.createPageWith(t, "alice", nb, "", fmt.Sprintf("Src %d", i), "[[New]]"))
		s.pages(t, nb)
	}
	n := tm.createPage(t, "alice", nb, "", "New")
	s.tree(t, nb, map[string]int{n: 1})
	s.links(t, nb, nil, []string{n})
	for _, src := range srcs {
		tm.resolves(t, src, n)
	}
}

// A content whose facts PostgreSQL's text would not hold saves, as it did
// before the index: a U+0000 that a Markdown link's %00 or a YAML escape
// writes is kept as U+FFFD; and a target, alias or tag longer than any
// title's key (linking's MaxKey) is kept without its key: such a link
// resolves to none, and such an alias or tag is none.
func TestAContentOfAnyFactsSaves(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	p := tm.createPage(t, "alice", nb, "", "P")
	// Letters at random: PostgreSQL compresses a repeated one into a B-tree
	// entry, and would take it.
	rnd, letters := rand.New(rand.NewPCG(1, 2)), make([]byte, 2800)
	for i := range letters {
		letters[i] = byte('a' + rnd.IntN(26))
	}
	long := string(letters)
	for i, content := range []string{
		"[[" + long + "]] #" + long,
		"---\naliases: [" + long + ", short]\ntags: [" + long + "]\n---\n",
		"---\nn: \"a\\0b\"\n\"k\\0\": [\"\\0\"]\naliases: [\"x\\0\"]\nsrc: \"[[a\\0b]]\"\n---\n[x](a%00b)",
	} {
		c := contentWrite("alice", p, content, i+1, "")
		if status, got := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], c.body); status != http.StatusOK {
			t.Fatalf("content %d = %d %s, want 200", i, status, got)
		}
		checkPages(t, tm.pool)
		switch i {
		case 0:
			if n := count(t, tm.pool, fmt.Sprintf(`SELECT count(*) FROM page_links WHERE source_id = '%s' AND target_key IS NULL
				AND resolved_id IS NULL`, p)); n != 1 || count(t, tm.pool, fmt.Sprintf("SELECT count(*) FROM page_tags WHERE source_id = '%s'", p)) != 0 {
				t.Errorf("a long target's link without a key: %d; want one, and no long tag", n)
			}
		case 1:
			if n := count(t, tm.pool, fmt.Sprintf("SELECT count(*) FROM page_aliases WHERE source_id = '%s' AND alias = 'short'", p)); n != 1 ||
				count(t, tm.pool, fmt.Sprintf("SELECT count(*) FROM page_aliases WHERE source_id = '%s'", p)) != 1 {
				t.Errorf("the aliases kept: want short alone")
			}
		case 2:
			for what, query := range map[string]string{
				"links":      "SELECT count(*) FROM page_links WHERE source_id = '%s' AND target = 'a' || chr(65533) || 'b'",
				"aliases":    "SELECT count(*) FROM page_aliases WHERE source_id = '%s' AND alias = 'x' || chr(65533)",
				"properties": "SELECT count(*) FROM page_properties WHERE source_id = '%s' AND (key = 'k' || chr(65533) OR value->>0 = chr(65533) OR value #>> '{}' = 'a' || chr(65533) || 'b')",
			} {
				if n := count(t, tm.pool, fmt.Sprintf(query, p)); n != map[string]int{"links": 2, "aliases": 1, "properties": 2}[what] {
					t.Errorf("%d %s with U+FFFD for U+0000", n, what)
				}
			}
		}
	}
}
