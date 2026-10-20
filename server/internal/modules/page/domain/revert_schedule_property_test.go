package domain_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The schedule ends and puts the notebook back (M9/P3 design 3.16.4,
// 3.21.1): a random tree, a batch on it as 4.11 lets one be (each
// operation on the tree as it is then, B1, B3, a deletion of a child then
// of its parent one group, a write then a deletion), its changeset
// reverted, that revert reverted, four levels down. The test judges by a
// model of its own, never the simulation schedule() moves: each step the
// plan answers is done on the model in turn, checked as a store would
// check it (its parent alive, no cycle, its name free, a restore's group
// and states as its deletion left them, a write back on the page's
// revision, of the version's content) and the model counted no deeper
// than MaxDepth after it (T5); each node renamed aside, moved aside and
// placed at most once each, its place after its steps aside (T1); the
// steps aside at most 2N; the schedule never ErrRevertUnordered (T2-T4).
// After each level the model is the notebook before the changeset: the
// tree, the contents of its pages, and of those in the trash. The next
// level's changeset is the one those steps record. Five settings in
// turn: short batches at MaxDepth 2, 3 and 4, long ones at 3 and 4. It
// is not idle: the first fallback, the second, and a write back before a
// deletion each come up at least as often as the simulation counts them
// for as many trials; the third, a defence no reachable state needs, is
// not counted (TestTheThirdFallbackMovesAPlaceOffTheRoot tests it).
func TestScheduleEndsAndPutsTheNotebookBack(t *testing.T) {
	trials, secondFloor := 20_000, 50
	if testing.Short() {
		trials, secondFloor = 2_000, 20
	}
	settings := []struct {
		long     bool
		maxDepth int
	}{{false, 2}, {false, 3}, {false, 4}, {true, 3}, {true, 4}}
	var counts propCounts
	for seed := range trials {
		s := settings[seed%len(settings)]
		p := newPropTrial(uint64(seed), s.long, s.maxDepth)
		if bad := p.run(&counts); bad != "" {
			t.Fatalf("seed %d (long %v, MaxDepth %d): %s", seed, s.long, s.maxDepth, bad)
		}
	}
	t.Logf("%d trials, %d reverts: renamed aside %d, moved aside from a deletion %d, moved aside otherwise %d, "+
		"written back then deleted %d; most steps aside in a revert %d, most steps %d", trials, counts.reverts, counts.first,
		counts.second, counts.third, counts.writeThenDelete, counts.mostAside, counts.mostSteps)
	if counts.first < 50 || counts.second < secondFloor || counts.writeThenDelete < 50 {
		t.Errorf("idle: renamed aside %d (want 50), moved aside from a deletion %d (want %d), written back then deleted %d "+
			"(want 50)", counts.first, counts.second, secondFloor, counts.writeThenDelete)
	}
}

// propCounts are what the trials did: reverts planned; the steps aside by
// fallback, the first renaming a holder where it is, the second moving a
// node from under one the revert deletes, the third any other move; the
// write backs of a page a later step deletes; and the most steps aside
// and steps of one revert.
type propCounts struct {
	reverts, first, second, third, writeThenDelete, mostAside, mostSteps int
}

// propNode is a node of the model, every one a page: where it is, alive
// or in the trash item of the root trash, and its content at its
// revision.
type propNode struct {
	parent   *uuid.UUID
	name     string
	order    float64
	alive    bool
	trash    uuid.UUID
	revision int
	content  string
}

func (n propNode) state() domain.TreeState {
	return domain.TreeState{ParentID: n.parent, Name: n.name, SortOrder: n.order}
}

// propTrash is a trash item of the model: when it was deleted, and each
// member where its deletion left it.
type propTrash struct {
	at      time.Time
	members map[uuid.UUID]domain.TreeState
}

// propModel is a notebook in the model: its nodes and its trash items by
// root. A trash item is never changed, only made and taken.
type propModel struct {
	nodes map[uuid.UUID]propNode
	trash map[uuid.UUID]propTrash
}

func (m propModel) clone() propModel {
	out := propModel{nodes: make(map[uuid.UUID]propNode, len(m.nodes)), trash: make(map[uuid.UUID]propTrash, len(m.trash))}
	for id, n := range m.nodes {
		out.nodes[id] = n
	}
	for id, tr := range m.trash {
		out.trash[id] = tr
	}
	return out
}

// propTrial is a trial: its random source and setting; every version a
// page wrote, its content and hash, by page and revision; the nodes it
// made, in id order; and the keys of the names it met.
type propTrial struct {
	rng      *rand.Rand
	long     bool
	maxDepth int
	versions map[domain.VersionRef]string
	hashes   map[domain.VersionRef][]byte
	ids      []uuid.UUID
	keys     map[string]string
	at       time.Time
}

func newPropTrial(seed uint64, long bool, maxDepth int) *propTrial {
	return &propTrial{rng: rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15)), long: long, maxDepth: maxDepth,
		versions: map[domain.VersionRef]string{}, hashes: map[domain.VersionRef][]byte{}, keys: map[string]string{},
		at: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)}
}

