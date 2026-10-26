package bootstrap

import (
	"net/http"
	"slices"
	"testing"
)

// The import's merged folders and both restores number reserved names
// at an LLM Wiki's root, skipping the names already imported there.
// The same archive in a standard notebook keeps its names.
func TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	wiki := tm.createWith(t, tm.tokens["alice"], `{"name":"Field notes","mode":"llm_wiki"}`)
	standard := tm.createWith(t, tm.tokens["alice"], `{"name":"Standard notes"}`)
	archive := vaultZip(t, vaultFile{"index.md", "index"}, vaultFile{"Index/a.md", "a"}, vaultFile{"log/b.md", "b"})
	for _, tt := range []struct {
		nb       string
		problems []string
		tree     []string
	}{
		{wiki.ID, []string{"renamed index.md", "renamed log/"},
			[]string{"index 2/a a", "log 2/b b", "root/index 2 index", "root/log 2 "}},
		{standard.ID, nil, []string{"index/a a", "log/b b", "root/index index", "root/log "}},
	} {
		j := tm.imported(t, "alice", tt.nb, "", archive)
		var problems []string
		for _, p := range j.Problems {
			problems = append(problems, p.Code+" "+p.Path)
		}
		if j.State != "succeeded" || j.Report == nil || j.Report.Failure != nil || j.Report.Counts["pages"] != 4 ||
			j.Report.Counts["attachments"] != 0 || j.Report.Counts["renamed"] != len(tt.problems) ||
			j.Progress.Done != 4 || j.Progress.Total != 4 || !slices.Equal(problems, tt.problems) {
			t.Fatalf("import into %s = %+v, report %+v; want four pages and problems %q", tt.nb, j, describe(j.Report), tt.problems)
		}
		got := queryStrings(t, tm.pool, `SELECT coalesce(p.name, 'root') || '/' || n.name || ' ' || c.content
			FROM nodes n LEFT JOIN nodes p ON p.id = n.parent_id JOIN page_contents c ON c.node_id = n.id
			WHERE n.notebook_id = $1 AND n.deleted_at IS NULL AND n.name_key NOT IN ('raw', 'agents') ORDER BY 1`, tt.nb)
		if !slices.Equal(got, tt.tree) {
			t.Errorf("imported tree = %q, want %q", got, tt.tree)
		}
	}
	trashChild := func(name string) string {
		t.Helper()
		parent := tm.createPage(t, "alice", wiki.ID, "", "Parent "+name)
		child := tm.createPage(t, "alice", wiki.ID, parent, name)
		tm.send(t, nodeDeletion("alice", child), http.StatusNoContent)
		tm.send(t, nodeDeletion("alice", parent), http.StatusNoContent)
		return child
	}
	log := trashChild("log")
	restored := tm.restored(t, "alice", log)
	if restored.ID != log || restored.Name != "log 3" || !restored.Renamed || restored.Placed.at() != " parent_in_trash" {
		t.Errorf("sync restore = %+v; want log 3 at the root, renamed, its parent in trash", restored)
	}
	index := trashChild("index")
	j := tm.background(t, trashRestoreStart("alice", wiki.ID, index))
	if j.State != "succeeded" || j.Report == nil || j.Report.Failure != nil || j.Report.Refusal != nil || j.Report.Placed == nil ||
		j.Report.Placed.at() != " parent_in_trash" || j.counts()["pages"] != 1 || j.counts()["attachments"] != 0 ||
		j.counts()["renamed"] != 1 || j.Progress.Done != 1 || j.Progress.Total != 1 {
		t.Fatalf("background restore = %s; want one page at the root, renamed, its parent in trash", j)
	}
	got := queryStrings(t, tm.pool, `SELECT name || ' ' || name_key || ' ' || (parent_id IS NULL)::text
		FROM nodes WHERE id IN ($1, $2) AND deleted_at IS NULL ORDER BY name`, index, log)
	if !slices.Equal(got, []string{"index 3 index 3 true", "log 3 log 3 true"}) {
		t.Errorf("restored nodes = %q, want index 3 and log 3 at the root", got)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL
		AND parent_id IS NULL AND name IN ('index 2', 'log 2')`, wiki.ID); n != 2 {
		t.Errorf("%d imported numbered roots remain, want 2", n)
	}
	checkPages(t, tm.pool)
	checkBackground(t, tm)
}
