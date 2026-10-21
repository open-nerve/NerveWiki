package domain

import (
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A background revert's batch (M9/P3 design 3.24): the job planned its
// steps once, as it started, and each batch checks its own under the
// notebook's lock, of the state it reads: L, its nodes as the changeset
// left them; then each step's premise in its order, which stands for S*
// at its moment.

// CheckRevertPart is a background revert's check of a batch (M9/P3 design
// 3.24): L as PlanRevert has it, of the batch's nodes alone, the
// conflicts in their order and the versions the purge took. A page the
// batch restores whose group it reads is checked by its content in the
// trash, against the changeset's at the deletion's revision; one a later
// member of a group an earlier batch began, the batch's caller checks as
// it is in the trash (a continuation). No S*
// and no steps: the job's plan has the steps, and each step's premise,
// checked in its order (CheckRevertSteps), stands for S* at its moment.
func CheckRevertPart(s RevertState) RevertPlan {
	p := planner{s: s, c: readReverted(s.Items, s.Versions), reasons: map[uuid.UUID]revertReason{}, missing: map[uuid.UUID]bool{},
		members: membersOf(s.Groups), maxDepth: MaxDepth}
	p.left()
	return p.plan()
}

// CheckRevertSteps checks a background revert's batch's steps in their
// order (M9/P3 design 3.24, step 4), on the state s the batch read, whose
// L passed: each step's premise at its moment, as the schedule checked
// it, the steps before it done on a simulation of s; it answers the steps
// as the unit does them, or the conflicts of the first that fails, each
// its reason. A place and a step aside go to To, their moves and renames
// from where their nodes are then, its name there a conflict's. A
// deletion's top's subtree then holds no node but its Subtree, the one at
// its moment in the schedule: each other whose parent is in it, the top
// of a part someone added since, is children_added, by itself, as
// PlanRevert's S* names one. A restore of a group's root s holds brings
// back its group, at its states and time as s read them, the group's
// later members with it; a continuation's, a member of a group an earlier
// batch began, which the batch's caller checked as the trash holds it, is
// Group[0] brought back to States[0], none for one someone restored where
// it was. A write back's page is alive, else changed, and written on its
// revision then: L's, or the trash's for a page the batch restores.
func CheckRevertSteps(s RevertState, steps []RevertOp) ([]RevertOp, []RevertConflict) {
	p := &planner{s: s, c: readReverted(s.Items, s.Versions), members: membersOf(s.Groups), maxDepth: MaxDepth}
	sim := p.treeSim()
	out := make([]RevertOp, len(steps))
	for i, op := range steps {
		conflict := func(reason ConflictReason, name string, holder *uuid.UUID) []RevertConflict {
			return []RevertConflict{{NodeID: op.NodeID, Name: name, Reason: reason, HolderID: holder}}
		}
		switch op.Kind {
		case RevertPlace, RevertAside:
			if reason, holder := p.canPlace(sim, op.NodeID, op.To); reason != "" {
				return nil, conflict(reason, op.To.Name, holder)
			}
			done := sim.place(op.Kind, op.NodeID, op.To)
			done.Seq = op.Seq
			out[i] = done
		case RevertDelete:
			if added := sim.added(op.NodeID, op.Subtree); len(added) > 0 {
				return nil, added
			}
			done := sim.delete(op.NodeID)
			done.Seq, done.Subtree = op.Seq, op.Subtree
			out[i] = done
		case RevertRestore:
			_, top := s.Groups[op.NodeID]
			_, member := p.members[op.NodeID]
			switch {
			case top:
				if reason, holder := p.canRestore(sim, op.NodeID); reason != "" {
					return nil, conflict(reason, p.c.items[op.NodeID].Before.Name, holder)
				}
				done := p.restore(sim, op.NodeID)
				done.Seq = op.Seq
				out[i] = done
			case member, len(op.Group) == 0:
				out[i] = op
			default:
				m, state := op.Group[0], op.States[0]
				sim.set(op.NodeID, simNode{parent: state.ParentID, name: state.Name, key: shared.TitleKey(state.Name), order: state.SortOrder,
					kind: m.Node.Kind, alive: true, revision: m.Revision})
				out[i] = op
			}
		case RevertContent:
			if reason := p.canWrite(sim, op.NodeID); reason != "" {
				return nil, conflict(reason, s.Nodes[op.NodeID].Name, nil)
			}
			op.Current = sim.nodes[op.NodeID].revision
			out[i] = op
		}
	}
	return out, nil
}

// added are the conflicts of a batch's deletion of the subtree of top,
// recorded its subtree at its moment in the schedule: each node of it now
// not recorded whose parent is, children_added, by node id.
func (s *revertSim) added(top uuid.UUID, recorded []uuid.UUID) []RevertConflict {
	kept := make(map[uuid.UUID]bool, len(recorded))
	for _, id := range recorded {
		kept[id] = true
	}
	var out []RevertConflict
	for _, id := range s.subtree(top) {
		if n := s.nodes[id]; !kept[id] && n.parent != nil && kept[*n.parent] {
			out = append(out, RevertConflict{NodeID: id, Name: n.name, Reason: ConflictChildrenAdded})
		}
	}
	slices.SortFunc(out, CompareConflicts)
	return out
}
