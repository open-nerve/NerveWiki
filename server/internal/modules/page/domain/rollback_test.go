package domain_test

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// rid is the n-th id of a rollback's test, in the order of n.
func rid(n int) uuid.UUID {
	var id uuid.UUID
	copy(id[:], []byte{0x01, 0x9a, 0x2b, 0x40, 0x00, 0x00, 0x70, 0x00, 0x80})
	id[14], id[15] = byte(n>>8), byte(n)
	return id
}

// under is a free node under parent, none for the root.
func under(n int, parent *uuid.UUID) domain.FreeNode {
	return domain.FreeNode{ID: rid(n), ParentID: parent}
}

// packed checks the steps of a forest packed with max: each step's
// subtrees max nodes at most together, every root a free subtree's whole
// (each node under it in it), no root under another of the pass, and
// none twice; it answers the nodes deleted.
func packed(t *testing.T, forest []domain.FreeNode, steps [][]uuid.UUID, max int) map[uuid.UUID]bool {
	t.Helper()
	parent := map[uuid.UUID]*uuid.UUID{}
	children := map[uuid.UUID][]uuid.UUID{}
	for _, n := range forest {
		parent[n.ID] = n.ParentID
		if n.ParentID != nil {
			children[*n.ParentID] = append(children[*n.ParentID], n.ID)
		}
	}
	subtree := func(root uuid.UUID) []uuid.UUID {
		out, stack := []uuid.UUID{}, []uuid.UUID{root}
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = append(stack[:len(stack)-1], children[id]...)
			out = append(out, id)
		}
		return out
	}
	deleted := map[uuid.UUID]bool{}
	roots := map[uuid.UUID]bool{}
	for i, step := range steps {
		n := 0
		for _, root := range step {
			if _, ok := parent[root]; !ok {
				t.Fatalf("step %d deletes %s, no free node", i, root)
			}
			if roots[root] {
				t.Fatalf("step %d deletes %s again", i, root)
			}
			roots[root] = true
			sub := subtree(root)
			n += len(sub)
			for _, id := range sub {
				deleted[id] = true
			}
		}
		if n > max || len(step) == 0 {
			t.Errorf("step %d holds %d nodes in %d subtrees, want 1 to %d", i, n, len(step), max)
		}
	}
	for root := range roots {
		for p := parent[root]; p != nil; p = parent[*p] {
			if roots[*p] {
				t.Errorf("the pass deletes %s under %s, which it deletes too", root, *p)
			}
		}
	}
	return deleted
}

