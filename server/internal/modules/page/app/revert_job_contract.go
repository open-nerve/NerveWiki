package app

import (
	"iter"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// checkJobContract judges the nominal batches' actual group actions, before
// locks and parsing. Runtime batches still re-read and guard their writes.
func checkJobContract(checked revertChecked, job JobPlan, cuts [][2]int, notebook domain.NotebookState, ask func(shared.Action) error) error {
	return checkRevertFacts(jobRevertFacts(checked, job, cuts, notebook.SourceRootID), notebook, ask)
}

// jobRevertFacts uses compact payloads and the same per-batch action
// enumeration as RevertBatch. The initial checked restore payload supplies
// each selected member's trash bytes and target state, never future members.
func jobRevertFacts(checked revertChecked, job JobPlan, cuts [][2]int, sourceRootID uuid.UUID) iter.Seq[domain.StepFacts] {
	members := map[uuid.UUID]domain.TrashMember{}
	states := map[uuid.UUID]domain.TreeState{}
	for _, op := range checked.plan.Ops {
		if op.Kind != domain.RevertRestore {
			continue
		}
		for i, member := range op.Group {
			members[member.Node.ID], states[member.Node.ID] = member, op.States[i]
		}
	}
	actions := func(yield func(domain.RevertOp) bool) {
		for _, cut := range cuts {
			steps := job.revert.steps[cut[0]:cut[1]]
			ops := job.revert.ops(steps, restoring{})
			for _, action := range revertActions(steps) {
				op := ops[action.index]
				if op.Kind == domain.RevertRestore {
					op.Group = make([]domain.TrashMember, len(action.members))
					op.States = make([]domain.TreeState, len(action.members))
					for i, at := range action.members {
						id := steps[at].node
						op.Group[i], op.States[i] = members[id], states[id]
					}
				}
				if !yield(op) {
					return
				}
			}
		}
	}
	return domain.RevertActionFacts(checked.state, actions, sourceRootID)
}
