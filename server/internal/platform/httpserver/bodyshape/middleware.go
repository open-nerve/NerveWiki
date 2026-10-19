package bodyshape

import (
	"bytes"
	"io"
	"net/http"
)

// Middleware checks the body of every request whose route has a root in t
// with Table.Check, before the generated strict handler decodes it. It reads
// the whole body (already bounded by the platform's body limit) and puts it
// back unchanged.
//
// onError answers a body that could not be read (the read error, e.g.
// *http.MaxBytesError) and Check's errors: ErrNotJSON, *TooManyValues and
// *Error. A body that Check has nothing to say about, such as an empty one,
// is passed on.
func Middleware(t *Table, onError func(http.ResponseWriter, *http.Request, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := t.Roots[r.Pattern]; !ok || r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				onError(w, r, err)
				return
			}
			r.Body = read{Reader: bytes.NewReader(body), body: body}
			if err := t.Check(r.Pattern, body); err != nil {
				onError(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// read is a body Middleware read and put back, which Bytes finds.
type read struct {
	*bytes.Reader
	body []byte
}

func (read) Close() error { return nil }

// Bytes is the body of r as Middleware read and checked it, and true;
// false when Middleware did not read it. It is the bytes r.Body reads,
// not a copy, whatever of them r.Body has read since: a handler can look
// into a large body without reading it twice, and must not change it.
func Bytes(r *http.Request) ([]byte, bool) {
	b, ok := r.Body.(read)
	return b.body, ok
}
