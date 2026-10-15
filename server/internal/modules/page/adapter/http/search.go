package httpadapter

import (
	"context"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// SearchUseCase is app.Search.
type SearchUseCase interface {
	Execute(ctx context.Context, slug string, r app.SearchRequest) (app.SearchPage, error)
}

// SearchWorkspace serves GET /api/v0/workspaces/{slug}/search.
func (h handler) SearchWorkspace(ctx context.Context, req gen.SearchWorkspaceRequestObject) (gen.SearchWorkspaceResponseObject, error) {
	p, err := h.uc.Search.Execute(ctx, req.Slug, app.SearchRequest{Q: req.Params.Q, NotebookID: req.Params.NotebookID,
		Cursor: req.Params.Cursor, Limit: req.Params.Limit})
	if err != nil {
		return nil, err
	}
	out := gen.SearchWorkspace200JSONResponse{Data: make([]gen.SearchHit, len(p.Hits)), NextCursor: nullable.NewNullNullable[string](),
		Count: nullable.NewNullNullable[gen.SearchCount](), Building: p.Building}
	for i, hit := range p.Hits {
		out.Data[i] = searchHitOf(hit)
	}
	if p.NextCursor != "" {
		out.NextCursor = nullable.NewNullableWithValue(p.NextCursor)
	}
	if p.Count != nil {
		out.Count = nullable.NewNullableWithValue(gen.SearchCount{Value: p.Count.Value, Kind: gen.SearchCountKind(p.Count.Kind)})
	}
	return out, nil
}

func searchHitOf(h app.SearchHit) gen.SearchHit {
	out := gen.SearchHit{
		Node:       gen.SearchNode{ID: h.Node.ID, NotebookID: h.Node.NotebookID, Kind: gen.NodeKind(h.Node.Kind), Name: h.Node.Name},
		Ancestors:  make([]gen.Ancestor, len(h.Ancestors)),
		TitleHits:  rangesOf(h.TitleHits),
		ChangedAt:  h.ChangedAt,
		Excerpt:    nullable.NewNullNullable[gen.SearchExcerpt](),
		Attachment: nullable.NewNullNullable[gen.AttachmentInfo](),
	}
	for i, a := range h.Ancestors {
		out.Ancestors[i] = gen.Ancestor{ID: a.ID, Name: a.Name}
	}
	if e := h.Excerpt; e != nil {
		out.Excerpt = nullable.NewNullableWithValue(gen.SearchExcerpt{Text: e.Text, Hits: rangesOf(e.Hits), CutStart: e.CutStart,
			CutEnd: e.CutEnd})
	}
	if a := h.Attachment; a != nil {
		out.Attachment = nullable.NewNullableWithValue(gen.AttachmentInfo{Mime: a.MIME, ByteSize: a.Bytes})
	}
	return out
}

func rangesOf(rs []domain.TextRange) []gen.TextRange {
	out := make([]gen.TextRange, len(rs))
	for i, r := range rs {
		out[i] = gen.TextRange{Start: r.Start, End: r.End}
	}
	return out
}
