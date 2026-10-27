package domain

import (
	"iter"
	"maps"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// RevertFacts are the contract facts of a checked plan, each at the
// state left by the steps before it. Its simulation is the scheduler's;
// moves and deletions include the subtree at that moment, root first.
func RevertFacts(state RevertState, plan RevertPlan, sourceRootID uuid.UUID) iter.Seq[StepFacts] {
	return func(yield func(StepFacts) bool) {
		p := planner{s: state, c: readReverted(state.Items, state.Versions)}
		sim := p.treeSim()
		contents := make(map[uuid.UUID]ContentNow, len(state.Contents))
		maps.Copy(contents, state.Contents)
		for _, op := range plan.Ops {
			beforeLine := revertSourceLine(sim, state.Lines, sim.nodes[op.NodeID].parent)
			var operation Operation
			var changes []Change
			switch op.Kind {
			case RevertPlace, RevertAside:
				operation = OpRename
				ids := []uuid.UUID{op.NodeID}
				if op.Move {
					operation, ids = OpMove, sim.orderedSubtree(op.NodeID)
				}
				for _, id := range ids {
					before := sim.nodes[id].state()
					after := before
					if id == op.NodeID {
						after = op.To
					}
					changes = append(changes, Change{NodeID: id, Before: &before, After: &after})
				}
				sim.place(op.Kind, op.NodeID, op.To)
			case RevertDelete:
				operation = OpDelete
				for _, id := range sim.orderedSubtree(op.NodeID) {
					before := sim.nodes[id].state()
					changes = append(changes, Change{NodeID: id, Before: &before})
				}
				sim.delete(op.NodeID)
			case RevertRestore:
				operation = OpRestore
				for i, member := range op.Group {
					after := op.States[i]
					changes = append(changes, Change{NodeID: member.Node.ID, After: &after})
					sim.set(member.Node.ID, simNode{parent: after.ParentID, name: after.Name, key: shared.TitleKey(after.Name),
						order: after.SortOrder, kind: member.Node.Kind, alive: true, revision: member.Revision})
					if member.Node.Kind == KindPage {
						contents[member.Node.ID] = ContentNow{Revision: member.Revision, Bytes: member.Bytes,
							Hash: state.Hashes[VersionRef{NodeID: member.Node.ID, Revision: member.Revision}]}
					}
				}
			case RevertContent:
				operation = OpContent
				before := sim.nodes[op.NodeID].state()
				changes = []Change{{NodeID: op.NodeID, Before: &before, After: &before}}
			}
			facts := FactsOf(operation, ChangesetRevert, changes)
			for i := range facts.Nodes {
				facts.Nodes[i].Kind = sim.nodes[facts.Nodes[i].ID].kind
			}
			if op.Kind == RevertContent {
				facts.Content = &ContentFacts{Hash: op.Hash, Bytes: op.Bytes, BeforeBytes: contents[op.NodeID].Bytes, Merged: 1}
				facts.Rewrite = op.Rewrite
				contents[op.NodeID] = ContentNow{Revision: op.Current + 1, Hash: op.Hash, Bytes: op.Bytes}
			}
			afterLine := revertSourceLine(sim, state.Lines, sim.nodes[op.NodeID].parent)
			if !yield(SourceFacts(facts, sourceRootID, beforeLine, afterLine, nil)) {
				return
			}
		}
	}
}

// revertSourceLine reads a parent line from the current simulation. When
// a content-only plan has no tree, its supplied line fills the unread
// ancestors; a tree plan always follows its current simulated parents.
func revertSourceLine(sim *revertSim, lines map[uuid.UUID][]Ancestor, parent *uuid.UUID) []Ancestor {
	var tail, line []Ancestor
	for parent != nil {
		n, ok := sim.nodes[*parent]
		if !ok {
			line = slices.Clone(lines[*parent])
			break
		}
		tail = append(tail, Ancestor{ID: *parent, Name: n.name})
		parent = n.parent
	}
	slices.Reverse(tail)
	return append(line, tail...)
}
