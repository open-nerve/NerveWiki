package bootstrap

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The revert's property through serve (M8/P3 design 3.12): after a random
// sequence of a notebook's changes through the API, the newest k
// changesets reverted newest first each succeed, and leave the notebook
// as it was before them: its tree alive, each node's parent, name and
// place among its siblings, its pages' bytes, its link index and its
// attachments' rows. What was in the trash before them still is, its
// time and name perhaps not; what they created is in the trash.

// revertWorld is a notebook alice changes at random through the API, read
// back through SQL.
type revertWorld struct {
	tm    acmeTeam
	nb    string
	rng   *rand.Rand
	files int
	// made counts the changes made by kind, skipped those refused.
	made, skipped map[string]int
}

// revertTitles are the pages' titles and the targets of their links: few,
// so that the changes meet, take each other's names and move links.
func revertTitles() []string { return []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta"} }

// liveNode is a node alive: its id, its parent ("" for the root), its
// kind.
type liveNode struct{ id, parent, kind string }

// alive is the notebook's nodes alive, each parent's in their order.
func (w *revertWorld) alive(t *testing.T) []liveNode {
	t.Helper()
	var out []liveNode
	for _, r := range queryStrings(t, w.tm.pool, `SELECT id::text || '|' || coalesce(parent_id::text, '') || '|' || kind FROM nodes
		WHERE notebook_id = $1 AND deleted_at IS NULL ORDER BY parent_id NULLS FIRST, sort_order, id`, w.nb) {
		f := strings.Split(r, "|")
		out = append(out, liveNode{f[0], f[1], f[2]})
	}
	return out
}

// content is a page's content: links to a few of the titles, and a word.
func (w *revertWorld) content() string {
	var s strings.Builder
	titles := revertTitles()
	for range w.rng.IntN(3) {
		fmt.Fprintf(&s, "[[%s]] ", titles[w.rng.IntN(len(titles))])
	}
	fmt.Fprintf(&s, "word %d", w.rng.IntN(1000))
	return s.String()
}

// jsonBody is v as a request's JSON.
func jsonBody(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// change makes one change of the notebook at random as alice: a page's
// creation or an attachment's, a rename, a move, a reorder, a deletion, a
// restore from the trash, a content written, a version restored, a revert
// of an earlier changeset. One the server refuses as a conflict, 409, is
// skipped; any other refusal fails t. It reports whether the server
// answered it.
func (w *revertWorld) change(t *testing.T) bool {
	t.Helper()
	nodes := w.alive(t)
	var pages []liveNode
	for _, n := range nodes {
		if n.kind == "page" {
			pages = append(pages, n)
		}
	}
	pick := func(of []liveNode) liveNode { return of[w.rng.IntN(len(of))] }
	parent := func() any {
		if len(pages) == 0 || w.rng.IntN(3) == 0 {
			return nil
		}
		return pick(pages).id
	}
	title := func() string { titles := revertTitles(); return titles[w.rng.IntN(len(titles))] }
	var c step
	op := w.rng.IntN(10)
	if len(pages) < 3 {
		op = 0
	}
	kind := [...]string{"creation", "upload", "rename", "move", "reorder", "deletion", "restore", "write", "version", "revert"}[op]
	switch op {
	case 1:
		w.files++
		p, _ := parent().(string)
		c = assetUpload(t, "alice", w.nb, p, "f"+strconv.Itoa(w.files)+".png", sevenByFive)
	case 2:
		n := pick(pages)
		c = request("alice", http.MethodPatch, "/api/v0/nodes/"+n.id, jsonBody(map[string]string{"name": title()}))
	case 3:
		n, to := pick(nodes), parent()
		if p, _ := to.(string); p == n.parent {
			return false
		}
		c = request("alice", http.MethodPost, "/api/v0/nodes/"+n.id+"/move", jsonBody(map[string]any{"parent_id": to}))
	case 4:
		n := pick(nodes)
		var siblings []liveNode
		for _, s := range nodes {
			if s.parent == n.parent {
				siblings = append(siblings, s)
			}
		}
		at := slices.IndexFunc(siblings, func(s liveNode) bool { return s.id == n.id })
		// After the sibling before it, or first when it is: no change.
		to := w.rng.IntN(len(siblings) + 1)
		if to == at || to == at+1 {
			return false
		}
		var after any
		if to > 0 {
			after = siblings[to-1].id
		}
		var p any
		if n.parent != "" {
			p = n.parent
		}
		c = request("alice", http.MethodPost, "/api/v0/nodes/"+n.id+"/move", jsonBody(map[string]any{"parent_id": p, "after_id": after}))
	case 5:
		c = nodeDeletion("alice", pick(nodes).id)
	case 6:
		roots := queryStrings(t, w.tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND trash_root ORDER BY id", w.nb)
		if len(roots) == 0 {
			return false
		}
		c = nodeRestore("alice", roots[w.rng.IntN(len(roots))])
	case 7:
		n := pick(pages)
		c = contentWrite("alice", n.id, w.content(), w.tm.content(t, "alice", n.id).Revision, "")
	case 8:
		n := pick(pages)
		rev := w.tm.content(t, "alice", n.id).Revision
		if rev == 1 {
			return false
		}
		c = versionRestore("alice", n.id, 1+w.rng.IntN(rev-1), rev)
	case 9:
		ids := queryStrings(t, w.tm.pool, `SELECT id::text FROM changesets WHERE notebook_id = $1 ORDER BY created_at DESC, id DESC
			LIMIT 20`, w.nb)
		c = changesetRevert("alice", ids[w.rng.IntN(len(ids))])
	default:
		c = request("alice", http.MethodPost, "/api/v0/notebooks/"+w.nb+"/pages",
			jsonBody(map[string]any{"parent_id": parent(), "title": title(), "content": w.content()}))
	}
	status, answer := askTyped(t, w.tm.contract, c.method, w.tm.base+c.path, w.tm.tokens[c.by], c.contentType, c.body)
	if status/100 != 2 && status != http.StatusConflict {
		t.Fatalf("%s %s = %d %s, want it done or a conflict", c.method, c.path, status, answer)
	}
	if status/100 != 2 {
		w.skipped[kind]++
		return false
	}
	w.made[kind]++
	return true
}

// changesets are the notebook's changesets newer than after, newer first.
func (w *revertWorld) changesets(t *testing.T, after string) []string {
	t.Helper()
	ids := queryStrings(t, w.tm.pool, `SELECT id::text FROM changesets WHERE notebook_id = $1 ORDER BY created_at DESC, id DESC`, w.nb)
	at := slices.Index(ids, after)
	if at < 0 {
		t.Fatalf("the changeset %s is not in the log", after)
	}
	return ids[:at]
}

// Of a few seeds, 30 changes, then 12 changesets more, reverted newest
// first by bob.
func TestRevertingTheNewestChangesetsPutsTheNotebookBack(t *testing.T) {
	const before, k = 30, 12
	for _, seed := range []uint64{1, 2, 3, 4} {
		t.Run(fmt.Sprint("seed ", seed), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			w := &revertWorld{tm: tm, nb: tm.openNotebook(t, "alice", "Eng"), rng: rand.New(rand.NewPCG(seed, 0)),
				made: map[string]int{}, skipped: map[string]int{}}
			for done := 0; done < before; {
				if w.change(t) {
					done++
				}
			}
			was := tm.notebookState(t, w.nb)
			trashed := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL", w.nb)
			known := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1", w.nb)
			newest := tm.newest(t, w.nb).ID
			for tries := 0; len(w.changesets(t, newest)) < k; tries++ {
				if tries > 20*k {
					t.Fatalf("%d changes tried, %d changesets made; want %d", tries, len(w.changesets(t, newest)), k)
				}
				w.change(t)
			}
			kinds := map[string]int{}
			for _, c := range w.changesets(t, newest) {
				kinds[tm.changeset(t, "bob", c).Kind]++
				tm.reverted(t, "bob", c)
			}
			t.Logf("changes made %v, skipped %v; the changesets reverted %v", w.made, w.skipped, kinds)
			if now := tm.notebookState(t, w.nb); now != was {
				t.Errorf("the notebook after the reverts:\n%s\nwant as it was:\n%s", now, was)
			}
			for _, id := range trashed {
				if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NOT NULL", id) != 1 {
					t.Errorf("the node %s, in the trash before, is not after", id)
				}
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL AND NOT id::text = ANY($2)",
				w.nb, known); n != 0 {
				t.Errorf("%d nodes the reverted changesets created are alive, want each in the trash", n)
			}
		})
	}
}
