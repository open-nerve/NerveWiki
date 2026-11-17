// Package client is perf's way to a serve (M12/P4 design 3.4, 3.4.4): a
// small REST client that records each request, MCP's JSON-RPC by hand,
// and psql in the PostgreSQL container through docker exec, its executor
// given so that a test answers for it.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"
	"sync"
	"time"
)

// Record is a request's line (design 3.4.4): its operation, the
// credential's number, when it was sent, when the answer's head came and
// when its body was read (Unix nanoseconds), the status, a problem's code,
// the answer's bytes, and an error of the transport.
type Record struct {
	Op         string `json:"op"`
	Credential int    `json:"credential"`
	Sent       int64  `json:"sent_unix_ns"`
	Header     int64  `json:"header_unix_ns"`
	Done       int64  `json:"done_unix_ns"`
	Status     int    `json:"status"`
	Code       string `json:"code,omitempty"`
	Bytes      int64  `json:"bytes"`
	Error      string `json:"error,omitempty"`
}

// ToHeader is the time from the request's sending to its answer's head.
func (r Record) ToHeader() time.Duration { return time.Duration(r.Header - r.Sent) }

// Took is the time from the request's sending to its answer's end.
func (r Record) Took() time.Duration { return time.Duration(r.Done - r.Sent) }

// Log writes records, a JSON line each, from any goroutine; it keeps the
// first error of its writer, and its records for a summary.
type Log struct {
	mu      sync.Mutex
	w       io.Writer
	err     error
	records []Record
}

// NewLog is a log to w; nil keeps the records alone.
func NewLog(w io.Writer) *Log { return &Log{w: w} }

// Add writes r.
func (l *Log) Add(r Record) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, r)
	if l.w == nil || l.err != nil {
		return
	}
	b, err := json.Marshal(r)
	if err == nil {
		_, err = l.w.Write(append(b, '\n'))
	}
	l.err = err
}

// Records are the records so far.
func (l *Log) Records() []Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.records)
}

// Err is the writer's first error.
func (l *Log) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

// Client is a credential's client of a serve at Base: Token its bearer
// token, Credential its number in the records.
type Client struct {
	Base       string
	Token      string
	Credential int
	HTTP       *http.Client
	Log        *Log
	Now        func() time.Time
}

// New is a client of base as token, its records to log, with no timeout
// of its own: a measurement waits for what the serve does.
func New(base, token string, credential int, log *Log) *Client {
	return &Client{Base: base, Token: token, Credential: credential, HTTP: &http.Client{}, Log: log, Now: time.Now}
}

// StatusError is an answer of a status not wanted.
type StatusError struct {
	Op     string
	Status int
	Code   string
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: %d %s %.300s", e.Op, e.Status, e.Code, e.Body)
}

// Request is a request to send: its operation's name for the record, its
// method, path and query, body and its type, and headers besides.
type Request struct {
	Op, Method, Path string
	Body             io.Reader
	ContentType      string
	Header           http.Header
}

// Send sends r and reads its whole answer: its record, logged, and its
// body; into, when given, takes the body instead, which is then not kept.
func (c *Client) Send(ctx context.Context, r Request, into io.Writer) (Record, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, r.Method, c.Base+r.Path, r.Body)
	if err != nil {
		return Record{}, nil, err
	}
	for k, vs := range r.Header {
		req.Header[k] = vs
	}
	if r.ContentType != "" {
		req.Header.Set("Content-Type", r.ContentType)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	rec := Record{Op: r.Op, Credential: c.Credential, Sent: c.Now().UnixNano()}
	res, err := c.HTTP.Do(req)
	if err != nil {
		rec.Header, rec.Done, rec.Error = c.Now().UnixNano(), c.Now().UnixNano(), err.Error()
		c.add(rec)
		return rec, nil, err
	}
	rec.Header, rec.Status = c.Now().UnixNano(), res.StatusCode
	var body bytes.Buffer
	sink := io.Writer(&body)
	problem := isProblem(res.Header.Get("Content-Type"))
	if into != nil && !problem {
		sink = into
	}
	n, err := io.Copy(sink, res.Body)
	_ = res.Body.Close()
	rec.Done, rec.Bytes = c.Now().UnixNano(), n
	if err != nil {
		rec.Error = err.Error()
	}
	if problem {
		var p struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(body.Bytes(), &p) == nil {
			rec.Code = p.Code
		}
	}
	c.add(rec)
	return rec, body.Bytes(), err
}

func (c *Client) add(r Record) {
	if c.Log != nil {
		c.Log.Add(r)
	}
}

// isProblem reports whether a content type is problem+json's.
func isProblem(contentType string) bool {
	t, _, err := mime.ParseMediaType(contentType)
	return err == nil && t == "application/problem+json"
}

// JSON sends in as JSON (none when nil) and decodes the answer into out
// (unless nil): an error unless the status is one of want.
func (c *Client) JSON(ctx context.Context, op, method, path string, in, out any, want ...int) (Record, error) {
	var body io.Reader
	contentType := ""
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return Record{}, err
		}
		body, contentType = bytes.NewReader(b), "application/json"
	}
	rec, b, err := c.Send(ctx, Request{Op: op, Method: method, Path: path, Body: body, ContentType: contentType}, nil)
	if err != nil {
		return rec, err
	}
	if !slices.Contains(want, rec.Status) {
		return rec, &StatusError{Op: op, Status: rec.Status, Code: rec.Code, Body: string(b)}
	}
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			return rec, fmt.Errorf("%s: %w", op, err)
		}
	}
	return rec, nil
}

// Login signs in by email and password: the access token.
func Login(ctx context.Context, base, email, password string) (string, error) {
	c := New(base, "", -1, nil)
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if _, err := c.JSON(ctx, "login", http.MethodPost, "/api/v0/auth/login", map[string]string{"email": email, "password": password},
		&out, http.StatusOK); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("login answered no access token")
	}
	return out.AccessToken, nil
}
