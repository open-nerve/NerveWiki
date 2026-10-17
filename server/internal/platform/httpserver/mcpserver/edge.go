package mcpserver

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"net/http"
	"net/url"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// Pattern is the route of both entries on the root router; httpserver
// owns the string, since its fixed headers key on it (M9/P1 design 3.2.3).
const Pattern = httpserver.MCPPath

// Bodies is Pattern's body structure: one object, whose params, when
// there are any, are an object: its name a string, its arguments and its
// _meta objects, and no inputResponses or requestState, which answer an
// input-required result that no protocol version of this server's has.
// An array (a JSON-RPC batch), a member named twice and an unpaired
// surrogate are 400 before the SDK (M9 design 4.3, 4.4); so are params
// the SDK could not read into a request's, whose error repeats them,
// escaped to four times their size (M9/P1 S1 review A-M1): of a
// tools/call, which alone may be as long as the route's limit
// (maxOtherBytes), they cannot fail. Null is the member left out, as the
// SDK reads it. A function, not a package variable: each caller gets its
// own table (gochecknoglobals).
func Bodies() *bodyshape.Table {
	const (
		message = iota
		params
		name
		object
		null
	)
	open := func(types bodyshape.Type, props map[string]int) bodyshape.Node {
		return bodyshape.Node{Types: types, Props: props, Extra: bodyshape.Open, Items: bodyshape.Open}
	}
	return &bodyshape.Table{
		Nodes: []bodyshape.Node{
			message: open(bodyshape.Object, map[string]int{"params": params}),
			params: open(bodyshape.Object|bodyshape.Null, map[string]int{
				"name": name, "arguments": object, "_meta": object, "inputResponses": null, "requestState": null,
			}),
			name:   open(bodyshape.String, nil),
			object: open(bodyshape.Object|bodyshape.Null, nil),
			null:   open(bodyshape.Null, nil),
		},
		Roots: map[string]int{Pattern: message},
	}
}

// maxOtherBytes bounds the body of every request but a tools/call of a
// tool its entry serves: a handshake, a ping or a prompt's get is a few
// hundred bytes, and the SDK's error of params it cannot read repeats
// them, escaped to four times their size; a tools/call's arguments, which
// may be a page, it never reads (M9/P1 S1 review A-M1).
const maxOtherBytes = 64 << 10

// maxIDBytes bounds the id of a body over maxOtherBytes, as it is written:
// the SDK repeats the id in its answer, escaped to as much as twice its
// bytes (M9/P1 S1 fix-check A F2).
const maxIDBytes = 256

// sizeError is the 413 of the body of r, as bodyshape read it, when it is
// over maxOtherBytes and no tools/call that the SDK reads without an
// answer as long: the error of API.Wrap's body limit over the route's.
// The body was one JSON object without a name twice, and is read as the
// SDK reads it. Its jsonrpc is 2.0, as the SDK's error of another tag
// repeats it, escaped; its id at most maxIDBytes; every number of its
// params' _meta one that a float64 holds, as the SDK's error of one that
// none does repeats the params (fix-check A F1, F2); its method is
// tools/call, of a tool of the entry the query asks for, which alone the
// SDK reads the whole body for.
//
// encoding/json/v2, not encoding/json: it reads names by their exact
// case, as the SDK's decoder does, where encoding/json would take a
// METHOD for the method. In go1.27.1 it is behind GOEXPERIMENT jsonv2,
// which is on by default: a toolchain bump checks that it still is, or
// that the package has left the experiment.
func (s *Server) sizeError(r *http.Request) error {
	body, ok := bodyshape.Bytes(r)
	if !ok || len(body) <= maxOtherBytes {
		return nil
	}
	var m struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      jsontext.Value `json:"id"`
		Method  string         `json:"method"`
		Params  struct {
			Name string         `json:"name"`
			Meta map[string]any `json:"_meta"`
		} `json:"params"`
	}
	if err := jsonv2.Unmarshal(body, &m); err == nil && m.JSONRPC == "2.0" && len(m.ID) <= maxIDBytes &&
		m.Method == methodCallTool && s.tools[askedEntry(r.URL)][m.Params.Name] {
		return nil
	}
	return &http.MaxBytesError{Limit: maxOtherBytes}
}

// Subtree answers every path below Pattern: 404 problem+json, not the
// web app's page.
func Subtree() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpserver.WriteProblem(w, httpserver.Problem{
			Status: http.StatusNotFound,
			Code:   httpserver.CodeNotFound,
			Title:  http.StatusText(http.StatusNotFound),
			Detail: "No MCP endpoint at " + r.URL.Path + "; the endpoint is " + Pattern + ".",
		})
	})
}

