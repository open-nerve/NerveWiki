// Package mcpserver is nervewiki's MCP server (M9 design 4.1), the only
// production code that imports the MCP SDK: the stateless handler of
// both entries, /mcp and /mcp?notebook=<id>, each a *mcp.Server of its
// own; the registry's tools and prompts; every method's call context, its
// recovery from a panic, the results and errors with their bound; and the
// logs. It is part of httpserver, whose classification of errors and
// problems it answers with. The modules build its Tool values in their
// adapter/mcp and never meet the SDK's types; bootstrap fills Options.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// protocolVersions are the versions the server speaks (M9 design 4.3): an
// initialize asking for a later one is answered with 2025-11-25, and a
// request of 2026-07-28 is refused.
func protocolVersions() []string {
	return []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}
}

// Options configure the MCP server; bootstrap fills them.
type Options struct {
	Name, Version string // serverInfo
	Tools         []Tool
	Prompts       []Prompt // none in M9's production wiring; the platform test's double (M9/P1 design 3.4.7)
	Credentials   func(ctx context.Context) (Credential, bool)
	Notebooks     Notebooks   // the notebook entry's gate and instructions
	Account       string      // the account entry's instructions (S3; "" sends none)
	Names         ClientNames // the client names (S3; nil: every call's client is UnknownClient)
	Errors        httpserver.APIErrors
	// RequestTimeout is server.request_timeout: a call's work from the
	// request's body read, MaxBodyBytes the route's body limit.
	RequestTimeout time.Duration
	MaxBodyBytes   int64
	Logger         *slog.Logger
}

// Credential is the account a request acts as, and its personal access
// token: TokenID is the zero id for a session's access token.
type Credential struct{ UserID, TokenID uuid.UUID }

// Notebooks decide the notebook entry (page's MCPEntry).
type Notebooks interface {
	// Gate lets a call into the notebook, or answers why not: none, a
	// deleted one, one the caller cannot see, and the zero id, which it
	// answers without a read, are notebook.not_found.
	Gate(ctx context.Context, notebookID uuid.UUID) error
	// Instructions are the notebook's instructions for an initialize, ""
	// for none (S3).
	Instructions(ctx context.Context, notebookID uuid.UUID) (string, error)
}

// ClientNames clean the names clients tell and remember them on their
// personal access tokens (M9 design 4.5).
type ClientNames interface {
	Clean(name string) string // shared.CleanClientName: "" for none left
	Recorded(ctx context.Context, tokenID uuid.UUID) (string, error)
	Record(ctx context.Context, tokenID uuid.UUID, name string) (written bool, err error)
}

// Server serves both entries of Pattern. Its Tools and Prompts are fixed
// at New: the list of each entry never changes while it runs, so neither
// says listChanged.
type Server struct {
	credentials    func(ctx context.Context) (Credential, bool)
	notebooks      Notebooks
	names          ClientNames
	errors         httpserver.APIErrors
	requestTimeout time.Duration
	logger         *slog.Logger
	// notebookEntry and accountEntry are the entries' servers, handler
	// the SDK's stateless handler that getServer picks one of for.
	notebookEntry, accountEntry *mcp.Server
	handler                     http.Handler
}

// errNoCredential is a request that reached the server without the
// authentication's credential: the route is mounted wrongly.
var errNoCredential = errors.New("mcpserver: no credential on the request: the route is not behind the authentication")

// errUnwritten is a request whose context lives, that the SDK returned
// from without an answer: not a path the SDK has.
var errUnwritten = errors.New("mcpserver: the SDK wrote no answer")