// PackFreeSubtrees packs the free subtrees into steps of max nodes at
// most, many a step (M11 decision 231), their roots apart; one past max
// waits, the largest parts under it deleted first (M11/P1 design 3.18).
func TestPackFreeSubtrees(t *testing.T) {
	holder := rid(900)
	t.Run("small subtrees share a step, past the bound a new one", func(t *testing.T) {
		// a alone; b with two children; c with one; d under a holder,
		// whose parent is no free node.
		forest := []domain.FreeNode{under(1, nil), under(2, nil), under(3, ptr(rid(2))), under(4, ptr(rid(2))),
			under(5, nil), under(6, ptr(rid(5))), under(7, &holder)}
		steps := domain.PackFreeSubtrees(forest, 4)
		want := [][]uuid.UUID{{rid(1), rid(2)}, {rid(5), rid(7)}}
		if !slices.EqualFunc(steps, want, slices.Equal) {
			t.Fatalf("PackFreeSubtrees() = %v, want %v", steps, want)
		}
		if deleted := packed(t, forest, steps, 4); len(deleted) != len(forest) {
			t.Errorf("deleted %d nodes, want all %d", len(deleted), len(forest))
		}
	})
	t.Run("a subtree past the bound waits, its parts under it first", func(t *testing.T) {
		// r holds s (three nodes) and u (two), six in all; past 4, s and
		// u go this pass, each a step of its own, and r waits.
		forest := []domain.FreeNode{under(10, nil), under(11, ptr(rid(10))), under(12, ptr(rid(11))), under(13, ptr(rid(11))),
			under(14, ptr(rid(10))), under(15, ptr(rid(14)))}
		steps := domain.PackFreeSubtrees(forest, 4)
		want := [][]uuid.UUID{{rid(11)}, {rid(14)}}
		if !slices.EqualFunc(steps, want, slices.Equal) {
			t.Fatalf("PackFreeSubtrees() = %v, want %v", steps, want)
		}
		deleted := packed(t, forest, steps, 4)
		if deleted[rid(10)] || len(deleted) != 5 {
			t.Errorf("deleted %v, want all but r", deleted)
		}
		// The next pass, r alone left, deletes it.
		if next := domain.PackFreeSubtrees([]domain.FreeNode{under(10, nil)}, 4); !slices.EqualFunc(next,
			[][]uuid.UUID{{rid(10)}}, slices.Equal) {
			t.Errorf("the next pass = %v, want r", next)
		}
	})
	t.Run("deep past the bound, the largest parts down the line", func(t *testing.T) {
		// r > s > (t1 of two, t2 of two, t3); past 3 r and s wait.
		forest := []domain.FreeNode{under(20, nil), under(21, ptr(rid(20))), under(22, ptr(rid(21))), under(23, ptr(rid(22))),
			under(24, ptr(rid(21))), under(25, ptr(rid(24))), under(26, ptr(rid(21)))}
		steps := domain.PackFreeSubtrees(forest, 3)
		want := [][]uuid.UUID{{rid(22)}, {rid(24), rid(26)}}
		if !slices.EqualFunc(steps, want, slices.Equal) {
			t.Fatalf("PackFreeSubtrees() = %v, want %v", steps, want)
		}
		packed(t, forest, steps, 3)
	})
	t.Run("none", func(t *testing.T) {
		if steps := domain.PackFreeSubtrees(nil, 4); steps != nil {
			t.Errorf("PackFreeSubtrees(nil) = %v, want none", steps)
		}
	})
	t.Run("random forests", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(7, 11))
		for trial := range 2000 {
			n, max := 1+rng.IntN(60), 1+rng.IntN(12)
			var forest []domain.FreeNode
			for i := range n {
				var parent *uuid.UUID
				switch k := rng.IntN(4); {
				case k == 0 && i > 0:
					parent = ptr(rid(1000 + rng.IntN(i)))
				case k == 1:
					parent = &holder
				}
				forest = append(forest, domain.FreeNode{ID: rid(1000 + i), ParentID: parent})
			}
			// The forest in another order packs alike.
			shuffled := slices.Clone(forest)
			rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
			steps := domain.PackFreeSubtrees(forest, max)
			if again := domain.PackFreeSubtrees(shuffled, max); !slices.EqualFunc(steps, again, slices.Equal) {
				t.Fatalf("trial %d: %v, shuffled %v", trial, steps, again)
			}
			if deleted := packed(t, forest, steps, max); len(deleted) == 0 {
				t.Fatalf("trial %d: a forest of %d deletes nothing", trial, n)
			}
			// Each pass deletes something, so the passes end.
			left := forest
			for passes := 0; len(left) > 0; passes++ {
				if passes > n {
					t.Fatalf("trial %d: no end", trial)
				}
				gone := packed(t, left, domain.PackFreeSubtrees(left, max), max)
				left = slices.DeleteFunc(slices.Clone(left), func(f domain.FreeNode) bool { return gone[f.ID] })
			}
		}
	})
}

// baselineOf is a baseline row of a node at place, a page's at revision of
// content.
func baselineOf(id uuid.UUID, kind domain.Kind, place domain.TreeState, revision int, content string) domain.BaselineNode {
	b := domain.BaselineNode{NodeID: id, Kind: kind, ParentID: place.ParentID, Name: place.Name, SortOrder: place.SortOrder}
	if kind == domain.KindPage {
		b.Revision, b.Hash, b.Bytes = revision, hash(content), len(content)
	}
	return b
}

func nowOf(id uuid.UUID, kind domain.Kind, place domain.TreeState) domain.NodeNow {
	return domain.NodeNow{Node: domain.Node{ID: id, Kind: kind, ParentID: place.ParentID, Name: place.Name,
		NameKey: shared.TitleKey(place.Name), SortOrder: place.SortOrder}}
}

