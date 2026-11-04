package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

type linkIngestBefore struct{ before page.IngestBefore }

func (r linkIngestBefore) NodesBefore(ctx context.Context, ids []uuid.UUID) ([]linking.IngestNodeBefore, error) {
	rows, err := r.before.NodesBefore(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]linking.IngestNodeBefore, len(rows))
	for i, n := range rows {
		out[i] = linking.IngestNodeBefore{ID: n.ID, Exists: n.Exists, Kind: string(n.Kind), InSource: n.InSource,
			IsSourceRoot: n.IsSourceRoot, IsRootAgents: n.IsRootAgents, PageBytes: n.PageBytes, PageBytesKnown: n.PageBytesKnown,
			Revision: n.Revision, RevisionKnown: n.RevisionKnown}
	}
	return out, nil
}

type linkIngestMetadata struct{ metadata page.IngestMetadata }

func linkIngestHeads(rows []page.IngestHead) []linking.IngestHead {
	out := make([]linking.IngestHead, len(rows))
	for i, n := range rows {
		out[i] = linking.IngestHead{ID: n.ID, Exists: n.Exists, Live: n.Live, Kind: string(n.Kind), HasContent: n.HasContent,
			Revision: n.Revision, PageBytes: n.PageBytes, InSource: n.InSource, IsSourceRoot: n.IsSourceRoot, IsRootAgents: n.IsRootAgents}
	}
	return out
}
func (r linkIngestMetadata) Now(ctx context.Context, notebook, root uuid.UUID, ids []uuid.UUID) ([]linking.IngestHead, error) {
	rows, err := r.metadata.Now(ctx, notebook, root, ids)
	if err != nil {
		return nil, err
	}
	return linkIngestHeads(rows), nil
}
func (r linkIngestMetadata) Heads(ctx context.Context, notebook, root uuid.UUID, after *uuid.UUID, take int) (linking.IngestHeadBatch, error) {
	batch, err := r.metadata.Heads(ctx, notebook, root, after, take)
	if err != nil {
		return linking.IngestHeadBatch{}, err
	}
	return linking.IngestHeadBatch{Rows: linkIngestHeads(batch.Rows), Last: batch.Last, Done: batch.Done}, nil
}

type linkIngestMarks struct{ marks page.IngestMarks }

func (r linkIngestMarks) Add(ctx context.Context, notebook, changeset uuid.UUID, ids []uuid.UUID) (linking.IngestMarkResult, error) {
	result, err := r.marks.Add(ctx, notebook, changeset, ids)
	return linking.IngestMarkResult{Added: result.Added, Count: result.Count}, err
}
