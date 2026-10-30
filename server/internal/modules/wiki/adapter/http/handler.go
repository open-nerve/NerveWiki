// Package httpadapter serves the wiki's health checks through its existing runtime.
package httpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// LintUseCase is the runtime shared by the module's callers.
type LintUseCase interface {
	Execute(context.Context, app.LintScope, app.LintQuery) (domain.LintResult, error)
}

// Register mounts the generated strict routes behind the platform's middlewares.
func Register(router *httpserver.Router, api *httpserver.API, lint LintUseCase) {
	strict := gen.NewStrictHandlerWithOptions(handler{lint: lint}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: api.Errors.BodyError, ResponseErrorHandlerFunc: api.Errors.Write,
	})
	var middlewares []gen.MiddlewareFunc
	for _, middleware := range api.Middlewares(gen.BodyShapes()) {
		middlewares = append(middlewares, middleware)
	}
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseRouter: router, Middlewares: middlewares, ErrorHandlerFunc: api.Errors.BadRequest,
	})
}

type handler struct{ lint LintUseCase }

func (h handler) GetLint(ctx context.Context, req gen.GetLintRequestObject) (gen.GetLintResponseObject, error) {
	result, err := h.lint.Execute(ctx, app.LintScope{NotebookID: req.NotebookID}, app.LintQuery{})
	if err != nil {
		return nil, err
	}
	out := gen.GetLint200JSONResponse{
		Mode: gen.LintMode(result.Mode), ComputedAt: result.ComputedAt, StalePages: result.StalePages,
		Rules: make([]gen.LintRuleSummary, len(result.Rules)),
	}
	for i, rule := range result.Rules {
		out.Rules[i], err = lintSummary(rule)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h handler) ListLintItems(ctx context.Context, req gen.ListLintItemsRequestObject) (gen.ListLintItemsResponseObject, error) {
	result, err := h.lint.Execute(ctx, app.LintScope{NotebookID: req.NotebookID}, app.LintQuery{
		Rule: &req.Rule, Limit: req.Params.Limit, Cursor: req.Params.Cursor,
	})
	if err != nil {
		return nil, err
	}
	if len(result.Rules) != 1 {
		return nil, errLintProjection
	}
	rule := result.Rules[0]
	out := gen.ListLintItems200JSONResponse{
		Mode: gen.LintMode(result.Mode), ComputedAt: result.ComputedAt, StalePages: result.StalePages,
		Data: make([]gen.LintItem, len(rule.Items)), NextCursor: lintNextCursor(rule.NextCursor),
	}
	for i, item := range rule.Items {
		out.Data[i], err = lintItem(item)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