// NoOAuth is what a client looking for OAuth is told, under /.well-known/
// and at RegisterPattern alike, and the detail of Pattern's 401 without a
// token, which bootstrap gives the API (httpserver.APIConfig's
// NoTokenDetails): a client without OAuth shows that detail.
const NoOAuth = "This server has no OAuth: MCP takes a personal access token in the Authorization header."

// WellKnown answers every path under /.well-known/, where an MCP client
// that got a 401 looks for OAuth's metadata (M9 design 4.4): 404
// problem+json, so that it finds no OAuth rather than the web app's page.
func WellKnown() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpserver.WriteProblem(w, httpserver.Problem{
			Status: http.StatusNotFound,
			Code:   httpserver.CodeNotFound,
			Title:  http.StatusText(http.StatusNotFound),
			Detail: NoOAuth,
		})
	})
}

// RegisterPattern is OAuth's dynamic client registration at the origin,
// where a client turns when no metadata names an authorization server
// (the default endpoints of MCP 2025-03-26): the TypeScript SDK posts its
// registration there after the 404s under /.well-known/, and shows what
// answers it as an OAuth error (M9/P1 S1's client trial, step 4b).
// httpserver owns the path: its answers are no-store.
const RegisterPattern = http.MethodPost + " " + httpserver.RegisterPath

// Register answers RegisterPattern with OAuth's error, 400
// invalid_client_metadata (RFC 7591 3.2.2): a client shows its
// error_description as it is, where the web app's 405 would be a JSON
// parse error. It reads no body.
func Register() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(oauthError{Error: "invalid_client_metadata", Description: NoOAuth}) // nothing to do if the client is gone
	})
}

// oauthError is an OAuth error response (RFC 6749 5.2, RFC 7591 3.2.2).
type oauthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

// forbidden is the 403 of a credential that is valid but no personal
// access token. It satisfies httpserver.ProblemError by structure.
type forbidden struct{}

func (forbidden) Error() string {
	return "MCP takes a personal access token, not a session's access token."
}

// ProblemStatus is 403: the credential is valid, but not enough (RFC 6750
// 3.1; v0.1 design 13.1 item 17).
func (forbidden) ProblemStatus() int { return http.StatusForbidden }

// ProblemCode is the platform's forbidden.
func (forbidden) ProblemCode() string { return "forbidden" }

// notPAT is a session's access token on Pattern (M9 design 4.4): it
// expires in a quarter of an hour, and no MCP client refreshes it.
func notPAT() error { return forbidden{} }

// errGateLetZero is a notebook gate that let the zero id through: a fault
// of the gate, never the account entry.
var errGateLetZero = errors.New("mcpserver: the notebook gate let the zero id through")

// askedEntry is the entry whose server reads the body of a request to u:
// the account entry's for a query that parses and has no notebook
// parameter, as entryOf decides; for any other, the notebook entry's, or
// none, entryOf answering 404 first.
func askedEntry(u *url.URL) Entry {
	if query, err := url.ParseQuery(u.RawQuery); err == nil && !query.Has("notebook") {
		return AccountEntry
	}
	return NotebookEntry
}

// entryOf is the entry of a POST to Pattern (M9 design 4.6): the account
// entry without a notebook parameter; the notebook entry with exactly one
// that uuid.Parse takes and that Gate lets through. A query that does not
// parse, the parameter twice or more, empty, or no UUID is
// notebook.not_found from Gate on the zero id, which it answers without a
// read. url.ParseQuery, not u.Query(): that drops a pair it cannot parse,
// notebook=%zz among them, and the client would land in the account
// entry; askedEntry reads the query alike. Other parameters are the
// client's own and are not read.
func (s *Server) entryOf(ctx context.Context, u *url.URL) (Entry, uuid.UUID, error) {
	query, err := url.ParseQuery(u.RawQuery)
	values, named := query["notebook"]
	switch {
	case err == nil && !named:
		return AccountEntry, uuid.UUID{}, nil
	case err == nil && len(values) == 1:
		if id, err := uuid.Parse(values[0]); err == nil {
			if err := s.notebooks.Gate(ctx, id); err != nil {
				return 0, uuid.UUID{}, err
			}
			return NotebookEntry, id, nil
		}
	}
	if err := s.notebooks.Gate(ctx, uuid.UUID{}); err != nil {
		return 0, uuid.UUID{}, err
	}
	return 0, uuid.UUID{}, errGateLetZero
}
