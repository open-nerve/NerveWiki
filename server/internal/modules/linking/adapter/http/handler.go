// Package httpadapter serves the linking module's API, the link index's
// reads (M6/P5) and a link's landing (M6/P6): it implements the strict server that oapi-codegen
// generates from api/modules/linking.yaml into the gen package, and
// translates between the generated types and the use cases.
package httpadapter

import (
	"context"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// ListBacklinksUseCase is app.ListBacklinks.
type ListBacklinksUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, limit *int, cursor *string) (app.Backlinks, error)
}

// GetPagePropertiesUseCase is app.GetPageProperties.
type GetPagePropertiesUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (app.Properties, error)
}

// ListTagsUseCase is app.ListTags.
type ListTagsUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID) ([]app.Tag, error)
}

// GetTagUseCase is app.GetTag.
type GetTagUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, name string) ([]uuid.UUID, error)
}

// ListLinkTargetsUseCase is app.ListLinkTargets.
type ListLinkTargetsUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID) ([]app.LinkTarget, error)
}

// GetLinkLandingUseCase is app.GetLinkLanding.
type GetLinkLandingUseCase interface {
	Execute(ctx context.Context, id uuid.UUID, target *string) (domain.Landing, error)
}

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	ListBacklinks     ListBacklinksUseCase
	GetPageProperties GetPagePropertiesUseCase
	ListTags          ListTagsUseCase
	GetTag            GetTagUseCase
	ListLinkTargets   ListLinkTargetsUseCase
	GetLinkLanding    GetLinkLandingUseCase
}

// Register mounts the module's routes on router, the root router from
// httpserver.NewRouter, behind the platform's per-route middlewares.
// Binding, decoding and handler errors are answered as problem+json by
// api.Errors.
func Register(router *httpserver.Router, api *httpserver.API, uc UseCases) {
	strict := gen.NewStrictHandlerWithOptions(handler{uc: uc}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  api.Errors.BodyError,
		ResponseErrorHandlerFunc: api.Errors.Write,
	})
	var middlewares []gen.MiddlewareFunc
	for _, m := range api.Middlewares(gen.BodyShapes()) {
		middlewares = append(middlewares, m)
	}
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseRouter:       router,
		Middlewares:      middlewares,
		ErrorHandlerFunc: api.Errors.BadRequest,
	})
}

// handler implements gen.StrictServerInterface: it only translates between
// the generated types and the use cases.
type handler struct {
	uc UseCases
}