func (p *propTrial) between(lo, hi int) int { return lo + p.rng.IntN(hi-lo+1) }

func (p *propTrial) key(name string) string {
	k, ok := p.keys[name]
	if !ok {
		k = shared.TitleKey(name)
		p.keys[name] = k
	}
	return k
}

// newID is a node's id: a UUIDv7 of one millisecond, as a batch's are, in
// the order of the nodes made, its last 62 bits at random.
func (p *propTrial) newID() uuid.UUID {
	var id uuid.UUID
	copy(id[:6], []byte{0x01, 0x92, 0xf3, 0xa4, 0x5b, 0x6c})
	n := len(p.ids)
	id[6], id[7], id[8] = 0x70|byte(n>>8&0x0f), byte(n), 0x80|byte(p.rng.IntN(64))
	for i := 9; i < len(id); i++ {
		id[i] = byte(p.rng.IntN(256))
	}
	p.ids = append(p.ids, id)
	return id
}

// write records the page id's content at revision, a version.
func (p *propTrial) write(id uuid.UUID, revision int, content string) {
	ref := domain.VersionRef{NodeID: id, Revision: revision}
	p.versions[ref], p.hashes[ref] = content, hash(content)
}

// The model's tree, its alive nodes'.

func (p *propTrial) aliveOrRoot(m propModel, parent *uuid.UUID) bool {
	return parent == nil || m.nodes[*parent].alive
}

func (p *propTrial) holder(m propModel, parent *uuid.UUID, key string) (uuid.UUID, bool) {
	for _, id := range p.ids {
		if n := m.nodes[id]; n.alive && domain.SameParent(n.parent, parent) && p.key(n.name) == key {
			return id, true
		}
	}
	return uuid.UUID{}, false
}

func (p *propTrial) children(m propModel, parent uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range p.ids {
		if n := m.nodes[id]; n.alive && n.parent != nil && *n.parent == parent {
			out = append(out, id)
		}
	}
	return out
}

func (p *propTrial) subtree(m propModel, id uuid.UUID) []uuid.UUID {
	out := []uuid.UUID{id}
	for at := 0; at < len(out); at++ {
		out = append(out, p.children(m, out[at])...)
	}
	return out
}

// within reports whether at is id or under it.
func (p *propTrial) within(m propModel, at, id uuid.UUID) bool {
	for range len(p.ids) + 1 {
		if at == id {
			return true
		}
		n := m.nodes[at]
		if n.parent == nil {
			return false
		}
		at = *n.parent
	}
	return true
}

// depthUnder is the depth of a node under parent: a root's is 1.
func (p *propTrial) depthUnder(m propModel, parent *uuid.UUID) int {
	d := 1
	for at := parent; at != nil && d <= len(p.ids)+1; d++ {
		at = m.nodes[*at].parent
	}
	return d
}

func (p *propTrial) height(m propModel, id uuid.UUID) int {
	h := 0
	for _, c := range p.children(m, id) {
		h = max(h, p.height(m, c))
	}
	return h + 1
}

// deepest is the depth of the deepest node alive.
func (p *propTrial) deepest(m propModel) int {
	d := 0
	for _, id := range p.ids {
		if n := m.nodes[id]; n.alive {
			d = max(d, p.depthUnder(m, n.parent))
		}
	}
	return d
}

func (p *propTrial) alive(m propModel) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range p.ids {
		if m.nodes[id].alive {
			out = append(out, id)
		}
	}
	return out
}

