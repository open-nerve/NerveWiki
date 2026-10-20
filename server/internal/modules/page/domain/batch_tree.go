package domain

import (
	"cmp"
	"slices"
	"uuid"
)

// The tree a batch's plan simulates (M9/P3 design 3.4.1): the notebook's
// nodes alive as read before the batch, indexed by parent and key and by
// key alone, and each op's edits in their order, so that a view of the
// tree after any op reads it as the unit's reads would then: a child by
// key, a subtree level by level, a parent's line, a sibling holding a key.

// batchTree is the tree before the batch and the edits of its ops.
type batchTree struct {
	base  map[uuid.UUID]Node
	kids  map[uuid.UUID][]uuid.UUID // by parent, the root's at the zero id
	slots map[slot]uuid.UUID
	keys  map[string][]uuid.UUID
	edits []treeEdit
	// subtrees counts the subtrees read, which a rename under its parent
	// never reads (M9/P3 review A-N11).
	subtrees int
}

// slot is a key under a parent, the root's at the zero id: one node holds
// it at a time.
type slot struct {
	parent uuid.UUID
	key    string
}

// slotOf is the slot n holds.
func slotOf(n Node) slot {
	s := slot{key: n.NameKey}
	if n.ParentID != nil {
		s.parent = *n.ParentID
	}
	return s
}

// treeEdit is an op's edit of a node: its state after, nil when the op
// deleted it, its state before, nil when the op created it, and whether
// the op follows it with links (a move's update_links; true otherwise).
type treeEdit struct {
	op          int
	after       *Node
	before      *Node
	updateLinks bool
}

// newBatchTree is the tree of nodes, a notebook's alive.
func newBatchTree(nodes []Node) *batchTree {
	t := &batchTree{base: make(map[uuid.UUID]Node, len(nodes)), kids: map[uuid.UUID][]uuid.UUID{},
		slots: make(map[slot]uuid.UUID, len(nodes)), keys: map[string][]uuid.UUID{}}
	for _, n := range nodes {
		t.base[n.ID] = n
		s := slotOf(n)
		t.kids[s.parent] = append(t.kids[s.parent], n.ID)
		t.slots[s] = n.ID
		t.keys[n.NameKey] = append(t.keys[n.NameKey], n.ID)
	}
	return t
}

// treeView is the tree after some of its edits: each node an edit
// changed at its last state there, nil when deleted, over the tree before
// the batch, and the edited nodes alive indexed as the base is.
type treeView struct {
	t     *batchTree
	over  map[uuid.UUID]*Node
	slots map[slot]uuid.UUID
	kids  map[uuid.UUID]map[uuid.UUID]bool
	keys  map[string]map[uuid.UUID]bool
	gone  []uuid.UUID
}

// view is the tree after the edits of the ops up to op, none for -1.
func (t *batchTree) view(op int) *treeView {
	v := &treeView{t: t, over: map[uuid.UUID]*Node{}, slots: map[slot]uuid.UUID{},
		kids: map[uuid.UUID]map[uuid.UUID]bool{}, keys: map[string]map[uuid.UUID]bool{}}
	for _, e := range t.edits {
		if e.op > op {
			break
		}
		v.apply(e)
	}
	return v
}

// record has op's edit e on the view the plan walks, and in the tree's
// edits that later views replay.
func (v *treeView) record(e treeEdit) {
	v.t.edits = append(v.t.edits, e)
	v.apply(e)
}

// apply has the view take e: the node leaves where it was, and holds its
// new place unless e deletes it.
func (v *treeView) apply(e treeEdit) {
	id := idOf(e)
	if was, ok := v.over[id]; ok && was != nil {
		s := slotOf(*was)
		delete(v.slots, s)
		delete(v.kids[s.parent], id)
		delete(v.keys[was.NameKey], id)
	}
	v.over[id] = e.after
	if e.after == nil {
		v.gone = append(v.gone, id)
		return
	}
	s := slotOf(*e.after)
	v.slots[s] = id
	if v.kids[s.parent] == nil {
		v.kids[s.parent] = map[uuid.UUID]bool{}
	}
	v.kids[s.parent][id] = true
	if v.keys[e.after.NameKey] == nil {
		v.keys[e.after.NameKey] = map[uuid.UUID]bool{}
	}
	v.keys[e.after.NameKey][id] = true
}

// idOf is the node e edits.
func idOf(e treeEdit) uuid.UUID {
	if e.after != nil {
		return e.after.ID
	}
	return e.before.ID
}

// node is id's node alive in the view.
func (v *treeView) node(id uuid.UUID) (Node, bool) {
	if n, ok := v.over[id]; ok {
		if n == nil {
			return Node{}, false
		}
		return *n, true
	}
	n, ok := v.t.base[id]
	return n, ok
}

// based reports whether id is a node of the tree before the batch that
// no edit of the view changed: one its base indexes hold where it is.
func (v *treeView) based(id uuid.UUID) bool {
	_, edited := v.over[id]
	_, ok := v.t.base[id]
	return ok && !edited
}

// child is the node keyed key under parent, the root for nil.
func (v *treeView) child(parent *uuid.UUID, key string) (Node, bool) {
	s := slot{key: key}
	if parent != nil {
		s.parent = *parent
	}
	if id, ok := v.slots[s]; ok {
		return v.node(id)
	}
	if id, ok := v.t.slots[s]; ok && v.based(id) {
		return v.t.base[id], true
	}
	return Node{}, false
}

