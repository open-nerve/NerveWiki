package domain

import "errors"

// MatchIndex is where Replace's matcher finds p in s at or after from, -1
// for nowhere, for the tests to set it against strings.Index (M9/P2 design
// 3.9).
func MatchIndex(s, p string, from int) int { return newMatcher(p).index(s, from) }

// SubtreeReads is how many subtrees p's simulated tree read.
func SubtreeReads(p *BatchPlan) int { return p.tree.subtrees }

// PlanRevertWithin is PlanRevert with pages nesting maxDepth deep, for the
// tests that check the schedule on small trees (M9/P3 design 3.21.1).
func PlanRevertWithin(s RevertState, maxDepth int) (RevertPlan, error) {
	return planRevert(s, maxDepth)
}

// ScheduleRounds is PlanRevertWithin's schedule of s, which has no
// conflict nor missing version: its steps, how many rounds it began and
// how many steps were pending as it began, as the schedule counts them
// for its bound, for the test of the bound (T2; M9/P3 S2 fix rounds 1 and
// 2, A-M1, A's N4).
func ScheduleRounds(s RevertState, maxDepth int) (ops []RevertOp, rounds, pending int, err error) {
	plan, sim, err := scheduled(s, maxDepth)
	if sim == nil {
		return nil, 0, 0, errors.New("the revert has conflicts or missing versions")
	}
	return plan.Ops, sim.rounds, sim.pending, err
}
