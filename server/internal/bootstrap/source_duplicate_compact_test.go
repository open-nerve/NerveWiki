package bootstrap

import (
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// A creation and its deferred rewrite keep a nil base and rewrite=false in
// their merged version. Undo deletes them without inventing another rewrite.
// This public history corrects the design's original A-I1 example; differences
// between synchronous and compact ordering need their separate module fixture.
func TestTheSourceCreationRewriteUndoKeepsItsRecordedBase(t *testing.T) {
	for _, mode := range []string{"sync", "background"} {
		t.Run(mode, func(t *testing.T) {
			tm := newAcmeTeamWired(t, "member", "", nil, transfer.Batch{Nodes: 100})
			wiki := tm.llmWiki(t, "Compact duplicate "+mode)
			f1 := tm.createPage(t, "alice", wiki.id, wiki.raw, "F1")
			f2 := tm.createPage(t, "alice", wiki.id, wiki.raw, "F2")
			target := tm.createPage(t, "alice", wiki.id, f1, "Old")
			tm.createPage(t, "alice", wiki.id, f2, "New")
			const a, b = "[x](./Old.md)", "[x](./New.md)"
			d := tm.agent(t, "alice", wiki.id).batchDone(t,
				opWrite("raw/F1/S1.md", a), opWrite("raw/F2/S2.md", b),
				opWith(opMove("raw/F1/Old.md", "raw/F1/New.md"), "update_links", true))
			if d.ChangesetID == nil || len(d.Results) != 3 {
				t.Fatalf("creation and deferred rewrite = %+v", d)
			}
			s1, s2 := d.Results[0].NodeID, d.Results[1].NodeID
			if s1 == "" || s2 == "" || s1 >= s2 {
				t.Fatalf("S1 must precede S2 in the compact plan: %s / %s", s1, s2)
			}
			tm.wrote(t, s1, b, 2)
			tm.wrote(t, s2, b, 1)
			tm.sourceRewriteVersion(t, *d.ChangesetID, s1, b, 0, 2, false)
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			c := tm.reverted(t, "alice", *d.ChangesetID)
			if count(t, tm.pool, "SELECT count(*) FROM page_revisions WHERE changeset_id=$1 AND node_id IN ($2,$3)", c, s1, s2) != 0 {
				t.Fatal("undo of a creation invented a write-back version or an automatic link rewrite")
			}
			if rows := queryStrings(t, tm.pool, `SELECT n.id::text || ':' || p.content FROM nodes n JOIN page_contents p ON p.node_id=n.id
				WHERE n.id IN ($1,$2) AND n.deleted_at IS NOT NULL AND n.trash_root AND n.source_hash IS NULL ORDER BY n.id`, s1, s2); !slices.Equal(rows, []string{s1 + ":" + b, s2 + ":" + b}) {
				t.Fatalf("independent trash groups after undo D = %q, want S1(B), S2(B)", rows)
			}
			if tm.bgName(t, target) != "Old" {
				t.Fatal("undo D did not return the real link target to Old")
			}
			before := sourceEntryRows(t, tm.pool)
			q := changesetRevert("alice", c)
			if mode == "background" {
				q = revertStart("alice", wiki.id, c, "")
			}
			status, body := ask(t, tm.contract, q.method, tm.base+q.path, tm.tokens[q.by], q.body)
			var problem map[string]any
			decodeAnswer(t, body, &problem)
			_, hasTarget := problem["duplicate_of"]
			if status != http.StatusConflict || problem["code"] != "page.source_duplicate" || hasTarget {
				t.Errorf("%s undo of C = %d %s, want duplicate without a target: both sources remain trash", mode, status, body)
			}
			if after := sourceEntryRows(t, tm.pool); !maps.Equal(after, before) {
				t.Error("refused precheck changed persistent state")
			}
			if count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id=$1", wiki.id) != 0 ||
				count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind='transfer.revert'") != 0 {
				t.Error("precheck enqueued a necessarily refused background plan")
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
		})
	}
}
