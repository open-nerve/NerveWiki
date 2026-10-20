package domain

import (
	"bytes"
	"cmp"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A changeset's revert (M8 design 4.8; M8/P3 design 3.5): every node it
// changed goes back to where it was before it, compared by state, not by
// time or revision, whole or not at all.

// ConflictReason is why a node keeps a revert from its changeset.
type ConflictReason string

// The reasons, in the order the conflicts are listed (M8/P3 design 3.5): a
// node is no longer as the changeset left it; a deletion's trash item is
// past the time it can be restored in; a node would go back under a
// parent no longer alive; a node another changeset put under one the
// revert deletes would go with it; a node would go back to a name another
// holds there; under itself; or with pages deeper than MaxDepth.
const (
	ConflictChanged       ConflictReason = "changed"
	ConflictExpired       ConflictReason = "expired"
	ConflictParentGone    ConflictReason = "parent_gone"
	ConflictChildrenAdded ConflictReason = "children_added"
	ConflictTitleTaken    ConflictReason = "title_taken"
	ConflictCycle         ConflictReason = "cycle"
	ConflictTooDeep       ConflictReason = "too_deep"
)

// MaxConflicts is how many of a revert's conflicts its problem names (M8
// design 4.14): the first, with how many there are.
const MaxConflicts = 100

// MaxRevertSteps bounds a revert by its steps that are each a unit's
// operation on the tree, its places, deletions and restores (M8/P3A
// review C I-2), and its steps aside among them (M9/P3 design 3.19): each
// reads and writes its node's subtree or its trash item, so a changeset
// of many tops, a flat import's creation or the revert of its revert,
// which restores as many trash items, takes its time by their number,
// within the unit's nodes. A node takes at most three of them, a step
// aside of each kind and its place, so a batch's revert is far within.
// Measured as MaxUnitNodes is: BenchmarkRevertOfManyTops, on the same
// machine beside a load of 4 to 16, in notebooks of 0 and 100,000 more
// nodes, at the three statistics.
// The revert of 1,000, 2,000 and 3,000 pages' creations at the root took
// 1.6-1.8, 2.9-3.8 and 4.2-5.8 s; of attachments' 1.2-1.8, 2.5-3.6 and
// 3.3-5.3 s; the restore of as many trash items, the revert of that
// revert, the costliest, 1.7-2.5 s, 3.5-7.7 s (once past the request's 15
// s), 6.0-10.0 s, and of 1,500 2.6-4.8 s. The bound is the most steps
// measured within 5 seconds, rounded down.
const MaxRevertSteps = 1_500

// RevertConflict is a node that keeps a revert from its changeset: its
// name where the revert would put it (the name a title_taken contends
// for), its name now when the revert would leave it nowhere; why; and the
// node holding the name of a title_taken.
type RevertConflict struct {
	NodeID   uuid.UUID
	Name     string
	Reason   ConflictReason
	HolderID *uuid.UUID
}

// NodeNow is a node as it is now, alive or in the trash: DeletedAt is when
// it was deleted, nil while it is alive; a trash item's root holds its
// group's pages and attachments, and DeletedByRevert when its deletion,
// at that time, undid the restore of the trash item the changeset
// reverted made: it is the revert of a changeset, a trash restore or a
// revert, that restored the node after that changeset, with no deletion
// of the node between them.
type NodeNow struct {
	Node
	DeletedAt               *time.Time
	TrashRoot               bool
	TrashPages, TrashAssets int
	DeletedByRevert         bool
}

// ContentNow is a page's content now: its revision and hash.
type ContentNow struct {
	Revision int
	Hash     []byte
}

// ErrRevertUnordered is a revert whose schedule finds no step to take,
// none of its fallbacks applying, or runs past its rounds' bound, though
// the state before the changeset is valid (M9/P3 design 3.16.4, (T2) and
// (T4)): from a valid state some step or fallback always applies, and the
// rounds end within the bound, so this is a defect, no problem of the API.
var ErrRevertUnordered = errors.New("page: a revert's schedule finds no step to take")

// RevertState is what a revert plans by (M8/P3 design 3.5): the
// changeset's items and versions, by node; its nodes as they are now, one
// the purge took not among them; the contents of its pages alive; the
// hashes of the versions RevertRefs names, one the purge took not among
// them; the groups of the trash items RevertGroups names, by root, as
// they are now, one gone since not among them; the notebook's nodes
// alive, read when the changeset has items; and the time and the purge's
// retention a restore is judged at.
type RevertState struct {
	Items     []ChangesetItem
	Versions  []ChangesetVersion
	Nodes     map[uuid.UUID]NodeNow
	Contents  map[uuid.UUID]ContentNow
	Hashes    map[VersionRef][]byte
	Groups    map[uuid.UUID][]TrashMember
	Tree      []Node
	At        time.Time
	Retention time.Duration
}

// RevertPlan is a revert's plan: its conflicts, the first MaxConflicts in
// their order, and how many there are; the pages whose versions it
// compares or writes back the purge took, by id; and, with neither, its
// schedule's steps in their order.
type RevertPlan struct {
	Conflicts []RevertConflict
	Count     int
	Missing   []uuid.UUID
	Ops       []RevertOp
}

// Steps is how many of the plan's steps MaxRevertSteps bounds: its
// places, its steps aside, deletions and restores. A content's write is
// bounded as the pages a unit parses are.
func (p RevertPlan) Steps() int {
	n := 0
	for _, op := range p.Ops {
		if op.Kind != RevertContent {
			n++
		}
	}
	return n
}

// RevertOpKind is what a step of a revert does.
type RevertOpKind string

// The steps, in the order a round of the schedule tries them (M8/P3
// design 3.5; M9/P3 design 3.16), and the step aside a fallback takes.
const (
	RevertPlace   RevertOpKind = "place"
	RevertDelete  RevertOpKind = "delete"
	RevertRestore RevertOpKind = "restore"
	RevertContent RevertOpKind = "content"
	RevertAside   RevertOpKind = "aside"
)

// RevertOp is a step of a revert, Seq its place in the schedule: a place
// puts the node NodeID back at To, a move when Move (its parent or order
// differ), then a rename when Rename; a step aside puts it at To for a
// while, under the root when Move, under its parent otherwise, renamed,
// as a place does; a delete deletes the subtree of NodeID; a restore
// brings Group back, the trash item of the root NodeID deleted at
// DeletedAt, each member to its state in States, the root's To; a
// content writes the version Base of the page NodeID back on its revision
// Current, whose content hashes to Hash.
type RevertOp struct {
	Kind         RevertOpKind
	Seq          int
	NodeID       uuid.UUID
	To           TreeState
	Move, Rename bool
	Group        []TrashMember
	States       []TreeState
	DeletedAt    time.Time
	Base         int
	Current      int
	Hash         []byte
}

// RevertGroups are the trash items whose groups a revert reads, by root,
// at their time (M8/P3 design 3.6): each top of the changeset's deletions,
// a deleted node whose parent before it the changeset did not delete,
// that is still a trash item's root holding as many pages and
// attachments as the changeset deleted under it, itself included. Any
// other top is changed, its group not read: one wrapped in a larger
// deletion since, or restored, filled and deleted again, is never read
// whole only to be refused.
func RevertGroups(items []ChangesetItem, nodes map[uuid.UUID]NodeNow) map[uuid.UUID]time.Time {
	c := readReverted(items, nil)
	out := map[uuid.UUID]time.Time{}
	for _, top := range c.tops {
		if c.rootHolds(top, nodes) {
			out[top] = *nodes[top].DeletedAt
		}
	}
	return out
}

// RevertWrites are the versions whose bases a revert writes back (M9
// design 4.12): every version of the changeset on a base. A page the
// changeset created has none, its version a first one; a page it deleted
// after writing it is restored, then written back; a page it restored
// and wrote, a revert's, is written back, then deleted.
func RevertWrites(items []ChangesetItem, versions []ChangesetVersion) []ChangesetVersion {
	return readReverted(items, versions).writes
}

// RevertRefs are the versions whose hashes a revert compares (M8/P3
// design 3.5), by page and revision: a restored page's at the revision
// the changeset left it at; a deleted page's there and at its revision in
// its group now; and a page's the revert writes back, at the version's
// base. A page's content in the trash is its version at its revision:
// each content's revision has its version. An attachment has none.
func RevertRefs(items []ChangesetItem, versions []ChangesetVersion, groups map[uuid.UUID][]TrashMember) []VersionRef {
	members := membersOf(groups)
	seen := map[VersionRef]bool{}
	var out []VersionRef
	add := func(id uuid.UUID, revision int) {
		if ref := (VersionRef{NodeID: id, Revision: revision}); revision > 0 && !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	for _, it := range items {
		switch {
		case it.Restored:
			add(it.NodeID, it.Revision)
		case it.After == nil:
			add(it.NodeID, it.Revision)
			add(it.NodeID, members[it.NodeID].Revision)
		}
	}
	for _, v := range RevertWrites(items, versions) {
		add(v.NodeID, *v.Base)
	}
	slices.SortFunc(out, func(a, b VersionRef) int {
		return cmp.Or(a.NodeID.Compare(b.NodeID), cmp.Compare(a.Revision, b.Revision))
	})
	return out
}

// membersOf are the groups' members by node.
func membersOf(groups map[uuid.UUID][]TrashMember) map[uuid.UUID]TrashMember {
	out := map[uuid.UUID]TrashMember{}
	for _, g := range groups {
		for _, m := range g {
			out[m.Node.ID] = m
		}
	}
	return out
}

// reverted is a changeset as a revert reads it: its items and versions by
// node; every node it changed, by id; its deletions' tops, by id, the top
// of each node it deleted and each top's deleted nodes, by id; and the
// versions whose bases a revert writes back, by page.
type reverted struct {
	items    map[uuid.UUID]ChangesetItem
	versions map[uuid.UUID]ChangesetVersion
	ids      []uuid.UUID
	tops     []uuid.UUID
	top      map[uuid.UUID]uuid.UUID
	members  map[uuid.UUID][]uuid.UUID
	writes   []ChangesetVersion
}

func readReverted(items []ChangesetItem, versions []ChangesetVersion) reverted {
	c := reverted{items: make(map[uuid.UUID]ChangesetItem, len(items)), versions: make(map[uuid.UUID]ChangesetVersion, len(versions)),
		top: map[uuid.UUID]uuid.UUID{}, members: map[uuid.UUID][]uuid.UUID{}}
	for _, it := range items {
		c.items[it.NodeID] = it
		c.ids = append(c.ids, it.NodeID)
	}
	for _, v := range versions {
		c.versions[v.NodeID] = v
		if _, ok := c.items[v.NodeID]; !ok {
			c.ids = append(c.ids, v.NodeID)
		}
	}
	slices.SortFunc(c.ids, uuid.UUID.Compare)
	for _, id := range c.ids {
		if it, ok := c.items[id]; ok && it.After == nil {
			top := c.topOf(id)
			c.top[id], c.members[top] = top, append(c.members[top], id)
			if top == id {
				c.tops = append(c.tops, id)
			}
		}
	}
	for _, id := range c.ids {
		if v, ok := c.versions[id]; ok && v.Base != nil {
			c.writes = append(c.writes, v)
		}
	}
	return c
}

// topOf is the top of the deletion of the node id the changeset deleted:
// up its parents before the changeset while the changeset deleted them.
// A tree has no cycle; a defect's would end where it closes.
func (c reverted) topOf(id uuid.UUID) uuid.UUID {
	for range len(c.items) {
		parent := c.items[id].Before.ParentID
		if parent == nil {
			break
		}
		if p, ok := c.items[*parent]; !ok || p.After != nil || *parent == id {
			break
		}
		id = *parent
	}
	return id
}

// deleted reports whether the changeset deleted the node id; gone whether
// it created or restored it, a node the state before it has not.
func (c reverted) deleted(id uuid.UUID) bool {
	it, ok := c.items[id]
	return ok && it.After == nil
}

func (c reverted) gone(id uuid.UUID) bool {
	it, ok := c.items[id]
	return ok && it.Before == nil
}

// rootHolds reports whether the top is a trash item's root holding as
// many pages and attachments as the changeset deleted under it.
func (c reverted) rootHolds(top uuid.UUID, nodes map[uuid.UUID]NodeNow) bool {
	n, ok := nodes[top]
	if !ok || n.DeletedAt == nil || !n.TrashRoot {
		return false
	}
	pages, assets := 0, 0
	for _, id := range c.members[top] {
		m, ok := nodes[id]
		switch {
		case !ok:
			return false
		case m.Kind == KindAsset:
			assets++
		default:
			pages++
		}
	}
	return n.TrashPages == pages && n.TrashAssets == assets
}

// PlanRevert plans the revert of a changeset in state s (M8/P3 design
// 3.5): by state, not by time or revision. Each node the changeset
// changed must be as it left it, else it is changed: created or restored,
// alive where it put it, a page at the content it left; deleted, in the
// trash, each top of its deletion the root of a trash item that holds
// exactly the nodes it deleted under it, each node where and as it
// deleted it (a top whose restore was reverted excepted: below), a page
// at the content it deleted; renamed, moved or reordered,
// alive where it put it; written, alive at the content it wrote. A
// version it compares by that the purge took is missing, no conflict.
// Then the state before it, S*, is checked once: what it deleted alive
// where it was, what it created or restored gone, what it moved where it
// was, the rest as it is. The conflicts come in their reasons' order,
// then by node id, one a node, each named as S* names it, or as the node
// is now where S* has none. Without conflicts or missing versions, it
// schedules the steps from now (schedule): ErrRevertUnordered is a defect.
func PlanRevert(s RevertState) (RevertPlan, error) {
	return planRevert(s, MaxDepth)
}

// planRevert is PlanRevert with pages nesting maxDepth deep: MaxDepth, but
// in the tests that check the schedule on small trees.
func planRevert(s RevertState, maxDepth int) (RevertPlan, error) {
	plan, _, err := scheduled(s, maxDepth)
	return plan, err
}

// scheduled is planRevert's plan and the simulation its schedule ran on,
// nil when the plan stops at its conflicts or missing versions: the test
// of the schedule's bound reads the rounds and the steps pending there
// (ScheduleRounds), the plan made as PlanRevert makes it.
func scheduled(s RevertState, maxDepth int) (RevertPlan, *revertSim, error) {
	p := planner{s: s, c: readReverted(s.Items, s.Versions), reasons: map[uuid.UUID]revertReason{}, missing: map[uuid.UUID]bool{},
		members: membersOf(s.Groups), maxDepth: maxDepth}
	p.left()
	if len(s.Items) > 0 {
		p.before()
	}
	plan := p.plan()
	if plan.Count > 0 || len(plan.Missing) > 0 {
		return plan, nil, nil
	}
	sim := p.newRevertSim()
	ops, err := p.schedule(sim)
	if err != nil {
		return RevertPlan{}, sim, err
	}
	plan.Ops = ops
	return plan, sim, nil
}

// revertReason is a node's conflict: its reason and, for a title_taken,
// the node holding the title.
type revertReason struct {
	reason ConflictReason
	holder *uuid.UUID
}

// starNode is a node in S*: where it is, its name's key, its kind.
type starNode struct {
	parent *uuid.UUID
	name   string
	key    string
	order  float64
	kind   Kind
}

// planner plans a revert: its conflicts by node, the first reason each
// met, and its pages missing; S* by node, with its children by parent
// once read; and how deep pages nest, MaxDepth.
type planner struct {
	s        RevertState
	c        reverted
	members  map[uuid.UUID]TrashMember
	reasons  map[uuid.UUID]revertReason
	missing  map[uuid.UUID]bool
	star     map[uuid.UUID]starNode
	children map[uuid.UUID][]uuid.UUID
	heights  map[uuid.UUID]int
	maxDepth int
}

func (p *planner) conflict(id uuid.UUID, reason ConflictReason, holder *uuid.UUID) {
	if _, ok := p.reasons[id]; !ok {
		p.reasons[id] = revertReason{reason: reason, holder: holder}
	}
}

// left checks each node is as the changeset left it, and each top of its
// deletions can still be restored.
func (p *planner) left() {
	for _, id := range p.c.ids {
		it, item := p.c.items[id]
		v, version := p.c.versions[id]
		n, exists := p.s.Nodes[id]
		alive := exists && n.DeletedAt == nil
		switch {
		case item && it.After == nil:
			p.leftDeleted(id, it, n, exists)
			continue
		case !alive || item && !sameState(n.Node, *it.After):
			p.conflict(id, ConflictChanged, nil)
			continue
		case n.Kind == KindAsset:
		case version:
			p.sameContent(id, v.Hash)
		case it.Revision > 0:
			p.sameVersion(id, it.Revision)
		}
		if w := p.c.versions[id]; version && w.Base != nil && !p.c.gone(id) {
			if _, ok := p.s.Hashes[VersionRef{NodeID: id, Revision: *w.Base}]; !ok {
				p.missing[id] = true
			}
		}
	}
	for _, top := range p.c.tops {
		if _, ok := p.reasons[top]; !ok && !Restorable(*p.s.Nodes[top].DeletedAt, p.s.At, p.s.Retention) {
			p.conflict(top, ConflictExpired, nil)
		}
	}
}

// leftDeleted checks a node the changeset deleted is in the trash as it
// left it: each node under the parent, at the order and of the name it
// deleted it from, a newer rename or move of it, restored and deleted
// again, changed (M8/P3A review A-1). A top whose current deletion
// undid the restore of this deletion's own trash item is compared by its
// group alone (NodeNow.DeletedByRevert): the first changeset after the
// deletion that brought the item back, a trash restore or a revert, put it
// where it chose (a restore at the root, its name numbered; the revert of
// an older deletion where that one left it), and reverted newest first,
// its revert deleted it there, before this one. That pair cancels out, so
// putting the top back where the deletion left it undoes nothing newer.
// Any other deletion of the top is compared where and as the deletion
// left it: an edit's; a creation's revert; the revert of a restore of a
// later trash item of the node, which a deletion after this one made
// (M8/P3A fix checks 1 and 2).
func (p *planner) leftDeleted(id uuid.UUID, it ChangesetItem, n NodeNow, exists bool) {
	top := p.c.top[id]
	t := p.s.Nodes[top]
	switch {
	case !exists || n.DeletedAt == nil:
		p.conflict(id, ConflictChanged, nil)
		return
	case id == top:
		if !p.c.rootHolds(top, p.s.Nodes) || !p.groupIsDeleted(top) || !n.DeletedByRevert && !sameState(n.Node, *it.Before) {
			p.conflict(id, ConflictChanged, nil)
			return
		}
	case n.TrashRoot || !sameState(n.Node, *it.Before) || t.DeletedAt == nil || !n.DeletedAt.Equal(*t.DeletedAt):
		p.conflict(id, ConflictChanged, nil)
		return
	}
	if n.Kind == KindAsset {
		return
	}
	m, read := p.members[id]
	switch {
	case it.Revision == 0:
		// A deletion's item before M8/P3 holds no revision.
		p.conflict(id, ConflictChanged, nil)
	case !read:
		// Its top is changed: its group was not read.
	case m.Revision == 0:
		p.missing[id] = true
	default:
		deleted, ok := p.s.Hashes[VersionRef{NodeID: id, Revision: it.Revision}]
		trashed, held := p.s.Hashes[VersionRef{NodeID: id, Revision: m.Revision}]
		switch {
		case !ok || !held:
			p.missing[id] = true
		case !bytes.Equal(deleted, trashed):
			p.conflict(id, ConflictChanged, nil)
		}
	}
}

// groupIsDeleted reports whether the top's group is the nodes the
// changeset deleted under it.
func (p *planner) groupIsDeleted(top uuid.UUID) bool {
	g, ok := p.s.Groups[top]
	if !ok || len(g) != len(p.c.members[top]) {
		return false
	}
	for _, m := range g {
		if p.c.top[m.Node.ID] != top || !p.c.deleted(m.Node.ID) {
			return false
		}
	}
	return true
}

// sameContent compares the page id's content now with hash; sameVersion
// with its version at revision.
func (p *planner) sameContent(id uuid.UUID, hash []byte) {
	if c, ok := p.s.Contents[id]; !ok || !bytes.Equal(c.Hash, hash) {
		p.conflict(id, ConflictChanged, nil)
	}
}

func (p *planner) sameVersion(id uuid.UUID, revision int) {
	hash, ok := p.s.Hashes[VersionRef{NodeID: id, Revision: revision}]
	if !ok {
		p.missing[id] = true
		return
	}
	p.sameContent(id, hash)
}

// sameState reports whether n is where state puts it, its name the same
// string.
func sameState(n Node, state TreeState) bool {
	return n.State().Same(state)
}

// before checks S*, the notebook's nodes alive less those the changeset
// created or restored, with those it deleted, and those it moved where
// they were before it.
func (p *planner) before() {
	p.star = make(map[uuid.UUID]starNode, len(p.s.Tree)+len(p.c.items))
	for _, n := range p.s.Tree {
		p.star[n.ID] = starNode{parent: n.ParentID, name: n.Name, key: n.NameKey, order: n.SortOrder, kind: n.Kind}
	}
	for id, it := range p.c.items {
		if it.Before == nil {
			delete(p.star, id)
			continue
		}
		kind := KindPage
		if n, ok := p.s.Nodes[id]; ok {
			kind = n.Kind
		}
		p.star[id] = starNode{parent: it.Before.ParentID, name: it.Before.Name, key: shared.TitleKey(it.Before.Name),
			order: it.Before.SortOrder, kind: kind}
	}
	for _, id := range p.c.ids {
		if n, ok := p.star[id]; ok && n.parent != nil {
			if _, alive := p.star[*n.parent]; !alive {
				p.conflict(id, ConflictParentGone, nil)
			}
		}
	}
	ids := make([]uuid.UUID, 0, len(p.star))
	for id := range p.star {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	type slot struct {
		parent uuid.UUID
		key    string
	}
	holders := map[slot][]uuid.UUID{}
	for _, id := range ids {
		n := p.star[id]
		if _, touched := p.c.items[id]; !touched && n.parent != nil && p.c.gone(*n.parent) {
			p.conflict(id, ConflictChildrenAdded, nil)
		}
		at := slot{key: n.key}
		if n.parent != nil {
			at.parent = *n.parent
		}
		holders[at] = append(holders[at], id)
	}
	for _, id := range p.c.ids {
		n, ok := p.star[id]
		if !ok || !p.moves(id) {
			continue
		}
		at := slot{key: n.key}
		if n.parent != nil {
			at.parent = *n.parent
		}
		var holder *uuid.UUID
		for _, o := range holders[at] {
			_, touched := p.c.items[o]
			if o != id && (holder == nil || !touched) {
				holder = &o
				if !touched {
					break
				}
			}
		}
		if holder != nil {
			p.conflict(id, ConflictTitleTaken, holder)
		}
	}
	for _, id := range p.c.ids {
		if !p.movesBack(id) {
			continue
		}
		depth, ok := p.depth(id)
		switch {
		case !ok:
			if p.closes(id) {
				p.conflict(id, ConflictCycle, nil)
			}
		case depth+p.height(id, 0)-1 > p.maxDepth:
			p.conflict(id, ConflictTooDeep, nil)
		}
	}
}

// moves reports whether the node id is somewhere else in S* than now, or
// named otherwise: a node the changeset deleted comes back; one it moved
// goes back.
func (p *planner) moves(id uuid.UUID) bool {
	it, ok := p.c.items[id]
	if !ok || it.Before == nil {
		return false
	}
	n, exists := p.s.Nodes[id]
	return it.After == nil || !exists || n.DeletedAt != nil || !it.Before.Same(n.State())
}

// movesBack reports whether the node id goes under another parent in S*:
// a top of a deletion comes back, a node moved goes back.
func (p *planner) movesBack(id uuid.UUID) bool {
	it, ok := p.c.items[id]
	switch {
	case !ok || it.Before == nil:
		return false
	case it.After == nil:
		return p.c.top[id] == id
	}
	return !SameParent(it.Before.ParentID, it.After.ParentID)
}

// depth is the node id's depth in S*, a root's 1; not ok when a parent up
// its line is not in S*, or the line closes.
func (p *planner) depth(id uuid.UUID) (int, bool) {
	depth := 1
	for at := p.star[id]; at.parent != nil; depth++ {
		next, ok := p.star[*at.parent]
		if !ok || depth > len(p.star) {
			return 0, false
		}
		at = next
	}
	return depth, true
}

// closes reports whether the node id's line in S* comes back to it.
func (p *planner) closes(id uuid.UUID) bool {
	at := p.star[id]
	for range len(p.star) {
		if at.parent == nil {
			return false
		}
		if *at.parent == id {
			return true
		}
		next, ok := p.star[*at.parent]
		if !ok {
			return false
		}
		at = next
	}
	return false
}

// height is how many levels of pages the node id's subtree spans in S*,
// as Subtree.Height counts: a page's is one more than its children's
// most, an attachment's 0. Down cycles it stops at the bound of S*'s
// size.
func (p *planner) height(id uuid.UUID, down int) int {
	if p.children == nil {
		p.children, p.heights = map[uuid.UUID][]uuid.UUID{}, map[uuid.UUID]int{}
		for cid, n := range p.star {
			if n.parent != nil {
				p.children[*n.parent] = append(p.children[*n.parent], cid)
			}
		}
	}
	if h, ok := p.heights[id]; ok {
		return h
	}
	if p.star[id].kind == KindAsset || down > len(p.star) {
		return 0
	}
	h := 0
	for _, child := range p.children[id] {
		h = max(h, p.height(child, down+1))
	}
	p.heights[id] = h + 1
	return h + 1
}

// rank is the reason's place in the order a revert's conflicts are
// listed in.
func (r ConflictReason) rank() int {
	return slices.Index([]ConflictReason{ConflictChanged, ConflictExpired, ConflictParentGone, ConflictChildrenAdded,
		ConflictTitleTaken, ConflictCycle, ConflictTooDeep}, r)
}

// CompareConflicts orders two conflicts as a revert lists them: by their
// reasons' order, then by node id.
func CompareConflicts(a, b RevertConflict) int {
	return cmp.Or(cmp.Compare(a.Reason.rank(), b.Reason.rank()), a.NodeID.Compare(b.NodeID))
}

// plan is the conflicts in their order, at most MaxConflicts named, and
// the pages missing.
func (p *planner) plan() RevertPlan {
	ids := make([]uuid.UUID, 0, len(p.reasons))
	for id := range p.reasons {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int {
		return cmp.Or(cmp.Compare(p.reasons[a].reason.rank(), p.reasons[b].reason.rank()), a.Compare(b))
	})
	out := RevertPlan{Count: len(ids)}
	for _, id := range ids[:min(len(ids), MaxConflicts)] {
		r := p.reasons[id]
		out.Conflicts = append(out.Conflicts, RevertConflict{NodeID: id, Name: p.nameOf(id), Reason: r.reason, HolderID: r.holder})
	}
	for id := range p.missing {
		out.Missing = append(out.Missing, id)
	}
	slices.SortFunc(out.Missing, uuid.UUID.Compare)
	return out
}

// nameOf is the node id's name in S*, else now, else as the changeset
// left it or found it: a node the purge took.
func (p *planner) nameOf(id uuid.UUID) string {
	if n, ok := p.star[id]; ok {
		return n.name
	}
	if it, ok := p.c.items[id]; ok && it.Before != nil && it.After == nil {
		return it.Before.Name
	}
	if n, ok := p.s.Nodes[id]; ok {
		return n.Name
	}
	if it, ok := p.c.items[id]; ok && it.After != nil {
		return it.After.Name
	}
	return ""
}
