package domain

import (
	"bytes"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A revert's schedule (M9 design 4.12; M9/P3 design 3.16): its steps by
// what each waits for, in rounds, from the notebook as it is now to S*,
// the state before the changeset, which PlanRevert checked is valid.

// revertSim is the notebook's tree as a revert's schedule leaves it so far
// (M9/P3 design 3.16.1): each node, alive or in the trash, and the keys
// alive under each parent, the root's the zero id; S*'s keys under each;
// and the steps pending: the nodes to place, the tops to restore and the
// pages to write back, and the nodes each fallback moved aside.
type revertSim struct {
	nodes    map[uuid.UUID]simNode
	under    map[uuid.UUID]map[string]uuid.UUID
	star     map[uuid.UUID]map[string]bool
	places   map[uuid.UUID]bool
	restores map[uuid.UUID]bool
	writes   map[uuid.UUID]RevertOp
	// renamed and movedAside are the nodes the first fallback renamed
	// aside, and the second or third moved aside to the root, renamed too:
	// each at most once by each (T1).
	renamed, movedAside map[uuid.UUID]bool
}

// simNode is a node in a revert's simulation: where it is, its name and
// its name's key, its kind, whether it is alive, and a page's content's
// revision.
type simNode struct {
	parent   *uuid.UUID
	name     string
	key      string
	order    float64
	kind     Kind
	alive    bool
	revision int
}

// state is where the node is.
func (n simNode) state() TreeState {
	return TreeState{ParentID: n.parent, Name: n.name, SortOrder: n.order}
}

func parentOf(parent *uuid.UUID) uuid.UUID {
	if parent == nil {
		return uuid.UUID{}
	}
	return *parent
}

// newRevertSim is the notebook now, as the plan read it, with the steps its
// revert has pending: each node the changeset moved, renamed or reordered,
// alive now and in S*, to place where S* differs from now; each top of its
// deletions to restore; each page of RevertWrites to write back, but one
// alive at its base's content already. A changeset of no items has no
// tree read: its pages are alive where they are.
func (p *planner) newRevertSim() *revertSim {
	s := &revertSim{nodes: make(map[uuid.UUID]simNode, len(p.s.Tree)), under: map[uuid.UUID]map[string]uuid.UUID{},
		star: map[uuid.UUID]map[string]bool{}, places: map[uuid.UUID]bool{}, restores: map[uuid.UUID]bool{},
		writes: map[uuid.UUID]RevertOp{}, renamed: map[uuid.UUID]bool{}, movedAside: map[uuid.UUID]bool{}}
	alive := func(n Node) {
		s.set(n.ID, simNode{parent: n.ParentID, name: n.Name, key: n.NameKey, order: n.SortOrder, kind: n.Kind, alive: true,
			revision: p.s.Contents[n.ID].Revision})
	}
	for _, n := range p.s.Tree {
		alive(n)
	}
	if len(p.s.Items) == 0 {
		for _, id := range p.c.ids {
			if n := p.s.Nodes[id]; n.DeletedAt == nil {
				alive(n.Node)
			}
		}
	}
	for _, n := range p.star {
		at := parentOf(n.parent)
		if s.star[at] == nil {
			s.star[at] = map[string]bool{}
		}
		s.star[at][n.key] = true
	}
	for _, id := range p.c.ids {
		if it, ok := p.c.items[id]; ok && it.Before != nil && it.After != nil && s.alive(id) && !it.Before.Same(s.nodes[id].state()) {
			s.places[id] = true
		}
	}
	for _, top := range p.c.tops {
		s.restores[top] = true
	}
	for _, v := range p.c.writes {
		hash := p.s.Hashes[VersionRef{NodeID: v.NodeID, Revision: *v.Base}]
		if c, ok := p.s.Contents[v.NodeID]; ok && s.nodes[v.NodeID].alive && bytes.Equal(c.Hash, hash) {
			continue
		}
		s.writes[v.NodeID] = RevertOp{Kind: RevertContent, NodeID: v.NodeID, Base: *v.Base, Hash: hash}
	}
	return s
}

// holder is the node alive under parent whose name's key is key.
func (s *revertSim) holder(parent *uuid.UUID, key string) (uuid.UUID, bool) {
	id, ok := s.under[parentOf(parent)][key]
	return id, ok
}

func (s *revertSim) set(id uuid.UUID, n simNode) {
	if old, ok := s.nodes[id]; ok && old.alive {
		delete(s.under[parentOf(old.parent)], old.key)
	}
	s.nodes[id] = n
	if n.alive {
		at := parentOf(n.parent)
		if s.under[at] == nil {
			s.under[at] = map[string]uuid.UUID{}
		}
		s.under[at][n.key] = id
	}
}

func (s *revertSim) alive(id uuid.UUID) bool {
	return s.nodes[id].alive
}

// aliveOrRoot reports whether parent is alive, or the root.
func (s *revertSim) aliveOrRoot(parent *uuid.UUID) bool {
	return parent == nil || s.alive(*parent)
}

// subtree is the node id and the nodes alive under it, a level at a time.
func (s *revertSim) subtree(id uuid.UUID) []uuid.UUID {
	sub := []uuid.UUID{id}
	for at := 0; at < len(sub) && at <= len(s.nodes); at++ {
		for _, child := range s.under[sub[at]] {
			sub = append(sub, child)
		}
	}
	return sub
}

// within reports whether the node at is the node id or under it, now.
func (s *revertSim) within(at, id uuid.UUID) bool {
	for range len(s.nodes) + 1 {
		if at == id {
			return true
		}
		n, ok := s.nodes[at]
		if !ok || n.parent == nil {
			return false
		}
		at = *n.parent
	}
	return false
}

// depthUnder is the depth of a node under parent, now: a root's is 1.
func (s *revertSim) depthUnder(parent *uuid.UUID) int {
	depth := 1
	for at := parent; at != nil && depth <= len(s.nodes); depth++ {
		at = s.nodes[*at].parent
	}
	return depth
}

// keyAt is the key of the node id's name at to: the one it has when its
// name stays, as a move keeps it; else to's name's, by today's rules.
func (s *revertSim) keyAt(id uuid.UUID, to TreeState) string {
	if now := s.nodes[id]; now.alive && now.name == to.Name {
		return now.key
	}
	return shared.TitleKey(to.Name)
}

// height is how many levels of pages the node id's subtree spans now, as
// Subtree.Height counts: an attachment's is 0.
func (s *revertSim) height(id uuid.UUID) int {
	h := 0
	level := []uuid.UUID{id}
	for depth := 1; len(level) > 0 && depth <= len(s.nodes); depth++ {
		var next []uuid.UUID
		for _, n := range level {
			if s.nodes[n].kind == KindAsset {
				continue
			}
			h = depth
			for _, child := range s.under[n] {
				next = append(next, child)
			}
		}
		level = next
	}
	return h
}

// The premises of a revert's steps (M9/P3 design 3.16.2), each holding,
// "", or why not: the schedule takes a step when its premise holds at its
// moment.

// canPlace is the premise of a place of the node id, or of its step
// aside, at to: its parent alive or the root, else parent_gone; not in
// the node's own subtree, else cycle; its name's key free there (the
// node's own holding none), else title_taken with the node holding it;
// the subtree's deepest page within maxDepth there, as Unit.Relocate
// checks a move, else too_deep.
func (p *planner) canPlace(s *revertSim, id uuid.UUID, to TreeState) (ConflictReason, *uuid.UUID) {
	switch {
	case !s.aliveOrRoot(to.ParentID):
		return ConflictParentGone, nil
	case to.ParentID != nil && s.within(*to.ParentID, id):
		return ConflictCycle, nil
	}
	if holder, held := s.holder(to.ParentID, s.keyAt(id, to)); held && holder != id {
		return ConflictTitleTaken, &holder
	}
	if s.depthUnder(to.ParentID)+s.height(id)-1 > p.maxDepth {
		return ConflictTooDeep, nil
	}
	return "", nil
}

// canDelete is the premise of the deletion of the subtree of top: no node
// of S* alive in it, and no page in it to write back first (a revert's
// revert writes back what it then deletes), else children_added.
func (p *planner) canDelete(s *revertSim, top uuid.UUID) ConflictReason {
	for _, id := range s.subtree(top) {
		if _, kept := p.star[id]; kept {
			return ConflictChildrenAdded
		}
		if _, pending := s.writes[id]; pending {
			return ConflictChildrenAdded
		}
	}
	return ""
}

// canRestore is the premise of the restore of the trash item of top: its
// parent in S* alive or the root, else parent_gone; its name's key free
// there, else title_taken with the node holding it; its group's deepest
// page within maxDepth there, else too_deep. Its other members go under
// its own.
func (p *planner) canRestore(s *revertSim, top uuid.UUID) (ConflictReason, *uuid.UUID) {
	to := *p.c.items[top].Before
	if !s.aliveOrRoot(to.ParentID) {
		return ConflictParentGone, nil
	}
	if holder, held := s.holder(to.ParentID, shared.TitleKey(to.Name)); held {
		return ConflictTitleTaken, &holder
	}
	levels := 0
	for _, m := range p.s.Groups[top] {
		if m.Node.Kind == KindPage {
			levels = max(levels, m.Level)
		}
	}
	if s.depthUnder(to.ParentID)+levels-1 > p.maxDepth {
		return ConflictTooDeep, nil
	}
	return "", nil
}

// canWrite is the premise of a page's write back: it is alive, else
// changed.
func (p *planner) canWrite(s *revertSim, page uuid.UUID) ConflictReason {
	if !s.alive(page) {
		return ConflictChanged
	}
	return ""
}

// schedule is the revert's steps from now (M9 design 4.12; M9/P3 design
// 3.16), on s: rounds, each trying every step pending in a fixed order
// (places, deletions, restores, writes back, each by node id) and doing
// those whose premise holds at its moment, until none is pending. A round
// that does none takes the first fallback that applies (fallback): a
// holder of a key another step waits for renamed aside; else a node S*
// keeps, under one the revert deletes, moved aside to the root; else, a
// defence no reachable state has needed, a node to place, not at the root
// and not moved aside, moved aside to the root; each node at most once by
// each. None applying is ErrRevertUnordered, a defect; so is a round past
// |P|+|D|+|R|+|W|+2N+1 (T2's bound, N the nodes the changeset touched),
// which no revert reaches and a defect would otherwise spin past under
// the notebook's lock. The steps are numbered in their order (Seq).
func (p *planner) schedule(s *revertSim) ([]RevertOp, error) {
	bound := len(s.places) + len(p.tops(s)) + len(s.restores) + len(s.writes) + 2*len(p.c.ids) + 1
	var ops []RevertOp
	for round := 1; ; round++ {
		if round > bound {
			return nil, ErrRevertUnordered
		}
		n := len(ops)
		for _, id := range p.c.ids {
			if !s.places[id] {
				continue
			}
			if to := *p.c.items[id].Before; p.canPlaceOK(s, id, to) {
				ops = append(ops, s.place(RevertPlace, id, to))
				delete(s.places, id)
			}
		}
		for _, top := range p.tops(s) {
			if p.canDelete(s, top) == "" {
				ops = append(ops, s.delete(top))
			}
		}
		for _, top := range p.c.tops {
			if !s.restores[top] {
				continue
			}
			if reason, _ := p.canRestore(s, top); reason == "" {
				ops = append(ops, p.restore(s, top))
				delete(s.restores, top)
			}
		}
		for _, id := range p.c.ids {
			if op, ok := s.writes[id]; ok && p.canWrite(s, id) == "" {
				op.Current = s.nodes[id].revision
				ops = append(ops, op)
				delete(s.writes, id)
			}
		}
		if len(s.places) == 0 && len(s.restores) == 0 && len(s.writes) == 0 && len(p.tops(s)) == 0 {
			break
		}
		if len(ops) > n {
			continue
		}
		op, ok := p.fallback(s)
		if !ok {
			return nil, ErrRevertUnordered
		}
		ops = append(ops, op)
	}
	for i := range ops {
		ops[i].Seq = i
	}
	return ops, nil
}

// canPlaceOK reports whether a place of the node id at to holds.
func (p *planner) canPlaceOK(s *revertSim, id uuid.UUID, to TreeState) bool {
	reason, _ := p.canPlace(s, id, to)
	return reason == ""
}

// tops are the deletions pending, in id order: the nodes the changeset
// created or restored, alive, under no other of them alive.
func (p *planner) tops(s *revertSim) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range p.c.ids {
		if !p.c.gone(id) || !s.alive(id) {
			continue
		}
		if parent := s.nodes[id].parent; parent != nil && p.c.gone(*parent) && s.alive(*parent) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// place is the step of kind, a place or a step aside, of the node id to
// to, done on s: a move when its parent or order differ, a rename when its
// name does.
func (s *revertSim) place(kind RevertOpKind, id uuid.UUID, to TreeState) RevertOp {
	now := s.nodes[id]
	op := RevertOp{Kind: kind, NodeID: id, To: to, Move: !SameParent(now.parent, to.ParentID) || now.order != to.SortOrder,
		Rename: now.name != to.Name}
	now.parent, now.name, now.key, now.order = to.ParentID, to.Name, s.keyAt(id, to), to.SortOrder
	s.set(id, now)
	return op
}

// delete is the deletion of the subtree of top, done on s.
func (s *revertSim) delete(top uuid.UUID) RevertOp {
	for _, id := range s.subtree(top) {
		n := s.nodes[id]
		n.alive = false
		s.set(id, n)
	}
	return RevertOp{Kind: RevertDelete, NodeID: top}
}

// restore is the restore of the trash item of top, done on s: each member
// of its group where the changeset deleted it, a page at its content's
// revision in the trash, at the time it is deleted at now.
func (p *planner) restore(s *revertSim, top uuid.UUID) RevertOp {
	group := p.s.Groups[top]
	op := RevertOp{Kind: RevertRestore, NodeID: top, To: *p.c.items[top].Before, Group: group, States: make([]TreeState, len(group)),
		DeletedAt: *p.s.Nodes[top].DeletedAt}
	for i, m := range group {
		state := *p.c.items[m.Node.ID].Before
		s.set(m.Node.ID, simNode{parent: state.ParentID, name: state.Name, key: shared.TitleKey(state.Name), order: state.SortOrder,
			kind: m.Node.Kind, alive: true, revision: m.Revision})
		op.States[i] = state
	}
	return op
}

// fallback is the first of the three that applies to s stuck, the step it
// takes and the node it moves; false for none (M9/P3 design 3.16.3):
// first, each place and restore pending in turn, by id, whose parent in
// S* is alive and whose key there a node to place holds that is not
// renamed aside yet: that holder renamed aside where it is. Second, of
// the nodes S* keeps alive under a node the revert deletes, the deletions
// pending, those not moved aside yet, the least by id moved aside to the
// root: it is to be placed, its parent in S* alive while the one it is
// under now is none of S*'s. Third, a defence: of the nodes to place, not
// at the root and not moved aside yet, the least by id moved aside to the
// root. A node moved aside is renamed too, from its name as the plan
// began, never another aside name on one; it stays to be placed.
func (p *planner) fallback(s *revertSim) (RevertOp, bool) {
	waiting := slices.Concat(p.pending(s.places), p.pending(s.restores))
	for _, id := range waiting {
		to := *p.c.items[id].Before
		if !s.aliveOrRoot(to.ParentID) {
			continue
		}
		if h, held := s.holder(to.ParentID, shared.TitleKey(to.Name)); held && h != id && s.places[h] && !s.renamed[h] {
			if op, ok := p.aside(s, h, false); ok {
				return op, true
			}
		}
	}
	var kept []uuid.UUID
	for _, top := range p.tops(s) {
		for _, id := range s.subtree(top) {
			if _, ok := p.star[id]; ok && !s.movedAside[id] && s.nodes[id].parent != nil && p.c.gone(*s.nodes[id].parent) {
				kept = append(kept, id)
			}
		}
	}
	slices.SortFunc(kept, uuid.UUID.Compare)
	for _, id := range kept {
		if op, ok := p.aside(s, id, true); ok {
			return op, true
		}
	}
	for _, id := range p.pending(s.places) {
		if s.nodes[id].parent != nil && !s.movedAside[id] {
			if op, ok := p.aside(s, id, true); ok {
				return op, true
			}
		}
	}
	return RevertOp{}, false
}

// pending are the nodes of set, in id order.
func (p *planner) pending(set map[uuid.UUID]bool) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

// aside is the step aside of the node id, done on s: renamed where it is;
// or, toRoot, moved to the root and renamed, at the order it has. Its name
// is AsideName's, of its name as the plan began, free there now and none
// of S*'s keys there; false when its premise fails.
func (p *planner) aside(s *revertSim, id uuid.UUID, toRoot bool) (RevertOp, bool) {
	parent := s.nodes[id].parent
	if toRoot {
		parent = nil
	}
	n := p.s.Nodes[id]
	under, star := s.under[parentOf(parent)], s.star[parentOf(parent)]
	name := AsideName(n.Name, id, n.Kind == KindAsset, func(key string) bool {
		holder, held := under[key]
		return held && holder != id || star[key]
	})
	to := TreeState{ParentID: parent, Name: name, SortOrder: s.nodes[id].order}
	if !p.canPlaceOK(s, id, to) {
		return RevertOp{}, false
	}
	s.renamed[id], s.movedAside[id] = true, s.movedAside[id] || toRoot
	return s.place(RevertAside, id, to), true
}
