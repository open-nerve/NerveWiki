// Package httpadapter serves the instance module's API: it implements the
// strict server that oapi-codegen generates from api/modules/instance.yaml
// into the gen package.
package httpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	GetInfo *app.GetInfo
}

// PublicOperations are the module's routes that need no token, as the
// generated code registers them.
func PublicOperations() []string {
	return []string{"GET /api/v0/instance"}
}

// Register mounts the module's routes on router, the root router from
// httpserver.NewRouter, behind the platform's per-route middlewares. They
// are more specific than the platform's /api/ fallback, which keeps
// answering every other API path with a 404 problem. Binding, decoding and
// handler errors are answered as problem+json by api.Errors.
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

// GetInstance serves GET /api/v0/instance.
func (h handler) GetInstance(context.Context, gen.GetInstanceRequestObject) (gen.GetInstanceResponseObject, error) {
	info := h.uc.GetInfo.Execute()
	return gen.GetInstance200JSONResponse{
		Product:                  info.Product,
		Version:                  info.Version,
		Commit:                   info.Commit,
		APIVersion:               gen.InstanceInfoAPIVersion(info.APIVersion),
		SignupEnabled:            info.SignupEnabled,
		WorkspaceCreationEnabled: info.WorkspaceCreationEnabled,
	}, nil
}
