package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yuin/goldmark"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// These assertions consume the real serve job's downloaded ZIP. Removing
// bootstrap's contributor must lose the two files and their meta entries.
func TestVirtualExport(t *testing.T) {
	t.Run("whole_last_hop", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		wiki := tm.llmWiki(t, "ExportWiki")
		tm.createPageWith(t, "alice", wiki.id, "", "Guide", "---\ntype: note\nsummary: a real guide\n---\nGuide body")
		job := tm.endedJob(t, "alice", tm.startExport(t, "alice", wiki.id, "").ID)
		if job.State != "succeeded" || job.Download == nil {
			t.Fatalf("whole LLM export failed: %+v", job)
		}
		_, entries, _ := tm.archiveAt(t, job.Download.URL)
		for _, name := range []string{"index.md", "log.md"} {
			file, ok := entries["ExportWiki/"+name]
			if !ok || !strings.Contains(file.data, "](<Guide.md>)") {
				t.Fatalf("whole LLM ZIP missing virtual %s or its real Guide destination: %q", name, file.data)
			}
		}
		if entries["ExportWiki/Guide.md"].data != "---\ntype: note\nsummary: a real guide\n---\nGuide body" {
			t.Fatal("virtual contribution lost the real page")
		}
		var meta struct {
			Contributed []string `json:"contributed"`
		}
		if err := json.Unmarshal([]byte(entries["ExportWiki/.nerve/meta.json"].data), &meta); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(meta.Contributed, []string{"index.md", "log.md"}) {
			t.Fatalf("whole LLM ZIP contributed metadata = %q", meta.Contributed)
		}
		checkPages(t, tm.pool)
	})
	t.Run("scope", virtualExportScopes)
	t.Run("snapshot", virtualExportSnapshot)
	t.Run("folder_only_paths", virtualExportFolders)
	t.Run("renamed_subtree_paths", virtualExportRenamed)
	t.Run("path_identity", virtualExportIdentity)
	t.Run("percent_space_identity", func(t *testing.T) { virtualExportIdentityOf(t, []string{"x%20y.md", "x y.md"}) })
	t.Run("entity_slash_identity", func(t *testing.T) { virtualExportIdentityOf(t, []string{"folder&sol;leaf.md", "folder/leaf.md"}) })
	t.Run("meta_skip", virtualExportMetaSkip)
	t.Run("fallback_conflict", virtualExportFallbackConflict)
	t.Run("map_consumers", virtualExportMapConsumers)
	t.Run("no_inbound", virtualExportNoInbound)
	t.Run("percent_budget", virtualExportPercentBudget)
	// With 235 percent signs, the next line is 1015 encoded bytes versus
	// 545 before encoding, with 968 bytes left at the real 2MiB boundary.
	t.Run("percent_next_item_budget", func(t *testing.T) { virtualExportPercentBudgetOf(t, 235) })
}

func virtualExportEntries(t *testing.T, tm acmeTeam, notebook, root string) map[string]zipEntry {
	t.Helper()
	job := tm.endedJob(t, "alice", tm.startExport(t, "alice", notebook, root).ID)
	if job.State != "succeeded" || job.Download == nil {
		t.Fatalf("export failed: %+v", job)
	}
	_, entries, _ := tm.archiveAt(t, job.Download.URL)
	return entries
}

func virtualExportScopes(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "ScopeWiki")
	parent := tm.createPageWith(t, "alice", wiki.id, "", "Parent", "parent")
	tm.createPageWith(t, "alice", wiki.id, parent, "index", "real nested index")
	for _, tt := range []struct {
		name, notebook, root, prefix string
		virtual                      bool
	}{{"whole", wiki.id, "", "ScopeWiki/", true}, {"subtree", wiki.id, parent, "Parent/", false},
		{"standard", tm.openNotebook(t, "alice", "Eng"), "", "Eng/", false}} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "standard" {
				tm.createPageWith(t, "alice", tt.notebook, "", "index", "real standard index")
				tm.createPageWith(t, "alice", tt.notebook, "", "log", "real standard log")
			}
			entries := virtualExportEntries(t, tm, tt.notebook, tt.root)
			_, _, contributed := metaPaths(t, entries[tt.prefix+".nerve/meta.json"].data)
			if tt.virtual {
				if !slices.Equal(contributed, []string{"index.md", "log.md"}) || entries[tt.prefix+"Parent/index.md"].data != "real nested index" {
					t.Fatalf("whole contribution or real nested page lost: %q", contributed)
				}
			} else if len(contributed) != 0 {
				t.Fatalf("%s unexpectedly contributed: %q", tt.name, contributed)
			}
			if tt.name == "subtree" {
				if _, exists := entries[tt.prefix+"index.md"]; exists || entries[tt.prefix+"Parent/index.md"].data != "real nested index" {
					t.Fatal("subtree added a virtual root or lost its real nested page")
				}
			}
			if tt.name == "standard" && (entries[tt.prefix+"index.md"].data != "real standard index" || entries[tt.prefix+"log.md"].data != "real standard log") {
				t.Fatal("standard export overwrote its real same-name pages")
			}
		})
	}
}

