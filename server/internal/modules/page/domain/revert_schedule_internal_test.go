package domain

import (
	"encoding/hex"
	"testing"
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
