package domain

import (
	"slices"
	"uuid"
)

// SourceFacts adds strict source-area membership to each side of root-first
// facts, using that side's parent graph and the operation root's ancestors.
// Each ancestor line runs from the notebook root through the parent. The
// source root itself is outside; its children, pages or attachments, are in.
// Neither the caller's facts nor its ancestor lines are changed.
func SourceFacts(facts StepFacts, sourceRootID uuid.UUID, before, after []Ancestor, afterParents map[uuid.UUID][]Ancestor) StepFacts {
	facts.Nodes = slices.Clone(facts.Nodes)
	sourcePlaces(facts.Nodes, sourceRootID, before, nil, true)
	sourcePlaces(facts.Nodes, sourceRootID, after, afterParents, false)
	return facts
}

// sourcePlaces marks one side, retaining each node's own parent relation.
// Root-first order puts a changed parent before any child that uses it.
func sourcePlaces(nodes []NodeFacts, root uuid.UUID, line []Ancestor, parents map[uuid.UUID][]Ancestor, before bool) {
	inArea := make(map[uuid.UUID]bool, len(line)+len(nodes))
	under := false
	for _, ancestor := range line {
		inArea[ancestor.ID] = under && ancestor.ID != root
		under = under || ancestor.ID == root
	}
	// Each supplied line belongs to its own external parent. Never join
	// independent lines: seeing the source root on one says nothing of another.
	for parent, ancestors := range parents {
		inArea[parent] = parent != root && slices.ContainsFunc(ancestors, func(a Ancestor) bool { return a.ID == root })
	}
	// Changed parents overwrite external snapshots before their children.
	for i := range nodes {
		place := nodes[i].After
		if before {
			place = nodes[i].Before
		}
		if place == nil {
			continue
		}
		marked := *place
		marked.InArea = root != uuid.Nil() && nodes[i].ID != root && marked.ParentID != nil &&
			(*marked.ParentID == root || inArea[*marked.ParentID])
		inArea[nodes[i].ID] = marked.InArea
		if before {
			nodes[i].Before = &marked
		} else {
			nodes[i].After = &marked
		}
	}
}