// lookup is the view's point lookup, as NodePaths' is the store's.
func (v *treeView) lookup(chain []Node, key string) (Node, bool, error) {
	var parent *uuid.UUID
	if len(chain) > 0 {
		parent = &chain[len(chain)-1].ID
	}
	n, ok := v.child(parent, key)
	return n, ok, nil
}

// children are the nodes under parent, the root for nil, in their order:
// by order, then id, as the tree reads.
func (v *treeView) children(parent *uuid.UUID) []Node {
	var p uuid.UUID
	if parent != nil {
		p = *parent
	}
	var out []Node
	for _, id := range v.t.kids[p] {
		if v.based(id) {
			out = append(out, v.t.base[id])
		}
	}
	for id := range v.kids[p] {
		out = append(out, *v.over[id])
	}
	slices.SortFunc(out, func(a, b Node) int {
		return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), cmp.Compare(a.ID.String(), b.ID.String()))
	})
	return out
}

// subtree is id's node alive and the nodes under it, level by level, each
// level in its order, as Nodes.Subtree reads it.
func (v *treeView) subtree(id uuid.UUID) Subtree {
	v.t.subtrees++
	n, ok := v.node(id)
	if !ok {
		return nil
	}
	out := Subtree{{Node: n, Level: 1}}
	for i := 0; i < len(out); i++ {
		for _, c := range v.children(&out[i].Node.ID) {
			out = append(out, SubtreeNode{Node: c, Level: out[i].Level + 1})
		}
	}
	slices.SortStableFunc(out, func(a, b SubtreeNode) int { return cmp.Compare(a.Level, b.Level) })
	return out
}

// ancestors are id's node's ancestors from the root, the node left out.
func (v *treeView) ancestors(id uuid.UUID) []Node {
	var out []Node
	n, ok := v.node(id)
	for ok && n.ParentID != nil {
		n, ok = v.node(*n.ParentID)
		if ok {
			out = append(out, n)
		}
	}
	slices.Reverse(out)
	return out
}

// under reports whether id's node is alive under anc's node, at any depth,
// in the view.
func (v *treeView) under(id, anc uuid.UUID) bool {
	n, ok := v.node(id)
	for ok && n.ParentID != nil {
		if *n.ParentID == anc {
			return true
		}
		n, ok = v.node(*n.ParentID)
	}
	return false
}

// line is the ancestors a child of parent has, from the root down to the
// parent itself: none at the root, as lineOf reads them.
func (v *treeView) line(parent *uuid.UUID) []Ancestor {
	if parent == nil {
		return nil
	}
	p, _ := v.node(*parent)
	var out []Ancestor
	for _, a := range v.ancestors(p.ID) {
		out = append(out, Ancestor{ID: a.ID, Name: a.Name})
	}
	return append(out, Ancestor{ID: p.ID, Name: p.Name})
}

// path is id's node's path from the root, its own step last: false when
// it is not alive in the view.
func (v *treeView) path(id uuid.UUID) ([]PathStep, bool) {
	n, ok := v.node(id)
	if !ok {
		return nil, false
	}
	var out []PathStep
	for _, a := range v.ancestors(id) {
		out = append(out, PathStep{ID: a.ID, Key: a.NameKey, Name: a.Name})
	}
	return append(out, PathStep{ID: n.ID, Key: n.NameKey, Name: n.Name}), true
}

// lastOrder is the order after every child of parent: where a node
// created or moved there goes, last among its new siblings.
func (v *treeView) lastOrder(parent *uuid.UUID) float64 {
	order := 0.0
	for _, c := range v.children(parent) {
		order = max(order, c.SortOrder)
	}
	return order + 1
}

// PlanTree is the tree of a batch's plan after one of its ops (M9/P3
// design 3.4.1, 3.7.4): the reads linking resolves links by, on it, and
// the nodes the batch deleted up to then.
type PlanTree struct{ v *treeView }

// NodePath is a node of a PlanTree: its id, its path from the root, its
// own step last, and whether it is an attachment.
type NodePath struct {
	ID    uuid.UUID
	Path  []PathStep
	Asset bool
}

// ByKeys is the nodes whose title key is one of keys, each with its path.
func (t PlanTree) ByKeys(keys []string) []NodePath {
	var out []NodePath
	for _, key := range keys {
		var ids []uuid.UUID
		for _, id := range t.v.t.keys[key] {
			if t.v.based(id) {
				ids = append(ids, id)
			}
		}
		for id := range t.v.keys[key] {
			ids = append(ids, id)
		}
		slices.SortFunc(ids, uuid.UUID.Compare)
		out = append(out, t.Paths(ids)...)
	}
	return out
}

// Paths is the nodes among ids alive in the tree, each with its path.
func (t PlanTree) Paths(ids []uuid.UUID) []NodePath {
	var out []NodePath
	for _, id := range ids {
		n, ok := t.v.node(id)
		if !ok {
			continue
		}
		path, _ := t.v.path(id)
		out = append(out, NodePath{ID: id, Path: path, Asset: n.Kind == KindAsset})
	}
	return out
}

// Subtree is the node id and the nodes under it, as steps.
func (t PlanTree) Subtree(id uuid.UUID) []PathStep {
	var out []PathStep
	for _, n := range t.v.subtree(id) {
		out = append(out, PathStep{ID: n.Node.ID, Key: n.Node.NameKey, Name: n.Node.Name})
	}
	return out
}

// Gone is the nodes the batch deleted up to the tree's op.
func (t PlanTree) Gone() []uuid.UUID { return slices.Clone(t.v.gone) }
