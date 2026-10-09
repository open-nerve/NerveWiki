package bootstrap

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// The exports against Obsidian (M7/P5 design 3.15): each case of the
// fixture set's resolve/ is a notebook, made through the API, its pages,
// their aliases and its attachments, an image a real PNG; the i-th link
// is the page q<i>, holding only the link, under the parent of the page it
// is written from, as verify-resolve.mjs writes it. Each exports through
// serve as a vault whose every page, attachment, folder and linked
// folder-only page is checked, and where each link's target, as this
// system resolves it, is the file the case names. With
// NWIKI_EXPORT_VAULTS=<dir>, each case's archive and its links as this
// system resolves them, by their paths in the vault, are written there for
// tools/md-fixtures/obsidian/verify-export.mjs to compare with Obsidian.

// exportCase is a case of resolve/ (its README).
type exportCase struct {
	Source  string              `json:"source"`
	Pages   []string            `json:"pages"`
	Aliases map[string][]string `json:"aliases"`
	Assets  []string            `json:"assets"`
	Links   []caseLink          `json:"links"`
}

// caseLink is a link of a case: written from a page, leading to a page or
// an attachment, or to nothing; Source, when set, overrides its case's.
type caseLink struct {
	From   string  `json:"from"`
	Link   string  `json:"link"`
	To     *string `json:"to"`
	Source string  `json:"source"`
}

// casePNG is a 7×5 PNG, verify-resolve.mjs's, for the attachments that are
// images; any other's bytes are a few of text.
const casePNG = "iVBORw0KGgoAAAANSUhEUgAAAAcAAAAFCAYAAACJmvbYAAAAEklEQVR42mM4YWPzHxdmGABJADoKTp7oONgaAAAAAElFTkSuQmCC"

var imageName = regexp.MustCompile(`(?i)\.(png|jpe?g|gif|webp|bmp|avif)$`)

// caseFile is the bytes of the case's attachment name.
func caseFile(t *testing.T, name string) string {
	t.Helper()
	if !imageName.MatchString(name) {
		return "not really\n"
	}
	png, err := base64.StdEncoding.DecodeString(casePNG)
	if err != nil {
		t.Fatal(err)
	}
	return string(png)
}

// splitPath is a case's path's parent's path, "" at the root, and its name.
func splitPath(path string) (parent, name string) {
	if at := strings.LastIndex(path, "/"); at >= 0 {
		return path[:at], path[at+1:]
	}
	return "", path
}

// vaultLink is a link of an exported case as verify-export.mjs reads it:
// the file holding it in the vault, the link, the file this system
// resolves it to, null for none, and whether Obsidian must agree.
type vaultLink struct {
	Query  string  `json:"query"`
	Link   string  `json:"link"`
	Nerve  *string `json:"nerve"`
	Source string  `json:"source"`
}