// ListBacklinks serves GET /api/v0/pages/{page_id}/backlinks.
func (h handler) ListBacklinks(ctx context.Context, req gen.ListBacklinksRequestObject) (gen.ListBacklinksResponseObject, error) {
	page, err := h.uc.ListBacklinks.Execute(ctx, req.PageID, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	out := gen.ListBacklinks200JSONResponse{Data: make([]gen.Backlink, len(page.Pages)), NextCursor: nullable.NewNullNullable[string]()}
	if page.NextCursor != "" {
		out.NextCursor = nullable.NewNullableWithValue(page.NextCursor)
	}
	for i, b := range page.Pages {
		out.Data[i] = gen.Backlink{ID: b.PageID, Count: b.Links, Contexts: b.Contexts}
	}
	return out, nil
}

// GetPageProperties serves GET /api/v0/pages/{page_id}/properties. A
// property's value is its JSON as the index has it, written as it is.
func (h handler) GetPageProperties(ctx context.Context, req gen.GetPagePropertiesRequestObject) (gen.GetPagePropertiesResponseObject, error) {
	p, err := h.uc.GetPageProperties.Execute(ctx, req.PageID)
	if err != nil {
		return nil, err
	}
	out := gen.GetPageProperties200JSONResponse{
		Valid: p.Valid, Properties: make([]gen.PageProperty, len(p.Properties)), Links: make([]gen.PropertyLink, len(p.Links)),
	}
	for i, prop := range p.Properties {
		out.Properties[i] = gen.PageProperty{Key: prop.Key, Value: prop.Value}
	}
	for i, l := range p.Links {
		out.Links[i] = gen.PropertyLink{
			Key: l.Key, NodeID: nullable.NewNullNullable[uuid.UUID](), Kind: nullable.NewNullNullable[gen.LinkTargetKind](),
			URL: nullable.NewNullNullable[string](),
		}
		if l.NodeID != (uuid.UUID{}) {
			out.Links[i].NodeID = nullable.NewNullableWithValue(l.NodeID)
			out.Links[i].Kind = nullable.NewNullableWithValue(kind(l.Asset))
		}
		if l.URL != "" {
			out.Links[i].URL = nullable.NewNullableWithValue(l.URL)
		}
	}
	return out, nil
}

// ListTags serves GET /api/v0/notebooks/{notebook_id}/tags.
func (h handler) ListTags(ctx context.Context, req gen.ListTagsRequestObject) (gen.ListTagsResponseObject, error) {
	tags, err := h.uc.ListTags.Execute(ctx, req.NotebookID)
	if err != nil {
		return nil, err
	}
	out := gen.ListTags200JSONResponse{Data: make([]gen.TagCount, len(tags))}
	for i, t := range tags {
		out.Data[i] = gen.TagCount{Tag: t.Tag, Count: t.Pages}
	}
	return out, nil
}

// GetTag serves GET /api/v0/notebooks/{notebook_id}/tags/{tag}.
func (h handler) GetTag(ctx context.Context, req gen.GetTagRequestObject) (gen.GetTagResponseObject, error) {
	ids, err := h.uc.GetTag.Execute(ctx, req.NotebookID, req.Tag)
	if err != nil {
		return nil, err
	}
	out := gen.GetTag200JSONResponse{Data: make([]gen.TagPage, len(ids))}
	for i, id := range ids {
		out.Data[i] = gen.TagPage{ID: id}
	}
	return out, nil
}

// ListLinkTargets serves GET /api/v0/notebooks/{notebook_id}/link-targets.
func (h handler) ListLinkTargets(ctx context.Context, req gen.ListLinkTargetsRequestObject) (gen.ListLinkTargetsResponseObject, error) {
	targets, err := h.uc.ListLinkTargets.Execute(ctx, req.NotebookID)
	if err != nil {
		return nil, err
	}
	out := gen.ListLinkTargets200JSONResponse{Data: make([]gen.LinkTarget, len(targets))}
	for i, t := range targets {
		out.Data[i] = gen.LinkTarget{ID: t.ID, Kind: kind(t.Asset), Name: t.Name, Link: t.Link, Aliases: t.Aliases}
	}
	return out, nil
}

// kind is a link target's kind: an attachment's if asset, else a page's.
func kind(asset bool) gen.LinkTargetKind {
	if asset {
		return gen.LinkTargetKindAsset
	}
	return gen.LinkTargetKindPage
}

// GetLinkLanding serves GET /api/v0/pages/{page_id}/link-landing: one of
// its three fields is set, the others null.
func (h handler) GetLinkLanding(ctx context.Context, req gen.GetLinkLandingRequestObject) (gen.GetLinkLandingResponseObject, error) {
	l, err := h.uc.GetLinkLanding.Execute(ctx, req.PageID, req.Params.Target)
	if err != nil {
		return nil, err
	}
	out := gen.GetLinkLanding200JSONResponse{
		NodeID: nullable.NewNullNullable[uuid.UUID](), Landing: nullable.NewNullNullable[gen.Landing](), Reason: nullable.NewNullNullable[gen.LandingReason](),
	}
	switch {
	case l.Node != (uuid.UUID{}):
		out.NodeID = nullable.NewNullableWithValue(l.Node)
	case l.Reason != "":
		out.Reason = nullable.NewNullableWithValue(gen.LandingReason(l.Reason))
	default:
		landing := gen.Landing{ParentID: nullable.NewNullNullable[uuid.UUID](), Title: l.Title}
		if l.Parent != (uuid.UUID{}) {
			landing.ParentID = nullable.NewNullableWithValue(l.Parent)
		}
		out.Landing = nullable.NewNullableWithValue(landing)
	}
	return out, nil
}
