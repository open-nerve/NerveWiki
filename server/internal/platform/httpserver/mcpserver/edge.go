package mcpserver

import (
	"context"
	"encoding/json"
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

// Bodies is Pattern's body structure: one object, its members unchecked.
// An array (a JSON-RPC batch), a member named twice and an unpaired
// surrogate are 400 before the SDK (M9 design 4.3, 4.4). A function, not
// a package variable: each caller gets its own table (gochecknoglobals).
func Bodies() *bodyshape.Table {
	return &bodyshape.Table{
		Nodes: []bodyshape.Node{{Types: bodyshape.Object, Extra: bodyshape.Open, Items: bodyshape.Open}},
		Roots: map[string]int{Pattern: 0},
	}
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

// noOAuth is what a client looking for OAuth is told, under /.well-known/
// and at RegisterPattern alike.
const noOAuth = "This server has no OAuth: MCP takes a personal access token in the Authorization header."

// WellKnown answers every path under /.well-known/, where an MCP client
// that got a 401 looks for OAuth's metadata (M9 design 4.4): 404
// problem+json, so that it finds no OAuth rather than the web app's page.
func WellKnown() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpserver.WriteProblem(w, httpserver.Problem{
			Status: http.StatusNotFound,
			Code:   httpserver.CodeNotFound,
			Title:  http.StatusText(http.StatusNotFound),
			Detail: noOAuth,
		})
	})
}

// RegisterPattern is OAuth's dynamic client registration at the origin,
// where a client turns when no metadata names an authorization server
// (the default endpoints of MCP 2025-03-26): the TypeScript SDK posts its
// registration there after the 404s under /.well-known/, and shows what
// answers it as an OAuth error (M9/P1 S1's client trial, step 4b).
const RegisterPattern = "POST /register"

// Register answers RegisterPattern with OAuth's error, 400
// invalid_client_metadata (RFC 7591 3.2.2), never stored: a client shows
// its error_description as it is, where the web app's 405 would be a JSON
// parse error. It reads no body.
func Register() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(oauthError{Error: "invalid_client_metadata", Description: noOAuth}) // nothing to do if the client is gone
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

// entryOf is the entry of a POST to Pattern (M9 design 4.6): the account
// entry without a notebook parameter; the notebook entry with exactly one
// that uuid.Parse takes and that Gate lets through. A query that does not
// parse, the parameter twice or more, empty, or no UUID is
// notebook.not_found from Gate on the zero id, which it answers without a
// read; the query is parsed once, here. url.ParseQuery, not u.Query():
// that drops a pair it cannot parse, notebook=%zz among them, and the
// client would land in the account entry. Other parameters are the
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
