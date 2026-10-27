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
	return RevertActionFacts(state, slices.Values(plan.Ops), sourceRootID)
}

// RevertActionFacts simulates the supplied actions in order. Synchronous
// callers supply whole-group schedule operations; jobs supply only the
// members selected by each actual batch action. The simulation persists
// across the iterator's batch boundaries and reads no database state.
func RevertActionFacts(state RevertState, actions iter.Seq[RevertOp], sourceRootID uuid.UUID) iter.Seq[StepFacts] {
	return func(yield func(StepFacts) bool) {
		p := planner{s: state, c: readReverted(state.Items, state.Versions)}
		sim := p.treeSim()
		contents := make(map[uuid.UUID]ContentNow, len(state.Contents))
		maps.Copy(contents, state.Contents)
		for op := range actions {
			beforeLine := revertSourceLine(sim, state.Lines, sim.nodes[op.NodeID].parent)
			var operation Operation
			var changes []Change
			switch op.Kind {
			case RevertPlace, RevertAside:
				operation = OpRename
				ids := []uuid.UUID{op.NodeID}
				now := sim.nodes[op.NodeID]
				if !SameParent(now.parent, op.To.ParentID) || now.order != op.To.SortOrder {
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
				contents[op.NodeID] = ContentNow{Revision: contents[op.NodeID].Revision + 1, Hash: op.Hash, Bytes: op.Bytes}
			}
			afterLine := revertSourceLine(sim, state.Lines, sim.nodes[op.NodeID].parent)
			var afterParents map[uuid.UUID][]Ancestor
			if op.Kind == RevertRestore {
				afterParents = make(map[uuid.UUID][]Ancestor)
				changed := make(map[uuid.UUID]bool, len(changes))
				for _, c := range changes {
					changed[c.NodeID] = true
				}
				for _, c := range changes {
					if parent := c.After.ParentID; parent != nil && !changed[*parent] {
						if _, ok := afterParents[*parent]; !ok {
							afterParents[*parent] = revertSourceLine(sim, state.Lines, parent)
						}
					}
				}
			}
			if !yield(SourceFacts(facts, sourceRootID, beforeLine, afterLine, afterParents)) {
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
