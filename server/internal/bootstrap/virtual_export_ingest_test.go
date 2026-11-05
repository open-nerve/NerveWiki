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

// Only an optional path is removed, after the real producer and the real
// Recent/bridge have supplied the nonempty facts and their persistent IDs.
type virtualIngestWithoutPaths struct {
	wiki.VirtualLogReads
	cleared *atomic.Int32
}

func (r virtualIngestWithoutPaths) Recent(ctx context.Context, notebook uuid.UUID, limit int) ([]wiki.VirtualLogChangeset, error) {
	rows, err := r.VirtualLogReads.Recent(ctx, notebook, limit)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		for j := range rows[i].IngestSources {
			source := &rows[i].IngestSources[j]
			if source.NodeID != nil && source.Path != nil {
				r.cleared.Add(1)
			}
			source.Path = nil
		}
	}
	return rows, nil
}

func virtualExportIngestSources(t *testing.T, clearPaths bool) {
	var cleared atomic.Int32
	tm := newAcmeTeamWiring(t, "member", "", wiring{transfer: func(d *transfer.Deps) {
		if !clearPaths {
			return
		}
		if len(d.Contributors) != 1 {
			t.Fatal("real contributor missing")
		}
		actual := d.Contributors[0].(wikiExport)
		actual.files = wiki.NewVirtualFiles(wiki.VirtualFileDeps{
			Index:   wikiFileIndex{linking.NewIndexFileReads(d.Pool)},
			Tree:    wikiFileTree{page.NewVirtualTreeReads(d.Pool)},
			Sources: wikiFileSources{page.NewSourceArea(d.Pool)},
			Log:     virtualIngestWithoutPaths{wikiFileLog{page.NewVirtualLogReads(d.Pool, displayNames{identity.NewDirectory(d.Pool)})}, &cleared},
		})
		d.Contributors[0] = actual
	}})
	book := tm.llmWiki(t, "IngestMapWiki")
	tm.createPageWith(t, "alice", book.id, "", "N", "sibling file")
	source := tm.createPageWith(t, "alice", book.id, book.raw, "Leaf", "page source")
	asset := tm.upload(t, "alice", book.id, book.raw, "clip.png", pngHead+"pixels")
	m := tm.agent(t, "alice", book.id)
	structured(t, sourceMCPCall(t, m, "notebook", "write", map[string]any{"path": "raw.md", "content": "", "base_version": book.raw + ":1"}))
	structured(t, sourceMCPCall(t, m, "notebook", "write", map[string]any{"path": "AGENTS.md", "content": "Instructions without links.", "base_version": book.agents + ":1"}))
	structured(t, sourceMCPCall(t, m, "notebook", "move", map[string]any{"from": "raw.md", "to": "N.md.md"}))
	note := tm.createPageWith(t, "alice", book.id, "", "Summary", "before")
	written := answerOf[mcpWritten](t, sourceMCPCall(t, m, "notebook", "write", map[string]any{
		"path": "Summary.md", "base_version": note + ":1", "content": "[[N.md/Leaf]]\n![](N.md/clip.png)\n",
	}))
	if !written.Changed || written.NodeID != note || written.ChangesetID == nil {
		t.Fatalf("real producer wrote no Summary changeset: %+v", written)
	}
	cs := *written.ChangesetID
	ids := []string{source, asset.ID}
	slices.Sort(ids)
	if got := queryStrings(t, tm.pool, "SELECT notebook_id::text || ':' || kind || ':' || ingest_count::text FROM changesets WHERE id=$1 AND deleted_at IS NULL", cs); !slices.Equal(got, []string{book.id + ":edit:2"}) {
		t.Fatalf("real producer identity/count = %q, want %s:edit:2", got, book.id)
	}
	if got := queryStrings(t, tm.pool, "SELECT seq::text || ':' || node_id::text FROM changeset_sources WHERE changeset_id=$1 ORDER BY seq", cs); !slices.Equal(got, []string{"1:" + ids[0], "2:" + ids[1]}) {
		t.Fatalf("real producer complete marks = %q, want seq1/2 of %q", got, ids)
	}
	entries := virtualExportEntries(t, tm, book.id, "")
	if entries["IngestMapWiki/N.md"].data != "sibling file" || entries["IngestMapWiki/N.md 2/Leaf.md"].data != "page source" || entries["IngestMapWiki/N.md 2/clip.png"].data != pngHead+"pixels" {
		t.Fatal("real collision plan did not preserve the sibling, page source and attachment")
	}
	if _, exists := entries["IngestMapWiki/N.md 2/"]; !exists {
		t.Fatal("real export did not rename the source root's folder")
	}
	_, _, contributed := metaPaths(t, entries["IngestMapWiki/.nerve/meta.json"].data)
	if !slices.Equal(contributed, []string{"index.md", "log.md"}) {
		t.Fatalf("real ZIP's contributed metadata = %q", contributed)
	}
	entry := virtualIngestEntry(t, entries["IngestMapWiki/log.md"].data, cs)
	if !strings.HasSuffix(entry[0], "; ingests: 2") {
		t.Fatalf("actual producer's log count = %q, want 2", entry[0])
	}
	paths := map[string]string{source: "N.md 2/Leaf.md", asset.ID: "N.md 2/clip.png"}
	want := []string{}
	for _, id := range ids {
		path := paths[id]
		want = append(want, "  - source: ["+path+"](<"+path+">)")
	}
	got := []string{}
	for _, line := range entry[1:] {
		if strings.HasPrefix(line, "  - source: ") {
			got = append(got, line)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("source NodeID-to-ZIP mapping = %q, want complete ordered %q", got, want)
	}
	if clearPaths && cleared.Load() != 2 {
		t.Fatalf("nil-Path fixture cleared %d live producer source paths, want exactly 2", cleared.Load())
	}
	checkPages(t, tm.pool)
}

func virtualIngestEntry(t *testing.T, log, cs string) []string {
	t.Helper()
	lines := strings.Split(log, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "- ") && strings.Contains(line, " "+cs+" — ") {
			if start != -1 {
				t.Fatalf("real log repeats changeset %s", cs)
			}
			start = i
		}
	}
	if start == -1 {
		t.Fatalf("real log omitted producer changeset %s: %q", cs, log)
	}
	end := start + 1
	for end < len(lines) && strings.HasPrefix(lines[end], "  - ") {
		end++
	}
	return lines[start:end]
}
