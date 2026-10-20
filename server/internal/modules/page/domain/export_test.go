package domain

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
