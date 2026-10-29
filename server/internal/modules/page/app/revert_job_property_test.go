package app_test

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A background revert's steps put the notebook back (M9/P3 design 3.26): a
// changeset of a random batch, the domain's generator of 3.16.4, or of a
// folder larger than the job's batches (a group whose members span them, a
// page of it written back, a restore cut short in a segment of steps aside
// with members after it), held by the fake store; the job's plan at its
// start (PlanRevert). Its steps joined are the segments, no more: from a
// step aside to its node's last place, a page's restore and its write back
// right after it, a write back right before the deletion that takes its
// page. Its steps, in their order, done on the test's own model, each
// checked as a store checks it (a restore a member at a time, each where
// its deletion left it, its parent back before it), end at S*: the tree,
// and the contents of S*'s pages, alive and in the trash; each group's
// every member restored once. Then the job runs on the fake store in
// batches of random sizes, cut only between segments, each read and
// checked by RevertBatch: no batch conflicts; every write the store gets
// is one a store takes (its node alive, its parent alive, its name free
// there, a write back on the revision it read, a restore of nodes in the
// trash at its time); no node at a batch's end is anywhere but where the
// job found it or where S* has it, so no name aside is left between two
// batches; and the store ends at S*. The job's changeset is then reverted
// the same way: a revert's revert, which writes pages back before the
// deletions that take them. Between two batches, review B's movegone:
// someone moves B, the top of a deletion of a later step, under A, the top
// of an earlier one, both the changeset's: the job stops at A's batch, B
// alive, no conflict named A; B alone children_added when its deletion is
// in a later batch, as the deletion of A finds it, B changed by L when the
// batch holds both. Between two batches too, the design model's injections
// (review C-I1), each on a fixture of its own: a later place's or step
// aside's node renamed, L's changed; a later place's or group's S* name
// taken, title_taken with the taker; a child put under a later deletion's
// top, children_added; a later place's S* parent deleted, parent_gone; a
// later place's S* parent moved under its node, cycle; a later place's
// node grown to MaxDepth where it is, or a later group's S* parent moved
// as deep as it goes, too_deep (review C1-M1); a later group's S* parent
// deleted, the restore's parent_gone (review C1-N1): the batch of that step
// stops, its conflict that step's alone, named as in S*, nothing written
// in it, no node aside. Not idle: each kind of segment, a restore cut
// short, a group across batches, each movegone and each injection come up
// at least as often as its floor counts it, each apart.
func TestTheJobsStepsReplayToStar(t *testing.T) {
	trials, short := 2_000, testing.Short()
	if short {
		trials = 400
	}
	var counts jobCounts
	for seed := range trials {
		if bad := newJobTrial(uint64(seed)).run(&counts); bad != "" {
			t.Fatalf("seed %d: %s", seed, bad)
		}
	}
	t.Logf("%d trials, %d jobs in %d batches: segments of steps aside %d, of a restore and its write back %d, of a write back "+
		"and its deletion %d; restores cut short %d; groups across batches %d; movegone in a later batch %d, in one %d; injected "+
		"renames %d, names taken %d, children %d, places' parents deleted %d, cycles %d, places too deep %d, restores too deep %d, "+
		"restores' parents deleted %d",
		trials, counts.jobs, counts.batches, counts.aside, counts.restoreWrite, counts.writeDelete, counts.cut, counts.across,
		counts.goneLater, counts.goneSame, counts.injected[injectRename], counts.injected[injectKey], counts.injected[injectChild],
		counts.injected[injectTrash], counts.injected[injectCycle], counts.injected[injectDeepPlace], counts.injected[injectDeepRestore],
		counts.injected[injectTrashRestore])
	// Each kind's floor, in full and under -short, is two thirds of its
	// least count over seeds other than the test's: 20 bases of 2,000
	// trials (from 0, 100,000, ..., 1,900,000) and 40 of 400 (from 0,
	// 1,000, ..., 39,000), so a change that only reshuffles the trials
	// keeps to them (review C2-N1). -short runs 400 trials, the fewest
	// that keep each of its floors at 3 or more.
	for _, c := range []struct {
		what        string
		n           int
		full, short int
	}{
		{"segments of steps aside", counts.aside, 741, 138},
		{"segments of a restore and its write back", counts.restoreWrite, 836, 157},
		{"segments of a write back and its deletion", counts.writeDelete, 512, 86},
		{"restores cut short", counts.cut, 176, 26},
		{"groups across batches", counts.across, 548, 100},
		{"movegone in a later batch", counts.goneLater, 73, 11},
		{"movegone in one", counts.goneSame, 126, 22},
		{"renames injected", counts.injected[injectRename], 427, 78},
		{"names taken injected", counts.injected[injectKey], 364, 66},
		{"children injected", counts.injected[injectChild], 235, 40},
		{"places' parents deleted injected", counts.injected[injectTrash], 58, 8},
		{"cycles injected", counts.injected[injectCycle], 28, 3},
		{"places too deep injected", counts.injected[injectDeepPlace], 78, 11},
		{"restores too deep injected", counts.injected[injectDeepRestore], 41, 5},
		{"restores' parents deleted injected", counts.injected[injectTrashRestore], 66, 9},
	} {
		floor := c.full
		if short {
			floor = c.short
		}
		if c.n < floor {
			t.Errorf("idle: %s %d, want %d", c.what, c.n, floor)
		}
	}
}

// jobCounts are what the trials did: jobs run and their batches; the
// segments of each kind; the restores cut short in a segment, members of
// their group after it; the groups whose members two batches or more
// restore; and the movegone injected, B's deletion in a later batch than
// A's or in A's.
type jobCounts struct {
	jobs, batches, aside, restoreWrite, writeDelete, cut, across, goneLater, goneSame int
	injected                                                                          [injects]int
}

// jobNode is a node of the model, a page or an attachment (asset): where
// it is, alive or a member of the trash item of the root trash, and a
// page's content at its revision.
type jobNode struct {
	parent   *uuid.UUID
	name     string
	order    float64
	asset    bool
	alive    bool
	trash    uuid.UUID
	revision int
	content  string
}

func (n jobNode) state() domain.TreeState {
	return domain.TreeState{ParentID: n.parent, Name: n.name, SortOrder: n.order}
}

// jobModel is a notebook in the model: its nodes, and its trash items by
// root, each member where its deletion left it.
type jobModel struct {
	nodes map[uuid.UUID]jobNode
	trash map[uuid.UUID]map[uuid.UUID]domain.TreeState
}

func newJobModel() jobModel {
	return jobModel{nodes: map[uuid.UUID]jobNode{}, trash: map[uuid.UUID]map[uuid.UUID]domain.TreeState{}}
}

func (m jobModel) clone() jobModel {
	out := newJobModel()
	for id, n := range m.nodes {
		out.nodes[id] = n
	}
	for root, members := range m.trash {
		out.trash[root] = make(map[uuid.UUID]domain.TreeState, len(members))
		for id, at := range members {
			out.trash[root][id] = at
		}
	}
	return out
}

// jobTrial is a trial: its seed and random source; every version a page wrote, its
// content and hash, by page and revision; the nodes it made, in id order;
// the keys of the names it met; when its changeset was made.
type jobTrial struct {
	seed     uint64
	rng      *rand.Rand
	versions map[domain.VersionRef]string
	hashes   map[domain.VersionRef][]byte
	ids      []uuid.UUID
	keys     map[string]string
	at       time.Time
}

func newJobTrial(seed uint64) *jobTrial {
	return &jobTrial{seed: seed, rng: rand.New(rand.NewPCG(seed, 0x5851f42d4c957f2d)), versions: map[domain.VersionRef]string{},
		hashes: map[domain.VersionRef][]byte{}, keys: map[string]string{}, at: now().Add(-time.Hour)}
}

func (p *jobTrial) between(lo, hi int) int { return lo + p.rng.IntN(hi-lo+1) }

func (p *jobTrial) key(name string) string {
	k, ok := p.keys[name]
	if !ok {
		k = shared.TitleKey(name)
		p.keys[name] = k
	}
	return k
}

