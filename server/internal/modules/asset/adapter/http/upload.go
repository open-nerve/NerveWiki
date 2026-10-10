package httpadapter

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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
	form, err := httpserver.NewForm(r, []string{"parent_id", "name"}, "file", maxPreface)
	if err != nil {
		h.early(w, r, err)
		return
	}
	req, file, err := fields(form, pathID(r, "notebook_id"), client)
	if err != nil {
		h.early(w, r, err)
		return
	}
	if br, err := bounded(r, func(r *http.Request) error { return h.uc.Check(r.Context(), req) }); err != nil {
		h.early(w, br, err)
		return
	}
	form.Open()
	blob, err := h.uc.Store(r.Context(), req, file)
	if err != nil {
		h.early(w, r, err)
		return
	}
	if err := form.End(r); err != nil {
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
// no error. A file over the largest is 413 (the route's limit, the largest
// and the Envelope, is reached after the file only, which end answers); a
// body that did not arrive whole, the client gone or too slow, is logged
// and answered 400 if the connection still takes it, its cause named but
// not the parser's words, which quote the client's lines. Any other error
// is the platform's to answer: a ProblemError as itself, the rest 500.
func (h upload) readFailed(w http.ResponseWriter, r *http.Request, err error) {
	var read *app.ReadError
	var body *httpserver.BodyReadError
	switch {
	case errors.Is(err, httpserver.ErrShuttingDown):
		h.logger.InfoContext(r.Context(), "upload cut off by the shutdown", slog.String("request_id", httpserver.RequestID(r.Context())))
		panic(http.ErrAbortHandler)
	case errors.Is(err, domain.ErrTooLarge):
		h.errors.Write(w, r, &http.MaxBytesError{Limit: h.maxBytes})
	case errors.As(err, &read) || errors.As(err, &body):
		h.logger.InfoContext(r.Context(), "upload not received", slog.String("request_id", httpserver.RequestID(r.Context())),
			slog.String("cause", httpserver.ReadCause(err)))
		h.errors.Write(w, r, badRequest("The request body was not read whole: it ended early, was not well-formed, or arrived too slowly."))
	default:
		h.errors.Write(w, r, err)
	}
}

// fields reads form's parts up to the file's: parent_id and name, each at
// most once and in order, then file, whose part it answers to be read.
// Without a name, or with one of blanks, the file's name is taken.
// Anything else is 400.
func fields(form *httpserver.Form, notebookID uuid.UUID, client string) (app.Request, io.Reader, error) {
	values, file, err := form.Fields(map[string]int{"parent_id": len("00000000-0000-0000-0000-000000000000"), "name": maxName})
	if err != nil {
		return app.Request{}, nil, err
	}
	req := app.Request{NotebookID: notebookID, Client: client, Name: file.FileName()}
	if v, ok := values["parent_id"]; ok {
		id, err := uuid.Parse(v)
		if err != nil {
			return app.Request{}, nil, httpserver.InvalidPart("parent_id")
		}
		req.ParentID = &id
	}
	if v := values["name"]; strings.TrimSpace(v) != "" {
		req.Name = v
	}
	return req, file, nil
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
		ID: n.ID, NotebookID: n.NotebookID, ParentID: nullableOf(n.ParentID), Name: n.Name, Link: linkOf(a.Link), Mime: b.MIME, ByteSize: b.Bytes,
		Sha256: hex.EncodeToString(b.SHA256), Width: sideOf(b.Width), Height: sideOf(b.Height), CreatedBy: b.CreatedBy,
		CreatedAt: b.CreatedAt, ContentURL: ContentURL(n.ID, b.ID, s, false), DownloadURL: ContentURL(n.ID, b.ID, s, true),
		ExpiresAt: s.Expires,
	}
}

// ContentURL is the signed address of the content of node's file blob,
// shown or downloaded: the content route's path and query, a path of this
// site.
func ContentURL(node, blob uuid.UUID, s app.Signed, download bool) string {
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

// linkOf is an attachment's link, null for none.
func linkOf(link string) nullable.Nullable[string] {
	if link == "" {
		return nullable.NewNullNullable[string]()
	}
	return nullable.NewNullableWithValue(link)
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
