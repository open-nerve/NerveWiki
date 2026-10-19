package bootstrap

import (
	"testing"
)

// A revert in the web puts back an agent's move that renames in one place
// (M9/P2 design 3.5.4): under Drafts, Plan and Plan v2, and Plan v2 under
// Final too; the agent moves Drafts/Plan v2.md to Final/Plan.md, which
// one revert puts back where it was, though Plan holds its name under
// Drafts in the meantime, and Plan v2 its old one under Final. The first
// notebook's changeset is reverted at once, the second's in the
// background: in each, Plan v2 is under Drafts again, at its place, the
// others as they were, the revert one item.
func TestARevertPutsBackAnMCPMoveThatRenames(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	for _, background := range []bool{false, true} {
		nb := tm.openNotebook(t, "alice", map[bool]string{false: "At once", true: "In the background"}[background])
		drafts, final := tm.createPage(t, "alice", nb, "", "Drafts"), tm.createPage(t, "alice", nb, "", "Final")
		plan := tm.createPage(t, "alice", nb, drafts, "Plan")
		moved := tm.createPage(t, "alice", nb, drafts, "Plan v2")
		other := tm.createPage(t, "alice", nb, final, "Plan v2")
		place := func(id string) string {
			return queryStrings(t, tm.pool, "SELECT coalesce(parent_id::text, '') || '/' || name || ' ' || sort_order FROM nodes WHERE id = $1",
				id)[0]
		}
		was := map[string]string{plan: place(plan), moved: place(moved), other: place(other)}
		mv := answerOf[mcpMoved](t, tm.agent(t, "alice", nb).use(t, "move", map[string]any{"from": "Drafts/Plan v2.md", "to": "Final/Plan.md"}))
		if mv.Path != "Final/Plan.md" || mv.ChangesetID == nil {
			t.Fatalf("the agent's move = %+v", mv)
		}
		var revert string
		if background {
			job := tm.background(t, revertStart("alice", nb, *mv.ChangesetID, ""))
			if job.State != "succeeded" || job.ChangesetID == nil {
				t.Fatalf("the background revert = %+v, want it succeeded with its changeset", job)
			}
			revert = *job.ChangesetID
		} else {
			revert = tm.reverted(t, "alice", *mv.ChangesetID)
		}
		for id, want := range was {
			if got := place(id); got != want {
				t.Errorf("background %v: the node %s is at %s after the revert, want %s", background, id, got, want)
			}
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id = $1", revert); n != 1 {
			t.Errorf("background %v: the revert has %d items, want one", background, n)
		}
	}
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkBackground(t, tm)
}