// newID is a node's id: a UUIDv7 of one millisecond, as a batch's are, in
// the order of the nodes made, its last 62 bits at random.
func (p *jobTrial) newID() uuid.UUID {
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

// version records the page id's content at revision.
func (p *jobTrial) version(id uuid.UUID, revision int, content string) {
	ref := domain.VersionRef{NodeID: id, Revision: revision}
	p.versions[ref], p.hashes[ref] = content, hashOf(content)
}

// The model's tree, its alive nodes'.

func (p *jobTrial) aliveOrRoot(m jobModel, parent *uuid.UUID) bool {
	return parent == nil || m.nodes[*parent].alive
}

func (p *jobTrial) holder(m jobModel, parent *uuid.UUID, key string) (uuid.UUID, bool) {
	for _, id := range p.ids {
		if n := m.nodes[id]; n.alive && domain.SameParent(n.parent, parent) && p.key(n.name) == key {
			return id, true
		}
	}
	return uuid.UUID{}, false
}

func (p *jobTrial) children(m jobModel, parent uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range p.ids {
		if n := m.nodes[id]; n.alive && n.parent != nil && *n.parent == parent {
			out = append(out, id)
		}
	}
	return out
}

func (p *jobTrial) subtree(m jobModel, id uuid.UUID) []uuid.UUID {
	under := map[uuid.UUID][]uuid.UUID{}
	for _, c := range p.ids {
		if n := m.nodes[c]; n.alive && n.parent != nil {
			under[*n.parent] = append(under[*n.parent], c)
		}
	}
	out := []uuid.UUID{id}
	for at := 0; at < len(out); at++ {
		out = append(out, under[out[at]]...)
	}
	return out
}

// within reports whether at is id or under it.
func (p *jobTrial) within(m jobModel, at, id uuid.UUID) bool {
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
func (p *jobTrial) depthUnder(m jobModel, parent *uuid.UUID) int {
	d := 1
	for at := parent; at != nil && d <= len(p.ids)+1; d++ {
		at = m.nodes[*at].parent
	}
	return d
}

// height is how many levels of pages the subtree of id spans, an
// attachment's none.
func (p *jobTrial) height(m jobModel, id uuid.UUID) int {
	if m.nodes[id].asset {
		return 0
	}
	h := 0
	for _, c := range p.children(m, id) {
		h = max(h, p.height(m, c))
	}
	return h + 1
}

func (p *jobTrial) alive(m jobModel) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range p.ids {
		if m.nodes[id].alive {
			out = append(out, id)
		}
	}
	return out
}

// create makes a page under parent named name, its first content its
// version 1; or an attachment, asset, of no content.
func (p *jobTrial) create(m jobModel, parent *uuid.UUID, name string, asset bool) uuid.UUID {
	id := p.newID()
	if asset {
		m.nodes[id] = jobNode{parent: parent, name: name, order: float64(len(p.ids)), asset: true, alive: true}
		return id
	}
	content := "c" + id.String()
	m.nodes[id] = jobNode{parent: parent, name: name, order: float64(len(p.ids)), alive: true, revision: 1, content: content}
	p.version(id, 1, content)
	return id
}

// pages are the pages of ids.
func (p *jobTrial) pages(m jobModel, ids []uuid.UUID) []*uuid.UUID {
	var out []*uuid.UUID
	for _, id := range ids {
		if !m.nodes[id].asset {
			out = append(out, &id)
		}
	}
	return out
}

// levels is 1 for an attachment, a level it may be below MaxDepth; 0 for
// a page.
func levels(asset bool) int {
	if asset {
		return 1
	}
	return 0
}

// jobNameOf is name as a node, or an attachment, asset, takes it: an
// attachment's ends with its extension.
func jobNameOf(name string, asset bool) string {
	if asset && !strings.HasSuffix(name, ".png") {
		return name + ".png"
	}
	return name
}

// jobSlot is a place a node left: its parent and name.
type jobSlot struct {
	parent *uuid.UUID
	name   string
}

// jobEdit is a batch on the model m, as 4.11 lets one be (the domain's
// generator, 3.16.4): what it created, moved or renamed, and wrote; the
// trash items it made, by root; the places nodes left.
type jobEdit struct {
	p                       *jobTrial
	m                       jobModel
	created, moved, written map[uuid.UUID]bool
	groups                  map[uuid.UUID]map[uuid.UUID]domain.TreeState
	freed                   []jobSlot
	serial                  int
}

func (p *jobTrial) edit(m jobModel) *jobEdit {
	return &jobEdit{p: p, m: m, created: map[uuid.UUID]bool{}, moved: map[uuid.UUID]bool{}, written: map[uuid.UUID]bool{},
		groups: map[uuid.UUID]map[uuid.UUID]domain.TreeState{}}
}

func (e *jobEdit) okMove(x uuid.UUID, parent *uuid.UUID, name string) bool {
	m, p := e.m, e.p
	n := m.nodes[x]
	if parent != nil && (!m.nodes[*parent].alive || m.nodes[*parent].asset || p.within(m, *parent, x)) {
		return false
	}
	if h, held := p.holder(m, parent, p.key(name)); held && h != x || domain.SameParent(n.parent, parent) && n.name == name {
		return false
	}
	return p.depthUnder(m, parent)+p.height(m, x)-1 <= domain.MaxDepth
}

// move moves or renames x, as an op of the batch on the tree as it is
// then.
func (e *jobEdit) move(x uuid.UUID, parent *uuid.UUID, name string) bool {
	name = jobNameOf(name, e.m.nodes[x].asset)
	if !e.okMove(x, parent, name) {
		return false
	}
	n := e.m.nodes[x]
	e.freed = append(e.freed, jobSlot{n.parent, n.name})
	n.parent, n.name = parent, name
	e.m.nodes[x], e.moved[x] = n, true
	return true
}

func (e *jobEdit) create(parent *uuid.UUID, name string, asset bool) (uuid.UUID, bool) {
	name = jobNameOf(name, asset)
	if _, held := e.p.holder(e.m, parent, e.p.key(name)); e.p.aliveOrRoot(e.m, parent) && !held &&
		e.p.depthUnder(e.m, parent) <= domain.MaxDepth+levels(asset) {
		id := e.p.create(e.m, parent, name, asset)
		e.created[id], e.written[id] = true, !asset
		return id, true
	}
	return uuid.UUID{}, false
}

func (e *jobEdit) write(x uuid.UUID) {
	e.serial++
	n := e.m.nodes[x]
	n.revision++
	n.content = fmt.Sprintf("w%s.%d", x, e.serial)
	e.m.nodes[x], e.written[x] = n, true
	e.p.version(x, n.revision, n.content)
}

func (e *jobEdit) touched(ids []uuid.UUID) bool {
	return slices.ContainsFunc(ids, func(id uuid.UUID) bool { return e.created[id] || e.moved[id] })
}

// del deletes the subtree of x, a trash item with those under it the
// batch deleted before; none that the batch created or moved (B1, B3).
func (e *jobEdit) del(x uuid.UUID) bool {
	m, p := e.m, e.p
	sub := p.subtree(m, x)
	var absorbed []uuid.UUID
	for _, top := range p.ids {
		if g, ok := e.groups[top]; ok && g[top].ParentID != nil && slices.Contains(sub, *g[top].ParentID) {
			absorbed = append(absorbed, top)
		}
	}
	members := map[uuid.UUID]domain.TreeState{}
	all := slices.Clone(sub)
	for _, top := range absorbed {
		for id, s := range e.groups[top] {
			members[id] = s
			all = append(all, id)
		}
	}
	if e.touched(all) {
		return false
	}
	for _, top := range absorbed {
		delete(e.groups, top)
	}
	for _, id := range sub {
		n := m.nodes[id]
		members[id], n.alive = n.state(), false
		m.nodes[id] = n
	}
	e.groups[x] = members
	e.freed = append(e.freed, jobSlot{members[x].ParentID, members[x].Name})
	return true
}

// done is the notebook after the batch, its trash items made, and the
// batch's changeset as the store records it.
func (e *jobEdit) done(t0 jobModel) (jobModel, []domain.ChangesetItem, []domain.ChangesetVersion) {
	for top, g := range e.groups {
		e.m.trash[top] = g
		for id := range g {
			n := e.m.nodes[id]
			n.trash = top
			e.m.nodes[id] = n
		}
	}
	items, versions := e.p.changesetOf(t0, e.m, e.written)
	return e.m, items, versions
}

// random is a random tree, T0, and a random batch on it, the domain's
// generator: short, or long.
func (p *jobTrial) random(long bool) (t0, post jobModel, items []domain.ChangesetItem, versions []domain.ChangesetVersion) {
	m := newJobModel()
	size := p.between(3, 6)
	if long {
		size = p.between(5, 12)
	}
	for range size {
		parents := append([]*uuid.UUID{nil}, p.pages(m, p.ids)...)
		parent, asset := parents[p.rng.IntN(len(parents))], p.rng.IntN(4) == 0
		if p.depthUnder(m, parent) > domain.MaxDepth+levels(asset) {
			continue
		}
		name := jobNameOf(string("abc"[p.rng.IntN(3)]), asset)
		if _, held := p.holder(m, parent, p.key(name)); held {
			continue
		}
		p.create(m, parent, name, asset)
	}
	t0 = m.clone()
	e := p.edit(m)
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
	ops := p.between(3, 12)
	if long {
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
			if len(e.freed) > 0 {
				s := e.freed[p.rng.IntN(len(e.freed))]
				e.create(s.parent, s.name, strings.HasSuffix(s.name, ".png"))
			}
		case 1: // into a page the batch made
			var made []uuid.UUID
			for _, id := range alive {
				if e.created[id] && !m.nodes[id].asset {
					made = append(made, id)
				}
			}
			if len(made) > 0 {
				x := pick()
				e.move(x, &made[p.rng.IntN(len(made))], m.nodes[x].name)
			}
		case 2: // out, to the root or a level up
			x := pick()
			if parent := m.nodes[x].parent; parent != nil {
				to := []*uuid.UUID{nil, m.nodes[*parent].parent}[p.rng.IntN(2)]
				e.move(x, to, m.nodes[x].name)
			}
		case 3: // two siblings' names swapped through "t"
			siblings := map[uuid.UUID][]uuid.UUID{}
			var parents []*uuid.UUID
			for _, id := range alive {
				at := m.nodes[id].parent
				k := parentOf(at)
				if len(siblings[k]) == 0 {
					parents = append(parents, at)
				}
				siblings[k] = append(siblings[k], id)
			}
			parents = slices.DeleteFunc(parents, func(at *uuid.UUID) bool { return len(siblings[parentOf(at)]) < 2 })
			if len(parents) > 0 {
				at := parents[p.rng.IntN(len(parents))]
				ids := siblings[parentOf(at)]
				i := p.rng.IntN(len(ids))
				j := (i + 1 + p.rng.IntN(len(ids)-1)) % len(ids)
				a, b := ids[i], ids[j]
				ka, kb := m.nodes[a].name, m.nodes[b].name
				if m.nodes[a].asset == m.nodes[b].asset && e.move(a, at, "t") && e.move(b, at, ka) {
					e.move(a, at, kb)
				}
			}
		case 4: // to a place a node left
			if len(e.freed) > 0 {
				s := e.freed[p.rng.IntN(len(e.freed))]
				e.move(pick(), s.parent, s.name)
			}
		case 5:
			e.del(pick())
		case 6:
			parents := append([]*uuid.UUID{nil}, p.pages(m, alive)...)
			e.move(pick(), parents[p.rng.IntN(len(parents))], namePick())
		case 7:
			parents := append([]*uuid.UUID{nil}, p.pages(m, alive)...)
			e.create(parents[p.rng.IntN(len(parents))], namePick(), p.rng.IntN(4) == 0)
		case 8:
			if x := pick(); !m.nodes[x].asset {
				e.write(x)
			}
		case 9: // written, then deleted
			x := pick()
			if !m.nodes[x].asset && !e.touched(p.subtree(m, x)) {
				e.write(x)
				e.del(x)
			}
		}
	}
	post, items, versions = e.done(t0)
	return t0, post, items, versions
}