// create makes a page under parent named name, its first content its
// version 1.
func (p *propTrial) create(m propModel, parent *uuid.UUID, name string) uuid.UUID {
	id := p.newID()
	content := "c" + id.String()
	m.nodes[id] = propNode{parent: parent, name: name, order: float64(len(p.ids)), alive: true, revision: 1, content: content}
	p.write(id, 1, content)
	return id
}

// propSlot is a place a node left: its parent and name.
type propSlot struct {
	parent *uuid.UUID
	name   string
}

// batch is a random tree, T0, and a batch on it: the notebook after it and
// its changeset, as the store records it.
func (p *propTrial) batch() (t0, now propModel, items []domain.ChangesetItem, versions []domain.ChangesetVersion) {
	m := propModel{nodes: map[uuid.UUID]propNode{}, trash: map[uuid.UUID]propTrash{}}
	size := p.between(3, 6)
	if p.long {
		size = p.between(5, 12)
	}
	for range size {
		parents := append([]*uuid.UUID{nil}, pointers(p.ids)...)
		parent := parents[p.rng.IntN(len(parents))]
		if p.depthUnder(m, parent) > p.maxDepth {
			continue
		}
		name := string("abc"[p.rng.IntN(3)])
		if _, held := p.holder(m, parent, p.key(name)); held {
			continue
		}
		p.create(m, parent, name)
	}
	t0 = m.clone()
	created, moved, written := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	groups := map[uuid.UUID]map[uuid.UUID]domain.TreeState{}
	var freed []propSlot
	serial := 0

	namePick := func() string {
		if p.rng.Float64() < 0.12 && len(p.ids) > 0 {
			j := p.ids[p.rng.IntN(len(p.ids))]
			letter, digits := string("abc"[p.rng.IntN(3)]), hex.EncodeToString(j[:])
			if p.rng.IntN(2) == 0 {
				digits = digits[len(digits)-8:]
			}
			return letter + " ~" + digits
		}
		return string("abct"[p.rng.IntN(4)])
	}
	okMove := func(x uuid.UUID, parent *uuid.UUID, name string) bool {
		n := m.nodes[x]
		if parent != nil && (!m.nodes[*parent].alive || p.within(m, *parent, x)) {
			return false
		}
		if h, held := p.holder(m, parent, p.key(name)); held && h != x || domain.SameParent(n.parent, parent) && n.name == name {
			return false
		}
		return p.depthUnder(m, parent)+p.height(m, x)-1 <= p.maxDepth
	}
	move := func(x uuid.UUID, parent *uuid.UUID, name string) bool {
		if !okMove(x, parent, name) {
			return false
		}
		n := m.nodes[x]
		freed = append(freed, propSlot{n.parent, n.name})
		n.parent, n.name = parent, name
		m.nodes[x], moved[x] = n, true
		return true
	}
	create := func(parent *uuid.UUID, name string) {
		if _, held := p.holder(m, parent, p.key(name)); p.aliveOrRoot(m, parent) && !held && p.depthUnder(m, parent) <= p.maxDepth {
			id := p.create(m, parent, name)
			created[id], written[id] = true, true
		}
	}
	write := func(x uuid.UUID) {
		serial++
		n := m.nodes[x]
		n.revision++
		n.content = fmt.Sprintf("w%s.%d", x, serial)
		m.nodes[x], written[x] = n, true
		p.write(x, n.revision, n.content)
	}
	touchedByB := func(ids []uuid.UUID) bool {
		return slices.ContainsFunc(ids, func(id uuid.UUID) bool { return created[id] || moved[id] })
	}
	del := func(x uuid.UUID) {
		sub := p.subtree(m, x)
		var absorbed []uuid.UUID
		for _, top := range p.ids {
			if g, ok := groups[top]; ok && g[top].ParentID != nil && slices.Contains(sub, *g[top].ParentID) {
				absorbed = append(absorbed, top)
			}
		}
		members := map[uuid.UUID]domain.TreeState{}
		all := slices.Clone(sub)
		for _, top := range absorbed {
			for id, s := range groups[top] {
				members[id] = s
				all = append(all, id)
			}
		}
		if touchedByB(all) { // B1, B3
			return
		}
		for _, top := range absorbed {
			delete(groups, top)
		}
		for _, id := range sub {
			n := m.nodes[id]
			members[id], n.alive = n.state(), false
			m.nodes[id] = n
		}
		groups[x] = members
		freed = append(freed, propSlot{members[x].ParentID, members[x].Name})
	}

	ops := p.between(3, 12)
	if p.long {
		ops = p.between(8, 24)
	}
	for range ops {
		alive := p.alive(m)
		if len(alive) == 0 {
			break
		}
		pick := func() uuid.UUID { return alive[p.rng.IntN(len(alive))] }
		switch p.rng.IntN(10) {
		case 0: // recreate a name a node left
			if len(freed) > 0 {
				s := freed[p.rng.IntN(len(freed))]
				create(s.parent, s.name)
			}
		case 1: // into a page the batch made
			var made []uuid.UUID
			for _, id := range alive {
				if created[id] {
					made = append(made, id)
				}
			}
			if len(made) > 0 {
				x := pick()
				move(x, &made[p.rng.IntN(len(made))], m.nodes[x].name)
			}
		case 2: // out, to the root or a level up
			x := pick()
			if parent := m.nodes[x].parent; parent != nil {
				to := []*uuid.UUID{nil, m.nodes[*parent].parent}[p.rng.IntN(2)]
				move(x, to, m.nodes[x].name)
			}
		case 3: // two siblings' names swapped through "t"
			var parents []*uuid.UUID
			siblings := map[uuid.UUID][]uuid.UUID{}
			for _, id := range alive {
				at := m.nodes[id].parent
				k := uuid.UUID{}
				if at != nil {
					k = *at
				}
				if len(siblings[k]) == 0 {
					parents = append(parents, at)
				}
				siblings[k] = append(siblings[k], id)
			}
			var full []*uuid.UUID
			for _, at := range parents {
				k := uuid.UUID{}
				if at != nil {
					k = *at
				}
				if len(siblings[k]) >= 2 {
					full = append(full, at)
				}
			}
			if len(full) > 0 {
				at := full[p.rng.IntN(len(full))]
				k := uuid.UUID{}
				if at != nil {
					k = *at
				}
				ids := siblings[k]
				i := p.rng.IntN(len(ids))
				j := (i + 1 + p.rng.IntN(len(ids)-1)) % len(ids)
				a, b := ids[i], ids[j]
				ka, kb := m.nodes[a].name, m.nodes[b].name
				if move(a, at, "t") && move(b, at, ka) {
					move(a, at, kb)
				}
			}
		case 4: // to a place a node left
			if len(freed) > 0 {
				s := freed[p.rng.IntN(len(freed))]
				move(pick(), s.parent, s.name)
			}
		case 5:
			del(pick())
		case 6:
			parents := append([]*uuid.UUID{nil}, pointers(alive)...)
			move(pick(), parents[p.rng.IntN(len(parents))], namePick())
		case 7:
			parents := append([]*uuid.UUID{nil}, pointers(alive)...)
			create(parents[p.rng.IntN(len(parents))], namePick())
		case 8:
			write(pick())
		case 9: // written, then deleted
			x := pick()
			if !touchedByB(p.subtree(m, x)) {
				write(x)
				del(x)
			}
		}
	}
	for top, g := range groups {
		m.trash[top] = propTrash{at: p.at, members: g}
		for id := range g {
			n := m.nodes[id]
			n.trash = top
			m.nodes[id] = n
		}
	}
	items, versions = p.changesetOf(t0, m, written)
	return t0, m, items, versions
}

