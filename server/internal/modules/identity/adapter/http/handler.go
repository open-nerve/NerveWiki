// Package httpadapter serves the identity module's API: it implements the
// strict server that oapi-codegen generates from api/modules/identity.yaml
// into the gen package, translates between the generated types and the use
// cases, and applies the module's own rate limits.
package httpadapter

import (
	"context"
	"log/slog"
	"net/netip"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// RegisterUseCase is app.Register.
type RegisterUseCase interface {
	Execute(ctx context.Context, in app.RegisterInput) (app.Tokens, error)
}

// LoginUseCase is app.Login.
type LoginUseCase interface {
	Execute(ctx context.Context, in app.LoginInput) (app.Tokens, error)
}

// RefreshUseCase is app.Refresh.
type RefreshUseCase interface {
	Execute(ctx context.Context, token string, ip netip.Addr) (app.Tokens, error)
}

// LogoutUseCase is app.Logout.
type LogoutUseCase interface {
	Execute(ctx context.Context, token string) error
}

// GetMeUseCase is app.GetMe.
type GetMeUseCase interface {
	Execute(ctx context.Context) (domain.User, error)
}

// ListAPITokensUseCase is app.ListAPITokens.
type ListAPITokensUseCase interface {
	Execute(ctx context.Context) ([]domain.APIToken, error)
}

// CreateAPITokenUseCase is app.CreateAPIToken.
type CreateAPITokenUseCase interface {
	Execute(ctx context.Context, in app.CreateAPITokenInput) (app.CreatedAPIToken, error)
}

// RevokeAPITokenUseCase is app.RevokeAPIToken.
type RevokeAPITokenUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) error
}

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	Register       RegisterUseCase
	Login          LoginUseCase
	Refresh        RefreshUseCase
	Logout         LogoutUseCase
	GetMe          GetMeUseCase
	ListAPITokens  ListAPITokensUseCase
	CreateAPIToken CreateAPITokenUseCase
	RevokeAPIToken RevokeAPITokenUseCase
}

// Settings are what the handler applies around the use cases.
type Settings struct {
	Limits Limits
	Logger *slog.Logger
}

// PublicOperations are the module's routes that need no token (M1/P1
// design 3.5), as the generated code registers them.
func PublicOperations() []string {
	return []string{
		"POST /api/v0/auth/register",
		"POST /api/v0/auth/login",
		"POST /api/v0/auth/refresh",
		"POST /api/v0/auth/logout",
	}
}

// RequestTimeouts are the module's routes that answer sooner than the
// request timeout (M1/P2 design 3.5): refresh and logout, within
// refreshDeadline (auth.refresh_deadline). It is part of the rotation
// protocol: the server answers before the web client gives up.
func RequestTimeouts(refreshDeadline time.Duration) map[string]time.Duration {
	return map[string]time.Duration{
		"POST /api/v0/auth/refresh": refreshDeadline,
		"POST /api/v0/auth/logout":  refreshDeadline,
	}
}

// Register mounts the module's routes on router behind api's per-route
// middlewares; api.Errors answers binding, decoding and handler errors.
func Register(router *httpserver.Router, api *httpserver.API, uc UseCases, s Settings) {
	strict := gen.NewStrictHandlerWithOptions(handler{uc: uc, s: s}, nil, gen.StrictHTTPServerOptions{
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

// handler implements gen.StrictServerInterface.
type handler struct {
	uc UseCases
	s  Settings
}
