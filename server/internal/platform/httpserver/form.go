package httpserver

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A streamed form (M7/P2 design 3.5; M7/P6 design 3.7): an upload's body,
// multipart/form-data, read as its parts come. Small text parts first,
// each at most once and in the route's order, then the file's part, whose
// bytes pass only once the route has decided on what came before it, then
// the closing boundary and nothing else. The attachments' upload and an
// import's read their bodies so.

// errPreface is a body with more bytes before its file than its preface
// lets through.
var errPreface = errors.New("too many bytes before the file")

// errInvalidPart is a text part that is not what its field takes.
var errInvalidPart = errors.New("the part is not valid")

// FormError is a form that is not what its route takes: 400 bad_request,
// naming the part whose value is wrong, if one is.
type FormError struct {
	Detail string
	// Part is the part whose value is not what its field takes: the
	// problem's field, invalid_format; "" for none.
	Part string
}

func (e *FormError) Error() string       { return e.Detail }
func (e *FormError) ProblemStatus() int  { return http.StatusBadRequest }
func (e *FormError) ProblemCode() string { return CodeBadRequest }

// ProblemFields is the part whose value is wrong, if one is.
func (e *FormError) ProblemFields() []error {
	if e.Part == "" {
		return nil
	}
	return []error{formField(e.Part)}
}

// formField is a part whose value is not what its field takes.
type formField string

func (f formField) Error() string        { return "has the wrong type or format" }
func (f formField) ProblemField() string { return string(f) }
func (f formField) ProblemCode() string  { return fieldInvalidFormat }

// InvalidPart is 400 on the part named name: its value is not what its
// field takes, which the route tells once the form read it.
func InvalidPart(name string) error {
	return &FormError{Detail: "The form's " + name + " part is not valid.", Part: name}
}

// BodyReadError is a body that failed to arrive: it ended early, was not
// well-formed under the form, or came too slowly. Err is the read's error;
// ReadCause names it for the log.
type BodyReadError struct {
	Err error
}

func (e *BodyReadError) Error() string { return "read the request body: " + e.Err.Error() }
func (e *BodyReadError) Unwrap() error { return e.Err }

// ReadCause names how a body failed to arrive, for the log: too slow, cut
// short, the connection failing, or malformed. The parser's words are not
// logged: they quote the client's lines.
func ReadCause(err error) string {
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

// Form is a streamed form's body, read as its parts come.
type Form struct {
	reader  *multipart.Reader
	preface *preface
	// texts are the text parts' names, in order; file the file part's.
	texts []string
	file  string
}

// NewForm reads r's body as multipart/form-data of the text parts texts,
// each at most once, in this order, any left out, then the part file;
// at most maxPreface bytes of the body before the file's bytes. 400 for
// a body of another type, or without a boundary.
func NewForm(r *http.Request, texts []string, file string, maxPreface int64) (*Form, error) {
	media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "multipart/form-data" || params["boundary"] == "" {
		return nil, &FormError{Detail: "The request body is not multipart/form-data with a boundary."}
	}
	p := &preface{r: r.Body, left: maxPreface}
	return &Form{reader: multipart.NewReader(p, params["boundary"]), preface: p, texts: texts, file: file}, nil
}

// Fields reads the parts up to the file's: each text part's value, of at
// most limits[name] bytes of UTF-8 (400 on the part for more, or other
// bytes); then the file's part, which it answers to be read once Open
// lets its bytes through. A part is read as it was sent: a
// Content-Transfer-Encoding is not decoded. A part out of order, twice,
// unknown, or no file part is 400; a body that fails is a
// *BodyReadError.
func (f *Form) Fields(limits map[string]int) (map[string]string, *multipart.Part, error) {
	values := map[string]string{}
	order := append(slices.Clone(f.texts), f.file)
	next := 0
	for {
		part, err := f.reader.NextRawPart()
		switch {
		case errors.Is(err, io.EOF):
			return nil, nil, &FormError{Detail: "The form has no " + f.file + " part."}
		case err != nil:
			return nil, nil, f.readError(err)
		}
		i := slices.Index(order, part.FormName())
		if i < next {
			return nil, nil, &FormError{Detail: "The form's parts are " + listed(order) + ", each once and in this order."}
		}
		next = i + 1
		if part.FormName() == f.file {
			return values, part, nil
		}
		v, err := value(part, limits[part.FormName()])
		if errors.Is(err, errInvalidPart) {
			return nil, nil, InvalidPart(part.FormName())
		}
		if err != nil {
			return nil, nil, f.readError(err)
		}
		values[part.FormName()] = v
	}
}

// listed is names as a sentence lists them: "a, b and c".
func listed(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// value reads a text part of at most limit bytes of UTF-8: errInvalidPart
// for more, or other bytes; the body's error when it fails.
func value(part *multipart.Part, limit int) (string, error) {
	b, err := io.ReadAll(io.LimitReader(part, int64(limit)+1))
	switch {
	case err != nil:
		return "", err
	case len(b) > limit || !utf8.Valid(b):
		return "", errInvalidPart
	}
	return string(b), nil
}

// Open lets the file's bytes through: the route decided on what came
// before them.
func (f *Form) Open() {
	f.preface.open = true
}

// readError is the body's failure under the form: 400 for more bytes than
// the preface's before the file, whatever the parser made of the read that
// ran out (a header line it cuts reads as malformed); a *BodyReadError
// otherwise.
func (f *Form) readError(err error) error {
	if errors.Is(err, errPreface) || !f.preface.open && f.preface.left <= 0 {
		return &FormError{Detail: "The form holds more than " + bytesWord(f.preface.limit()) + " before the file."}
	}
	return &BodyReadError{Err: err}
}

// End reads what follows the file: its closing boundary and nothing else,
// then the body to its end, so that the stream's read deadline goes (M7/P1
// design 7). A part after the file is 400, answered before it is read,
// one whose header runs past the route's limit or the parser's too, and so
// is a body that goes on past the limit after the form. A body that fails
// is a *BodyReadError.
func (f *Form) End(r *http.Request) error {
	var tooLarge *http.MaxBytesError
	_, err := f.reader.NextRawPart()
	switch {
	case err == nil || errors.As(err, &tooLarge) || errors.Is(err, multipart.ErrMessageTooLarge):
		return &FormError{Detail: "The form has a part after the " + f.file + "."}
	case !errors.Is(err, io.EOF):
		return f.readError(err)
	}
	switch _, err := io.Copy(io.Discard, r.Body); {
	case errors.As(err, &tooLarge):
		return &FormError{Detail: "The body goes on after the form's end."}
	case err != nil:
		return f.readError(err)
	}
	return nil
}

// preface lets left bytes of the body through until it is open: a body
// that holds more before the file fails with errPreface.
type preface struct {
	r     io.Reader
	left  int64
	given int64
	open  bool
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
	p.given += int64(n)
	return n, err
}

// limit is how many bytes the preface lets through.
func (p *preface) limit() int64 {
	return p.left + p.given
}

// bytesWord is n bytes as a detail says them: KiB when whole.
func bytesWord(n int64) string {
	if n%1024 == 0 {
		return strconv.FormatInt(n/1024, 10) + " KiB"
	}
	return strconv.FormatInt(n, 10) + " bytes"
}