// folder is a folder G larger than the job's batches, and a batch that
// deletes it: G's pages, some with a page under them, an attachment
// among them, and K, one of them, with x and y under it; the batch
// renames another page O; moves x, y or both out of K, to the root;
// writes some of G's pages, or G itself; deletes G; and, when it moved y,
// makes a new G at the root and moves y into it, so that the revert steps
// y aside, deletes the new G, restores G, as far as K in its pre-order,
// and puts x and y back, the rest of G after them (M9/P3 design 3.22.2,
// review B-I2). Its trash item is a group across the job's batches.
func (p *jobTrial) folder() (t0, post jobModel, items []domain.ChangesetItem, versions []domain.ChangesetVersion) {
	m := newJobModel()
	o := p.create(m, nil, "O", false)
	g := p.create(m, nil, "G", false)
	var kids []uuid.UUID
	for i := range p.between(4, 14) {
		asset := i > 0 && p.rng.IntN(6) == 0
		kid := p.create(m, &g, jobNameOf(fmt.Sprintf("p%d", i), asset), asset)
		kids = append(kids, kid)
		if !asset && p.rng.IntN(3) == 0 {
			p.create(m, &kid, "q", false)
		}
	}
	pages := slices.DeleteFunc(slices.Clone(kids), func(id uuid.UUID) bool { return m.nodes[id].asset })
	k := pages[len(pages)-1]
	if p.rng.IntN(3) == 0 {
		k = pages[p.rng.IntN(len(pages))]
	}
	x := p.create(m, &k, "x", false)
	y := p.create(m, &k, "y", false)
	t0 = m.clone()
	e := p.edit(m)
	if p.rng.IntN(2) == 0 {
		e.move(o, nil, "O2")
	}
	outX, outY := p.rng.IntN(3) > 0, p.rng.IntN(4) > 0
	if outX {
		e.move(x, nil, "x")
	}
	if outY {
		e.move(y, nil, "y")
	}
	for _, id := range pages {
		if p.rng.IntN(4) == 0 {
			e.write(id)
		}
	}
	if p.rng.IntN(3) == 0 {
		e.write(g)
	}
	e.del(g)
	if outY {
		if n, ok := e.create(nil, "G", false); ok && p.rng.IntN(4) > 0 {
			e.move(y, &n, "y")
		}
	}
	post, items, versions = e.done(t0)
	return t0, post, items, versions
}

