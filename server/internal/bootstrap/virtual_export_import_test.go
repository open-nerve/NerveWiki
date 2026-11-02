package bootstrap

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

func virtualExportMetaSkip(t *testing.T) {
	t.Run("real_zip", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		wiki := tm.llmWiki(t, "ImportWiki")
		tm.createPageWith(t, "alice", wiki.id, "", "Guide", "real content")
		job := tm.endedJob(t, "alice", tm.startExport(t, "alice", wiki.id, "").ID)
		if job.State != "succeeded" || job.Download == nil {
			t.Fatalf("export before reimport: %+v", job)
		}
		_, entries, _ := tm.archiveAt(t, job.Download.URL)
		paths, _, contributed := metaPaths(t, entries["ImportWiki/.nerve/meta.json"].data)
		if !slices.Equal(contributed, []string{"index.md", "log.md"}) || slices.Contains(paths, "index.md") || slices.Contains(paths, "log.md") {
			t.Fatalf("real ZIP meta confuses contributed files and nodes: %q/%q", contributed, paths)
		}
		req := newRequest(t, http.MethodGet, tm.base+job.Download.URL, "", nil)
		response, archive := sendRequest(t, req)
		tm.contract.CheckResponse(t, req, response)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("download for real ZIP reimport: %d", response.StatusCode)
		}
		for _, mode := range []string{"llm", "standard"} {
			t.Run(mode, func(t *testing.T) {
				target := tm.openNotebook(t, "alice", "Eng")
				want := []string{"AGENTS", "Guide", "raw"}
				if mode == "llm" {
					target = tm.llmWiki(t, "ImportedWiki").id
					want = []string{"AGENTS", "AGENTS 2", "Guide", "raw", "raw 2"}
				}
				imported := tm.imported(t, "alice", target, "", archive)
				if imported.State != "succeeded" {
					t.Fatalf("reimport actual ZIP: %+v", imported)
				}
				var names []string
				for _, node := range tm.rootOf(t, target) {
					names = append(names, node.Name)
					if node.Name == "Guide" && node.Content != "real content" {
						t.Fatal("contributed skip discarded or changed the real Guide")
					}
				}
				slices.Sort(names)
				if !slices.Equal(names, want) {
					t.Fatalf("real contributed ZIP became nodes: %q, want %q", names, want)
				}
			})
		}
		checkPages(t, tm.pool)
	})
	t.Run("exact_nfc", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		archive := vaultZip(t,
			vaultFile{"V/Cafe\u0301.md", "contributed NFC spelling"},
			vaultFile{"V/café.md", "case-distinct real page"},
			vaultFile{"V/Folder/Café.md", "nested real page"},
			vaultFile{"V/.nerve/meta.json", `{"format":1,"nodes":[],"contributed":["Cafe\u0301.md"]}`})
		if job := tm.imported(t, "alice", nb, "", archive); job.State != "succeeded" {
			t.Fatalf("NFC contributed import: %+v", job)
		}
		got := queryStrings(t, tm.pool, `SELECT n.name || ':' || c.content FROM nodes n JOIN page_contents c ON c.node_id=n.id
			WHERE n.notebook_id=$1 AND c.content<>'' ORDER BY c.content`, nb)
		if !slices.Equal(got, []string{"café:case-distinct real page", "Café:nested real page"}) {
			t.Fatalf("contributed matching was not exact NFC: %q", got)
		}
	})
}

func virtualExportFallbackConflict(t *testing.T) {
	for _, metadata := range []string{"absent", "oversize_first"} {
		t.Run(metadata, func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			files := []vaultFile{{"V/.obsidian/app.json", "{}"}, {"V/index.md", "fallback index"}, {"V/log.md", "fallback log"}}
			if metadata == "oversize_first" {
				// The first meta really crosses 16 MiB. A later valid one may
				// not take its place and silently suppress the two real files.
				meta := `{"format":1,"contributed":["index.md","log.md"]}`
				files = append(files, vaultFile{"V/.nerve/meta.json", meta + strings.Repeat(" ", (16<<20)+1-len(meta))},
					vaultFile{"V/.nerve/meta.json", `{"format":1,"contributed":["index.md","log.md"]}`})
			}
			archive := vaultZip(t, files...)
			for _, mode := range []string{"llm", "standard"} {
				t.Run(mode, func(t *testing.T) {
					nb := tm.openNotebook(t, "alice", "Eng")
					want := []string{"index:fallback index", "log:fallback log"}
					if mode == "llm" {
						nb = tm.llmWiki(t, "FallbackWiki").id
						want = []string{"index 2:fallback index", "log 2:fallback log"}
					}
					if job := tm.imported(t, "alice", nb, "", archive); job.State != "succeeded" {
						t.Fatalf("fallback import: %+v", job)
					}
					got := queryStrings(t, tm.pool, `SELECT n.name || ':' || c.content FROM nodes n JOIN page_contents c ON c.node_id=n.id
						WHERE n.notebook_id=$1 AND c.content LIKE 'fallback %' ORDER BY n.name`, nb)
					if !slices.Equal(got, want) {
						t.Fatalf("%s %s fallback skipped or misnumbered the real files: %q, want %q", metadata, mode, got, want)
					}
				})
			}
		})
	}
	t.Run("reserved_folder_conflict", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		wiki := tm.llmWiki(t, "LegacyWiki")
		legacy := tm.createPage(t, "alice", wiki.id, "", "Legacy")
		tm.createPageWith(t, "alice", wiki.id, legacy, "Child", "child")
		// Model an old invalid reserved root; today's write guard correctly
		// cannot create this state. The transfer plan must still refuse it.
		tm.originExec(t, `UPDATE nodes SET name='index.md',name_key='index.md' WHERE id=$1`, legacy)
		job := tm.endedJob(t, "alice", tm.startExport(t, "alice", wiki.id, "").ID)
		if job.State != "failed" || job.Report == nil || job.Report.Failure == nil || *job.Report.Failure != "contributor_conflict" || job.Download != nil {
			t.Fatalf("reserved root folder conflict was swallowed or overwritten: %+v", job)
		}
	})
}
