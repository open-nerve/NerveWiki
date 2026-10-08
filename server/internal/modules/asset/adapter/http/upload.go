package httpadapter

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The upload's handler (M7/P2 design 3.5): it reads the parts as they
// come, decides before the file, streams the file to the store, and
// creates the attachment once the body is read to its end.

// Envelope is how many bytes the body may hold besides the file: the
// parts before it and the boundaries. The route's body limit is
// asset.max_bytes and this.
const Envelope = 64 << 10

// maxPreface is the most bytes read of the body before the file is opened:
// the parts before it and their headers, the file part's header, and what
// the parser reads ahead of them.
const maxPreface = 4 << 10

// maxName is the longest name part, in bytes.
const maxName = 1 << 10

// parts are the form's parts, in their order: the first two may be left
// out.
func parts() []string { return []string{"parent_id", "name", "file"} }

// errPreface is a body with more than maxPreface bytes before its file.
var errPreface = errors.New("more than 4 KiB before the file")

type upload struct {
	uc       *app.Upload
	errors   httpserver.APIErrors
	logger   *slog.Logger
	maxBytes int64
}

func (h upload) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	client, err := clientOf(r)
	if err != nil {
		h.early(w, r, err)
		return
	}
	form, err := newForm(r)
	if err != nil {
		h.early(w, r, err)
		return
	}
	req, file, err := form.fields(pathID(r, "notebook_id"), client)
	if err != nil {
		h.early(w, r, err)
		return
	}
	if br, err := bounded(r, func(r *http.Request) error { return h.uc.Check(r.Context(), req) }); err != nil {
		h.early(w, br, err)
		return
	}
	form.preface.open = true
	blob, err := h.uc.Store(r.Context(), req, file)
	if err != nil {
		h.early(w, r, err)
		return
	}
	if err := form.end(r); err != nil {
		h.discard(r, blob)
		h.early(w, r, err)
		return
	}
	var asset app.Asset
	br, err := bounded(r, func(r *http.Request) error {
		var err error
		asset, err = h.uc.Create(r.Context(), req, blob)
		return err
	})
	if err != nil {
		h.errors.Write(w, br, err)
		return
	}
	writeJSON(w, http.StatusCreated, assetOf(asset))
}

func (h upload) discard(r *http.Request, blob domain.Blob) {
	ctx, cancel := httpserver.Bounded(r.Context())
	defer cancel()
	h.uc.Discard(ctx, blob)
}

// early answers err before the body's end, closing the connection after
// the answer: net/http would read up to 256 KiB more of the body first,
// and a client that holds its file back until it hears would wait for the
// read deadline. A browser still sending may see the connection reset: the
// web checks before it sends (M7/P2 design 3.5).
func (h upload) early(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Connection", "close")
	h.readFailed(w, r, err)
}

// readFailed answers an upload whose body failed. The server's shutdown
// cut it off, its deadlines passed: the connection is aborted, logged as
// no error. A file over the largest is 413; a body that did not arrive
// whole, the client gone or too slow, is logged and answered 400 if the
// connection still takes it, its cause named but not the parser's words,
// which quote the client's lines. Any other error is the platform's to
// answer: a ProblemError as itself, the rest 500.
func (h upload) readFailed(w http.ResponseWriter, r *http.Request, err error) {
	var tooLarge *http.MaxBytesError
	var read *app.ReadError
	switch {
	case errors.Is(err, httpserver.ErrShuttingDown):
		h.logger.InfoContext(r.Context(), "upload cut off by the shutdown", slog.String("request_id", httpserver.RequestID(r.Context())))
		panic(http.ErrAbortHandler)
	case errors.Is(err, domain.ErrTooLarge) || errors.As(err, &tooLarge):
		h.errors.Write(w, r, &http.MaxBytesError{Limit: h.maxBytes})
	case errors.As(err, &read):
		h.logger.InfoContext(r.Context(), "upload not received", slog.String("request_id", httpserver.RequestID(r.Context())),
			slog.String("cause", readCause(read.Err)))
		h.errors.Write(w, r, badRequest("The request body was not read whole: it ended early, was not well-formed, or arrived too slowly."))
	default:
		h.errors.Write(w, r, err)
	}
}