// changesetOf is the changeset that took the notebook from pre to post, as
// the store records it: an item for each node alive in one and not the
// other, or elsewhere, a page's the revision of its content when the
// batch made it, its first, or deleted it (an item of a move holds none,
// and a write records no item, unit.recordItems); and a version for each
// page written, on its revision in pre, none for one it made.
func (p *jobTrial) changesetOf(pre, post jobModel, written map[uuid.UUID]bool) ([]domain.ChangesetItem, []domain.ChangesetVersion) {
	var items []domain.ChangesetItem
	var versions []domain.ChangesetVersion
	stateIn := func(m jobModel, id uuid.UUID) *domain.TreeState {
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
		it := domain.ChangesetItem{NodeID: id, Before: b, After: a}
		switch {
		case post.nodes[id].asset:
		case a == nil:
			it.Revision = post.nodes[id].revision
		case b == nil:
			it.Revision = 1
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

// load is a fixture whose store holds the notebook m, every version the
// trial wrote in its history, and the changeset of items and versions,
// made at the trial's time; the trash items' counts as a deletion
// records them.
func (p *jobTrial) load(m jobModel, items []domain.ChangesetItem, versions []domain.ChangesetVersion) (*fixture, *fakeChangeset) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	for ref, content := range p.versions {
		f.log.versions[ref] = content
	}
	for _, id := range p.ids {
		n, ok := m.nodes[id]
		if !ok {
			continue
		}
		kind := domain.KindPage
		if n.asset {
			kind = domain.KindAsset
		}
		node := domain.Node{ID: id, NotebookID: f.eng, ParentID: n.parent, Kind: kind, Name: n.name, NameKey: p.key(n.name),
			SortOrder: n.order, CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: p.at, UpdatedAt: p.at}
		sum := sha256.Sum256([]byte(n.content))
		content := app.Content{NodeID: id, Content: n.content, Revision: n.revision, Hash: sum[:], ByteSize: len(n.content), By: f.alice,
			At: p.at}
		if n.alive {
			f.store.nodes[id] = node
			if !n.asset {
				f.store.contents[id] = content
			}
			continue
		}
		f.store.trash[id] = app.AnyNode{Node: node, DeletedAt: new(p.at), TrashRoot: n.trash == id}
		if !n.asset {
			f.store.trashed[id] = content
		}
	}
	for root := range m.trash {
		group, err := f.store.group(f.eng, root, p.at)
		if err != nil {
			panic(err)
		}
		nodes := groupOf(group)
		f.store.counts[root] = [3]int{int(nodes[0].Pages), int(nodes[0].Assets), int(nodes[0].Height)}
	}
	return f, f.log.changeset(domain.ChangesetEdit, f.alice, p.at, items, versions...)
}

// groupOf is a trash item's group, as TrashGroup reads it, in its
// pre-order with its counts.
func groupOf(group []domain.TrashMember) []domain.GroupNode {
	var below []domain.ShapeNode
	for _, m := range group[1:] {
		below = append(below, domain.ShapeNode{ID: m.Node.ID, ParentID: *m.Node.ParentID, Asset: m.Node.Kind == domain.KindAsset})
	}
	nodes := domain.GroupPreOrder(domain.ShapeNode{ID: group[0].Node.ID, Asset: group[0].Node.Kind == domain.KindAsset}, below)
	domain.GroupCounts(nodes)
	return nodes
}

// modelOf is the fixture's store as the model has it: its nodes alive and
// in the trash, each trash item its members where their deletion left
// them.
func (p *jobTrial) modelOf(f *fixture) jobModel {
	m := newJobModel()
	for _, id := range p.ids {
		if n, ok := f.store.nodes[id]; ok {
			c := f.store.contents[id]
			m.nodes[id] = jobNode{parent: n.ParentID, name: n.Name, order: n.SortOrder, asset: n.Kind == domain.KindAsset, alive: true,
				revision: c.Revision, content: c.Content}
		}
	}
	for _, id := range p.ids {
		t, ok := f.store.trash[id]
		if !ok || !t.TrashRoot {
			continue
		}
		group, err := f.store.group(f.eng, id, *t.DeletedAt)
		if err != nil {
			panic(err)
		}
		members := map[uuid.UUID]domain.TreeState{}
		for _, g := range group {
			n := g.Node
			c := f.store.trashed[n.ID]
			members[n.ID] = n.State()
			m.nodes[n.ID] = jobNode{parent: n.ParentID, name: n.Name, order: n.SortOrder, asset: n.Kind == domain.KindAsset, trash: id,
				revision: c.Revision, content: c.Content}
		}
		m.trash[id] = members
	}
	return m
}

// checkedStore is the fake store checking each write a job's batch makes
// as a store does: a place's node alive, a write back's on the revision
// the page is at, a deletion's and a restore's nodes alive and in the
// trash at its time; after each, every node alive under a parent alive,
// no two alike named under one parent, no cycle.
type checkedStore struct {
	*fakeStore
	bad []string
}

func (c *checkedStore) fail(format string, args ...any) {
	c.bad = append(c.bad, fmt.Sprintf(format, args...))
}

// check checks the tree after a write, what.
func (c *checkedStore) check(what string) {
	keys := map[[2]string]uuid.UUID{}
	for id, n := range c.nodes {
		if n.ParentID != nil {
			if _, ok := c.nodes[*n.ParentID]; !ok {
				c.fail("after %s, %s (%s) under %s, not alive", what, n.Name, id, *n.ParentID)
			}
		}
		k := [2]string{parentOf(n.ParentID).String(), n.NameKey}
		if other, held := keys[k]; held {
			c.fail("after %s, %s and %s both named %q under %s", what, id, other, n.Name, k[0])
		}
		keys[k] = id
		at := n.ParentID
		for range len(c.nodes) + 1 {
			if at == nil {
				break
			}
			if *at == id {
				c.fail("after %s, %s under itself", what, id)
				break
			}
			at = c.nodes[*at].ParentID
		}
	}
}

func (c *checkedStore) placed(n domain.Node, write func() error) error {
	if _, ok := c.nodes[n.ID]; !ok {
		c.fail("a place of %s, not alive", n.ID)
	}
	err := write()
	c.check("a place of " + n.ID.String())
	return err
}

func (c *checkedStore) RenameNode(ctx context.Context, n domain.Node) error {
	return c.placed(n, func() error { return c.fakeStore.RenameNode(ctx, n) })
}

func (c *checkedStore) MoveNode(ctx context.Context, n domain.Node) error {
	return c.placed(n, func() error { return c.fakeStore.MoveNode(ctx, n) })
}

func (c *checkedStore) RelocateNode(ctx context.Context, n domain.Node) error {
	return c.placed(n, func() error { return c.fakeStore.RelocateNode(ctx, n) })
}

func (c *checkedStore) WriteContent(ctx context.Context, content app.Content) error {
	if old, ok := c.contents[content.NodeID]; !ok || content.Revision != old.Revision+1 {
		c.fail("a write of %s at revision %d, the page at %d (alive %v)", content.NodeID, content.Revision, old.Revision, ok)
	}
	return c.fakeStore.WriteContent(ctx, content)
}

func (c *checkedStore) DeleteNodes(ctx context.Context, d app.Deletion) error {
	for _, id := range d.IDs {
		if _, ok := c.nodes[id]; !ok {
			c.fail("a deletion of %s, not alive", id)
		}
	}
	err := c.fakeStore.DeleteNodes(ctx, d)
	c.check("a deletion of " + d.Root.String())
	return err
}

func (c *checkedStore) RestoreNodes(ctx context.Context, r app.Restoration) (app.RestoredRows, error) {
	for _, id := range r.IDs {
		if n, ok := c.trash[id]; !ok || !n.DeletedAt.Equal(r.DeletedAt) {
			c.fail("a restore of %s, not in the trash at %v", id, r.DeletedAt)
		}
	}
	rows, err := c.fakeStore.RestoreNodes(ctx, r)
	c.check("a restore of " + r.Root.String())
	return rows, err
}

// checkedJobs are the background jobs' writes over the fixture, its
// writes through c.
func (f *fixture) checkedJobs(c *checkedStore) *app.JobWrites {
	w := app.NewWriter(app.WriterDeps{
		Tx: f.tx, Clock: f.clock, Auth: f.auth, Workspaces: f.workspaces, Notebooks: f.notebooks, Nodes: f.store,
		Sources: f.store, Attachments: fakeAttachments{recorder: f.rec, infos: f.infos},
		NodeWriter: c, Trash: f.store, History: f.log, Changesets: f.store, SessionWriter: f.store, Search: f.store, Names: f.names,
	})
	return app.NewJobWrites(f.trashReads(now()), f.log, w, app.NewContentParser(w, f.md, f.budget, f.wait), f.store, 0, 0)
}

// recorded is the job's changeset id as the history holds it, a revert of
// reverted: the items and versions its batches recorded, their contents
// in the history; a deletion's item with its page's revision in the trash,
// as the store records it (M8/P3 design 3.4).
func (f *fixture) recorded(id, reverted uuid.UUID) *fakeChangeset {
	var items []domain.ChangesetItem
	for _, it := range f.store.recorded {
		if it.ChangesetID == id {
			c := it.Change
			if c.After == nil {
				c.Revision = f.store.trashed[c.NodeID].Revision
			}
			items = append(items, domain.ChangesetItem{NodeID: c.NodeID, Before: c.Before, After: c.After, Restored: c.Restored != nil,
				Revision: c.Revision})
		}
	}
	var versions []domain.ChangesetVersion
	for _, r := range f.store.revisions {
		if r.ChangesetID == id {
			f.log.versions[domain.VersionRef{NodeID: r.NodeID, Revision: r.Revision}] = r.Content
			versions = append(versions, domain.ChangesetVersion{NodeID: r.NodeID, Base: r.Base, Revision: r.Revision, Hash: r.Hash,
				Rewrite: r.Rewrite})
		}
	}
	c := f.log.changeset(domain.ChangesetRevert, f.alice, now(), items, versions...)
	c.ID, c.RevertsID = id, reverted
	return c
}

// run is the trial: a batch's changeset, its job's revert, then the
// revert of that, each judged on the model and on the store; and the
// movegone on the first. "" when all is well, else what went wrong.
func (p *jobTrial) run(counts *jobCounts) string {
	var star, post jobModel
	var items []domain.ChangesetItem
	var versions []domain.ChangesetVersion
	switch p.rng.IntN(3) {
	case 0:
		star, post, items, versions = p.folder()
	default:
		star, post, items, versions = p.random(p.rng.IntN(2) == 0)
	}
	if len(items) == 0 && len(versions) == 0 {
		return ""
	}
	if bad := p.movegone(post, items, versions, counts); bad != "" {
		return "movegone: " + bad
	}
	for kind := range injects {
		if bad := p.inject(kind, post, items, versions, counts); bad != "" {
			return "inject " + bad
		}
	}
	f, c := p.load(post, items, versions)
	for level := 1; level <= 2; level++ {
		start := p.modelOf(f)
		job, bad := p.job(f, c.ID, start, star, counts)
		if bad != "" {
			return fmt.Sprintf("R%d: %s", level, bad)
		}
		c, star = f.recorded(job, c.ID), start
		if len(c.items) == 0 && len(c.versions) == 0 {
			return ""
		}
	}
	return ""
}

// job plans the revert of the changeset id in f, from start, replays its
// steps on the model to star, then runs it in batches cut at random, and
// answers its changeset's id.
func (p *jobTrial) job(f *fixture, id uuid.UUID, start, star jobModel, counts *jobCounts) (uuid.UUID, string) {
	store := &checkedStore{fakeStore: f.store}
	w, spec := f.checkedJobs(store), f.jobSpec()
	plan, err := w.PlanRevert(f.asJob(), spec, id, allJobBatches)
	if err != nil {
		var e *shared.Error
		if errors.As(err, &e) {
			dbg := ""
			for _, cf := range e.Conflicts {
				for _, it := range f.log.find(id).items {
					if it.NodeID == cf.NodeID {
						dbg += fmt.Sprintf(" item %+v before %+v after %+v;", it, it.Before, it.After)
					}
				}
				dbg += fmt.Sprintf(" store %+v trash %+v", f.store.nodes[cf.NodeID], f.store.trash[cf.NodeID])
			}
			return uuid.UUID{}, fmt.Sprintf("PlanRevert = %v %+v%s", err, e.Conflicts, dbg)
		}
		return uuid.UUID{}, fmt.Sprintf("PlanRevert = %v", err)
	}
	steps, groups := plan.RevertSteps()
	if bad := joints(plan.Steps(), steps); bad != "" {
		return uuid.UUID{}, fmt.Sprintf("the segments: %s; steps %s", bad, describeSteps(steps))
	}
	if bad := p.replay(start.clone(), star, steps, groups); bad != "" {
		return uuid.UUID{}, fmt.Sprintf("the steps on the model: %s; steps %s", bad, describeSteps(steps))
	}
	p.segments(plan.Steps(), steps, counts)
	cuts := p.cuts(plan.Steps())
	counts.jobs, counts.batches = counts.jobs+1, counts.batches+len(cuts)
	batchOf := map[int]map[int]bool{}
	for b, cut := range cuts {
		for k := cut[0]; k < cut[1]; k++ {
			if s := steps[k]; s.Kind == domain.RevertRestore {
				if batchOf[s.Group] == nil {
					batchOf[s.Group] = map[int]bool{}
				}
				batchOf[s.Group][b] = true
			}
		}
		if _, err := w.RunBatch(f.asJob(), spec, plan, cut[0], cut[1]); err != nil {
			return uuid.UUID{}, fmt.Sprintf("batch %v of %v: %v; steps %s", cut, cuts, err, describeSteps(steps))
		}
		if len(store.bad) > 0 {
			return uuid.UUID{}, fmt.Sprintf("batch %v of %v wrote %s; steps %s", cut, cuts, strings.Join(store.bad, "; "), describeSteps(steps))
		}
		if bad := p.inPlace(start, star, f); bad != "" {
			return uuid.UUID{}, fmt.Sprintf("after batch %v of %v: %s; steps %s", cut, cuts, bad, describeSteps(steps))
		}
	}
	for _, in := range batchOf {
		if len(in) > 1 {
			counts.across++
		}
	}
	if bad := p.compare(star, p.modelOf(f)); bad != "" {
		return uuid.UUID{}, fmt.Sprintf("the store after the job: %s; steps %s", bad, describeSteps(steps))
	}
	return spec.Changeset, ""
}

// cuts are a job's batches, from step 0, each of a random size, grown to
// its segment's end as the job's worker grows it; one batch now and then.
func (p *jobTrial) cuts(steps []domain.JobStep) [][2]int {
	return cutsBy(p.rng, steps)
}

// cutsBy are cuts of rng.
func cutsBy(rng *rand.Rand, steps []domain.JobStep) [][2]int {
	var out [][2]int
	whole := rng.IntN(8) == 0
	for from := 0; from < len(steps); {
		to := len(steps)
		if !whole {
			to = min(from+1+rng.IntN(4), len(steps))
		}
		for to < len(steps) && steps[to-1].Joined {
			to++
		}
		out, from = append(out, [2]int{from, to}), to
	}
	return out
}

// segments counts the plan's segments by kind, and its restores cut short:
// a group whose member steps a step other than a write back separates.
func (p *jobTrial) segments(jobSteps []domain.JobStep, steps []app.RevertStep, counts *jobCounts) {
	for from := 0; from < len(steps); {
		to := from + 1
		for to < len(steps) && jobSteps[to-1].Joined {
			to++
		}
		seg := steps[from:to]
		has := func(kind domain.RevertOpKind) bool {
			return slices.ContainsFunc(seg, func(s app.RevertStep) bool { return s.Kind == kind })
		}
		switch {
		case len(seg) == 1:
		case has(domain.RevertAside):
			counts.aside++
		case len(seg) == 2 && seg[0].Kind == domain.RevertRestore && seg[1].Kind == domain.RevertContent && seg[0].Node == seg[1].Node:
			counts.restoreWrite++
		case has(domain.RevertContent) && has(domain.RevertDelete):
			counts.writeDelete++
		}
		from = to
	}
	last := map[int]int{}
	cut := map[int]bool{}
	for k, s := range steps {
		if s.Kind != domain.RevertRestore {
			continue
		}
		if j, ok := last[s.Group]; ok {
			for _, between := range steps[j+1 : k] {
				if between.Kind != domain.RevertContent && (between.Kind != domain.RevertRestore || between.Group != s.Group) {
					cut[s.Group] = true
				}
			}
		}
		last[s.Group] = k
	}
	counts.cut += len(cut)
}

// joints is "" when the steps joined are the segments (M9/P3 design
// 3.22.2), no more: from each step aside to its node's last place; a
// page's restore and its write back, right after it; a write back and the
// deletion that takes its page, right before it, with the other write
// backs it takes between them (review A-M1).
func joints(jobSteps []domain.JobStep, steps []app.RevertStep) string {
	want := make([]bool, len(steps))
	join := func(from, to int) {
		for k := from; k < to; k++ {
			want[k] = true
		}
	}
	last := map[uuid.UUID]int{}
	restored := map[uuid.UUID]int{}
	for k, s := range steps {
		switch s.Kind {
		case domain.RevertPlace:
			last[s.Node] = k
		case domain.RevertRestore:
			restored[s.Node] = k
		}
	}
	for k, s := range steps {
		switch s.Kind {
		case domain.RevertAside:
			join(k, last[s.Node])
		case domain.RevertContent:
			if r, ok := restored[s.Node]; ok {
				if r != k-1 {
					return fmt.Sprintf("the write back of %s, step %d, not right after its restore, step %d", s.Node, k, r)
				}
				join(r, k)
			}
			for j, d := range steps[k+1:] {
				if d.Kind != domain.RevertDelete || !slices.Contains(d.Nodes, s.Node) {
					continue
				}
				for _, between := range steps[k+1 : k+1+j] {
					if between.Kind != domain.RevertContent || !slices.Contains(d.Nodes, between.Node) {
						return fmt.Sprintf("the write back of %s, step %d, not right before the deletion that takes it, step %d", s.Node, k,
							k+1+j)
					}
				}
				join(k, k+1+j)
			}
		}
	}
	for k, js := range jobSteps {
		if js.Joined != want[k] {
			return fmt.Sprintf("step %d joined %v, want %v", k, js.Joined, want[k])
		}
	}
	return ""
}

// replay does the steps on m in turn, each checked as a store checks it:
// a place's node alive, its parent alive or the root, not under itself,
// its name free there; a deletion's top alive, its subtree the one the
// step keeps; a restore's member in its trash item, under its group's
// parent, which is alive, or the root, and its name free there; a write
// back's page alive, of the version's hash, written once. Then each
// group's every member restored, and m is star.
func (p *jobTrial) replay(m, star jobModel, steps []app.RevertStep, groups [][]domain.GroupNode) string {
	restored := make([]int, len(groups))
	written := map[uuid.UUID]bool{}
	for i, s := range steps {
		bad := ""
		switch s.Kind {
		case domain.RevertPlace, domain.RevertAside:
			var parent *uuid.UUID
			if s.Parent != (uuid.UUID{}) {
				parent = &s.Parent
			}
			bad = p.place(m, s.Node, domain.TreeState{ParentID: parent, Name: s.Name, SortOrder: s.Order})
		case domain.RevertDelete:
			bad = p.delete(m, s.Node, s.Nodes)
		case domain.RevertRestore:
			bad = p.restore(m, groups[s.Group], s.Member)
			restored[s.Group]++
		case domain.RevertContent:
			ref := domain.VersionRef{NodeID: s.Node, Revision: s.Base}
			n := m.nodes[s.Node]
			content, ok := p.versions[ref]
			switch {
			case !n.alive:
				bad = "a write back of a page not alive"
			case !ok || !bytes.Equal(s.Hash, p.hashes[ref]):
				bad = fmt.Sprintf("a write back of version %d, not the hash it has", s.Base)
			case written[s.Node]:
				bad = "a page written back twice"
			}
			n.revision++
			n.content = content
			m.nodes[s.Node], written[s.Node] = n, true
		default:
			bad = fmt.Sprintf("a step of kind %q", s.Kind)
		}
		if bad != "" {
			return fmt.Sprintf("step %d (%s %s): %s", i, s.Kind, s.Node, bad)
		}
	}
	for g, n := range restored {
		if n != len(groups[g]) {
			return fmt.Sprintf("%d of the %d members of the group of %s restored", n, len(groups[g]), groups[g][0].ID)
		}
	}
	return p.compare(star, m)
}

// place puts the node id at to on m: alive, its parent alive or the
// root, not under itself, its name free there.
func (p *jobTrial) place(m jobModel, id uuid.UUID, to domain.TreeState) string {
	n := m.nodes[id]
	switch {
	case !n.alive:
		return "a place of a node not alive"
	case !p.aliveOrRoot(m, to.ParentID):
		return "a place under a parent gone"
	case to.ParentID != nil && p.within(m, *to.ParentID, id):
		return "a place under itself"
	}
	if h, held := p.holder(m, to.ParentID, p.key(to.Name)); held && h != id {
		return fmt.Sprintf("a place at the name %q taken", to.Name)
	}
	n.parent, n.name, n.order = to.ParentID, to.Name, to.SortOrder
	m.nodes[id] = n
	return ""
}

// delete deletes the subtree of top on m, a trash item: top alive, the
// subtree the step keeps, in the order a unit reads it, a level at a time,
// each level by order and id (review C-M1).
func (p *jobTrial) delete(m jobModel, top uuid.UUID, recorded []uuid.UUID) string {
	if !m.nodes[top].alive {
		return "a deletion of a node not alive"
	}
	sub := p.subtree(m, top)
	if want := p.levelOrder(m, sub); !slices.Equal(recorded, want) {
		return fmt.Sprintf("a deletion keeping the subtree %v, its moment's %v", recorded, want)
	}
	members := map[uuid.UUID]domain.TreeState{}
	for _, id := range sub {
		n := m.nodes[id]
		members[id], n.alive, n.trash = n.state(), false, top
		m.nodes[id] = n
	}
	m.trash[top] = members
	return ""
}

// levelOrder is sub, a subtree of m root first, as a unit reads it: the
// root, then a level at a time, each level by order and id.
func (p *jobTrial) levelOrder(m jobModel, sub []uuid.UUID) []uuid.UUID {
	depth := map[uuid.UUID]int{sub[0]: 0}
	for _, id := range sub[1:] {
		depth[id] = depth[*m.nodes[id].parent] + 1
	}
	out := slices.Clone(sub)
	slices.SortStableFunc(out, func(a, b uuid.UUID) int {
		return cmp.Or(cmp.Compare(depth[a], depth[b]), cmp.Compare(m.nodes[a].order, m.nodes[b].order), a.Compare(b))
	})
	return out
}

// restore brings back the member of the group on m: in its trash item,
// its parent the group's member's, alive, or the root's place for the
// group's first; its name free there.
func (p *jobTrial) restore(m jobModel, group []domain.GroupNode, member int) string {
	id := group[member].ID
	n := m.nodes[id]
	at, ok := m.trash[n.trash][id]
	switch {
	case n.alive || !ok:
		return "a restore of a node in no trash item"
	case member == 0 && n.trash != id:
		return "a restore of a group whose first is no trash item's root"
	case member > 0 && (at.ParentID == nil || *at.ParentID != group[group[member].Parent].ID):
		return "a restore of a member elsewhere than under its group's parent"
	case !p.aliveOrRoot(m, at.ParentID):
		return "a restore under a parent gone"
	}
	if _, held := p.holder(m, at.ParentID, p.key(at.Name)); held {
		return fmt.Sprintf("a restore at the name %q taken", at.Name)
	}
	delete(m.trash[n.trash], id)
	if len(m.trash[n.trash]) == 0 {
		delete(m.trash, n.trash)
	}
	n.parent, n.name, n.order, n.alive, n.trash = at.ParentID, at.Name, at.SortOrder, true, uuid.UUID{}
	m.nodes[id] = n
	return ""
}

// compare is "" when m is star: the tree, and the contents of star's
// nodes, alive or in the trash.
func (p *jobTrial) compare(star, m jobModel) string {
	for _, id := range p.ids {
		b, inStar := star.nodes[id]
		a := m.nodes[id]
		switch {
		case b.alive != a.alive:
			return fmt.Sprintf("node %s alive %v, in S* %v", id, a.alive, b.alive)
		case b.alive && !a.state().Same(b.state()):
			return fmt.Sprintf("node %s at %+v, in S* %+v", id, a.state(), b.state())
		case inStar && a.content != b.content:
			return fmt.Sprintf("node %s (alive %v) of content %q, in S* %q", id, a.alive, a.content, b.content)
		}
	}
	return ""
}

// inPlace is "" when every node alive in the store is where the job found
// it or where S* has it: no node left aside.
func (p *jobTrial) inPlace(start, star jobModel, f *fixture) string {
	for id, n := range f.store.nodes {
		at := jobSlot{n.ParentID, n.Name}
		same := func(m jobModel) bool {
			o := m.nodes[id]
			return o.alive && domain.SameParent(o.parent, at.parent) && o.name == at.name
		}
		if !same(start) && !same(star) {
			return fmt.Sprintf("%s alive at %s:%q, where neither the job found it nor S* has it", id, parentOf(n.ParentID), n.Name)
		}
	}
	return ""
}

// movegone is review B's movegone on the job of the changeset of items and
// versions on start: of two deletions of the changeset's own tops, A's
// before B's, A a page, B not under A nor A under B, someone moves B under
// A between the batches, before A's: the job stops at A's batch, B alone
// its conflict, children_added when a later batch holds B's deletion,
// changed in L when A's holds it too; B alive, nothing named A. None when
// no two deletions are so, or when the move is not one a write between
// batches makes, or would meet an earlier step of A's batch first.
func (p *jobTrial) movegone(start jobModel, items []domain.ChangesetItem, versions []domain.ChangesetVersion, counts *jobCounts) string {
	f, c := p.load(start, items, versions)
	w, spec := f.jobs(now(), 0, 0), f.jobSpec()
	plan, err := w.PlanRevert(f.asJob(), spec, c.ID, allJobBatches)
	if err != nil {
		return fmt.Sprintf("PlanRevert = %v", err)
	}
	steps, _ := plan.RevertSteps()
	var tops []int
	for k, s := range steps {
		if s.Kind == domain.RevertDelete {
			tops = append(tops, k)
		}
	}
	m := p.modelOf(f)
	var pairs [][2]int
	for i, ia := range tops {
		for _, ib := range tops[i+1:] {
			a, b := steps[ia].Node, steps[ib].Node
			if _, held := p.holder(m, &a, p.key(m.nodes[b].name)); !held && !m.nodes[a].asset && !p.within(m, a, b) &&
				!p.within(m, b, a) {
				pairs = append(pairs, [2]int{ia, ib})
			}
		}
	}
	if len(pairs) == 0 {
		return ""
	}
	pair := pairs[p.rng.IntN(len(pairs))]
	a, b := steps[pair[0]].Node, steps[pair[1]].Node
	cuts := p.cuts(plan.Steps())
	in := func(k int) int {
		return slices.IndexFunc(cuts, func(c [2]int) bool { return c[0] <= k && k < c[1] })
	}
	at, later := in(pair[0]), in(pair[1]) > in(pair[0])
	for _, cut := range cuts[:at] {
		if _, err := w.RunBatch(f.asJob(), spec, plan, cut[0], cut[1]); err != nil {
			return fmt.Sprintf("batch %v before the move: %v", cut, err)
		}
	}
	// B's move is one a write between batches makes, in the store now:
	// A a page not under B, as earlier batches may have put it, B's name
	// free under A, its pages within MaxDepth there. And no step of A's
	// batch before A's deletion goes under B's subtree or moves a node of
	// A's line, whose own conflict the move would make first: a cycle, say
	// (review C2-M1).
	before := map[uuid.UUID]*domain.TreeState{}
	for _, it := range items {
		before[it.NodeID] = it.Before
	}
	if sub := storeSubtree(f, b); slices.Contains(sub, a) || !storeRoom(f, &a, storeHeight(f, b)) ||
		storeHolds(f, &a, f.store.nodes[b].Name) ||
		!unmet(steps[cuts[at][0]:pair[0]], stepTarget(steps, before), cuts[at][0], sub, storeLine(f, a)) {
		return ""
	}
	moved := f.store.nodes[b]
	moved.ParentID = &a
	f.store.nodes[b] = moved
	want := domain.ConflictChanged
	if later {
		want = domain.ConflictChildrenAdded
	}
	var e *shared.Error
	_, err = w.RunBatch(f.asJob(), spec, plan, cuts[at][0], cuts[at][1])
	switch {
	case !errors.As(err, &e) || len(e.Conflicts) != 1 || e.Conflicts[0].NodeID != b || e.Conflicts[0].Reason != string(want):
		return fmt.Sprintf("B's deletion in a later batch %v: A's batch %v = %v; want B (%s) alone %s; steps %s", later, cuts[at], err, b,
			want, describeSteps(steps))
	case f.store.nodes[b].ParentID == nil || *f.store.nodes[b].ParentID != a:
		return "B not alive under A after A's batch"
	}
	if later {
		counts.goneLater++
	} else {
		counts.goneSame++
	}
	return ""
}

// parentOf is the id of parent, the zero id for the root.
func parentOf(parent *uuid.UUID) uuid.UUID {
	if parent == nil {
		return uuid.UUID{}
	}
	return *parent
}

// describeSteps are the steps, each its kind, node and where it puts it,
// its group and member, or the subtree it deletes.
func describeSteps(steps []app.RevertStep) string {
	short := func(id uuid.UUID) string {
		if id == (uuid.UUID{}) {
			return "/"
		}
		return id.String()[32:]
	}
	var out []string
	for _, s := range steps {
		d := fmt.Sprintf("%s %s", s.Kind, short(s.Node))
		switch s.Kind {
		case domain.RevertPlace, domain.RevertAside:
			d += fmt.Sprintf(" to %s:%s", short(s.Parent), s.Name)
		case domain.RevertRestore:
			d += fmt.Sprintf(" g%d.%d", s.Group, s.Member)
		case domain.RevertDelete:
			d += fmt.Sprintf(" (%d)", len(s.Nodes))
		}
		out = append(out, d)
	}
	return "[" + strings.Join(out, ", ") + "]"
}

// The injections of the design's model (sim2/batches_rev.py), each
// someone's write between two batches that a later batch's premise alone
// sees (M9/P3 S3 review C-I1): a later place's or step aside's node
// renamed, L's changed; a later place's or group's S* name taken, its
// premise's title_taken with the taker; a child put under a later
// deletion's top, children_added; a later place's S* parent deleted, its
// premise's parent_gone. And the premises' other reasons (review C1-M1):
// a later place's S* parent moved under the place's node, cycle; a later
// place's node grown by a chain of pages to MaxDepth where it is, its S*
// parent no shallower, too_deep; a later group's S* parent moved to the
// end of a chain of pages as deep as a move takes it, too_deep; a later
// group's S* parent deleted, the restore's parent_gone (review C1-N1),
// apart from a place's.
const (
	injectRename = iota
	injectKey
	injectChild
	injectTrash
	injectCycle
	injectDeepPlace
	injectDeepRestore
	injectTrashRestore
	injects
)

// injectName is the injection kind's name.
func injectName(kind int) string {
	return [injects]string{"rename", "key", "child", "trash", "cycle", "deep place", "deep restore", "trash restore"}[kind]
}

// injected is a later step's conflict an injection makes: the step's
// index, its node, reason, name and holder.
type injected struct {
	step   int
	node   uuid.UUID
	reason domain.ConflictReason
	name   string
	holder *uuid.UUID
}

func (i injected) String() string {
	holder := "none"
	if i.holder != nil {
		holder = i.holder.String()
	}
	return fmt.Sprintf("step %d: %s %s named %q, holder %s", i.step, i.node, i.reason, i.name, holder)
}

// inject is the injection kind on the job of the changeset of items and
// versions on start, on a fixture of its own and a random source of its
// own: before a batch other than the first, at random, or before the
// second when there is no target then, a write on a target a later step
// meets, chosen at random. The batches before the one of that step
// write, which stops, its conflict that step's alone; it writes nothing,
// and no node is left aside. "" when all is well or there is no target.
func (p *jobTrial) inject(kind int, start jobModel, items []domain.ChangesetItem, versions []domain.ChangesetVersion,
	counts *jobCounts,
) string {
	rng := rand.New(rand.NewPCG(p.seed, uint64(kind)+1))
	f, c := p.load(start, items, versions)
	w, spec := f.jobs(now(), 0, 0), f.jobSpec()
	plan, err := w.PlanRevert(f.asJob(), spec, c.ID, allJobBatches)
	if err != nil {
		return fmt.Sprintf("PlanRevert = %v", err)
	}
	steps, groups := plan.RevertSteps()
	cuts := cutsBy(rng, plan.Steps())
	if len(cuts) < 2 {
		return ""
	}
	// Before a batch at random, else before the second, on a fixture
	// loaded again.
	ats := []int{1 + rng.IntN(len(cuts)-1)}
	if ats[0] != 1 {
		ats = append(ats, 1)
	}
	var want injected
	at := 0
	for i, try := range ats {
		if i > 0 {
			f, c = p.load(start, items, versions)
			w, spec = f.jobs(now(), 0, 0), f.jobSpec()
			if plan, err = w.PlanRevert(f.asJob(), spec, c.ID, allJobBatches); err != nil {
				return fmt.Sprintf("PlanRevert again = %v", err)
			}
		}
		for _, cut := range cuts[:try] {
			if _, err := w.RunBatch(f.asJob(), spec, plan, cut[0], cut[1]); err != nil {
				return fmt.Sprintf("batch %v before the injection: %v", cut, err)
			}
		}
		var ok bool
		if want, ok = p.injection(kind, rng, f, items, steps, groups, cuts[try][0]); ok {
			at = try
			break
		}
	}
	if at == 0 {
		return ""
	}
	stop := slices.IndexFunc(cuts, func(c [2]int) bool { return c[0] <= want.step && want.step < c[1] })
	for b := at; b < len(cuts); b++ {
		was := snapshotOf(f.store)
		_, err := w.RunBatch(f.asJob(), spec, plan, cuts[b][0], cuts[b][1])
		if err == nil && b < stop {
			continue
		}
		var e *shared.Error
		got := conflictsOf(err)
		switch {
		case b != stop || !errors.As(err, &e):
			return fmt.Sprintf("%s before batch %v: batch %v = %v; want batch %v stopped, %s; steps %s", injectName(kind), cuts[at],
				cuts[b], err, cuts[stop], want, describeSteps(steps))
		case len(got) != 1 || got[0].NodeID != want.node || got[0].Reason != string(want.reason) || got[0].Name != want.name ||
			!sameHolder(got[0].HolderID, want.holder):
			return fmt.Sprintf("%s: batch %v's conflicts %+v; want %s; steps %s", injectName(kind), cuts[b], got, want, describeSteps(steps))
		case !was.same(snapshotOf(f.store)):
			return fmt.Sprintf("%s: batch %v stopped, but wrote", injectName(kind), cuts[b])
		}
		// No step aside's node left at its aside name: each one's own,
		// as a page the changeset left may be named like one (review
		// C2-M1).
		for _, s := range steps {
			if n, alive := f.store.nodes[s.Node]; s.Kind == domain.RevertAside && alive && n.Name == s.Name {
				return fmt.Sprintf("%s: %s named aside, %q, after the stop", injectName(kind), s.Node, s.Name)
			}
		}
		counts.injected[kind]++
		return ""
	}
	return fmt.Sprintf("%s before batch %v: the job wrote every batch; want batch %v stopped", injectName(kind), cuts[at], cuts[stop])
}

// injection makes the write of kind on f, the batches from step from on
// to come, and answers the conflict it makes; none without a target.
func (p *jobTrial) injection(kind int, rng *rand.Rand, f *fixture, items []domain.ChangesetItem, steps []app.RevertStep,
	groups [][]domain.GroupNode, from int,
) (injected, bool) {
	before := map[uuid.UUID]*domain.TreeState{}
	for _, it := range items {
		before[it.NodeID] = it.Before
	}
	touched := map[uuid.UUID]bool{}
	written := map[uuid.UUID]bool{}
	first := map[uuid.UUID]int{}
	for k, s := range steps {
		if k < from {
			continue
		}
		touched[s.Node] = true
		for _, id := range s.Nodes {
			touched[id] = true
		}
		if s.Kind == domain.RevertRestore {
			for _, g := range groups[s.Group] {
				touched[g.ID] = true
			}
		}
		if _, seen := first[s.Node]; !seen {
			first[s.Node] = k
		}
		if s.Kind == domain.RevertContent {
			written[s.Node] = true
		}
	}
	target := stepTarget(steps, before)
	var cands []injected
	switch kind {
	case injectRename:
		for k := from; k < len(steps); k++ {
			s := steps[k]
			if _, alive := f.store.nodes[s.Node]; alive && (s.Kind == domain.RevertPlace || s.Kind == domain.RevertAside) &&
				first[s.Node] == k && !written[s.Node] && before[s.Node] != nil {
				cands = append(cands, injected{step: k, node: s.Node, reason: domain.ConflictChanged, name: before[s.Node].Name})
			}
		}
	case injectKey:
		for k := from; k < len(steps); k++ {
			parent, name, ok := target(k)
			if !ok || !storePage(f, steps[k].Node, groups, steps[k]) || !storeRoom(f, parent, 1) || storeHolds(f, parent, name) {
				continue
			}
			cands = append(cands, injected{step: k, node: steps[k].Node, reason: domain.ConflictTitleTaken, name: name})
		}
	case injectChild:
		for k := from; k < len(steps); k++ {
			s := steps[k]
			if n, alive := f.store.nodes[s.Node]; alive && s.Kind == domain.RevertDelete && n.Kind == domain.KindPage &&
				storeRoom(f, &s.Node, 1) {
				cands = append(cands, injected{step: k, node: s.Node, reason: domain.ConflictChildrenAdded})
			}
		}
	case injectTrash:
		for k := from; k < len(steps); k++ {
			s := steps[k]
			if s.Kind != domain.RevertPlace || s.Parent == (uuid.UUID{}) {
				continue
			}
			if _, alive := f.store.nodes[s.Parent]; !alive || slices.ContainsFunc(storeSubtree(f, s.Parent), func(id uuid.UUID) bool {
				return touched[id]
			}) {
				continue
			}
			cands = append(cands, injected{step: k, node: s.Parent})
		}
	case injectCycle:
		for k := from; k < len(steps); k++ {
			s := steps[k]
			if s.Kind != domain.RevertPlace || s.Parent == (uuid.UUID{}) || first[s.Node] != k {
				continue
			}
			x, parent := s.Node, s.Parent
			sub, ok := movable(f, parent, touched)
			// The parent can go under x, and no step before this one goes
			// under it or under x, or moves x or a node above it.
			if n, alive := f.store.nodes[x]; !ok || !alive || n.Kind != domain.KindPage || slices.Contains(sub, x) ||
				!storeRoom(f, &x, storeHeight(f, parent)) || storeHolds(f, &x, f.store.nodes[parent].Name) ||
				!unmet(steps[from:k], target, from, append(sub, x), storeLine(f, x)) {
				continue
			}
			cands = append(cands, injected{step: k, node: x, reason: domain.ConflictCycle, name: s.Name})
		}
	case injectDeepPlace:
		for k := from; k < len(steps); k++ {
			s := steps[k]
			if s.Kind != domain.RevertPlace || s.Parent == (uuid.UUID{}) || first[s.Node] != k {
				continue
			}
			x, parent := s.Node, s.Parent
			n, alive := f.store.nodes[x]
			p, there := f.store.nodes[parent]
			// x grown to MaxDepth where it is goes a level past it under a
			// parent no shallower than x is, not under x; no step before
			// this one moves x, the parent or a node above either, so the
			// depths hold at the place (review C2-M1).
			if !alive || n.Kind != domain.KindPage || !there || p.Kind != domain.KindPage || storeDepth(f, x) >= domain.MaxDepth ||
				storeDepth(f, parent) < storeDepth(f, x) || slices.Contains(storeLine(f, parent), x) ||
				!unmet(steps[from:k], target, from, nil, append(storeLine(f, x), storeLine(f, parent)...)) {
				continue
			}
			cands = append(cands, injected{step: k, node: x, reason: domain.ConflictTooDeep, name: s.Name})
		}
	case injectTrashRestore:
		for k := from; k < len(steps); k++ {
			s := steps[k]
			parent, _, ok := target(k)
			if !ok || s.Kind != domain.RevertRestore || parent == nil {
				continue
			}
			if sub, ok := movable(f, *parent, touched); ok && unmet(steps[from:k], target, from, sub, nil) {
				cands = append(cands, injected{step: k, node: *parent})
			}
		}
	case injectDeepRestore:
		for k := from; k < len(steps); k++ {
			s := steps[k]
			parent, name, ok := target(k)
			if !ok || s.Kind != domain.RevertRestore || parent == nil {
				continue
			}
			sub, ok := movable(f, *parent, touched)
			if h := storeHeight(f, *parent); !ok || h >= domain.MaxDepth || int(groups[s.Group][0].Height) < h ||
				!unmet(steps[from:k], target, from, sub, nil) {
				continue
			}
			cands = append(cands, injected{step: k, node: s.Node, reason: domain.ConflictTooDeep, name: name})
		}
	}
	if len(cands) == 0 {
		return injected{}, false
	}
	want := cands[rng.IntN(len(cands))]
	switch kind {
	case injectRename:
		n := f.store.nodes[want.node]
		n.Name = fmt.Sprintf("zz %06d", rng.IntN(1_000_000))
		n.NameKey = shared.TitleKey(n.Name)
		f.store.nodes[want.node] = n
	case injectKey:
		parent, name, _ := target(want.step)
		taker := f.page(name, parent, 1e6)
		want.holder = &taker.ID
	case injectChild:
		top := want.node
		child := f.page(fmt.Sprintf("zz %06d", rng.IntN(1_000_000)), &top, 1e6)
		want.node, want.name = child.ID, child.Name
	case injectTrash, injectTrashRestore:
		gone := storeSubtree(f, want.node)
		f.trashed(want.node, now().Add(-30*time.Minute), f.alice)
		// The first later step that meets it: a place under it, or a
		// group's restore under it in S*.
		for k := from; k < len(steps); k++ {
			if parent, name, ok := target(k); ok && parent != nil && slices.Contains(gone, *parent) {
				return injected{step: k, node: steps[k].Node, reason: domain.ConflictParentGone, name: name}, true
			}
		}
		panic("a trash injection's parent no later step meets")
	case injectCycle:
		x, parent := want.node, steps[want.step].Parent
		n := f.store.nodes[parent]
		n.ParentID = &x
		f.store.nodes[parent] = n
	case injectDeepPlace:
		x := want.node
		deepChain(f, rng, &x, domain.MaxDepth-storeDepth(f, x))
	case injectDeepRestore:
		parent := *before[want.node].ParentID
		n := f.store.nodes[parent]
		n.ParentID = deepChain(f, rng, nil, domain.MaxDepth-storeHeight(f, parent))
		f.store.nodes[parent] = n
	}
	return want, true
}

// stepTarget is a step's S* place, of steps and the changeset's items'
// Before by node: a place's parent and name, or a group's first's.
func stepTarget(steps []app.RevertStep, before map[uuid.UUID]*domain.TreeState) func(int) (*uuid.UUID, string, bool) {
	return func(k int) (*uuid.UUID, string, bool) {
		s := steps[k]
		switch {
		case s.Kind == domain.RevertPlace:
			if s.Parent == (uuid.UUID{}) {
				return nil, s.Name, true
			}
			return &s.Parent, s.Name, true
		case s.Kind == domain.RevertRestore && s.Member == 0 && before[s.Node] != nil:
			return before[s.Node].ParentID, before[s.Node].Name, true
		}
		return nil, "", false
	}
}

// deepChain makes n pages in f's store, each under the one before, the first
// under parent, and answers the last's id.
func deepChain(f *fixture, rng *rand.Rand, parent *uuid.UUID, n int) *uuid.UUID {
	for i := range n {
		d := f.page(fmt.Sprintf("zz deep %d %06d", i, rng.IntN(1_000_000)), parent, 1e6)
		parent = &d.ID
	}
	return parent
}

// movable is the subtree of parent, alive in f's store, a page, as a
// write between batches moves it: none of it a node a later step meets.
func movable(f *fixture, parent uuid.UUID, touched map[uuid.UUID]bool) ([]uuid.UUID, bool) {
	if n, alive := f.store.nodes[parent]; !alive || n.Kind != domain.KindPage {
		return nil, false
	}
	sub := storeSubtree(f, parent)
	return sub, !slices.ContainsFunc(sub, func(id uuid.UUID) bool { return touched[id] })
}

// unmet reports whether no step of before, the later steps before the
// injection's, from on, goes under a node of sub, the subtree the write
// moves, nor moves, deletes or restores a node of kept, whose shape the
// injection's step reads: each of them holds as it did before the write.
func unmet(before []app.RevertStep, target func(int) (*uuid.UUID, string, bool), from int, sub, kept []uuid.UUID) bool {
	for j, s := range before {
		parent, _, ok := target(from + j)
		if s.Kind == domain.RevertAside && s.Parent != (uuid.UUID{}) {
			parent, ok = &s.Parent, true
		}
		if ok && parent != nil && slices.Contains(sub, *parent) || slices.Contains(kept, s.Node) ||
			slices.ContainsFunc(s.Nodes, func(id uuid.UUID) bool { return slices.Contains(kept, id) }) {
			return false
		}
	}
	return true
}

// storeHeight is how many levels of pages the subtree of id, alive in f's
// store, spans: none for an attachment.
func storeHeight(f *fixture, id uuid.UUID) int {
	kids := map[uuid.UUID][]uuid.UUID{}
	for _, n := range f.store.nodes {
		if n.ParentID != nil {
			kids[*n.ParentID] = append(kids[*n.ParentID], n.ID)
		}
	}
	var height func(id uuid.UUID) int
	height = func(id uuid.UUID) int {
		if f.store.nodes[id].Kind != domain.KindPage {
			return 0
		}
		h := 0
		for _, k := range kids[id] {
			h = max(h, height(k))
		}
		return h + 1
	}
	return height(id)
}

// storeDepth is how many levels of pages id, alive in f's store, is
// below the root, its own counted.
func storeDepth(f *fixture, id uuid.UUID) int {
	return len(storeLine(f, id))
}

// storeLine is id and its ancestors in f's store.
func storeLine(f *fixture, id uuid.UUID) []uuid.UUID {
	line := []uuid.UUID{id}
	for n := f.store.nodes[id]; n.ParentID != nil; n = f.store.nodes[*n.ParentID] {
		line = append(line, *n.ParentID)
	}
	return line
}

// storePage reports whether the node of step s is a page: in the store,
// or a group's member the step restores.
func storePage(f *fixture, id uuid.UUID, groups [][]domain.GroupNode, s app.RevertStep) bool {
	if n, ok := f.store.nodes[id]; ok {
		return n.Kind == domain.KindPage
	}
	if s.Kind == domain.RevertRestore {
		return !groups[s.Group][s.Member].Asset
	}
	return false
}

// storeRoom reports whether parent, alive in f's store or the root, holds
// levels more pages below it within MaxDepth.
func storeRoom(f *fixture, parent *uuid.UUID, levels int) bool {
	depth := 0
	for at := parent; at != nil; depth++ {
		n, ok := f.store.nodes[*at]
		if !ok || n.Kind != domain.KindPage {
			return false
		}
		at = n.ParentID
	}
	return depth+levels <= domain.MaxDepth
}

// storeHolds reports whether a node alive in f's store under parent holds
// name's key.
func storeHolds(f *fixture, parent *uuid.UUID, name string) bool {
	key := shared.TitleKey(name)
	for _, n := range f.store.nodes {
		if domain.SameParent(n.ParentID, parent) && n.NameKey == key {
			return true
		}
	}
	return false
}

// storeSubtree is id and the nodes alive under it in f's store.
func storeSubtree(f *fixture, id uuid.UUID) []uuid.UUID {
	sub := []uuid.UUID{id}
	for at := 0; at < len(sub); at++ {
		for _, n := range f.store.nodes {
			if n.ParentID != nil && *n.ParentID == sub[at] {
				sub = append(sub, n.ID)
			}
		}
	}
	return sub
}

func sameHolder(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// storeSnapshot is what a fake store holds: its nodes alive and in the
// trash, their contents, the items and versions it recorded.
type storeSnapshot struct {
	nodes              map[uuid.UUID]domain.Node
	contents, trashed  map[uuid.UUID]app.Content
	trash              map[uuid.UUID]app.AnyNode
	recorded, versions int
}

func snapshotOf(s *fakeStore) storeSnapshot {
	return storeSnapshot{nodes: maps.Clone(s.nodes), contents: maps.Clone(s.contents), trashed: maps.Clone(s.trashed),
		trash: maps.Clone(s.trash), recorded: len(s.recorded), versions: len(s.revisions)}
}

func (a storeSnapshot) same(b storeSnapshot) bool {
	return reflect.DeepEqual(a, b)
}
