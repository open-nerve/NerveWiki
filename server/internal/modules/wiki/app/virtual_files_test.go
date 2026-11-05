package app_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
)

type fileReads struct {
	heads        []app.FileIndexHead
	nodes        []app.VirtualTreeNode
	facts        map[uuid.UUID]app.FileIndexFacts
	log          []app.VirtualLogChangeset
	inboundCalls int
	treeCalls    int
	logCalls     int
	logLimit     int
	logContext   context.Context
	logError     error
}

func (f *fileReads) Heads(context.Context, uuid.UUID) ([]app.FileIndexHead, error) {
	return f.heads, nil
}
func (f *fileReads) Items(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]app.FileIndexFacts, error) {
	out := map[uuid.UUID]app.FileIndexFacts{}
	for _, id := range ids {
		out[id] = f.facts[id]
	}
	return out, nil
}
func (f *fileReads) Inbound(context.Context, []uuid.UUID) (map[uuid.UUID]int, error) {
	f.inboundCalls++
	return nil, nil
}
func (f *fileReads) Nodes(context.Context, uuid.UUID) ([]app.VirtualTreeNode, error) {
	f.treeCalls++
	return f.nodes, nil
}
func (f *fileReads) ChangedAt(context.Context, []uuid.UUID) (map[uuid.UUID]time.Time, error) {
	return nil, nil
}
func (f *fileReads) Recent(ctx context.Context, _ uuid.UUID, limit int) ([]app.VirtualLogChangeset, error) {
	f.logCalls++
	f.logLimit, f.logContext = limit, ctx
	return f.log, f.logError
}

type fileSources []app.VirtualSourceNode

func (f fileSources) Nodes(context.Context, uuid.UUID, uuid.UUID) ([]app.VirtualSourceNode, error) {
	return f, nil
}