// RollbackState is the change from the baseline to now (M11 design 4.10):
// an item of each baseline node not in place, by parent, name or order
// alone; one of each holder, from nowhere; a version of each baseline
// page whose content differs, on the baseline's version; nothing of a
// node or a page back as it was; every item's Revision 0, none restored;
// past its items' bound page.revert_too_large; past what is left of it,
// the versions of the pages of the lower ids.
func TestRollbackState(t *testing.T) {
	root := func(name string, order float64) domain.TreeState {
		return domain.TreeState{Name: name, SortOrder: order}
	}
	in := func(parent uuid.UUID, name string, order float64) domain.TreeState {
		return domain.TreeState{ParentID: &parent, Name: name, SortOrder: order}
	}
	moved, renamed, reordered, kept, written, writtenBack, attachment, deleted, holder := rid(1), rid(2), rid(3), rid(4), rid(5), rid(6),
		rid(7), rid(8), rid(9)
	baseline := []domain.BaselineNode{
		baselineOf(moved, domain.KindPage, root("Moved", 1), 1, "m"),
		baselineOf(renamed, domain.KindPage, root("Renamed", 2), 1, "r"),
		baselineOf(reordered, domain.KindPage, root("Reordered", 3), 1, "o"),
		baselineOf(kept, domain.KindPage, root("Kept", 4), 1, "k"),
		baselineOf(written, domain.KindPage, in(kept, "Written", 1), 2, "before"),
		baselineOf(writtenBack, domain.KindPage, in(kept, "Back", 2), 2, "same"),
		baselineOf(attachment, domain.KindAsset, in(kept, "a.png", 3), 0, ""),
		baselineOf(deleted, domain.KindPage, root("Deleted", 5), 1, "d"),
	}
	now := map[uuid.UUID]domain.NodeNow{
		moved:       nowOf(moved, domain.KindPage, in(holder, "Moved", 1)),
		renamed:     nowOf(renamed, domain.KindPage, root("Renamed 2", 2)),
		reordered:   nowOf(reordered, domain.KindPage, root("Reordered", 3.5)),
		kept:        nowOf(kept, domain.KindPage, root("Kept", 4)),
		written:     nowOf(written, domain.KindPage, in(kept, "Written", 1)),
		writtenBack: nowOf(writtenBack, domain.KindPage, in(kept, "Back", 2)),
		attachment:  nowOf(attachment, domain.KindAsset, in(holder, "a.png", 3)),
		holder:      nowOf(holder, domain.KindPage, root("Topics", 6)),
	}
	deletedAt := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	now[deleted] = domain.NodeNow{Node: nowOf(deleted, domain.KindPage, root("Deleted", 5)).Node, DeletedAt: &deletedAt}
	contents := map[uuid.UUID]domain.ContentNow{
		moved: {Revision: 1, Hash: hash("m"), Bytes: 1}, renamed: {Revision: 1, Hash: hash("r"), Bytes: 1},
		reordered: {Revision: 1, Hash: hash("o"), Bytes: 1}, kept: {Revision: 1, Hash: hash("k"), Bytes: 1},
		written: {Revision: 5, Hash: hash("after"), Bytes: 5}, writtenBack: {Revision: 7, Hash: hash("same"), Bytes: 4},
	}
	s, err := domain.RollbackState(baseline, []uuid.UUID{holder}, now, contents, domain.MaxRollbackItems)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range s.Items {
		if it.Revision != 0 || it.Restored {
			t.Errorf("item of %s: Revision %d, Restored %v; want 0 and false", it.NodeID, it.Revision, it.Restored)
		}
		before := "nowhere"
		if it.Before != nil {
			before = it.Before.Name
		}
		got = append(got, fmt.Sprintf("%s: %s -> %s", it.NodeID, before, it.After.Name))
	}
	want := []string{
		moved.String() + ": Moved -> Moved", renamed.String() + ": Renamed -> Renamed 2", reordered.String() + ": Reordered -> Reordered",
		attachment.String() + ": a.png -> a.png", holder.String() + ": nowhere -> Topics",
	}
	if !slices.Equal(got, want) {
		t.Errorf("items %v, want %v", got, want)
	}
	if it := s.Items[0]; *it.After.ParentID != holder || it.Before.ParentID != nil {
		t.Errorf("the moved page's item %+v, want from the root to the holder", it)
	}
	if it := s.Items[2]; it.Before.SortOrder != 3 || it.After.SortOrder != 3.5 {
		t.Errorf("the reordered page's item %+v, want from order 3 to 3.5", it)
	}
	if len(s.Versions) != 1 {
		t.Fatalf("versions %+v, want the written page's alone", s.Versions)
	}
	v := s.Versions[0]
	if v.NodeID != written || *v.Base != 2 || v.BaseBytes != 6 || v.Revision != 5 || !bytes.Equal(v.Hash, hash("after")) || v.Rewrite {
		t.Errorf("version %+v, want on the baseline's 2 of 6 bytes, now 5", v)
	}
	if h := s.Hashes[domain.VersionRef{NodeID: written, Revision: 2}]; !bytes.Equal(h, hash("before")) || len(s.Hashes) != 1 {
		t.Errorf("hashes %v, want the baseline's of the written page's version alone", s.Hashes)
	}
	if len(s.Groups) != 0 {
		t.Errorf("groups %v, want none: a rollback restores nothing", s.Groups)
	}
	if plan, err := domain.PlanRevert(withTree(s, now)); err != nil || plan.Count != 0 || len(plan.Missing) != 0 {
		t.Errorf("PlanRevert() = %+v, %v; want a plan", plan, err)
	}

	if _, err := domain.RollbackState(baseline, []uuid.UUID{holder}, now, contents, 4); !errors.Is(err, domain.ErrRevertTooLarge) {
		t.Errorf("five items past 4 = %v, want page.revert_too_large", err)
	}
	// Room for one version of two: the page of the lower id's, the other
	// waiting for a later pass.
	contents[reordered] = domain.ContentNow{Revision: 2, Hash: hash("o2"), Bytes: 2}
	s, err = domain.RollbackState(baseline, []uuid.UUID{holder}, now, contents, 6)
	if err != nil || len(s.Versions) != 1 || s.Versions[0].NodeID != reordered || len(s.Hashes) != 1 {
		t.Errorf("RollbackState() with room for one version = %+v, %v; want the reordered page's alone", s.Versions, err)
	}
}

