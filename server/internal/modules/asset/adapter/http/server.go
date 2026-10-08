package httpadapter

import (
	"context"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http/gen"
)

// server implements gen.StrictServerInterface: it only translates between
// the generated types and the use cases.
type server struct {
	uc UseCases
}

// GetAsset serves GET /api/v0/assets/{node_id}.
func (s server) GetAsset(ctx context.Context, req gen.GetAssetRequestObject) (gen.GetAssetResponseObject, error) {
	a, err := s.uc.Reads.Get(ctx, req.NodeID)
	if err != nil {
		return nil, err
	}
	return gen.GetAsset200JSONResponse(assetOf(a)), nil
}

// ListAssets serves GET /api/v0/notebooks/{notebook_id}/assets.
func (s server) ListAssets(ctx context.Context, req gen.ListAssetsRequestObject) (gen.ListAssetsResponseObject, error) {
	page, err := s.uc.Reads.List(ctx, req.NotebookID, req.Params.ParentID, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	out := gen.ListAssets200JSONResponse{Data: make([]gen.Asset, len(page.Assets)), NextCursor: nullable.NewNullNullable[string]()}
	if page.NextCursor != "" {
		out.NextCursor = nullable.NewNullableWithValue(page.NextCursor)
	}
	for i, a := range page.Assets {
		out.Data[i] = assetOf(a)
	}
	return out, nil
}
