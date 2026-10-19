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
// sequence of a notebook's changes through the API and an agent's moves
// through MCP (M9/P2 design 3.5.4), the newest k
// changesets reverted newest first each succeed, and leave the notebook
// as it was before them: its tree alive, each node's parent, name and
// place among its siblings, its pages' bytes, its link index and its
// attachments' rows. What was in the trash before them still is, its
// time and name perhaps not; what they created is in the trash.

// revertWorld is a notebook alice changes at random through the API and
// her agent, read back through SQL.
type revertWorld struct {
	tm    acmeTeam
	nb    string
	agent *mcpApp // alice's, in nb
	rng   *rand.Rand
	files int
	// made counts the changes made by kind, skipped those refused.
	made, skipped map[string]int
	// batches has change make the agent's batches too, whose changesets
	// batched holds: a revert of one passes them by (M9/P3 design 3.14.6,
	// review C-M4). S1's revert may answer one of them 500, a page written
	// and deleted in one changeset (ErrRevertUnordered, until S2's
	// schedule), so only TestTheSearchIndexIsItsRebuild turns it on in S1.
	batches bool
	batched map[string]bool
}

// newRevertWorld is the world of a notebook Eng of alice's in tm, its
// changes drawn from seed.
func newRevertWorld(t *testing.T, tm acmeTeam, seed uint64) *revertWorld {
	t.Helper()
	nb := tm.openNotebook(t, "alice", "Eng")
	return &revertWorld{tm: tm, nb: nb, agent: tm.agent(t, "alice", nb), rng: rand.New(rand.NewPCG(seed, 0)),
		made: map[string]int{}, skipped: map[string]int{}, batched: map[string]bool{}}
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
// of an earlier changeset, a page moved and renamed in one step by her
// agent, and when w.batches her agent's batch. One the server refuses as a
// conflict, 409, is skipped; any other refusal fails t. It reports whether
// the server answered it.
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
	kinds := 11
	if w.batches {
		kinds++
	}
	op := w.rng.IntN(kinds)
	if len(pages) < 3 {
		op = 0
	}
	kind := [...]string{"creation", "upload", "rename", "move", "reorder", "deletion", "restore", "write", "version", "revert",
		"agent's move", "agent's batch"}[op]
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
		if ids = slices.DeleteFunc(ids, func(id string) bool { return w.batched[id] }); len(ids) == 0 {
			return false
		}
		c = changesetRevert("alice", ids[w.rng.IntN(len(ids))])
	case 10:
		to := title() + ".md"
		if p, ok := parent().(string); ok {
			to = w.pathOf(t, p) + "/" + to
		}
		return w.agentMove(t, kind, w.pathOf(t, pick(pages).id)+".md", to)
	case 11:
		return w.agentBatch(t, kind, pages)
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

// pathOf is the node id's path from the notebook's root, without a page's
// .md: its ancestors' names and its own.
func (w *revertWorld) pathOf(t *testing.T, id string) string {
	t.Helper()
	return queryStrings(t, w.tm.pool, `WITH RECURSIVE up AS (
			SELECT parent_id, name, 1 AS depth FROM nodes WHERE id = $1
			UNION ALL
			SELECT n.parent_id, n.name, up.depth + 1 FROM nodes n JOIN up ON n.id = up.parent_id)
		SELECT string_agg(name, '/' ORDER BY depth DESC) FROM up`, id)[0]
}

// agentMove is the agent's move of the page at from to the path to, a
// change of kind: its refusal of a name taken, of a cycle or of a tree too
// deep, an MCP call's answer rather than a 409, is skipped as a REST
// conflict is, and a move to where the page is, which changes nothing, is
// no change (M9/P2 design 3.5.4).
func (w *revertWorld) agentMove(t *testing.T, kind, from, to string) bool {
	t.Helper()
	a := w.agent.use(t, "move", map[string]any{"from": from, "to": to})
	if a.result(t)["isError"] == true {
		if code := a.toolError(t)["code"]; code != "page.title_taken" && code != "page.cycle" && code != "page.too_deep" {
			t.Fatalf("the agent's move of %s to %s = %.300s, want it done or a conflict", from, to, a.body)
		}
		w.skipped[kind]++
		return false
	}
	if !answerOf[mcpMoved](t, a).Changed {
		return false
	}
	w.made[kind]++
	return true
}

// agentBatch is the agent's batch of two or three ops at random, a change
// of kind (M9/P3 design 3.14.6): a page created at the root, or one of
// pages, each unrelated to the others the batch picks, written on its
// version, renamed, or deleted with what is under it. A refusal of a name
// taken, of a page not found or of what a batch may not do is skipped as a
// REST conflict is; a batch that changes nothing is no change.
func (w *revertWorld) agentBatch(t *testing.T, kind string, pages []liveNode) bool {
	t.Helper()
	var ops []map[string]any
	var picked []string // the paths of the pages the ops act on, without .md
	for range 2 + w.rng.IntN(2) {
		title := revertTitles()[w.rng.IntN(len(revertTitles()))]
		op := w.rng.IntN(4)
		if op == 0 {
			ops = append(ops, opWrite(title+".md", w.content()))
			continue
		}
		n := pages[w.rng.IntN(len(pages))]
		path := w.pathOf(t, n.id)
		if slices.ContainsFunc(picked, func(p string) bool {
			return p == path || strings.HasPrefix(p, path+"/") || strings.HasPrefix(path, p+"/")
		}) {
			continue
		}
		picked = append(picked, path)
		switch op {
		case 1:
			ops = append(ops, opWriteOn(path+".md", w.content(), versionOf(n.id, w.tm.content(t, "alice", n.id).Revision)))
		case 2:
			ops = append(ops, opMove(path+".md", path[:strings.LastIndexByte(path, '/')+1]+title+".md"))
		default:
			ops = append(ops, opDeleteAll(path+".md"))
		}
	}
	if len(ops) == 0 {
		return false
	}
	a := w.agent.batch(t, ops...)
	if a.result(t)["isError"] == true {
		if code := a.toolError(t)["code"]; code != "page.title_taken" && code != "page.not_found" && code != "validation_failed" {
			t.Fatalf("the agent's batch of %v = %.300s, want it done or a conflict", ops, a.body)
		}
		w.skipped[kind]++
		return false
	}
	done := answerOf[mcpBatched](t, a)
	if done.ChangesetID == nil {
		return false
	}
	w.batched[*done.ChangesetID] = true
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
// first by bob; among them, of all the seeds, an agent's move at least.
func TestRevertingTheNewestChangesetsPutsTheNotebookBack(t *testing.T) {
	const before, k = 30, 12
	seeds := []uint64{1, 2, 3, 4}
	ran, agents := 0, 0
	for _, seed := range seeds {
		t.Run(fmt.Sprint("seed ", seed), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			w := newRevertWorld(t, tm, seed)
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
			kinds, reverted := map[string]int{}, w.changesets(t, newest)
			agent := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id::text = ANY($1) AND client LIKE 'mcp:%'", reverted)
			for _, c := range reverted {
				kinds[tm.changeset(t, "bob", c).Kind]++
				tm.reverted(t, "bob", c)
			}
			agents += agent
			t.Logf("changes made %v, skipped %v; the changesets reverted %v, the agent's %d", w.made, w.skipped, kinds, agent)
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
			ran++ // at its end: a seed skipped (go test -short, no database) or stopped by a Fatal is not counted
		})
	}
	// The agent's move and rename in one step is the revert's to put back
	// in one place (M9/P2 design 3.5.4): the seeds revert one at least, when
	// every seed ran to its end.
	if ran == len(seeds) && agents == 0 {
		t.Error("no seed reverted an agent's move: the property does not reach it")
	}
}
