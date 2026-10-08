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
	Upload *app.Upload
}

// Limits are the stream routes' bounds: asset.max_bytes, the largest
// file, and asset.upload_min_rate, the slowest rate in bytes a second.
type Limits struct {
	MaxBytes int64
	MinRate  int64
}

// uploadRoute is the upload's route pattern.
const uploadRoute = "POST /api/v0/notebooks/{notebook_id}/assets"

// Register mounts the module's routes on router, the root router from
// httpserver.NewRouter: the upload through api.Stream, its body the
// largest file and the parts around it, at the lowest rate, under the
// platform's buckets, its notebook_id bound first.
func Register(router *httpserver.Router, api *httpserver.API, uc UseCases, limits Limits, logger *slog.Logger) {
	router.Handle(uploadRoute, bindNotebook(api.Errors, api.Stream(
		upload{uc: uc.Upload, errors: api.Errors, logger: logger, maxBytes: limits.MaxBytes},
		httpserver.StreamPolicy{MaxBytes: limits.MaxBytes + Envelope, MinRate: limits.MinRate})))
}

type notebookKey struct{}

// bindNotebook binds the route's notebook_id before next, as the generated
// wrappers bind their parameters before the middlewares (M1/P1 design
// 3.9): a value that is no id is 400 before authentication, the body
// unread, the connection closed after (early).
func bindNotebook(errs httpserver.APIErrors, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("notebook_id"))
		if err != nil {
			w.Header().Set("Connection", "close")
			errs.BadRequest(w, r, &gen.InvalidParamFormatError{ParamName: "notebook_id", Err: err})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), notebookKey{}, id)))
	})
}

// notebookOf is the notebook_id bindNotebook bound. A handler mounted
// without it is a wiring fault: notebookOf panics.
func notebookOf(r *http.Request) uuid.UUID {
	id, ok := r.Context().Value(notebookKey{}).(uuid.UUID)
	if !ok {
		panic("httpadapter: a route without bindNotebook")
	}
	return id
}
