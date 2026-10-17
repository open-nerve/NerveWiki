package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// sdkLogger is logger for the SDK: warn and above. The SDK logs every
// stateless POST's session at info (sdk/mcp/server.go:1394, :1414, :1498),
// without a request id. The attributes go inside the filter: Logger.With
// would call the wrapper's WithAttrs, which an embedded Handler answers
// with the inner handler, unfiltered.
func sdkLogger(logger *slog.Logger) *slog.Logger {
	return slog.New(warnAndAbove{logger.Handler().WithAttrs([]slog.Attr{slog.String("component", "mcp-sdk")})})
}

// warnAndAbove passes on the records of warn and above, and stays itself
// through WithAttrs and WithGroup, so that no later With drops the filter.
type warnAndAbove struct{ slog.Handler }

func (h warnAndAbove) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn && h.Handler.Enabled(ctx, level)
}

func (h warnAndAbove) WithAttrs(attrs []slog.Attr) slog.Handler {
	return warnAndAbove{h.Handler.WithAttrs(attrs)}
}

func (h warnAndAbove) WithGroup(name string) slog.Handler {
	return warnAndAbove{h.Handler.WithGroup(name)}
}

// handshake gives every method its call's client, the name a request
// tells recorded on its token (M9/P1 design 3.16.4); and fills an
// initialize's result with the instructions of its entry, read after the
// SDK answered, so that no other method reads them (3.15.2), and logs it.
// The result is this request's own: setting its instructions changes no
// other's. Instructions that fail to read are the handshake's JSON-RPC
// internal error, never the built-in ones, which would tell an agent the
// notebook has no conventions.
func (s *Server) handshake(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		r := requestOf(ctx)
		ctx = withClient(ctx, s.names.of(ctx, r.cred.TokenID, toldName(req)))
		res, err := next(ctx, method, req)
		init, ok := res.(*mcp.InitializeResult)
		if !ok || err != nil {
			return res, err
		}
		text, err := s.instructionsOf(ctx, r)
		if err != nil {
			return nil, s.methodError(ctx, method, fmt.Errorf("mcpserver: the instructions: %w", err))
		}
		init.Instructions = text
		attrs := []slog.Attr{
			slog.String("client", clientOf(ctx)),
			slog.String("protocol_version", init.ProtocolVersion),
			slog.String("entry", r.entry.String()),
		}
		attrs = append(attrs, credentialAttrs(ctx, r.notebook, r.cred)...)
		s.logger.LogAttrs(ctx, slog.LevelInfo, "MCP initialize", attrs...)
		return init, nil
	}
}

// instructionsOf are the instructions of r's entry: its notebook's, which
// the Gate let in, or the account entry's ("" sends none).
func (s *Server) instructionsOf(ctx context.Context, r *request) (string, error) {
	if r.entry == NotebookEntry {
		return s.notebooks.Instructions(ctx, r.notebook)
	}
	return s.account, nil
}

// logCall logs a tool's call: who, through what, what came of it and how
// long it took, never its arguments nor its answer, which hold titles and
// content (v0.1 design 13.1 item 10).
func (s *Server) logCall(ctx context.Context, tool string, c Call, outcome string, took time.Duration) {
	attrs := []slog.Attr{slog.String("tool", tool)}
	attrs = append(attrs, credentialAttrs(ctx, c.Notebook, Credential{UserID: c.UserID, TokenID: c.TokenID})...)
	attrs = append(attrs, slog.String("client", c.Client), slog.String("outcome", outcome), slog.Int64("duration_ms", took.Milliseconds()))
	s.logger.LogAttrs(ctx, slog.LevelInfo, "MCP tool call", attrs...)
}

// credentialAttrs are the log's notebook, when there is one, the account,
// the token and the request id.
func credentialAttrs(ctx context.Context, notebook uuid.UUID, cred Credential) []slog.Attr {
	var attrs []slog.Attr
	if notebook != (uuid.UUID{}) {
		attrs = append(attrs, slog.String("notebook_id", notebook.String()))
	}
	return append(attrs,
		slog.String("user_id", cred.UserID.String()),
		slog.String("token_id", cred.TokenID.String()),
		slog.String("request_id", httpserver.RequestID(ctx)),
	)
}