type virtualContribution func(context.Context, transfer.ExportScope, transfer.ExportSink) error

func (f virtualContribution) Contribute(ctx context.Context, scope transfer.ExportScope, sink transfer.ExportSink) error {
	return f(ctx, scope, sink)
}

func virtualExportSnapshot(t *testing.T) {
	for _, operation := range []string{"rename", "downgrade"} {
		t.Run(operation, func(t *testing.T) {
			gate := &sourceWriteGate{reached: make(chan struct{}), release: make(chan struct{})}
			t.Cleanup(gate.open)
			tm := newAcmeTeamWiring(t, "member", "", wiring{transfer: func(d *transfer.Deps) {
				if len(d.Contributors) != 1 {
					t.Fatal("real wiki contributor was not registered")
				}
				actual := d.Contributors[0]
				d.Contributors[0] = virtualContribution(func(ctx context.Context, scope transfer.ExportScope, sink transfer.ExportSink) error {
					var isolation, readonly string
					if err := postgres.DB(ctx, d.Pool).QueryRow(ctx, "SELECT current_setting('transaction_isolation'), current_setting('transaction_read_only')").Scan(&isolation, &readonly); err != nil {
						return err
					}
					if !postgres.InTx(ctx) || isolation != "repeatable read" || readonly != "on" {
						t.Errorf("export contribution outside original read-only RR: %s/%s", isolation, readonly)
					}
					close(gate.reached)
					select {
					case <-gate.release:
					case <-ctx.Done():
						return ctx.Err()
					}
					return actual.Contribute(ctx, scope, sink)
				})
			}})
			wiki := tm.llmWiki(t, "SnapshotWiki")
			tm.createPageWith(t, "alice", wiki.id, "", "BeforeAlpha", "alpha")
			m := tm.agent(t, "alice", wiki.id)
			job := tm.startExport(t, "alice", wiki.id, "")
			select {
			case <-gate.reached:
			case <-time.After(interleavingWait):
				t.Fatal("export did not reach its real contribution after planning in RR")
			}
			if operation == "rename" {
				structured(t, sourceMCPCall(t, m, "account", "move", map[string]any{"from": "BeforeAlpha.md", "to": "AfterAlpha.md", "update_links": false}))
			} else if status, body := tm.downgrade(t, "alice", wiki.id); status != 200 {
				t.Fatalf("concurrent downgrade: %d %s", status, body)
			}
			gate.open()
			job = tm.endedJob(t, "alice", job.ID)
			if job.State != "succeeded" || job.Download == nil {
				t.Fatalf("snapshot export failed: %+v", job)
			}
			_, entries, _ := tm.archiveAt(t, job.Download.URL)
			for _, name := range []string{"index.md", "log.md"} {
				body := entries["SnapshotWiki/"+name].data
				if !strings.Contains(body, "](<BeforeAlpha.md>)") || strings.Contains(body, "AfterAlpha") {
					t.Fatalf("snapshot %s mixed paths/mode with concurrent %s: %q", name, operation, body)
				}
			}
			if entries["SnapshotWiki/BeforeAlpha.md"].data != "alpha" {
				t.Fatal("snapshot export lost its real page")
			}
		})
	}
}

