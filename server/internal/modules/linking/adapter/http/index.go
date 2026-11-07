package httpadapter

import (
	"context"
	"slices"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// This alias is the generated nullable object's shape, including its JSON tags.
type indexSourceArea = struct {
	RootID  uuid.UUID `json:"root_id"`
	Sources int64     `json:"sources"`
}

// ListIndex translates the directory's REST-only options and required fields.
// MCP invokes the same use case with Catalogue and ExcludeArea false.
func (h handler) ListIndex(ctx context.Context, req gen.ListIndexRequestObject) (gen.ListIndexResponseObject, error) {
	q := app.IndexQuery{Type: req.Params.Type, Tag: req.Params.Tag, ParentID: req.Params.ParentID,
		AtRoot: req.Params.AtRoot != nil && *req.Params.AtRoot, Limit: req.Params.Limit, Cursor: req.Params.Cursor, Catalogue: true, ExcludeArea: true}
	page, err := h.uc.ListIndex.Execute(ctx, req.NotebookID, q)
	if err != nil {
		return nil, err
	}
	out := gen.ListIndex200JSONResponse{Data: make([]gen.NotebookIndexItem, len(page.Items)), NextCursor: nullable.NewNullNullable[string](),
		Types: make([]gen.NotebookIndexType, len(page.Types)), TypesComplete: page.TypesComplete, UnfilterableTypePages: page.UnfilterableTypePages,
		Mode: gen.NotebookIndexMode(page.Mode), SourceArea: nullable.NewNullNullable[indexSourceArea]()}
	if page.Next != "" {
		out.NextCursor = nullable.NewNullableWithValue(page.Next)
	}
	if page.SourceArea != nil {
		out.SourceArea = nullable.NewNullableWithValue(indexSourceArea{RootID: page.SourceArea.RootID, Sources: page.SourceArea.Sources})
	}
	for i, typ := range page.Types {
		out.Types[i] = gen.NotebookIndexType{Type: typ.Type, Count: typ.Count}
	}
	for i, item := range page.Items {
		row := gen.NotebookIndexItem{PageID: item.PageID, ParentID: nullable.NewNullNullable[uuid.UUID](), Title: item.Title,
			Type: nullable.NewNullNullable[string](), TypeKey: nullable.NewNullNullable[string](), Summary: nullable.NewNullNullable[string](), SummaryCut: slices.Contains(item.Cut, "summary"),
			Tags: append([]string{}, item.Tags...), UpdatedAt: item.ChangedAt, Inbound: item.Inbound, InboundCapped: item.Inbound == domain.MaxInbound}
		if item.ParentID != nil {
			row.ParentID = nullable.NewNullableWithValue(*item.ParentID)
		}
		if item.TypeKey != nil {
			row.TypeKey = nullable.NewNullableWithValue(*item.TypeKey)
		}
		if item.Type != nil {
			row.Type = nullable.NewNullableWithValue(*item.Type)
		}
		if item.Summary != nil {
			row.Summary = nullable.NewNullableWithValue(*item.Summary)
		}
		out.Data[i] = row
	}
	return out, nil
}
