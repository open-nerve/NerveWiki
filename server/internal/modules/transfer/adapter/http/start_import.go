package httpadapter

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The import's start (M7/P6 design 3.9): it reads the parts as they come,
// decides before the file, streams the file to the store, and creates the
// job once the body is read to its end, as an attachment's upload does.

// Envelope is how many bytes the body may hold besides the archive: the
// part before it and the boundaries. The route's body limit is
// transfer.import_max_bytes and this.
const Envelope = 64 << 10

// maxPreface is the most bytes read of the body before the file is opened:
// the part before it and its headers, the file part's header, and what
// the parser reads ahead of them.
const maxPreface = 4 << 10

type startImport struct {
	uc       *app.StartImport
	errors   httpserver.APIErrors
	logger   *slog.Logger
	maxBytes int64
}

func (h startImport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	client, err := clientOf(r.Context())
	if err != nil {
		h.early(w, r, err)
		return
	}
	// A body that says it is larger than the route takes is 413 at once.
	if r.ContentLength > h.maxBytes+Envelope {
		h.early(w, r, app.ErrTooLarge)
		return
	}
	form, err := httpserver.NewForm(r, []string{"parent_id"}, "file", maxPreface)
	if err != nil {
		h.early(w, r, err)
		return
	}
	req, file, err := fields(form, pathID(r, "notebook_id"), client)
	if err != nil {
		h.early(w, r, err)
		return
	}
	req.Size = r.ContentLength
	if br, err := bounded(r, func(r *http.Request) error { return h.uc.Check(r.Context(), req) }); err != nil {
		h.early(w, br, err)
		return
	}
	defer h.uc.Release(req)
	form.Open()
	stored, err := h.uc.Store(r.Context(), file)
	if err != nil {
		h.early(w, r, err)
		return
	}
	if err := form.End(r); err != nil {
		h.discard(r, stored)
		h.early(w, r, err)
		return
	}
	var job app.JobView
	br, err := bounded(r, func(r *http.Request) error {
		var err error
		job, err = h.uc.Create(r.Context(), req, stored)
		return err
	})
	if err != nil {
		h.errors.Write(w, br, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(jobOf(job))
}

func (h startImport) discard(r *http.Request, stored app.Stored) {
	ctx, cancel := httpserver.Bounded(r.Context())
	defer cancel()
	h.uc.Discard(ctx, stored)
}

// early answers err before the body's end, closing the connection after
// the answer: net/http would read up to 256 KiB more of the body first,
// and a client that holds its file back until it hears would wait for the
// read deadline.
func (h startImport) early(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Connection", "close")
	h.readFailed(w, r, err)
}

// readFailed answers a start whose body failed. The server's shutdown cut
// it off, its deadlines passed: the connection is aborted, logged as no
// error. An archive over the largest is 413 (the route's limit, the
// largest and the Envelope, is reached after the file only, which End
// answers); a body that did not arrive whole, the client gone or too
// slow, is logged and answered 400 if the connection still takes it, its
// cause named but not the parser's words. Any other error is the
// platform's to answer: a ProblemError as itself, the rest 500.
func (h startImport) readFailed(w http.ResponseWriter, r *http.Request, err error) {
	var read *app.ReadError
	var body *httpserver.BodyReadError
	switch {
	case errors.Is(err, httpserver.ErrShuttingDown):
		h.logger.InfoContext(r.Context(), "import's upload cut off by the shutdown", slog.String("request_id", httpserver.RequestID(r.Context())))
		panic(http.ErrAbortHandler)
	case errors.Is(err, app.ErrTooLarge):
		h.errors.Write(w, r, &http.MaxBytesError{Limit: h.maxBytes})
	case errors.As(err, &read) || errors.As(err, &body):
		h.logger.InfoContext(r.Context(), "import's upload not received", slog.String("request_id", httpserver.RequestID(r.Context())),
			slog.String("cause", httpserver.ReadCause(err)))
		h.errors.Write(w, r, &shared.Error{Kind: shared.KindBadRequest, Code: shared.CodeBadRequest,
			Detail: "The request body was not read whole: it ended early, was not well-formed, or arrived too slowly."})
	default:
		h.errors.Write(w, r, err)
	}
}

// fields reads form's parts up to the file's: parent_id, at most once,
// then file, whose part it answers to be read. Anything else is 400.
func fields(form *httpserver.Form, notebookID uuid.UUID, client domain.Client) (app.ImportRequest, io.Reader, error) {
	values, file, err := form.Fields(map[string]int{"parent_id": len("00000000-0000-0000-0000-000000000000")})
	if err != nil {
		return app.ImportRequest{}, nil, err
	}
	req := app.ImportRequest{NotebookID: notebookID, FileName: file.FileName(), Client: client}
	if v, ok := values["parent_id"]; ok {
		id, err := uuid.Parse(v)
		if err != nil {
			return app.ImportRequest{}, nil, httpserver.InvalidPart("parent_id")
		}
		req.ParentID = &id
	}
	return req, file, nil
}