// withTree is s with the notebook's tree, its nodes alive.
func withTree(s domain.RevertState, now map[uuid.UUID]domain.NodeNow) domain.RevertState {
	for _, n := range now {
		if n.DeletedAt == nil {
			s.Tree = append(s.Tree, domain.RevertTreeNode{ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Name: n.Name, NameKey: n.NameKey,
				SortOrder: n.SortOrder})
		}
	}
	s.At, s.Retention = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC), retention
	return s
}

// rbNode is a node of the rollback's model: where it is, a page's content
// at its revision.
type rbNode struct {
	parent   *uuid.UUID
	name     string
	order    float64
	asset    bool
	alive    bool
	revision int
	hash     []byte
}

func (n rbNode) state() domain.TreeState {
	return domain.TreeState{ParentID: n.parent, Name: n.name, SortOrder: n.order}
}

// rbTrial is a rollback's trial: the model, its baseline and its random
// source.
type rbTrial struct {
	rng      *rand.Rand
	nodes    map[uuid.UUID]*rbNode
	ids      []uuid.UUID
	baseline map[uuid.UUID]rbNode
	names    []string
}

func (p *rbTrial) newNode(n rbNode) uuid.UUID {
	id := rid(len(p.ids) + 1)
	p.ids = append(p.ids, id)
	p.nodes[id] = &n
	return id
}

// alive are the model's nodes alive, in id order.
func (p *rbTrial) alive() []uuid.UUID {
	var out []uuid.UUID
	for _, id := range p.ids {
		if p.nodes[id].alive {
			out = append(out, id)
		}
	}
	return out
}

// free reports whether name's key is no other alive node's under parent.
func (p *rbTrial) free(parent *uuid.UUID, name string, except uuid.UUID) bool {
	key := shared.TitleKey(name)
	for _, id := range p.ids {
		if n := p.nodes[id]; id != except && n.alive && domain.SameParent(n.parent, parent) && shared.TitleKey(n.name) == key {
			return false
		}
	}
	return true
}

