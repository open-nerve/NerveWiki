package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
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
// 4.1): page's list and read, whose attachments asset reads; notebook's
// come with M9/P1 S3, linking's with P4.
func mcpTools(pg *page.Module, as *asset.Module, _ *linking.Module, _ *notebook.Module) []mcpserver.Tool {
	return pg.Tools(page.ToolDeps{Attachments: pageAttachments{as.AttachmentReads()}})
}

// pageAttachments is asset's AttachmentReads as page's read calls them
// (M9/P1 design 3.11.2): asset.not_found is page.not_found to the model,
// which asked for a path, and asset's missing file page's.
type pageAttachments struct {
	asset asset.AttachmentReads
}

func (p pageAttachments) Read(ctx context.Context, nodeID uuid.UUID) (page.Attachment, error) {
	a, err := p.asset.Read(ctx, nodeID)
	if err != nil {
		return page.Attachment{}, pageAttachmentErr(err)
	}
	return page.Attachment(a), nil
}

func (p pageAttachments) Content(ctx context.Context, a page.Attachment, maxBytes int64) ([]byte, int, int, error) {
	data, width, height, err := p.asset.Content(ctx, asset.Attachment(a), maxBytes)
	return data, width, height, pageAttachmentErr(err)
}

// pageAttachmentErr is err, asset's, in page's words.
func pageAttachmentErr(err error) error {
	switch {
	case errors.Is(err, asset.ErrNotFound):
		return page.ErrNotFound
	case errors.Is(err, asset.ErrNoFile):
		return page.ErrNoAttachmentFile
	}
	return err
}

// mcpServer is the MCP server of both entries: tools, the personal access
// tokens of the per-route middlewares' authentication, page's notebook
// entry and the account entry's instructions, and each call's work within
// server.request_timeout.
func mcpServer(cfg config.Config, logger *slog.Logger, api *httpserver.API, pg *page.Module, tools []mcpserver.Tool) *mcpserver.Server {
	return mcpserver.New(mcpserver.Options{
		Name: "nervewiki", Version: buildinfo.Get().Version, Tools: tools,
		Credentials: mcpCredentials, Notebooks: pg.MCPEntry(), Account: page.AccountInstructions(), Errors: api.Errors,
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
