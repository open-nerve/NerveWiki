// Package httpadapter serves the identity module's API: it implements the
// strict server that oapi-codegen generates from api/modules/identity.yaml
// into the gen package, and translates between the generated types and the
// use cases.
package httpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// RegisterUseCase is app.Register.
type RegisterUseCase interface {
	Execute(ctx context.Context, in app.RegisterInput) (app.Tokens, error)
}

// GetMeUseCase is app.GetMe.
type GetMeUseCase interface {
	Execute(ctx context.Context) (domain.User, error)
}

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	Register RegisterUseCase
	GetMe    GetMeUseCase
}

// PublicOperations are the module's routes that need no token (M1/P1
// design 3.5), as the generated code registers them.
func PublicOperations() []string {
	return []string{"POST /api/v0/auth/register"}
}

// Register mounts the module's routes on router behind api's per-route
// middlewares; api.Errors answers binding, decoding and handler errors.
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
