package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
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

// The large request bodies' bounds (M9/P2 design 3.2): largeBodies the
// server reads and serves at once, at most one of them a credential's,
// each waiting largeBodyWait at most for its slot, then 503. A call at
// /mcp's limit peaks at about +0.5 GB, so two together at about 1 GB (M9
// design 10); two is the owner's (M9 design 11, item 16), and the wait
// is page.parse_max_wait's default. /mcp's threshold is
// mcpserver.MaxOtherBytes, REST's server.max_body_bytes.
const (
	largeBodies   = 2
	largeBodyWait = 2 * time.Second
)

// mcpTools are the modules' MCP tools, each module's in turn (M9 design
// 4.1): page's, read's attachments asset's; linking's backlinks and index;
// notebook's notebooks.
func mcpTools(pg *page.Module, as *asset.Module, ln *linking.Module, nb *notebook.Module) []mcpserver.Tool {
	return slices.Concat(pg.Tools(page.ToolDeps{Attachments: pageAttachments{as.AttachmentReads()}}), ln.Tools(), nb.Tools())
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
// entry and the account entry's instructions, the client names on the
// tokens, and each call's work within server.request_timeout.
func mcpServer(cfg config.Config, logger *slog.Logger, api *httpserver.API, pg *page.Module, names mcpserver.ClientNames,
	tools []mcpserver.Tool,
) *mcpserver.Server {
	return mcpserver.New(mcpserver.Options{
		Name: "nervewiki", Version: buildinfo.Get().Version, Tools: tools,
		Credentials: mcpCredentials, Notebooks: pg.MCPEntry(), Account: page.AccountInstructions(), Names: names, Errors: api.Errors,
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

// accountWorkspaces is workspace's Memberships as the notebook module's
// AccountWorkspaces, which MCP's notebooks reads (M9/P1 design 3.17.1):
// the two modules do not import each other.
type accountWorkspaces struct {
	workspace.Memberships
}

// WorkspacesOf is the account's workspaces, with its role there and their
// slugs and names.
func (a accountWorkspaces) WorkspacesOf(ctx context.Context, userID uuid.UUID) ([]notebook.AccountWorkspace, error) {
	listed, err := a.Memberships.WorkspacesOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]notebook.AccountWorkspace, len(listed))
	for i, m := range listed {
		out[i] = notebook.AccountWorkspace{ID: m.WorkspaceID, Slug: m.Slug, Name: m.Name, Role: m.Role}
	}
	return out, nil
}

// mcpNames are identity's client names as mcpserver remembers them, each
// cleaned by shared.CleanClientName first, which the platform does not
// import (M9 design 4.5; v0.1 design 13.1 item 2).
type mcpNames struct{ identity.ClientNames }

// Clean is name as a token's client_name holds it: "" for none left.
func (mcpNames) Clean(name string) string { return shared.CleanClientName(name) }

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