func pointers(ids []uuid.UUID) []*uuid.UUID {
	out := make([]*uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = &id
	}
	return out
}

// changesetOf is the changeset that took the notebook from pre to post, as
// the store records it: an item for each node alive in one and not the
// other, or elsewhere, a node alive again restored, a page's the revision
// it left the content at when it made, restored, wrote or deleted it;
// and a version for each page written, on its revision in pre, none for
// one it made.
func (p *propTrial) changesetOf(pre, post propModel, written map[uuid.UUID]bool) ([]domain.ChangesetItem, []domain.ChangesetVersion) {
	var items []domain.ChangesetItem
	var versions []domain.ChangesetVersion
	stateIn := func(m propModel, id uuid.UUID) *domain.TreeState {
		if n, ok := m.nodes[id]; ok && n.alive {
			return new(n.state())
		}
		return nil
	}
	for _, id := range p.ids {
		b, a := stateIn(pre, id), stateIn(post, id)
		if b == nil && a == nil || b != nil && a != nil && b.Same(*a) {
			continue
		}
		_, existed := pre.nodes[id]
		it := domain.ChangesetItem{NodeID: id, Before: b, After: a, Restored: b == nil && existed}
		if a == nil || b == nil || written[id] {
			it.Revision = post.nodes[id].revision
		}
		items = append(items, it)
	}
	for _, id := range p.ids {
		if !written[id] {
			continue
		}
		n := post.nodes[id]
		v := domain.ChangesetVersion{NodeID: id, Revision: n.revision, Hash: p.hashes[domain.VersionRef{NodeID: id, Revision: n.revision}]}
		if was, ok := pre.nodes[id]; ok {
			v.Base = new(was.revision)
		}
		versions = append(versions, v)
	}
	return items, versions
}