func TestVirtualFiles(t *testing.T) {
	t.Run("ingest", testVirtualIngestLog)
	t.Run("groups", func(t *testing.T) {
		root, parent, a, z, b, r, c := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		alpha, beta := "Alpha", "Beta"
		f := &fileReads{
			nodes: []app.VirtualTreeNode{{ID: root, Name: "raw"}, {ID: parent, Name: "P"}, {ID: a, Name: "A"}, {ID: z, Name: "Z"}, {ID: b, Name: "B"}, {ID: r, Name: "Root"}, {ID: c, Name: "Child", ParentID: &parent}},
			heads: []app.FileIndexHead{{ID: c}, {ID: z, Typed: true, TypeKey: "a"}, {ID: b, Typed: true, TypeKey: "b"}, {ID: r}, {ID: a, Typed: true, TypeKey: "a"}, {ID: root}},
			facts: map[uuid.UUID]app.FileIndexFacts{a: {Type: &alpha}, z: {Type: &alpha}, b: {Type: &beta}},
		}
		v := app.NewVirtualFiles(app.VirtualFileDeps{Index: f, Tree: f, Sources: fileSources{}, Log: f})
		out, err := v.GenerateInSnapshot(context.Background(), app.VirtualScope{NotebookID: uuid.NewV7(), SourceRootID: root}, app.IndexFile, app.ReadFile)
		if err != nil {
			t.Fatal(err)
		}
		got := string(out)
		last := -1
		for _, want := range []string{"## Type: Alpha\n", "- [A](<A.md>)", "- [Z](<Z.md>)", "## Type: Beta\n", "- [B](<B.md>)", "## Parent: /\n", "- [Root](<Root.md>)", "- [raw](<raw.md>)", "## Parent: P\n", "- [Child](<P/Child.md>)"} {
			at := strings.Index(got, want)
			if at <= last {
				t.Fatalf("missing or out-of-order group/title/path %q in %q", want, got)
			}
			last = at
		}
		if strings.Count(got, "## Type: Alpha") != 1 || strings.Contains(got, "[P/Child.md]") {
			t.Fatal("group repeated or path replaced the title label")
		}
	})
	t.Run("source_line", func(t *testing.T) {
		root, a, b, asset, empty := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		f := &fileReads{nodes: []app.VirtualTreeNode{{ID: root, Name: "raw"}}, heads: []app.FileIndexHead{{ID: root}}}
		for _, id := range []uuid.UUID{a, b, empty} {
			f.nodes = append(f.nodes, app.VirtualTreeNode{ID: id, ParentID: &root, Name: id.String()})
			f.heads = append(f.heads, app.FileIndexHead{ID: id})
		}
		f.nodes = append(f.nodes, app.VirtualTreeNode{ID: asset, ParentID: &root, Name: "one.txt", Asset: true})
		sources := fileSources{{ID: a, Bytes: 1}, {ID: b, Bytes: 2}, {ID: asset, Asset: true}, {ID: empty}}
		v := app.NewVirtualFiles(app.VirtualFileDeps{Index: f, Tree: f, Sources: sources, Log: f})
		bts, err := v.GenerateInSnapshot(context.Background(), app.VirtualScope{NotebookID: uuid.NewV7(), SourceRootID: root}, app.IndexFile, app.ReadFile)
		if err != nil {
			t.Fatal(err)
		}
		got := string(bts)
		if !strings.Contains(got, "Source area: [raw.md](<raw.md>) — 3 sources. Use pending_sources or lint for status.") || !strings.Contains(got, "Showing 1 of 1 pages.") {
			t.Fatalf("source count or strict membership: %q", got)
		}
		if strings.Contains(got, empty.String()) || f.inboundCalls != 1 {
			t.Fatal("empty/source descendant leaked or read profile did not get inbound")
		}
	})
	t.Run("export_mapping", func(t *testing.T) {
		root, note, gone := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
		old := "raw/Old.md"
		f := &fileReads{nodes: []app.VirtualTreeNode{{ID: root, Name: "raw"}, {ID: note, Name: "Original"}}, heads: []app.FileIndexHead{{ID: root}, {ID: note}}, log: []app.VirtualLogChangeset{{Changes: []app.VirtualLogChange{{NodeID: note, Name: "Original", Path: &old}, {NodeID: gone, Name: "Gone"}}}}}
		v := app.NewVirtualFiles(app.VirtualFileDeps{Index: f, Tree: f, Sources: fileSources{}, Log: f})
		scope := app.VirtualScope{NotebookID: uuid.NewV7(), SourceRootID: root, Export: &app.VirtualExportScope{Paths: []app.ExportPath{{ID: root, Path: "raw/"}, {ID: note, Path: "renamed%41.md"}}}}
		for _, name := range []app.FileName{app.IndexFile, app.LogFile} {
			b, err := v.GenerateInSnapshot(context.Background(), scope, name, app.ExportFile)
			if err != nil {
				t.Fatal(err)
			}
			got := string(b)
			if !strings.Contains(got, "](<renamed%2541.md>)") || strings.Contains(got, old) || strings.Contains(got, "[raw/](") {
				t.Fatalf("export did not consume mapping: %q", got)
			}
		}
		if f.inboundCalls != 0 {
			t.Fatal("export read inbound counts")
		}
		for _, paths := range [][]app.ExportPath{
			{{ID: root, Path: "raw/"}},
			{{ID: root, Path: "raw/"}, {ID: note, Path: "raw/"}},
			{{ID: root, Path: "raw/"}, {ID: note, Asset: true, Path: "one.md"}},
			{{ID: root, Path: "raw/"}, {ID: note, Path: "../secret.md"}},
		} {
			scope.Export = &app.VirtualExportScope{Paths: paths}
			if _, err := v.GenerateInSnapshot(context.Background(), scope, app.LogFile, app.ExportFile); err == nil {
				t.Fatal("invalid mapping accepted before log generation")
			}
		}
	})
}
