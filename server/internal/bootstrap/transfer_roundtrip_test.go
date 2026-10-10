package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The exports imported again (M7/P6 design 3.17): notebooks of fixed
// seeds grown at random through serve, with pages linking in several
// forms, empty ones, ones with children only, linked or not; attachments
// of random bytes and several types; names not ASCII, and of one title key
// in different folders; siblings in the order insertions and moves left.
// Each exports through serve and its archive imports into a notebook of
// its own, which is the same: each node at its path, of its kind, in its
// place among its siblings; each page's content, byte for byte; each
// attachment's file, byte for byte, and its type; each link resolving to
// the node at the same path, or to none.
func TestAnExportImportsAsItWas(t *testing.T) {
	if testing.Short() {
		t.Skip("notebooks exported and imported through serve")
	}
	tm := newAcmeTeam(t, "", "")
	var grown growth
	defer func() {
		if !t.Failed() && (grown.linkedFolders < 10 || grown.folders < 12 || grown.empty < 15 || grown.moved < 30 ||
			grown.assets < 50 || grown.toAssets < 20) {
			t.Errorf("the notebooks grew %+v: the runs test little of some", grown)
		}
	}()
	for seed := range uint64(24) {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			g := grower{t: t, tm: tm, rnd: rand.New(rand.NewPCG(seed, 7)), nb: tm.openNotebook(t, "alice", fmt.Sprintf("Grown %d", seed))}
			grown.add(g.grow())
			want := tm.shape(t, g.nb)

			exported := tm.endedJob(t, "alice", tm.startExport(t, "alice", g.nb, "").ID)
			if exported.State != "succeeded" || exported.Download == nil || len(exported.Problems) != 0 {
				t.Fatalf("the export = %+v, want succeeded without problems", exported)
			}
			status, archive := tm.rawArchive(t, exported.Download.URL)
			if status != http.StatusOK {
				t.Fatalf("the export's download = %d", status)
			}
			nb := tm.openNotebook(t, "alice", fmt.Sprintf("Imported %d", seed))
			imported := tm.imported(t, "alice", nb, "", archive)
			if imported.State != "succeeded" || len(imported.Problems) != 0 {
				t.Fatalf("the import = %+v, report %s; want succeeded without problems", imported, describe(imported.Report))
			}
			if got := tm.shape(t, nb); !slices.Equal(got, want) {
				t.Errorf("the notebook imported is\n%s\nthe one exported\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// shape is the notebook nb as the roundtrip compares it, a node a line,
// depth first in its order: its path and kind; a page's content, quoted,
// and where its links lead, by path, in their order; an attachment's
// type, size and its file's SHA-256, the file as the store holds it.
func (tm acmeTeam) shape(t *testing.T, nb string) []string {
	t.Helper()
	ctx := context.Background()
	nodes := notebookTree(t, tm.pool, nb)
	paths := map[string]string{}
	for _, n := range nodes {
		paths[n.id] = n.path
	}
	files := storedBlobs(t, tm.storage)
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		line := n.path + " " + n.kind
		switch n.kind {
		case "page":
			var content string
			if err := tm.pool.QueryRow(ctx, "SELECT content FROM page_contents WHERE node_id = $1", n.id).Scan(&content); err != nil {
				t.Fatalf("%s's content: %v", n.path, err)
			}
			var leads []string
			for _, to := range queryStrings(t, tm.pool, "SELECT coalesce(resolved_id::text, '') FROM page_links WHERE source_id = $1 ORDER BY range_start",
				n.id) {
				switch p, ok := paths[to]; {
				case to == "":
					leads = append(leads, "none")
				case ok:
					leads = append(leads, p)
				default:
					leads = append(leads, "elsewhere")
				}
			}
			line += fmt.Sprintf(" %q leads to %s", content, strings.Join(leads, ", "))
		case "asset":
			var blob, mime, size string
			var sum []byte
			if err := tm.pool.QueryRow(ctx, `SELECT id::text, mime, byte_size || coalesce(' ' || width || 'x' || height, ''), sha256
				FROM asset_blobs WHERE node_id = $1 AND deleted_at IS NULL`, n.id).Scan(&blob, &mime, &size, &sum); err != nil {
				t.Fatalf("%s's file: %v", n.path, err)
			}
			data, err := os.ReadFile(files[blob])
			if err != nil {
				t.Fatalf("%s's file: %v", n.path, err)
			}
			if stored := sha256.Sum256(data); !slices.Equal(stored[:], sum) {
				t.Errorf("%s's file's SHA-256 is %x, its row's %x", n.path, stored, sum)
			}
			line += fmt.Sprintf(" %s %s %x", mime, size, sum)
		}
		out = append(out, line)
	}
	return out
}

// growth is what grown notebooks held: pages without content and with
// children, linked or not, empty pages, nodes moved, attachments, links to
// them.
type growth struct {
	folders, linkedFolders, empty, moved, assets, toAssets int
}

func (g *growth) add(o growth) {
	g.folders += o.folders
	g.linkedFolders += o.linkedFolders
	g.empty += o.empty
	g.moved += o.moved
	g.assets += o.assets
	g.toAssets += o.toAssets
}

// grower grows a notebook at random through serve, as alice.
type grower struct {
	t   *testing.T
	tm  acmeTeam
	rnd *rand.Rand
	nb  string
}

// The names nodes are given: of one title key and not ASCII, which the
// tree refuses among siblings and takes in different folders; never one
// the export renames (a page's ending in ".md", or too long).

func (g grower) title() string {
	return pickOf(g.rnd, "Notes", "notes", "NOTES", "Café", "Cafe\u0301 2", "日记", "Straße", "STRASSE", "Plan B", "αβγ", "x-1", "Todo (2)")
}

func (g grower) fileName() string {
	return pickOf(g.rnd, "pic.png", "Pic.PNG", "scan.jpg", "doc.pdf", "notes.txt", "data.bin", "图.png", "Résumé.pdf", "a b.gif")
}

// grow grows the notebook: nodes created under random pages, some moved
// under others or among their siblings, then the pages' contents written,
// linking to where the nodes ended; it tells what the notebook holds.
func (g grower) grow() growth {
	g.t.Helper()
	for range 8 + g.rnd.IntN(10) {
		g.create()
	}
	var out growth
	for range 2 + g.rnd.IntN(4) {
		if g.move() {
			out.moved++
		}
	}
	nodes := notebookTree(g.t, g.tm.pool, g.nb)
	linked := map[string]bool{}
	for _, n := range nodes {
		if n.kind == "page" && g.rnd.IntN(4) > 0 {
			content, to := g.content(nodes)
			g.write(n, content)
			for _, id := range to {
				linked[id] = true
			}
		}
	}
	nodes = notebookTree(g.t, g.tm.pool, g.nb)
	for _, n := range nodes {
		children := slices.ContainsFunc(nodes, func(c treeNode) bool { return strings.HasPrefix(c.path, n.path+"/") })
		switch {
		case n.kind == "asset":
			out.assets++
			if linked[n.id] {
				out.toAssets++
			}
		case g.tm.content(g.t, "alice", n.id).Content != "":
		case !children:
			out.empty++
		case linked[n.id]:
			out.linkedFolders++
		default:
			out.folders++
		}
	}
	return out
}

// create creates a page or uploads an attachment, under the root or a
// page at most three deep: one its parent holds the name of is refused,
// and skipped.
func (g grower) create() {
	g.t.Helper()
	parents := []treeNode{{}}
	for _, n := range notebookTree(g.t, g.tm.pool, g.nb) {
		if n.kind == "page" && strings.Count(n.path, "/") < 3 {
			parents = append(parents, n)
		}
	}
	parent := parents[g.rnd.IntN(len(parents))]
	var c step
	if g.rnd.IntN(3) == 0 {
		name := g.fileName()
		c = assetUpload(g.t, "alice", g.nb, parent.id, name, g.file(name))
	} else {
		body := map[string]any{"parent_id": nil, "title": g.title()}
		if parent.id != "" {
			body["parent_id"] = parent.id
		}
		b, _ := json.Marshal(body)
		c = request("alice", http.MethodPost, "/api/v0/notebooks/"+g.nb+"/pages", string(b))
	}
	status, answer := askTyped(g.t, g.tm.contract, c.method, g.tm.base+c.path, g.tm.tokens["alice"], c.contentType, c.body)
	if status != http.StatusCreated && (status != http.StatusConflict || !strings.Contains(answer, `"page.title_taken"`)) {
		g.t.Fatalf("%s %s = %d %s", c.method, c.path, status, answer)
	}
}

// file is an attachment's random bytes: a PNG's, or not, for an image.
func (g grower) file(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".png") && g.rnd.IntN(2) == 0 {
		png, _ := base64.StdEncoding.DecodeString(casePNG)
		return string(png)
	}
	b := make([]byte, 1+g.rnd.IntN(3000))
	for i := range b {
		b[i] = byte(g.rnd.IntN(256))
	}
	return string(b)
}

// move moves a node under the root or another page, first, after a
// sibling or last: one into its own subtree, too deep, or of a name taken
// there is refused, and tells false.
func (g grower) move() bool {
	g.t.Helper()
	nodes := notebookTree(g.t, g.tm.pool, g.nb)
	if len(nodes) == 0 {
		return false
	}
	n := nodes[g.rnd.IntN(len(nodes))]
	parent := ""
	if pages := slices.DeleteFunc(slices.Clone(nodes), func(p treeNode) bool { return p.kind != "page" }); len(pages) > 0 && g.rnd.IntN(3) > 0 {
		parent = pages[g.rnd.IntN(len(pages))].id
	}
	body := map[string]any{"parent_id": nil}
	if parent != "" {
		body["parent_id"] = parent
	}
	var siblings []treeNode
	for _, s := range nodes {
		if s.id != n.id && s.parent == parent {
			siblings = append(siblings, s)
		}
	}
	switch at := g.rnd.IntN(3); {
	case at == 0:
		body["after_id"] = nil
	case at == 1 && len(siblings) > 0:
		body["after_id"] = siblings[g.rnd.IntN(len(siblings))].id
	}
	b, _ := json.Marshal(body)
	status, answer := ask(g.t, g.tm.contract, http.MethodPost, g.tm.base+"/api/v0/nodes/"+n.id+"/move", g.tm.tokens["alice"], string(b))
	switch status {
	case http.StatusOK:
		return true
	case http.StatusConflict, http.StatusUnprocessableEntity:
		return false
	}
	g.t.Fatalf("move %s = %d %s", n.path, status, answer)
	return false
}

// content is a page's random content: lines of text, not ASCII, ending in
// "\n" or "\r\n", and links to the nodes, by their paths or, a name no
// other node has, by their names, in several forms, or to none; and the
// ids it links to.
func (g grower) content(nodes []treeNode) (string, []string) {
	var b strings.Builder
	var to []string
	keys := map[string]int{}
	for _, n := range nodes {
		keys[shared.TitleKey(nameOf(n.path))]++
	}
	for range g.rnd.IntN(6) {
		b.WriteString(pickOf(g.rnd, "Text", "Ünïcødé  ", "日本語\t", "🙂", "", "  # Heading"))
		if g.rnd.IntN(2) == 0 && len(nodes) > 0 {
			n := nodes[g.rnd.IntN(len(nodes))]
			target := n.path
			if keys[shared.TitleKey(nameOf(n.path))] == 1 && g.rnd.IntN(2) == 0 {
				target = nameOf(n.path)
			}
			switch g.rnd.IntN(4) {
			case 0:
				b.WriteString(" [[" + target + "]]")
			case 1:
				b.WriteString(" [[" + target + "|shown]]")
			case 2:
				b.WriteString(" ![[" + target + "]]")
			default:
				b.WriteString(" [[" + target + "#Heading]]")
			}
			to = append(to, n.id)
		}
		if g.rnd.IntN(8) == 0 {
			b.WriteString(" [[missing]]")
		}
		b.WriteString(pickOf(g.rnd, "\n", "\r\n"))
	}
	return b.String(), to
}

// write writes the page n's content.
func (g grower) write(n treeNode, content string) {
	g.t.Helper()
	c := contentWrite("alice", n.id, content, g.tm.content(g.t, "alice", n.id).Revision, "")
	if status, answer := ask(g.t, g.tm.contract, c.method, g.tm.base+c.path, g.tm.tokens["alice"], c.body); status != http.StatusOK {
		g.t.Fatalf("write %s = %d %s", n.path, status, answer)
	}
}

// nameOf is the last name of a path.
func nameOf(path string) string {
	_, name := splitPath(path)
	return name
}