// form is the body's parts, read as they come, the reads before the file
// bounded by preface.
type form struct {
	reader  *multipart.Reader
	preface *preface
}

// newForm reads r's body as multipart/form-data: 400 for another type, or
// one without a boundary.
func newForm(r *http.Request) (*form, error) {
	media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "multipart/form-data" || params["boundary"] == "" {
		return nil, badRequest("The request body is not multipart/form-data with a boundary.")
	}
	p := &preface{r: r.Body, left: maxPreface}
	return &form{reader: multipart.NewReader(p, params["boundary"]), preface: p}, nil
}

// fields reads the parts up to the file's: parent_id and name, each at
// most once and in order, then file, whose part it answers to be read.
// Without a name, or with one of blanks, the file's name is taken. A part
// is read as it was sent: a Content-Transfer-Encoding is not decoded.
// Anything else is 400.
func (f *form) fields(notebookID uuid.UUID, client string) (app.Request, io.Reader, error) {
	req := app.Request{NotebookID: notebookID, Client: client}
	next := 0
	var named bool
	for {
		part, err := f.reader.NextRawPart()
		switch {
		case errors.Is(err, io.EOF):
			return app.Request{}, nil, badRequest("The form has no file part.")
		case err != nil:
			return app.Request{}, nil, f.readError(err)
		}
		i := slices.Index(parts(), part.FormName())
		if i < next {
			return app.Request{}, nil, badRequest("The form's parts are parent_id, name and file, each once and in this order.")
		}
		next = i + 1
		switch part.FormName() {
		case "parent_id":
			v, err := value(part, len("00000000-0000-0000-0000-000000000000"))
			if err != nil {
				return app.Request{}, nil, f.partError("parent_id", err)
			}
			id, err := uuid.Parse(v)
			if err != nil {
				return app.Request{}, nil, f.partError("parent_id", errInvalid)
			}
			req.ParentID = &id
		case "name":
			v, err := value(part, maxName)
			if err != nil {
				return app.Request{}, nil, f.partError("name", err)
			}
			req.Name, named = v, strings.TrimSpace(v) != ""
		case "file":
			if !named {
				req.Name = part.FileName()
			}
			return req, part, nil
		}
	}
}

// errInvalid is a text part that is not what its field takes.
var errInvalid = errors.New("the part is not valid")

// value reads a text part of at most limit bytes of UTF-8: errInvalid
// for more, or other bytes; the body's error when it fails.
func value(part *multipart.Part, limit int) (string, error) {
	b, err := io.ReadAll(io.LimitReader(part, int64(limit)+1))
	switch {
	case err != nil:
		return "", err
	case len(b) > limit || !utf8.Valid(b):
		return "", errInvalid
	}
	return string(b), nil
}

// partError is 400 on the part named field for errInvalid; any other
// error is the body's (readError).
func (f *form) partError(field string, err error) error {
	if errors.Is(err, errInvalid) {
		return badRequest("The form's "+field+" part is not valid.",
			shared.FieldError{Field: field, Code: shared.FieldInvalidFormat, Message: "has the wrong type or format"})
	}
	return f.readError(err)
}

// readError is the body's failure under the form: 400 for more than
// maxPreface bytes before the file, whatever the parser made of the read
// that ran out (a header line it cuts reads as malformed); an
// *app.ReadError otherwise, which readFailed tells apart.
func (f *form) readError(err error) error {
	if errors.Is(err, errPreface) || !f.preface.open && f.preface.left <= 0 {
		return badRequest("The form holds more than 4 KiB before the file.")
	}
	return &app.ReadError{Err: err}
}

