// Package httpadapter serves the asset module's API: the strict server
// that oapi-codegen generates from api/modules/asset.yaml into the gen
// package, and the module's own handlers of its raw operations (x-raw),
// the upload and the download, which stream their bytes.
package httpadapter

import (
	"context"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// UseCases are the use cases behind the module's operations.
type UseCases struct {
	Upload  *app.Upload
	Reads   *app.Reads
	Content *app.Content
}

// Limits are the stream routes' bounds: asset.max_bytes, the largest
// file, and asset.upload_min_rate, the slowest rate in bytes a second,
// both ways; and the downloads' bucket, ratelimit.asset_content.
type Limits struct {
	MaxBytes      int64
	MinRate       int64
	ContentBucket httpserver.Limiter
}

// The raw operations' route patterns.
const (
	uploadRoute  = "POST /api/v0/notebooks/{notebook_id}/assets"
	contentRoute = "GET /api/v0/assets/{node_id}/content"
)

// PublicOperations are the module's routes that need no token: the
// download, whose signed address is the grant.
func PublicOperations() []string {
	return []string{contentRoute}
}

// Register mounts the module's routes on router, the root router from
// httpserver.NewRouter: the generated ones behind api's per-route
// middlewares; the upload through api.Stream, its body the largest file
// and the parts around it, at the lowest rate, under the platform's
// buckets; the download through api.Stream too, at the lowest rate, under
// its own bucket. The raw routes bind their path's id first.
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
	router.Handle(uploadRoute, bindID("notebook_id", api.Errors, api.Stream(
		upload{uc: uc.Upload, errors: api.Errors, logger: logger, maxBytes: limits.MaxBytes},
		httpserver.StreamPolicy{MaxBytes: limits.MaxBytes + Envelope, MinRate: limits.MinRate})))
	router.Handle(contentRoute, bindID("node_id", api.Errors, api.Stream(
		content{uc: uc.Content, errors: api.Errors},
		httpserver.StreamPolicy{MinRate: limits.MinRate, Bucket: limits.ContentBucket, BucketName: "asset_content"})))
}

type pathIDKey struct{ param string }

// bindID binds the route's id param before next, as the generated
// wrappers bind their parameters before the middlewares (M1/P1 design
// 3.9): a value that is no id is 400 before authentication, the body
// unread, the connection closed after (early).
func bindID(param string, errs httpserver.APIErrors, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue(param))
		if err != nil {
			w.Header().Set("Connection", "close")
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

// bounded runs step, a step of a stream that is not its bytes, within the
// request's timeout (httpserver.Bounded).
func bounded(r *http.Request, step func(r *http.Request) error) error {
	ctx, cancel := httpserver.Bounded(r.Context())
	defer cancel()
	return step(r.WithContext(ctx))
}