// state is the notebook m as PlanRevert reads it for the changeset at at.
func (p *propTrial) state(m propModel, items []domain.ChangesetItem, versions []domain.ChangesetVersion, at time.Time) domain.RevertState {
	s := domain.RevertState{Items: items, Versions: versions, Nodes: map[uuid.UUID]domain.NodeNow{}, Contents: map[uuid.UUID]domain.ContentNow{},
		Hashes: p.hashes, Groups: map[uuid.UUID][]domain.TrashMember{}, At: at, Retention: retention}
	node := func(id uuid.UUID, at domain.TreeState) domain.Node {
		return domain.Node{ID: id, ParentID: at.ParentID, Kind: domain.KindPage, Name: at.Name, NameKey: p.key(at.Name), SortOrder: at.SortOrder}
	}
	for _, id := range p.ids {
		n, ok := m.nodes[id]
		if !ok || !n.alive {
			continue
		}
		s.Contents[id] = domain.ContentNow{Revision: n.revision, Hash: p.hashes[domain.VersionRef{NodeID: id, Revision: n.revision}]}
		if len(items) > 0 {
			s.Tree = append(s.Tree, node(id, n.state()))
		}
	}
	touch := func(id uuid.UUID) {
		n := m.nodes[id]
		now := domain.NodeNow{Node: node(id, n.state())}
		if !n.alive {
			tr := m.trash[n.trash]
			now.DeletedAt = new(tr.at)
			if n.trash == id {
				now.TrashRoot, now.TrashPages = true, len(tr.members)
			}
		}
		s.Nodes[id] = now
	}
	for _, it := range items {
		touch(it.NodeID)
	}
	for _, v := range versions {
		touch(v.NodeID)
	}
	for root := range domain.RevertGroups(items, s.Nodes) {
		tr := m.trash[root]
		level := func(id uuid.UUID) int {
			l := 1
			for at := tr.members[id]; id != root; l++ {
				id = *at.ParentID
				at = tr.members[id]
			}
			return l
		}
		var g []domain.TrashMember
		for id, at := range tr.members {
			g = append(g, domain.TrashMember{Node: node(id, at), Level: level(id), Revision: m.nodes[id].revision})
		}
		slices.SortFunc(g, func(a, b domain.TrashMember) int {
			if a.Level != b.Level {
				return a.Level - b.Level
			}
			return a.Node.ID.Compare(b.Node.ID)
		})
		s.Groups[root] = g
	}
	return s
}

// run is the trial: a batch, then its revert and each revert's revert,
// four levels, judged on the model; "" when all is well, else what went
// wrong.
func (p *propTrial) run(counts *propCounts) string {
	pre, now, items, versions := p.batch()
	if len(items) == 0 && len(versions) == 0 {
		return ""
	}
	for level := 1; level <= 4; level++ {
		at := p.at.Add(time.Duration(level) * time.Minute)
		plan, err := domain.PlanRevertWithin(p.state(now, items, versions, at), p.maxDepth)
		switch {
		case err != nil:
			return fmt.Sprintf("R%d: PlanRevert = %v", level, err)
		case plan.Count != 0 || len(plan.Missing) != 0:
			return fmt.Sprintf("R%d: conflicts %+v, missing %v", level, plan.Conflicts, plan.Missing)
		}
		after := now.clone()
		touched := map[uuid.UUID]bool{}
		for _, it := range items {
			touched[it.NodeID] = true
		}
		for _, v := range versions {
			touched[v.NodeID] = true
		}
		written, bad := p.judge(pre, after, plan, at, len(touched), counts)
		if bad != "" {
			return fmt.Sprintf("R%d: %s; steps %s", level, bad, p.describe(now, plan))
		}
		if bad := p.compare(pre, after); bad != "" {
			return fmt.Sprintf("R%d: %s; steps %s", level, bad, p.describe(now, plan))
		}
		items, versions = p.changesetOf(now, after, written)
		pre, now = now, after
	}
	return ""
}

