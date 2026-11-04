package postgresadapter

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func ingestMetadataIDs(ids []uuid.UUID) error {
	if len(ids) > 512 {
		return errors.New("ingest metadata: more than 512 IDs")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1].Compare(ids[i]) >= 0 {
			return errors.New("ingest metadata: IDs must be sorted and unique")
		}
	}
	return nil
}
func ingestMetadataHead(id uuid.UUID, exists, live bool, kind string, content bool, revision int32, size int64,
	root bool, source, agents, line *bool,
) (app.IngestHead, error) {
	h := app.IngestHead{ID: id, Exists: exists, Live: live, Kind: domain.Kind(kind), HasContent: content,
		Revision: int(revision), PageBytes: size, IsSourceRoot: root}
	if source == nil || agents == nil || line == nil || size < 0 || revision < 0 ||
		exists && kind != "page" && kind != "asset" || live && (!exists || !*line || kind == "page" && (!content || revision < 1)) {
		return app.IngestHead{}, errors.New("ingest metadata: invalid live node or ancestry")
	}
	h.InSource, h.IsRootAgents = *source, *agents
	return h, nil
}

// Now includes an explicit absent row for each missing requested ID. No content
// locks or source-area scans are taken, including under the index advisory lock.
func (s *Store) Now(ctx context.Context, notebook, root uuid.UUID, ids []uuid.UUID) ([]app.IngestHead, error) {
	if err := ingestMetadataIDs(ids); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.planned(ctx).IngestNodesNow(ctx, gen.IngestNodesNowParams{NotebookID: notebook, SourceRootID: root,
		Ids: ids, MaxPageDepth: domain.MaxDepth, AgentsKey: shared.TitleKey(domain.AgentsTitle)})
	if err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, errors.New("ingest metadata: incomplete point rows")
	}
	out := make([]app.IngestHead, len(rows))
	for i, r := range rows {
		if r.ID != ids[i] {
			return nil, errors.New("ingest metadata: unordered point rows")
		}
		out[i], err = ingestMetadataHead(r.ID, r.NodeExists, r.Live, r.Kind, r.HasContent, r.Revision, r.PageBytes,
			r.IsSourceRoot, r.InSource, r.IsRootAgents, r.LineOk)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Heads takes LIMIT on raw nodes before role filtering. The tail cursor belongs
// to the raw result, and a full batch requires a subsequent request for EOF.
func (s *Store) Heads(ctx context.Context, notebook, root uuid.UUID, after *uuid.UUID, take int) (app.IngestHeadBatch, error) {
	out := app.IngestHeadBatch{}
	if take < 1 || take > 512 {
		return out, errors.New("ingest metadata: invalid raw take")
	}
	var rows []gen.IngestHeadsFirstRow
	if after == nil {
		var err error
		rows, err = s.planned(ctx).IngestHeadsFirst(ctx, gen.IngestHeadsFirstParams{NotebookID: notebook, SourceRootID: root,
			Take: int32(take), MaxPageDepth: domain.MaxDepth, AgentsKey: shared.TitleKey(domain.AgentsTitle)})
		if err != nil {
			return out, err
		}
	} else {
		got, err := s.planned(ctx).IngestHeadsAfter(ctx, gen.IngestHeadsAfterParams{NotebookID: notebook, SourceRootID: root,
			AfterID: *after, Take: int32(take), MaxPageDepth: domain.MaxDepth, AgentsKey: shared.TitleKey(domain.AgentsTitle)})
		if err != nil {
			return out, err
		}
		rows = make([]gen.IngestHeadsFirstRow, len(got))
		for i, r := range got {
			rows[i] = gen.IngestHeadsFirstRow(r)
		}
	}
	if len(rows) > take {
		return out, errors.New("ingest metadata: raw take exceeded")
	}
	for _, r := range rows {
		if out.Last != nil && out.Last.Compare(r.ID) >= 0 || after != nil && after.Compare(r.ID) >= 0 {
			return app.IngestHeadBatch{}, errors.New("ingest metadata: raw rows do not advance")
		}
		h, err := ingestMetadataHead(r.ID, true, r.Live, r.Kind, r.HasContent, r.Revision, r.PageBytes,
			r.IsSourceRoot, r.InSource, r.IsRootAgents, r.LineOk)
		if err != nil {
			return app.IngestHeadBatch{}, err
		}
		out.Rows = append(out.Rows, h)
		id := r.ID
		out.Last = &id
	}
	out.Done = len(rows) < take
	return out, nil
}