// readCause names how a body failed to arrive, for the log: too slow, cut
// short, the connection failing, or malformed.
func readCause(err error) string {
	var netErr net.Error
	switch {
	case errors.As(err, &netErr) && netErr.Timeout():
		return "too slow"
	case errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF):
		return "ended early"
	case errors.As(err, &netErr):
		return "connection failed"
	}
	return "malformed"
}

// end reads what follows the file: its closing boundary and nothing else,
// then the body to its end, so that the stream's read deadline goes (M7/P1
// design 7). A part after the file is 400, answered before it is read, and
// so is a body that goes on past the route's limit after the form.
func (f *form) end(r *http.Request) error {
	_, err := f.reader.NextRawPart()
	switch {
	case err == nil:
		return badRequest("The form has a part after the file.")
	case !errors.Is(err, io.EOF):
		return f.readError(err)
	}
	var tooLarge *http.MaxBytesError
	switch _, err := io.Copy(io.Discard, r.Body); {
	case errors.As(err, &tooLarge):
		return badRequest("The body goes on after the form's end.")
	case err != nil:
		return f.readError(err)
	}
	return nil
}

// preface lets left bytes of the body through until it is open: a body
// that holds more before the file fails with errPreface.
type preface struct {
	r    io.Reader
	left int64
	open bool
}

func (p *preface) Read(b []byte) (int, error) {
	if p.open {
		return p.r.Read(b)
	}
	if p.left <= 0 {
		return 0, errPreface
	}
	if int64(len(b)) > p.left {
		b = b[:p.left]
	}
	n, err := p.r.Read(b)
	p.left -= int64(n)
	return n, err
}

// badRequest is 400 bad_request with detail, and the fields when any.
func badRequest(detail string, fields ...shared.FieldError) error {
	return &shared.Error{Kind: shared.KindBadRequest, Code: shared.CodeBadRequest, Detail: detail, Fields: fields}
}

// clientOf is where the request came from: a personal access token's is
// the API's, a sign-in session's access token the web's.
func clientOf(r *http.Request) (string, error) {
	actor, err := shared.RequireActor(r.Context())
	switch {
	case err != nil:
		return "", err
	case actor.APITokenID != uuid.UUID{}:
		return "api", nil
	}
	return "web", nil
}

// assetOf is the attachment as the API answers it, its addresses signed.
func assetOf(a app.Asset) gen.Asset {
	n, b, s := a.Node, a.Blob, a.Signed
	return gen.Asset{
		ID: n.ID, NotebookID: n.NotebookID, ParentID: nullableOf(n.ParentID), Name: n.Name, Mime: b.MIME, ByteSize: b.Bytes,
		Sha256: hex.EncodeToString(b.SHA256), Width: sideOf(b.Width), Height: sideOf(b.Height), CreatedBy: b.CreatedBy,
		CreatedAt: b.CreatedAt, ContentURL: contentURL(n.ID, b.ID, s, false), DownloadURL: contentURL(n.ID, b.ID, s, true),
		ExpiresAt: s.Expires,
	}
}

// contentURL is the signed address of the content of node's file blob,
// shown or downloaded.
func contentURL(node, blob uuid.UUID, s app.Signed, download bool) string {
	u := "/api/v0/assets/" + node.String() + "/content?b=" + blob.String() + "&e=" + strconv.FormatInt(s.Expires.Unix(), 10)
	if download {
		return u + "&s=" + s.Download + "&d=1"
	}
	return u + "&s=" + s.Inline
}

func nullableOf(id *uuid.UUID) nullable.Nullable[uuid.UUID] {
	if id == nil {
		return nullable.NewNullNullable[uuid.UUID]()
	}
	return nullable.NewNullableWithValue(*id)
}

func sideOf(n int) nullable.Nullable[int] {
	if n == 0 {
		return nullable.NewNullNullable[int]()
	}
	return nullable.NewNullableWithValue(n)
}

// writeJSON answers v as JSON with status, as the generated handlers do.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
