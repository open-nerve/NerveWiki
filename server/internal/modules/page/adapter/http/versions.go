package httpadapter

import (
	"context"
	"time"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// ListVersionsUseCase is app.ListVersions.
type ListVersionsUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, limit *int, cursor *string) (app.VersionList, error)
}

// GetVersionUseCase is app.GetVersion.
type GetVersionUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, revision int) (app.VersionContent, error)
}

// GetVersionViewUseCase is app.GetVersionView.
type GetVersionViewUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, revision int) (app.ReadingView, error)
}

// RestoreVersionUseCase is app.RestoreVersion.
type RestoreVersionUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, revision, base int, client domain.Client) (app.VersionRestored, error)
}

// ListPageVersions serves GET /api/v0/pages/{page_id}/versions.
func (h handler) ListPageVersions(ctx context.Context, req gen.ListPageVersionsRequestObject) (gen.ListPageVersionsResponseObject, error) {
	l, err := h.uc.ListVersions.Execute(ctx, req.PageID, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	out := gen.ListPageVersions200JSONResponse{Data: make([]gen.PageVersion, len(l.Versions)), NextCursor: nullable.NewNullNullable[string]()}
	for i, v := range l.Versions {
		out.Data[i] = pageVersionOf(v)
	}
	if l.NextCursor != "" {
		out.NextCursor = nullable.NewNullableWithValue(l.NextCursor)
	}
	return out, nil
}

// GetPageVersion serves GET /api/v0/pages/{page_id}/versions/{revision}.
func (h handler) GetPageVersion(ctx context.Context, req gen.GetPageVersionRequestObject) (gen.GetPageVersionResponseObject, error) {
	c, err := h.uc.GetVersion.Execute(ctx, req.PageID, req.Revision)
	if err != nil {
		return nil, err
	}
	v := pageVersionOf(c.Version)
	return gen.GetPageVersion200JSONResponse{
		ID: v.ID, Revision: v.Revision, BaseRevision: v.BaseRevision, ByteSize: v.ByteSize, CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt, Editing: v.Editing, Changeset: v.Changeset, Content: c.Content,
	}, nil
}

// GetPageVersionView serves GET /api/v0/pages/{page_id}/versions/{revision}/view.
func (h handler) GetPageVersionView(ctx context.Context, req gen.GetPageVersionViewRequestObject) (gen.GetPageVersionViewResponseObject, error) {
	v, err := h.uc.GetVersionView.Execute(ctx, req.PageID, req.Revision)
	if err != nil {
		return nil, err
	}
	return gen.GetPageVersionView200JSONResponse(pageViewOf(v)), nil
}

// RestorePageVersion serves POST /api/v0/pages/{page_id}/versions/{revision}/restore.
func (h handler) RestorePageVersion(ctx context.Context, req gen.RestorePageVersionRequestObject) (gen.RestorePageVersionResponseObject, error) {
	client, err := clientOf(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.uc.RestoreVersion.Execute(ctx, req.PageID, req.Revision, req.Body.BaseRevision, client)
	if err != nil {
		return nil, err
	}
	out := gen.RestorePageVersion200JSONResponse{Revision: r.Revision, ChangesetID: nullable.NewNullNullable[uuid.UUID]()}
	if r.ChangesetID != (uuid.UUID{}) {
		out.ChangesetID, out.Changed = nullable.NewNullableWithValue(r.ChangesetID), true
	}
	return out, nil
}

func pageVersionOf(v domain.Version) gen.PageVersion {
	base := nullable.NewNullNullable[int]()
	if v.Base != nil {
		base = nullable.NewNullableWithValue(*v.Base)
	}
	c := v.Changeset
	return gen.PageVersion{
		ID: v.ID, Revision: v.Revision, BaseRevision: base, ByteSize: v.ByteSize, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
		Editing: v.Editing, Changeset: gen.VersionChangeset{
			ID: c.ID, Kind: gen.ChangesetKind(c.Kind), Client: string(c.Client), CreatedAt: c.CreatedAt,
			CreatedBy: gen.ChangesetAuthor{UserID: c.By, DisplayName: c.ByName},
		},
	}
}

// pageViewOf is a reading view's answer: assets_expire_at null for a view
// with no attachment's address.
func pageViewOf(v app.ReadingView) gen.PageView {
	expires := nullable.NewNullNullable[time.Time]()
	if !v.Expires.IsZero() {
		expires = nullable.NewNullableWithValue(v.Expires)
	}
	return gen.PageView{HTML: v.HTML, Revision: v.Revision, AssetsExpireAt: expires}
}
