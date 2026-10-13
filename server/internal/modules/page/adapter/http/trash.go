package httpadapter

import (
	"context"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apigen"
)

// ListTrashUseCase is app.ListTrash.
type ListTrashUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, limit *int, cursor *string) (app.TrashList, error)
}

// GetTrashItemUseCase is app.GetTrashItem.
type GetTrashItemUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (app.TrashItem, error)
}

// RestoreNodeUseCase is app.RestoreNode.
type RestoreNodeUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, client domain.Client) (app.NodeRestored, error)
}

// ListTrash serves GET /api/v0/notebooks/{notebook_id}/trash.
func (h handler) ListTrash(ctx context.Context, req gen.ListTrashRequestObject) (gen.ListTrashResponseObject, error) {
	l, err := h.uc.ListTrash.Execute(ctx, req.NotebookID, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	out := gen.ListTrash200JSONResponse{Data: make([]gen.TrashItem, len(l.Items)), NextCursor: nullable.NewNullNullable[string]()}
	for i, it := range l.Items {
		out.Data[i] = trashItemOf(it)
	}
	if l.NextCursor != "" {
		out.NextCursor = nullable.NewNullableWithValue(l.NextCursor)
	}
	return out, nil
}

// GetTrashItem serves GET /api/v0/nodes/{node_id}/trash-item.
func (h handler) GetTrashItem(ctx context.Context, req gen.GetTrashItemRequestObject) (gen.GetTrashItemResponseObject, error) {
	it, err := h.uc.GetTrashItem.Execute(ctx, req.NodeID)
	if err != nil {
		return nil, err
	}
	return gen.GetTrashItem200JSONResponse(trashItemOf(it)), nil
}

// RestoreNode serves POST /api/v0/nodes/{node_id}/restore.
func (h handler) RestoreNode(ctx context.Context, req gen.RestoreNodeRequestObject) (gen.RestoreNodeResponseObject, error) {
	client, err := clientOf(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.uc.RestoreNode.Execute(ctx, req.NodeID, client)
	if err != nil {
		return nil, err
	}
	return gen.RestoreNode200JSONResponse{
		ID: r.Node.ID, Kind: gen.NodeKind(r.Node.Kind), Name: r.Node.Name, Renamed: r.Renamed, Placed: placementOf(r.Placement),
		ChangesetID: r.ChangesetID,
	}, nil
}

func trashItemOf(it app.TrashItem) gen.TrashItem {
	root := it.Root
	ancestors := make([]gen.Ancestor, len(it.Ancestors))
	for i, a := range it.Ancestors {
		ancestors[i] = gen.Ancestor{ID: a.ID, Name: a.Name}
	}
	out := gen.TrashItem{
		ID: root.Node.ID, NotebookID: root.Node.NotebookID, Kind: gen.NodeKind(root.Node.Kind), Name: root.Node.Name,
		Ancestors: ancestors, DeletedAt: root.DeletedAt, RestorableUntil: it.RestorableUntil, Pages: root.Pages, Assets: root.Assets,
		DeletedBy: apigen.UserRef{UserID: root.Node.UpdatedBy, DisplayName: it.DeletedByName}, RestoreTo: placementOf(it.Placement),
		Revision: nullable.NewNullNullable[int](), Asset: nullable.NewNullNullable[gen.TrashAttachment](),
	}
	if root.Node.Kind == domain.KindPage && root.Revision > 0 {
		out.Revision = nullable.NewNullableWithValue(root.Revision)
	}
	if it.Asset != nil {
		out.Asset = nullable.NewNullableWithValue(gen.TrashAttachment{Mime: it.Asset.MIME, ByteSize: it.Asset.Bytes})
	}
	return out
}

func placementOf(p domain.Placement) gen.TrashPlacement {
	out := gen.TrashPlacement{ParentID: nullable.NewNullNullable[uuid.UUID](), Reason: gen.TrashPlacementReason(p.Reason)}
	if p.ParentID != nil {
		out.ParentID = nullable.NewNullableWithValue(*p.ParentID)
	}
	return out
}
