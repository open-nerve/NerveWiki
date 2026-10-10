package httpserver

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// ServeFixed sends content, modified at modtime, through http.ServeContent
// for an address whose file never changes: a range and the conditions on a
// copy the client holds are answered (206, 304, 416). A condition on a
// change (If-Match, If-Unmodified-Since) has nothing to guard, and its 412
// would carry the file's headers: it is passed by. So is a Range of
// several ranges (RFC 9110 lets a server ignore Range): each would be a
// part of its own, a header of thousands of them an answer many times the
// file's size (M7 closeout A-N1); the whole file is sent.
func ServeFixed(w http.ResponseWriter, r *http.Request, modtime time.Time, content io.ReadSeeker) {
	r.Header.Del("If-Match")
	r.Header.Del("If-Unmodified-Since")
	if strings.Contains(r.Header.Get("Range"), ",") {
		r.Header.Del("Range")
	}
	http.ServeContent(w, r, "", modtime, content)
}
