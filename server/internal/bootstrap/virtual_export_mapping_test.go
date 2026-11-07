package bootstrap

import (
	"context"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

func virtualExportMapConsumers(t *testing.T) {
	t.Run("index_path", virtualExportRenamed)
	t.Run("source_path", virtualExportSourcePath)
	t.Run("history_missing", virtualExportMissingHistory)
	t.Run("history_nil_path", virtualExportNilHistoryPath)
	t.Run("ingest_sources", func(t *testing.T) { virtualExportIngestSources(t, false) })
	t.Run("ingest_nil_path", func(t *testing.T) { virtualExportIngestSources(t, true) })
	t.Run("ingest_purged_source", virtualExportIngestPurgedSource)
	t.Run("invalid", virtualExportInvalidMaps)
}

func virtualExportSourcePath(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	book := tm.llmWiki(t, "SourceMapWiki")
	tm.createPageWith(t, "alice", book.id, "", "N", "sibling file")
	tm.createPageWith(t, "alice", book.id, book.raw, "Leaf", "source body")
	m := tm.agent(t, "alice", book.id)
	structured(t, sourceMCPCall(t, m, "notebook", "write", map[string]any{"path": "raw.md", "content": "", "base_version": book.raw + ":1"}))
	structured(t, sourceMCPCall(t, m, "notebook", "write", map[string]any{"path": "AGENTS.md", "content": "Instructions without links.", "base_version": book.agents + ":1"}))
	// Admins may rename the source root. Its empty body and no inbound
	// links make a folder that collides with N's real file in the ZIP.
	structured(t, sourceMCPCall(t, m, "account", "move", map[string]any{"from": "raw.md", "to": "N.md.md", "update_links": false}))
	entries := virtualExportEntries(t, tm, book.id, "")
	if _, ok := entries["SourceMapWiki/N.md 2/"]; !ok || entries["SourceMapWiki/N.md 2/Leaf.md"].data != "source body" {
		t.Fatal("real export did not rename its source-area folder")
	}
	index, log := entries["SourceMapWiki/index.md"].data, entries["SourceMapWiki/log.md"].data
	if !strings.Contains(index, "Source area: N.md 2/ — 1 sources.") || strings.Contains(index, "Source area: [") {
		t.Fatalf("source-area consumer ignored its actual folder mapping: %q", index)
	}
	if !strings.Contains(log, "](<N.md 2/Leaf.md>)") || strings.Contains(log, "](<N.md/Leaf.md>)") {
		t.Fatalf("history consumer guessed the source's old path: %q", log)
	}
}

func virtualExportMissingHistory(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	book := tm.llmWiki(t, "MissingHistoryWiki")
	tm.createPageWith(t, "alice", book.id, "", "Gone", "gone")
	tm.createPageWith(t, "alice", book.id, "", "Kept", "kept")
	m := tm.agent(t, "alice", book.id)
	structured(t, sourceMCPCall(t, m, "notebook", "delete", map[string]any{"path": "Gone.md"}))
	entries := virtualExportEntries(t, tm, book.id, "")
	log := entries["MissingHistoryWiki/log.md"].data
	if !strings.Contains(log, "page: Gone") || strings.Contains(log, "[Gone") || !strings.Contains(log, "](<Kept.md>)") {
		t.Fatalf("missing historical node became a false link or vanished: %q", log)
	}
	if _, ok := entries["MissingHistoryWiki/Gone.md"]; ok {
		t.Fatal("deleted history became an actual ZIP page")
	}
}

type virtualHistoryWithoutPaths struct{ wiki.VirtualLogReads }

func (r virtualHistoryWithoutPaths) Recent(ctx context.Context, notebook uuid.UUID, limit int) ([]wiki.VirtualLogChangeset, error) {
	rows, err := r.VirtualLogReads.Recent(ctx, notebook, limit)
	for i := range rows {
		for j := range rows[i].Changes {
			rows[i].Changes[j].Path = nil
		}
	}
	return rows, err
}

func virtualExportNilHistoryPath(t *testing.T) {
	tm := newAcmeTeamWiring(t, "member", "", wiring{transfer: func(d *transfer.Deps) {
		if len(d.Contributors) != 1 {
			t.Fatal("real contributor missing")
		}
		actual := d.Contributors[0].(wikiExport)
		// The real database supplies every history fact and ID. Remove only
		// the optional live path, which the export must not depend on.
		actual.files = wiki.NewVirtualFiles(wiki.VirtualFileDeps{
			Index:   wikiFileIndex{linking.NewIndexFileReads(d.Pool)},
			Tree:    wikiFileTree{page.NewVirtualTreeReads(d.Pool)},
			Sources: wikiFileSources{page.NewSourceArea(d.Pool)},
			Log:     virtualHistoryWithoutPaths{wikiFileLog{page.NewVirtualLogReads(d.Pool, displayNames{identity.NewDirectory(d.Pool)})}},
		})
		d.Contributors[0] = actual
	}})
	book := tm.llmWiki(t, "NilHistoryWiki")
	tm.createPageWith(t, "alice", book.id, "", "N", "sibling")
	parent := tm.createPage(t, "alice", book.id, "", "N.md")
	tm.createPageWith(t, "alice", book.id, parent, "Child", "child")
	entries := virtualExportEntries(t, tm, book.id, "")
	if !strings.Contains(entries["NilHistoryWiki/log.md"].data, "](<N.md 2/Child.md>)") || entries["NilHistoryWiki/N.md 2/Child.md"].data != "child" {
		t.Fatal("history with a live mapped ID but no old Path lost its real ZIP destination")
	}
}

func virtualExportInvalidMaps(t *testing.T) {
	for _, fault := range []string{"duplicate_id", "duplicate_path", "missing_live", "unknown_id", "wrong_kind",
		"empty", "absolute", "dot", "dotdot", "backslash", "invalid_utf8", "control", "line_separator", "asset_folder"} {
		t.Run(fault, func(t *testing.T) {
			var generations atomic.Int32
			tm := newAcmeTeamWiring(t, "member", "", wiring{transfer: func(d *transfer.Deps) {
				if len(d.Contributors) != 1 {
					t.Fatal("real contributor missing")
				}
				actual := d.Contributors[0].(wikiExport)
				actual.files = wiki.NewVirtualFiles(wiki.VirtualFileDeps{
					Index:   virtualExportIndexProbe{wikiFileIndex{linking.NewIndexFileReads(d.Pool)}, &generations},
					Tree:    wikiFileTree{page.NewVirtualTreeReads(d.Pool)},
					Sources: wikiFileSources{page.NewSourceArea(d.Pool)},
					Log:     wikiFileLog{page.NewVirtualLogReads(d.Pool, displayNames{identity.NewDirectory(d.Pool)})},
				})
				d.Contributors[0] = virtualContribution(func(ctx context.Context, scope transfer.ExportScope, sink transfer.ExportSink) error {
					scope.Nodes = slices.Clone(scope.Nodes)
					switch fault {
					case "duplicate_id":
						n := scope.Nodes[0]
						n.Path = "Another.md"
						scope.Nodes = append(scope.Nodes, n)
					case "duplicate_path":
						scope.Nodes[1].Path = scope.Nodes[0].Path
					case "missing_live":
						scope.Nodes = scope.Nodes[1:]
					case "unknown_id":
						scope.Nodes[0].ID = uuid.NewV7()
					case "wrong_kind":
						scope.Nodes[0].Asset = true
					case "asset_folder":
						for i := range scope.Nodes {
							if scope.Nodes[i].Asset {
								scope.Nodes[i].Path += "/"
							}
						}
					default:
						paths := map[string]string{"empty": "", "absolute": "/Guide.md", "dot": "a/./Guide.md", "dotdot": "a/../Guide.md",
							"backslash": `a\Guide.md`, "invalid_utf8": string([]byte{0xff}) + ".md", "control": "bad\n.md", "line_separator": "bad\u2028.md"}
						scope.Nodes[0].Path = paths[fault]
					}
					return actual.Contribute(ctx, scope, sink)
				})
			}})
			book := tm.llmWiki(t, "InvalidMapWiki")
			guide := tm.createPageWith(t, "alice", book.id, "", "Guide", "guide")
			tm.upload(t, "alice", book.id, guide, "a.png", pngHead+"pixels")
			job := tm.endedJob(t, "alice", tm.startExport(t, "alice", book.id, "").ID)
			if generations.Load() != 0 {
				t.Fatalf("invalid export map %s reached generation before its mapping was rejected", fault)
			}
			if job.State != "failed" || job.Report == nil || job.Report.Failure == nil || *job.Report.Failure != "internal" || job.Download != nil {
				t.Fatalf("invalid export map %s did not fail without publishing an archive: %+v", fault, job)
			}
		})
	}
}

type virtualExportIndexProbe struct {
	wiki.IndexFileReads
	calls *atomic.Int32
}

func (p virtualExportIndexProbe) Heads(ctx context.Context, notebook uuid.UUID) ([]wiki.FileIndexHead, error) {
	p.calls.Add(1)
	return p.IndexFileReads.Heads(ctx, notebook)
}
