package mcpserver

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// methodCallTool is the method of a tool's call.
const methodCallTool = "tools/call"

type requestKey struct{}

// request is what ServeHTTP tells the methods of one POST.
type request struct {
	ctx      context.Context // the POST's, with the per-route deadline
	read     time.Time       // when its body had been read: ServeHTTP runs after bodyshape read it
	cred     Credential
	entry    Entry
	notebook uuid.UUID
}

// requestOf is the request ServeHTTP put in ctx, which every method's
// context holds: the SDK hands each the POST's context, its values kept.
// It is never nil: the SDK is reached through ServeHTTP alone.
func requestOf(ctx context.Context) *request {
	r, _ := ctx.Value(requestKey{}).(*request)
	return r
}

// outer is the outermost receiving middleware of both servers: every
// method's call context, and the recovery (M9 design 4.3). Stateless, the
// SDK gives each method the POST's context without its deadline or its
// cancellation (jsonrpc2's notDone); the call's comes back from the
// request: the earlier of its deadline and the body's read plus
// server.request_timeout, and its end when the client goes or the server
// stops. A notification reaches no method stateless, so the end of the
// client's own request is the one cancellation there is.
func (s *Server) outer(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (res mcp.Result, err error) {
		r := requestOf(ctx)
		deadline := r.read.Add(s.requestTimeout)
		if d, ok := r.ctx.Deadline(); ok && d.Before(deadline) {
			deadline = d
		}
		call, cancel := context.WithDeadline(ctx, deadline) // keeps the SDK's values and its cancellation
		defer cancel()
		stop := context.AfterFunc(r.ctx, cancel) // the client went, or the server stops
		defer stop()
		defer s.recover(call, method, req, &res, &err)
		return next(call, method, req)
	}
}

// recover answers a panic of a method as withRecover answers one of a
// handler: an error log with the stack, the method, the tool and the
// request id; a tool error internal_error for tools/call, a JSON-RPC
// internal error for any other method. It never re-panics: no one above
// it in this goroutine would recover, and the process would exit. The
// tool is the client's word: logged only when it is a tool's name, never
// a name as long as the body.
func (s *Server) recover(ctx context.Context, method string, req mcp.Request, res *mcp.Result, err *error) {
	v := recover()
	if v == nil {
		return
	}
	tool := ""
	if call, ok := req.(*mcp.CallToolRequest); ok && call.Params != nil && toolName.MatchString(call.Params.Name) {
		tool = call.Params.Name
	}
	s.logPanic(ctx, method, tool, v)
	if method == methodCallTool {
		*res, *err = s.internalError(ctx, internalFailed), nil
		return
	}
	*res, *err = nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
}

// recoverTool is recover inside a tool's wrapper, which the SDK calls
// within its tools/call: the call's log line follows it with the answer.
func (s *Server) recoverTool(ctx context.Context, tool string, res **mcp.CallToolResult, err *error, outcome *string) {
	v := recover()
	if v == nil {
		return
	}
	s.logPanic(ctx, methodCallTool, tool, v)
	*res, *err, *outcome = s.internalError(ctx, internalFailed), nil, httpserver.CodeInternal
}

// logPanic logs a method's panic with its stack: never the arguments.
func (s *Server) logPanic(ctx context.Context, method, tool string, v any) {
	attrs := []slog.Attr{
		slog.String("request_id", httpserver.RequestID(ctx)),
		slog.String("method", method),
	}
	if tool != "" {
		attrs = append(attrs, slog.String("tool", tool))
	}
	attrs = append(attrs, slog.Any("panic", v), slog.String("stack", string(debug.Stack())))
	s.logger.LogAttrs(ctx, slog.LevelError, "MCP method panicked", attrs...)
}
