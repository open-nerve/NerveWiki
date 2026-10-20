package domain

import (
	"encoding/hex"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// simWith is a revert's simulation of the nodes alive, each its id, its
// parent and its name, a page, with S*'s keys under the root star, and a
// planner of nodes, as the plan began, of the same.
func simWith(star []string, alive ...simAlive) (*planner, *revertSim) {
	p := &planner{s: RevertState{Nodes: map[uuid.UUID]NodeNow{}}, maxDepth: MaxDepth}
	s := &revertSim{nodes: map[uuid.UUID]simNode{}, under: map[uuid.UUID]map[string]uuid.UUID{}, star: map[uuid.UUID]map[string]bool{},
		places: map[uuid.UUID]bool{}, restores: map[uuid.UUID]bool{}, writes: map[uuid.UUID]RevertOp{}, renamed: map[uuid.UUID]bool{},
		movedAside: map[uuid.UUID]bool{}}
	for _, n := range alive {
		s.set(n.id, simNode{parent: n.parent, name: n.name, key: shared.TitleKey(n.name), kind: KindPage, alive: true})
		p.s.Nodes[n.id] = NodeNow{Node: Node{ID: n.id, ParentID: n.parent, Kind: KindPage, Name: n.name, NameKey: shared.TitleKey(n.name)}}
	}
	s.star[uuid.UUID{}] = map[string]bool{}
	for _, name := range star {
		s.star[uuid.UUID{}][shared.TitleKey(name)] = true
	}
	return p, s
}

// simAlive is a node simWith has alive.
type simAlive struct {
	id     uuid.UUID
	parent *uuid.UUID
	name   string
}

// A step aside's name is free where it goes now and none of S*'s keys
// there (M9/P3 design 3.17): a node S* keeps in the trash, to be restored
// later, is never kept from its name by it.
func TestAStepAsideTakesNoKeyOfS(t *testing.T) {
	b := uuid.MustParse("0192f3a4-5b6c-7d8e-9fa0-b1c2d3e4f5a6")
	last := "A ~" + hex.EncodeToString(b[12:])
	for _, tt := range []struct {
		name  string
		star  []string
		alive []simAlive
		want  string
	}{
		{"nothing in the way", []string{"A", "B"}, nil, last},
		{"a node of S* in the trash named so", []string{"A", "B", last}, nil, "A ~" + hex.EncodeToString(b[:])},
		{"a node alive named so", []string{"A", "B"}, []simAlive{{uuid.NewV7(), nil, last}}, "A ~" + hex.EncodeToString(b[:])},
	} {
		p, s := simWith(tt.star, append(tt.alive, simAlive{b, nil, "A"})...)
		s.places[b] = true
		op, ok := p.aside(s, b, false)
		if !ok || op.Kind != RevertAside || op.To.Name != tt.want || op.Move || !op.Rename || !s.renamed[b] || s.movedAside[b] {
			t.Errorf("%s: aside = %+v, %v; want %q, a rename where it is", tt.name, op, ok, tt.want)
		}
	}
}

// The third fallback, a defence no reachable state has needed, is tested
// on a revert stuck as none is (M9/P3 design 3.16.3, 3.21.1): no node to
// place holds a key another waits for, each waiting for a parent gone,
// and no deletion pending. Of the nodes to place, it moves the least by
// id that is not at the root and not moved aside yet to the root, renamed
// aside; a lesser one at the root, and a lesser one moved aside already,
// are never taken.
func TestTheThirdFallbackMovesAPlaceOffTheRoot(t *testing.T) {
	folder, gone := uuid.NewV7(), uuid.NewV7()
	atRoot, movedAside, least, other := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	p, s := simWith(nil, simAlive{folder, nil, "F"}, simAlive{atRoot, nil, "R"}, simAlive{movedAside, &folder, "M"},
		simAlive{least, &folder, "L"}, simAlive{other, &folder, "O"})
	var items []ChangesetItem
	for _, id := range []uuid.UUID{atRoot, movedAside, least, other} {
		items = append(items, ChangesetItem{NodeID: id, Before: &TreeState{ParentID: &gone, Name: p.s.Nodes[id].Name},
			After: new(s.nodes[id].state())})
		s.places[id] = true
	}
	p.c = readReverted(items, nil)
	s.movedAside[movedAside] = true
	op, ok := p.fallback(s)
	want := AsideName("L", least, false, func(string) bool { return false })
	if !ok || op.Kind != RevertAside || op.NodeID != least || op.To.ParentID != nil || op.To.Name != want || !op.Move || !op.Rename {
		t.Fatalf("fallback() = %+v, %v; want L moved aside to the root as %q", op, ok, want)
	}
	if !s.movedAside[least] || !s.renamed[least] || !s.places[least] || s.nodes[least].parent != nil {
		t.Errorf("after it L is moved aside %v, renamed %v, to place %v, under %v; want all, at the root", s.movedAside[least],
			s.renamed[least], s.places[least], s.nodes[least].parent)
	}
}

// (T4): a schedule stuck with no fallback applying answers
// ErrRevertUnordered, no loop and no other step (M9/P3 design 3.16.4).
// Only a defect gets there: here S* holds two nodes of one key under the
// root, the one the revert would place back and one it leaves, which
// PlanRevert's check of S* would have refused as title_taken.
func TestScheduleAnswersUnorderedForADefect(t *testing.T) {
	kept, placed := uuid.NewV7(), uuid.NewV7()
	p, s := simWith([]string{"X"}, simAlive{kept, nil, "X"}, simAlive{placed, nil, "Y"})
	p.c = readReverted([]ChangesetItem{{NodeID: placed, Before: &TreeState{Name: "X"}, After: &TreeState{Name: "Y"}}}, nil)
	p.star = map[uuid.UUID]starNode{kept: {name: "X", key: "x", kind: KindPage}, placed: {name: "X", key: "x", kind: KindPage}}
	s.places[placed] = true
	if ops, err := p.schedule(s); !errors.Is(err, ErrRevertUnordered) || ops != nil {
		t.Errorf("schedule() = %+v, %v; want ErrRevertUnordered and no step", ops, err)
	}
}

// A node is renamed aside once (T1; M9/P3 design 3.16.3): a holder the
// first fallback renamed aside already is never renamed aside again, even
// holding the key a place waits for, which only a defect gets to (its
// name aside is none of S*'s keys). Here none of the fallbacks applies.
func TestAFallbackRenamesANodeAsideOnce(t *testing.T) {
	h, z, gone := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	aside := "A ~" + hex.EncodeToString(h[12:])
	p, s := simWith(nil, simAlive{h, nil, aside}, simAlive{z, nil, "Z"})
	p.c = readReverted([]ChangesetItem{{NodeID: h, Before: &TreeState{ParentID: &gone, Name: "A"}, After: &TreeState{Name: "A"}},
		{NodeID: z, Before: &TreeState{Name: aside}, After: &TreeState{Name: "Z"}}}, nil)
	s.places[h], s.places[z], s.renamed[h] = true, true, true
	if op, ok := p.fallback(s); ok {
		t.Errorf("fallback() = %+v, want none: H renamed aside already", op)
	}
}

// A trash item restored under a node another trash item restores waits
// for it, a round later (M9/P3 design 3.16.2, 3.21.1). No batch nor
// revert makes such a changeset (3.21.2), and PlanRevert reads the
// deletion of a node under another it deleted as one group: the planner
// is built as two such groups would read, B, the lesser id, tried first.
func TestARestoreWaitsForTheGroupItGoesUnder(t *testing.T) {
	b, a, x := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	early, late := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC), time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	states := map[uuid.UUID]TreeState{a: {Name: "A"}, x: {ParentID: &a, Name: "X"}, b: {ParentID: &x, Name: "B"}}
	s := RevertState{Nodes: map[uuid.UUID]NodeNow{}, Groups: map[uuid.UUID][]TrashMember{}, At: late.Add(time.Hour)}
	var items []ChangesetItem
	for _, id := range []uuid.UUID{b, a, x} {
		at := states[id]
		n := Node{ID: id, ParentID: at.ParentID, Kind: KindPage, Name: at.Name, NameKey: shared.TitleKey(at.Name)}
		deleted, pages := new(late), 1
		switch id {
		case b:
			deleted = new(early)
		case a:
			pages = 2
		}
		s.Nodes[id] = NodeNow{Node: n, DeletedAt: deleted, TrashRoot: id != x, TrashPages: pages}
		items = append(items, ChangesetItem{NodeID: id, Before: new(at), Revision: 1})
	}
	s.Items = items
	s.Groups[a] = []TrashMember{{Node: s.Nodes[a].Node, Level: 1, Revision: 1}, {Node: s.Nodes[x].Node, Level: 2, Revision: 1}}
	s.Groups[b] = []TrashMember{{Node: s.Nodes[b].Node, Level: 1, Revision: 1}}
	p := planner{s: s, c: readReverted(items, nil), reasons: map[uuid.UUID]revertReason{}, missing: map[uuid.UUID]bool{},
		members: membersOf(s.Groups), maxDepth: MaxDepth}
	p.c.tops, p.c.top[b], p.c.members[a], p.c.members[b] = []uuid.UUID{b, a}, b, []uuid.UUID{a, x}, []uuid.UUID{b}
	p.before()
	if len(p.reasons) != 0 {
		t.Fatalf("S* = %v", p.reasons)
	}
	ops, err := p.schedule(p.newRevertSim())
	if err != nil || len(ops) != 2 || ops[0].NodeID != a || ops[1].NodeID != b || ops[1].Kind != RevertRestore ||
		!SameParent(ops[1].To.ParentID, &x) || !ops[1].DeletedAt.Equal(early) || ops[1].Seq != 1 {
		t.Errorf("schedule() = %+v, %v; want A's group restored, then B under X", ops, err)
	}
}