// New builds the server: a wiring fault panics (a tool or a prompt named
// twice, a tool of no entry, an input type that is no struct, a schema it
// cannot normalize, a Notebook tool that is not in the account entry,
// where alone the flag adds its "notebook").
func New(o Options) *Server {
	switch {
	case o.Credentials == nil:
		panic("mcpserver: no credentials")
	case o.Notebooks == nil:
		panic("mcpserver: no notebooks")
	case o.Logger == nil:
		panic("mcpserver: no logger")
	case o.Errors == (httpserver.APIErrors{}):
		panic("mcpserver: no errors: the API's are the problems' writer")
	case o.RequestTimeout <= 0 || o.MaxBodyBytes <= 0:
		panic("mcpserver: the request timeout and the body limit must be positive")
	}
	s := &Server{
		credentials: o.Credentials, notebooks: o.Notebooks, names: o.Names, errors: o.Errors,
		requestTimeout: o.RequestTimeout, logger: o.Logger,
	}
	sdk := sdkLogger(o.Logger)
	s.notebookEntry, s.accountEntry = s.newEntry(o, sdk), s.newEntry(o, sdk)
	s.addTools(o.Tools)
	s.addPrompts(o.Prompts)
	s.handler = mcp.NewStreamableHTTPHandler(s.getServer, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// A reverse proxy on the same host forwards a Host that is no
		// loopback address; the bearer token guards the route (M9 design
		// 4.4).
		DisableLocalhostProtection: true,
		MaxRequestBodyBytes:        o.MaxBodyBytes,
		Logger:                     sdk,
	})
	return s
}

// newEntry is one entry's server, without its tools: the protocol versions,
// the capabilities that never change, every result private and never
// cached, and the receiving middlewares, outer outermost. Its instructions
// are the handshake's to fill per request.
func (s *Server) newEntry(o Options, logger *slog.Logger) *mcp.Server {
	caps := &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{ListChanged: false}}
	if len(o.Prompts) > 0 {
		// Left nil, the SDK fills it in with listChanged true once a
		// prompt is added.
		caps.Prompts = &mcp.PromptCapabilities{ListChanged: false}
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: o.Name, Version: o.Version}, &mcp.ServerOptions{
		Logger:                    logger,
		SupportedProtocolVersions: protocolVersions(),
		Capabilities:              caps,
		SetCacheable: func(_ context.Context, _ mcp.Request, c *mcp.Cacheable) {
			c.CacheScope, c.TTLMs = "private", 0
		},
	})
	srv.AddReceivingMiddleware(s.outer, s.handshake)
	return srv
}

// getServer is the server of the request's entry, which ServeHTTP put in
// its context. It is never nil: the SDK answers nil with 400.
func (s *Server) getServer(r *http.Request) *mcp.Server {
	return s.serverOf(requestOf(r.Context()).entry)
}

// serverOf is the server of entry e: the notebook entry's, or the
// account entry's for any other.
func (s *Server) serverOf(e Entry) *mcp.Server {
	if e == NotebookEntry {
		return s.notebookEntry
	}
	return s.accountEntry
}

// ServeHTTP serves a request that API.Wrap's middlewares let through, its
// body read and checked: a body over its method's bound is 413, as one
// over the route's limit is; a session's access token is 403 before
// anything else; a POST's entry is decided, its notebook gated; then the
// SDK. Every answer is no-store, and one the SDK never wrote is answered
// as Write answers why the request's context ended.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ns := &noStore{ResponseWriter: w}
	cred, ok := s.credentials(r.Context())
	switch {
	case !ok: // the authentication always ran: a wiring fault
		s.errors.Write(ns, r, errNoCredential)
		return
	}
	if err := sizeError(r); err != nil {
		s.errors.Write(ns, r, err)
		return
	}
	if cred.TokenID == (uuid.UUID{}) { // a session's access token
		s.errors.Write(ns, r, notPAT())
		return
	}
	req := &request{ctx: r.Context(), read: time.Now(), cred: cred, entry: AccountEntry}
	if r.Method == http.MethodPost { // other methods: the SDK's 405, before a read
		entry, notebook, err := s.entryOf(r.Context(), r.URL)
		if err != nil {
			s.errors.Write(ns, r, err)
			return
		}
		req.entry, req.notebook = entry, notebook
	}
	s.handler.ServeHTTP(ns, r.WithContext(context.WithValue(r.Context(), requestKey{}, req)))
	if !ns.wrote {
		// r's context ended while a method ran (M9/P1 design 3.2.2): the
		// client went away (499, debug), or the request's deadline passed
		// (500, warn); a live context the SDK wrote nothing for is a fault
		// (500, error).
		err := r.Context().Err()
		if err == nil {
			err = errUnwritten
		}
		s.errors.Write(ns, r, err)
	}
}

// String is the entry as the logs write it.
func (e Entry) String() string {
	switch e {
	case NotebookEntry:
		return "notebook"
	case AccountEntry:
		return "account"
	case BothEntries:
		return "both"
	}
	return fmt.Sprintf("Entry(%d)", uint8(e))
}