// judge does the plan's steps on m in turn, each checked as the store
// checks it, the model counted after it: the pages it wrote back, or
// what went wrong. pre is the notebook before the changeset, S*.
func (p *propTrial) judge(pre, m propModel, plan domain.RevertPlan, at time.Time, touched int, counts *propCounts) (map[uuid.UUID]bool, string) {
	counts.reverts++
	written := map[uuid.UUID]bool{}
	renamedAside, movedAside, placed := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	aside := 0
	for i, op := range plan.Ops {
		if op.Seq != i {
			return nil, fmt.Sprintf("step %d's Seq %d", i, op.Seq)
		}
		switch op.Kind {
		case domain.RevertAside:
			aside++
			switch {
			case placed[op.NodeID]:
				return nil, fmt.Sprintf("step %d: a step aside of a node placed", i)
			case !op.Move && renamedAside[op.NodeID], op.Move && movedAside[op.NodeID]:
				return nil, fmt.Sprintf("step %d: a node renamed aside twice, or moved aside twice", i)
			case op.Move && op.To.ParentID != nil:
				return nil, fmt.Sprintf("step %d: moved aside under a parent", i)
			case !op.Move:
				counts.first++
			default:
				parent := m.nodes[op.NodeID].parent
				if parent != nil && !pre.nodes[*parent].alive {
					counts.second++
				} else {
					counts.third++
				}
			}
			// A node moved aside is renamed too: no step renames it aside
			// again (T1).
			renamedAside[op.NodeID] = true
			movedAside[op.NodeID] = movedAside[op.NodeID] || op.Move
			if bad := p.place(m, op); bad != "" {
				return nil, fmt.Sprintf("step %d: %s", i, bad)
			}
		case domain.RevertPlace:
			if placed[op.NodeID] {
				return nil, fmt.Sprintf("step %d: a node placed twice", i)
			}
			placed[op.NodeID] = true
			if bad := p.place(m, op); bad != "" {
				return nil, fmt.Sprintf("step %d: %s", i, bad)
			}
		case domain.RevertDelete:
			n := m.nodes[op.NodeID]
			if !n.alive {
				return nil, fmt.Sprintf("step %d: a deletion of a node not alive", i)
			}
			sub := p.subtree(m, op.NodeID)
			if slices.ContainsFunc(sub, func(id uuid.UUID) bool { return written[id] }) {
				counts.writeThenDelete++
			}
			members := map[uuid.UUID]domain.TreeState{}
			for _, id := range sub {
				n := m.nodes[id]
				members[id], n.alive, n.trash = n.state(), false, op.NodeID
				m.nodes[id] = n
			}
			m.trash[op.NodeID] = propTrash{at: at, members: members}
		case domain.RevertRestore:
			if bad := p.restore(m, op); bad != "" {
				return nil, fmt.Sprintf("step %d: %s", i, bad)
			}
		case domain.RevertContent:
			n := m.nodes[op.NodeID]
			ref := domain.VersionRef{NodeID: op.NodeID, Revision: op.Base}
			content, ok := p.versions[ref]
			switch {
			case !n.alive:
				return nil, fmt.Sprintf("step %d: a write back of a page not alive", i)
			case op.Current != n.revision:
				return nil, fmt.Sprintf("step %d: a write back on revision %d of a page at %d", i, op.Current, n.revision)
			case !ok || !bytes.Equal(op.Hash, p.hashes[ref]):
				return nil, fmt.Sprintf("step %d: a write back of version %d, not the hash it has", i, op.Base)
			case written[op.NodeID]:
				return nil, fmt.Sprintf("step %d: a page written back twice", i)
			}
			n.revision++
			n.content = content
			m.nodes[op.NodeID], written[op.NodeID] = n, true
			p.write(op.NodeID, n.revision, content)
		default:
			return nil, fmt.Sprintf("step %d: a step of kind %q", i, op.Kind)
		}
		if d := p.deepest(m); d > p.maxDepth {
			return nil, fmt.Sprintf("step %d: a page %d deep", i, d)
		}
	}
	for id := range renamedAside {
		if !placed[id] {
			return nil, "a node stepped aside, never placed"
		}
	}
	if aside > 2*touched {
		return nil, fmt.Sprintf("%d steps aside, past 2N of %d", aside, touched)
	}
	counts.mostAside, counts.mostSteps = max(counts.mostAside, aside), max(counts.mostSteps, len(plan.Ops))
	return written, ""
}