// under reports whether id is at or under top.
func (p *rbTrial) under(id, top uuid.UUID) bool {
	for at := &id; at != nil; at = p.nodes[*at].parent {
		if *at == top {
			return true
		}
	}
	return false
}

// depth is how many pages are above id and it.
func (p *rbTrial) depth(id uuid.UUID) int {
	d := 0
	for at := &id; at != nil; at = p.nodes[*at].parent {
		d++
	}
	return d
}

// parentFor is a random place to put a node under: the root or a live
// page.
func (p *rbTrial) parentFor() *uuid.UUID {
	var pages []uuid.UUID
	for _, id := range p.alive() {
		if !p.nodes[id].asset && p.depth(id) < 4 {
			pages = append(pages, id)
		}
	}
	if len(pages) == 0 || p.rng.IntN(4) == 0 {
		return nil
	}
	return ptr(pages[p.rng.IntN(len(pages))])
}

// height is how many levels id's subtree spans, itself included.
func (p *rbTrial) height(id uuid.UUID) int {
	h := 1
	for _, c := range p.alive() {
		if n := p.nodes[c]; n.parent != nil && *n.parent == id {
			h = max(h, 1+p.height(c))
		}
	}
	return h
}

// agent changes the model as the initiator's agent may, through the
// guardrails: moves, renames (to names freed, and the names of siblings
// in a cycle, swaps among them), reorders, new pages and folders holding baseline nodes, writes,
// some back to the baseline's content, and deletes of new subtrees
// holding no baseline node.
func (p *rbTrial) agent(ops int) {
	for range ops {
		live := p.alive()
		id := live[p.rng.IntN(len(live))]
		n := p.nodes[id]
		switch p.rng.IntN(9) {
		case 7, 8: // the names of two or three siblings in a cycle, as the agent's renames through a free name leave them
			var sibs []uuid.UUID
			for _, o := range live {
				if domain.SameParent(p.nodes[o].parent, n.parent) && p.nodes[o].asset == n.asset {
					sibs = append(sibs, o)
				}
			}
			p.rng.Shuffle(len(sibs), func(i, j int) { sibs[i], sibs[j] = sibs[j], sibs[i] })
			if len(sibs) > 3 {
				sibs = sibs[:2+p.rng.IntN(2)]
			}
			if len(sibs) < 2 {
				continue
			}
			first := p.nodes[sibs[0]].name
			for i := range len(sibs) - 1 {
				p.nodes[sibs[i]].name = p.nodes[sibs[i+1]].name
			}
			p.nodes[sibs[len(sibs)-1]].name = first
		case 0, 1: // a move, maybe renamed
			to := p.parentFor()
			name := n.name
			if p.rng.IntN(3) == 0 {
				name = p.names[p.rng.IntN(len(p.names))]
			}
			if to != nil && (p.under(*to, id) || p.depth(*to)+p.height(id) > 5) || !p.free(to, name, id) {
				continue
			}
			n.parent, n.name, n.order = to, name, float64(p.rng.IntN(8))
		case 2: // a rename
			name := p.names[p.rng.IntN(len(p.names))]
			if p.free(n.parent, name, id) {
				n.name = name
			}
		case 3: // a reorder
			n.order += float64(p.rng.IntN(3)) - 1 + 0.5
		case 4: // a new page or folder
			to := p.parentFor()
			name := p.names[p.rng.IntN(len(p.names))]
			if p.free(to, name, uuid.UUID{}) {
				p.newNode(rbNode{parent: to, name: name, order: float64(p.rng.IntN(8)), alive: true, revision: 1, hash: hash("new")})
			}
		case 5: // a write, a third of them back to the baseline's content
			if n.asset {
				continue
			}
			n.revision++
			n.hash = hash(fmt.Sprintf("w%d", p.rng.IntN(1000)))
			if b, ok := p.baseline[id]; ok && p.rng.IntN(3) == 0 {
				n.hash = b.hash
			}
		case 6: // a delete of a new subtree holding no baseline node
			if _, ok := p.baseline[id]; ok || p.holds(id) {
				continue
			}
			for _, c := range p.alive() {
				if p.under(c, id) {
					p.nodes[c].alive = false
				}
			}
		}
	}
}

