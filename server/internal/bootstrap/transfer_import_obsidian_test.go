package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// The imports against Obsidian (M7/P6 design 3.17): each case of the
// fixture set's resolve/, checked against Obsidian 1.12.7, is a vault
// zipped as verify-resolve.mjs writes it for Obsidian: each page its .md,
// a page with children its folder too, an attachment its file, an image a
// real PNG; the i-th link the page q<i>, holding only the link, beside the
// page it is written from; .obsidian/app.json, all in the vault's folder,
// and macOS's __MACOSX beside it. It imports through serve into a
// notebook of its own, where each page and attachment is at its path and
// each link resolves to the node the case names: what Obsidian resolves
// in the vault, or, for a nerve-defined case, what this system does. The
// vault zipped with its names in NFD and "\" between them imports the
// same.

func TestEveryResolutionCaseImportsFromAVault(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	variants := []struct {
		name  string
		entry func(name string) string
	}{
		{"as zipped", func(name string) string { return name }},
		{"in NFD with backslashes", func(name string) string { return strings.ReplaceAll(norm.NFD.String(name), "/", `\`) }},
	}
	for _, f := range markdowntest.ResolveCases(t) {
		name := strings.TrimSuffix(f.Name, ".json")
		var c exportCase
		if err := json.Unmarshal(f.JSON, &c); err != nil {
			t.Fatal(err)
		}
		for at, v := range variants {
			t.Run(name+"/"+v.name, func(t *testing.T) {
				nb := tm.openNotebook(t, "alice", fmt.Sprintf("%s %d", name, at))
				j := tm.imported(t, "alice", nb, "", caseVault(t, name, c, v.entry))
				pages := len(c.Pages) + len(c.Links) + len(caseFolders(c, false))
				if j.State != "succeeded" || len(j.Problems) != 0 || j.Report.Counts["pages"] != pages ||
					j.Report.Counts["attachments"] != len(c.Assets) {
					t.Fatalf("the import = %+v, report %s; want succeeded, %d pages and %d attachments, no problems", j, describe(j.Report),
						pages, len(c.Assets))
				}
				nodes := map[string]treeNode{}
				for _, n := range notebookTree(t, tm.pool, nb) {
					nodes[n.path] = n
				}
				for _, p := range c.Pages {
					if nodes[p].kind != "page" {
						t.Errorf("%s = %+v, want a page", p, nodes[p])
					}
				}
				for _, a := range c.Assets {
					if nodes[a].kind != "asset" {
						t.Errorf("%s = %+v, want an attachment", a, nodes[a])
					}
				}
				for i, l := range c.Links {
					parent, _ := splitPath(l.From)
					q := nodes[path.Join(parent, fmt.Sprintf("q%03d", i))]
					var to *string
					if err := tm.pool.QueryRow(context.Background(), "SELECT resolved_id::text FROM page_links WHERE source_id = $1",
						q.id).Scan(&to); err != nil {
						t.Fatalf("q%03d's link: %v", i, err)
					}
					got := "none"
					if to != nil {
						got = pathOfID(nodes, *to)
					}
					if want := pathOrNone(l.To); got != want {
						t.Errorf("%s from %s leads to %s, want %s", l.Link, l.From, got, want)
					}
				}
			})
		}
	}
}

// caseVault zips the case c as the vault name, each entry's name through
// entry: its folders, then its pages, attachments and links, as
// verify-resolve.mjs writes them, and Obsidian's settings; macOS's
// AppleDouble files beside them.
func caseVault(t *testing.T, name string, c exportCase, entry func(string) string) []byte {
	t.Helper()
	root := name + "/"
	files := []vaultFile{{entry(root), ""}, {entry(root + ".obsidian/"), ""}, {entry(root + ".obsidian/app.json"), `{"showUnsupportedFiles":true}`}}
	for _, folder := range caseFolders(c, true) {
		files = append(files, vaultFile{entry(root + folder + "/"), ""})
	}
	for _, p := range c.Pages {
		content := ""
		if aliases := c.Aliases[p]; aliases != nil {
			b, _ := json.Marshal(aliases)
			content = "---\naliases: " + string(b) + "\n---\n"
		}
		files = append(files, vaultFile{entry(root + p + ".md"), content}, vaultFile{entry("__MACOSX/" + root + appleDouble(p+".md")), "\x00\x05\x16\x07"})
	}
	for _, a := range c.Assets {
		_, file := splitPath(a)
		files = append(files, vaultFile{entry(root + a), caseFile(t, file)})
	}
	for i, l := range c.Links {
		parent, _ := splitPath(l.From)
		files = append(files, vaultFile{entry(root + path.Join(parent, fmt.Sprintf("q%03d.md", i))), l.Link + "\n"})
	}
	return vaultZip(t, files...)
}

// caseFolders are the case c's folders, each a parent of a page, an
// attachment or a link's page, parents first; with pages, those of a page
// too, else only those no page is.
func caseFolders(c exportCase, pages bool) []string {
	var out []string
	add := func(p string) {
		for parent, _ := splitPath(p); parent != ""; parent, _ = splitPath(parent) {
			if !slices.Contains(out, parent) && (pages || !slices.Contains(c.Pages, parent)) {
				out = append(out, parent)
			}
		}
	}
	for _, p := range slices.Concat(c.Pages, c.Assets) {
		add(p)
	}
	for _, l := range c.Links {
		add(l.From)
	}
	slices.SortFunc(out, func(a, b string) int { return strings.Count(a, "/") - strings.Count(b, "/") })
	return out
}

// appleDouble is the name macOS's AppleDouble file of the file p has
// beside it in __MACOSX.
func appleDouble(p string) string {
	parent, name := splitPath(p)
	return path.Join(parent, "._"+name)
}

// pathOfID is the path of the node id among nodes, or the id when it is
// none of them.
func pathOfID(nodes map[string]treeNode, id string) string {
	for p, n := range nodes {
		if n.id == id {
			return p
		}
	}
	return id
}

// treeNode is a node of a notebook's tree as the tests read it: its
// parent's id, "" at the root; its path, its ancestors' names and its own
// joined by "/".
type treeNode struct {
	id, parent, path, kind string
}

// notebookTree is the notebook nb's nodes not deleted, depth first, each
// parent's children in their order.
func notebookTree(t *testing.T, pool *pgxpool.Pool, nb string) []treeNode {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id::text, coalesce(parent_id::text, ''), kind, name FROM nodes
		WHERE notebook_id = $1 AND deleted_at IS NULL ORDER BY sort_order, id`, nb)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	children := map[string][]treeNode{}
	for rows.Next() {
		var n treeNode
		if err := rows.Scan(&n.id, &n.parent, &n.kind, &n.path); err != nil {
			t.Fatal(err)
		}
		children[n.parent] = append(children[n.parent], n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	var out []treeNode
	var walk func(parent, prefix string)
	walk = func(parent, prefix string) {
		for _, n := range children[parent] {
			n.path = prefix + n.path
			out = append(out, n)
			walk(n.id, n.path+"/")
		}
	}
	walk("", "")
	return out
}
