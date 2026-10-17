package bootstrap

import (
	"context"
	"log/slog"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// mcpBodyLimit is /mcp's body limit (M9 design 4.4): a call's string
// arguments hold at most a page's content and 1 MiB besides (paths, the
// text an edit replaces, a message), each byte at JSON's longest escape of
// one byte, "\u00XX", and room for the rest of the request.
const mcpBodyLimit = 6*(page.MaxContentBytes+1<<20) + 64<<10

// mcpTools are the modules' MCP tools, each module's in turn (M9 design
// 4.1): none yet, page's list and read come with M9/P1 S2, notebook's
// with S3, linking's with P4.
func mcpTools(*page.Module, *linking.Module, *notebook.Module) []mcpserver.Tool {
	return nil
}

// mcpServer is the MCP server of both entries: tools, the personal access
// tokens of the per-route middlewares' authentication, page's notebook
// entry, and each call's work within server.request_timeout.
func mcpServer(cfg config.Config, logger *slog.Logger, api *httpserver.API, pg *page.Module, tools []mcpserver.Tool) *mcpserver.Server {
	return mcpserver.New(mcpserver.Options{
		Name: "nervewiki", Version: buildinfo.Get().Version, Tools: tools,
		Credentials: mcpCredentials, Notebooks: pg.MCPEntry(), Errors: api.Errors,
		RequestTimeout: cfg.Server.RequestTimeout, MaxBodyBytes: mcpBodyLimit, Logger: logger,
	})
}

// mcpCredentials is the request's actor as mcpserver reads it, which does
// not import internal/shared: its account and its personal access token,
// the zero id for a session's access token.
func mcpCredentials(ctx context.Context) (mcpserver.Credential, bool) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return mcpserver.Credential{}, false
	}
	return mcpserver.Credential{UserID: actor.UserID, TokenID: actor.APITokenID}, true
}

// mountMCP mounts the MCP server on /mcp behind the per-route middlewares,
// without a method: GET and DELETE reach the SDK's 405 rather than the web
// app. The paths below it and under /.well-known/ answer 404 problem+json,
// not the web app's page that a client would read as OAuth's metadata;
// OAuth's registration at the origin answers OAuth's error, behind the
// per-route middlewares too, without a token (mcpPublicOperations), its
// body never read.
func mountMCP(router *httpserver.Router, api *httpserver.API, srv *mcpserver.Server) {
	router.Handle(mcpserver.Pattern, api.Wrap(srv, mcpserver.Bodies()))
	router.Handle(mcpserver.Pattern+"/", mcpserver.Subtree())
	router.Handle("/.well-known/", mcpserver.WellKnown())
	router.Handle(mcpserver.RegisterPattern, api.Wrap(mcpserver.Register(), &bodyshape.Table{}))
}

// mcpPublicOperations are the MCP routes that need no token: the client
// that posts its OAuth registration has none.
func mcpPublicOperations() []string { return []string{mcpserver.RegisterPattern} }

// mcpNoTokenDetails is the detail of /mcp's 401 without a token: the
// sentence of the 404s under /.well-known/ and of the registration's
// error, so that a client without OAuth shows what one with it does.
func mcpNoTokenDetails() map[string]string {
	return map[string]string{mcpserver.Pattern: mcpserver.NoOAuth}
}