// holds reports whether a baseline node alive is under id.
func (p *rbTrial) holds(id uuid.UUID) bool {
	for b := range p.baseline {
		if b != id && p.nodes[b].alive && p.under(b, id) {
			return true
		}
	}
	return false
}

// newRollbackTrial is a baseline of 4 to 30 pages and attachments up to
// three levels deep, names drawn from a pool shared with the agent.
func newRollbackTrial(seed uint64) *rbTrial {
	p := &rbTrial{rng: rand.New(rand.NewPCG(seed, 0x51ed)), nodes: map[uuid.UUID]*rbNode{}, baseline: map[uuid.UUID]rbNode{}}
	for i := range 12 {
		p.names = append(p.names, fmt.Sprintf("n%d", i))
	}
	p.names = append(p.names, "a.png", "b.png")
	for range 4 + p.rng.IntN(27) {
		to := p.parentFor()
		name := p.names[p.rng.IntN(len(p.names))]
		if !p.free(to, name, uuid.UUID{}) {
			continue
		}
		asset := p.rng.IntN(5) == 0
		id := p.newNode(rbNode{parent: to, name: name, order: float64(p.rng.IntN(8)), asset: asset, alive: true,
			revision: 1 + p.rng.IntN(3)})
		if !asset {
			p.nodes[id].hash = hash(fmt.Sprintf("base %d", len(p.ids)))
		}
	}
	for id, n := range p.nodes {
		p.baseline[id] = *n
	}
	return p
}

// state is the rollback's state read from the model as a pass of the
// rollback reads it once the free subtrees are deleted (M11/P1 design
// 3.18): the baseline's rows, the holders, the nodes and contents now and
// the tree.
func (p *rbTrial) state(t *testing.T) domain.RevertState {
	t.Helper()
	// The pass that deletes first: every new node alive holding no
	// baseline node, with its subtree.
	for _, id := range p.alive() {
		if _, ok := p.baseline[id]; !ok && !p.holds(id) {
			p.nodes[id].alive = false
		}
	}
	var baseline []domain.BaselineNode
	var holders []uuid.UUID
	now := map[uuid.UUID]domain.NodeNow{}
	contents := map[uuid.UUID]domain.ContentNow{}
	for _, id := range p.ids {
		n := p.nodes[id]
		kind := domain.KindPage
		if n.asset {
			kind = domain.KindAsset
		}
		if b, ok := p.baseline[id]; ok {
			row := domain.BaselineNode{NodeID: id, Kind: kind, ParentID: b.parent, Name: b.name, SortOrder: b.order}
			if !b.asset {
				row.Revision, row.Hash, row.Bytes = b.revision, b.hash, 9
			}
			baseline = append(baseline, row)
		} else if n.alive {
			holders = append(holders, id)
		}
		if !n.alive {
			continue
		}
		now[id] = nowOf(id, kind, n.state())
		if !n.asset {
			contents[id] = domain.ContentNow{Revision: n.revision, Hash: n.hash, Bytes: 9}
		}
	}
	s, err := domain.RollbackState(baseline, holders, now, contents, domain.MaxRollbackItems)
	if err != nil {
		t.Fatal(err)
	}
	return withTree(s, now)
}