func virtualExportFolders(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "FolderWiki")
	a := tm.createPage(t, "alice", wiki.id, "", "A")
	tm.createPageWith(t, "alice", wiki.id, a, "B", "child")
	tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Source", "source")
	// Both the template body and AGENTS's real inbound link make raw a
	// file in the export plan. Remove both through ordinary writes before
	// asserting the distinct folder-only source-area representation.
	m := tm.agent(t, "alice", wiki.id)
	structured(t, sourceMCPCall(t, m, "notebook", "write", map[string]any{"path": "raw.md", "content": "", "base_version": wiki.raw + ":1"}))
	structured(t, sourceMCPCall(t, m, "notebook", "write", map[string]any{"path": "AGENTS.md", "content": "Instructions without links.", "base_version": wiki.agents + ":1"}))
	entries := virtualExportEntries(t, tm, wiki.id, "")
	if _, ok := entries["FolderWiki/A/"]; !ok || entries["FolderWiki/A/B.md"].data != "child" {
		t.Fatal("real ZIP lost its folder-only A or child B")
	}
	if _, ok := entries["FolderWiki/A.md"]; ok {
		t.Fatal("folder-only A acquired a nonexistent Markdown file")
	}
	index := entries["FolderWiki/index.md"].data
	if !strings.Contains(index, "- A; changed:") || strings.Contains(index, "[A](") || !strings.Contains(index, "Source area: raw/ — 1 sources.") {
		t.Fatalf("folder-only index/source destinations are not non-links: %q", index)
	}
	for _, name := range []string{"index.md", "log.md"} {
		body := entries["FolderWiki/"+name].data
		if !strings.Contains(body, "](<A/B.md>)") || strings.Contains(body, "](<A.md>)") || strings.Contains(body, "](<raw.md>)") {
			t.Fatalf("%s guessed a live path instead of its ZIP mapping: %q", name, body)
		}
	}
}

func virtualExportRenamed(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "RenamedWiki")
	tm.createPageWith(t, "alice", wiki.id, "", "N", "first")
	parent := tm.createPage(t, "alice", wiki.id, "", "N.md")
	tm.createPageWith(t, "alice", wiki.id, parent, "c", "child")
	entries := virtualExportEntries(t, tm, wiki.id, "")
	if entries["RenamedWiki/N.md"].data != "first" || entries["RenamedWiki/N.md 2/c.md"].data != "child" {
		t.Fatal("real ZIP did not use its collision-renamed subtree")
	}
	for _, name := range []string{"index.md", "log.md"} {
		body := entries["RenamedWiki/"+name].data
		if !strings.Contains(body, "](<N.md 2/c.md>)") || strings.Contains(body, "](<N.md/c.md>)") {
			t.Fatalf("%s did not link to the actual renamed ZIP entry: %q", name, body)
		}
	}
	if !strings.Contains(entries["RenamedWiki/index.md"].data, "## Parent: N.md 2") {
		t.Fatal("untyped index group used the live parent instead of the export directory")
	}
}

func virtualExportIdentity(t *testing.T) {
	virtualExportIdentityOf(t, []string{"x%41.md", "xA.md", "x%20y.md", "x y.md", "x&amp;Y.md", "x&Y.md", "folder&sol;leaf.md", "folder/leaf.md"})
}

func virtualExportIdentityOf(t *testing.T, paths []string) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "IdentityWiki")
	for i, path := range paths {
		parent, title := "", strings.TrimSuffix(path, ".md")
		if path == "folder/leaf.md" {
			parent = tm.createPage(t, "alice", wiki.id, "", "folder")
			title = "leaf"
		}
		tm.createPageWith(t, "alice", wiki.id, parent, title, "identity "+string(rune('a'+i)))
	}
	entries := virtualExportEntries(t, tm, wiki.id, "")
	for _, name := range []string{"index.md", "log.md"} {
		var output bytes.Buffer
		if err := goldmark.Convert([]byte(entries["IdentityWiki/"+name].data), &output); err != nil {
			t.Fatal(err)
		}
		destinations := map[string]bool{}
		for _, link := range regexp.MustCompile(`href="([^"]*)"`).FindAllStringSubmatch(output.String(), -1) {
			path, err := url.PathUnescape(html.UnescapeString(link[1]))
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := entries["IdentityWiki/"+path]; !ok {
				t.Fatalf("%s rendered destination %q has no actual ZIP entry", name, path)
			}
			destinations[path] = true
		}
		for i, path := range paths {
			if !destinations[path] || entries["IdentityWiki/"+path].data != "identity "+string(rune('a'+i)) {
				t.Fatalf("%s collapsed identity %q onto another real ZIP entry", name, path)
			}
		}
	}
}
