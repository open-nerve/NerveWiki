package bootstrap

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// The first batch puts Notes back, freeing raw's name for the second
// (M10/P1 design 3.28). Bob starts as an admin, but a demotion queued
// behind that batch can make the source root's put-back a writer's:
// page.revert still passes, its step's mode guard refuses it. The first
// batch stays committed, and the refusal reaches the persisted report.
func TestTheSourceRootsPutBackIsRefusedInALaterBatch(t *testing.T) {
	tm := newAcmeTeamWired(t, "member", "", nil, transfer.Batch{Nodes: 1})
	for try := range betweenUnits {
		nb := tm.llmWiki(t, "Wiki "+strconv.Itoa(try))
		bobs := tm.addNotebookMember(t, "alice", nb.id, "bob", "admin")
		notes := tm.createPage(t, "alice", nb.id, "", "Notes")
		if nb.raw >= notes {
			t.Fatal("raw must precede Notes by id, so its put-back first waits for Notes")
		}
		c := tm.agent(t, "bob", nb.id).batchDone(t, opMove("raw.md", "sources.md"), opMove("Notes.md", "raw.md"))
		if !c.Changed || c.ChangesetID == nil {
			t.Fatalf("the two renames = %+v, want one changed changeset", c)
		}
		demote := request("alice", http.MethodPatch, "/api/v0/notebook-members/"+bobs, `{"role":"editor"}`)
		j, demoted := tm.behindFirstBatch(t, nb.id, revertStart("bob", nb.id, *c.ChangesetID, ""), demote)
		if !demoted.is(http.StatusOK, "") {
			t.Fatalf("bob's demotion = %d %s, want 200", demoted.status, demoted.body)
		}
		if name := tm.bgName(t, notes); name != "Notes" {
			t.Fatalf("the first batch left Notes named %q, want Notes", name)
		}
		if j.State == "succeeded" {
			if j.counts()["put_back"] != 2 || tm.bgName(t, nb.raw) != "raw" || j.Report.Refusal != nil {
				t.Fatalf("the second batch first: %s, raw named %q; want both put back", j, tm.bgName(t, nb.raw))
			}
			checkPages(t, tm.pool)
			continue
		}
		if j.State != "failed" || j.failure() != "refused" || j.counts()["put_back"] != 1 || j.Report.Refusal == nil ||
			j.Report.Refusal.Code != "page.source_admin_only" || j.Report.Refusal.NodeID != nb.raw || j.Report.Lock != nil {
			t.Fatalf("the demotion between batches: %s, want refused at the source root after one put-back", j)
		}
		if name := tm.bgName(t, nb.raw); name != "sources" {
			t.Errorf("the refused second batch left raw named %q, want sources", name)
		}
		checkPages(t, tm.pool)
		return
	}
	t.Fatalf("the demotion came after the second batch, %d times of %d", betweenUnits, betweenUnits)
}