// replay does the plan's steps on the model, each checked as the store
// would: a place or a step aside of a node alive under the root or a page
// alive, no cycle, its name free there; a deletion of a new node's whole
// subtree as it is then, no baseline node in it; a write back on the
// page's revision. A restore is none of a rollback's.
func (p *rbTrial) replay(ops []domain.RevertOp) string {
	for i, op := range ops {
		n := p.nodes[op.NodeID]
		switch op.Kind {
		case domain.RevertPlace, domain.RevertAside:
			to := op.To.ParentID
			switch {
			case !n.alive:
				return fmt.Sprintf("step %d places %s, not alive", i, op.NodeID)
			case to != nil && (!p.nodes[*to].alive || p.nodes[*to].asset):
				return fmt.Sprintf("step %d places %s under %s, no page alive", i, op.NodeID, *to)
			case to != nil && p.under(*to, op.NodeID):
				return fmt.Sprintf("step %d places %s under itself", i, op.NodeID)
			case !p.free(to, op.To.Name, op.NodeID):
				return fmt.Sprintf("step %d places %s as %q, taken", i, op.NodeID, op.To.Name)
			}
			n.parent, n.name, n.order = to, op.To.Name, op.To.SortOrder
		case domain.RevertDelete:
			var sub []uuid.UUID
			for _, id := range p.alive() {
				if p.under(id, op.NodeID) {
					sub = append(sub, id)
				}
			}
			got := slices.Clone(op.Subtree)
			slices.SortFunc(got, func(a, b uuid.UUID) int { return a.Compare(b) })
			if !slices.Equal(got, sub) || op.Subtree[0] != op.NodeID {
				return fmt.Sprintf("step %d deletes %v, the subtree of %s is %v", i, op.Subtree, op.NodeID, sub)
			}
			for _, id := range sub {
				if _, ok := p.baseline[id]; ok {
					return fmt.Sprintf("step %d deletes the baseline node %s", i, id)
				}
				p.nodes[id].alive = false
			}
		case domain.RevertContent:
			if !n.alive || n.asset || n.revision != op.Current {
				return fmt.Sprintf("step %d writes %s back on revision %d, it is at %d", i, op.NodeID, op.Current, n.revision)
			}
			n.revision, n.hash = n.revision+1, op.Hash
		default:
			return fmt.Sprintf("step %d is a %s", i, op.Kind)
		}
	}
	return ""
}

// left is what is left of the rollback, judged by the baseline's rows
// directly, not by the synthesis: a baseline node not alive or not in
// place, a baseline page of another content, a node alive with no
// baseline row.
func (p *rbTrial) left() domain.RollbackLeft {
	var l domain.RollbackLeft
	for _, id := range p.ids {
		n := p.nodes[id]
		b, ok := p.baseline[id]
		switch {
		case !ok:
			if n.alive {
				l.Created++
			}
			continue
		case !n.alive || !n.state().Same(b.state()):
			l.Nodes++
		}
		if !b.asset && n.alive && !bytes.Equal(n.hash, b.hash) {
			l.Pages++
		}
	}
	return l
}

// A rollback's plan, one pass after the free subtrees went, puts the
// notebook back as the upgrade started (M11 design 4.10; M11/P1 design
// 2.1 item 12, P1 review A-M3): random baselines and agents (rename
// cycles, moves across levels, new folders holding baseline nodes, names
// swapped, orders changed alone, writes and writes back); PlanRevert never
// ErrRevertUnordered nor a conflict; its steps, replayed on a model of
// the test's own as a store checks them, leave nothing of the difference,
// judged from the baseline's rows: a second pass would have nothing to
// do.
func TestRollbackPlansPutTheBaselineBack(t *testing.T) {
	trials := 5000
	if testing.Short() {
		trials = 500
	}
	var planned, steps, cycles int
	for seed := range trials {
		p := newRollbackTrial(uint64(seed))
		p.agent(5 + p.rng.IntN(40))
		s := p.state(t)
		plan, err := domain.PlanRevert(s)
		switch {
		case err != nil:
			t.Fatalf("seed %d: PlanRevert() = %v", seed, err)
		case plan.Count > 0 || len(plan.Missing) > 0:
			t.Fatalf("seed %d: conflicts %+v, missing %v", seed, plan.Conflicts, plan.Missing)
		}
		if bad := p.replay(plan.Ops); bad != "" {
			t.Fatalf("seed %d: %s", seed, bad)
		}
		if l := p.left(); !l.Zero() {
			t.Fatalf("seed %d: left %+v after one pass, want nothing", seed, l)
		}
		if len(s.Items)+len(s.Versions) > 0 {
			planned++
		}
		steps += len(plan.Ops)
		for _, op := range plan.Ops {
			if op.Kind == domain.RevertAside {
				cycles++
				break
			}
		}
	}
	t.Logf("%d trials, %d with a plan, %d steps, %d with a step aside", trials, planned, steps, cycles)
	if planned < trials/2 || cycles < trials/10 {
		t.Errorf("idle: %d of %d with a plan, %d with a step aside", planned, trials, cycles)
	}
}
