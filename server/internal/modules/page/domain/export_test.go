package domain

import (
	"errors"
	"uuid"
)

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
// how many steps were pending as it began, for the test of its bound (T2;
// M9/P3 S2 fix round 1, A-M1).
func ScheduleRounds(s RevertState, maxDepth int) (ops []RevertOp, rounds, pending int, err error) {
	p := planner{s: s, c: readReverted(s.Items, s.Versions), reasons: map[uuid.UUID]revertReason{}, missing: map[uuid.UUID]bool{},
		members: membersOf(s.Groups), maxDepth: maxDepth}
	p.left()
	if len(s.Items) > 0 {
		p.before()
	}
	if plan := p.plan(); plan.Count > 0 || len(plan.Missing) > 0 {
		return nil, 0, 0, errors.New("the revert has conflicts or missing versions")
	}
	sim := p.newRevertSim()
	pending = len(sim.places) + len(p.tops(sim)) + len(sim.restores) + len(sim.writes)
	ops, err = p.schedule(sim)
	return ops, sim.rounds, pending, err
}
