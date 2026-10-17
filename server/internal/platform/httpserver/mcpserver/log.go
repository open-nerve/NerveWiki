package mcpserver

import (
	"context"
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

// handshake fills an initialize's result: the instructions (S3), and
// logs it; it records a client's name told by any request (S3).
func (s *Server) handshake(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		res, err := next(ctx, method, req)
		if init, ok := res.(*mcp.InitializeResult); ok && err == nil {
			r := requestOf(ctx)
			attrs := []slog.Attr{
				slog.String("client", s.clientOf(req)),
				slog.String("protocol_version", init.ProtocolVersion),
				slog.String("entry", r.entry.String()),
			}
			attrs = append(attrs, credentialAttrs(ctx, r.notebook, r.cred)...)
			s.logger.LogAttrs(ctx, slog.LevelInfo, "MCP initialize", attrs...)
		}
		return res, err
	}
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
