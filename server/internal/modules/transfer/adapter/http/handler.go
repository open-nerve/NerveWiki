// Package httpadapter serves the transfer module's API: the strict server
// that oapi-codegen generates from api/modules/transfer.yaml into the gen
// package, and the module's own handlers of its raw operations (x-raw),
// the start of an import, which streams its archive in, and the download
// of an export's archive, which streams its bytes out.
package httpadapter

import (
	"context"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	Start    *app.StartExport
	Import   *app.StartImport
	Reads    *app.Reads
	Cancel   *app.Cancel
	Download *app.Download
}

// Limits are the raw routes' bounds: transfer.import_max_bytes, the
// largest archive an import sends, and asset.upload_min_rate, the slowest
// rate in bytes a second, both ways.
type Limits struct {
	ImportMaxBytes int64
	MinRate        int64
}

// The raw operations' route patterns.
const (
	importRoute   = "POST /api/v0/notebooks/{notebook_id}/imports"
	downloadRoute = "GET /api/v0/transfer-jobs/{job_id}/download"
)

// PublicOperations are the module's routes that need no token: the
// download, whose signed address is the grant.
func PublicOperations() []string {
	return []string{downloadRoute}
}

// Register mounts the module's routes on router, the root router from
// httpserver.NewRouter: the generated ones behind api's per-route
// middlewares; the import's start through api.Stream, its body the
// largest archive and the parts around it, at the lowest rate, under the
// platform's buckets; the download through api.Stream too, at the lowest
// rate, under the platform's buckets, every answer sandboxed. The raw
// routes bind their path's id first.
func Register(router *httpserver.Router, api *httpserver.API, uc UseCases, limits Limits, logger *slog.Logger) {
	strict := gen.NewStrictHandlerWithOptions(server{uc: uc}, nil, gen.StrictHTTPServerOptions{
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
	router.Handle(importRoute, bindID("notebook_id", api.Errors, api.Stream(
		startImport{uc: uc.Import, errors: api.Errors, logger: logger, maxBytes: limits.ImportMaxBytes}, importPolicy(limits))))
	router.Handle(downloadRoute, sandboxed(bindID("job_id", api.Errors, api.Stream(
		download{uc: uc.Download, errors: api.Errors}, httpserver.StreamPolicy{MinRate: limits.MinRate}))))
}

// importPolicy bounds the import's body by the largest archive and the
// Envelope, at the lowest rate, under the platform's buckets.
func importPolicy(limits Limits) httpserver.StreamPolicy {
	return httpserver.StreamPolicy{MaxBytes: limits.ImportMaxBytes + Envelope, MinRate: limits.MinRate}
}

type pathIDKey struct{ param string }

// bindID binds the route's id param before next, as the generated
// wrappers bind their parameters before the middlewares (M1/P1 design
// 3.9): a value that is no id is 400 before anything else, the connection
// closed after when a body is left unread (early).
func bindID(param string, errs httpserver.APIErrors, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue(param))
		if err != nil {
			if r.Body != nil && r.Body != http.NoBody {
				w.Header().Set("Connection", "close")
			}
			errs.BadRequest(w, r, &gen.InvalidParamFormatError{ParamName: param, Err: err})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pathIDKey{param}, id)))
	})
}

// pathID is the id bindID bound of param. A handler mounted without it is
// a wiring fault: pathID panics.
func pathID(r *http.Request, param string) uuid.UUID {
	id, ok := r.Context().Value(pathIDKey{param}).(uuid.UUID)
	if !ok {
		panic("httpadapter: a route without bindID(" + param + ")")
	}
	return id
}
