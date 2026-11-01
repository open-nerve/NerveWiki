package bootstrap

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

type wikiFileIndex struct{ reads linking.IndexFileReads }

func (p wikiFileIndex) Heads(ctx context.Context, id uuid.UUID) ([]wiki.FileIndexHead, error) {
	rows, err := p.reads.Heads(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]wiki.FileIndexHead, len(rows))
	for i, r := range rows {
		out[i] = wiki.FileIndexHead{ID: r.ID, Typed: r.Typed, TypeStart: r.TypeStart, TypeKey: r.TypeKey}
	}
	return out, nil
}

func (p wikiFileIndex) Items(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]wiki.FileIndexFacts, error) {
	rows, err := p.reads.Items(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]wiki.FileIndexFacts, len(rows))
	for id, r := range rows {
		out[id] = wiki.FileIndexFacts{Type: r.Type, Summary: r.Summary, Tags: r.Tags}
	}
	return out, nil
}

func (p wikiFileIndex) Inbound(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	return p.reads.Inbound(ctx, ids)
}

type wikiFileTree struct{ reads page.VirtualTreeReads }

func (p wikiFileTree) Nodes(ctx context.Context, id uuid.UUID) ([]wiki.VirtualTreeNode, error) {
	rows, err := p.reads.Nodes(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]wiki.VirtualTreeNode, len(rows))
	for i, r := range rows {
		out[i] = wiki.VirtualTreeNode{ID: r.ID, ParentID: r.ParentID, Name: r.Name, Asset: r.Asset}
	}
	return out, nil
}

func (p wikiFileTree) ChangedAt(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]time.Time, error) {
	return p.reads.ChangedAt(ctx, ids)
}

type wikiFileSources struct{ reads page.SourceArea }

func (p wikiFileSources) Nodes(ctx context.Context, notebook, root uuid.UUID) ([]wiki.VirtualSourceNode, error) {
	rows, err := p.reads.Nodes(ctx, notebook, root)
	if err != nil {
		return nil, err
	}
	out := make([]wiki.VirtualSourceNode, len(rows))
	for i, r := range rows {
		out[i] = wiki.VirtualSourceNode{ID: r.ID, Asset: r.Kind == page.KindAsset, Bytes: r.PageBytes}
	}
	return out, nil
}

type wikiFileLog struct{ reads page.VirtualLogReads }

func (p wikiFileLog) Recent(ctx context.Context, notebook uuid.UUID, limit int) ([]wiki.VirtualLogChangeset, error) {
	rows, err := p.reads.Recent(ctx, notebook, limit)
	if err != nil {
		return nil, err
	}
	out := make([]wiki.VirtualLogChangeset, len(rows))
	for i, r := range rows {
		cs := wiki.VirtualLogChangeset{ID: r.ID, Kind: r.Kind, Client: r.Client, ByName: r.ByName, CreatedAt: r.CreatedAt,
			Message: r.Message, NodeCount: r.NodeCount, ContentCount: r.ContentCount, IngestCount: r.IngestCount,
			Changes: make([]wiki.VirtualLogChange, len(r.Changes)), IngestSources: make([]wiki.VirtualLogSource, len(r.IngestSources))}
		for j, c := range r.Changes {
			cs.Changes[j] = wiki.VirtualLogChange{NodeID: c.NodeID, Name: c.Name, Path: c.Path, Kind: c.Kind, Asset: c.Asset}
		}
		for j, s := range r.IngestSources {
			cs.IngestSources[j] = wiki.VirtualLogSource{NodeID: s.NodeID, Path: s.Path, Asset: s.Asset}
		}
		out[i] = cs
	}
	return out, nil
}

type pageVirtualFiles struct{ files *wiki.VirtualFiles }

func (p pageVirtualFiles) GenerateInSnapshot(ctx context.Context, s page.VirtualReadScope, name page.VirtualName) (string, error) {
	if s.Notebook.SourceRootID == (uuid.UUID{}) {
		return "", errors.New("virtual file: missing source root")
	}
	var file wiki.FileName
	switch name {
	case page.VirtualIndex:
		file = wiki.IndexFile
	case page.VirtualLog:
		file = wiki.LogFile
	default:
		return "", errors.New("virtual file: unknown name")
	}
	b, err := p.files.GenerateInSnapshot(ctx, wiki.VirtualScope{NotebookID: s.NotebookID, SourceRootID: s.Notebook.SourceRootID}, file, wiki.ReadFile)
	return string(b), err
}
