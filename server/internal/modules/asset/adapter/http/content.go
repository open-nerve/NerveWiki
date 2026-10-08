package httpadapter

import (
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The download's handler (M7/P2 design 3.6): it reads the address's query
// strictly, opens the content, then sets the headers of its type and
// serves its bytes, ranges and conditional requests answered.

// contentPolicy is every content's Content-Security-Policy: a document
// the browser opens, an SVG one, runs in a sandbox of an opaque origin,
// without scripts, and asks nothing of another server (M7 design 4.5).
const contentPolicy = "sandbox; default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'"

// busyRetry is the Retry-After of a download asked for as the server shuts
// down: the time a restart takes.
const busyRetry = 5 * time.Second

type content struct {
	uc     *app.Content
	errors httpserver.APIErrors
}

func (h content) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a, ok := addressOf(pathID(r, "node_id"), r.URL.RawQuery)
	if !ok {
		h.errors.Write(w, r, domain.ErrContentNotFound)
		return
	}
	var o app.Opened
	err := bounded(r, func(r *http.Request) error {
		var err error
		o, err = h.uc.Open(r.Context(), a)
		return err
	})
	if err != nil {
		h.errors.Write(w, r, err)
		return
	}
	defer func() { _ = o.File.Close() }()
	if err := httpserver.Sending(r, o.Blob.Bytes); err != nil {
		if errors.Is(err, httpserver.ErrShuttingDown) {
			err = shared.ServerBusy(busyRetry)
		}
		h.errors.Write(w, r, err)
		return
	}
	setContentHeaders(w.Header(), o, a.Download)
	http.ServeContent(w, r, "", o.File.ModTime(), o.File)
}

// addressOf reads an address's query as the server writes it: b, e, s and
// d, each at most once, d=1 or absent, the rest present; b an id in
// lower-case with hyphens, e a decimal without sign or leading zeros, s 22
// base64url characters. Nothing is unescaped: an escaped spelling is
// another address. Anything else is no address.
func addressOf(node uuid.UUID, query string) (app.Address, bool) {
	values := map[string]string{}
	for part := range strings.SplitSeq(query, "&") {
		key, value, ok := strings.Cut(part, "=")
		if _, seen := values[key]; !ok || seen {
			return app.Address{}, false
		}
		values[key] = value
	}
	a := app.Address{Node: node, Signature: values["s"]}
	blob, err := uuid.Parse(values["b"])
	if err != nil || blob.String() != values["b"] {
		return app.Address{}, false
	}
	a.Blob = blob
	a.Expires, err = strconv.ParseInt(values["e"], 10, 64)
	if err != nil || a.Expires <= 0 || strconv.FormatInt(a.Expires, 10) != values["e"] {
		return app.Address{}, false
	}
	if !signature(a.Signature) {
		return app.Address{}, false
	}
	switch d, ok := values["d"]; {
	case ok && d != "1":
		return app.Address{}, false
	case ok:
		a.Download = true
	}
	for key := range values {
		if key != "b" && key != "e" && key != "s" && key != "d" {
			return app.Address{}, false
		}
	}
	return a, true
}

// signature reports whether s is spelled as a signature: 22 base64url
// characters.
func signature(s string) bool {
	if len(s) != 22 {
		return false
	}
	for _, c := range []byte(s) {
		if !alnum(c) && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// alnum reports whether c is an ASCII letter or digit.
func alnum(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9'
}

// setContentHeaders sets the headers of o's answer, which replace the
// API's no-store (M7/P2 design 3.6): its type, shown or downloaded under
// its name, the sandbox, private caching until the address expires, its
// SHA-256 as its ETag, and no reading by another site.
func setContentHeaders(h http.Header, o app.Opened, download bool) {
	h.Set("Content-Type", o.Blob.MIME)
	h.Set("Content-Disposition", disposition(domain.Inline(o.Blob.MIME, download), o.Name))
	h.Set("Content-Security-Policy", contentPolicy)
	h.Set("Cache-Control", "private, max-age="+strconv.FormatInt(max(0, int64(o.Left/time.Second)), 10)+", immutable")
	h.Set("ETag", `"`+hex.EncodeToString(o.Blob.SHA256)+`"`)
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
}

// disposition is the Content-Disposition of a file named name (RFC 6266):
// inline or attachment, its name in ASCII, every other character, a quote
// and a backslash an underscore, and in UTF-8 (RFC 8187).
func disposition(inline bool, name string) string {
	kind := "attachment"
	if inline {
		kind = "inline"
	}
	var ascii, utf strings.Builder
	for _, r := range name {
		if r < 0x20 || r >= 0x7f || r == '"' || r == '\\' {
			ascii.WriteByte('_')
		} else {
			ascii.WriteRune(r)
		}
	}
	for _, c := range []byte(name) {
		if attrChar(c) {
			utf.WriteByte(c)
		} else {
			utf.WriteString("%" + strings.ToUpper(hex.EncodeToString([]byte{c})))
		}
	}
	return kind + `; filename="` + ascii.String() + `"; filename*=UTF-8''` + utf.String()
}

// attrChar reports whether c is written as itself in an RFC 8187 value.
func attrChar(c byte) bool {
	return alnum(c) || strings.IndexByte("!#$&+-.^_`|~", c) >= 0
}