func TestEveryResolutionCaseExportsAsAVault(t *testing.T) {
	out := os.Getenv("NWIKI_EXPORT_VAULTS")
	tm := newAcmeTeam(t, "", "")
	for _, f := range markdowntest.ResolveCases(t) {
		name := strings.TrimSuffix(f.Name, ".json")
		t.Run(name, func(t *testing.T) {
			var c exportCase
			if err := json.Unmarshal(f.JSON, &c); err != nil {
				t.Fatal(err)
			}
			nb := tm.openNotebook(t, "alice", name)
			ids := map[string]string{"": ""}
			for _, p := range c.Pages {
				parent, title := splitPath(p)
				content := ""
				if aliases := c.Aliases[p]; aliases != nil {
					b, _ := json.Marshal(aliases)
					content = "---\naliases: " + string(b) + "\n---\n"
				}
				ids[p] = tm.createPageWith(t, "alice", nb, ids[parent], title, content)
			}
			for _, a := range c.Assets {
				parent, file := splitPath(a)
				ids[a] = tm.upload(t, "alice", nb, ids[parent], file, caseFile(t, file)).ID
			}
			queries := make([]string, len(c.Links))
			for i, l := range c.Links {
				parent, _ := splitPath(l.From)
				queries[i] = tm.createPageWith(t, "alice", nb, ids[parent], fmt.Sprintf("q%03d", i), l.Link+"\n")
			}

			j := tm.endedJob(t, "alice", tm.startExport(t, "alice", nb, "").ID)
			if j.State != "succeeded" || j.Download == nil || len(j.Problems) != 0 {
				t.Fatalf("the export = %+v, want succeeded without problems", j)
			}
			names, entries, _ := tm.archiveAt(t, j.Download.URL)
			root := name + "/"
			// The nodes' paths in the vault, by id, from meta.json.
			var meta struct {
				Nodes []struct {
					Path string `json:"path"`
					ID   string `json:"id"`
				} `json:"nodes"`
			}
			if err := json.Unmarshal([]byte(entries[root+".nerve/meta.json"].data), &meta); err != nil {
				t.Fatal(err)
			}
			pathOf := map[string]string{}
			for _, n := range meta.Nodes {
				pathOf[n.ID] = n.Path
			}
			links := make([]vaultLink, len(c.Links))
			linked := map[string]bool{}
			for i, l := range c.Links {
				links[i] = vaultLink{Query: pathOf[queries[i]], Link: l.Link, Source: cmp.Or(l.Source, c.Source)}
				var to *string
				if err := tm.pool.QueryRow(context.Background(), "SELECT resolved_id::text FROM page_links WHERE source_id = $1",
					queries[i]).Scan(&to); err != nil {
					t.Fatalf("q%03d's link: %v", i, err)
				}
				if to != nil {
					p, ok := pathOf[*to]
					if !ok {
						t.Fatalf("q%03d leads to %s, which the vault does not hold", i, *to)
					}
					links[i].Nerve, linked[*to] = &p, true
				}
				// The case's target, as its file in the vault: an attachment
				// is its file, a page its .md, written since it is linked.
				var want *string
				if l.To != nil {
					w := *l.To + ".md"
					if slices.Contains(c.Assets, *l.To) {
						w = *l.To
					}
					want = &w
				}
				if (want == nil) != (links[i].Nerve == nil) || want != nil && *want != *links[i].Nerve {
					t.Errorf("%s from %s leads to %s in the vault, want %s", l.Link, l.From, pathOrNone(links[i].Nerve), pathOrNone(want))
				}
			}
			// Every page with content, or without children, is its file; a
			// folder-only page is its file when a link leads to it, its
			// folder's entry otherwise; every attachment is its file, as
			// uploaded.
			for _, p := range c.Pages {
				parentOf := func(q string) bool { parent, _ := splitPath(q); return parent == p }
				children := slices.ContainsFunc(c.Pages, parentOf) || slices.ContainsFunc(c.Assets, parentOf) ||
					slices.ContainsFunc(c.Links, func(l caseLink) bool { return parentOf(l.From) })
				_, file := entries[root+p+".md"]
				_, folder := entries[root+p+"/"]
				switch {
				case c.Aliases[p] != nil || !children:
					if !file {
						t.Errorf("%s.md is not in the vault", p)
					}
				case file != linked[ids[p]] || folder == linked[ids[p]]:
					t.Errorf("the folder-only %s: its file %t, its folder's entry %t; want its file when a link leads to it (%t), its folder's otherwise",
						p, file, folder, linked[ids[p]])
				}
			}
			for _, a := range c.Assets {
				_, file := splitPath(a)
				if got := entries[root+a]; got.data != caseFile(t, file) {
					t.Errorf("%s = %q, want its upload", a, got.data)
				}
			}
			if out == "" {
				return
			}
			if !slices.Contains(names, root+".nerve/meta.json") {
				t.Fatal("no meta.json")
			}
			_, archive := tm.rawArchive(t, j.Download.URL)
			b, err := json.MarshalIndent(map[string]any{"source": c.Source, "links": links}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, name+".zip"), archive, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, name+".json"), b, 0o600); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// rawArchive downloads the archive at address, with no token: its status
// and bytes.
func (tm acmeTeam) rawArchive(t *testing.T, address string) (int, []byte) {
	t.Helper()
	res, body := sendRequest(t, newRequest(t, http.MethodGet, tm.base+address, "", nil))
	return res.StatusCode, body
}

// pathOrNone is the path p points to, or "none".
func pathOrNone(p *string) string {
	if p == nil {
		return "none"
	}
	return *p
}
