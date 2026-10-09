// Package httpadapter serves the transfer module's API: the strict server
// that oapi-codegen generates from api/modules/transfer.yaml into the gen
// package, and the module's own handler of its raw operation (x-raw), the
// download of an export's archive, which streams its bytes.
package httpadapter

import (
	"context"
	"net/http"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	Start    *app.StartExport
	Reads    *app.Reads
	Cancel   *app.Cancel
	Download *app.Download
}

// downloadRoute is the raw operation's route pattern.
const downloadRoute = "GET /api/v0/transfer-jobs/{job_id}/download"

// PublicOperations are the module's routes that need no token: the
// download, whose signed address is the grant.
func PublicOperations() []string {
	return []string{downloadRoute}
}

// Register mounts the module's routes on router, the root router from
// httpserver.NewRouter: the generated ones behind api's per-route
// middlewares; the download through api.Stream, at the lowest rate
// minRate (asset.upload_min_rate), under the platform's buckets, every
// answer sandboxed. The raw route binds its path's id first.
func Register(router *httpserver.Router, api *httpserver.API, uc UseCases, minRate int64) {
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
	router.Handle(downloadRoute, sandboxed(bindID(api.Errors, api.Stream(
		download{uc: uc.Download, errors: api.Errors}, httpserver.StreamPolicy{MinRate: minRate}))))
}

type pathIDKey struct{}

// bindID binds the route's job_id before next, as the generated wrappers
// bind their parameters before the middlewares (M1/P1 design 3.9): a value
// that is no id is 400 before anything else.
func bindID(errs httpserver.APIErrors, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("job_id"))
		if err != nil {
			errs.BadRequest(w, r, &gen.InvalidParamFormatError{ParamName: "job_id", Err: err})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pathIDKey{}, id)))
	})
}

// pathID is the id bindID bound. A handler mounted without it is a wiring
// fault: pathID panics.
func pathID(r *http.Request) uuid.UUID {
	id, ok := r.Context().Value(pathIDKey{}).(uuid.UUID)
	if !ok {
		panic("httpadapter: a route without bindID")
	}
	return id
}