// place does a place or a step aside on m: the node alive, its parent
// alive or the root, not under itself, its name free there, a move and a
// rename as it says.
func (p *propTrial) place(m propModel, op domain.RevertOp) string {
	n := m.nodes[op.NodeID]
	to := op.To
	switch {
	case !n.alive:
		return "a place of a node not alive"
	case !p.aliveOrRoot(m, to.ParentID):
		return "a place under a parent gone"
	case to.ParentID != nil && p.within(m, *to.ParentID, op.NodeID):
		return "a place under itself"
	}
	if h, held := p.holder(m, to.ParentID, p.key(to.Name)); held && h != op.NodeID {
		return fmt.Sprintf("a place at the name %q taken", to.Name)
	}
	if move := !domain.SameParent(n.parent, to.ParentID) || n.order != to.SortOrder; op.Move != move || op.Rename != (n.name != to.Name) {
		return fmt.Sprintf("a place's move %v and rename %v, for one from %+v to %+v", op.Move, op.Rename, n.state(), to)
	}
	n.parent, n.name, n.order = to.ParentID, to.Name, to.SortOrder
	m.nodes[op.NodeID] = n
	return ""
}

// restore does a restore on m: its trash item's, each member as its
// deletion left it, at its time, its root's parent alive or the root and
// its name free there.
func (p *propTrial) restore(m propModel, op domain.RevertOp) string {
	tr, ok := m.trash[op.NodeID]
	switch {
	case !ok:
		return "a restore of no trash item"
	case !op.DeletedAt.Equal(tr.at) || len(op.Group) != len(tr.members) || len(op.States) != len(op.Group):
		return "a restore of another group"
	case !op.To.Same(tr.members[op.NodeID]):
		return "a restore of its root elsewhere"
	}
	for i, member := range op.Group {
		at, ok := tr.members[member.Node.ID]
		if !ok || !at.Same(op.States[i]) || member.Revision != m.nodes[member.Node.ID].revision {
			return "a restore of a member elsewhere"
		}
	}
	root := tr.members[op.NodeID]
	if !p.aliveOrRoot(m, root.ParentID) {
		return "a restore under a parent gone"
	}
	if _, held := p.holder(m, root.ParentID, p.key(root.Name)); held {
		return fmt.Sprintf("a restore at the name %q taken", root.Name)
	}
	for id, at := range tr.members {
		n := m.nodes[id]
		n.parent, n.name, n.order, n.alive, n.trash = at.ParentID, at.Name, at.SortOrder, true, uuid.UUID{}
		m.nodes[id] = n
	}
	delete(m.trash, op.NodeID)
	return ""
}

// compare is "" when after is pre: the tree, and the contents of pre's
// nodes, alive or in the trash.
func (p *propTrial) compare(pre, after propModel) string {
	for _, id := range p.ids {
		b, inPre := pre.nodes[id]
		a := after.nodes[id]
		switch {
		case b.alive != a.alive:
			return fmt.Sprintf("node %s alive %v, before %v", id, a.alive, b.alive)
		case b.alive && !a.state().Same(b.state()):
			return fmt.Sprintf("node %s at %+v, before %+v", id, a.state(), b.state())
		case inPre && a.content != b.content:
			return fmt.Sprintf("node %s (alive %v) of content %q, before %q", id, a.alive, a.content, b.content)
		}
	}
	return ""
}

// describe is the plan's steps, each its kind, node and where it puts it.
func (p *propTrial) describe(m propModel, plan domain.RevertPlan) string {
	short := func(id *uuid.UUID) string {
		if id == nil {
			return "/"
		}
		return id.String()[32:]
	}
	var out []string
	for _, op := range plan.Ops {
		s := fmt.Sprintf("%s %s(%s)", op.Kind, short(&op.NodeID), m.nodes[op.NodeID].name)
		if op.Kind == domain.RevertPlace || op.Kind == domain.RevertAside {
			s += fmt.Sprintf(" to %s:%s", short(op.To.ParentID), op.To.Name)
		}
		out = append(out, s)
	}
	return "[" + strings.Join(out, ", ") + "]"
}
