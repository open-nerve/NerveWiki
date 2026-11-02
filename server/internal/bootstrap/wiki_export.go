package bootstrap

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

// The contributor borrows transfer's existing snapshot and its completed
// plan. It neither starts another transaction nor invokes the lint runtime.
type wikiExport struct {
	notebooks notebook.Notebooks
	files     *wiki.VirtualFiles
}

func (c wikiExport) Contribute(ctx context.Context, scope transfer.ExportScope, sink transfer.ExportSink) error {
	if scope.RootID != nil {
		return nil
	}
	state, found, err := c.notebooks.StateOf(ctx, scope.NotebookID)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("virtual export: notebook missing")
	}
	if state.Mode != notebook.ModeLLMWiki {
		return nil
	}
	paths := make([]wiki.ExportPath, len(scope.Nodes))
	for i, node := range scope.Nodes {
		paths[i] = wiki.ExportPath{ID: node.ID, Asset: node.Asset, Path: node.Path}
	}
	view := wiki.VirtualScope{NotebookID: scope.NotebookID, SourceRootID: state.SourceRootID,
		Export: &wiki.VirtualExportScope{Paths: paths}}
	for _, name := range []wiki.FileName{wiki.IndexFile, wiki.LogFile} {
		content, err := c.files.GenerateInSnapshot(ctx, view, name, wiki.ExportFile)
		if err != nil {
			return err
		}
		if err := sink.Add(string(name), content); err != nil {
			return err
		}
	}
	return nil
}
