package bootstrap

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// Both a single unit and one-node batches use the full plan to keep a
// duplicate parent's descendants. Reports are read through the real API.
func TestTheSourceImportKeepsDuplicateParentsChildren(t *testing.T) {
	for _, batchNodes := range []int{1, 100} {
		t.Run(fmt.Sprintf("batch_%d", batchNodes), func(t *testing.T) {
			tm := newAcmeTeamWired(t, "member", "", nil, transfer.Batch{Nodes: batchNodes})
			wiki := tm.llmWiki(t, "Import duplicates")
			tm.createPage(t, "alice", wiki.id, "", "Target")
			const existing = "same source [[Target]]\n"
			tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Known", existing)
			tm.upload(t, "alice", wiki.id, wiki.raw, "known.bin", "")
			beforeNodes := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1", wiki.id)
			beforeFiles := len(storedBlobs(t, tm.storage))
			archive := vaultZip(t,
				vaultFile{"Parent.md", existing}, vaultFile{"Parent/Child.md", "unique child\n"},
				vaultFile{"Leaf.md", existing}, vaultFile{"emptyA.md", ""}, vaultFile{"emptyB.md", ""},
				vaultFile{"copy.bin", ""}, vaultFile{"Fresh.md", "fresh unique source\n"},
				vaultFile{"Next.md", "fresh unique source\n"}, vaultFile{"A.bin", "asset bytes"}, vaultFile{"B.bin", "asset bytes"})
			j := tm.imported(t, "bob", wiki.id, wiki.raw, archive)
			if j.State != "succeeded" || j.Report == nil || j.Report.Failure != nil ||
				j.Report.Counts["pages"] != 4 || j.Report.Counts["attachments"] != 1 || j.Report.Counts["skipped"] != 5 ||
				j.Report.Counts["renamed"] != 0 || j.Progress.Done != 10 || j.Progress.Total != 10 {
				t.Errorf("duplicate import = %+v, report=%+v; want 4 pages, 1 attachment, 5 skips, all 10 processed", j, j.Report)
			}
			status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/transfer-jobs/"+j.ID, tm.tokens["bob"], "")
			var detail struct {
				Problems []struct {
					Path string  `json:"path"`
					Code string  `json:"code"`
					To   *string `json:"to"`
				} `json:"problems"`
			}
			decodeAnswer(t, body, &detail)
			got := map[string]string{}
			for _, p := range detail.Problems {
				if p.Code != "duplicate_source" || p.To == nil {
					t.Errorf("duplicate problem = %+v, want duplicate_source and current notebook path", p)
					continue
				}
				got[p.Path] = *p.To
			}
			want := map[string]string{"Parent.md": "raw/Known.md", "Leaf.md": "raw/Known.md", "copy.bin": "raw/known.bin", "Next.md": "raw/Fresh.md", "B.bin": "raw/A.bin"}
			if status != http.StatusOK || len(detail.Problems) != 5 || !maps.Equal(got, want) {
				t.Errorf("duplicate problems = %d %s, want %v", status, body, want)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1", wiki.id); n != beforeNodes+6 {
				t.Errorf("import created %d nodes, want empty Parent, Child, two empty pages, Fresh and A.bin", n-beforeNodes)
			}
			rows := queryStrings(t, tm.pool, `SELECT n.name || ':' || c.content FROM nodes n JOIN page_contents c ON c.node_id=n.id
				WHERE n.notebook_id=$1 AND n.name IN ('Parent','Child','emptyA','emptyB','Fresh','Leaf','Next') AND n.deleted_at IS NULL ORDER BY n.name`, wiki.id)
			if !slices.Equal(rows, []string{"Child:unique child\n", "Fresh:fresh unique source\n", "Parent:", "emptyA:", "emptyB:"}) {
				t.Errorf("imported pages = %q, want an empty duplicate parent with its child intact and no duplicate leaves", rows)
			}
			if count(t, tm.pool, `SELECT count(*) FROM nodes c JOIN nodes p ON p.id=c.parent_id
				WHERE c.notebook_id=$1 AND c.name='Child' AND p.name='Parent' AND p.parent_id=$2`, wiki.id, wiki.raw) != 1 {
				t.Error("duplicate parent's child lost its real parent mapping")
			}
			if n := len(storedBlobs(t, tm.storage)); n != beforeFiles+1 {
				t.Errorf("stored attachment files = %d, want %d; duplicate uploads must be removed", n, beforeFiles+1)
			}
			if files := storedImports(t, tm.storage); len(files) != 0 {
				t.Errorf("finished duplicate import kept archives: %v", files)
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
		})
	}
}
