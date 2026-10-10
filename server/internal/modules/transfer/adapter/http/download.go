package httpadapter

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The download's handler (M7/P5 design 3.11): it reads the address's query
// strictly, opens the archive, then sets its headers and serves its bytes,
// ranges answered.

// archivePolicy is every answer's Content-Security-Policy: a zip file a
// browser opens runs nothing.
const archivePolicy = "sandbox; default-src 'none'"

// busyRetry is the Retry-After of a download asked for as the server shuts
// down: the time a restart takes.
const busyRetry = 5 * time.Second

type download struct {
	uc     *app.Download
	errors httpserver.APIErrors
}

func (h download) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a, ok := addressOf(pathID(r, "job_id"), r.PathValue("job_id"), r.URL.RawQuery)
	if !ok || r.URL.RawPath != "" {
		// RawPath is set when the path was sent escaped: the router has
		// unescaped what addressOf reads.
		h.errors.Write(w, r, domain.ErrDownloadNotFound)
		return
	}
	var o app.Opened
	br, err := bounded(r, func(r *http.Request) error {
		var err error
		o, err = h.uc.Open(r.Context(), a)
		return err
	})
	if err != nil {
		h.errors.Write(w, br, err)
		return
	}
	defer func() { _ = o.File.Close() }()
	if err := httpserver.Sending(r); err != nil {
		if errors.Is(err, httpserver.ErrShuttingDown) {
			err = shared.ServerBusy(busyRetry)
		}
		h.errors.Write(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", httpserver.Disposition(false, o.Name))
	httpserver.ServeFixed(w, r, o.File.ModTime(), o.File)
}

// sandboxed sets the archive's policy on every answer of the download, a
// refusal too.
func sandboxed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", archivePolicy)
		next.ServeHTTP(w, r)
	})
}

// addressOf reads an address as the server writes it, the job's id
// spelled path: the id in lower case with hyphens; the query e and s, in
// that order, e a decimal without sign or leading zeros, s 22 base64url
// characters. Nothing is unescaped: an escaped spelling is another
// address. Anything else is no address.
func addressOf(id uuid.UUID, path, query string) (app.Address, bool) {
	a := app.Address{JobID: id}
	if a.JobID.String() != path {
		return app.Address{}, false
	}
	e, rest, ok := strings.Cut(query, "&")
	e, eok := strings.CutPrefix(e, "e=")
	s, sok := strings.CutPrefix(rest, "s=")
	if !ok || !eok || !sok {
		return app.Address{}, false
	}
	expires, err := strconv.ParseInt(e, 10, 64)
	if err != nil || expires <= 0 || strconv.FormatInt(expires, 10) != e || !signature(s) {
		return app.Address{}, false
	}
	a.Expires, a.Signature = expires, s
	return a, true
}

// signature reports whether s is spelled as a signature: 22 base64url
// characters.
func signature(s string) bool {
	if len(s) != 22 {
		return false
	}
	for _, c := range []byte(s) {
		alnum := 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9'
		if !alnum && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// bounded runs step, a step of a stream that is not its bytes, within the
// request's timeout (httpserver.Bounded). It answers the request step ran
// with: written with it, an error tells a deadline that passed from any
// other failure (APIErrors.Write), as a generated route's does.
func bounded(r *http.Request, step func(r *http.Request) error) (*http.Request, error) {
	ctx, cancel := httpserver.Bounded(r.Context())
	defer cancel()
	br := r.WithContext(ctx)
	return br, step(br)
}
